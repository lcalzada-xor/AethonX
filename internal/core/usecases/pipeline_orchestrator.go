// internal/core/usecases/pipeline_orchestrator.go
package usecases

import (
	"context"
	"fmt"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/ui"
)

// PipelineOrchestrator coordina la ejecución de sources en stages secuenciales.
// Implementa stage-based execution pipeline con dependency resolution automático.
type PipelineOrchestrator struct {
	// sources lista completa de sources registradas
	sources []ports.Source

	// registry para obtener metadata de sources
	sourceMetadata map[string]ports.SourceMetadata

	// stages construidos dinámicamente mediante topological sort
	stages []Stage

	// Servicios auxiliares
	dedupeService  *DedupeService
	mergeService   *MergeService
	graphService   *GraphService
	logger         logx.Logger

	// Configuración de ejecución
	maxWorkers      int
	streamingWriter StreamingWriter
	streamingConfig StreamingConfig

	// Observers para eventos
	observers []ports.Notifier

	// UI Presenter para visualización del progreso
	presenter ui.Presenter

	// stageResults almacena resultados de todos los stages para estadísticas
	stageResults []StageResult
}

// PipelineOrchestratorOptions configura el pipeline orchestrator.
type PipelineOrchestratorOptions struct {
	Sources         []ports.Source
	SourceMetadata  map[string]ports.SourceMetadata
	Logger          logx.Logger
	Observers       []ports.Notifier
	MaxWorkers      int
	StreamingWriter StreamingWriter
	StreamingConfig StreamingConfig
	Presenter       ui.Presenter
}

// NewPipelineOrchestrator crea una nueva instancia del pipeline orchestrator.
func NewPipelineOrchestrator(opts PipelineOrchestratorOptions) *PipelineOrchestrator {
	if opts.MaxWorkers <= 0 {
		opts.MaxWorkers = 4
	}
	if opts.Logger == nil {
		opts.Logger = logx.New()
	}
	if opts.StreamingConfig.ArtifactThreshold <= 0 {
		opts.StreamingConfig.ArtifactThreshold = 1000
	}
	if opts.Presenter == nil {
		opts.Presenter = ui.NewNoopPresenter()
	}

	return &PipelineOrchestrator{
		sources:         opts.Sources,
		sourceMetadata:  opts.SourceMetadata,
		dedupeService:   NewDedupeService(),
		mergeService:    NewMergeService(opts.Logger),
		logger:          opts.Logger.With("component", "pipeline_orchestrator"),
		observers:       opts.Observers,
		maxWorkers:      opts.MaxWorkers,
		streamingWriter: opts.StreamingWriter,
		streamingConfig: opts.StreamingConfig,
		presenter:       opts.Presenter,
	}
}

// BuildStages construye los stages mediante topological sort del grafo de dependencias.
// Retorna los stages ordenados por nivel de dependencia.
func (p *PipelineOrchestrator) BuildStages(sources []ports.Source) ([]Stage, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("no sources provided")
	}

	p.logger.Info("building stages from dependency graph", "sources", len(sources))

	// Construir dependency graph
	graph := p.buildDependencyGraph(sources)

	// Ejecutar topological sort por niveles
	stages, err := p.topologicalSortByLevels(graph)
	if err != nil {
		return nil, fmt.Errorf("failed to build stages: %w", err)
	}

	p.logger.Info("stages built successfully",
		"stage_count", len(stages),
		"total_sources", len(sources),
	)

	for _, stage := range stages {
		p.logger.Debug("stage details",
			"stage_id", stage.ID,
			"stage_name", stage.Name,
			"sources", stage.SourceCount(),
		)
	}

	return stages, nil
}

