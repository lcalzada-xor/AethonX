// Package subfinder implements integration with Project Discovery's subfinder CLI tool.
// It executes subfinder as a subprocess and parses its JSON output to create artifacts.
package subfinder

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
	"aethonx/internal/sources/common"
)

const (
	sourceName     = "subfinder"
	defaultTimeout = 240 * time.Second // subfinder with free sources
	defaultThreads = 10
)

// SubfinderSource implements ports.Source and ports.AdvancedSource.
// It wraps Project Discovery's subfinder CLI tool for subdomain discovery.
type SubfinderSource struct {
	*common.BaseCLISource // Embedded base for subprocess management

	threads   int
	rateLimit int
	sources   []string // Specific sources to use (-s flag)
	parser    *Parser
}

// New creates a new SubfinderSource with default configuration.
// Uses only free sources that don't require API keys for immediate results.
func New(logger logx.Logger) *SubfinderSource {
	return &SubfinderSource{
		BaseCLISource: common.NewBaseCLISource(logger, common.BaseCLIConfig{
			SourceName:     sourceName,
			ExecPath:       "subfinder",
			Timeout:        defaultTimeout,
			ProgressBuffer: 10,
		}),
		threads:   defaultThreads,
		rateLimit: 0, // No limit by default (subfinder manages this internally)
		sources:   []string{"alienvault", "anubis", "commoncrawl", "crtsh", "digitorus", "dnsdumpster", "hackertarget", "rapiddns", "sitedossier", "waybackarchive"},
		parser:    NewParser(logger, sourceName),
	}
}

// NewWithConfig creates SubfinderSource with custom configuration.
func NewWithConfig(logger logx.Logger, execPath string, timeout time.Duration, threads, rateLimit int, sources []string) *SubfinderSource {
	return &SubfinderSource{
		BaseCLISource: common.NewBaseCLISource(logger, common.BaseCLIConfig{
			SourceName:     sourceName,
			ExecPath:       execPath,
			Timeout:        timeout,
			ProgressBuffer: 10,
		}),
		threads:   threads,
		rateLimit: rateLimit,
		sources:   sources,
		parser:    NewParser(logger, sourceName),
	}
}

// Name returns the source name.
func (s *SubfinderSource) Name() string {
	return sourceName
}

// Mode returns the source operation mode (passive).
func (s *SubfinderSource) Mode() domain.SourceMode {
	return domain.SourceModePassive
}

// Type returns the source type (CLI).
func (s *SubfinderSource) Type() domain.SourceType {
	return domain.SourceTypeCLI
}

// Run executes subfinder against the target domain.
func (s *SubfinderSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	startTime := time.Now()

	s.GetLogger().Info("starting subfinder scan",
		"target", target.Root,
		"sources", s.sources,
		"threads", s.threads,
		"rate_limit", s.rateLimit,
	)

	// Build command arguments
	args := s.buildCommandArgs(target)

	// Create handler for processing output
	handler := &subfinderHandler{
		parser:    s.parser,
		target:    target,
		logger:    s.GetLogger(),
		responses: make([]*SubfinderResponse, 0, 100),
	}

	// Execute CLI with handler (BaseCLISource handles all subprocess logic)
	result, stderrOutput, err := s.ExecuteCLI(ctx, target, args, handler)

	// Handle fatal errors (e.g., failed to start process)
	if result == nil {
		return nil, fmt.Errorf("subfinder failed to start: %w", err)
	}

	// Handle stderr warnings
	if len(stderrOutput) > 0 {
		s.GetLogger().Debug("subfinder stderr", "output", stderrOutput)
		result.AddWarning("subfinder", fmt.Sprintf("stderr output: %s", stderrOutput))
	}

	// Handle errors (partial results tolerated)
	if err != nil {
		responseCount := len(handler.responses)
		if responseCount > 0 {
			s.GetLogger().Warn("subfinder exited with error but produced results",
				"error", err.Error(),
				"responses", responseCount,
			)
			result.AddWarning("subfinder", fmt.Sprintf("process exited with error: %v", err))
		} else {
			return nil, fmt.Errorf("subfinder failed: %w", err)
		}
	}

	// Parse responses into artifacts (after ExecuteCLI completes)
	artifacts := s.parser.ParseMultipleResponses(handler.responses, target)
	for _, artifact := range artifacts {
		result.AddArtifact(artifact)
	}

	// Log warning if responses were found but no artifacts created (filtered out)
	if len(handler.responses) > 0 && len(result.Artifacts) == 0 {
		s.GetLogger().Warn("subfinder found responses but all were filtered out",
			"target", target.Root,
			"responses", len(handler.responses),
			"reason", "likely out of scope or wildcards",
		)
		result.AddWarning("subfinder", fmt.Sprintf("found %d responses but all were filtered out (out of scope or wildcards)", len(handler.responses)))
	}

	// Log warning if no responses at all
	if len(handler.responses) == 0 {
		s.GetLogger().Warn("subfinder completed but found 0 responses",
			"target", target.Root,
			"sources", s.sources,
		)
		result.AddWarning("subfinder", "scan completed but no responses were found - target may have no exposed subdomains or sources are unavailable/rate-limited")
	}

	duration := time.Since(startTime)
	s.GetLogger().Info("subfinder scan completed",
		"target", target.Root,
		"duration", duration.String(),
		"responses", len(handler.responses),
		"artifacts", len(result.Artifacts),
	)

	return result, nil
}

