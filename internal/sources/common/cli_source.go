// Package common provides shared abstractions for source implementations.
package common

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

// OutputHandler processes output from CLI tools.
// Implementations define how to parse and handle stdout/stderr from subprocess.
type OutputHandler interface {
	// ProcessLine handles each line of stdout in real-time.
	// Return error to stop processing (non-fatal errors should be logged instead).
	ProcessLine(line []byte) error

	// Finalize is called after all lines are processed.
	// Use this for cleanup, validation, or final artifact creation.
	Finalize() error
}

// BaseCLISource provides common functionality for CLI-based reconnaissance sources.
// It handles subprocess execution, I/O management, signal handling, and resource cleanup.
//
// Usage:
//   1. Embed BaseCLISource in your source struct
//   2. Call Initialize() with your config
//   3. Implement OutputHandler for parsing logic
//   4. Call ExecuteCLI() in your Run() method
type BaseCLISource struct {
	logger     logx.Logger
	execPath   string        // Path to CLI binary
	timeout    time.Duration // Timeout for subprocess
	progressCh chan ports.ProgressUpdate
	chClosed   bool          // Track if progressCh is closed

	// Process management
	mu  sync.Mutex
	cmd *exec.Cmd
}

// BaseCLIConfig contains configuration for BaseCLISource.
type BaseCLIConfig struct {
	SourceName     string        // Source name for logging
	ExecPath       string        // Path to binary (will be resolved via LookPath)
	Timeout        time.Duration // Subprocess timeout
	ProgressBuffer int           // Progress channel buffer size (default: 10)
}

// NewBaseCLISource creates a new BaseCLISource with the given configuration.
func NewBaseCLISource(logger logx.Logger, cfg BaseCLIConfig) *BaseCLISource {
	if cfg.ProgressBuffer <= 0 {
		cfg.ProgressBuffer = 10
	}

	return &BaseCLISource{
		logger:     logger.With("source", cfg.SourceName),
		execPath:   cfg.ExecPath,
		timeout:    cfg.Timeout,
		progressCh: make(chan ports.ProgressUpdate, cfg.ProgressBuffer),
	}
}

// ExecuteCLI executes a CLI command with the given arguments and processes output via handler.
//
// Key features:
//   - Automatic stdout/stderr pipe management
//   - Background stderr reader (prevents blocking)
//   - Context cancellation support
//   - Graceful error handling (tolerates partial results)
//   - Resource cleanup via defer
//   - Thread-safe process tracking
//
// Returns:
//   - result: ScanResult populated by handler
//   - stderrOutput: Captured stderr for warnings/debugging
//   - err: Fatal error (nil if partial results tolerated)
func (b *BaseCLISource) ExecuteCLI(
	ctx context.Context,
	target domain.Target,
	args []string,
	handler OutputHandler,
) (result *domain.ScanResult, stderrOutput string, err error) {
	result = domain.NewScanResult(target)
	startTime := time.Now()

	b.logger.Info("executing CLI command",
		"exec_path", b.execPath,
		"args", args,
		"timeout", b.timeout.String(),
	)

	// Build command with context
	cmd := exec.CommandContext(ctx, b.execPath, args...)

	// Create stdout pipe for streaming output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Create stderr pipe for warnings
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Store command reference for Close()
	b.mu.Lock()
	b.cmd = cmd
	b.mu.Unlock()

	// Start subprocess
	if err := cmd.Start(); err != nil {
		return nil, "", fmt.Errorf("failed to start process: %w", err)
	}

	b.logger.Debug("subprocess started", "pid", cmd.Process.Pid)

	// Read stderr in background to prevent blocking
	var stderrBytes []byte
	var stderrMu sync.Mutex
	var stderrWg sync.WaitGroup
	stderrWg.Add(1)

	go func() {
		defer stderrWg.Done()
		data, readErr := io.ReadAll(stderr)
		if readErr != nil {
			b.logger.Warn("error reading stderr", "error", readErr.Error())
		}
		stderrMu.Lock()
		stderrBytes = data
		stderrMu.Unlock()
	}()

	// Process stdout line by line
	scanner := bufio.NewScanner(stdout)

	// Increase buffer size for large output lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // 10MB max token size

	for scanner.Scan() {
		line := scanner.Bytes()

		// Call handler to process line
		if err := handler.ProcessLine(line); err != nil {
			b.logger.Warn("handler error", "error", err.Error())
			// Continue processing despite handler errors
		}
	}

	if err := scanner.Err(); err != nil {
		b.logger.Warn("scanner error", "error", err.Error())
	}

	// Finalize handler (e.g., flush buffers, create artifacts)
	if err := handler.Finalize(); err != nil {
		b.logger.Warn("handler finalization error", "error", err.Error())
	}

	// Wait for process to complete
	waitErr := cmd.Wait()

	// Wait for stderr goroutine to finish reading all output
	stderrWg.Wait()

	// Get stderr from background goroutine
	stderrMu.Lock()
	stderrOutput = string(stderrBytes)
	stderrMu.Unlock()

	if len(stderrOutput) > 0 {
		b.logger.Debug("subprocess stderr", "output", stderrOutput)
	}

	// Handle process exit errors
	if waitErr != nil {
		// Log execution time even on failure
		duration := time.Since(startTime)
		b.logger.Warn("subprocess exited with error",
			"error", waitErr.Error(),
			"duration", duration.String(),
		)

		// Return error as non-fatal (caller decides based on partial results)
		return result, stderrOutput, fmt.Errorf("process exited with error: %w", waitErr)
	}

	duration := time.Since(startTime)
	b.logger.Info("CLI command completed successfully",
		"duration", duration.String(),
	)

	return result, stderrOutput, nil
}

