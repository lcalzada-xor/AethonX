// Package httpx implements integration with Project Discovery's httpx CLI tool.
// It executes httpx as a subprocess and parses its JSON output to create artifacts.
package httpx

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
	"aethonx/internal/platform/logx"
)

const (
	sourceName     = "httpx"
	defaultTimeout = 120 * time.Second
	defaultThreads = 50
	defaultRateLimit = 150
)

// HTTPXSource implements ports.Source and ports.AdvancedSource.
// It wraps Project Discovery's httpx CLI tool for HTTP probing and fingerprinting.
type HTTPXSource struct {
	logger      logx.Logger
	execPath    string        // Path to httpx binary
	profile     ScanProfile   // Scan profile to use
	timeout     time.Duration
	threads     int
	rateLimit   int
	customFlags []string
	parser      *Parser

	// Process management
	mu     sync.Mutex
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

// New creates a new HTTPXSource with default configuration.
func New(logger logx.Logger) *HTTPXSource {
	return &HTTPXSource{
		logger:      logger.With("source", sourceName),
		execPath:    "httpx", // Default: search in PATH
		profile:     ProfileFull,
		timeout:     defaultTimeout,
		threads:     defaultThreads,
		rateLimit:   defaultRateLimit,
		customFlags: []string{},
		parser:      NewParser(logger, sourceName),
	}
}

// NewWithConfig creates HTTPXSource with custom configuration.
func NewWithConfig(logger logx.Logger, execPath string, profile ScanProfile, timeout time.Duration, threads, rateLimit int) *HTTPXSource {
	return &HTTPXSource{
		logger:      logger.With("source", sourceName),
		execPath:    execPath,
		profile:     profile,
		timeout:     timeout,
		threads:     threads,
		rateLimit:   rateLimit,
		customFlags: []string{},
		parser:      NewParser(logger, sourceName),
	}
}

// Name returns the source name.
func (h *HTTPXSource) Name() string {
	return sourceName
}

// Mode returns the source operation mode (active).
func (h *HTTPXSource) Mode() domain.SourceMode {
	return domain.SourceModeActive
}

// Type returns the source type (CLI).
func (h *HTTPXSource) Type() domain.SourceType {
	return domain.SourceTypeCLI
}

// Run executes httpx against the target domain.
func (h *HTTPXSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	result := domain.NewScanResult(target)
	startTime := time.Now()

	h.logger.Info("starting httpx scan",
		"target", target.Root,
		"profile", h.profile,
		"threads", h.threads,
		"rate_limit", h.rateLimit,
	)

	// Build command with context
	cmd := h.buildCommand(ctx, target)

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
	h.mu.Lock()
	h.cmd = cmd
	h.mu.Unlock()

	// Start httpx process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start httpx: %w", err)
	}

	h.logger.Debug("httpx process started", "pid", cmd.Process.Pid)

	// Parse stdout in real-time (streaming JSONL)
	responses := make([]*HTTPXResponse, 0, 100)
	scanner := bufio.NewScanner(stdout)

	// Increase buffer size for large responses
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max token size

	for scanner.Scan() {
		line := scanner.Text()

		var resp HTTPXResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			h.logger.Warn("failed to parse httpx output", "line", line, "error", err.Error())
			continue
		}

		responses = append(responses, &resp)

		h.logger.Debug("parsed httpx response",
			"url", resp.URL,
			"status_code", resp.StatusCode,
			"title", resp.Title,
		)
	}

	if err := scanner.Err(); err != nil {
		h.logger.Warn("scanner error", "error", err.Error())
	}

	// Capture stderr for warnings
	stderrBytes, _ := io.ReadAll(stderr)
	if len(stderrBytes) > 0 {
		stderrStr := string(stderrBytes)
		h.logger.Debug("httpx stderr", "output", stderrStr)
		result.AddWarning("httpx", fmt.Sprintf("stderr output: %s", stderrStr))
	}

	// Wait for process to complete
	if err := cmd.Wait(); err != nil {
		// Don't fail if we got some results
		if len(responses) > 0 {
			h.logger.Warn("httpx exited with error but produced results", "error", err.Error())
			result.AddWarning("httpx", fmt.Sprintf("process exited with error: %v", err))
		} else {
			return nil, fmt.Errorf("httpx failed: %w", err)
		}
	}

	// Parse responses into artifacts
	artifacts := h.parser.ParseMultipleResponses(responses, target)
	for _, artifact := range artifacts {
		result.AddArtifact(artifact)
	}

	duration := time.Since(startTime)
	h.logger.Info("httpx scan completed",
		"target", target.Root,
		"duration", duration.String(),
		"responses", len(responses),
		"artifacts", len(result.Artifacts),
	)

	return result, nil
}