// Run ejecuta el pipeline completo de stages.
func (p *PipelineOrchestrator) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	startTime := time.Now()

	// Validar target
	if err := target.Validate(); err != nil {
		return nil, fmt.Errorf("invalid target: %w", err)
	}

	// Filtrar sources compatibles con el scan mode
	compatibleSources := p.filterCompatibleSources(p.sources, target.Mode)
	if len(compatibleSources) == 0 {
		return nil, domain.ErrNoSourcesAvailable
	}

	// Resetear stageResults para esta ejecución
	p.stageResults = nil

	p.logger.Info("starting pipeline execution",
		"target", target.Root,
		"mode", target.Mode,
		"sources", len(compatibleSources),
		"workers", p.maxWorkers,
	)

	// Construir stages
	stages, err := p.BuildStages(compatibleSources)
	if err != nil {
		return nil, fmt.Errorf("failed to build stages: %w", err)
	}

	// Iniciar presentación visual
	p.presenter.Start(ui.ScanInfo{
		Target:         target.Root,
		Mode:           string(target.Mode),
		Workers:        p.maxWorkers,
		TimeoutSeconds: int(p.streamingConfig.ArtifactThreshold),
		StreamingOn:    p.streamingWriter != nil,
		TotalStages:    len(stages),
	})
	defer p.presenter.Close()

	// Inicializar resultado acumulador
	result := domain.NewScanResult(target)
	result.Metadata.TotalSources = len(compatibleSources)

	// Notificar inicio
	p.notifyEvent(ctx, ports.NewEvent(
		ports.EventTypeScanStarted,
		"pipeline_orchestrator",
		ports.ScanStartedEvent{
			ScanID: result.ID,
			Target: target,
		},
	))

	// Ejecutar stages secuencialmente
	for i, stage := range stages {
		stageStartTime := time.Now()
		p.logger.Info("executing stage",
			"stage_id", stage.ID,
			"stage_name", stage.Name,
			"sources", stage.SourceCount(),
		)

		// Notificar inicio de stage al presenter
		sourceNames := make([]string, 0, len(stage.Sources))
		for _, src := range stage.Sources {
			sourceNames = append(sourceNames, src.Name())
		}
		p.presenter.StartStage(ui.StageInfo{
			Number:      i + 1,
			TotalStages: len(stages),
			Name:        stage.Name,
			Sources:     sourceNames,
		})

		// Ejecutar stage con artifacts acumulados como input
		stageResult, err := p.executeStage(ctx, stage, result)
		if err != nil {
			// Fail-soft: log error pero continuar con siguientes stages
			p.logger.Warn("stage execution failed",
				"stage_id", stage.ID,
				"stage_name", stage.Name,
				"error", err.Error(),
			)
			result.AddWarning("pipeline", fmt.Sprintf("Stage '%s' failed: %v", stage.Name, err))
			continue
		}

		stageDuration := time.Since(stageStartTime)
		p.logger.Info("stage completed",
			"stage_id", stage.ID,
			"stage_name", stage.Name,
			"duration_ms", stageDuration.Milliseconds(),
			"artifacts", stageResult.TotalArtifacts(),
			"successful_sources", stageResult.SuccessfulSources(),
			"failed_sources", stageResult.FailedSources(),
		)

		// Almacenar resultado del stage para estadísticas
		p.stageResults = append(p.stageResults, *stageResult)

		// Notificar finalización de stage al presenter
		p.presenter.FinishStage(i+1, stageDuration)

		// Merge stage results con acumulador
		if stageResult.ConsolidatedResult != nil {
			result.Artifacts = append(result.Artifacts, stageResult.ConsolidatedResult.Artifacts...)
			result.Warnings = append(result.Warnings, stageResult.ConsolidatedResult.Warnings...)
			result.Errors = append(result.Errors, stageResult.ConsolidatedResult.Errors...)
		}

		// Deduplicar incrementalmente para reducir memory footprint
		result.Artifacts = p.dedupeService.Deduplicate(result.Artifacts)

		// Stream a disco si threshold excedido
		if p.streamingWriter != nil && len(result.Artifacts) >= p.streamingConfig.ArtifactThreshold {
			p.logger.Info("streaming accumulated results to disk",
				"artifacts", len(result.Artifacts),
				"threshold", p.streamingConfig.ArtifactThreshold,
			)

			filepath, writeErr := p.streamingWriter.WritePartial(fmt.Sprintf("stage_%d", stage.ID), result)
			if writeErr != nil {
				p.logger.Warn("failed to stream results", "error", writeErr.Error())
			} else {
				p.logger.Info("results streamed to disk", "file", filepath)
				result.Artifacts = nil // Free memory
			}
		}
	}

	// Consolidación final: cargar partial results si existen
	if p.streamingWriter != nil {
		p.logger.Info("loading partial results from disk")
		pattern := p.streamingWriter.GetPattern()
		partialResults, err := p.mergeService.LoadPartialResults(p.streamingConfig.OutputDir, pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to load partial results: %w", err)
		}

		if len(partialResults) > 0 {
			if err := p.mergeService.ConsolidateIntoResult(result, partialResults); err != nil {
				return nil, fmt.Errorf("failed to consolidate partial results: %w", err)
			}
			p.logger.Info("partial results consolidated", "sources", len(partialResults))
		}
	}

	// Deduplicación final
	result.Artifacts = p.dedupeService.Deduplicate(result.Artifacts)

	// Construir grafo de relaciones
	p.graphService = NewGraphService(result.Artifacts, p.logger)
	graphStats := p.graphService.GetStats()
	result.Metadata.TotalRelations = graphStats.TotalRelations
	result.Metadata.RelationsByType = graphStats.RelationsByType

	// Finalizar resultado
	result.Finalize()

	totalDuration := time.Since(startTime)
	p.logger.Info("pipeline execution completed",
		"target", target.Root,
		"total_duration_ms", totalDuration.Milliseconds(),
		"artifacts", len(result.Artifacts),
		"relations", graphStats.TotalRelations,
		"warnings", len(result.Warnings),
		"errors", len(result.Errors),
	)

	// Limpiar archivos parciales
	if p.streamingWriter != nil {
		pattern := p.streamingWriter.GetPattern()
		if err := p.mergeService.ClearPartialFiles(p.streamingConfig.OutputDir, pattern); err != nil {
			p.logger.Warn("failed to clear partial files", "error", err.Error())
		}
	}

	// Notificar finalización
	p.notifyEvent(ctx, ports.NewEvent(
		ports.EventTypeScanCompleted,
		"pipeline_orchestrator",
		ports.ScanCompletedEvent{
			ScanID:         result.ID,
			Target:         target,
			ArtifactsCount: len(result.Artifacts),
			Duration:       result.Metadata.Duration,
		},
	))

	// Calcular estadísticas y notificar al presenter
	artifactsByType := make(map[string]int)
	for _, artifact := range result.Artifacts {
		artifactsByType[string(artifact.Type)]++
	}

	// Calcular sources succeeded/failed de los resultados reales
	sourcesSucceeded := 0
	sourcesFailed := 0
	for _, stageResult := range p.stageResults {
		for _, sourceResult := range stageResult.SourceResults {
			if sourceResult.Error == nil {
				sourcesSucceeded++
			} else {
				sourcesFailed++
			}
		}
	}

	p.presenter.Finish(ui.ScanStats{
		TotalDuration:      totalDuration,
		TotalArtifacts:     len(result.Artifacts),
		UniqueArtifacts:    len(result.Artifacts),
		SourcesSucceeded:   sourcesSucceeded,
		SourcesFailed:      sourcesFailed,
		ArtifactsByType:    artifactsByType,
		RelationshipsBuilt: graphStats.TotalRelations,
	})

	return result, nil
}

