// Package abuseipdb provides IP reputation and threat intelligence enrichment via AbuseIPDB.com API.
// It enriches IP artifacts with abuse confidence scores, blacklist status, and abuse reporting data.
package abuseipdb

import (
	"context"
	"fmt"
	"net"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
)

const (
	sourceName = "abuseipdb"
)

// AbuseIPDBSource implements ports.Source for IP reputation enrichment.
// It queries abuseipdb.com to enrich IP artifacts with:
// - Threat intelligence: abuse confidence score (0-100)
// - Reputation categories: clean, low, medium, high, critical
// - Blacklist/whitelist status
// - Abuse reporting metrics: total reports, last reported date
type AbuseIPDBSource struct {
	client *AbuseIPDBClient
	parser *Parser
	logger logx.Logger
	apiKey string // Required for AbuseIPDB API
}

// New creates a new AbuseIPDBSource with default configuration.
// Requires a valid API key (free tier: 1000 requests/day).
// Get your API key at: https://www.abuseipdb.com/api
func New(logger logx.Logger, apiKey string) *AbuseIPDBSource {
	return NewWithTimeout(logger, apiKey, 10*time.Second)
}

// NewWithTimeout creates a new AbuseIPDBSource with custom timeout.
func NewWithTimeout(logger logx.Logger, apiKey string, timeout time.Duration) *AbuseIPDBSource {
	return &AbuseIPDBSource{
		client: NewAbuseIPDBClient(logger, apiKey, timeout),
		parser: NewParser(),
		logger: logger.With("source", sourceName),
		apiKey: apiKey,
	}
}

// Name returns the source name.
func (s *AbuseIPDBSource) Name() string {
	return sourceName
}

// Mode returns the source operation mode (passive).
// abuseipdb only queries external APIs without directly contacting the target.
func (s *AbuseIPDBSource) Mode() domain.SourceMode {
	return domain.SourceModePassive
}

// Type returns the source type (API).
func (s *AbuseIPDBSource) Type() domain.SourceType {
	return domain.SourceTypeAPI
}

// Run executes IP reputation enrichment for the target.
// It resolves the target domain to IPs and queries abuseipdb.com for each unique IP.
// Private IPs and reserved IPs are filtered out before querying.
func (s *AbuseIPDBSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	startTime := time.Now()

	s.logger.Info("starting abuseipdb scan", "target", target.Root)

	result := domain.NewScanResult(target)
	result.Metadata.SourcesUsed = []string{s.Name()}

	// Collect unique IPs from target resolution
	ips := s.resolveTargetIPs(ctx, target.Root)

	if len(ips) == 0 {
		s.logger.Warn("no IPs resolved for target", "target", target.Root)
		return result, nil
	}

	s.logger.Info("resolved IPs", "count", len(ips), "ips", ips)

	// Enrich each IP with reputation data
	enrichedCount := 0
	for _, ip := range ips {
		// Skip private IPs (abuseipdb.com rejects them)
		if IsPrivateIP(ip) {
			s.logger.Debug("skipping private IP", "ip", ip)
			continue
		}

		// Query abuseipdb.com
		resp, err := s.client.CheckIP(ctx, ip)
		if err != nil {
			s.logger.Warn("failed to check IP", "ip", ip, "error", err.Error())
			result.AddError(s.Name(), fmt.Sprintf("check failed for %s: %v", ip, err), false)
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

		// Add threat intelligence tags
		artifact.AddTag("threat-intel")
		artifact.AddTag(fmt.Sprintf("reputation:%s", ipMeta.Reputation))

		if ipMeta.Blacklisted {
			artifact.AddTag("blacklisted")
		}
		if resp.Data.IsWhitelisted {
			artifact.AddTag("whitelisted")
		}
		if resp.Data.TotalReports > 0 {
			artifact.AddTag("abuse-reports")
		}
		if ipMeta.ThreatScore >= 75 {
			artifact.AddTag("high-threat")
		}

		result.AddArtifact(artifact)
		enrichedCount++
	}

	duration := time.Since(startTime)
	s.logger.Info("abuseipdb scan completed",
		"target", target.Root,
		"duration", duration.String(),
		"ips_checked", len(ips),
		"artifacts", enrichedCount,
	)

	return result, nil
}

// Close implements ports.Source.Close.
func (s *AbuseIPDBSource) Close() error {
	s.logger.Debug("closing abuseipdb source")
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// resolveTargetIPs resolves a domain to its IP addresses.
// Returns a deduplicated list of IPs (both IPv4 and IPv6).
func (s *AbuseIPDBSource) resolveTargetIPs(ctx context.Context, domain string) []string {
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

// IsPrivateIP checks if an IP address is private (RFC 1918, RFC 4193, etc.).
// Private IPs cannot be queried on AbuseIPDB.
func IsPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// Check for private IPv4 ranges (RFC 1918)
	privateIPv4Ranges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8", // Loopback
	}

	for _, cidr := range privateIPv4Ranges {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return true
		}
	}

	// Check for private IPv6 ranges (RFC 4193, loopback)
	if ip.To4() == nil {
		// IPv6
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return true
		}
		// FC00::/7 (Unique Local Addresses)
		_, ulaNet, _ := net.ParseCIDR("fc00::/7")
		if ulaNet.Contains(ip) {
			return true
		}
	}

	return false
}