// Close terminates the httpx process and cleans up resources.
func (h *HTTPXSource) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.logger.Debug("closing httpx source")

	// Cancel context
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}

	// Kill process if still running
	if h.cmd != nil && h.cmd.Process != nil {
		// Check if process is still running by getting process state
		if h.cmd.ProcessState == nil || !h.cmd.ProcessState.Exited() {
			// Try SIGTERM first
			if err := h.cmd.Process.Signal(os.Interrupt); err != nil {
				// Only log if it's not "already finished"
				if err.Error() != "os: process already finished" {
					h.logger.Warn("SIGTERM failed, forcing kill", "error", err.Error())
					if killErr := h.cmd.Process.Kill(); killErr != nil && killErr.Error() != "os: process already finished" {
						h.logger.Warn("failed to kill httpx process", "error", killErr.Error())
					}
				}
			}
		}

		h.cmd = nil
	}

	h.logger.Debug("httpx source closed")
	return nil
}

// Initialize verifies that httpx is installed and accessible.
// Implements ports.AdvancedSource.
func (h *HTTPXSource) Initialize() error {
	h.logger.Debug("initializing httpx source", "exec_path", h.execPath)

	// Check if httpx binary exists
	execPath, err := exec.LookPath(h.execPath)
	if err != nil {
		return fmt.Errorf("httpx not found in PATH: %w (install with: go install github.com/projectdiscovery/httpx/cmd/httpx@latest)", err)
	}

	h.execPath = execPath
	h.logger.Debug("found httpx binary", "path", execPath)

	// Check version
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, h.execPath, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check httpx version: %w", err)
	}

	version := string(output)
	h.logger.Info("httpx initialized successfully", "version", version)

	return nil
}

// Validate checks if the source configuration is valid.
// Implements ports.AdvancedSource.
func (h *HTTPXSource) Validate() error {
	if h.execPath == "" {
		return fmt.Errorf("httpx exec path is empty")
	}

	if h.timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if h.threads <= 0 || h.threads > 1000 {
		return fmt.Errorf("threads must be between 1 and 1000")
	}

	if h.rateLimit < 0 {
		return fmt.Errorf("rate limit cannot be negative")
	}

	if _, exists := Profiles[h.profile]; !exists {
		return fmt.Errorf("invalid scan profile: %s", h.profile)
	}

	return nil
}

// HealthCheck verifies that httpx is responsive.
// Implements ports.AdvancedSource.
func (h *HTTPXSource) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, h.execPath, "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("httpx health check failed: %w", err)
	}

	return nil
}

// buildCommand constructs the httpx command with appropriate flags.
func (h *HTTPXSource) buildCommand(ctx context.Context, target domain.Target) *exec.Cmd {
	profileCfg := GetProfile(h.profile)

	args := []string{
		"-u", target.Root, // Target URL/domain
		"-json",           // JSON output
		"-silent",         // No progress output
		"-no-color",       // No ANSI colors
	}

	// Add profile-specific flags
	args = append(args, profileCfg.Flags...)

	// Add performance flags
	args = append(args,
		"-t", strconv.Itoa(h.threads),
		"-rl", strconv.Itoa(h.rateLimit),
		"-timeout", strconv.Itoa(int(h.timeout.Seconds())),
		"-retries", "2",
		"-maxr", "5", // Max redirects
	)

	// Add optimization flags
	args = append(args,
		"-no-fallback",      // Don't try HTTP if HTTPS fails
		"-random-agent",     // Random User-Agent
		"-follow-redirects", // Follow redirects
	)

	// Add custom flags
	args = append(args, h.customFlags...)

	// Create command with timeout context
	cmdCtx, cancel := context.WithTimeout(ctx, h.timeout+30*time.Second) // +30s buffer
	h.mu.Lock()
	h.cancel = cancel
	h.mu.Unlock()

	cmd := exec.CommandContext(cmdCtx, h.execPath, args...)

	h.logger.Debug("built httpx command",
		"args", args,
		"timeout", h.timeout.String(),
	)

	return cmd
}

// SetCustomFlags allows adding custom httpx flags.
func (h *HTTPXSource) SetCustomFlags(flags []string) {
	h.customFlags = flags
}

// SetProfile changes the scan profile.
func (h *HTTPXSource) SetProfile(profile ScanProfile) {
	h.profile = profile
}