// filterCompatibleSources filtra sources compatibles con el scan mode.
func (p *PipelineOrchestrator) filterCompatibleSources(sources []ports.Source, mode domain.ScanMode) []ports.Source {
	var compatible []ports.Source
	for _, s := range sources {
		if s.Mode().CompatibleWith(mode) {
			compatible = append(compatible, s)
		}
	}
	return compatible
}

// executeStage ejecuta un stage completo con concurrencia limitada.
func (p *PipelineOrchestrator) executeStage(ctx context.Context, stage Stage, inputArtifacts *domain.ScanResult) (*StageResult, error) {
	stageResult := &StageResult{
		StageID:            stage.ID,
		StageName:          stage.Name,
		SourceResults:      make([]SourceExecutionResult, 0, len(stage.Sources)),
		ConsolidatedResult: domain.NewScanResult(inputArtifacts.Target),
		Errors:             make([]error, 0),
		Warnings:           make([]string, 0),
	}

	// Ejecutar sources concurrentemente con worker pool pattern
	sem := make(chan struct{}, p.maxWorkers)
	results := make(chan SourceExecutionResult, len(stage.Sources))

	for _, source := range stage.Sources {
		go func(src ports.Source) {
			// Adquirir semáforo
			sem <- struct{}{}
			defer func() { <-sem }()

			// Ejecutar source
			execResult := p.executeSourceInStage(ctx, src, inputArtifacts)
			results <- execResult
		}(source)
	}

	// Recolectar resultados
	for i := 0; i < len(stage.Sources); i++ {
		execResult := <-results
		stageResult.SourceResults = append(stageResult.SourceResults, execResult)

		// Consolidar resultado si exitoso
		if execResult.Error == nil && execResult.Result != nil {
			// Merge artifacts
			stageResult.ConsolidatedResult.Artifacts = append(
				stageResult.ConsolidatedResult.Artifacts,
				execResult.Result.Artifacts...,
			)

			// Merge warnings y errors
			stageResult.ConsolidatedResult.Warnings = append(
				stageResult.ConsolidatedResult.Warnings,
				execResult.Result.Warnings...,
			)
			stageResult.ConsolidatedResult.Errors = append(
				stageResult.ConsolidatedResult.Errors,
				execResult.Result.Errors...,
			)
		} else if execResult.Error != nil {
			stageResult.Errors = append(stageResult.Errors, execResult.Error)
		}
	}

	close(results)

	return stageResult, nil
}

