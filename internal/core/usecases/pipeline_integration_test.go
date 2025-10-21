package usecases

import (
	"context"
	"testing"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
)

// MockPassiveSource simula una source de Stage 0 (sin inputs)
type MockPassiveSource struct {
	name string
}

func (m *MockPassiveSource) Name() string                                 { return m.name }
func (m *MockPassiveSource) Mode() domain.SourceMode                      { return domain.SourceModePassive }
func (m *MockPassiveSource) Type() domain.SourceType                      { return domain.SourceTypeAPI }
func (m *MockPassiveSource) Close() error                                 { return nil }
func (m *MockPassiveSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	result := domain.NewScanResult(target)

	// Simular descubrimiento de subdomains
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "api."+target.Root, m.name))
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "www."+target.Root, m.name))
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeDomain, target.Root, m.name))

	return result, nil
}

// MockActiveSource simula una source de Stage 1 (consume subdomains)
type MockActiveSource struct {
	name string
}

func (m *MockActiveSource) Name() string                                 { return m.name }
func (m *MockActiveSource) Mode() domain.SourceMode                      { return domain.SourceModeActive }
func (m *MockActiveSource) Type() domain.SourceType                      { return domain.SourceTypeBuiltin }
func (m *MockActiveSource) Close() error                                 { return nil }
func (m *MockActiveSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	// Fallback sin inputs
	return domain.NewScanResult(target), nil
}

func (m *MockActiveSource) RunWithInput(ctx context.Context, target domain.Target, input *domain.ScanResult) (*domain.ScanResult, error) {
	result := domain.NewScanResult(target)

	// Procesar subdomains del input y generar URLs
	for _, artifact := range input.Artifacts {
		if artifact.Type == domain.ArtifactTypeSubdomain || artifact.Type == domain.ArtifactTypeDomain {
			url := "https://" + artifact.Value
			result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeURL, url, m.name))

			// Simular descubrimiento de IP
			ip := "192.168.1.1" // Mock IP
			result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeIP, ip, m.name))
		}
	}

	return result, nil
}

