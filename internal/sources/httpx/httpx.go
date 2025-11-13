// Package httpx implements integration with Project Discovery's httpx CLI tool.
// It executes httpx as a subprocess and parses its JSON output to create artifacts.
package httpx

import (
	"context"
	"fmt"
	"strconv"
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

	// Calculate effective timeout: use the most restrictive between:
	// 1. HTTPx default timeout (120s)
	// 2. Context deadline (if present)
	effectiveTimeout := h.calculateEffectiveTimeout(ctx)
	h.GetLogger().Debug("calculated effective timeout",
		"default_timeout", h.GetTimeout(),
		"effective_timeout", effectiveTimeout,
	)

	// Build command arguments with effective timeout
	args := h.buildCommandArgsWithTimeout(target, effectiveTimeout)

	// Create handler using JSONLineHandler
	handler := common.NewJSONLineHandler[HTTPXResponse](h.GetLogger(), target)

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

	// Get parsed responses from handler
	responses := handler.GetResponses()

	// Convert []HTTPXResponse to []*HTTPXResponse for parser
	responsePtrs := make([]*HTTPXResponse, len(responses))
	for i := range responses {
		responsePtrs[i] = &responses[i]
	}

	// Parse responses into artifacts (siempre, incluso con error)
	artifacts := h.parser.ParseMultipleResponses(responsePtrs, target)
	for _, artifact := range artifacts {
		result.AddArtifact(artifact)
	}

	duration := time.Since(startTime)
	responseCount := len(responses)
	artifactCount := len(result.Artifacts)

	// Handle errors (partial results tolerados y retornados)
	if err != nil {
		if artifactCount > 0 {
			h.GetLogger().Warn("httpx exited with error but produced partial results",
				"error", err.Error(),
				"responses", responseCount,
				"artifacts", artifactCount,
			)
			result.AddWarning("httpx", fmt.Sprintf("process exited with error: %v", err))

			h.GetLogger().Info("returning partial results from httpx",
				"artifacts", artifactCount,
			)
			// Retornar resultado parcial + error para que orchestrator decida
			return result, fmt.Errorf("httpx timeout/cancelled but produced %d partial results: %w", artifactCount, err)
		}

		// Sin resultados: retornar error total
		return nil, fmt.Errorf("httpx failed without results: %w", err)
	}

	h.GetLogger().Info("httpx scan completed",
		"target", target.Root,
		"duration", duration.String(),
		"responses", responseCount,
		"artifacts", artifactCount,
	)

	return result, nil
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

// calculateEffectiveTimeout calculates the most restrictive timeout between:
// - HTTPx default timeout
// - Context deadline (time remaining until context cancellation)
// This prevents double timeout conflicts where httpx's internal timeout
// exceeds the context deadline, causing abrupt process termination.
func (h *HTTPXSource) calculateEffectiveTimeout(ctx context.Context) time.Duration {
	defaultTimeout := h.GetTimeout()

	// Check if context has a deadline
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		// No context deadline, use httpx default
		return defaultTimeout
	}

	// Calculate time remaining until context deadline
	remaining := time.Until(deadline)

	// Reserve buffer for graceful shutdown and enforce minimum timeout
	const gracefulBuffer = 5 * time.Second
	const minTimeout = 1 * time.Second

	// Check if we have enough time for buffer + minimum timeout
	if remaining <= gracefulBuffer+minTimeout {
		// Very little time left, use minimum viable timeout
		h.GetLogger().Debug("context deadline too close, using minimum timeout",
			"context_remaining", remaining,
			"min_timeout", minTimeout,
		)
		remaining = minTimeout
	} else {
		// Subtract buffer to allow graceful shutdown
		remaining -= gracefulBuffer
	}

	// Return the most restrictive (minimum) timeout
	if remaining < defaultTimeout {
		h.GetLogger().Debug("using context-based timeout (more restrictive than default)",
			"context_remaining", remaining,
			"default", defaultTimeout,
		)
		return remaining
	}

	return defaultTimeout
}

// buildCommandArgs constructs the httpx command arguments (deprecated, use buildCommandArgsWithTimeout).
func (h *HTTPXSource) buildCommandArgs(target domain.Target) []string {
	return h.buildCommandArgsWithTimeout(target, h.GetTimeout())
}

