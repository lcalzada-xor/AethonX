// internal/core/usecases/orchestrator.go
package usecases

import (
	"context"
	"sync"

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
	maxWorkers int
	failFast   bool
}

// OrchestratorOptions configura el orchestrator.
type OrchestratorOptions struct {
	Sources    []ports.Source
	Logger     logx.Logger
	Observers  []ports.Notifier
	MaxWorkers int
	FailFast   bool
}

// NewOrchestrator crea una nueva instancia del orchestrator.
func NewOrchestrator(opts OrchestratorOptions) *Orchestrator {
	if opts.MaxWorkers <= 0 {
		opts.MaxWorkers = 4
	}
	if opts.Logger == nil {
		opts.Logger = logx.New()
	}

	return &Orchestrator{
		sources:    opts.Sources,
		dedupe:     NewDedupeService(),
		logger:     opts.Logger.With("component", "orchestrator"),
		observers:  opts.Observers,
		maxWorkers: opts.MaxWorkers,
		failFast:   opts.FailFast,
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

	// Consolidar resultados
	o.consolidateResults(result, sourceResults)

	// Deduplicar y normalizar artifacts
	result.Artifacts = o.dedupe.Deduplicate(result.Artifacts)

	// Finalizar resultado
	result.Finalize()

	o.logger.Info("scan completed",
		"target", target.Root,
		"artifacts", len(result.Artifacts),
		"warnings", len(result.Warnings),
		"errors", len(result.Errors),
		"duration_ms", result.Metadata.Duration.Milliseconds(),
	)

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

	o.logger.Debug("source completed",
		"source", sourceName,
		"artifacts", len(scanResult.Artifacts),
	)

	// Notificar finalización de fuente
	o.notify(ctx, ports.NewEvent(
		ports.EventTypeSourceCompleted,
		sourceName,
		len(scanResult.Artifacts),
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
func (o *Orchestrator) notify(ctx context.Context, event ports.Event) {
	for _, observer := range o.observers {
		// Ejecutar de forma asíncrona para no bloquear
		go func(notifier ports.Notifier) {
			if err := notifier.Notify(ctx, event); err != nil {
				o.logger.Warn("notification failed", "error", err.Error())
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
