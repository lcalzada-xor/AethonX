// Package waybackurls implements integration with waybackurls CLI tool.
// It executes waybackurls as a subprocess and parses its output to create artifacts.
package waybackurls

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
)

const (
	sourceName     = "waybackurls"
	defaultTimeout = 120 * time.Second // Wayback Machine can be slow
)

// WaybackurlsSource implements ports.Source and ports.AdvancedSource.
// It wraps waybackurls CLI tool for historical URL discovery.
type WaybackurlsSource struct {
	logger     logx.Logger
	execPath   string        // Path to waybackurls binary
	timeout    time.Duration
	withDates  bool // -dates flag
	noSubs     bool // -no-subs flag
	parser     *Parser
	progressCh chan ports.ProgressUpdate

	// Process management
	mu  sync.Mutex
	cmd *exec.Cmd
}

// New creates a new WaybackurlsSource with default configuration.
func New(logger logx.Logger) *WaybackurlsSource {
	return &WaybackurlsSource{
		logger:     logger.With("source", sourceName),
		execPath:   "waybackurls",
		timeout:    defaultTimeout,
		withDates:  false, // Don't need dates by default
		noSubs:     false, // Include subdomains
		parser:     NewParser(logger, sourceName),
		progressCh: make(chan ports.ProgressUpdate, 100),
	}
}

// NewWithConfig creates WaybackurlsSource with custom configuration.
func NewWithConfig(logger logx.Logger, execPath string, timeout time.Duration, withDates, noSubs bool) *WaybackurlsSource {
	return &WaybackurlsSource{
		logger:     logger.With("source", sourceName),
		execPath:   execPath,
		timeout:    timeout,
		withDates:  withDates,
		noSubs:     noSubs,
		parser:     NewParser(logger, sourceName),
		progressCh: make(chan ports.ProgressUpdate, 100),
	}
}

// Name returns the source name.
func (w *WaybackurlsSource) Name() string {
	return sourceName
}

// Mode returns the source operation mode (passive).
func (w *WaybackurlsSource) Mode() domain.SourceMode {
	return domain.SourceModePassive
}

// Type returns the source type (CLI).
func (w *WaybackurlsSource) Type() domain.SourceType {
	return domain.SourceTypeCLI
}

// Run executes waybackurls against the target domain.
func (w *WaybackurlsSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	result := domain.NewScanResult(target)
	startTime := time.Now()

	w.logger.Info("starting waybackurls scan",
		"target", target.Root,
		"with_dates", w.withDates,
		"no_subs", w.noSubs,
		"timeout", w.timeout.String(),
	)

	// Build command with context
	cmd := w.buildCommand(ctx, target)

	// Create stdin pipe to feed domain
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Create stdout pipe for streaming URLs
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Create stderr pipe for warnings
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Store command reference for Close()
	w.mu.Lock()
	w.cmd = cmd
	w.mu.Unlock()

	// Start waybackurls process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start waybackurls: %w", err)
	}

	w.logger.Debug("waybackurls process started", "pid", cmd.Process.Pid)

	// Write target domain to stdin
	go func() {
		defer stdin.Close()
		fmt.Fprintln(stdin, target.Root)
	}()

	// Read stderr in background to prevent blocking
	var stderrBytes []byte
	var stderrMu sync.Mutex
	var stderrWg sync.WaitGroup
	stderrWg.Add(1)

	go func() {
		defer stderrWg.Done()
		data, err := io.ReadAll(stderr)
		if err != nil {
			w.logger.Warn("error reading stderr", "error", err.Error())
		}
		stderrMu.Lock()
		stderrBytes = data
		stderrMu.Unlock()
	}()

	// Parse stdout in real-time (streaming URLs)
	scanner := bufio.NewScanner(stdout)

	// Increase buffer size for long URLs
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // 10MB max token size

	// Deduplication map
	seen := make(map[string]bool) // key: "type:value"
	artifactCount := 0
	urlCount := 0
	lastProgress := time.Now()

	for scanner.Scan() {
		line := scanner.Text()
		urlCount++

		// Parse line and extract artifacts
		artifacts := w.parser.ParseLine(line, target)

		for _, artifact := range artifacts {
			// Deduplicate on the fly
			key := string(artifact.Type) + ":" + artifact.Value
			if !seen[key] {
				seen[key] = true
				result.AddArtifact(artifact)
				artifactCount++
			}
		}

		// Emit progress every 100 URLs or every 2 seconds
		if urlCount%100 == 0 || time.Since(lastProgress) > 2*time.Second {
			select {
			case w.progressCh <- ports.ProgressUpdate{
				ArtifactCount: artifactCount,
				Message:       fmt.Sprintf("Processed %d URLs, found %d artifacts", urlCount, artifactCount),
			}:
				lastProgress = time.Now()
			default:
				// Channel full, skip update
			}
		}
	}

	if err := scanner.Err(); err != nil {
		w.logger.Warn("scanner error", "error", err.Error())
	}

	// Wait for process to complete
	if err := cmd.Wait(); err != nil {
		// Wait for stderr goroutine to finish before returning error
		stderrWg.Wait()

		// Don't fail if we got some results
		if artifactCount > 0 {
			w.logger.Warn("waybackurls exited with error but produced results", "error", err.Error())
			result.AddWarning("waybackurls", fmt.Sprintf("process exited with error: %v", err))
		} else {
			return nil, fmt.Errorf("waybackurls failed: %w", err)
		}
	}

	// Wait for stderr goroutine to finish reading all output
	stderrWg.Wait()

	// Get stderr from background goroutine
	stderrMu.Lock()
	stderrLen := len(stderrBytes)
	stderrStr := string(stderrBytes)
	stderrMu.Unlock()

	if stderrLen > 0 {
		w.logger.Debug("waybackurls stderr", "output", stderrStr)
		result.AddWarning("waybackurls", fmt.Sprintf("stderr output: %s", stderrStr))
	}

	// Log warnings if no results
	if urlCount == 0 {
		w.logger.Warn("waybackurls completed but found 0 URLs",
			"target", target.Root,
		)
		result.AddWarning("waybackurls", "scan completed but no URLs were found - target may not be archived in Wayback Machine")
	}

	// Log final statistics
	duration := time.Since(startTime)
	w.logger.Info("waybackurls scan completed",
		"target", target.Root,
		"duration", duration.String(),
		"urls_processed", urlCount,
		"artifacts", len(result.Artifacts),
	)

	// Log artifact breakdown by type
	typeCount := make(map[domain.ArtifactType]int)
	for _, artifact := range result.Artifacts {
		typeCount[artifact.Type]++
	}
	w.logger.Debug("artifact breakdown", "counts", typeCount)

	return result, nil
}

