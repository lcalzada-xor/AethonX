// internal/core/usecases/pipeline_orchestrator.go
package usecases

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
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
	dedupeService              *DedupeService
	mergeService               *MergeService
	graphService               *GraphService
	vulnEnrichmentService      *VulnerabilityEnrichmentService
	serviceVulnEnrichmentService interface {
		EnrichServices(artifacts []*domain.Artifact)
		Close() error
	}
	logger                     logx.Logger

	// Configuración de ejecución
	maxWorkers      int
	streamingWriter StreamingWriter
	streamingConfig StreamingConfig

	// Observers para eventos
	observers []ports.Notifier

	// UI Presenter para visualización del progreso
	presenter ui.Presenter
	uiConfig  UIConfig

	// Enrichment configuration
	vulnerabilityEnrichmentEnabled bool

	// stageResults almacena resultados de todos los stages para estadísticas
	stageResults []StageResult

	// sigintChannel recibe señales SIGINT para cancelación por stage
	sigintChannel chan struct{}
}

// PipelineOrchestratorOptions configura el pipeline orchestrator.
type PipelineOrchestratorOptions struct {
	Sources                        []ports.Source
	SourceMetadata                 map[string]ports.SourceMetadata
	Logger                         logx.Logger
	Observers                      []ports.Notifier
	MaxWorkers                     int
	StreamingWriter                StreamingWriter
	StreamingConfig                StreamingConfig
	Presenter                      ui.Presenter
	UIConfig                       UIConfig
	VulnerabilityEnrichmentService *VulnerabilityEnrichmentService
	VulnerabilityEnrichmentEnabled bool
	ServiceVulnEnrichmentService   interface {
		EnrichServices(artifacts []*domain.Artifact)
		Close() error
	}
	SigintChannel                  chan struct{}
}

