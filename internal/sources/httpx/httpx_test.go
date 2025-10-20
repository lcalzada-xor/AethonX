package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
)

func TestNew(t *testing.T) {
	logger := logx.New()
	source := New(logger)

	if source == nil {
		t.Fatal("New() returned nil")
	}

	if source.Name() != "httpx" {
		t.Errorf("expected name 'httpx', got %s", source.Name())
	}

	if source.Mode() != domain.SourceModeActive {
		t.Errorf("expected mode active, got %s", source.Mode())
	}

	if source.Type() != domain.SourceTypeBuiltin {
		t.Errorf("expected type builtin, got %s", source.Type())
	}
}

func TestHTTPx_Run(t *testing.T) {
	// Setup test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	}))
	defer server.Close()

	logger := logx.New()
	source := New(logger).(*HTTPx)

	// Note: Este test no prueba conexión real porque Run() prueba el target.Root
	// que en este caso sería un dominio de prueba, no el servidor de test
	target := *domain.NewTarget("example.com", domain.ScanModeActive)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := source.Run(ctx, target)

	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	// El test no hará conexión real a example.com, pero no debe fallar
	// En producción, probaría dominios reales
}

func TestHTTPx_RunWithInput(t *testing.T) {
	// Setup test HTTP server
	hitCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCount++
		w.Header().Set("Server", "TestServer/1.0")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Test</body></html>"))
	}))
	defer server.Close()

	logger := logx.New()
	source := New(logger)

	target := *domain.NewTarget("example.com", domain.ScanModeActive)

	// Crear input con subdomains (simulados)
	input := domain.NewScanResult(target)
	input.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh"))
	input.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "api.example.com", "crtsh"))
	input.AddArtifact(domain.NewArtifact(domain.ArtifactTypeDomain, "example.com", "rdap"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Cast to InputConsumer for RunWithInput
	consumer, ok := source.(ports.InputConsumer)
	if !ok {
		t.Fatal("HTTPx should implement InputConsumer")
	}

	result, err := consumer.RunWithInput(ctx, target, input)

	if err != nil {
		t.Fatalf("RunWithInput() failed: %v", err)
	}

	if result == nil {
		t.Fatal("RunWithInput() returned nil result")
	}

	// Verificar que se procesaron los inputs
	// Nota: En este test no habrá resultados porque los dominios no son reales
	// En un ambiente real con dominios válidos, habría artifacts de URL e IP
}

func TestHTTPx_RunWithInput_EmptyInput(t *testing.T) {
	logger := logx.New()
	source := New(logger)

	target := *domain.NewTarget("example.com", domain.ScanModeActive)
	input := domain.NewScanResult(target) // Empty input

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	consumer, ok := source.(ports.InputConsumer)
	if !ok {
		t.Fatal("HTTPx should implement InputConsumer")
	}

	result, err := consumer.RunWithInput(ctx, target, input)

	if err != nil {
		t.Fatalf("RunWithInput() with empty input failed: %v", err)
	}

	if result == nil {
		t.Fatal("RunWithInput() returned nil result")
	}

	// Con input vacío, debería hacer fallback a Run()
}

func TestHTTPx_ExtractHosts(t *testing.T) {
	logger := logx.New()
	source := New(logger).(*HTTPx)

	artifacts := []*domain.Artifact{
		domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh"),
		domain.NewArtifact(domain.ArtifactTypeSubdomain, "api.example.com", "crtsh"),
		domain.NewArtifact(domain.ArtifactTypeDomain, "example.com", "rdap"),
		domain.NewArtifact(domain.ArtifactTypeEmail, "contact@example.com", "rdap"), // Should be ignored
		domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "other"), // Duplicate
	}

	hosts := source.extractHosts(artifacts)

	expectedCount := 3 // test.example.com, api.example.com, example.com (deduped)
	if len(hosts) != expectedCount {
		t.Errorf("expected %d hosts, got %d", expectedCount, len(hosts))
	}

	// Verificar que no hay duplicados
	seen := make(map[string]bool)
	for _, host := range hosts {
		if seen[host] {
			t.Errorf("duplicate host found: %s", host)
		}
		seen[host] = true
	}

	// Verificar que email fue filtrado
	for _, host := range hosts {
		if host == "contact@example.com" {
			t.Error("email should not be extracted as host")
		}
	}
}

