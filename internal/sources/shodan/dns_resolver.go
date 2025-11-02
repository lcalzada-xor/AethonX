// internal/sources/shodan/dns_resolver.go
package shodan

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"aethonx/internal/platform/httpclient"
	"aethonx/internal/platform/logx"
)

// DNSResolver provides DNS resolution with automatic fallback to multiple providers.
// Fallback chain: Cloudflare DNS → Google DNS → System DNS
type DNSResolver struct {
	cloudflare *CloudflareDNS
	google     *GoogleDNS
	system     *SystemDNS
	logger     logx.Logger
}

// NewDNSResolver creates a new DNS resolver with fallback chain.
func NewDNSResolver(logger logx.Logger) *DNSResolver {
	httpConfig := httpclient.Config{
		Timeout:         10 * time.Second,
		MaxRetries:      2,
		RetryBackoff:    500 * time.Millisecond,
		MaxRetryBackoff: 2 * time.Second,
		UserAgent:       "AethonX/1.0",
		RateLimit:       0, // No rate limit for DNS queries
	}

	client := httpclient.New(httpConfig, logger)

	return &DNSResolver{
		cloudflare: &CloudflareDNS{client: client, logger: logger.With("dns", "cloudflare")},
		google:     &GoogleDNS{client: client, logger: logger.With("dns", "google")},
		system:     &SystemDNS{logger: logger.With("dns", "system")},
		logger:     logger.With("component", "dns-resolver"),
	}
}

// ResolveWithFallback resolves a hostname to IP addresses using fallback chain.
// Returns the first successful result from: Cloudflare → Google → System DNS.
func (r *DNSResolver) ResolveWithFallback(ctx context.Context, hostname string) ([]string, error) {
	// Try Cloudflare DNS first
	ips, err := r.cloudflare.Resolve(ctx, hostname)
	if err == nil && len(ips) > 0 {
		r.logger.Debug("resolved via Cloudflare DNS", "hostname", hostname, "ips", len(ips))
		return ips, nil
	}
	if err != nil {
		r.logger.Debug("Cloudflare DNS failed, trying Google DNS", "hostname", hostname, "error", err.Error())
	}

	// Fallback to Google DNS
	ips, err = r.google.Resolve(ctx, hostname)
	if err == nil && len(ips) > 0 {
		r.logger.Debug("resolved via Google DNS", "hostname", hostname, "ips", len(ips))
		return ips, nil
	}
	if err != nil {
		r.logger.Debug("Google DNS failed, trying system DNS", "hostname", hostname, "error", err.Error())
	}

	// Final fallback to system DNS
	ips, err = r.system.Resolve(ctx, hostname)
	if err == nil && len(ips) > 0 {
		r.logger.Debug("resolved via system DNS", "hostname", hostname, "ips", len(ips))
		return ips, nil
	}

	return nil, fmt.Errorf("all DNS providers failed for hostname %s: %w", hostname, err)
}

// ReverseLookupWithFallback performs reverse DNS lookup using fallback chain.
// Returns the first successful result from: Cloudflare → Google → System DNS.
func (r *DNSResolver) ReverseLookupWithFallback(ctx context.Context, ip string) ([]string, error) {
	// Try Cloudflare DNS first
	hostnames, err := r.cloudflare.ResolvePtr(ctx, ip)
	if err == nil && len(hostnames) > 0 {
		r.logger.Debug("reverse lookup via Cloudflare DNS", "ip", ip, "hostnames", len(hostnames))
		return hostnames, nil
	}
	if err != nil {
		r.logger.Debug("Cloudflare PTR failed, trying Google DNS", "ip", ip, "error", err.Error())
	}

	// Fallback to Google DNS
	hostnames, err = r.google.ResolvePtr(ctx, ip)
	if err == nil && len(hostnames) > 0 {
		r.logger.Debug("reverse lookup via Google DNS", "ip", ip, "hostnames", len(hostnames))
		return hostnames, nil
	}
	if err != nil {
		r.logger.Debug("Google PTR failed, trying system DNS", "ip", ip, "error", err.Error())
	}

	// Final fallback to system DNS
	hostnames, err = r.system.ResolvePtr(ctx, ip)
	if err == nil && len(hostnames) > 0 {
		r.logger.Debug("reverse lookup via system DNS", "ip", ip, "hostnames", len(hostnames))
		return hostnames, nil
	}

	return nil, fmt.Errorf("all DNS providers failed for IP %s: %w", ip, err)
}

// =================================================================
// Cloudflare DNS over HTTPS Provider
// =================================================================

// CloudflareDNS implements DNS resolution using Cloudflare's 1.1.1.1 service.
// Endpoint: https://cloudflare-dns.com/dns-query
// No authentication required, no rate limits.
type CloudflareDNS struct {
	client *httpclient.Client
	logger logx.Logger
}

