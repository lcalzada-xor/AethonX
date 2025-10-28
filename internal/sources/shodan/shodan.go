// Package shodan implements integration with Shodan, the search engine for Internet-connected devices.
// It provides both API-based and CLI-based reconnaissance capabilities.
package shodan

import (
	"context"
	"fmt"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
)

const (
	sourceName       = "shodan"
	defaultTimeout   = 60 * time.Second
	defaultRateLimit = 1.0 // 1 req/s (free tier)
)

// ShodanSource implements ports.Source and ports.AdvancedSource.
// It provides passive reconnaissance by querying Shodan's database of internet-facing devices.
type ShodanSource struct {
	apiClient *ShodanAPIClient
	cliExec   *ShodanCLIExecutor
	parser    *Parser
	logger    logx.Logger
	useCLI    bool
	apiKey    string
}

// New creates a new ShodanSource with default configuration.
// By default, it uses API mode (requires API key).
func New(logger logx.Logger) *ShodanSource {
	return NewWithConfig(logger, "", false, defaultTimeout, defaultRateLimit)
}

// NewWithConfig creates a ShodanSource with custom configuration.
func NewWithConfig(logger logx.Logger, apiKey string, useCLI bool, timeout time.Duration, rateLimit float64) *ShodanSource {
	src := &ShodanSource{
		parser: NewParser(logger, sourceName),
		logger: logger.With("source", sourceName),
		useCLI: useCLI,
		apiKey: apiKey,
	}

	if useCLI {
		src.cliExec = NewCLIExecutor(logger)
		src.logger.Info("shodan source initialized in CLI mode")
	} else {
		if apiKey != "" {
			src.apiClient = NewAPIClientWithConfig(apiKey, logger, rateLimit, timeout)
			src.logger.Info("shodan source initialized in API mode")
		} else {
			src.logger.Warn("shodan API key not provided, source may fail during execution")
		}
	}

	return src
}

// Name returns the source name.
func (s *ShodanSource) Name() string {
	return sourceName
}

// Mode returns the source operation mode (passive).
// Shodan is always passive because it queries pre-indexed data.
func (s *ShodanSource) Mode() domain.SourceMode {
	return domain.SourceModePassive
}

// Type returns the source type (API or CLI depending on configuration).
func (s *ShodanSource) Type() domain.SourceType {
	if s.useCLI {
		return domain.SourceTypeCLI
	}
	return domain.SourceTypeAPI
}

// Run executes the Shodan reconnaissance against the target.
// It discovers subdomains, IPs, services, vulnerabilities, and more.
func (s *ShodanSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	startTime := time.Now()

	s.logger.Info("starting shodan scan",
		"target", target.Root,
		"mode", s.Type(),
	)

	result := domain.NewScanResult(target)
	result.Metadata.SourcesUsed = []string{s.Name()}

	// Execute appropriate mode
	var artifacts []*domain.Artifact
	var err error

	if s.useCLI {
		artifacts, err = s.runCLIMode(ctx, target, result)
	} else {
		artifacts, err = s.runAPIMode(ctx, target, result)
	}

	// Handle errors (tolerate partial results)
	if err != nil {
		result.AddError(s.Name(), err.Error(), false)
		s.logger.Warn("shodan scan completed with errors",
			"error", err.Error(),
			"artifacts", len(artifacts),
		)
	}

	// Add artifacts to result
	for _, artifact := range artifacts {
		result.AddArtifact(artifact)
	}

	duration := time.Since(startTime)
	s.logger.Info("shodan scan completed",
		"target", target.Root,
		"duration", duration.String(),
		"artifacts", len(artifacts),
	)

	return result, nil
}

// runAPIMode executes reconnaissance using Shodan REST API.
func (s *ShodanSource) runAPIMode(ctx context.Context, target domain.Target, result *domain.ScanResult) ([]*domain.Artifact, error) {
	if s.apiClient == nil {
		return nil, fmt.Errorf("API client not initialized (API key required)")
	}

	artifacts := make([]*domain.Artifact, 0)

	// Step 1: Discover subdomains via /dns/domain/{domain}
	s.logger.Debug("fetching subdomains via DNS API", "target", target.Root)
	domainRecords, err := s.apiClient.GetDomainInfo(ctx, target.Root)
	if err != nil {
		s.logger.Warn("failed to fetch domain info",
			"error", err.Error(),
			"note", "This endpoint requires Shodan Membership plan",
		)
		result.AddWarning(s.Name(), fmt.Sprintf("DNS domain enumeration failed: %v", err))
	} else {
		s.logger.Info("discovered subdomains from DNS API", "count", len(domainRecords))
		for _, record := range domainRecords {
			artifact := s.parser.ParseDomainResponse(&record, target)
			if artifact != nil {
				artifacts = append(artifacts, artifact)
			}
		}
	}

	// Step 2: Search for hosts via /shodan/host/search?query=hostname:example.com
	s.logger.Debug("searching hosts via search API", "target", target.Root)
	query := fmt.Sprintf("hostname:%s", target.Root)
	hostResults, err := s.apiClient.SearchHosts(ctx, query)
	if err != nil {
		s.logger.Warn("failed to search hosts", "error", err.Error())
		result.AddWarning(s.Name(), fmt.Sprintf("Host search failed: %v", err))
	} else {
		s.logger.Info("discovered hosts from search API", "count", len(hostResults))
		for _, hostResp := range hostResults {
			hostArtifacts := s.parser.ParseHostResponse(&hostResp, target)
			artifacts = append(artifacts, hostArtifacts...)
		}
	}

	// Step 3: Additional search by organization (if we discovered org info)
	// This is optional and can discover related infrastructure
	if len(hostResults) > 0 && hostResults[0].Org != "" {
		org := hostResults[0].Org
		s.logger.Debug("searching by organization", "org", org)

		orgQuery := fmt.Sprintf("org:\"%s\" hostname:%s", org, target.Root)
		orgResults, err := s.apiClient.SearchHosts(ctx, orgQuery)
		if err == nil && len(orgResults) > 0 {
			s.logger.Info("discovered additional hosts by org", "count", len(orgResults))
			for _, hostResp := range orgResults {
				hostArtifacts := s.parser.ParseHostResponse(&hostResp, target)
				artifacts = append(artifacts, hostArtifacts...)
			}
		}
	}

	return artifacts, nil
}

