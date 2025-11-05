// Package dnsx implements integration with Project Discovery's dnsx DNS toolkit.
// It performs DNS resolution and enrichment of subdomains discovered in Stage 1.
package dnsx

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
	sourceName       = "dnsx"
	defaultTimeout   = 60 * time.Second
	defaultThreads   = 100
	defaultRateLimit = -1 // unlimited
)

// DNSXSource implements ports.Source for DNS resolution.
// It consumes subdomains from Stage 1 and validates them via DNS queries.
type DNSXSource struct {
	*common.BaseCLISource

	config DNSXConfig
	parser *Parser
}

// New creates a new DNSXSource with default configuration.
func New(logger logx.Logger) *DNSXSource {
	return &DNSXSource{
		BaseCLISource: common.NewBaseCLISource(logger, common.BaseCLIConfig{
			SourceName:     sourceName,
			ExecPath:       "dnsx",
			Timeout:        defaultTimeout,
			ProgressBuffer: 10,
		}),
		config: DefaultDNSXConfig(),
		parser: NewParser(logger, sourceName),
	}
}

// NewWithConfig creates DNSXSource with custom configuration.
func NewWithConfig(logger logx.Logger, config DNSXConfig) *DNSXSource {
	return &DNSXSource{
		BaseCLISource: common.NewBaseCLISource(logger, common.BaseCLIConfig{
			SourceName:     sourceName,
			ExecPath:       config.ExecPath,
			Timeout:        config.Timeout,
			ProgressBuffer: 10,
		}),
		config: config,
		parser: NewParser(logger, sourceName),
	}
}

// Name returns the source name.
func (d *DNSXSource) Name() string {
	return sourceName
}

// Mode returns the source operation mode (active).
func (d *DNSXSource) Mode() domain.SourceMode {
	return domain.SourceModeActive // DNS resolution is active
}

// Type returns the source type (CLI).
func (d *DNSXSource) Type() domain.SourceType {
	return domain.SourceTypeCLI
}

// Run executes dnsx against the root target (fallback when no input).
func (d *DNSXSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	// For Stage 2, we always expect input from Stage 1
	// If called directly, just resolve the root domain
	return d.RunWithInput(ctx, target, domain.NewScanResult(target))
}

// RunWithInput consumes subdomains from Stage 1 and resolves them via DNS.
// This is the primary execution mode for Stage 2.
func (d *DNSXSource) RunWithInput(ctx context.Context, target domain.Target, input *domain.ScanResult) (*domain.ScanResult, error) {
	result := domain.NewScanResult(target)
	startTime := time.Now()

	// Extract unique subdomains from Stage 1
	subdomains := d.extractSubdomains(input)

	// If no subdomains found, add root target
	if len(subdomains) == 0 {
		d.GetLogger().Warn("no subdomains from Stage 1, using root target")
		subdomains = []string{target.Root}
	}

	d.GetLogger().Info("starting dnsx DNS resolution",
		"target", target.Root,
		"input_subdomains", len(subdomains),
		"query_types", d.config.QueryTypes,
		"threads", d.config.Threads,
	)

	// Build command arguments
	args := d.buildCommandArgs()

	// Create handler using JSONLineHandler
	handler := common.NewJSONLineHandler[DNSXResponse](d.GetLogger(), target)

	// Execute dnsx with stdin using BaseCLISource abstraction
	_, stderrOutput, execErr := d.ExecuteCLIWithStdin(ctx, target, args, subdomains, handler)

	// Handle stderr warnings
	if len(stderrOutput) > 0 {
		d.GetLogger().Debug("dnsx stderr", "output", stderrOutput)
		result.AddWarning("dnsx", fmt.Sprintf("stderr output: %s", stderrOutput))
	}

	// Get parsed responses from handler
	responses := handler.GetResponses()

	// Convert []DNSXResponse to []*DNSXResponse for parser
	responsePtrs := make([]*DNSXResponse, len(responses))
	for i := range responses {
		responsePtrs[i] = &responses[i]
	}

	// Parse responses into artifacts (always, even with errors)
	artifacts := d.parser.ParseMultipleResponses(responsePtrs, target, input.Artifacts)
	for _, artifact := range artifacts {
		result.AddArtifact(artifact)
	}

	duration := time.Since(startTime)
	resolvedCount := len(responses)
	artifactCount := len(result.Artifacts)

	d.GetLogger().Info("dnsx DNS resolution completed",
		"target", target.Root,
		"duration", duration.String(),
		"input_subdomains", len(subdomains),
		"resolved", resolvedCount,
		"artifacts", artifactCount,
	)

	// Store statistics in metadata
	if result.Metadata.Environment == nil {
		result.Metadata.Environment = make(map[string]string)
	}
	result.Metadata.Environment["dnsx_input"] = fmt.Sprintf("%d", len(subdomains))
	result.Metadata.Environment["dnsx_resolved"] = fmt.Sprintf("%d", resolvedCount)
	result.Metadata.Environment["dnsx_artifacts"] = fmt.Sprintf("%d", artifactCount)

	// Handle execution errors (partial results are tolerated)
	if execErr != nil {
		if artifactCount > 0 {
			d.GetLogger().Warn("dnsx exited with error but produced partial results",
				"error", execErr.Error(),
				"resolved", resolvedCount,
				"artifacts", artifactCount,
			)
			result.AddWarning("dnsx", fmt.Sprintf("process exited with error: %v", execErr))
			return result, fmt.Errorf("dnsx completed with %d partial results: %w", artifactCount, execErr)
		}

		// No results: fatal error
		return nil, fmt.Errorf("dnsx failed without results: %w", execErr)
	}

	return result, nil
}

