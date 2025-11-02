// internal/sources/shodan/dns_resolver_test.go
package shodan

import (
	"context"
	"testing"
	"time"

	"aethonx/internal/platform/logx"
)

func TestCloudflareResolve(t *testing.T) {
	logger := logx.New()
	resolver := NewDNSResolver(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tests := []struct {
		name     string
		hostname string
		wantErr  bool
	}{
		{
			name:     "valid domain",
			hostname: "example.com",
			wantErr:  false,
		},
		{
			name:     "valid subdomain",
			hostname: "www.example.com",
			wantErr:  false,
		},
		{
			name:     "invalid domain",
			hostname: "thisdoesnotexist12345.invalid",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ips, err := resolver.cloudflare.Resolve(ctx, tt.hostname)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %s, got nil", tt.hostname)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for %s: %v", tt.hostname, err)
				return
			}

			if len(ips) == 0 {
				t.Errorf("expected IPs for %s, got none", tt.hostname)
			}

			t.Logf("✓ Cloudflare resolved %s → %v", tt.hostname, ips)
		})
	}
}

func TestGoogleResolve(t *testing.T) {
	logger := logx.New()
	resolver := NewDNSResolver(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tests := []struct {
		name     string
		hostname string
		wantErr  bool
	}{
		{
			name:     "valid domain",
			hostname: "example.com",
			wantErr:  false,
		},
		{
			name:     "valid subdomain",
			hostname: "www.google.com",
			wantErr:  false,
		},
		{
			name:     "invalid domain",
			hostname: "thisdoesnotexist12345.invalid",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ips, err := resolver.google.Resolve(ctx, tt.hostname)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %s, got nil", tt.hostname)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for %s: %v", tt.hostname, err)
				return
			}

			if len(ips) == 0 {
				t.Errorf("expected IPs for %s, got none", tt.hostname)
			}

			t.Logf("✓ Google resolved %s → %v", tt.hostname, ips)
		})
	}
}

func TestSystemResolve(t *testing.T) {
	logger := logx.New()
	resolver := NewDNSResolver(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tests := []struct {
		name     string
		hostname string
		wantErr  bool
	}{
		{
			name:     "valid domain",
			hostname: "example.com",
			wantErr:  false,
		},
		{
			name:     "localhost",
			hostname: "localhost",
			wantErr:  false,
		},
		{
			name:     "invalid domain",
			hostname: "thisdoesnotexist12345.invalid",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ips, err := resolver.system.Resolve(ctx, tt.hostname)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %s, got nil", tt.hostname)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for %s: %v", tt.hostname, err)
				return
			}

			if len(ips) == 0 {
				t.Errorf("expected IPs for %s, got none", tt.hostname)
			}

			t.Logf("✓ System resolved %s → %v", tt.hostname, ips)
		})
	}
}

func TestResolveWithFallback(t *testing.T) {
	logger := logx.New()
	resolver := NewDNSResolver(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	tests := []struct {
		name     string
		hostname string
		wantErr  bool
	}{
		{
			name:     "example.com - should resolve via Cloudflare",
			hostname: "example.com",
			wantErr:  false,
		},
		{
			name:     "www.example.com - should resolve via Cloudflare",
			hostname: "www.example.com",
			wantErr:  false,
		},
		{
			name:     "google.com - should resolve via Cloudflare",
			hostname: "google.com",
			wantErr:  false,
		},
		{
			name:     "invalid domain - should fail all providers",
			hostname: "thisdoesnotexist12345.invalid",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ips, err := resolver.ResolveWithFallback(ctx, tt.hostname)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %s, got nil", tt.hostname)
				}
				t.Logf("✓ Expected failure for %s: %v", tt.hostname, err)
				return
			}

			if err != nil {
				t.Errorf("unexpected error for %s: %v", tt.hostname, err)
				return
			}

			if len(ips) == 0 {
				t.Errorf("expected IPs for %s, got none", tt.hostname)
			}

			t.Logf("✓ Fallback chain resolved %s → %v", tt.hostname, ips)
		})
	}
}

func TestReverseLookupWithFallback(t *testing.T) {
	logger := logx.New()
	resolver := NewDNSResolver(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	tests := []struct {
		name    string
		ip      string
		wantErr bool
	}{
		{
			name:    "Google DNS 8.8.8.8",
			ip:      "8.8.8.8",
			wantErr: false,
		},
		{
			name:    "Cloudflare DNS 1.1.1.1",
			ip:      "1.1.1.1",
			wantErr: false,
		},
		{
			name:    "Invalid IP",
			ip:      "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hostnames, err := resolver.ReverseLookupWithFallback(ctx, tt.ip)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %s, got nil", tt.ip)
				}
				t.Logf("✓ Expected failure for %s: %v", tt.ip, err)
				return
			}

			if err != nil {
				// Reverse DNS can fail for valid IPs, that's ok
				t.Logf("⚠ Reverse DNS failed for %s (expected): %v", tt.ip, err)
				return
			}

			if len(hostnames) == 0 {
				t.Logf("⚠ No PTR records found for %s (expected)", tt.ip)
				return
			}

			t.Logf("✓ Reverse lookup resolved %s → %v", tt.ip, hostnames)
		})
	}
}

func TestReverseIPForPTR(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected string
		wantErr  bool
	}{
		{
			name:     "valid IPv4",
			ip:       "8.8.8.8",
			expected: "8.8.8.8.in-addr.arpa",
			wantErr:  false,
		},
		{
			name:     "another valid IPv4",
			ip:       "93.184.216.34",
			expected: "34.216.184.93.in-addr.arpa",
			wantErr:  false,
		},
		{
			name:    "invalid IP",
			ip:      "invalid",
			wantErr: true,
		},
		{
			name:    "IPv6 not supported",
			ip:      "2606:2800:220:1:248:1893:25c8:1946",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := reverseIPForPTR(tt.ip)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %s, got nil", tt.ip)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for %s: %v", tt.ip, err)
				return
			}

			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}

			t.Logf("✓ PTR format: %s → %s", tt.ip, result)
		})
	}
}

func TestDNSResolverConcurrency(t *testing.T) {
	logger := logx.New()
	resolver := NewDNSResolver(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	hostnames := []string{
		"example.com",
		"google.com",
		"github.com",
		"cloudflare.com",
		"amazon.com",
	}

	// Test concurrent DNS resolution
	type result struct {
		hostname string
		ips      []string
		err      error
	}

	results := make(chan result, len(hostnames))

	for _, hostname := range hostnames {
		go func(h string) {
			ips, err := resolver.ResolveWithFallback(ctx, h)
			results <- result{hostname: h, ips: ips, err: err}
		}(hostname)
	}

	// Collect results
	successCount := 0
	for i := 0; i < len(hostnames); i++ {
		res := <-results
		if res.err != nil {
			t.Logf("⚠ Failed to resolve %s: %v", res.hostname, res.err)
		} else {
			successCount++
			t.Logf("✓ Resolved %s → %v", res.hostname, res.ips)
		}
	}

	if successCount == 0 {
		t.Error("all concurrent DNS resolutions failed")
	}

	t.Logf("✓ Concurrent test: %d/%d successful", successCount, len(hostnames))
}