// UIConfig contiene configuración de UI
type UIConfig struct {
	Mode        ui.UIMode
	ShowMetrics bool
	ShowPhases  bool
	TimeoutS    int
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
		opts.Presenter = ui.NewRawPresenter(ui.LogFormatText)
	}

	return &PipelineOrchestrator{
		sources:                        opts.Sources,
		sourceMetadata:                 opts.SourceMetadata,
		dedupeService:                  NewDedupeService(),
		mergeService:                   NewMergeService(opts.Logger),
		logger:                         opts.Logger.With("component", "pipeline_orchestrator"),
		observers:                      opts.Observers,
		maxWorkers:                     opts.MaxWorkers,
		streamingWriter:                opts.StreamingWriter,
		streamingConfig:                opts.StreamingConfig,
		presenter:                      opts.Presenter,
		uiConfig:                       opts.UIConfig,
		vulnEnrichmentService:          opts.VulnerabilityEnrichmentService,
		vulnerabilityEnrichmentEnabled: opts.VulnerabilityEnrichmentEnabled,
		serviceVulnEnrichmentService:   opts.ServiceVulnEnrichmentService,
		sigintChannel:                  opts.SigintChannel,
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
		TimeoutSeconds: p.uiConfig.TimeoutS,
		StreamingOn:    p.streamingWriter != nil,
		TotalStages:    len(stages),
		UIMode:         p.uiConfig.Mode,
		ShowMetrics:    p.uiConfig.ShowMetrics,
		ShowPhases:     p.uiConfig.ShowPhases,
	})
	defer p.presenter.Close()

	// Inicializar resultado acumulador
	result := domain.NewScanResult(target)
	result.Metadata.TotalSources = len(compatibleSources)

	// Generar seed URLs desde el target del usuario (bootstrap)
	seedArtifacts := p.generateSeedURLs(target)
	if len(seedArtifacts) > 0 {
		result.Artifacts = append(result.Artifacts, seedArtifacts...)
		p.logger.Info("generated seed URLs from target",
			"target", target.Root,
			"seed_urls", len(seedArtifacts),
		)
	}

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
		// Check if global context was cancelled (double Ctrl-C or SIGTERM)
		// If so, exit immediately without processing remaining stages
		if ctx.Err() != nil {
			p.logger.Info("global context cancelled, stopping pipeline execution",
				"remaining_stages", len(stages)-i,
				"reason", ctx.Err(),
			)
			p.presenter.Warning("Scan interrupted by user. Saving partial results...")
			break
		}

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

		// Create stage context with timeout from parent
		var stageCtx context.Context
		var stageCancel context.CancelFunc

		// Create child context from parent with optional timeout
		if p.uiConfig.TimeoutS > 0 {
			stageCtx, stageCancel = context.WithTimeout(ctx, time.Duration(p.uiConfig.TimeoutS)*time.Second)
		} else {
			stageCtx, stageCancel = context.WithCancel(ctx)
		}

		// Goroutine para escuchar SIGINT durante la ejecución del stage
		sigintDone := make(chan struct{})
		if p.sigintChannel != nil {
			go func() {
				select {
				case _, ok := <-p.sigintChannel:
					if !ok {
						// Canal cerrado (timeout global o shutdown), NO es Ctrl-C del usuario
						return
					}
					// SIGINT recibido, cancelar stage
					p.logger.Info("stage cancelled by SIGINT",
						"stage_id", stage.ID,
						"stage_name", stage.Name,
					)
					p.presenter.Warning(fmt.Sprintf("Stage %d cancelled by user (Ctrl-C), continuing to next stage...", i+1))
					stageCancel()
				case <-sigintDone:
					// Stage terminó normalmente
				}
			}()
		}

		// Ejecutar stage con artifacts acumulados como input
		stageResult, err := p.executeStage(stageCtx, stage, result)

		// Señalar que el stage terminó
		close(sigintDone)

		stageCancel() // Limpiar contexto del stage

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

	// Enrich vulnerabilities if enabled
	if p.vulnerabilityEnrichmentEnabled && p.vulnEnrichmentService != nil {
		p.logger.Info("enriching vulnerabilities")
		if err := p.vulnEnrichmentService.EnrichVulnerabilities(ctx, result); err != nil {
			p.logger.Warn("vulnerability enrichment failed", "error", err)
		}
	}

	// Enrich services with Vulners if enabled
	if p.serviceVulnEnrichmentService != nil {
		p.logger.Info("enriching services with vulnerability data")
		p.serviceVulnEnrichmentService.EnrichServices(result.Artifacts)
	}

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
		AllArtifacts:       result.Artifacts,
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

		// Consolidar resultado si hay artifacts (INCLUSO con error)
		// Esto permite aprovechar resultados parciales cuando una source da timeout o se cancela
		if execResult.Result != nil && len(execResult.Result.Artifacts) > 0 {
			// Merge artifacts parciales
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

			// Log si hay resultados parciales con error
			if execResult.Error != nil {
				p.logger.Info("consolidated partial results from failed source",
					"source", execResult.SourceName,
					"artifacts", len(execResult.Result.Artifacts),
					"error", execResult.Error.Error(),
				)
			}
		}

		// Registrar error (independientemente de si hay artifacts)
		if execResult.Error != nil {
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

	// Verificar si la source implementa StreamingSource para escuchar progreso
	var progressDone chan struct{}
	if streamingSource, ok := source.(ports.StreamingSource); ok {
		progressDone = make(chan struct{})
		go p.listenToProgress(ctx, streamingSource, sourceName, progressDone)
	}

	// Verificar si la source implementa InputConsumer
	if consumer, ok := source.(ports.InputConsumer); ok {
		p.logger.Debug("source implements InputConsumer",
			"source", sourceName,
			"type", fmt.Sprintf("%T", source),
		)
		// Filtrar artifacts según InputArtifacts declarados
		filteredInput := p.filterInputArtifacts(source, inputArtifacts)
		result, err = consumer.RunWithInput(ctx, inputArtifacts.Target, filteredInput)
	} else {
		p.logger.Debug("source does NOT implement InputConsumer, using Run()",
			"source", sourceName,
			"type", fmt.Sprintf("%T", source),
		)
		// Fallback: ejecutar sin inputs (source legacy)
		result, err = source.Run(ctx, inputArtifacts.Target)
	}

	// Detener goroutine de progreso si existe
	if progressDone != nil {
		close(progressDone)
	}

	duration := time.Since(startTime)

	// Calcular artifact count (puede ser 0 si hubo error total o resultados parciales)
	artifactCount := 0
	if result != nil {
		artifactCount = len(result.Artifacts)
	}

	execResult := SourceExecutionResult{
		SourceName:    sourceName,
		Result:        result,
		Error:         err,
		Duration:      duration,
		ArtifactCount: artifactCount,
	}

	// Manejar error (pero conservar resultados parciales)
	if err != nil {
		if artifactCount > 0 {
			// Resultados parciales: warning en lugar de error
			p.logger.Warn("source exited with error but produced partial results",
				"source", sourceName,
				"error", err.Error(),
				"artifacts", artifactCount,
			)
			p.notifyEvent(ctx, ports.NewEvent(
				ports.EventTypeSourceCompleted,
				sourceName,
				artifactCount,
			))

			// Generar summary con resultados parciales
			summary := p.buildSourceSummary(sourceName, result, nil, artifactCount)
			execResult.Summary = summary

			// Notificar warning al presenter
			p.presenter.FinishSource(sourceName, ui.StatusWarning, duration, artifactCount, summary)
		} else {
			// Error total sin resultados
			p.logger.Warn("source failed without results", "source", sourceName, "error", err.Error())
			p.notifyEvent(ctx, ports.NewEvent(
				ports.EventTypeSourceFailed,
				sourceName,
				err,
			))

			// Generar summary para error
			summary := p.buildSourceSummary(sourceName, nil, err, 0)
			execResult.Summary = summary

			// Notificar error al presenter
			p.presenter.FinishSource(sourceName, ui.StatusError, duration, 0, summary)
		}

		return execResult
	}

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

	// Generar summary para resultado exitoso
	summary := p.buildSourceSummary(sourceName, result, nil, artifactCount)
	execResult.Summary = summary

	// Notificar éxito al presenter
	status := ui.StatusSuccess
	if len(result.Warnings) > 0 {
		status = ui.StatusWarning
	}
	p.presenter.FinishSource(sourceName, status, duration, artifactCount, summary)

	return execResult
}