// ProgressChannel implements ports.StreamingSource
func (w *WaybackurlsSource) ProgressChannel() <-chan ports.ProgressUpdate {
	return w.progressCh
}

// Stream implements ports.StreamingSource
func (w *WaybackurlsSource) Stream(ctx context.Context, target domain.Target) (<-chan *domain.Artifact, <-chan error) {
	artifactCh := make(chan *domain.Artifact, 100)
	errorCh := make(chan error, 1)

	go func() {
		defer close(artifactCh)
		defer close(errorCh)

		result, err := w.Run(ctx, target)
		if err != nil {
			errorCh <- err
			return
		}

		for _, artifact := range result.Artifacts {
			select {
			case artifactCh <- artifact:
			case <-ctx.Done():
				return
			}
		}
	}()

	return artifactCh, errorCh
}

// Close terminates the waybackurls process and cleans up resources.
func (w *WaybackurlsSource) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.logger.Debug("closing waybackurls source")

	// Kill process if still running
	if w.cmd != nil && w.cmd.Process != nil {
		// Check if process is still running
		if w.cmd.ProcessState == nil || !w.cmd.ProcessState.Exited() {
			// Try SIGTERM first
			if err := w.cmd.Process.Signal(os.Interrupt); err != nil {
				// Check if process already finished (not a real error)
				if err != os.ErrProcessDone {
					w.logger.Warn("SIGTERM failed, forcing kill", "error", err.Error())
					if killErr := w.cmd.Process.Kill(); killErr != nil && killErr != os.ErrProcessDone {
						w.logger.Warn("failed to kill waybackurls process", "error", killErr.Error())
					}
				}
			}
		}

		w.cmd = nil
	}

	w.logger.Debug("waybackurls source closed")
	return nil
}

// Initialize verifies that waybackurls is installed and accessible.
// Implements ports.AdvancedSource.
func (w *WaybackurlsSource) Initialize() error {
	w.logger.Debug("initializing waybackurls source", "exec_path", w.execPath)

	// Check if waybackurls binary exists
	execPath, err := exec.LookPath(w.execPath)
	if err != nil {
		return fmt.Errorf("waybackurls not found in PATH: %w (install: go install github.com/tomnomnom/waybackurls@latest)", err)
	}

	w.execPath = execPath
	w.logger.Debug("found waybackurls binary", "path", execPath)

	// waybackurls doesn't have a -version flag, just check if it exists
	w.logger.Info("waybackurls initialized successfully", "path", execPath)

	return nil
}

// Validate checks if the source configuration is valid.
// Implements ports.AdvancedSource.
func (w *WaybackurlsSource) Validate() error {
	if w.execPath == "" {
		return fmt.Errorf("waybackurls exec path is empty")
	}

	if w.timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	return nil
}

// HealthCheck verifies that waybackurls is responsive.
// Implements ports.AdvancedSource.
func (w *WaybackurlsSource) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Just check if binary exists and is executable
	if _, err := exec.LookPath(w.execPath); err != nil {
		return fmt.Errorf("waybackurls health check failed: %w", err)
	}

	return nil
}

// buildCommand constructs the waybackurls command with appropriate flags.
func (w *WaybackurlsSource) buildCommand(ctx context.Context, target domain.Target) *exec.Cmd {
	args := []string{}

	// Add optional flags
	if w.withDates {
		args = append(args, "-dates")
	}

	if w.noSubs {
		args = append(args, "-no-subs")
	}

	// Use parent context directly
	cmd := exec.CommandContext(ctx, w.execPath, args...)

	w.logger.Debug("built waybackurls command",
		"args", args,
		"timeout", w.timeout.String(),
	)

	return cmd
}
