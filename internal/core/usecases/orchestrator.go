// internal/core/usecases/orchestrator.go
package usecases

import (
	"context"
	"fmt"
	"sync"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
)

// Orchestrator coordina la ejecución de múltiples fuentes de forma concurrente.
type Orchestrator struct {
	sources   []ports.Source
	dedupe    *DedupeService
	logger    logx.Logger
	observers []ports.Notifier

	// Configuración
	maxWorkers      int
	failFast        bool
	streamingWriter StreamingWriter
	streamingConfig StreamingConfig

	// Control de goroutines
	notifyWg sync.WaitGroup
}

// OrchestratorOptions configura el orchestrator.
type OrchestratorOptions struct {
	Sources         []ports.Source
	Logger          logx.Logger
	Observers       []ports.Notifier
	MaxWorkers      int
	FailFast        bool
	StreamingWriter StreamingWriter
	StreamingConfig StreamingConfig
}

// StreamingWriter es la interfaz para escribir resultados parciales.
type StreamingWriter interface {
	WritePartial(sourceName string, result *domain.ScanResult) (string, error)
	GetPattern() string
	GetFinalFilename() string
}

// StreamingConfig configura el comportamiento de streaming.
type StreamingConfig struct {
	ArtifactThreshold int
	OutputDir         string
}

// NewOrchestrator crea una nueva instancia del orchestrator.
func NewOrchestrator(opts OrchestratorOptions) *Orchestrator {
	if opts.MaxWorkers <= 0 {
		opts.MaxWorkers = 4
	}
	if opts.Logger == nil {
		opts.Logger = logx.New()
	}
	if opts.StreamingConfig.ArtifactThreshold <= 0 {
		opts.StreamingConfig.ArtifactThreshold = 1000 // default
	}

	return &Orchestrator{
		sources:         opts.Sources,
		dedupe:          NewDedupeService(),
		logger:          opts.Logger.With("component", "orchestrator"),
		observers:       opts.Observers,
		maxWorkers:      opts.MaxWorkers,
		failFast:        opts.FailFast,
		streamingWriter: opts.StreamingWriter,
		streamingConfig: opts.StreamingConfig,
	}
}

// Run ejecuta todas las fuentes compatibles contra el target.
func (o *Orchestrator) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	// Validar target
	if err := target.Validate(); err != nil {
		return nil, err
	}

	// Crear resultado
	result := domain.NewScanResult(target)

	// Filtrar fuentes compatibles con el modo de escaneo
	sources := o.filterCompatibleSources(target.Mode)
	if len(sources) == 0 {
		return nil, domain.ErrNoSourcesAvailable
	}

	result.Metadata.TotalSources = len(sources)
	o.logger.Info("starting scan",
		"target", target.Root,
		"mode", target.Mode,
		"sources", len(sources),
		"workers", o.maxWorkers,
	)

	// Notificar inicio
	o.notify(ctx, ports.NewEvent(
		ports.EventTypeScanStarted,
		"orchestrator",
		ports.ScanStartedEvent{
			ScanID: result.ID,
			Target: target,
		},
	))

	// Ejecutar fuentes en paralelo
	sourceResults := o.executeSources(ctx, sources, target)

	// Consolidar resultados en memoria
	o.consolidateResults(result, sourceResults)

	// Si hay streaming writer, cargar archivos parciales
	if o.streamingWriter != nil {
		o.logger.Info("loading partial results from disk")
		merger := NewMergeService(o.logger)
		pattern := o.streamingWriter.GetPattern()

		partialResults, err := merger.LoadPartialResults(o.streamingConfig.OutputDir, pattern)
		if err != nil {
			o.logger.Warn("failed to load partial results", "error", err.Error())
		} else if len(partialResults) > 0 {
			// Consolidar artifacts de archivos parciales
			merger.ConsolidateIntoResult(result, partialResults)
			o.logger.Info("partial results consolidated",
				"sources", len(partialResults),
				"total_artifacts", len(result.Artifacts),
			)
		}
	}

	// Deduplicar y normalizar artifacts (ahora con todos los artifacts)
	result.Artifacts = o.dedupe.Deduplicate(result.Artifacts)

	// Construir grafo y agregar estadísticas (requiere todos los artifacts deduplicados)
	graph := NewGraphService(result.Artifacts, o.logger)
	graphStats := graph.GetStats()

	// Almacenar estadísticas del grafo en metadata
	result.Metadata.TotalRelations = graphStats.TotalRelations
	result.Metadata.RelationsByType = graphStats.RelationsByType

	// Finalizar resultado
	result.Finalize()

	o.logger.Info("scan completed",
		"target", target.Root,
		"artifacts", len(result.Artifacts),
		"relations", graphStats.TotalRelations,
		"warnings", len(result.Warnings),
		"errors", len(result.Errors),
		"duration_ms", result.Metadata.Duration.Milliseconds(),
	)

	// Limpiar archivos parciales después de consolidación exitosa
	if o.streamingWriter != nil {
		merger := NewMergeService(o.logger)
		pattern := o.streamingWriter.GetPattern()
		if err := merger.ClearPartialFiles(o.streamingConfig.OutputDir, pattern); err != nil {
			o.logger.Warn("failed to clear partial files", "error", err.Error())
		}
	}

	// Notificar finalización
	o.notify(ctx, ports.NewEvent(
		ports.EventTypeScanCompleted,
		"orchestrator",
		ports.ScanCompletedEvent{
			ScanID:         result.ID,
			Target:         target,
			ArtifactsCount: len(result.Artifacts),
			Duration:       result.Metadata.Duration,
		},
	))

	// Esperar a que todas las notificaciones terminen antes de retornar
	o.logger.Debug("waiting for all notifications to complete")
	o.notifyWg.Wait()
	o.logger.Debug("all notifications completed")

	return result, nil
}

