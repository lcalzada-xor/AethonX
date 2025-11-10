// internal/sources/ipapi/ipapi_test.go
package ipapi

import (
	"context"
	"testing"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
)

func TestIPAPISource_Name(t *testing.T) {
	logger := logx.NewSilent()
	source := New(logger)

	if source.Name() != "ipapi" {
		t.Errorf("expected name 'ipapi', got '%s'", source.Name())
	}
}

func TestIPAPISource_Mode(t *testing.T) {
	logger := logx.NewSilent()
	source := New(logger)

	if source.Mode() != domain.SourceModePassive {
		t.Errorf("expected passive mode, got %v", source.Mode())
	}
}

func TestIPAPISource_Type(t *testing.T) {
	logger := logx.NewSilent()
	source := New(logger)

	if source.Type() != domain.SourceTypeAPI {
		t.Errorf("expected API type, got %v", source.Type())
	}
}

func TestIPAPISource_Close(t *testing.T) {
	logger := logx.NewSilent()
	source := New(logger)

	if err := source.Close(); err != nil {
		t.Errorf("Close() should not return error, got: %v", err)
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"IPv4 Public Google DNS", "8.8.8.8", false},
		{"IPv4 Public Cloudflare", "1.1.1.1", false},
		{"IPv4 Private 10.x", "10.0.0.1", true},
		{"IPv4 Private 192.168.x", "192.168.1.1", true},
		{"IPv4 Private 172.16.x", "172.16.0.1", true},
		{"IPv4 Private 172.31.x", "172.31.255.255", true},
		{"IPv4 Loopback", "127.0.0.1", true},
		{"IPv6 Loopback", "::1", true},
		{"IPv6 Link-local", "fe80::1", true},
		{"IPv6 Unique local fc", "fc00::1", true},
		{"IPv6 Unique local fd", "fd00::1", true},
		{"Empty string", "", true},
		{"Localhost", "localhost", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPrivateIP(tt.ip)
			if result != tt.expected {
				t.Errorf("IsPrivateIP(%s) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}

// TestIPAPISource_Run_Integration is an integration test that requires network access.
// Skip in CI environments by checking for specific environment variable.
func TestIPAPISource_Run_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logx.NewSilent()
	source := New(logger)
	defer source.Close()

	target := domain.Target{
		Root: "google.com",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := source.Run(ctx, target)
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	// Should have at least one IP artifact
	if len(result.Artifacts) == 0 {
		t.Error("Run() returned no artifacts (expected at least one IP)")
	}

	// Check artifact structure
	for _, artifact := range result.Artifacts {
		if artifact.Type != domain.ArtifactTypeIP {
			t.Errorf("expected artifact type IP, got %v", artifact.Type)
		}

		if artifact.Value == "" {
			t.Error("artifact value is empty")
		}

		if artifact.TypedMetadata == nil {
			t.Error("artifact has nil TypedMetadata")
			continue
		}

		// Verify metadata is IPMetadata
		if artifact.TypedMetadata.Type() != "ip" {
			t.Errorf("expected metadata type 'ip', got '%s'", artifact.TypedMetadata.Type())
		}

		// Check for enrichment tag
		hasEnrichmentTag := false
		for _, tag := range artifact.Tags {
			if tag == "enriched" {
				hasEnrichmentTag = true
				break
			}
		}

		if !hasEnrichmentTag {
			t.Error("artifact missing 'enriched' tag")
		}

		t.Logf("Artifact: %s, Tags: %v", artifact.Value, artifact.Tags)
	}
}
