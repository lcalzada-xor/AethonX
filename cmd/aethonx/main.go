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
	"aethonx/internal/platform/cveapi"
	"aethonx/internal/platform/cveapi/providers"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/registry"
	"aethonx/internal/platform/resilience"
	"aethonx/internal/platform/ui"

	// Import sources for auto-registration via init()
	_ "aethonx/internal/sources/crtsh"
	_ "aethonx/internal/sources/dnsx"
	_ "aethonx/internal/sources/golinkfinderevo"
	_ "aethonx/internal/sources/httpx"
	_ "aethonx/internal/sources/rdap"
	_ "aethonx/internal/sources/shodan"
	_ "aethonx/internal/sources/subfinder"
	_ "aethonx/internal/sources/waybackurls"
	_ "aethonx/internal/sources/whatweb"
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

	// 2. Determine UI mode and create appropriate logger
	// Pretty mode: Use silent logger (only errors)
	// Raw mode: Use regular logger
	usingVisualUI := cfg.Output.UIMode == "pretty" || cfg.Output.UIMode == ""

	var logger logx.Logger
	if usingVisualUI {
		// Pretty mode: silent logger (only critical errors go to stderr)
		logger = logx.NewSilent()
	} else {
		// Non-visual mode: regular logger respecting AETHONX_LOG_LEVEL
		logger = logx.New()
		logger.Info("AethonX starting",
			"version", version,
			"commit", commit,
			"date", date,
			"target", cfg.Core.Target,
			"active", cfg.Core.Active,
			"workers", cfg.Core.Workers,
		)
	}

	// 3. Context and signals for clean shutdown
	ctx, cancel, sigintChannel := rootContextWithSignals(cfg.Core.TimeoutS)
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

	// Inject active mode into all source configs (for hybrid sources)
	for sourceName, sourceConfig := range cfg.Source.Sources {
		if sourceConfig.Custom == nil {
			sourceConfig.Custom = make(map[string]interface{})
		}
		sourceConfig.Custom["active_mode"] = cfg.Core.Active
		cfg.Source.Sources[sourceName] = sourceConfig
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
				// Only log cleanup errors if not in visual mode
				if !usingVisualUI {
					logger.Warn("failed to close source",
						"source", src.Name(),
						"error", err.Error(),
					)
				}
			}
		}
	}()

	if !usingVisualUI {
		logger.Info("sources built", "count", len(sources))
	}

	// 6. Create streaming writer
	scanID := fmt.Sprintf("scan-%d", time.Now().Unix())
	streamingWriter := output.NewStreamingWriter(cfg.Output.Dir, scanID, cfg.Core.Target, logger)

	if !usingVisualUI {
		logger.Info("streaming configured",
			"threshold", cfg.Streaming.ArtifactThreshold,
			"output_dir", cfg.Output.Dir,
		)
	}

	// 7. Create enrichment service if enabled
	var vulnEnrichmentSvc *usecases.VulnerabilityEnrichmentService
	if cfg.Enrichment.Enabled {
		// Create CVE enrichment service
		enrichmentConfig := cveapi.EnrichmentConfig{
			CacheTTL:      cfg.Enrichment.CacheTTL,
			Timeout:       cfg.Enrichment.Timeout,
			MaxConcurrent: cfg.Enrichment.MaxConcurrent,
		}
		cveEnrichmentSvc := cveapi.NewEnrichmentService(enrichmentConfig, logger)

		// Register providers
		// Primary: NVD
		nvdProvider := providers.NewNVDProvider(providers.NVDConfig{
			APIKey: cfg.Enrichment.NVDAPIKey,
		}, logger)
		if err := cveEnrichmentSvc.RegisterProvider(nvdProvider); err != nil {
			logger.Warn("failed to register NVD provider", "error", err)
		}

		// Fallback: circl
		circlProvider := providers.NewCirclProvider(providers.CirclConfig{}, logger)
		if err := cveEnrichmentSvc.RegisterProvider(circlProvider); err != nil {
			logger.Warn("failed to register circl provider", "error", err)
		}

		// Create vulnerability enrichment service
		vulnEnrichmentSvc = usecases.NewVulnerabilityEnrichmentService(
			cveEnrichmentSvc,
			cfg.Enrichment.MaxConcurrent,
			logger,
		)

		if !usingVisualUI {
			logger.Info("CVE enrichment enabled",
				"provider", cfg.Enrichment.Provider,
				"cache_ttl", cfg.Enrichment.CacheTTL,
			)
		}
	}

	// 8. Get source metadata from registry
	sourceMetadata := registry.Global().GetAllMetadata()

	// 9. Create UI presenter based on configuration
	var presenter ui.Presenter
	switch cfg.Output.UIMode {
	case "raw":
		// Raw mode: plain logs (text or JSON format)
		logFormat := ui.LogFormatText
		if cfg.Output.LogFormat == "json" {
			logFormat = ui.LogFormatJSON
		}
		presenter = ui.NewRawPresenter(logFormat)
	case "pretty":
		// Pretty mode: visual UI with custom renderer
		presenter = ui.NewCustomPresenter()
	default:
		// Default to pretty mode
		presenter = ui.NewCustomPresenter()
	}

	// 10. Create pipeline orchestrator (stage-based execution)
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
		Presenter: presenter,
		UIConfig: usecases.UIConfig{
			Mode:        ui.UIMode(cfg.Output.UIMode),
			ShowMetrics: cfg.Output.ShowMetrics,
			ShowPhases:  cfg.Output.ShowPhases,
			TimeoutS:    cfg.Core.TimeoutS,
		},
		VulnerabilityEnrichmentService: vulnEnrichmentSvc,
		VulnerabilityEnrichmentEnabled: cfg.Enrichment.Enabled,
		SigintChannel:                  sigintChannel,
	})

	// 11. Execute scan workflow
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

	// 12. Handle execution errors
	if runErr != nil {
		logger.Err(runErr, "phase", "run", "elapsed_ms", elapsed.Milliseconds())
		// Continue to emit partial results (useful in pipelines)
	}

	// 13. Write outputs
	if result != nil {
		outErr := writeOutputs(cfg, result)
		if outErr != nil {
			logger.Err(outErr, "phase", "output")
			os.Exit(1)
		}
	}

	// 14. Summary (only in non-visual mode)
	if result != nil && !usingVisualUI {
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

	// Terminal-readable table only in pretty mode
	if cfg.Output.UIMode == "pretty" || cfg.Output.UIMode == "" {
		if err := output.OutputTable(result); err != nil {
			return fmt.Errorf("table output: %w", err)
		}
	}

	return nil
}

