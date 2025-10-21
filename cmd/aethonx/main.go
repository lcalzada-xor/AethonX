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
	_ "aethonx/internal/sources/httpx"
	_ "aethonx/internal/sources/rdap"
)

var (
	// Rellenables con -ldflags en build
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// 1. Load centralized config (handles help/version internally)
	cfg, err := config.Load(version, commit, date)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: configuration load failed: %v\n", err)
		os.Exit(2)
	}

	// Validate target
	if cfg.Core.Target == "" {
		fmt.Fprintln(os.Stderr, "Error: target domain is required")
		fmt.Fprintln(os.Stderr, "Usage: aethonx -t <domain>")
		fmt.Fprintln(os.Stderr, "Try: aethonx -h for help")
		os.Exit(2)
	}

	// 2. Shared logger
	logger := logx.New()

	logger.Info("AethonX starting",
		"version", version,
		"commit", commit,
		"date", date,
		"target", cfg.Core.Target,
		"active", cfg.Core.Active,
		"workers", cfg.Core.Workers,
	)

	// 3. Context and signals for clean shutdown
	ctx, cancel := rootContextWithSignals(cfg.Core.TimeoutS)
	defer cancel()

	// 4. Build target domain
	scanMode := domain.ScanModePassive
	if cfg.Core.Active {
		scanMode = domain.ScanModeActive
	}

	target := domain.NewTarget(cfg.Core.Target, scanMode)

	// Validate target
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

	// Ensure source cleanup on exit
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

	// 6. Create streaming writer
	scanID := fmt.Sprintf("scan-%d", time.Now().Unix())
	streamingWriter := output.NewStreamingWriter(cfg.Output.Dir, scanID, cfg.Core.Target, logger)

	logger.Info("streaming configured",
		"threshold", cfg.Streaming.ArtifactThreshold,
		"output_dir", cfg.Output.Dir,
	)

	// 7. Get source metadata from registry
	sourceMetadata := registry.Global().GetAllMetadata()

	// 8. Create pipeline orchestrator (stage-based execution)
	orch := usecases.NewPipelineOrchestrator(usecases.PipelineOrchestratorOptions{
		Sources:         sources,
		SourceMetadata:  sourceMetadata,
		Logger:          logger,
		Observers:       []ports.Notifier{}, // Future: webhooks, metrics, etc.
		MaxWorkers:      max(1, cfg.Core.Workers),
		StreamingWriter: streamingWriter,
		StreamingConfig: usecases.StreamingConfig{
			ArtifactThreshold: cfg.Streaming.ArtifactThreshold,
			OutputDir:         cfg.Output.Dir,
		},
	})

	// 9. Execute scan workflow
	start := time.Now()
	result, runErr := orch.Run(ctx, *target)
	elapsed := time.Since(start)

	// Add version metadata
	if result != nil {
		result.Metadata.Version = version
		result.Metadata.Environment = map[string]string{
			"commit": commit,
			"date":   date,
		}
	}

	// 10. Handle execution errors
	if runErr != nil {
		logger.Err(runErr, "phase", "run", "elapsed_ms", elapsed.Milliseconds())
		// Continue to emit partial results (useful in pipelines)
	}

	// 11. Write outputs
	if result != nil {
		outErr := writeOutputs(cfg, result)
		if outErr != nil {
			logger.Err(outErr, "phase", "output")
			os.Exit(1)
		}
	}

	// 12. Summary
	if result != nil {
		logger.Info("AethonX finished",
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

// buildSourcesWithResilience builds sources from registry with resilience wrappers.
func buildSourcesWithResilience(logger logx.Logger, cfg config.Config) ([]ports.Source, error) {
	// Build sources from registry
	sources, err := registry.Global().Build(cfg.Source.Sources, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to build sources: %w", err)
	}

	// Wrap sources with resilience (retry + circuit breaker) if enabled
	if cfg.Resilience.CircuitBreakerEnabled {
		resilientSources := make([]ports.Source, 0, len(sources))

		for _, src := range sources {
			// Create source-specific circuit breaker
			cb := resilience.NewCircuitBreaker(
				cfg.Resilience.CircuitBreakerThreshold,
				cfg.Resilience.CircuitBreakerTimeout,
				cfg.Resilience.CircuitBreakerHalfOpenMax,
			)

			// Wrap with RetryableSource
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

	// Resilience disabled, return sources without wrapper
	logger.Debug("resilience disabled, using sources directly")
	return sources, nil
}

// writeOutputs decides and executes outputs based on config.
// Keeping isolated from main makes it easier to add new formats.
func writeOutputs(cfg config.Config, result *domain.ScanResult) error {
	// ALWAYS generate consolidated JSON (required for streaming)
	// This file contains final result after deduplication and graph building
	if err := output.OutputJSON(cfg.Output.Dir, result); err != nil {
		return fmt.Errorf("json output: %w", err)
	}

	// Terminal-readable table if not disabled
	if !cfg.Output.TableDisabled {
		if err := output.OutputTable(result); err != nil {
			return fmt.Errorf("table output: %w", err)
		}
	}

	return nil
}

// rootContextWithSignals creates a root context with optional timeout and signal cancellation.
// Returns a context and cancel function that cleans up all resources (signals, goroutines).
func rootContextWithSignals(timeoutSeconds int) (context.Context, context.CancelFunc) {
	var base context.Context
	var baseCancel context.CancelFunc

	if timeoutSeconds > 0 {
		base, baseCancel = context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	} else {
		base, baseCancel = context.WithCancel(context.Background())
	}

	// System signal channel
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	// Goroutine waiting for signals OR context cancellation
	go func() {
		select {
		case sig := <-ch:
			// Signal received, cancel context
			_ = sig // Avoid unused variable warning
			baseCancel()
			// Goroutine terminates after canceling
		case <-base.Done():
			// Context canceled by timeout or other reason
			// Goroutine can terminate cleanly
		}
		// Goroutine always terminates here
	}()

	// Cleanup function that cleans up EVERYTHING
	cleanupCancel := func() {
		signal.Stop(ch) // Stop signal handler
		close(ch)       // Close channel
		baseCancel()    // Cancel base context
	}

	return base, cleanupCancel
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