// filterInputArtifacts filtra artifacts del input según InputArtifacts declarados por la source.
func (p *PipelineOrchestrator) filterInputArtifacts(source ports.Source, input *domain.ScanResult) *domain.ScanResult {
	sourceName := source.Name()
	meta, exists := p.sourceMetadata[sourceName]
	if !exists || len(meta.InputArtifacts) == 0 {
		// Sin metadata o sin InputArtifacts: retornar vacío
		p.logger.Warn("no input artifacts metadata for source",
			"source", sourceName,
			"exists", exists,
			"input_artifacts_len", func() int {
				if exists {
					return len(meta.InputArtifacts)
				}
				return 0
			}(),
		)
		return domain.NewScanResult(input.Target)
	}

	// Crear mapa de tipos requeridos para búsqueda rápida
	requiredTypes := make(map[domain.ArtifactType]bool)
	for _, artifactType := range meta.InputArtifacts {
		requiredTypes[artifactType] = true
	}

	// Filtrar artifacts
	filtered := domain.NewScanResult(input.Target)
	inputTypeCount := make(map[domain.ArtifactType]int)
	for _, artifact := range input.Artifacts {
		inputTypeCount[artifact.Type]++
		if requiredTypes[artifact.Type] {
			filtered.Artifacts = append(filtered.Artifacts, artifact)
		}
	}

	p.logger.Info("filtered input artifacts",
		"source", sourceName,
		"total_input", len(input.Artifacts),
		"input_types", inputTypeCount,
		"required_types", meta.InputArtifacts,
		"filtered_output", len(filtered.Artifacts),
	)

	return filtered
}