// rootContextWithSignals creates a root context with optional timeout and signal cancellation.
// Returns:
//   - context: root context for the entire application
//   - cancelFunc: cleanup function that cleans up all resources
//   - sigintChannel: channel that receives SIGINT notifications for per-stage cancellation
//
// Signal handling behavior:
//   - First Ctrl-C: sends notification to sigintChannel (cancels current stage)
//   - Second Ctrl-C (within 2s): cancels entire context (exits application)
//   - SIGTERM: immediately cancels entire context
func rootContextWithSignals(timeoutSeconds int) (context.Context, context.CancelFunc, chan struct{}) {
	var base context.Context
	var baseCancel context.CancelFunc

	if timeoutSeconds > 0 {
		base, baseCancel = context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	} else {
		base, baseCancel = context.WithCancel(context.Background())
	}

	// System signal channel (buffered to avoid blocking)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Stage cancellation channel (used by orchestrator)
	sigintChannel := make(chan struct{}, 1)

	// Track last SIGINT time for double-Ctrl-C detection
	var lastSigintTime time.Time
	const doubleCtrlCWindow = 2 * time.Second

	// Goroutine waiting for signals
	go func() {
		defer func() {
			signal.Stop(sigCh)
			close(sigCh)
			close(sigintChannel)
		}()

		for {
			select {
			case sig, ok := <-sigCh:
				if !ok {
					return
				}

				// SIGTERM: immediate exit
				if sig == syscall.SIGTERM {
					fmt.Fprintf(os.Stderr, "\nReceived SIGTERM, shutting down...\n")
					baseCancel()
					return
				}

				// SIGINT: check if it's a double Ctrl-C
				now := time.Now()
				if !lastSigintTime.IsZero() && now.Sub(lastSigintTime) < doubleCtrlCWindow {
					// Second Ctrl-C within window: force exit
					fmt.Fprintf(os.Stderr, "\nReceived second Ctrl-C, forcing shutdown...\n")
					baseCancel()
					return
				}

				// First Ctrl-C: notify stage cancellation
				lastSigintTime = now
				fmt.Fprintf(os.Stderr, "\nReceived Ctrl-C, cancelling current stage... (press Ctrl-C again within 2s to force exit)\n")

				// Non-blocking send to sigintChannel
				select {
				case sigintChannel <- struct{}{}:
				default:
					// Channel full, skip (stage is already being cancelled)
				}

			case <-base.Done():
				// Context canceled by timeout or other reason
				return
			}
		}
	}()

	// Cleanup function that cleans up EVERYTHING
	cleanupCancel := func() {
		signal.Stop(sigCh)
		baseCancel()
	}

	return base, cleanupCancel, sigintChannel
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