// extractSubdomains extracts unique subdomains from Stage 1 results.
func (d *DNSXSource) extractSubdomains(input *domain.ScanResult) []string {
	subdomainSet := make(map[string]bool)

	for _, artifact := range input.Artifacts {
		if artifact.Type == domain.ArtifactTypeSubdomain ||
			artifact.Type == domain.ArtifactTypeDomain {
			subdomainSet[artifact.Value] = true
		}
	}

	subdomains := make([]string, 0, len(subdomainSet))
	for subdomain := range subdomainSet {
		subdomains = append(subdomains, subdomain)
	}

	d.GetLogger().Debug("extracted subdomains from Stage 1",
		"unique_subdomains", len(subdomains),
	)

	return subdomains
}

// buildCommandArgs constructs dnsx command arguments.
func (d *DNSXSource) buildCommandArgs() []string {
	args := []string{
		"-json",     // JSON output
		"-silent",   // No banner
		"-no-color", // No ANSI colors
	}

	// Query types
	for _, qt := range d.config.QueryTypes {
		args = append(args, "-"+qt)
	}

	// Probes
	if d.config.EnableCDN {
		args = append(args, "-cdn")
	}
	if d.config.EnableASN {
		args = append(args, "-asn")
	}

	// Performance
	args = append(args,
		"-t", strconv.Itoa(d.config.Threads),
		"-retry", "2",
	)

	if d.config.RateLimit > 0 {
		args = append(args, "-rl", strconv.Itoa(d.config.RateLimit))
	}

	// Response filtering
	args = append(args,
		"-re",            // Include response
		"-rc", "noerror", // Only successful resolutions
	)

	// Wildcard filtering
	if d.config.WildcardFilter {
		args = append(args, "-wt", "5") // Wildcard threshold
	}

	d.GetLogger().Debug("built dnsx command",
		"args", args,
		"timeout", d.GetTimeout().String(),
	)

	return args
}

// Stream implements ports.StreamingSource.
func (d *DNSXSource) Stream(ctx context.Context, target domain.Target) (<-chan *domain.Artifact, <-chan error) {
	return d.DefaultStream(ctx, target, d.Run)
}

// Initialize verifies that dnsx is installed and accessible.
func (d *DNSXSource) Initialize() error {
	return d.DefaultInitialize(
		"dnsx",
		"go install github.com/projectdiscovery/dnsx/cmd/dnsx@latest",
	)
}

// Validate checks if the source configuration is valid.
func (d *DNSXSource) Validate() error {
	// Base validation
	if err := d.DefaultValidate(); err != nil {
		return err
	}

	// dnsx-specific validation
	if d.config.Threads <= 0 || d.config.Threads > 1000 {
		return fmt.Errorf("threads must be between 1 and 1000")
	}

	if d.config.RateLimit < -1 {
		return fmt.Errorf("rate limit must be -1 (unlimited) or positive")
	}

	if len(d.config.QueryTypes) == 0 {
		return fmt.Errorf("at least one query type must be specified")
	}

	return nil
}

// HealthCheck verifies that dnsx is responsive.
func (d *DNSXSource) HealthCheck(ctx context.Context) error {
	return d.DefaultHealthCheck(ctx)
}