// listenToProgress escucha el canal de progreso de un StreamingSource y actualiza el presenter.
func (p *PipelineOrchestrator) listenToProgress(ctx context.Context, source ports.StreamingSource, sourceName string, done chan struct{}) {
	progressCh := source.ProgressChannel()
	ticker := time.NewTicker(100 * time.Millisecond) // Debouncing: actualizar cada 100ms
	defer ticker.Stop()

	p.logger.Debug("progress listener started", "source", sourceName)

	var lastUpdate ports.ProgressUpdate
	var lastEmitted int

	for {
		select {
		case update, ok := <-progressCh:
			if !ok {
				// Canal cerrado, salir
				return
			}
			lastUpdate = update

		case <-ticker.C:
			// Emitir actualización con debouncing solo si hay cambios
			if lastUpdate.ArtifactCount > 0 && lastUpdate.ArtifactCount != lastEmitted {
				p.presenter.UpdateSource(sourceName, ui.ProgressMetrics{
					Current:    lastUpdate.ArtifactCount,
					Total:      0, // Indeterminado
					Percentage: -1,
					Phase:      lastUpdate.Message,
				})
				p.logger.Debug("progress update",
					"source", sourceName,
					"artifacts", lastUpdate.ArtifactCount,
					"message", lastUpdate.Message,
				)
				lastEmitted = lastUpdate.ArtifactCount
			}

		case <-done:
			// Source terminó, emitir última actualización si hay
			if lastUpdate.ArtifactCount > 0 && lastUpdate.ArtifactCount != lastEmitted {
				p.presenter.UpdateSource(sourceName, ui.ProgressMetrics{
					Current:    lastUpdate.ArtifactCount,
					Total:      0,
					Percentage: -1,
					Phase:      lastUpdate.Message,
				})
			}
			return

		case <-ctx.Done():
			// Contexto cancelado, salir
			return
		}
	}
}

// buildSourceSummary genera un resumen informativo del resultado de un source
func (p *PipelineOrchestrator) buildSourceSummary(
	sourceName string,
	result *domain.ScanResult,
	err error,
	artifactCount int,
) *ui.SourceSummary {
	// Caso 1: Error
	if err != nil {
		return &ui.SourceSummary{
			Summary: p.summarizeError(sourceName, err),
		}
	}

	// Caso 2: Sin resultados
	if artifactCount == 0 {
		// Verificar si hay warnings para mostrar contexto
		if result != nil && len(result.Warnings) > 0 {
			return &ui.SourceSummary{
				Summary: result.Warnings[0].Message,
			}
		}
		return &ui.SourceSummary{
			Summary: "no results found",
		}
	}

	// Caso 3: Resumen específico por source
	switch sourceName {
	case "waybackurls":
		return p.summarizeWaybackurls(result)
	case "httpx":
		return p.summarizeHTTPX(result)
	case "subfinder":
		return p.summarizeSubfinder(result)
	case "rdap":
		return p.summarizeRDAP(result)
	case "crtsh":
		return p.summarizeCRTSH(result)
	case "amass":
		return p.summarizeAmass(result)
	default:
		return p.summarizeGeneric(result)
	}
}

// summarizeError genera un resumen legible del error
func (p *PipelineOrchestrator) summarizeError(sourceName string, err error) string {
	errStr := err.Error()

	// Detectar errores comunes y humanizarlos
	switch {
	case strings.Contains(errStr, "executable file not found"):
		return fmt.Sprintf("binary '%s' not found in PATH", sourceName)
	case strings.Contains(errStr, "context deadline exceeded"):
		return "timeout exceeded"
	case strings.Contains(errStr, "connection refused"):
		return "connection refused"
	case strings.Contains(errStr, "no such host"):
		return "DNS resolution failed"
	case strings.Contains(errStr, "permission denied"):
		return "permission denied"
	default:
		// Truncar si es muy largo
		if len(errStr) > 200 {
			return errStr[:197] + "..."
		}
		return errStr
	}
}

// summarizeGeneric genera un resumen genérico por tipo de artifact
func (p *PipelineOrchestrator) summarizeGeneric(result *domain.ScanResult) *ui.SourceSummary {
	// Contar por tipo
	stats := result.Stats()

	// Tomar los 3 tipos más frecuentes
	type typeCount struct {
		name  string
		count int
	}

	types := make([]typeCount, 0, len(stats))
	for t, c := range stats {
		types = append(types, typeCount{name: t, count: c})
	}

	// Ordenar por count descendente
	sort.Slice(types, func(i, j int) bool {
		return types[i].count > types[j].count
	})

	// Construir resumen (max 3 tipos)
	parts := make([]string, 0, 3)
	for i := 0; i < len(types) && i < 3; i++ {
		parts = append(parts, fmt.Sprintf("%s: %d",
			strings.ToLower(types[i].name), types[i].count))
	}

	return &ui.SourceSummary{
		Summary: strings.Join(parts, " • "),
	}
}