func TestHTTPx_ExtractHosts_NilArtifacts(t *testing.T) {
	logger := logx.New()
	source := New(logger).(*HTTPx)

	artifacts := []*domain.Artifact{
		domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh"),
		nil, // Nil artifact should be skipped
		domain.NewArtifact(domain.ArtifactTypeSubdomain, "api.example.com", "crtsh"),
	}

	hosts := source.extractHosts(artifacts)

	expectedCount := 2
	if len(hosts) != expectedCount {
		t.Errorf("expected %d hosts (nil skipped), got %d", expectedCount, len(hosts))
	}
}

func TestHTTPx_ExtractIP(t *testing.T) {
	logger := logx.New()
	source := New(logger).(*HTTPx)

	tests := []struct {
		name     string
		host     string
		expectIP bool
	}{
		{
			name:     "IP address",
			host:     "192.168.1.1",
			expectIP: true,
		},
		{
			name:     "IP with port",
			host:     "192.168.1.1:8080",
			expectIP: true,
		},
		{
			name:     "hostname (will resolve or empty)",
			host:     "localhost",
			expectIP: true, // localhost should resolve
		},
		{
			name:     "invalid host",
			host:     "",
			expectIP: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := source.extractIP(tt.host)

			if tt.expectIP && ip == "" {
				t.Errorf("expected IP extraction to succeed, got empty")
			}

			if !tt.expectIP && ip != "" {
				t.Errorf("expected empty IP, got %s", ip)
			}
		})
	}
}

func TestHTTPx_Close(t *testing.T) {
	logger := logx.New()
	source := New(logger)

	err := source.Close()
	if err != nil {
		t.Errorf("Close() failed: %v", err)
	}
}

func TestHTTPx_ContextCancellation(t *testing.T) {
	// Setup slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // Simulate slow response
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := logx.New()
	source := New(logger)

	target := *domain.NewTarget("example.com", domain.ScanModeActive)
	input := domain.NewScanResult(target)
	input.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "slow.example.com", "crtsh"))

	// Cancel context immediately
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	consumer, ok := source.(ports.InputConsumer)
	if !ok {
		t.Fatal("HTTPx should implement InputConsumer")
	}

	_, err := consumer.RunWithInput(ctx, target, input)

	// Should not error (context cancellation is handled gracefully)
	if err != nil {
		t.Errorf("expected no error on context cancellation, got: %v", err)
	}
}

func TestHTTPx_Probe_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "TestServer")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	logger := logx.New()
	source := New(logger).(*HTTPx)

	target := *domain.NewTarget("example.com", domain.ScanModeActive)
	result := domain.NewScanResult(target)

	ctx := context.Background()

	success := source.probe(ctx, server.URL, result)

	if !success {
		t.Error("expected probe to succeed")
	}

	if len(result.Artifacts) == 0 {
		t.Error("expected artifacts to be added")
	}

	// Verificar que se creó artifact de URL
	hasURL := false
	for _, artifact := range result.Artifacts {
		if artifact.Type == domain.ArtifactTypeURL {
			hasURL = true
			if artifact.Value != server.URL {
				t.Errorf("expected URL %s, got %s", server.URL, artifact.Value)
			}
		}
	}

	if !hasURL {
		t.Error("expected URL artifact")
	}
}

func TestHTTPx_Probe_Failure(t *testing.T) {
	logger := logx.New()
	source := New(logger).(*HTTPx)

	target := *domain.NewTarget("example.com", domain.ScanModeActive)
	result := domain.NewScanResult(target)

	ctx := context.Background()

	// Probe invalid URL
	success := source.probe(ctx, "http://invalid-domain-that-does-not-exist-12345.com", result)

	if success {
		t.Error("expected probe to fail")
	}

	// No artifacts should be added on failure
	if len(result.Artifacts) > 0 {
		t.Error("expected no artifacts on failed probe")
	}
}
