package whatweb

import (
	"context"
	"testing"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/registry"
)

func TestMain(m *testing.M) {
	// Run tests (init() will already have registered the source)
	m.Run()
}

// TestRegistry_WhatWebSource validates that whatweb registers correctly.
func TestRegistry_WhatWebSource(t *testing.T) {
	// The init() function should have already run during package import

	// Verify registration
	if !registry.Global().IsRegistered("whatweb") {
		t.Fatal("whatweb source not registered")
	}

	// Verify metadata
	meta, ok := registry.Global().GetMetadata("whatweb")
	if !ok {
		t.Fatal("whatweb metadata not found")
	}

	// Validate metadata fields
	if meta.Name != "whatweb" {
		t.Errorf("expected name 'whatweb', got '%s'", meta.Name)
	}

	if meta.Mode != domain.SourceModeActive {
		t.Errorf("expected mode 'Active', got '%s'", meta.Mode)
	}

	if meta.Type != domain.SourceTypeCLI {
		t.Errorf("expected type 'CLI', got '%s'", meta.Type)
	}

	if meta.Priority != 16 {
		t.Errorf("expected priority 16, got %d", meta.Priority)
	}

	if meta.StageHint != 2 {
		t.Errorf("expected stage hint 2, got %d", meta.StageHint)
	}

	// Validate input artifacts
	expectedInputs := []domain.ArtifactType{
		domain.ArtifactTypeSubdomain,
		domain.ArtifactTypeURL,
	}

	if len(meta.InputArtifacts) != len(expectedInputs) {
		t.Errorf("expected %d input artifacts, got %d", len(expectedInputs), len(meta.InputArtifacts))
	}

	// Validate output artifacts
	expectedOutputs := []domain.ArtifactType{
		domain.ArtifactTypeTechnology,
		domain.ArtifactTypeService,
	}

	if len(meta.OutputArtifacts) != len(expectedOutputs) {
		t.Errorf("expected %d output artifacts, got %d", len(expectedOutputs), len(meta.OutputArtifacts))
	}
}

// TestFactory_WhatWebSource validates that the factory creates a valid source.
func TestFactory_WhatWebSource(t *testing.T) {
	logger := logx.New()
	cfg := getTestConfig()

	source, err := factory(cfg, logger)
	if err != nil {
		t.Fatalf("factory failed: %v", err)
	}

	if source == nil {
		t.Fatal("factory returned nil source")
	}

	// Verify interface implementation
	if source.Name() != "whatweb" {
		t.Errorf("expected name 'whatweb', got '%s'", source.Name())
	}

	if source.Mode() != domain.SourceModeActive {
		t.Errorf("expected mode 'Active', got '%s'", source.Mode())
	}

	if source.Type() != domain.SourceTypeCLI {
		t.Errorf("expected type 'CLI', got '%s'", source.Type())
	}

	// Verify InputConsumer interface
	if _, ok := source.(ports.InputConsumer); !ok {
		t.Error("source should implement InputConsumer interface")
	}

	// Cleanup
	if err := source.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}
}

// TestWhatWebSource_Close validates proper cleanup.
func TestWhatWebSource_Close(t *testing.T) {
	logger := logx.New()
	source := New(logger)

	// Close once
	if err := source.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}

	// Close again (should be idempotent)
	if err := source.Close(); err != nil {
		t.Errorf("Close() failed on second call: %v", err)
	}
}

// TestWhatWebSource_Validate validates configuration validation.
func TestWhatWebSource_Validate(t *testing.T) {
	logger := logx.New()

	tests := []struct {
		name        string
		aggression  int
		threads     int
		expectError bool
	}{
		{"valid_default", 1, 25, false},
		{"valid_max_aggression", 4, 50, false},
		{"invalid_aggression_low", 0, 25, true},
		{"invalid_aggression_high", 5, 25, true},
		{"invalid_threads_zero", 1, 0, true},
		{"invalid_threads_high", 1, 101, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := NewWithConfig(logger, "whatweb", 120*time.Second, tt.aggression, tt.threads, "test-ua")

			// WhatWebSource implements Validate directly via AdvancedSource embedding
			err := source.Validate()
			if tt.expectError && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}

			source.Close()
		})
	}
}

// TestWhatWebSource_ExtractInputItems validates input extraction logic.
func TestWhatWebSource_ExtractInputItems(t *testing.T) {
	logger := logx.New()
	source := New(logger)
	defer source.Close()

	target := domain.Target{
		Root: "example.com",
		Mode: domain.ScanModeActive,
	}

	// Create mock input with subdomains and URLs
	input := domain.NewScanResult(target)

	// Add subdomain
	subdomain := domain.NewArtifact(domain.ArtifactTypeSubdomain, "sub.example.com", "test")
	input.AddArtifact(subdomain)

	// Add URL
	url := domain.NewArtifact(domain.ArtifactTypeURL, "https://example.com/page", "test")
	input.AddArtifact(url)

	// Extract items
	items := source.extractInputItems(input)

	if len(items) == 0 {
		t.Fatal("expected items to be extracted, got 0")
	}

	// Verify URLs are present
	hasSubdomain := false
	hasURL := false

	for _, item := range items {
		if item == "https://sub.example.com" {
			hasSubdomain = true
		}
		if item == "https://example.com/page" {
			hasURL = true
		}
	}

	if !hasSubdomain {
		t.Error("expected subdomain URL to be extracted")
	}

	if !hasURL {
		t.Error("expected URL to be extracted")
	}
}

// TestWhatWebSource_Run validates basic Run() method.
func TestWhatWebSource_Run(t *testing.T) {
	t.Skip("Requires whatweb binary and network access - run manually for integration testing")

	logger := logx.New()
	source := New(logger)
	defer source.Close()

	target := domain.Target{
		Root: "example.com",
		Mode: domain.ScanModeActive,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := source.Run(ctx, target)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	// Note: Run() without input may produce warnings but should not fail
	t.Logf("Run() completed with %d artifacts", len(result.Artifacts))
}

// TestWhatWebSource_RunWithInput validates RunWithInput() method.
func TestWhatWebSource_RunWithInput(t *testing.T) {
	t.Skip("Requires whatweb binary and network access - run manually for integration testing")

	logger := logx.New()
	source := New(logger)
	defer source.Close()

	target := domain.Target{
		Root: "example.com",
		Mode: domain.ScanModeActive,
	}

	// Create mock input
	input := domain.NewScanResult(target)
	url := domain.NewArtifact(domain.ArtifactTypeURL, "https://example.com", "test")
	input.AddArtifact(url)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := source.RunWithInput(ctx, target, input)
	if err != nil {
		t.Fatalf("RunWithInput() failed: %v", err)
	}

	if result == nil {
		t.Fatal("RunWithInput() returned nil result")
	}

	// Should produce technology artifacts
	if len(result.Artifacts) == 0 {
		t.Error("expected artifacts, got 0")
	}

	t.Logf("RunWithInput() completed with %d artifacts", len(result.Artifacts))
}

// getTestConfig returns a test configuration for whatweb.
func getTestConfig() ports.SourceConfig {
	return ports.SourceConfig{
		Enabled:   true,
		Timeout:   120 * time.Second,
		Retries:   2,
		RateLimit: 0,
		Priority:  16,
		Custom: map[string]interface{}{
			"exec_path":  "whatweb",
			"aggression": 1,
			"threads":    25,
			"user_agent": "AethonX/1.0 Test",
		},
	}
}