// CloudflareDNSResponse represents the JSON response from Cloudflare DNS.
type CloudflareDNSResponse struct {
	Status   int                       `json:"Status"`
	TC       bool                      `json:"TC"`
	RD       bool                      `json:"RD"`
	RA       bool                      `json:"RA"`
	AD       bool                      `json:"AD"`
	CD       bool                      `json:"CD"`
	Question []CloudflareDNSQuestion   `json:"Question"`
	Answer   []CloudflareDNSAnswer     `json:"Answer"`
}

type CloudflareDNSQuestion struct {
	Name string `json:"name"`
	Type int    `json:"type"`
}

type CloudflareDNSAnswer struct {
	Name string `json:"name"`
	Type int    `json:"type"`
	TTL  int    `json:"TTL"`
	Data string `json:"data"`
}

// Resolve queries Cloudflare DNS for A/AAAA records.
func (c *CloudflareDNS) Resolve(ctx context.Context, hostname string) ([]string, error) {
	url := fmt.Sprintf("https://cloudflare-dns.com/dns-query?name=%s&type=A", hostname)

	headers := map[string]string{
		"accept": "application/dns-json",
	}

	resp, err := c.client.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("cloudflare DNS request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := httpclient.ReadBody(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to read cloudflare response: %w", err)
	}

	var dnsResp CloudflareDNSResponse
	if err := json.Unmarshal(body, &dnsResp); err != nil {
		return nil, fmt.Errorf("failed to parse cloudflare response: %w", err)
	}

	if dnsResp.Status != 0 {
		return nil, fmt.Errorf("cloudflare DNS returned error status: %d", dnsResp.Status)
	}

	ips := make([]string, 0, len(dnsResp.Answer))
	for _, answer := range dnsResp.Answer {
		// Type 1 = A record, Type 28 = AAAA record
		if answer.Type == 1 || answer.Type == 28 {
			ips = append(ips, answer.Data)
		}
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no A/AAAA records found for %s", hostname)
	}

	return ips, nil
}

// ResolvePtr performs reverse DNS lookup using Cloudflare DNS (PTR query).
func (c *CloudflareDNS) ResolvePtr(ctx context.Context, ip string) ([]string, error) {
	// Convert IP to PTR format (e.g., 1.2.3.4 → 4.3.2.1.in-addr.arpa)
	ptrName, err := reverseIPForPTR(ip)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://cloudflare-dns.com/dns-query?name=%s&type=PTR", ptrName)

	headers := map[string]string{
		"accept": "application/dns-json",
	}

	resp, err := c.client.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("cloudflare PTR request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := httpclient.ReadBody(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to read cloudflare PTR response: %w", err)
	}

	var dnsResp CloudflareDNSResponse
	if err := json.Unmarshal(body, &dnsResp); err != nil {
		return nil, fmt.Errorf("failed to parse cloudflare PTR response: %w", err)
	}

	if dnsResp.Status != 0 {
		return nil, fmt.Errorf("cloudflare PTR returned error status: %d", dnsResp.Status)
	}

	hostnames := make([]string, 0, len(dnsResp.Answer))
	for _, answer := range dnsResp.Answer {
		// Type 12 = PTR record
		if answer.Type == 12 {
			// Remove trailing dot if present
			hostname := strings.TrimSuffix(answer.Data, ".")
			hostnames = append(hostnames, hostname)
		}
	}

	if len(hostnames) == 0 {
		return nil, fmt.Errorf("no PTR records found for %s", ip)
	}

	return hostnames, nil
}

// =================================================================
// Google Public DNS JSON API Provider
// =================================================================

// GoogleDNS implements DNS resolution using Google Public DNS JSON API.
// Endpoint: https://dns.google/resolve
// No authentication required, no rate limits.
type GoogleDNS struct {
	client *httpclient.Client
	logger logx.Logger
}

// GoogleDNSResponse represents the JSON response from Google DNS.
type GoogleDNSResponse struct {
	Status   int                 `json:"Status"`
	TC       bool                `json:"TC"`
	RD       bool                `json:"RD"`
	RA       bool                `json:"RA"`
	AD       bool                `json:"AD"`
	CD       bool                `json:"CD"`
	Question []GoogleDNSQuestion `json:"Question"`
	Answer   []GoogleDNSAnswer   `json:"Answer"`
}

type GoogleDNSQuestion struct {
	Name string `json:"name"`
	Type int    `json:"type"`
}

type GoogleDNSAnswer struct {
	Name string `json:"name"`
	Type int    `json:"type"`
	TTL  int    `json:"TTL"`
	Data string `json:"data"`
}