// subfinderHandler implements common.OutputHandler for subfinder JSON output processing.
type subfinderHandler struct {
	parser    *Parser
	target    domain.Target
	logger    logx.Logger
	responses []*SubfinderResponse

	// State
	mu sync.Mutex
}

// ProcessLine handles each line of subfinder stdout (JSON lines).
func (h *subfinderHandler) ProcessLine(line []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var resp SubfinderResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		h.logger.Warn("failed to parse subfinder output", "line", string(line), "error", err.Error())
		return nil // Non-fatal, continue processing
	}

	h.responses = append(h.responses, &resp)

	h.logger.Debug("parsed subfinder response",
		"host", resp.Host,
		"sources", resp.Source,
	)

	return nil
}

// Finalize is called after all lines are processed.
func (h *subfinderHandler) Finalize() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.logger.Info("parsing responses to artifacts", "count", len(h.responses))

	// This is handled in Run() after ExecuteCLI returns
	// We don't populate result here because ExecuteCLI creates a new result
	// Instead, we store responses and let Run() handle artifact creation

	return nil
}

// Stream implements ports.StreamingSource.
func (s *SubfinderSource) Stream(ctx context.Context, target domain.Target) (<-chan *domain.Artifact, <-chan error) {
	return s.DefaultStream(ctx, target, s.Run)
}

// Initialize verifies that subfinder is installed and accessible.
// Implements ports.AdvancedSource.
func (s *SubfinderSource) Initialize() error {
	return s.DefaultInitialize(
		"subfinder",
		"https://github.com/projectdiscovery/subfinder",
	)
}

// Validate checks if the source configuration is valid.
// Implements ports.AdvancedSource.
func (s *SubfinderSource) Validate() error {
	// First check base validation
	if err := s.DefaultValidate(); err != nil {
		return err
	}

	// Additional subfinder-specific validation
	if s.threads <= 0 || s.threads > 1000 {
		return fmt.Errorf("threads must be between 1 and 1000")
	}

	if s.rateLimit < 0 {
		return fmt.Errorf("rate limit cannot be negative")
	}

	if len(s.sources) == 0 {
		return fmt.Errorf("sources list cannot be empty")
	}

	return nil
}

// HealthCheck verifies that subfinder is responsive.
// Implements ports.AdvancedSource.
func (s *SubfinderSource) HealthCheck(ctx context.Context) error {
	return s.DefaultHealthCheck(ctx)
}

// buildCommandArgs constructs the subfinder command arguments.
func (s *SubfinderSource) buildCommandArgs(target domain.Target) []string {
	args := []string{
		"-d", target.Root, // Target domain
		"-oJ",             // JSON output
		"-silent",         // No progress output
		"-nc",             // No color
	}

	// Add source selection flags
	if len(s.sources) > 0 {
		args = append(args, "-s", joinSources(s.sources))
	}

	// Add performance flags
	args = append(args, "-t", strconv.Itoa(s.threads))

	if s.rateLimit > 0 {
		args = append(args, "-rl", strconv.Itoa(s.rateLimit))
	}

	// Add timeout flag (in seconds)
	args = append(args, "-timeout", strconv.Itoa(int(s.GetTimeout().Seconds())))

	s.GetLogger().Debug("built subfinder command",
		"args", args,
		"timeout", s.GetTimeout().String(),
	)

	return args
}

// joinSources joins source names with commas for -s flag.
func joinSources(sources []string) string {
	result := ""
	for i, src := range sources {
		if i > 0 {
			result += ","
		}
		result += src
	}
	return result
}
