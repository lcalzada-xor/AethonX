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

	"github.com/spf13/pflag"
)

const (
	version = "1.0.0"
	appName = "AethonX Dependency Installer"
)

// Config holds CLI configuration.
type Config struct {
	ConfigPath   string
	InstallDir   string
	CheckOnly    bool
	Force        bool
	Quiet        bool
	Verbose      bool
	SkipGo       bool
	SkipExternal bool
	ShowVersion  bool
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
	logLevel := logx.LevelInfo
	if cfg.Verbose {
		logLevel = logx.LevelDebug
	}
	logger := logx.NewWithLevel(logLevel)

	// Run installer
	if err := run(ctx, cfg, logger); err != nil {
		if !cfg.Quiet {
			fmt.Fprintf(os.Stderr, "\n❌ Installation failed: %v\n\n", err)
		}
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
	pflag.BoolVar(&cfg.Verbose, "verbose", false, "Verbose mode (detailed logging)")
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
func run(ctx context.Context, cfg Config, logger logx.Logger) error {
	startTime := time.Now()

	// Initialize presenter
	presenter := installer.NewSimplePresenter(cfg.Quiet)

	// Show header
	presenter.ShowHeader()

	// Initialize orchestrator
	logger.Debug("initializing dependency installer", "config", cfg.ConfigPath)
	orch, err := installer.NewOrchestrator(cfg.ConfigPath, cfg.InstallDir)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	if err := orch.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize orchestrator: %w", err)
	}

	// Set progress callback for real-time updates
	orch.SetProgressCallback(func(toolName string, phase installer.InstallationPhase, message string) {
		if cfg.Verbose {
			logger.Debug("installation progress", "tool", toolName, "phase", phase, "message", message)
		}
		presenter.ShowProgress(toolName, phase, message)
	})

	// Check mode
	if cfg.CheckOnly {
		return runCheckMode(ctx, orch, presenter, logger)
	}

	// Install mode
	return runInstallMode(ctx, cfg, orch, presenter, logger, startTime)
}

// runCheckMode executes dependency check-only mode.
func runCheckMode(ctx context.Context, orch *installer.Orchestrator, presenter *installer.SimplePresenter, logger logx.Logger) error {
	logger.Debug("checking dependencies (check-only mode)")

	results, err := orch.Check(ctx)
	if err != nil {
		return fmt.Errorf("dependency check failed: %w", err)
	}

	// Display check results
	presenter.ShowCheckResults(results)

	return nil
}

// runInstallMode executes dependency installation mode.
func runInstallMode(ctx context.Context, cfg Config, orch *installer.Orchestrator, presenter *installer.SimplePresenter, logger logx.Logger, startTime time.Time) error {
	logger.Debug("installing dependencies")

	// Pre-installation check
	preResults, err := orch.Check(ctx)
	if err != nil {
		return fmt.Errorf("pre-installation check failed: %w", err)
	}

	// Show pre-installation summary
	presenter.ShowPreCheck(preResults, cfg.Force)

	// Count what needs installation
	toInstall := 0
	for _, result := range preResults {
		if cfg.Force || result.Status != installer.StatusAlreadyInstalled {
			toInstall++
		}
	}

	// Start installation
	presenter.StartInstallation(toInstall)

	// Install
	results, err := orch.Install(ctx, cfg.Force)
	if err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	// Show individual results
	fmt.Println()
	for _, result := range results {
		presenter.ShowResult(result)
	}

	// Check PATH
	pathWarning := ""
	inPath, _ := orch.CheckPath()
	if !inPath {
		pathWarning = orch.GetPathWarning()
	}

	// Display final summary
	presenter.ShowSummary(results, time.Since(startTime), pathWarning)

	// Calculate stats for logging
	stats := calculateStats(results)
	logger.Debug("installation completed",
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
