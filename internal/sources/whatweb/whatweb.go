// Package whatweb implements integration with WhatWeb CLI tool.
// WhatWeb is a web scanner that identifies technologies, frameworks, and services.
package whatweb

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/sources/common"
)

const (
	sourceName        = "whatweb"
	defaultTimeout    = 120 * time.Second
	defaultThreads    = 25
	defaultAggression = 4 // Heavy mode for maximum detection
	maxURLs           = 5 // Maximum URLs to scan per target
)

// WhatWebSource implements ports.Source and ports.InputConsumer.
// It wraps WhatWeb CLI tool for web technology fingerprinting.
type WhatWebSource struct {
	*common.BaseCLISource // Embedded base for subprocess management

	aggression int
	threads    int
	userAgent  string
	parser     *Parser
}

// Compile-time interface assertions
var (
	_ ports.Source        = (*WhatWebSource)(nil)
	_ ports.InputConsumer = (*WhatWebSource)(nil)
)

// New creates a new WhatWebSource with default configuration.
func New(logger logx.Logger) *WhatWebSource {
	return &WhatWebSource{
		BaseCLISource: common.NewBaseCLISource(logger, common.BaseCLIConfig{
			SourceName:     sourceName,
			ExecPath:       "whatweb",
			Timeout:        defaultTimeout,
			ProgressBuffer: 10,
		}),
		aggression: defaultAggression, // Heavy mode by default for maximum detection
		threads:    defaultThreads,
		userAgent:  "AethonX/1.0 (Web reconnaissance tool)",
		parser:     NewParser(logger, sourceName),
	}
}

// NewWithConfig creates WhatWebSource with custom configuration.
func NewWithConfig(
	logger logx.Logger,
	execPath string,
	timeout time.Duration,
	aggression int,
	threads int,
	userAgent string,
) *WhatWebSource {
	return &WhatWebSource{
		BaseCLISource: common.NewBaseCLISource(logger, common.BaseCLIConfig{
			SourceName:     sourceName,
			ExecPath:       execPath,
			Timeout:        timeout,
			ProgressBuffer: 10,
		}),
		aggression: aggression,
		threads:    threads,
		userAgent:  userAgent,
		parser:     NewParser(logger, sourceName),
	}
}

// Name returns the source name.
func (s *WhatWebSource) Name() string {
	return sourceName
}

// Mode returns the source operation mode.
func (s *WhatWebSource) Mode() domain.SourceMode {
	return domain.SourceModeActive
}

// Type returns the source type.
func (s *WhatWebSource) Type() domain.SourceType {
	return domain.SourceTypeCLI
}

// Run executes whatweb against the target (minimal scan without inputs).
func (s *WhatWebSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	// For Stage 2 sources, Run() is typically minimal - most work done in RunWithInput
	s.GetLogger().Info("whatweb run called without inputs",
		"target", target.Root,
		"note", "Stage 2 source - use RunWithInput for full scan",
	)

	result := domain.NewScanResult(target)
	result.AddWarning(sourceName, "whatweb is a Stage 2 source - provide URLs via RunWithInput for best results")

	// Optionally scan just the root domain
	items := []string{"http://" + target.Root, "https://" + target.Root}
	return s.scanURLs(ctx, target, items)
}

// RunWithInput executes whatweb with URLs from previous stages.
func (s *WhatWebSource) RunWithInput(ctx context.Context, target domain.Target, input *domain.ScanResult) (*domain.ScanResult, error) {
	startTime := time.Now()

	// Extract URLs and subdomains from input (max 5, root always included)
	items := s.extractInputItems(input, target)

	// If no items were found (httpx found no active URLs), scan root domain anyway
	if len(items) == 0 {
		s.GetLogger().Info("no active URLs from input, scanning root domain",
			"target", target.Root,
		)
		items = []string{"https://" + target.Root, "http://" + target.Root}
	}

	s.GetLogger().Info("starting whatweb scan with input",
		"target", target.Root,
		"input_items", len(items),
		"aggression", s.aggression,
		"threads", s.threads,
	)

	result, err := s.scanURLs(ctx, target, items)

	duration := time.Since(startTime)
	s.GetLogger().Info("whatweb scan with input completed",
		"target", target.Root,
		"duration", duration.String(),
		"artifacts", len(result.Artifacts),
	)

	return result, err
}