// EmitProgress sends a progress update (non-blocking).
func (b *BaseCLISource) EmitProgress(artifactCount int, message string) {
	select {
	case b.progressCh <- ports.ProgressUpdate{
		ArtifactCount: artifactCount,
		Message:       message,
	}:
	default:
		// Channel full, skip update
	}
}

// ProgressChannel returns the progress channel for streaming updates.
// Implements ports.StreamingSource.
func (b *BaseCLISource) ProgressChannel() <-chan ports.ProgressUpdate {
	return b.progressCh
}

// DefaultStream provides a default Stream implementation that wraps Run().
// Implements ports.StreamingSource by delegating to Run() and emitting artifacts.
func (b *BaseCLISource) DefaultStream(
	ctx context.Context,
	target domain.Target,
	runFunc func(ctx context.Context, target domain.Target) (*domain.ScanResult, error),
) (<-chan *domain.Artifact, <-chan error) {
	artifactCh := make(chan *domain.Artifact, 100)
	errorCh := make(chan error, 1)

	go func() {
		defer close(artifactCh)
		defer close(errorCh)

		result, err := runFunc(ctx, target)
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

// Close terminates the subprocess and cleans up resources.
// Implements ports.Source.Close() for all CLI-based sources.
// Safe to call multiple times (idempotent).
func (b *BaseCLISource) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.logger.Debug("closing CLI source")

	// Close progress channel to prevent goroutine leaks (only once)
	if !b.chClosed {
		close(b.progressCh)
		b.chClosed = true
	}

	// Kill process if still running
	// Note: We hold the mutex during the entire operation to prevent races
	if b.cmd != nil && b.cmd.Process != nil {
		proc := b.cmd.Process       // Copy reference to avoid race
		state := b.cmd.ProcessState // Copy state reference

		// Check if process is still running
		if state == nil || !state.Exited() {
			// Try SIGTERM first (graceful shutdown)
			if err := proc.Signal(os.Interrupt); err != nil {
				// Check if process already finished (not a real error)
				if err != os.ErrProcessDone {
					b.logger.Warn("SIGTERM failed, forcing kill", "error", err.Error())
					if killErr := proc.Kill(); killErr != nil && killErr != os.ErrProcessDone {
						b.logger.Warn("failed to kill process", "error", killErr.Error())
					}
				}
			}
		}

		b.cmd = nil
	}

	b.logger.Debug("CLI source closed")
	return nil
}

// DefaultInitialize provides a default Initialize implementation for AdvancedSource.
// Verifies that the CLI binary exists and is executable.
func (b *BaseCLISource) DefaultInitialize(sourceName, installInstructions string) error {
	b.logger.Debug("initializing CLI source", "exec_path", b.execPath)

	// Check if binary exists in PATH
	execPath, err := exec.LookPath(b.execPath)
	if err != nil {
		return fmt.Errorf("%s not found in PATH: %w (install: %s)", sourceName, err, installInstructions)
	}

	b.execPath = execPath
	b.logger.Debug("found binary", "path", execPath)

	// Try to get version (optional, best-effort)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, b.execPath, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Version check is optional, don't fail initialization
		b.logger.Debug("version check failed (non-fatal)", "error", err.Error())
	} else {
		version := string(output)
		b.logger.Info("CLI source initialized successfully", "version", version)
	}

	return nil
}

// DefaultValidate provides a default Validate implementation for AdvancedSource.
func (b *BaseCLISource) DefaultValidate() error {
	if b.execPath == "" {
		return fmt.Errorf("exec path is empty")
	}

	if b.timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	return nil
}

// DefaultHealthCheck provides a default HealthCheck implementation for AdvancedSource.
func (b *BaseCLISource) DefaultHealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Try running with -version or -h flag
	cmd := exec.CommandContext(ctx, b.execPath, "-version")
	if err := cmd.Run(); err != nil {
		// Try -h as fallback
		cmd = exec.CommandContext(ctx, b.execPath, "-h")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}
	}

	return nil
}

// GetExecPath returns the resolved executable path.
func (b *BaseCLISource) GetExecPath() string {
	return b.execPath
}

// GetTimeout returns the configured timeout.
func (b *BaseCLISource) GetTimeout() time.Duration {
	return b.timeout
}

// SetTimeout updates the timeout dynamically (useful for profile switching).
func (b *BaseCLISource) SetTimeout(timeout time.Duration) {
	b.timeout = timeout
}

// GetLogger returns the logger instance.
func (b *BaseCLISource) GetLogger() logx.Logger {
	return b.logger
}

// ProcessOutput processes stdout using the given handler.
// This is useful for sources that need direct control over subprocess execution
// (e.g., RunWithInput using stdin).
func (b *BaseCLISource) ProcessOutput(stdout io.Reader, handler OutputHandler) error {
	scanner := bufio.NewScanner(stdout)

	// Increase buffer size for large lines (1MB max token size)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // 10MB max token size

	for scanner.Scan() {
		line := scanner.Bytes()
		if err := handler.ProcessLine(line); err != nil {
			// Stop processing on error from handler
			b.logger.Debug("handler signaled stop", "error", err.Error())
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		b.logger.Warn("scanner error", "error", err.Error())
		return err
	}

	return nil
}