// summarizeWaybackurls resume resultados de waybackurls
// Formato esperado: "raw: 8432 → filtered: 1247"
func (p *PipelineOrchestrator) summarizeWaybackurls(result *domain.ScanResult) *ui.SourceSummary {
	artifactCount := len(result.Artifacts)

	// Buscar estadísticas de filtrado en metadata
	inputURLs := 0
	outputURLs := 0

	if result.Metadata.Environment != nil {
		if inputStr, ok := result.Metadata.Environment["waybackurls_filter_input_urls"]; ok {
			inputURLs, _ = strconv.Atoi(inputStr)
		}
		if outputStr, ok := result.Metadata.Environment["waybackurls_filter_output_urls"]; ok {
			outputURLs, _ = strconv.Atoi(outputStr)
		}
	}

	// Si hay estadísticas de filtrado, mostrarlas
	if inputURLs > 0 && outputURLs > 0 && inputURLs != outputURLs {
		return &ui.SourceSummary{
			Summary: fmt.Sprintf("raw: %d → filtered: %d", inputURLs, outputURLs),
		}
	}

	// Si solo hay outputURLs (sin diferencia), mostrar solo el total
	if outputURLs > 0 {
		return &ui.SourceSummary{
			Summary: fmt.Sprintf("urls: %d", outputURLs),
		}
	}

	// Fallback: usar artifact count
	return &ui.SourceSummary{
		Summary: fmt.Sprintf("urls: %d", artifactCount),
	}
}

// summarizeHTTPX resume resultados de httpx
// Formato esperado: "probed: 156 → alive: 89 (57%)"
func (p *PipelineOrchestrator) summarizeHTTPX(result *domain.ScanResult) *ui.SourceSummary {
	probed := 0
	alive := 0

	// Buscar estadísticas en metadata (estos valores son precisos)
	if result.Metadata.Environment != nil {
		if probedStr, ok := result.Metadata.Environment["httpx_probed"]; ok {
			probed, _ = strconv.Atoi(probedStr)
		}
		if aliveStr, ok := result.Metadata.Environment["httpx_alive"]; ok {
			alive, _ = strconv.Atoi(aliveStr)
		}
	}

	// Si no hay metadata, contar URL artifacts (cada host alive genera 1 URL)
	if alive == 0 && len(result.Artifacts) > 0 {
		for _, artifact := range result.Artifacts {
			if artifact.Type == domain.ArtifactTypeURL {
				alive++
			}
		}
	}

	// Si hay estadísticas de probed y alive, mostrar porcentaje
	if probed > 0 && alive > 0 {
		percentage := (alive * 100) / probed
		return &ui.SourceSummary{
			Summary: fmt.Sprintf("probed: %d → alive: %d (%d%%)", probed, alive, percentage),
		}
	}

	// Si solo hay alive count
	if alive > 0 {
		return &ui.SourceSummary{
			Summary: fmt.Sprintf("alive: %d", alive),
		}
	}

	// Sin resultados
	return &ui.SourceSummary{
		Summary: "no results",
	}
}

// summarizeSubfinder resume resultados de subfinder
func (p *PipelineOrchestrator) summarizeSubfinder(result *domain.ScanResult) *ui.SourceSummary {
	subdomains := 0
	for _, a := range result.Artifacts {
		if a.Type == domain.ArtifactTypeSubdomain {
			subdomains++
		}
	}

	// Contar sources únicas mencionadas (si hay metadata)
	sourcesUsed := len(result.Metadata.SourcesUsed)

	if sourcesUsed > 0 {
		return &ui.SourceSummary{
			Summary: fmt.Sprintf("sources: %d • unique: %d", sourcesUsed, subdomains),
		}
	}

	return &ui.SourceSummary{
		Summary: fmt.Sprintf("subdomains: %d", subdomains),
	}
}