// scanURLs performs the actual whatweb scan.
func (s *WhatWebSource) scanURLs(ctx context.Context, target domain.Target, items []string) (*domain.ScanResult, error) {
	// Build command arguments
	args := s.buildCommandArgs()

	// Create handler for processing output
	handler := &whatwebHandler{
		parser:    s.parser,
		target:    target,
		logger:    s.GetLogger(),
		responses: make([]WhatWebResponse, 0, len(items)),
	}

	// Execute with stdin (whatweb accepts URLs via stdin with -i /dev/stdin)
	result, stderrOutput, err := s.ExecuteCLIWithStdin(ctx, target, args, items, handler)

	// Handle fatal errors
	if result == nil {
		return nil, fmt.Errorf("whatweb failed to start: %w", err)
	}

	// Handle stderr warnings
	if len(stderrOutput) > 0 {
		s.GetLogger().Debug("whatweb stderr", "output", stderrOutput)
		result.AddWarning(sourceName, fmt.Sprintf("stderr output: %s", stderrOutput))
	}

	// Handle errors (partial results tolerated)
	if err != nil {
		responseCount := len(handler.responses)
		if responseCount > 0 {
			s.GetLogger().Warn("whatweb exited with error but produced results",
				"error", err.Error(),
				"responses", responseCount,
			)
			result.AddWarning(sourceName, fmt.Sprintf("process exited with error: %v", err))
		} else {
			return nil, fmt.Errorf("whatweb failed: %w", err)
		}
	}

	// Parse responses into artifacts
	artifacts := s.parser.ParseMultipleResponses(handler.responses, target)
	for _, artifact := range artifacts {
		result.AddArtifact(artifact)
	}

	s.GetLogger().Info("whatweb parsing completed",
		"responses", len(handler.responses),
		"artifacts", len(result.Artifacts),
	)

	return result, nil
}

// extractInputItems extracts up to 5 active URLs from input artifacts.
// Priority: root domain first (ALWAYS included), then up to 4 additional active URLs for variety.
func (s *WhatWebSource) extractInputItems(input *domain.ScanResult, target domain.Target) []string {
	items := make([]string, 0, maxURLs)
	seen := make(map[string]bool)

	// Collect all active URLs with metadata
	type urlCandidate struct {
		url     string
		isRoot  bool
		status  int
		hasSSL  bool
	}
	candidates := make([]urlCandidate, 0)

	// Variable to track if we found root in candidates
	foundRoot := false

	for _, artifact := range input.Artifacts {
		var candidate urlCandidate

		// Extract URLs from URL artifacts
		if artifact.Type == domain.ArtifactTypeURL {
			candidate.url = artifact.Value

			// Check metadata
			if domainMeta, ok := artifact.TypedMetadata.(*metadata.DomainMetadata); ok && domainMeta != nil {
				candidate.status = domainMeta.HTTPStatus
				candidate.hasSSL = domainMeta.HasSSL
			}
		}

		// Extract URLs from Subdomain artifacts with metadata
		if artifact.Type == domain.ArtifactTypeSubdomain {
			if domainMeta, ok := artifact.TypedMetadata.(*metadata.DomainMetadata); ok && domainMeta != nil {
				// Only consider active subdomains (status 200-399)
				if domainMeta.HTTPStatus >= 200 && domainMeta.HTTPStatus < 400 {
					if domainMeta.HasSSL {
						candidate.url = "https://" + artifact.Value
					} else {
						candidate.url = "http://" + artifact.Value
					}
					candidate.status = domainMeta.HTTPStatus
					candidate.hasSSL = domainMeta.HasSSL
				}
			}
		}

		// Add candidate if valid and not duplicate
		if candidate.url != "" && !seen[candidate.url] {
			// Check if this is a root domain URL
			candidate.isRoot = s.isRootURL(candidate.url, target.Root)
			if candidate.isRoot {
				foundRoot = true
			}
			candidates = append(candidates, candidate)
			seen[candidate.url] = true
		}
	}

	// If root was not found in candidates, add it manually
	if !foundRoot {
		s.GetLogger().Info("root domain not found in active URLs, adding it manually",
			"root", target.Root,
		)
		// Try HTTPS first (more common nowadays)
		rootHTTPS := "https://" + target.Root
		candidates = append([]urlCandidate{{
			url:    rootHTTPS,
			isRoot: true,
			status: 0,
			hasSSL: true,
		}}, candidates...)
		seen[rootHTTPS] = true
	}

	// Sort candidates: root first, then by status code (prioritize successful responses)
	sortCandidates := func(c []urlCandidate) {
		for i := 0; i < len(c); i++ {
			for j := i + 1; j < len(c); j++ {
				// Root URLs always come first
				if c[j].isRoot && !c[i].isRoot {
					c[i], c[j] = c[j], c[i]
				} else if c[i].isRoot == c[j].isRoot {
					// Among non-root or among root URLs, prefer better status codes
					if c[j].status > 0 && (c[i].status == 0 || c[j].status < c[i].status) {
						c[i], c[j] = c[j], c[i]
					}
				}
			}
		}
	}
	sortCandidates(candidates)

	// Select up to maxURLs (5) URLs
	for i := 0; i < len(candidates) && len(items) < maxURLs; i++ {
		items = append(items, candidates[i].url)
	}

	s.GetLogger().Info("selected URLs for whatweb scan",
		"selected", len(items),
		"total_candidates", len(candidates),
		"max_urls", maxURLs,
		"root_included", foundRoot || len(candidates) > 0,
	)

	return items
}