// buildCommandArgsWithTimeout constructs the httpx command arguments with explicit timeout.
func (h *HTTPXSource) buildCommandArgsWithTimeout(target domain.Target, timeout time.Duration) []string {
	profileCfg := GetProfile(h.profile)

	args := []string{
		"-u", target.Root, // Target URL/domain
		"-json",           // JSON output
		"-silent",         // No progress output
		"-no-color",       // No ANSI colors
	}

	// Add profile-specific flags
	args = append(args, profileCfg.Flags...)

	// Add performance flags with effective timeout
	args = append(args,
		"-t", strconv.Itoa(h.threads),
		"-rl", strconv.Itoa(h.rateLimit),
		"-timeout", strconv.Itoa(int(timeout.Seconds())),
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
		"timeout", timeout.String(),
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
		}
		// Merge results even with error (partial results)
		if verificationResults != nil {
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
		}
		// Merge results even with error (partial results)
		if fullResults != nil {
			for _, artifact := range fullResults.Artifacts {
				result.AddArtifact(artifact)
			}
		}
	}

	duration := time.Since(startTime)
	totalProbed := len(waybackurlsTargets) + len(otherTargets)

	// Count alive hosts: each alive host generates exactly 1 URL artifact
	totalAlive := 0
	for _, artifact := range result.Artifacts {
		if artifact.Type == domain.ArtifactTypeURL {
			totalAlive++
		}
	}

	h.GetLogger().Info("httpx scan completed with smart profiles",
		"target", target.Root,
		"duration", duration.String(),
		"waybackurls_verified", len(waybackurlsTargets),
		"others_scanned", len(otherTargets),
		"total_probed", totalProbed,
		"total_alive", totalAlive,
		"total_artifacts", len(result.Artifacts),
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

	// Calculate effective timeout for this profile
	effectiveTimeout := h.calculateEffectiveTimeout(ctx)

	h.GetLogger().Info("running httpx with profile",
		"profile", profile,
		"targets", len(targets),
		"threads", h.threads,
		"rate_limit", h.rateLimit,
		"effective_timeout", effectiveTimeout,
	)

	// Build command arguments for stdin mode with effective timeout
	args := h.buildCommandArgsWithStdinAndTimeout(effectiveTimeout)

	// Create handler using JSONLineHandler (type-safe, thread-safe)
	handler := common.NewJSONLineHandler[HTTPXResponse](h.GetLogger(), target)

	// Execute CLI with stdin using BaseCLISource abstraction
	result, stderrOutput, err := h.ExecuteCLIWithStdin(ctx, target, args, targets, handler)

	// Check if result is nil (can happen on early errors like pipe creation failure)
	if result == nil {
		h.GetLogger().Warn("httpx execution failed, result is nil", "error", err)
		return nil, fmt.Errorf("httpx execution failed: %w", err)
	}

	// Handle stderr warnings
	if len(stderrOutput) > 0 {
		h.GetLogger().Debug("httpx stderr", "output", stderrOutput)
		result.AddWarning("httpx", fmt.Sprintf("stderr output: %s", stderrOutput))
	}

	// Get parsed responses from handler
	responses := handler.GetResponses()

	// Convert []HTTPXResponse to []*HTTPXResponse for parser
	responsePtrs := make([]*HTTPXResponse, len(responses))
	for i := range responses {
		responsePtrs[i] = &responses[i]
	}

	// Parse responses into artifacts with confidence upgrade
	artifacts := h.parser.ParseMultipleResponsesWithInput(responsePtrs, target, inputArtifacts)
	for _, artifact := range artifacts {
		result.AddArtifact(artifact)
	}

	duration := time.Since(startTime)
	responseCount := len(responses)
	artifactCount := len(result.Artifacts)

	// Handle errors (partial results tolerados y retornados)
	if err != nil {
		if artifactCount > 0 {
			h.GetLogger().Warn("httpx exited with error but produced partial results",
				"error", err.Error(),
				"responses", responseCount,
				"artifacts", artifactCount,
			)
			result.AddWarning("httpx", fmt.Sprintf("process exited with error: %v", err))

			h.GetLogger().Info("returning partial results from httpx profile",
				"profile", profile,
				"artifacts", artifactCount,
			)
			// Retornar resultado parcial + error para que orchestrator decida
			return result, fmt.Errorf("httpx timeout/cancelled but produced %d partial results: %w", artifactCount, err)
		}

		// Sin resultados: retornar error total
		return nil, fmt.Errorf("httpx profile failed without results: %w", err)
	}

	h.GetLogger().Info("httpx profile execution completed",
		"target", target.Root,
		"duration", duration.String(),
		"input_targets", len(targets),
		"responses", responseCount,
		"artifacts", artifactCount,
	)

	return result, nil
}

// buildCommandArgsWithStdin constructs httpx command arguments to read targets from stdin (deprecated, use buildCommandArgsWithStdinAndTimeout).
func (h *HTTPXSource) buildCommandArgsWithStdin() []string {
	return h.buildCommandArgsWithStdinAndTimeout(h.GetTimeout())
}

// buildCommandArgsWithStdinAndTimeout constructs httpx command arguments to read targets from stdin with explicit timeout.
func (h *HTTPXSource) buildCommandArgsWithStdinAndTimeout(timeout time.Duration) []string {
	profileCfg := GetProfile(h.profile)

	args := []string{
		"-json",     // JSON output
		"-silent",   // No progress output
		"-no-color", // No ANSI colors
	}

	// Add profile-specific flags
	args = append(args, profileCfg.Flags...)

	// Add performance flags with effective timeout
	args = append(args,
		"-t", strconv.Itoa(h.threads),
		"-rl", strconv.Itoa(h.rateLimit),
		"-timeout", strconv.Itoa(int(timeout.Seconds())),
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
		"httpx_request_timeout", timeout.String(),
	)

	return args
}