// runCLIMode executes reconnaissance using Shodan CLI tool.
func (s *ShodanSource) runCLIMode(ctx context.Context, target domain.Target, result *domain.ScanResult) ([]*domain.Artifact, error) {
	if s.cliExec == nil {
		return nil, fmt.Errorf("CLI executor not initialized")
	}

	artifacts := make([]*domain.Artifact, 0)

	// Execute: shodan domain {target.Root}
	s.logger.Debug("executing shodan domain command", "target", target.Root)
	domainArtifacts, err := s.cliExec.RunDomainSearch(ctx, target)
	if err != nil {
		s.logger.Warn("shodan domain command failed",
			"error", err.Error(),
			"note", "This command requires Shodan Membership plan and API key initialization",
		)
		result.AddWarning(s.Name(), fmt.Sprintf("Domain search failed: %v", err))
	} else {
		s.logger.Info("discovered subdomains from CLI", "count", len(domainArtifacts))
		artifacts = append(artifacts, domainArtifacts...)
	}

	// Execute: shodan search hostname:{target.Root}
	s.logger.Debug("executing shodan search command", "target", target.Root)
	query := fmt.Sprintf("hostname:%s", target.Root)
	searchArtifacts, err := s.cliExec.RunSearch(ctx, query, target)
	if err != nil {
		s.logger.Warn("shodan search command failed", "error", err.Error())
		result.AddWarning(s.Name(), fmt.Sprintf("Host search failed: %v", err))
	} else {
		s.logger.Info("discovered hosts from CLI search", "count", len(searchArtifacts))
		artifacts = append(artifacts, searchArtifacts...)
	}

	return artifacts, nil
}

// Close releases any resources held by the source.
func (s *ShodanSource) Close() error {
	s.logger.Debug("closing shodan source")
	return nil
}

// Initialize verifies that the source is properly configured.
// Implements ports.AdvancedSource.
func (s *ShodanSource) Initialize() error {
	s.logger.Debug("initializing shodan source")

	if s.useCLI {
		// Verify CLI tool is installed
		return s.cliExec.DefaultInitialize("shodan", "https://cli.shodan.io")
	}

	// Verify API key is configured
	if s.apiKey == "" {
		return fmt.Errorf("shodan API key is required (set via --src.shodan.api_key or AETHONX_SRC_SHODAN_API_KEY)")
	}

	s.logger.Info("shodan source initialized successfully")
	return nil
}

// Validate checks if the source configuration is valid.
// Implements ports.AdvancedSource.
func (s *ShodanSource) Validate() error {
	s.logger.Debug("validating shodan source")

	if s.useCLI {
		return s.cliExec.DefaultValidate()
	}

	if s.apiClient == nil {
		return fmt.Errorf("API client not initialized")
	}

	return nil
}

// HealthCheck verifies that the Shodan service is accessible.
// Implements ports.AdvancedSource.
func (s *ShodanSource) HealthCheck(ctx context.Context) error {
	s.logger.Debug("running health check")

	if s.useCLI {
		return s.cliExec.DefaultHealthCheck(ctx)
	}

	if s.apiClient == nil {
		return fmt.Errorf("API client not initialized")
	}

	// Validate API key by fetching account info
	if err := s.apiClient.ValidateAPIKey(ctx); err != nil {
		return fmt.Errorf("API health check failed: %w", err)
	}

	s.logger.Info("health check passed")
	return nil
}

// Stream implements ports.StreamingSource for real-time artifact emission.
// This delegates to the default stream implementation.
func (s *ShodanSource) Stream(ctx context.Context, target domain.Target) (<-chan *domain.Artifact, <-chan error) {
	// Use BaseCLISource's DefaultStream helper
	if s.useCLI && s.cliExec != nil {
		return s.cliExec.DefaultStream(ctx, target, s.Run)
	}

	// For API mode, create channels and run in goroutine
	artifactCh := make(chan *domain.Artifact, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(artifactCh)
		defer close(errCh)

		result, err := s.Run(ctx, target)
		if err != nil {
			errCh <- err
			return
		}

		// Stream artifacts
		for _, artifact := range result.Artifacts {
			select {
			case artifactCh <- artifact:
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}
		}
	}()

	return artifactCh, errCh
}