// filterCompatibleSources filtra fuentes compatibles con el modo de escaneo.
func (o *Orchestrator) filterCompatibleSources(mode domain.ScanMode) []ports.Source {
	var compatible []ports.Source
	for _, s := range o.sources {
		if s.Mode().CompatibleWith(mode) {
			compatible = append(compatible, s)
		}
	}
	return compatible
}

// executeSources ejecuta las fuentes en paralelo con límite de workers.
func (o *Orchestrator) executeSources(
	ctx context.Context,
	sources []ports.Source,
	target domain.Target,
) []sourceResult {
	sem := make(chan struct{}, o.maxWorkers)
	var wg sync.WaitGroup
	results := make([]sourceResult, 0, len(sources))
	resultsMu := sync.Mutex{}

	for _, source := range sources {
		wg.Add(1)
		go func(s ports.Source) {
			defer wg.Done()

			// Adquirir semáforo
			sem <- struct{}{}
			defer func() { <-sem }()

			// Ejecutar fuente
			res := o.executeSource(ctx, s, target)

			// Guardar resultado
			resultsMu.Lock()
			results = append(results, res)
			resultsMu.Unlock()
		}(source)
	}

	wg.Wait()
	return results
}

// executeSource ejecuta una fuente individual y maneja errores.
func (o *Orchestrator) executeSource(
	ctx context.Context,
	source ports.Source,
	target domain.Target,
) sourceResult {
	sourceName := source.Name()
	o.logger.Debug("executing source", "source", sourceName)

	// Notificar inicio de fuente
	o.notify(ctx, ports.NewEvent(
		ports.EventTypeSourceStarted,
		sourceName,
		nil,
	))

	// Ejecutar fuente
	scanResult, err := source.Run(ctx, target)

	if err != nil {
		o.logger.Warn("source failed", "source", sourceName, "error", err.Error())
		o.notify(ctx, ports.NewEvent(
			ports.EventTypeSourceFailed,
			sourceName,
			err,
		))
		return sourceResult{
			source: sourceName,
			err:    err,
		}
	}

	artifactCount := len(scanResult.Artifacts)
	o.logger.Debug("source completed",
		"source", sourceName,
		"artifacts", artifactCount,
	)

	// Si streaming está habilitado Y se supera el threshold, escribir parcial
	if o.streamingWriter != nil && artifactCount >= o.streamingConfig.ArtifactThreshold {
		o.logger.Info("writing partial result to disk",
			"source", sourceName,
			"artifacts", artifactCount,
			"threshold", o.streamingConfig.ArtifactThreshold,
		)

		filepath, writeErr := o.streamingWriter.WritePartial(sourceName, scanResult)
		if writeErr != nil {
			o.logger.Warn("failed to write partial result", "source", sourceName, "error", writeErr.Error())
		} else {
			o.logger.Info("partial result written",
				"source", sourceName,
				"file", filepath,
			)

			// Liberar artifacts de memoria, mantener solo metadata y contadores
			// El GC podrá liberar la memoria ahora
			scanResult.Artifacts = nil

			// Marcar que este resultado fue escrito a disco
			scanResult.AddWarning(sourceName, fmt.Sprintf("artifacts written to disk (%d artifacts)", artifactCount))
		}
	}

	// Notificar finalización de fuente
	o.notify(ctx, ports.NewEvent(
		ports.EventTypeSourceCompleted,
		sourceName,
		artifactCount,
	))

	return sourceResult{
		source: sourceName,
		result: scanResult,
	}
}

// consolidateResults consolida resultados de todas las fuentes.
func (o *Orchestrator) consolidateResults(
	result *domain.ScanResult,
	sourceResults []sourceResult,
) {
	for _, sr := range sourceResults {
		result.Metadata.SourcesUsed = append(result.Metadata.SourcesUsed, sr.source)

		if sr.err != nil {
			result.AddError(sr.source, sr.err.Error(), false)
			continue
		}

		if sr.result != nil {
			// Añadir artifacts
			result.Artifacts = append(result.Artifacts, sr.result.Artifacts...)

			// Añadir warnings
			for _, w := range sr.result.Warnings {
				result.Warnings = append(result.Warnings, w)
			}

			// Añadir errores
			for _, e := range sr.result.Errors {
				result.Errors = append(result.Errors, e)
			}
		}
	}
}

// notify envía una notificación a todos los observers.
// Usa goroutines con WaitGroup y timeout para evitar leaks y bloqueos.
func (o *Orchestrator) notify(ctx context.Context, event ports.Event) {
	const notificationTimeout = 5 * time.Second

	for _, observer := range o.observers {
		o.notifyWg.Add(1)
		go func(notifier ports.Notifier) {
			defer o.notifyWg.Done()

			// Crear contexto con timeout para esta notificación
			notifyCtx, cancel := context.WithTimeout(ctx, notificationTimeout)
			defer cancel()

			// Canal para capturar el resultado
			done := make(chan error, 1)

			// Ejecutar notificación en goroutine separada
			go func() {
				done <- notifier.Notify(notifyCtx, event)
			}()

			// Esperar resultado o timeout
			select {
			case err := <-done:
				if err != nil {
					o.logger.Warn("notification failed", "error", err.Error())
				}
			case <-notifyCtx.Done():
				if notifyCtx.Err() == context.DeadlineExceeded {
					o.logger.Warn("notification timeout exceeded",
						"timeout", notificationTimeout,
						"event_type", event.Type,
					)
				}
			}
		}(observer)
	}
}

// sourceResult encapsula el resultado de ejecución de una fuente.
type sourceResult struct {
	source string
	result *domain.ScanResult
	err    error
}
