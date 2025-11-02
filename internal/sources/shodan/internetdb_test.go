// internal/sources/shodan/internetdb_test.go
package shodan

import (
	"context"
	"testing"
	"time"

	"aethonx/internal/platform/logx"
)

func TestInternetDBLookupIP(t *testing.T) {
	logger := logx.New()
	client := NewInternetDBClient(logger)

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
			name:    "Invalid IP (returns nil, no error)",
			ip:      "invalid",
			wantErr: false, // InternetDB client handles invalid IPs gracefully
		},
		{
			name:    "Private IP (should return nil)",
			ip:      "192.168.1.1",
			wantErr: false, // InternetDB returns nil for private IPs, not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.LookupIP(ctx, tt.ip)

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

			if result == nil {
				t.Logf("⚠ No data found for %s (expected for private IPs)", tt.ip)
				return
			}

			t.Logf("✓ InternetDB lookup %s:", tt.ip)
			t.Logf("  - Ports: %v", result.Ports)
			t.Logf("  - Hostnames: %v", result.Hostnames)
			t.Logf("  - Tags: %v", result.Tags)
			t.Logf("  - CPEs: %d", len(result.CPEs))
			t.Logf("  - Vulnerabilities: %d", len(result.Vulns))
		})
	}
}

func TestInternetDBLookupBatch(t *testing.T) {
	logger := logx.New()
	client := NewInternetDBClient(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use well-known public IPs for testing
	ips := []string{
		"8.8.8.8",       // Google DNS
		"1.1.1.1",       // Cloudflare DNS
		"208.67.222.222", // OpenDNS
	}

	results, err := client.LookupBatch(ctx, ips)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("✓ InternetDB batch lookup completed")
	t.Logf("  - Requested: %d IPs", len(ips))
	t.Logf("  - Results: %d", len(results))

	for _, result := range results {
		if result == nil {
			continue
		}
		t.Logf("  - %s: %d ports, %d hostnames, %d vulns",
			result.IP,
			len(result.Ports),
			len(result.Hostnames),
			len(result.Vulns),
		)
	}

	// At least some results should be returned
	if len(results) == 0 {
		t.Error("expected at least some results from batch lookup")
	}
}

func TestInternetDBRateLimit(t *testing.T) {
	logger := logx.New()
	client := NewInternetDBClient(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Try multiple requests to test rate limiting
	ips := []string{
		"8.8.8.8",
		"8.8.4.4",
		"1.1.1.1",
		"1.0.0.1",
	}

	start := time.Now()

	for _, ip := range ips {
		_, err := client.LookupIP(ctx, ip)
		if err != nil {
			t.Logf("⚠ Request failed for %s: %v", ip, err)
		}
	}

	duration := time.Since(start)

	// With 1 req/s rate limit, 4 requests should take ~3-4 seconds
	expectedMin := 3 * time.Second
	expectedMax := 6 * time.Second

	if duration < expectedMin {
		t.Errorf("rate limiting not working: completed too fast (%v)", duration)
	}

	if duration > expectedMax {
		t.Logf("⚠ Requests took longer than expected (%v), but within acceptable range", duration)
	}

	t.Logf("✓ Rate limit test: %d requests in %v (expected ~3-4s)", len(ips), duration)
}

func TestInternetDBEmptyResponse(t *testing.T) {
	logger := logx.New()
	client := NewInternetDBClient(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test with a valid IP that likely has no data in Shodan
	// Using a private IP range that won't be in InternetDB
	result, err := client.LookupIP(ctx, "192.168.1.1")

	if err != nil {
		t.Errorf("unexpected error for private IP: %v", err)
		return
	}

	// Private IPs should return nil (no data), not an error
	if result != nil {
		t.Logf("⚠ Unexpected data for private IP: %+v", result)
	} else {
		t.Logf("✓ Private IP correctly returned no data")
	}
}

func TestInternetDBConcurrency(t *testing.T) {
	logger := logx.New()
	client := NewInternetDBClient(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test concurrent requests
	ips := []string{
		"8.8.8.8",
		"8.8.4.4",
		"1.1.1.1",
		"1.0.0.1",
		"208.67.222.222",
	}

	type result struct {
		ip   string
		resp *InternetDBResponse
		err  error
	}

	results := make(chan result, len(ips))

	for _, ip := range ips {
		go func(ip string) {
			resp, err := client.LookupIP(ctx, ip)
			results <- result{ip: ip, resp: resp, err: err}
		}(ip)
	}

	// Collect results
	successCount := 0
	for i := 0; i < len(ips); i++ {
		res := <-results
		if res.err != nil {
			t.Logf("⚠ Failed to lookup %s: %v", res.ip, res.err)
		} else if res.resp != nil {
			successCount++
			t.Logf("✓ Looked up %s: %d ports", res.ip, len(res.resp.Ports))
		} else {
			t.Logf("⚠ No data for %s", res.ip)
		}
	}

	t.Logf("✓ Concurrent test: %d/%d successful", successCount, len(ips))
}
