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
	apiClient        *ShodanAPIClient
	internetDBClient *InternetDBClient // No API key required
	dnsResolver      *DNSResolver      // NEW: Free DNS fallback chain (Cloudflare → Google → System)
	cliExec          *ShodanCLIExecutor
	parser           *Parser
	logger           logx.Logger
	useCLI           bool
	apiKey           string
	useInternetDB    bool // Enable/disable InternetDB
}

// New creates a new ShodanSource with default configuration.
// By default, it uses API mode (requires API key).
func New(logger logx.Logger) *ShodanSource {
	return NewWithConfig(logger, "", false, defaultTimeout, defaultRateLimit)
}

// NewWithConfig creates a ShodanSource with custom configuration.
func NewWithConfig(logger logx.Logger, apiKey string, useCLI bool, timeout time.Duration, rateLimit float64) *ShodanSource {
	return NewWithFullConfig(logger, apiKey, useCLI, true, timeout, rateLimit)
}

// NewWithFullConfig creates a ShodanSource with all configuration options.
func NewWithFullConfig(logger logx.Logger, apiKey string, useCLI bool, useInternetDB bool, timeout time.Duration, rateLimit float64) *ShodanSource {
	src := &ShodanSource{
		parser:        NewParser(logger, sourceName),
		logger:        logger.With("source", sourceName),
		useCLI:        useCLI,
		apiKey:        apiKey,
		useInternetDB: useInternetDB,
	}

	// Initialize InternetDB client (always available, no API key needed)
	if useInternetDB {
		src.internetDBClient = NewInternetDBClient(logger)
	}

	// Initialize DNS resolver (always available, free fallback chain)
	src.dnsResolver = NewDNSResolver(logger)

	if useCLI {
		src.cliExec = NewCLIExecutor(logger)
		src.logger.Info("shodan source initialized in CLI mode")
	} else {
		if apiKey != "" {
			src.apiClient = NewAPIClientWithConfig(apiKey, logger, rateLimit, timeout)
			src.logger.Info("shodan source initialized in API mode with free DNS fallback")
		} else {
			src.logger.Warn("shodan API key not provided, using InternetDB and free DNS resolver")
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
// Attempts to execute ALL available endpoints (free + paid).
// Graceful degradation: failures are logged as warnings, execution continues.
func (s *ShodanSource) runAPIMode(ctx context.Context, target domain.Target, result *domain.ScanResult) ([]*domain.Artifact, error) {
	artifacts := make([]*domain.Artifact, 0)

	// Track discovered IPs for enrichment
	discoveredIPs := make(map[string]bool)

	// =================================================================
	// PAID ENDPOINTS (attempt, tolerate failures)
	// =================================================================

	if s.apiClient != nil {
		// Endpoint 1: /dns/domain/{domain} (PAID - Membership required)
		s.logger.Debug("attempting /dns/domain (paid endpoint)", "target", target.Root)
		domainRecords, err := s.apiClient.GetDomainInfo(ctx, target.Root)
		if err != nil {
			s.logger.Warn("paid endpoint failed: /dns/domain",
				"error", err.Error(),
				"note", "Requires Shodan Membership plan",
			)
			result.AddWarning(s.Name(), fmt.Sprintf("[PAID] DNS domain enumeration unavailable: %v", err))
		} else {
			s.logger.Info("✓ /dns/domain successful", "count", len(domainRecords))
			for _, record := range domainRecords {
				artifact := s.parser.ParseDomainResponse(&record, target)
				if artifact != nil {
					artifacts = append(artifacts, artifact)
				}
			}
		}

		// Endpoint 2: /shodan/host/search (PAID - Consumes query credits)
		s.logger.Debug("attempting /shodan/host/search (paid endpoint)", "target", target.Root)
		query := fmt.Sprintf("hostname:%s", target.Root)
		hostResults, err := s.apiClient.SearchHosts(ctx, query)
		if err != nil {
			s.logger.Warn("paid endpoint failed: /shodan/host/search",
				"error", err.Error(),
				"note", "Consumes query credits (free tier has 0 credits)",
			)
			result.AddWarning(s.Name(), fmt.Sprintf("[PAID] Host search unavailable: %v", err))
		} else {
			s.logger.Info("✓ /shodan/host/search successful", "count", len(hostResults))
			for _, hostResp := range hostResults {
				hostArtifacts := s.parser.ParseHostResponse(&hostResp, target)
				artifacts = append(artifacts, hostArtifacts...)

				// Track IPs for enrichment
				if hostResp.IPStr != "" {
					discoveredIPs[hostResp.IPStr] = true
				}
			}

			// Endpoint 3: Search by organization (if discovered)
			if len(hostResults) > 0 && hostResults[0].Org != "" {
				org := hostResults[0].Org
				s.logger.Debug("attempting org search (paid)", "org", org)

				orgQuery := fmt.Sprintf("org:\"%s\" hostname:%s", org, target.Root)
				orgResults, err := s.apiClient.SearchHosts(ctx, orgQuery)
				if err != nil {
					s.logger.Warn("org search failed", "error", err.Error())
				} else {
					s.logger.Info("✓ org search successful", "count", len(orgResults))
					for _, hostResp := range orgResults {
						hostArtifacts := s.parser.ParseHostResponse(&hostResp, target)
						artifacts = append(artifacts, hostArtifacts...)

						if hostResp.IPStr != "" {
							discoveredIPs[hostResp.IPStr] = true
						}
					}
				}
			}
		}
	}

	// =================================================================
	// FREE DNS RESOLUTION (Cloudflare → Google → System fallback)
	// =================================================================

	// DNS Resolution: Resolve target domain to IPs
	s.logger.Debug("attempting DNS resolution via free fallback chain", "target", target.Root)
	hostnames := []string{target.Root, "www." + target.Root}
	ipMap := make(map[string]string)

	for _, hostname := range hostnames {
		ips, err := s.dnsResolver.ResolveWithFallback(ctx, hostname)
		if err != nil {
			s.logger.Warn("DNS resolution failed for hostname", "hostname", hostname, "error", err.Error())
			continue
		}
		if len(ips) > 0 {
			ipMap[hostname] = ips[0] // Take first IP
			s.logger.Info("✓ DNS resolved", "hostname", hostname, "ip", ips[0])

			// Create IP artifact
			artifact := s.parser.ParseIPFromDNS(ips[0], hostname, target)
			if artifact != nil {
				artifacts = append(artifacts, artifact)
				discoveredIPs[ips[0]] = true
			}
		}
	}

	// Reverse DNS: Lookup hostnames for discovered IPs
	if len(discoveredIPs) > 0 {
		s.logger.Debug("attempting reverse DNS via free fallback chain", "ips", len(discoveredIPs))

		for ip := range discoveredIPs {
			hostnames, err := s.dnsResolver.ReverseLookupWithFallback(ctx, ip)
			if err != nil {
				s.logger.Debug("reverse DNS failed for IP", "ip", ip, "error", err.Error())
				continue
			}

			if len(hostnames) > 0 {
				s.logger.Info("✓ reverse DNS resolved", "ip", ip, "hostnames", len(hostnames))
				for _, hostname := range hostnames {
					artifact := s.parser.ParseSubdomainFromReverse(hostname, ip, target)
					if artifact != nil {
						artifacts = append(artifacts, artifact)
					}
				}
			}
		}
	}

	// =================================================================
	// FREE SHODAN API ENDPOINTS (with API key)
	// =================================================================

	if s.apiClient != nil {

		// Endpoint 6: /shodan/host/count (FREE)
		s.logger.Debug("attempting /shodan/host/count (free)", "target", target.Root)
		query := fmt.Sprintf("hostname:%s", target.Root)
		facets := []string{"org", "country", "port", "product", "asn", "domain"}
		countResp, err := s.apiClient.GetHostCount(ctx, query, facets)
		if err != nil {
			s.logger.Warn("free endpoint failed: /shodan/host/count", "error", err.Error())
			result.AddWarning(s.Name(), fmt.Sprintf("[FREE] Host count failed: %v", err))
		} else {
			s.logger.Info("✓ /shodan/host/count successful",
				"total_results", countResp.Total,
				"facets", len(countResp.Facets),
			)
			// Intelligence data logged for analysis, not added as artifacts
		}

		// Endpoint 7: /api-info (FREE - informational)
		s.logger.Debug("attempting /api-info (free)")
		apiInfo, err := s.apiClient.GetAPIInfo(ctx)
		if err != nil {
			s.logger.Warn("free endpoint failed: /api-info", "error", err.Error())
		} else {
			s.logger.Info("✓ /api-info successful", "plan", apiInfo["plan"])
			// API plan info logged for analysis
		}
	}

	// =================================================================
	// INTERNETDB (FREE - No API key required)
	// =================================================================

	if s.useInternetDB && s.internetDBClient != nil && len(discoveredIPs) > 0 {
		s.logger.Debug("attempting InternetDB enrichment (free, no auth)", "ips", len(discoveredIPs))

		ips := make([]string, 0, len(discoveredIPs))
		for ip := range discoveredIPs {
			ips = append(ips, ip)
		}

		idbResults, err := s.internetDBClient.LookupBatch(ctx, ips)
		if err != nil {
			s.logger.Warn("InternetDB batch lookup failed", "error", err.Error())
			result.AddWarning(s.Name(), fmt.Sprintf("[FREE] InternetDB enrichment failed: %v", err))
		} else {
			s.logger.Info("✓ InternetDB successful", "enriched_ips", len(idbResults))
			for _, idbResp := range idbResults {
				idbArtifacts := s.parser.ParseInternetDBResponse(idbResp, target)
				artifacts = append(artifacts, idbArtifacts...)
			}
		}
	}

	s.logger.Info("shodan API execution completed",
		"total_artifacts", len(artifacts),
		"unique_ips_discovered", len(discoveredIPs),
	)

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
