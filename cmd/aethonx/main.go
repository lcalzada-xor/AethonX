// cmd/aethonx/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aethonx/internal/adapters/output"
	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/core/usecases"
	"aethonx/internal/platform/config"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/registry"
	"aethonx/internal/platform/resilience"

	// Import sources for auto-registration via init()
	_ "aethonx/internal/sources/crtsh"
	_ "aethonx/internal/sources/rdap"
)

var (
	// Rellenables con -ldflags en build
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// 1. Cargar config centralizada
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(2)
	}

	// Version flag
	if cfg.PrintVersion {
		fmt.Printf("AethonX %s (commit %s, built %s)\n", version, commit, date)
		return
	}

	// Validar target
	if cfg.Target == "" {
		fmt.Fprintln(os.Stderr, "missing -target, try: aethonx -target example.com")
		os.Exit(2)
	}

	// 2. Logger compartido
	logger := logx.New()

	logger.Info("aethonx starting",
		"version", version,
		"commit", commit,
		"date", date,
		"target", cfg.Target,
		"active", cfg.Active,
		"workers", cfg.Workers,
	)

	// 3. Contexto y señales para shutdown limpio
	ctx, cancel := rootContextWithSignals(cfg.TimeoutS)
	defer cancel()

	// 4. Construir target del dominio
	scanMode := domain.ScanModePassive
	if cfg.Active {
		scanMode = domain.ScanModeActive
	}

	target := domain.NewTarget(cfg.Target, scanMode)

	// Validar target
	if err := target.Validate(); err != nil {
		logger.Err(err, "phase", "validation")
		os.Exit(2)
	}

	// 5. Build sources from registry with resilience wrappers
	sources, err := buildSourcesWithResilience(logger, cfg)
	if err != nil {
		logger.Err(err, "phase", "source-build")
		os.Exit(2)
	}

	if len(sources) == 0 {
		logger.Err(fmt.Errorf("no sources enabled"))
		os.Exit(2)
	}

	// Asegurar cleanup de sources al finalizar
	defer func() {
		for _, src := range sources {
			if err := src.Close(); err != nil {
				logger.Warn("failed to close source",
					"source", src.Name(),
					"error", err.Error(),
				)
			}
		}
	}()

	logger.Info("sources built", "count", len(sources))

	// 6. Crear streaming writer
	scanID := fmt.Sprintf("scan-%d", time.Now().Unix())
	streamingWriter := output.NewStreamingWriter(cfg.OutputDir, scanID, cfg.Target, logger)

	logger.Info("streaming configured",
		"threshold", cfg.Streaming.ArtifactThreshold,
		"output_dir", cfg.OutputDir,
	)

	// 7. Crear orquestador
	orch := usecases.NewOrchestrator(usecases.OrchestratorOptions{
		Sources:         sources,
		Logger:          logger,
		Observers:       []ports.Notifier{}, // Futuro: webhooks, metrics, etc.
		MaxWorkers:      max(1, cfg.Workers),
		FailFast:        false,
		StreamingWriter: streamingWriter,
		StreamingConfig: usecases.StreamingConfig{
			ArtifactThreshold: cfg.Streaming.ArtifactThreshold,
			OutputDir:         cfg.OutputDir,
		},
	})

	// 8. Ejecutar flujo
	start := time.Now()
	result, runErr := orch.Run(ctx, *target)
	elapsed := time.Since(start)

	// Añadir metadata de versión
	if result != nil {
		result.Metadata.Version = version
		result.Metadata.Environment = map[string]string{
			"commit": commit,
			"date":   date,
		}
	}

	// 9. Manejo de error de ejecución
	if runErr != nil {
		logger.Err(runErr, "phase", "run", "elapsed_ms", elapsed.Milliseconds())
		// Continuamos para emitir lo que haya, útil en pipelines
	}

	// 10. Salidas
	if result != nil {
		outErr := writeOutputs(cfg, result)
		if outErr != nil {
			logger.Err(outErr, "phase", "output")
			os.Exit(1)
		}
	}

	// 11. Resumen
	if result != nil {
		logger.Info("aethonx finished",
			"elapsed_ms", elapsed.Milliseconds(),
			"artifacts", result.TotalArtifacts(),
			"warnings", len(result.Warnings),
			"errors", len(result.Errors),
		)
	}

	if runErr != nil {
		os.Exit(1)
	}
}

// buildSourcesWithResilience construye sources desde el registry con resilience wrappers.
func buildSourcesWithResilience(logger logx.Logger, cfg config.Config) ([]ports.Source, error) {
	// Build sources from registry
	sources, err := registry.Global().Build(cfg.Sources, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to build sources: %w", err)
	}

	// Wrap sources con resilience (retry + circuit breaker) si está habilitado
	if cfg.Resilience.CircuitBreakerEnabled {
		resilientSources := make([]ports.Source, 0, len(sources))

		for _, src := range sources {
			// Crear circuit breaker específico para esta source
			cb := resilience.NewCircuitBreaker(
				cfg.Resilience.CircuitBreakerThreshold,
				cfg.Resilience.CircuitBreakerTimeout,
				cfg.Resilience.CircuitBreakerHalfOpenMax,
			)

			// Wrap con RetryableSource
			retryable := resilience.NewRetryableSource(
				src,
				cfg.Resilience.MaxRetries,
				cfg.Resilience.BackoffBase,
				cfg.Resilience.BackoffMultiplier,
				cb,
				logger,
			)

			resilientSources = append(resilientSources, retryable)

			logger.Debug("wrapped source with resilience",
				"source", src.Name(),
				"max_retries", cfg.Resilience.MaxRetries,
				"circuit_breaker", "enabled",
			)
		}

		return resilientSources, nil
	}

	// Resilience deshabilitada, retornar sources sin wrapper
	logger.Debug("resilience disabled, using sources directly")
	return sources, nil
}

// writeOutputs decide y ejecuta las salidas según config.
// Mantener aislado del main facilita añadir nuevos formatos sin tocar el flujo.
func writeOutputs(cfg config.Config, result *domain.ScanResult) error {
	// SIEMPRE generar JSON consolidado (requerido para streaming)
	// Este archivo contiene el resultado final después de deduplicación y construcción del grafo
	if err := output.OutputJSON(cfg.OutputDir, result); err != nil {
		return fmt.Errorf("json output: %w", err)
	}

	// Tabla legible por terminal si no se desactiva
	if !cfg.Outputs.TableDisabled {
		if err := output.OutputTable(result); err != nil {
			return fmt.Errorf("table output: %w", err)
		}
	}

	return nil
}

// rootContextWithSignals crea un contexto raíz con timeout opcional y cancelación por señal.
// Retorna un contexto y una función cancel que limpia todos los recursos (señales, goroutines).
func rootContextWithSignals(timeoutSeconds int) (context.Context, context.CancelFunc) {
	var base context.Context
	var baseCancel context.CancelFunc

	if timeoutSeconds > 0 {
		base, baseCancel = context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	} else {
		base, baseCancel = context.WithCancel(context.Background())
	}

	// Canal para señales del sistema
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	// Goroutine que espera señales O cancelación del contexto
	go func() {
		select {
		case <-ch:
			// Señal recibida, cancelar contexto
			baseCancel()
		case <-base.Done():
			// Contexto cancelado por timeout u otra razón
			// La goroutine puede terminar
			return
		}
	}()

	// Función de cancelación que limpia TODO
	cleanupCancel := func() {
		signal.Stop(ch) // Detener el handler de señales
		close(ch)       // Cerrar el canal
		baseCancel()    // Cancelar el contexto base
	}

	return base, cleanupCancel
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
