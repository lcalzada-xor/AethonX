package whatweb

import (
	"context"
	"testing"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
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
		domain.ArtifactTypeIP,
		domain.ArtifactTypeEmail,
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

	// Create mock input with subdomains and URLs (with metadata, as they would come from httpx)
	input := domain.NewScanResult(target)

	// Add subdomain with DomainMetadata (as httpx would provide)
	subdomainMeta := metadata.NewDomainMetadata()
	subdomainMeta.HTTPStatus = 200
	subdomainMeta.HasSSL = true
	subdomain := domain.NewArtifactWithMetadata(
		domain.ArtifactTypeSubdomain,
		"sub.example.com",
		"test",
		subdomainMeta,
	)
	input.AddArtifact(subdomain)

	// Add URL
	url := domain.NewArtifact(domain.ArtifactTypeURL, "https://example.com/page", "test")
	input.AddArtifact(url)

	// Extract items
	items := source.extractInputItems(input, target)

	if len(items) == 0 {
		t.Fatal("expected items to be extracted, got 0")
	}

	// Verify URLs are present
	hasSubdomain := false
	hasURL := false
	hasRoot := false

	for _, item := range items {
		if item == "https://sub.example.com" {
			hasSubdomain = true
		}
		if item == "https://example.com/page" {
			hasURL = true
		}
		if item == "https://example.com" {
			hasRoot = true
		}
	}

	if !hasRoot {
		t.Error("expected root domain URL to be included")
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

// TestParser_MetadataPlugins validates that metadata plugins are handled correctly.
func TestParser_MetadataPlugins(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "whatweb")
	target := domain.Target{Root: "example.com", Mode: domain.ScanModeActive}

	tests := []struct {
		name           string
		pluginName     string
		plugin         Plugin
		expectedType   domain.ArtifactType
		expectedValue  string
		shouldCreate   bool
	}{
		{
			name:       "IP plugin creates IP artifact",
			pluginName: "IP",
			plugin: Plugin{
				String: []string{"192.168.1.1"},
			},
			expectedType:  domain.ArtifactTypeIP,
			expectedValue: "192.168.1.1",
			shouldCreate:  true,
		},
		{
			name:       "Country plugin does not create artifact",
			pluginName: "Country",
			plugin: Plugin{
				String: []string{"UNITED STATES"},
				Data:   map[string]interface{}{"module": []string{"US"}},
			},
			shouldCreate: false,
		},
		{
			name:       "HTTPServer plugin creates Technology artifact with real server name",
			pluginName: "HTTPServer",
			plugin: Plugin{
				String: []string{"nginx/1.18.0"},
			},
			expectedType:  domain.ArtifactTypeTechnology,
			expectedValue: "nginx",
			shouldCreate:  true,
		},
		{
			name:       "HTTPServer with Apache",
			pluginName: "HTTPServer",
			plugin: Plugin{
				String: []string{"Apache/2.4.41 (Ubuntu)"},
			},
			expectedType:  domain.ArtifactTypeTechnology,
			expectedValue: "Apache",
			shouldCreate:  true,
		},
		{
			name:       "RedirectLocation creates URL artifact",
			pluginName: "RedirectLocation",
			plugin: Plugin{
				String: []string{"https://www.example.com"},
			},
			expectedType:  domain.ArtifactTypeURL,
			expectedValue: "https://www.example.com",
			shouldCreate:  true,
		},
		{
			name:       "Email plugin creates Email artifact",
			pluginName: "Email",
			plugin: Plugin{
				String: []string{"admin@example.com"},
			},
			expectedType:  domain.ArtifactTypeEmail,
			expectedValue: "admin@example.com",
			shouldCreate:  true,
		},
		{
			name:       "X-Powered-By creates Technology artifact",
			pluginName: "X-Powered-By",
			plugin: Plugin{
				String: []string{"PHP/7.4.3"},
			},
			expectedType:  domain.ArtifactTypeTechnology,
			expectedValue: "PHP",
			shouldCreate:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := WhatWebResponse{
				Target:     "https://example.com",
				HTTPStatus: 200,
				Plugins: map[string]Plugin{
					tt.pluginName: tt.plugin,
				},
			}

			artifacts := parser.ParseMultipleResponses([]WhatWebResponse{resp}, target)

			if tt.shouldCreate {
				if len(artifacts) == 0 {
					t.Fatalf("expected artifact to be created, got 0")
				}

				artifact := artifacts[0]
				if artifact.Type != tt.expectedType {
					t.Errorf("expected type %s, got %s", tt.expectedType, artifact.Type)
				}

				if artifact.Value != tt.expectedValue {
					t.Errorf("expected value %s, got %s", tt.expectedValue, artifact.Value)
				}
			} else {
				if len(artifacts) > 0 {
					t.Errorf("expected no artifacts, got %d", len(artifacts))
				}
			}
		})
	}
}

// TestParser_RealTechnologyPlugins validates that real technology plugins still work.
func TestParser_RealTechnologyPlugins(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "whatweb")
	target := domain.Target{Root: "example.com", Mode: domain.ScanModeActive}

	tests := []struct {
		name         string
		pluginName   string
		plugin       Plugin
		expectedTech string
	}{
		{
			name:       "HTML5 technology",
			pluginName: "HTML5",
			plugin:     Plugin{},
			expectedTech: "HTML5",
		},
		{
			name:       "Akamai CDN",
			pluginName: "Akamai Global Host",
			plugin:     Plugin{},
			expectedTech: "Akamai Global Host",
		},
		{
			name:       "Strict Transport Security",
			pluginName: "Strict-Transport-Security",
			plugin: Plugin{
				String: []string{"max-age=31536000"},
			},
			expectedTech: "Strict Transport Security",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := WhatWebResponse{
				Target:     "https://example.com",
				HTTPStatus: 200,
				Plugins: map[string]Plugin{
					tt.pluginName: tt.plugin,
				},
			}

			artifacts := parser.ParseMultipleResponses([]WhatWebResponse{resp}, target)

			if len(artifacts) == 0 {
				t.Fatalf("expected technology artifact, got 0")
			}

			artifact := artifacts[0]
			if artifact.Type != domain.ArtifactTypeTechnology {
				t.Errorf("expected type Technology, got %s", artifact.Type)
			}

			if artifact.Value != tt.expectedTech {
				t.Errorf("expected technology %s, got %s", tt.expectedTech, artifact.Value)
			}
		})
	}
}

// TestParser_parseServerString validates server string parsing.
func TestParser_parseServerString(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "whatweb")

	tests := []struct {
		input           string
		expectedName    string
		expectedVersion string
	}{
		{"nginx/1.18.0", "nginx", "1.18.0"},
		{"Apache/2.4.41 (Ubuntu)", "Apache", "2.4.41"},
		{"PHP/7.4.3", "PHP", "7.4.3"},
		{"Microsoft-IIS/10.0", "Microsoft-IIS", "10.0"},
		{"cloudflare", "cloudflare", ""},
		{"", "", ""},
		{"   ", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, version := parser.parseServerString(tt.input)
			if name != tt.expectedName {
				t.Errorf("expected name %q, got %q", tt.expectedName, name)
			}
			if version != tt.expectedVersion {
				t.Errorf("expected version %q, got %q", tt.expectedVersion, version)
			}
		})
	}
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