// summarizeRDAP resume resultados de RDAP
func (p *PipelineOrchestrator) summarizeRDAP(result *domain.ScanResult) *ui.SourceSummary {
	registrar := "unknown"
	nsCount := 0

	// Buscar en artifacts con metadata de tipo Domain
	for _, a := range result.Artifacts {
		if a.Type == domain.ArtifactTypeDomain && a.TypedMetadata != nil {
			if domainMeta, ok := a.TypedMetadata.(*metadata.DomainMetadata); ok {
				if domainMeta.Registrar != "" {
					registrar = domainMeta.Registrar
				}
				nsCount = len(domainMeta.Nameservers)
				break
			}
		}
	}

	return &ui.SourceSummary{
		Summary: fmt.Sprintf("registrar: %s • ns: %d", registrar, nsCount),
	}
}

// summarizeCRTSH resume resultados de crt.sh
func (p *PipelineOrchestrator) summarizeCRTSH(result *domain.ScanResult) *ui.SourceSummary {
	subdomains := 0
	certs := 0

	for _, a := range result.Artifacts {
		switch a.Type {
		case domain.ArtifactTypeSubdomain:
			subdomains++
		case domain.ArtifactTypeCertificate:
			certs++
		}
	}

	return &ui.SourceSummary{
		Summary: fmt.Sprintf("subdomains: %d • certs: %d", subdomains, certs),
	}
}

// summarizeAmass resume resultados de Amass
func (p *PipelineOrchestrator) summarizeAmass(result *domain.ScanResult) *ui.SourceSummary {
	subdomains := 0
	ips := 0
	asns := 0

	for _, a := range result.Artifacts {
		switch a.Type {
		case domain.ArtifactTypeSubdomain:
			subdomains++
		case domain.ArtifactTypeIP:
			ips++
		case domain.ArtifactTypeASN:
			asns++
		}
	}

	parts := []string{}
	if subdomains > 0 {
		parts = append(parts, fmt.Sprintf("subs: %d", subdomains))
	}
	if ips > 0 {
		parts = append(parts, fmt.Sprintf("ips: %d", ips))
	}
	if asns > 0 {
		parts = append(parts, fmt.Sprintf("asn: %d", asns))
	}

	if len(parts) == 0 {
		return &ui.SourceSummary{Summary: "no results"}
	}

	return &ui.SourceSummary{
		Summary: strings.Join(parts, " • "),
	}
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

// generateSeedURLs genera URLs base desde el target del usuario.
// Estas URLs seed se consideran "alive" (status 200) ya que el usuario las proporcionó.
// Esto garantiza que sources como golinkfinderevo siempre tengan al menos la URL raíz.
func (p *PipelineOrchestrator) generateSeedURLs(target domain.Target) []*domain.Artifact {
	var artifacts []*domain.Artifact
	timestamp := time.Now().Format(time.RFC3339)

	// Generar HTTPS URL (prioritaria)
	httpsURL := fmt.Sprintf("https://%s/", target.Root)
	httpsMeta := &metadata.DomainMetadata{
		HTTPStatus:  200,
		IsAlive:     true,
		ProbeStatus: "alive",
		LastProbed:  timestamp,
		ProbeSource: "bootstrap",
		HasSSL:      true,
	}

	httpsArtifact := domain.NewArtifactWithMetadata(
		domain.ArtifactTypeURL,
		httpsURL,
		"bootstrap",
		httpsMeta,
	)
	artifacts = append(artifacts, httpsArtifact)

	// Generar HTTP URL (fallback)
	httpURL := fmt.Sprintf("http://%s/", target.Root)
	httpMeta := &metadata.DomainMetadata{
		HTTPStatus:  200,
		IsAlive:     true,
		ProbeStatus: "alive",
		LastProbed:  timestamp,
		ProbeSource: "bootstrap",
		HasSSL:      false,
	}

	httpArtifact := domain.NewArtifactWithMetadata(
		domain.ArtifactTypeURL,
		httpURL,
		"bootstrap",
		httpMeta,
	)
	artifacts = append(artifacts, httpArtifact)

	p.logger.Debug("generated seed URLs",
		"target", target.Root,
		"https", httpsURL,
		"http", httpURL,
	)

	return artifacts
}