// TestPipelineOrchestrator_MultiStage prueba la ejecución de un pipeline con múltiples stages
func TestPipelineOrchestrator_MultiStage(t *testing.T) {
	logger := logx.New()

	// Crear sources con dependencias
	passiveSource1 := &MockPassiveSource{name: "crtsh-mock"}
	passiveSource2 := &MockPassiveSource{name: "rdap-mock"}
	activeSource := &MockActiveSource{name: "httpx-mock"}

	// Registrar metadata de sources
	sourceMetadata := map[string]ports.SourceMetadata{
		"crtsh-mock": {
			Name:            "crtsh-mock",
			InputArtifacts:  []domain.ArtifactType{}, // Stage 0
			OutputArtifacts: []domain.ArtifactType{domain.ArtifactTypeSubdomain, domain.ArtifactTypeDomain},
			Priority:        10,
		},
		"rdap-mock": {
			Name:            "rdap-mock",
			InputArtifacts:  []domain.ArtifactType{}, // Stage 0
			OutputArtifacts: []domain.ArtifactType{domain.ArtifactTypeDomain},
			Priority:        8,
		},
		"httpx-mock": {
			Name:            "httpx-mock",
			InputArtifacts:  []domain.ArtifactType{domain.ArtifactTypeSubdomain, domain.ArtifactTypeDomain}, // Stage 1
			OutputArtifacts: []domain.ArtifactType{domain.ArtifactTypeURL, domain.ArtifactTypeIP},
			Priority:        7,
		},
	}

	sources := []ports.Source{passiveSource1, passiveSource2, activeSource}

	// Crear pipeline orchestrator
	orchestrator := NewPipelineOrchestrator(PipelineOrchestratorOptions{
		Sources:        sources,
		SourceMetadata: sourceMetadata,
		Logger:         logger,
		MaxWorkers:     2,
		StreamingConfig: StreamingConfig{
			ArtifactThreshold: 1000,
		},
	})

	target := *domain.NewTarget("example.com", domain.ScanModeHybrid)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Ejecutar pipeline
	result, err := orchestrator.Run(ctx, target)

	if err != nil {
		t.Fatalf("pipeline execution failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// Verificar que se ejecutaron ambos stages
	t.Logf("Total artifacts: %d", len(result.Artifacts))
	t.Logf("Stats: %+v", result.Stats())

	// Stage 0 debería producir: 3 artifacts por source * 2 sources = 6 (antes de dedupe)
	// Stage 1 debería producir: URLs + IPs a partir de los 3 subdominios/dominios únicos

	stats := result.Stats()

	// Verificar que tenemos subdomains (de Stage 0)
	if stats[string(domain.ArtifactTypeSubdomain)] == 0 {
		t.Error("expected subdomains from Stage 0")
	}

	// Verificar que tenemos URLs (de Stage 1)
	if stats[string(domain.ArtifactTypeURL)] == 0 {
		t.Error("expected URLs from Stage 1 (httpx-mock)")
	}

	// Verificar que tenemos IPs (de Stage 1)
	if stats[string(domain.ArtifactTypeIP)] == 0 {
		t.Error("expected IPs from Stage 1 (httpx-mock)")
	}

	t.Logf("Pipeline test passed successfully!")
	t.Logf("  - Subdomains: %d", stats[string(domain.ArtifactTypeSubdomain)])
	t.Logf("  - Domains: %d", stats[string(domain.ArtifactTypeDomain)])
	t.Logf("  - URLs: %d", stats[string(domain.ArtifactTypeURL)])
	t.Logf("  - IPs: %d", stats[string(domain.ArtifactTypeIP)])
}

// TestPipelineOrchestrator_BuildStages prueba la construcción de stages
func TestPipelineOrchestrator_BuildStages(t *testing.T) {
	logger := logx.New()

	passiveSource := &MockPassiveSource{name: "stage0-source"}
	activeSource := &MockActiveSource{name: "stage1-source"}

	sourceMetadata := map[string]ports.SourceMetadata{
		"stage0-source": {
			Name:            "stage0-source",
			InputArtifacts:  []domain.ArtifactType{},
			OutputArtifacts: []domain.ArtifactType{domain.ArtifactTypeSubdomain},
		},
		"stage1-source": {
			Name:            "stage1-source",
			InputArtifacts:  []domain.ArtifactType{domain.ArtifactTypeSubdomain},
			OutputArtifacts: []domain.ArtifactType{domain.ArtifactTypeURL},
		},
	}

	orchestrator := NewPipelineOrchestrator(PipelineOrchestratorOptions{
		Sources:        []ports.Source{passiveSource, activeSource},
		SourceMetadata: sourceMetadata,
		Logger:         logger,
	})

	sources := []ports.Source{passiveSource, activeSource}
	stages, err := orchestrator.BuildStages(sources)

	if err != nil {
		t.Fatalf("BuildStages failed: %v", err)
	}

	if len(stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(stages))
	}

	// Verificar Stage 0
	stage0 := stages[0]
	if stage0.ID != 0 {
		t.Errorf("expected stage 0 ID=0, got %d", stage0.ID)
	}

	if len(stage0.Sources) != 1 {
		t.Errorf("expected 1 source in stage 0, got %d", len(stage0.Sources))
	}

	if stage0.Sources[0].Name() != "stage0-source" {
		t.Errorf("expected stage0-source in stage 0, got %s", stage0.Sources[0].Name())
	}

	// Verificar Stage 1
	stage1 := stages[1]
	if stage1.ID != 1 {
		t.Errorf("expected stage 1 ID=1, got %d", stage1.ID)
	}

	if len(stage1.Sources) != 1 {
		t.Errorf("expected 1 source in stage 1, got %d", len(stage1.Sources))
	}

	if stage1.Sources[0].Name() != "stage1-source" {
		t.Errorf("expected stage1-source in stage 1, got %s", stage1.Sources[0].Name())
	}

	t.Logf("Stage 0: %s (sources=%d)", stage0.Name, len(stage0.Sources))
	t.Logf("Stage 1: %s (sources=%d)", stage1.Name, len(stage1.Sources))
}

// TestPipelineOrchestrator_CircularDependency prueba detección de ciclos
func TestPipelineOrchestrator_CircularDependency(t *testing.T) {
	logger := logx.New()

	// Crear sources con dependencia circular: A → B → A
	sourceA := &MockActiveSource{name: "sourceA"}
	sourceB := &MockActiveSource{name: "sourceB"}

	sourceMetadata := map[string]ports.SourceMetadata{
		"sourceA": {
			Name:            "sourceA",
			InputArtifacts:  []domain.ArtifactType{domain.ArtifactTypeURL},    // Requires URL
			OutputArtifacts: []domain.ArtifactType{domain.ArtifactTypeSubdomain}, // Produces subdomain
		},
		"sourceB": {
			Name:            "sourceB",
			InputArtifacts:  []domain.ArtifactType{domain.ArtifactTypeSubdomain}, // Requires subdomain
			OutputArtifacts: []domain.ArtifactType{domain.ArtifactTypeURL},        // Produces URL
		},
	}

	orchestrator := NewPipelineOrchestrator(PipelineOrchestratorOptions{
		Sources:        []ports.Source{sourceA, sourceB},
		SourceMetadata: sourceMetadata,
		Logger:         logger,
	})

	sources := []ports.Source{sourceA, sourceB}
	_, err := orchestrator.BuildStages(sources)

	if err == nil {
		t.Fatal("expected circular dependency error, got nil")
	}

	t.Logf("Circular dependency detected correctly: %v", err)
}

// TestPipelineOrchestrator_InputConsumerIntegration verifica que InputConsumer se llama correctamente
func TestPipelineOrchestrator_InputConsumerIntegration(t *testing.T) {
	logger := logx.New()

	// Crear source pasiva que genera subdominios
	passiveSource := &MockPassiveSource{name: "crtsh-test"}

	// Crear source activa con InputConsumer que cuenta cuántos inputs recibe
	inputReceivedCount := 0
	activeSource := &mockInputConsumerSource{
		name: "httpx-test",
		onRunWithInput: func(ctx context.Context, target domain.Target, input *domain.ScanResult) (*domain.ScanResult, error) {
			inputReceivedCount = len(input.Artifacts)
			result := domain.NewScanResult(target)

			// Generar URLs a partir de los inputs
			for _, artifact := range input.Artifacts {
				if artifact.Type == domain.ArtifactTypeSubdomain || artifact.Type == domain.ArtifactTypeDomain {
					url := "https://" + artifact.Value
					result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeURL, url, "httpx-test"))
				}
			}

			return result, nil
		},
	}

	// Metadata de sources
	sourceMetadata := map[string]ports.SourceMetadata{
		"crtsh-test": {
			Name:            "crtsh-test",
			InputArtifacts:  []domain.ArtifactType{},
			OutputArtifacts: []domain.ArtifactType{domain.ArtifactTypeSubdomain, domain.ArtifactTypeDomain},
			Priority:        10,
		},
		"httpx-test": {
			Name:            "httpx-test",
			InputArtifacts:  []domain.ArtifactType{domain.ArtifactTypeSubdomain, domain.ArtifactTypeDomain},
			OutputArtifacts: []domain.ArtifactType{domain.ArtifactTypeURL},
			Priority:        5,
		},
	}

	sources := []ports.Source{passiveSource, activeSource}

	orchestrator := NewPipelineOrchestrator(PipelineOrchestratorOptions{
		Sources:        sources,
		SourceMetadata: sourceMetadata,
		Logger:         logger,
		MaxWorkers:     2,
	})

	target := *domain.NewTarget("example.com", domain.ScanModeHybrid)
	ctx := context.Background()

	result, err := orchestrator.Run(ctx, target)

	if err != nil {
		t.Fatalf("pipeline execution failed: %v", err)
	}

	// Verificar que RunWithInput fue llamado con los artifacts correctos
	// crtsh-test genera: api.example.com, www.example.com, example.com
	// www.example.com se normaliza a example.com, entonces deberíamos tener 2 únicos después de dedupe
	// PERO el input que recibe httpx-test es ANTES del dedupe final, así que recibe 3
	expectedInputCount := 3
	if inputReceivedCount != expectedInputCount {
		t.Errorf("expected RunWithInput to receive %d artifacts, got %d", expectedInputCount, inputReceivedCount)
	}

	// Verificar que se generaron URLs a partir de los inputs
	// Nota: Los URLs también se deduplicarán si los hosts se normalizan igual
	stats := result.Stats()
	urlCount := stats[string(domain.ArtifactTypeURL)]
	if urlCount < 1 {
		t.Errorf("expected at least 1 URL from InputConsumer, got %d", urlCount)
	}

	t.Logf("InputConsumer integration test passed!")
	t.Logf("  - Inputs received by httpx-test: %d", inputReceivedCount)
	t.Logf("  - URLs generated: %d", urlCount)
	t.Logf("  - Total artifacts: %d", len(result.Artifacts))
}

// mockInputConsumerSource es un mock que implementa InputConsumer
type mockInputConsumerSource struct {
	name           string
	onRunWithInput func(context.Context, domain.Target, *domain.ScanResult) (*domain.ScanResult, error)
}

func (m *mockInputConsumerSource) Name() string          { return m.name }
func (m *mockInputConsumerSource) Mode() domain.SourceMode { return domain.SourceModeActive }
func (m *mockInputConsumerSource) Type() domain.SourceType { return domain.SourceTypeBuiltin }
func (m *mockInputConsumerSource) Close() error           { return nil }

func (m *mockInputConsumerSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	// Fallback sin inputs (no debería ser llamado si hay inputs)
	return domain.NewScanResult(target), nil
}

func (m *mockInputConsumerSource) RunWithInput(ctx context.Context, target domain.Target, input *domain.ScanResult) (*domain.ScanResult, error) {
	if m.onRunWithInput != nil {
		return m.onRunWithInput(ctx, target, input)
	}
	return domain.NewScanResult(target), nil
}