// isRootURL checks if the URL corresponds to the root domain.
func (s *WhatWebSource) isRootURL(url string, rootDomain string) bool {
	// Simple check: URL contains only the root domain (no subdomain)
	return fmt.Sprintf("http://%s", rootDomain) == url ||
		fmt.Sprintf("https://%s", rootDomain) == url ||
		fmt.Sprintf("http://%s/", rootDomain) == url ||
		fmt.Sprintf("https://%s/", rootDomain) == url
}

// whatwebHandler implements common.OutputHandler for whatweb JSON output processing.
type whatwebHandler struct {
	parser    *Parser
	target    domain.Target
	logger    logx.Logger
	responses []WhatWebResponse
	mu        sync.Mutex
}

// ProcessLine handles each line of whatweb stdout (JSON lines).
func (h *whatwebHandler) ProcessLine(line []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var resp WhatWebResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		h.logger.Warn("failed to parse whatweb output", "line", string(line), "error", err.Error())
		return nil // Non-fatal, continue processing
	}

	h.responses = append(h.responses, resp)

	h.logger.Debug("parsed whatweb response",
		"target", resp.Target,
		"http_status", resp.HTTPStatus,
		"plugins_count", len(resp.Plugins),
	)

	return nil
}

// Finalize is called after all lines are processed.
func (h *whatwebHandler) Finalize() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.logger.Info("parsing responses to artifacts", "count", len(h.responses))
	return nil
}

// Stream implements ports.StreamingSource.
func (s *WhatWebSource) Stream(ctx context.Context, target domain.Target) (<-chan *domain.Artifact, <-chan error) {
	return s.DefaultStream(ctx, target, s.Run)
}

// Initialize verifies that whatweb is installed and accessible.
// Implements ports.AdvancedSource.
func (s *WhatWebSource) Initialize() error {
	return s.DefaultInitialize(
		"whatweb",
		"https://github.com/urbanadventurer/WhatWeb or apt-get install whatweb",
	)
}

// Validate checks if the source configuration is valid.
// Implements ports.AdvancedSource.
func (s *WhatWebSource) Validate() error {
	// Base validation
	if err := s.DefaultValidate(); err != nil {
		return err
	}

	// Aggression level validation
	if s.aggression < 1 || s.aggression > 4 {
		return fmt.Errorf("aggression must be between 1 and 4, got %d", s.aggression)
	}

	// Threads validation
	if s.threads <= 0 || s.threads > 100 {
		return fmt.Errorf("threads must be between 1 and 100, got %d", s.threads)
	}

	return nil
}

// HealthCheck verifies that whatweb is responsive.
// Implements ports.AdvancedSource.
func (s *WhatWebSource) HealthCheck(ctx context.Context) error {
	return s.DefaultHealthCheck(ctx)
}

// buildCommandArgs constructs the whatweb command arguments.
func (s *WhatWebSource) buildCommandArgs() []string {
	args := []string{
		"-i", "/dev/stdin", // Read targets from stdin
		"--log-json=-",                     // Output JSON to stdout
		"--colour=never",                   // Disable colors
		"--no-errors",                      // Don't print errors to stdout
		"-t", fmt.Sprintf("%d", s.threads), // Threads
		"-a", fmt.Sprintf("%d", s.aggression), // Aggression level
		"--user-agent", s.userAgent, // Custom user agent
	}

	s.GetLogger().Debug("built whatweb command",
		"args", args,
		"timeout", s.GetTimeout().String(),
	)

	return args
}
