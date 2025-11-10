// Package ipapi provides IP geolocation and network information enrichment via ip-api.com.
// It enriches IP artifacts with country, city, coordinates, timezone, ASN, ISP, and cloud provider detection.
package ipapi

import (
	"context"
	"fmt"
	"net"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
)

const (
	sourceName     = "ipapi"
	defaultTimeout = 30 * time.Second
)

// IPAPISource implements ports.Source for IP geolocation enrichment.
// It queries ip-api.com (free tier: 45 req/min) to enrich IP artifacts with:
// - Geolocation: country, city, coordinates, timezone
// - Network: ASN, ISP, organization
// - Cloud provider detection: AWS, GCP, Azure, etc.
type IPAPISource struct {
	client *IPAPIClient
	parser *Parser
	logger logx.Logger
}

// New creates a new IPAPISource with default configuration.
func New(logger logx.Logger) *IPAPISource {
	return NewWithTimeout(logger, defaultTimeout)
}

// NewWithTimeout creates a new IPAPISource with custom timeout.
func NewWithTimeout(logger logx.Logger, timeout time.Duration) *IPAPISource {
	return &IPAPISource{
		client: NewIPAPIClient(logger, timeout),
		parser: NewParser(logger, sourceName),
		logger: logger.With("source", sourceName),
	}
}

// Name returns the source name.
func (s *IPAPISource) Name() string {
	return sourceName
}

// Mode returns the source operation mode (passive).
// ipapi only queries external APIs without directly contacting the target.
func (s *IPAPISource) Mode() domain.SourceMode {
	return domain.SourceModePassive
}

// Type returns the source type (API).
func (s *IPAPISource) Type() domain.SourceType {
	return domain.SourceTypeAPI
}

// Run executes IP geolocation enrichment for the target.
// It resolves the target domain to IPs and queries ip-api.com for each unique IP.
// Private IPs are filtered out before querying.
func (s *IPAPISource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	startTime := time.Now()

	s.logger.Info("starting ipapi scan", "target", target.Root)

	result := domain.NewScanResult(target)
	result.Metadata.SourcesUsed = []string{s.Name()}

	// Collect unique IPs from target resolution
	ips := s.resolveTargetIPs(ctx, target.Root)

	if len(ips) == 0 {
		s.logger.Warn("no IPs resolved for target", "target", target.Root)
		return result, nil
	}

	s.logger.Info("resolved IPs", "count", len(ips), "ips", ips)

	// Enrich each IP with geolocation data
	for _, ip := range ips {
		// Skip private IPs (ip-api.com rejects them)
		if IsPrivateIP(ip) {
			s.logger.Debug("skipping private IP", "ip", ip)
			continue
		}

		// Query ip-api.com
		resp, err := s.client.Lookup(ctx, ip)
		if err != nil {
			s.logger.Warn("failed to lookup IP", "ip", ip, "error", err.Error())
			result.AddError(s.Name(), fmt.Sprintf("lookup failed for %s: %v", ip, err), false)
			continue
		}

		// Parse response to IPMetadata
		ipMeta := s.parser.ParseToIPMetadata(resp)

		// Create IP artifact with metadata
		artifact := domain.NewArtifactWithMetadata(
			domain.ArtifactTypeIP,
			ip,
			s.Name(),
			ipMeta,
		)

		// Add enrichment tags
		artifact.AddTag("enriched")
		if ipMeta.CloudProvider != "" {
			artifact.AddTag(fmt.Sprintf("cloud:%s", ipMeta.CloudProvider))
		}
		if ipMeta.CountryCode != "" {
			artifact.AddTag(fmt.Sprintf("country:%s", ipMeta.CountryCode))
		}
		if ipMeta.IPType != "" {
			artifact.AddTag(fmt.Sprintf("type:%s", ipMeta.IPType))
		}

		result.AddArtifact(artifact)
	}

	duration := time.Since(startTime)
	s.logger.Info("ipapi scan completed",
		"target", target.Root,
		"duration", duration.String(),
		"artifacts", len(result.Artifacts),
	)

	return result, nil
}

// Close implements ports.Source.Close (no cleanup needed for HTTP client).
func (s *IPAPISource) Close() error {
	s.logger.Debug("closing ipapi source")
	return nil
}

// resolveTargetIPs resolves a domain to its IP addresses.
// Returns a deduplicated list of IPs (both IPv4 and IPv6).
func (s *IPAPISource) resolveTargetIPs(ctx context.Context, domain string) []string {
	ips := make(map[string]bool)

	// Use context-aware DNS resolver
	resolver := &net.Resolver{
		PreferGo: true,
	}

	// Resolve IPv4 addresses
	ipv4s, err := resolver.LookupIP(ctx, "ip4", domain)
	if err != nil {
		s.logger.Warn("failed to resolve IPv4", "domain", domain, "error", err.Error())
	} else {
		for _, ip := range ipv4s {
			ips[ip.String()] = true
		}
	}

	// Resolve IPv6 addresses (optional, may timeout)
	ipv6s, err := resolver.LookupIP(ctx, "ip6", domain)
	if err != nil {
		// IPv6 failures are common, don't log as warning
		s.logger.Debug("no IPv6 addresses found", "domain", domain)
	} else {
		for _, ip := range ipv6s {
			ips[ip.String()] = true
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(ips))
	for ip := range ips {
		result = append(result, ip)
	}

	return result
}
