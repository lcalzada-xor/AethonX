// Package httpx implements integration with Project Discovery's httpx CLI tool.
// It executes httpx as a subprocess and parses its JSON output to create artifacts.
package httpx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
	"aethonx/internal/sources/common"
)

const (
	sourceName       = "httpx"
	defaultTimeout   = 120 * time.Second
	defaultThreads   = 75
	defaultRateLimit = 150

	// Verification profile optimizations (for waybackurls mass validation)
	verificationThreads   = 150
	verificationRateLimit = 300
	verificationTimeout   = 5 * time.Second
)

// HTTPXSource implements ports.Source and ports.AdvancedSource.
// It wraps Project Discovery's httpx CLI tool for HTTP probing and fingerprinting.
type HTTPXSource struct {
	*common.BaseCLISource // Embedded base for subprocess management

	profile     ScanProfile // Scan profile to use
	threads     int
	rateLimit   int
	customFlags []string
	parser      *Parser
}

// New creates a new HTTPXSource with default configuration.
func New(logger logx.Logger) *HTTPXSource {
	return &HTTPXSource{
		BaseCLISource: common.NewBaseCLISource(logger, common.BaseCLIConfig{
			SourceName:     sourceName,
			ExecPath:       "httpx",
			Timeout:        defaultTimeout,
			ProgressBuffer: 10,
		}),
		profile:     ProfileFull,
		threads:     defaultThreads,
		rateLimit:   defaultRateLimit,
		customFlags: []string{},
		parser:      NewParser(logger, sourceName),
	}
}

// NewWithConfig creates HTTPXSource with custom configuration.
func NewWithConfig(logger logx.Logger, execPath string, profile ScanProfile, timeout time.Duration, threads, rateLimit int) *HTTPXSource {
	return &HTTPXSource{
		BaseCLISource: common.NewBaseCLISource(logger, common.BaseCLIConfig{
			SourceName:     sourceName,
			ExecPath:       execPath,
			Timeout:        timeout,
			ProgressBuffer: 10,
		}),
		profile:     profile,
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
	startTime := time.Now()

	h.GetLogger().Info("starting httpx scan",
		"target", target.Root,
		"profile", h.profile,
		"threads", h.threads,
		"rate_limit", h.rateLimit,
	)

	// Build command arguments
	args := h.buildCommandArgs(target)

	// Create handler for processing output
	handler := &httpxHandler{
		parser:    h.parser,
		target:    target,
		logger:    h.GetLogger(),
		responses: make([]*HTTPXResponse, 0, 100),
	}

	// Execute CLI with handler (BaseCLISource handles all subprocess logic)
	result, stderrOutput, err := h.ExecuteCLI(ctx, target, args, handler)

	// Handle fatal errors (e.g., failed to start process)
	if result == nil {
		return nil, fmt.Errorf("httpx failed to start: %w", err)
	}

	// Handle stderr warnings
	if len(stderrOutput) > 0 {
		h.GetLogger().Debug("httpx stderr", "output", stderrOutput)
		result.AddWarning("httpx", fmt.Sprintf("stderr output: %s", stderrOutput))
	}

	// Handle errors (partial results tolerated)
	if err != nil {
		responseCount := len(handler.responses)
		if responseCount > 0 {
			h.GetLogger().Warn("httpx exited with error but produced results",
				"error", err.Error(),
				"responses", responseCount,
			)
			result.AddWarning("httpx", fmt.Sprintf("process exited with error: %v", err))
		} else {
			return nil, fmt.Errorf("httpx failed: %w", err)
		}
	}

	// Parse responses into artifacts (after ExecuteCLI completes)
	artifacts := h.parser.ParseMultipleResponses(handler.responses, target)
	for _, artifact := range artifacts {
		result.AddArtifact(artifact)
	}

	duration := time.Since(startTime)
	h.GetLogger().Info("httpx scan completed",
		"target", target.Root,
		"duration", duration.String(),
		"responses", len(handler.responses),
		"artifacts", len(result.Artifacts),
	)

	return result, nil
}

// httpxHandler implements common.OutputHandler for httpx JSON output processing.
type httpxHandler struct {
	parser    *Parser
	target    domain.Target
	logger    logx.Logger
	responses []*HTTPXResponse

	// State
	mu sync.Mutex
}

// ProcessLine handles each line of httpx stdout (JSON lines).
func (h *httpxHandler) ProcessLine(line []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var resp HTTPXResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		h.logger.Warn("failed to parse httpx output", "line", string(line), "error", err.Error())
		return nil // Non-fatal, continue processing
	}

	h.responses = append(h.responses, &resp)

	h.logger.Debug("parsed httpx response",
		"url", resp.URL,
		"status_code", resp.StatusCode,
		"title", resp.Title,
	)

	return nil
}