// executeSourceInStage ejecuta una source individual con manejo de inputs.
func (p *PipelineOrchestrator) executeSourceInStage(ctx context.Context, source ports.Source, inputArtifacts *domain.ScanResult) SourceExecutionResult {
	startTime := time.Now()
	sourceName := source.Name()

	p.logger.Debug("executing source", "source", sourceName)

	// Notificar inicio al presenter
	p.presenter.StartSource(0, sourceName) // stageNum no es necesario aquí

	// Notificar inicio
	p.notifyEvent(ctx, ports.NewEvent(
		ports.EventTypeSourceStarted,
		sourceName,
		nil,
	))

	var result *domain.ScanResult
	var err error

	// Verificar si la source implementa InputConsumer
	if consumer, ok := source.(ports.InputConsumer); ok {
		// Filtrar artifacts según InputArtifacts declarados
		filteredInput := p.filterInputArtifacts(source, inputArtifacts)
		result, err = consumer.RunWithInput(ctx, inputArtifacts.Target, filteredInput)
	} else {
		// Fallback: ejecutar sin inputs (source legacy)
		result, err = source.Run(ctx, inputArtifacts.Target)
	}

	duration := time.Since(startTime)

	execResult := SourceExecutionResult{
		SourceName: sourceName,
		Result:     result,
		Error:      err,
		Duration:   duration,
	}

	if err != nil {
		p.logger.Warn("source failed", "source", sourceName, "error", err.Error())
		p.notifyEvent(ctx, ports.NewEvent(
			ports.EventTypeSourceFailed,
			sourceName,
			err,
		))
		// Notificar error al presenter
		p.presenter.FinishSource(sourceName, ui.StatusError, duration, 0)
		return execResult
	}

	artifactCount := len(result.Artifacts)
	execResult.ArtifactCount = artifactCount

	p.logger.Debug("source completed",
		"source", sourceName,
		"artifacts", artifactCount,
		"duration_ms", duration.Milliseconds(),
	)

	// Stream si supera threshold
	if p.streamingWriter != nil && artifactCount >= p.streamingConfig.ArtifactThreshold {
		p.logger.Info("streaming source result to disk",
			"source", sourceName,
			"artifacts", artifactCount,
		)

		filepath, writeErr := p.streamingWriter.WritePartial(sourceName, result)
		if writeErr != nil {
			p.logger.Warn("failed to stream source result", "source", sourceName, "error", writeErr.Error())
		} else {
			p.logger.Info("source result streamed", "source", sourceName, "file", filepath)
			result.Artifacts = nil // Free memory
			execResult.StreamedToDisk = true
		}
	}

	// Notificar finalización
	p.notifyEvent(ctx, ports.NewEvent(
		ports.EventTypeSourceCompleted,
		sourceName,
		artifactCount,
	))

	// Notificar éxito al presenter
	status := ui.StatusSuccess
	if len(result.Warnings) > 0 {
		status = ui.StatusWarning
	}
	p.presenter.FinishSource(sourceName, status, duration, artifactCount)

	return execResult
}

// filterInputArtifacts filtra artifacts del input según InputArtifacts declarados por la source.
func (p *PipelineOrchestrator) filterInputArtifacts(source ports.Source, input *domain.ScanResult) *domain.ScanResult {
	sourceName := source.Name()
	meta, exists := p.sourceMetadata[sourceName]
	if !exists || len(meta.InputArtifacts) == 0 {
		// Sin metadata o sin InputArtifacts: retornar vacío
		return domain.NewScanResult(input.Target)
	}

	// Crear mapa de tipos requeridos para búsqueda rápida
	requiredTypes := make(map[domain.ArtifactType]bool)
	for _, artifactType := range meta.InputArtifacts {
		requiredTypes[artifactType] = true
	}

	// Filtrar artifacts
	filtered := domain.NewScanResult(input.Target)
	for _, artifact := range input.Artifacts {
		if requiredTypes[artifact.Type] {
			filtered.Artifacts = append(filtered.Artifacts, artifact)
		}
	}

	p.logger.Debug("filtered input artifacts",
		"source", sourceName,
		"total_input", len(input.Artifacts),
		"filtered_output", len(filtered.Artifacts),
	)

	return filtered
}

// notifyEvent envía una notificación a todos los observers de forma asíncrona.
func (p *PipelineOrchestrator) notifyEvent(ctx context.Context, event ports.Event) {
	for _, observer := range p.observers {
		go func(notifier ports.Notifier) {
			notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			if err := notifier.Notify(notifyCtx, event); err != nil {
				p.logger.Warn("notification failed", "error", err.Error())
			}
		}(observer)
	}
}
