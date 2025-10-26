// Package amass implements integration with OWASP Amass CLI tool.
// It executes amass as a subprocess and parses its JSON output to create artifacts.
package amass

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
)

const (
	sourceName     = "amass"
	defaultTimeout = 300 * time.Second // 5 minutes for amass
)

// AmassSource implements ports.Source and ports.AdvancedSource.
// It wraps OWASP Amass CLI tool for subdomain enumeration and network mapping.
type AmassSource struct {
	logger     logx.Logger
	execPath   string        // Path to amass binary
	timeout    time.Duration
	activeMode bool          // Enable --active flag
	maxDNSQPS  int          // DNS queries per second (0 = unlimited)
	brute      bool         // Enable brute force
	alts       bool         // Enable alterations
	parser     *Parser
	progressCh chan ports.ProgressUpdate

	// Process management
	mu  sync.Mutex
	cmd *exec.Cmd
}

// AmassConfig contiene la configuración para AmassSource.
type AmassConfig struct {
	ExecPath   string
	Timeout    time.Duration
	ActiveMode bool
	MaxDNSQPS  int
	Brute      bool
	Alts       bool
}

// New creates a new AmassSource with default configuration.
func New(logger logx.Logger) *AmassSource {
	return &AmassSource{
		logger:     logger.With("source", sourceName),
		execPath:   "amass",
		timeout:    defaultTimeout,
		activeMode: false,
		maxDNSQPS:  0,
		brute:      false,
		alts:       false,
		parser:     NewParser(logger, sourceName),
		progressCh: make(chan ports.ProgressUpdate, 10),
	}
}

// NewWithConfig creates AmassSource with custom configuration.
func NewWithConfig(logger logx.Logger, cfg AmassConfig) *AmassSource {
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.ExecPath == "" {
		cfg.ExecPath = "amass"
	}

	return &AmassSource{
		logger:     logger.With("source", sourceName),
		execPath:   cfg.ExecPath,
		timeout:    cfg.Timeout,
		activeMode: cfg.ActiveMode,
		maxDNSQPS:  cfg.MaxDNSQPS,
		brute:      cfg.Brute,
		alts:       cfg.Alts,
		parser:     NewParser(logger, sourceName),
		progressCh: make(chan ports.ProgressUpdate, 10),
	}
}

// Name returns the source name.
func (a *AmassSource) Name() string {
	return sourceName
}

// Mode returns the source operation mode (both/hybrid).
// Amass can operate in passive or active mode depending on configuration.
func (a *AmassSource) Mode() domain.SourceMode {
	return domain.SourceModeBoth
}

// Type returns the source type (CLI).
func (a *AmassSource) Type() domain.SourceType {
	return domain.SourceTypeCLI
}

// Run executes amass enum against the target domain.
func (a *AmassSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	result := domain.NewScanResult(target)
	startTime := time.Now()

	a.logger.Info("starting amass scan",
		"target", target.Root,
		"active", a.activeMode,
		"brute", a.brute,
		"alts", a.alts,
		"max_dns_qps", a.maxDNSQPS,
	)

	// Build command with context
	cmd := a.buildCommand(ctx, target)

	// Create stdout pipe for streaming JSON
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
	a.mu.Lock()
	a.cmd = cmd
	a.mu.Unlock()

	// Start amass process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start amass: %w", err)
	}

	a.logger.Debug("amass process started", "pid", cmd.Process.Pid)

	// Read stderr in background to prevent blocking
	var stderrBytes []byte
	var stderrMu sync.Mutex
	go func() {
		data, _ := io.ReadAll(stderr)
		stderrMu.Lock()
		stderrBytes = data
		stderrMu.Unlock()
	}()

	// Parse stdout in real-time (streaming JSONL)
	responses := make([]*AmassResponse, 0, 100)
	scanner := bufio.NewScanner(stdout)

	// Increase buffer size for large responses
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max token size

	artifactCount := 0
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		var resp AmassResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			a.logger.Warn("failed to parse amass output", "line", line, "error", err.Error())
			continue
		}

		responses = append(responses, &resp)
		artifactCount++

		// Emit progress (non-blocking)
		select {
		case a.progressCh <- ports.ProgressUpdate{
			ArtifactCount: artifactCount,
			Message:       fmt.Sprintf("Found %s", resp.Name),
		}:
		default:
			// Channel full, skip update
		}

		a.logger.Debug("parsed amass response",
			"name", resp.Name,
			"addresses", len(resp.Addresses),
			"tag", resp.Tag,
			"source", resp.Source,
		)
	}

	if err := scanner.Err(); err != nil {
		a.logger.Warn("scanner error", "error", err.Error())
	}

	// Get stderr from background goroutine
	stderrMu.Lock()
	stderrLen := len(stderrBytes)
	stderrStr := string(stderrBytes)
	stderrMu.Unlock()

	if stderrLen > 0 {
		a.logger.Debug("amass stderr", "output", stderrStr)
		result.AddWarning("amass", fmt.Sprintf("stderr output: %s", stderrStr))
	}

	// Wait for process to complete
	if err := cmd.Wait(); err != nil {
		// Don't fail if we got some results
		if len(responses) > 0 {
			a.logger.Warn("amass exited with error but produced results", "error", err.Error())
			result.AddWarning("amass", fmt.Sprintf("process exited with error: %v", err))
		} else {
			return nil, fmt.Errorf("amass failed: %w", err)
		}
	}

	// Parse responses into artifacts
	artifacts := a.parser.ParseMultipleResponses(responses, target)
	for _, artifact := range artifacts {
		result.AddArtifact(artifact)
	}

	duration := time.Since(startTime)
	a.logger.Info("amass scan completed",
		"target", target.Root,
		"duration", duration.String(),
		"responses", len(responses),
		"artifacts", len(result.Artifacts),
	)

	return result, nil
}