// Finalize is called after all lines are processed.
func (h *httpxHandler) Finalize() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.logger.Info("parsing responses to artifacts", "count", len(h.responses))

	// This is handled in Run() after ExecuteCLI returns
	// We don't populate result here because ExecuteCLI creates a new result
	// Instead, we store responses and let Run() handle artifact creation

	return nil
}

// Stream implements ports.StreamingSource.
func (h *HTTPXSource) Stream(ctx context.Context, target domain.Target) (<-chan *domain.Artifact, <-chan error) {
	return h.DefaultStream(ctx, target, h.Run)
}

// Initialize verifies that httpx is installed and accessible.
// Implements ports.AdvancedSource.
func (h *HTTPXSource) Initialize() error {
	return h.DefaultInitialize(
		"httpx",
		"go install github.com/projectdiscovery/httpx/cmd/httpx@latest",
	)
}

// Validate checks if the source configuration is valid.
// Implements ports.AdvancedSource.
func (h *HTTPXSource) Validate() error {
	// First check base validation
	if err := h.DefaultValidate(); err != nil {
		return err
	}

	// Additional httpx-specific validation
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
	return h.DefaultHealthCheck(ctx)
}

// buildCommandArgs constructs the httpx command arguments.
func (h *HTTPXSource) buildCommandArgs(target domain.Target) []string {
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
		"-timeout", strconv.Itoa(int(h.GetTimeout().Seconds())),
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

	h.GetLogger().Debug("built httpx command",
		"args", args,
		"timeout", h.GetTimeout().String(),
	)

	return args
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

	// Separate artifacts by confidence level (waybackurls vs others)
	waybackurlsTargets, otherTargets := h.separateTargetsBySource(input)

	if len(waybackurlsTargets) == 0 && len(otherTargets) == 0 {
		h.GetLogger().Warn("no input artifacts found, using root target", "target", target.Root)
		return h.Run(ctx, target)
	}

	h.GetLogger().Info("starting httpx scan with smart profile selection",
		"target", target.Root,
		"waybackurls_targets", len(waybackurlsTargets),
		"other_targets", len(otherTargets),
	)

	// Execute verification profile for waybackurls (fast)
	if len(waybackurlsTargets) > 0 {
		verificationResults, err := h.runWithProfile(ctx, target, waybackurlsTargets, ProfileVerification, input.Artifacts)
		if err != nil {
			h.GetLogger().Warn("verification profile failed", "error", err.Error())
			result.AddWarning("httpx", fmt.Sprintf("verification failed: %v", err))
		} else {
			// Merge results
			for _, artifact := range verificationResults.Artifacts {
				result.AddArtifact(artifact)
			}
		}
	}

	// Execute full profile for other sources (comprehensive)
	if len(otherTargets) > 0 {
		fullResults, err := h.runWithProfile(ctx, target, otherTargets, h.profile, input.Artifacts)
		if err != nil {
			h.GetLogger().Warn("full profile failed", "error", err.Error())
			result.AddWarning("httpx", fmt.Sprintf("full profile failed: %v", err))
		} else {
			// Merge results
			for _, artifact := range fullResults.Artifacts {
				result.AddArtifact(artifact)
			}
		}
	}

	duration := time.Since(startTime)
	totalProbed := len(waybackurlsTargets) + len(otherTargets)
	totalAlive := len(result.Artifacts)

	h.GetLogger().Info("httpx scan completed with smart profiles",
		"target", target.Root,
		"duration", duration.String(),
		"waybackurls_verified", len(waybackurlsTargets),
		"others_scanned", len(otherTargets),
		"total_probed", totalProbed,
		"total_alive", totalAlive,
	)

	// Store statistics in metadata for UI summary
	if result.Metadata.Environment == nil {
		result.Metadata.Environment = make(map[string]string)
	}
	result.Metadata.Environment["httpx_probed"] = fmt.Sprintf("%d", totalProbed)
	result.Metadata.Environment["httpx_alive"] = fmt.Sprintf("%d", totalAlive)

	return result, nil
}

// separateTargetsBySource separates targets into waybackurls and others based on artifact source.
func (h *HTTPXSource) separateTargetsBySource(input *domain.ScanResult) (waybackurls []string, others []string) {
	waybackurlsSet := make(map[string]bool)
	othersSet := make(map[string]bool)

	for _, artifact := range input.Artifacts {
		var target string

		switch artifact.Type {
		case domain.ArtifactTypeSubdomain, domain.ArtifactTypeDomain:
			target = artifact.Value
		case domain.ArtifactTypeURL:
			target = artifact.Value
		default:
			continue
		}

		if target == "" {
			continue
		}

		// Check if artifact is from waybackurls
		isFromWaybackurls := false
		for _, source := range artifact.Sources {
			if source == "waybackurls" {
				isFromWaybackurls = true
				break
			}
		}

		if isFromWaybackurls {
			waybackurlsSet[target] = true
		} else {
			othersSet[target] = true
		}
	}

	// Convert sets to slices
	waybackurls = make([]string, 0, len(waybackurlsSet))
	for target := range waybackurlsSet {
		waybackurls = append(waybackurls, target)
	}

	others = make([]string, 0, len(othersSet))
	for target := range othersSet {
		others = append(others, target)
	}

	h.GetLogger().Debug("separated targets by source",
		"waybackurls", len(waybackurls),
		"others", len(others),
	)

	return waybackurls, others
}