// Resolve queries Google DNS for A/AAAA records.
func (g *GoogleDNS) Resolve(ctx context.Context, hostname string) ([]string, error) {
	url := fmt.Sprintf("https://dns.google/resolve?name=%s&type=A", hostname)

	resp, err := g.client.Get(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("google DNS request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := httpclient.ReadBody(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to read google response: %w", err)
	}

	var dnsResp GoogleDNSResponse
	if err := json.Unmarshal(body, &dnsResp); err != nil {
		return nil, fmt.Errorf("failed to parse google response: %w", err)
	}

	if dnsResp.Status != 0 {
		return nil, fmt.Errorf("google DNS returned error status: %d", dnsResp.Status)
	}

	ips := make([]string, 0, len(dnsResp.Answer))
	for _, answer := range dnsResp.Answer {
		// Type 1 = A record, Type 28 = AAAA record
		if answer.Type == 1 || answer.Type == 28 {
			ips = append(ips, answer.Data)
		}
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no A/AAAA records found for %s", hostname)
	}

	return ips, nil
}

// ResolvePtr performs reverse DNS lookup using Google DNS (PTR query).
func (g *GoogleDNS) ResolvePtr(ctx context.Context, ip string) ([]string, error) {
	// Convert IP to PTR format
	ptrName, err := reverseIPForPTR(ip)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://dns.google/resolve?name=%s&type=PTR", ptrName)

	resp, err := g.client.Get(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("google PTR request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := httpclient.ReadBody(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to read google PTR response: %w", err)
	}

	var dnsResp GoogleDNSResponse
	if err := json.Unmarshal(body, &dnsResp); err != nil {
		return nil, fmt.Errorf("failed to parse google PTR response: %w", err)
	}

	if dnsResp.Status != 0 {
		return nil, fmt.Errorf("google PTR returned error status: %d", dnsResp.Status)
	}

	hostnames := make([]string, 0, len(dnsResp.Answer))
	for _, answer := range dnsResp.Answer {
		// Type 12 = PTR record
		if answer.Type == 12 {
			hostname := strings.TrimSuffix(answer.Data, ".")
			hostnames = append(hostnames, hostname)
		}
	}

	if len(hostnames) == 0 {
		return nil, fmt.Errorf("no PTR records found for %s", ip)
	}

	return hostnames, nil
}

// =================================================================
// System DNS Provider (Go stdlib)
// =================================================================

// SystemDNS implements DNS resolution using Go's standard library.
// This is the final fallback and always available.
type SystemDNS struct {
	logger logx.Logger
}

// Resolve uses Go's net.LookupIP for DNS resolution.
func (s *SystemDNS) Resolve(ctx context.Context, hostname string) ([]string, error) {
	resolver := &net.Resolver{}
	ips, err := resolver.LookupIP(ctx, "ip", hostname)
	if err != nil {
		return nil, fmt.Errorf("system DNS lookup failed: %w", err)
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no IPs found for %s", hostname)
	}

	result := make([]string, 0, len(ips))
	for _, ip := range ips {
		result = append(result, ip.String())
	}

	return result, nil
}

// ResolvePtr uses Go's net.LookupAddr for reverse DNS lookup.
func (s *SystemDNS) ResolvePtr(ctx context.Context, ip string) ([]string, error) {
	resolver := &net.Resolver{}
	hostnames, err := resolver.LookupAddr(ctx, ip)
	if err != nil {
		return nil, fmt.Errorf("system PTR lookup failed: %w", err)
	}

	if len(hostnames) == 0 {
		return nil, fmt.Errorf("no hostnames found for %s", ip)
	}

	// Remove trailing dots
	result := make([]string, 0, len(hostnames))
	for _, hostname := range hostnames {
		result = append(result, strings.TrimSuffix(hostname, "."))
	}

	return result, nil
}

// =================================================================
// Helper Functions
// =================================================================

// reverseIPForPTR converts an IP address to PTR query format.
// Example: 93.184.216.34 → 34.216.184.93.in-addr.arpa
// Example: 2606:2800:220:1:248:1893:25c8:1946 → 6.4.9.1...ip6.arpa
func reverseIPForPTR(ip string) (string, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return "", fmt.Errorf("invalid IP address: %s", ip)
	}

	// IPv4
	if parsedIP.To4() != nil {
		parts := strings.Split(ip, ".")
		if len(parts) != 4 {
			return "", fmt.Errorf("invalid IPv4 address: %s", ip)
		}
		// Reverse octets
		return fmt.Sprintf("%s.%s.%s.%s.in-addr.arpa", parts[3], parts[2], parts[1], parts[0]), nil
	}

	// IPv6 - more complex, convert to full notation and reverse nibbles
	// For simplicity, let stdlib handle IPv6 PTR in SystemDNS fallback
	return "", fmt.Errorf("IPv6 PTR not supported for online DNS providers, will use system DNS")
}