// ProgressChannel implements ports.StreamingSource
func (a *AmassSource) ProgressChannel() <-chan ports.ProgressUpdate {
	return a.progressCh
}

// Stream implements ports.StreamingSource (no usado actualmente pero requerido por interfaz)
func (a *AmassSource) Stream(ctx context.Context, target domain.Target) (<-chan *domain.Artifact, <-chan error) {
	artifactCh := make(chan *domain.Artifact, 100)
	errorCh := make(chan error, 1)

	go func() {
		defer close(artifactCh)
		defer close(errorCh)

		result, err := a.Run(ctx, target)
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

// Close terminates the amass process and cleans up resources.
func (a *AmassSource) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.logger.Debug("closing amass source")

	// Kill process if still running
	if a.cmd != nil && a.cmd.Process != nil {
		// Check if process is still running
		if a.cmd.ProcessState == nil || !a.cmd.ProcessState.Exited() {
			// Try SIGTERM first
			if err := a.cmd.Process.Signal(os.Interrupt); err != nil {
				// Only log if it's not "already finished"
				if err.Error() != "os: process already finished" {
					a.logger.Warn("SIGTERM failed, forcing kill", "error", err.Error())
					if killErr := a.cmd.Process.Kill(); killErr != nil && killErr.Error() != "os: process already finished" {
						a.logger.Warn("failed to kill amass process", "error", killErr.Error())
					}
				}
			}
		}

		a.cmd = nil
	}

	a.logger.Debug("amass source closed")
	return nil
}

// Initialize verifies that amass is installed and accessible.
// Implements ports.AdvancedSource.
func (a *AmassSource) Initialize() error {
	a.logger.Debug("initializing amass source", "exec_path", a.execPath)

	// Check if amass binary exists
	execPath, err := exec.LookPath(a.execPath)
	if err != nil {
		return fmt.Errorf("amass not found in PATH: %w (install from: https://github.com/owasp-amass/amass)", err)
	}

	a.execPath = execPath
	a.logger.Debug("found amass binary", "path", execPath)

	// Check version
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.execPath, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check amass version: %w", err)
	}

	version := string(output)
	a.logger.Info("amass initialized successfully", "version", version)

	return nil
}

// Validate checks if the source configuration is valid.
// Implements ports.AdvancedSource.
func (a *AmassSource) Validate() error {
	if a.execPath == "" {
		return fmt.Errorf("amass exec path is empty")
	}

	if a.timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if a.maxDNSQPS < 0 {
		return fmt.Errorf("max DNS QPS cannot be negative")
	}

	return nil
}

// HealthCheck verifies that amass is responsive.
// Implements ports.AdvancedSource.
func (a *AmassSource) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.execPath, "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("amass health check failed: %w", err)
	}

	return nil
}

// buildCommand constructs the amass command with appropriate flags.
func (a *AmassSource) buildCommand(ctx context.Context, target domain.Target) *exec.Cmd {
	args := []string{
		"enum",            // Use enum subcommand
		"-d", target.Root, // Target domain
		"-silent",         // No progress output
		"-nocolor",        // No color in output
	}

	// Active mode flag
	if a.activeMode {
		args = append(args, "-active")
	}

	// Brute force flag
	if a.brute {
		args = append(args, "-brute")
	}

	// Alterations flag
	if a.alts {
		args = append(args, "-alts")
	}

	// DNS rate limiting
	if a.maxDNSQPS > 0 {
		args = append(args, "-dns-qps", strconv.Itoa(a.maxDNSQPS))
	}

	// Timeout (in minutes)
	timeoutMinutes := int(a.timeout.Minutes())
	if timeoutMinutes <= 0 {
		timeoutMinutes = 1
	}
	args = append(args, "-timeout", strconv.Itoa(timeoutMinutes))

	// Use parent context directly
	cmd := exec.CommandContext(ctx, a.execPath, args...)

	a.logger.Debug("built amass command",
		"args", args,
		"timeout", a.timeout.String(),
	)

	return cmd
}