// runWithProfile executes httpx with a specific profile for the given targets.
func (h *HTTPXSource) runWithProfile(ctx context.Context, target domain.Target, targets []string, profile ScanProfile, inputArtifacts []*domain.Artifact) (*domain.ScanResult, error) {
	result := domain.NewScanResult(target)
	startTime := time.Now()

	// Temporarily switch profile
	originalProfile := h.profile
	originalThreads := h.threads
	originalRateLimit := h.rateLimit
	originalTimeout := h.GetTimeout()

	h.profile = profile

	// Apply optimized settings for verification profile
	if profile == ProfileVerification {
		h.threads = verificationThreads
		h.rateLimit = verificationRateLimit
		h.SetTimeout(verificationTimeout)
		h.GetLogger().Debug("applying verification profile optimizations",
			"threads", h.threads,
			"rate_limit", h.rateLimit,
			"timeout", verificationTimeout.String(),
		)
	}

	defer func() {
		h.profile = originalProfile
		h.threads = originalThreads
		h.rateLimit = originalRateLimit
		h.SetTimeout(originalTimeout)
	}()

	h.GetLogger().Info("running httpx with profile",
		"profile", profile,
		"targets", len(targets),
		"threads", h.threads,
		"rate_limit", h.rateLimit,
	)

	// Build command arguments for stdin mode
	args := h.buildCommandArgsWithStdin()

	// Create handler for processing output
	handler := &httpxHandler{
		parser:    h.parser,
		target:    target,
		logger:    h.GetLogger(),
		responses: make([]*HTTPXResponse, 0, len(targets)),
	}

	// Build command with context
	cmd := exec.CommandContext(ctx, h.GetExecPath(), args...)

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

	// Start httpx process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start httpx: %w", err)
	}

	h.GetLogger().Debug("httpx process started", "pid", cmd.Process.Pid)

	// Write targets to stdin in goroutine
	go func() {
		defer stdin.Close()
		for _, t := range targets {
			fmt.Fprintln(stdin, t)
		}
	}()

	// Process stdout using handler
	if err := h.ProcessOutput(stdout, handler); err != nil {
		h.GetLogger().Warn("output processing error", "error", err.Error())
	}

	// Capture stderr for warnings
	stderrBytes, _ := io.ReadAll(stderr)
	if len(stderrBytes) > 0 {
		stderrStr := string(stderrBytes)
		h.GetLogger().Debug("httpx stderr", "output", stderrStr)
		result.AddWarning("httpx", fmt.Sprintf("stderr output: %s", stderrStr))
	}

	// Wait for process to complete
	if err := cmd.Wait(); err != nil {
		// Don't fail if we got some results
		if len(handler.responses) > 0 {
			h.GetLogger().Warn("httpx exited with error but produced results", "error", err.Error())
			result.AddWarning("httpx", fmt.Sprintf("process exited with error: %v", err))
		} else {
			return nil, fmt.Errorf("httpx failed: %w", err)
		}
	}

	// Finalize handler
	if err := handler.Finalize(); err != nil {
		h.GetLogger().Warn("handler finalization error", "error", err.Error())
	}

	// Parse responses into artifacts with confidence upgrade
	artifacts := h.parser.ParseMultipleResponsesWithInput(handler.responses, target, inputArtifacts)
	for _, artifact := range artifacts {
		result.AddArtifact(artifact)
	}

	duration := time.Since(startTime)
	h.GetLogger().Info("httpx profile execution completed",
		"target", target.Root,
		"duration", duration.String(),
		"input_targets", len(targets),
		"responses", len(handler.responses),
		"artifacts", len(result.Artifacts),
	)

	return result, nil
}

// buildCommandArgsWithStdin constructs httpx command arguments to read targets from stdin.
func (h *HTTPXSource) buildCommandArgsWithStdin() []string {
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
		"-timeout", strconv.Itoa(int(h.GetTimeout().Seconds())),
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

	h.GetLogger().Debug("built httpx command with stdin",
		"args", args,
		"httpx_request_timeout", h.GetTimeout().String(),
	)

	return args
}
