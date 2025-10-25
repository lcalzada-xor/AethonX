// Package main implements the AethonX dependency installer CLI.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aethonx/cmd/install-deps/installer"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/ui"

	"github.com/spf13/pflag"
)

const (
	version = "1.0.0"
	appName = "AethonX Dependency Installer"
)

// Config holds CLI configuration.
type Config struct {
	ConfigPath  string
	InstallDir  string
	CheckOnly   bool
	Force       bool
	Quiet       bool
	SkipGo      bool
	SkipExternal bool
	ShowVersion bool
}

func main() {
	// Parse flags
	cfg := parseFlags()

	// Show version and exit
	if cfg.ShowVersion {
		fmt.Printf("%s v%s\n", appName, version)
		os.Exit(0)
	}

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\nReceived interrupt signal. Cleaning up...")
		cancel()
	}()

	// Initialize logger
	logger := logx.NewWithLevel(logx.LevelInfo)

	// Initialize presenter
	var presenter ui.Presenter
	if cfg.Quiet {
		presenter = ui.NewRawPresenter(ui.LogFormatText)
	} else {
		presenter = ui.NewCustomPresenter()
	}
	defer presenter.Close()

	// Run installer
	if err := run(ctx, cfg, logger, presenter); err != nil {
		logger.Err(err, "installation failed")
		os.Exit(1)
	}
}

// parseFlags parses command-line flags.
func parseFlags() Config {
	var cfg Config

	pflag.StringVar(&cfg.ConfigPath, "config", "deps.yaml", "Path to dependencies configuration file")
	pflag.StringVar(&cfg.InstallDir, "dir", "", "Installation directory (overrides config)")
	pflag.BoolVar(&cfg.CheckOnly, "check", false, "Only check dependencies, do not install")
	pflag.BoolVar(&cfg.Force, "force", false, "Force reinstall even if already installed")
	pflag.BoolVarP(&cfg.Quiet, "quiet", "q", false, "Quiet mode (no UI, minimal output)")
	pflag.BoolVar(&cfg.SkipGo, "skip-go", false, "Skip Go module dependencies")
	pflag.BoolVar(&cfg.SkipExternal, "skip-external", false, "Skip external tool dependencies")
	pflag.BoolVarP(&cfg.ShowVersion, "version", "v", false, "Show version and exit")

	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s v%s\n\n", appName, version)
		fmt.Fprintf(os.Stderr, "USAGE:\n")
		fmt.Fprintf(os.Stderr, "  install-deps [flags]\n\n")
		fmt.Fprintf(os.Stderr, "FLAGS:\n")
		pflag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEXAMPLES:\n")
		fmt.Fprintf(os.Stderr, "  # Install all dependencies\n")
		fmt.Fprintf(os.Stderr, "  install-deps\n\n")
		fmt.Fprintf(os.Stderr, "  # Check dependencies only\n")
		fmt.Fprintf(os.Stderr, "  install-deps --check\n\n")
		fmt.Fprintf(os.Stderr, "  # Force reinstall to custom directory\n")
		fmt.Fprintf(os.Stderr, "  install-deps --force --dir /usr/local/bin\n\n")
	}

	pflag.Parse()

	return cfg
}

// run executes the main installation logic.
func run(ctx context.Context, cfg Config, logger logx.Logger, presenter ui.Presenter) error {
	startTime := time.Now()

	// Start scan
	presenter.Start(ui.ScanInfo{
		Target:         "AethonX Dependencies",
		Mode:           "Installation",
		Workers:        1,
		TimeoutSeconds: 300,
		TotalStages:    1,
		UIMode:         ui.UIModePretty,
	})

	// Initialize orchestrator
	logger.Info("initializing dependency installer", "config", cfg.ConfigPath)
	orch, err := installer.NewOrchestrator(cfg.ConfigPath, cfg.InstallDir)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	if err := orch.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize orchestrator: %w", err)
	}

	// Start stage
	presenter.StartStage(ui.StageInfo{
		Number:      1,
		TotalStages: 1,
		Name:        "Dependency Check & Installation",
		Sources:     []string{},
	})

	// Check mode
	if cfg.CheckOnly {
		logger.Info("checking dependencies (check-only mode)")
		presenter.Info("Running in check-only mode")

		results, err := orch.Check(ctx)
		if err != nil {
			return fmt.Errorf("dependency check failed: %w", err)
		}

		displayResults(results, presenter)
		presenter.FinishStage(1, time.Since(startTime))

		// Check PATH
		inPath, _ := orch.CheckPath()
		if !inPath {
			presenter.Warning(orch.GetPathWarning())
		}

		presenter.Finish(ui.ScanStats{
			TotalDuration: time.Since(startTime),
		})

		return nil
	}

	// Install mode
	logger.Info("installing dependencies")
	presenter.Info("Installing dependencies...")

	results, err := orch.Install(ctx, cfg.Force)
	if err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	displayResults(results, presenter)
	presenter.FinishStage(1, time.Since(startTime))

	// Check PATH
	inPath, _ := orch.CheckPath()
	if !inPath {
		presenter.Warning(orch.GetPathWarning())
	}

	// Final statistics
	stats := calculateStats(results)
	presenter.Finish(ui.ScanStats{
		TotalDuration:    time.Since(startTime),
		TotalArtifacts:   stats.Total,
		UniqueArtifacts:  stats.Success,
		SourcesSucceeded: stats.Success,
		SourcesFailed:    stats.Failed,
	})

	logger.Info("installation completed",
		"duration", time.Since(startTime),
		"success", stats.Success,
		"failed", stats.Failed,
		"skipped", stats.Skipped,
	)

	// Exit with error if any installation failed
	if stats.Failed > 0 {
		return fmt.Errorf("%d dependencies failed to install", stats.Failed)
	}

	return nil
}

// displayResults shows installation results using the presenter.
func displayResults(results []installer.InstallationResult, presenter ui.Presenter) {
	for _, result := range results {
		switch result.Status {
		case installer.StatusSuccess:
			presenter.FinishSource(
				result.Dependency.Name,
				ui.StatusSuccess,
				result.Duration,
				1,
			)
			presenter.Info(fmt.Sprintf("✓ %s: %s", result.Dependency.Name, result.Message))

		case installer.StatusAlreadyInstalled:
			presenter.FinishSource(
				result.Dependency.Name,
				ui.StatusSuccess,
				result.Duration,
				1,
			)
			presenter.Info(fmt.Sprintf("✓ %s: %s", result.Dependency.Name, result.Message))

		case installer.StatusFailed:
			presenter.FinishSource(
				result.Dependency.Name,
				ui.StatusError,
				result.Duration,
				0,
			)
			presenter.Error(fmt.Sprintf("✗ %s: %s", result.Dependency.Name, result.Message))

		case installer.StatusSkipped:
			presenter.FinishSource(
				result.Dependency.Name,
				ui.StatusSkipped,
				result.Duration,
				0,
			)
			presenter.Info(fmt.Sprintf("⊘ %s: %s", result.Dependency.Name, result.Message))
		}
	}
}

// Stats holds installation statistics.
type Stats struct {
	Total   int
	Success int
	Failed  int
	Skipped int
}

// calculateStats computes statistics from installation results.
func calculateStats(results []installer.InstallationResult) Stats {
	stats := Stats{Total: len(results)}

	for _, result := range results {
		switch result.Status {
		case installer.StatusSuccess, installer.StatusAlreadyInstalled:
			stats.Success++
		case installer.StatusFailed:
			stats.Failed++
		case installer.StatusSkipped:
			stats.Skipped++
		}
	}

	return stats
}