// RunWithInput executes httpx with artifacts from previous stages.
// Implements ports.InputConsumer interface.
func (h *HTTPXSource) RunWithInput(ctx context.Context, target domain.Target, input *domain.ScanResult) (*domain.ScanResult, error) {
	result := domain.NewScanResult(target)
	startTime := time.Now()

	// Extract targets from input artifacts
	targets := h.extractTargetsFromInput(input)

	if len(targets) == 0 {
		h.logger.Warn("no input artifacts found, using root target", "target", target.Root)
		return h.Run(ctx, target)
	}

	h.logger.Info("starting httpx scan with input artifacts",
		"target", target.Root,
		"profile", h.profile,
		"input_targets", len(targets),
		"threads", h.threads,
		"rate_limit", h.rateLimit,
	)

	// Build command with targets via stdin
	cmd := h.buildCommandWithStdin(ctx, targets)

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

	// Create stdin pipe to send targets
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Store command reference for Close()
	h.mu.Lock()
	h.cmd = cmd
	h.mu.Unlock()

	// Start httpx process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start httpx: %w", err)
	}

	h.logger.Debug("httpx process started", "pid", cmd.Process.Pid)

	// Write targets to stdin in goroutine
	go func() {
		defer stdin.Close()
		for _, t := range targets {
			fmt.Fprintln(stdin, t)
		}
	}()

	// Parse stdout in real-time (streaming JSONL)
	responses := make([]*HTTPXResponse, 0, len(targets))
	scanner := bufio.NewScanner(stdout)

	// Increase buffer size for large responses
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max token size

	for scanner.Scan() {
		line := scanner.Text()

		var resp HTTPXResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			h.logger.Warn("failed to parse httpx output", "line", line, "error", err.Error())
			continue
		}

		responses = append(responses, &resp)

		h.logger.Debug("parsed httpx response",
			"url", resp.URL,
			"status_code", resp.StatusCode,
			"title", resp.Title,
		)
	}

	if err := scanner.Err(); err != nil {
		h.logger.Warn("scanner error", "error", err.Error())
	}

	// Capture stderr for warnings
	stderrBytes, _ := io.ReadAll(stderr)
	if len(stderrBytes) > 0 {
		stderrStr := string(stderrBytes)
		h.logger.Debug("httpx stderr", "output", stderrStr)
		result.AddWarning("httpx", fmt.Sprintf("stderr output: %s", stderrStr))
	}

	// Wait for process to complete
	if err := cmd.Wait(); err != nil {
		// Don't fail if we got some results
		if len(responses) > 0 {
			h.logger.Warn("httpx exited with error but produced results", "error", err.Error())
			result.AddWarning("httpx", fmt.Sprintf("process exited with error: %v", err))
		} else {
			return nil, fmt.Errorf("httpx failed: %w", err)
		}
	}

	// Parse responses into artifacts
	artifacts := h.parser.ParseMultipleResponses(responses, target)
	for _, artifact := range artifacts {
		result.AddArtifact(artifact)
	}

	duration := time.Since(startTime)
	h.logger.Info("httpx scan completed with input",
		"target", target.Root,
		"duration", duration.String(),
		"input_targets", len(targets),
		"responses", len(responses),
		"artifacts", len(result.Artifacts),
	)

	return result, nil
}

// extractTargetsFromInput extracts domains, subdomains, and URLs from input artifacts.
func (h *HTTPXSource) extractTargetsFromInput(input *domain.ScanResult) []string {
	targets := make([]string, 0, len(input.Artifacts))
	seen := make(map[string]bool)

	for _, artifact := range input.Artifacts {
		var target string

		switch artifact.Type {
		case domain.ArtifactTypeSubdomain, domain.ArtifactTypeDomain:
			// Add as domain (httpx will try http:// and https://)
			target = artifact.Value
		case domain.ArtifactTypeURL:
			// Use URL directly
			target = artifact.Value
		default:
			continue
		}

		// Deduplicate targets
		if target != "" && !seen[target] {
			targets = append(targets, target)
			seen[target] = true
		}
	}

	h.logger.Debug("extracted targets from input",
		"total_artifacts", len(input.Artifacts),
		"extracted_targets", len(targets),
	)

	return targets
}

// buildCommandWithStdin constructs httpx command to read targets from stdin.
func (h *HTTPXSource) buildCommandWithStdin(ctx context.Context, targets []string) *exec.Cmd {
	profileCfg := GetProfile(h.profile)

	args := []string{
		"-json",     // JSON output
		"-silent",   // No progress output
		"-no-color", // No ANSI colors
	}

	// Add profile-specific flags
	args = append(args, profileCfg.Flags...)

	// Add performance flags
	args = append(args,
		"-t", strconv.Itoa(h.threads),
		"-rl", strconv.Itoa(h.rateLimit),
		"-timeout", strconv.Itoa(int(h.timeout.Seconds())),
		"-retries", "2",
		"-maxr", "5", // Max redirects
	)

	// Add optimization flags
	args = append(args,
		"-no-fallback",      // Don't try HTTP if HTTPS fails
		"-random-agent",     // Random User-Agent
		"-follow-redirects", // Follow redirects
	)

	// Add custom flags
	args = append(args, h.customFlags...)

	// Create command with timeout context
	// Calculate timeout based on number of targets
	estimatedDuration := time.Duration(len(targets)) * (h.timeout / time.Duration(h.threads))
	totalTimeout := estimatedDuration + 60*time.Second // +60s buffer

	cmdCtx, cancel := context.WithTimeout(ctx, totalTimeout)
	h.mu.Lock()
	h.cancel = cancel
	h.mu.Unlock()

	cmd := exec.CommandContext(cmdCtx, h.execPath, args...)

	h.logger.Debug("built httpx command with stdin",
		"args", args,
		"targets_count", len(targets),
		"estimated_duration", estimatedDuration.String(),
		"total_timeout", totalTimeout.String(),
	)

	return cmd
}
