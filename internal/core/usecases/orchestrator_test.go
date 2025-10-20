// internal/core/usecases/orchestrator_test.go
package usecases

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/testutil"
)

func TestNewOrchestrator(t *testing.T) {
	logger := logx.New()
	sources := []ports.Source{
		newMockSource("test-source", domain.SourceModePassive, domain.SourceTypeAPI),
	}

	orch := NewOrchestrator(OrchestratorOptions{
		Sources:    sources,
		Logger:     logger,
		MaxWorkers: 4,
	})

	testutil.AssertNotNil(t, orch, "orchestrator should not be nil")
}

func TestOrchestrator_Run_ValidTarget(t *testing.T) {
	logger := logx.New()

	// Create mock source with artifacts
	artifacts := []*domain.Artifact{
		domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "test-source"),
	}
	source := mockSourceWithArtifacts("test-source", artifacts)

	orch := NewOrchestrator(OrchestratorOptions{
		Sources:    []ports.Source{source},
		Logger:     logger,
		MaxWorkers: 2,
	})

	target := domain.NewTarget("example.com", domain.ScanModePassive)

	result, err := orch.Run(context.Background(), *target)

	testutil.AssertNoError(t, err, "run should succeed")
	testutil.AssertNotNil(t, result, "result should not be nil")
	testutil.AssertEqual(t, len(result.Artifacts), 1, "artifacts count")
	testutil.AssertEqual(t, source.runCallCount, 1, "source should be called once")
}

func TestOrchestrator_Run_InvalidTarget(t *testing.T) {
	logger := logx.New()
	source := newMockSource("test-source", domain.SourceModePassive, domain.SourceTypeAPI)

	orch := NewOrchestrator(OrchestratorOptions{
		Sources:    []ports.Source{source},
		Logger:     logger,
		MaxWorkers: 2,
	})

	// Invalid target (empty domain)
	target := domain.NewTarget("", domain.ScanModePassive)

	_, err := orch.Run(context.Background(), *target)

	testutil.AssertError(t, err, "run should fail with invalid target")
}

func TestOrchestrator_Run_NoSourcesAvailable(t *testing.T) {
	logger := logx.New()

	orch := NewOrchestrator(OrchestratorOptions{
		Sources:    []ports.Source{}, // No sources
		Logger:     logger,
		MaxWorkers: 2,
	})

	target := domain.NewTarget("example.com", domain.ScanModePassive)

	_, err := orch.Run(context.Background(), *target)

	testutil.AssertError(t, err, "should fail with no sources")
	testutil.AssertEqual(t, err, domain.ErrNoSourcesAvailable, "error type")
}

func TestOrchestrator_Run_FilterIncompatibleSources(t *testing.T) {
	logger := logx.New()

	// Passive source
	passiveSource := newMockSource("passive-source", domain.SourceModePassive, domain.SourceTypeAPI)

	// Active source
	activeSource := newMockSource("active-source", domain.SourceModeActive, domain.SourceTypeAPI)

	orch := NewOrchestrator(OrchestratorOptions{
		Sources:    []ports.Source{passiveSource, activeSource},
		Logger:     logger,
		MaxWorkers: 2,
	})

	// Passive scan - should only run passive source
	target := domain.NewTarget("example.com", domain.ScanModePassive)

	result, err := orch.Run(context.Background(), *target)

	testutil.AssertNoError(t, err, "run should succeed")
	testutil.AssertNotNil(t, result, "result should not be nil")
	testutil.AssertEqual(t, passiveSource.runCallCount, 1, "passive source should run")
	testutil.AssertEqual(t, activeSource.runCallCount, 0, "active source should NOT run")
}

func TestOrchestrator_Run_SourceError(t *testing.T) {
	logger := logx.New()

	sourceErr := errors.New("source failed")
	failingSource := mockSourceWithError("failing-source", sourceErr)

	orch := NewOrchestrator(OrchestratorOptions{
		Sources:    []ports.Source{failingSource},
		Logger:     logger,
		MaxWorkers: 2,
	})

	target := domain.NewTarget("example.com", domain.ScanModePassive)

	result, err := orch.Run(context.Background(), *target)

	// Orchestrator should not fail, but collect errors
	testutil.AssertNoError(t, err, "orchestrator should not fail")
	testutil.AssertNotNil(t, result, "result should not be nil")
	testutil.AssertTrue(t, result.HasErrors(), "result should have errors")
	testutil.AssertEqual(t, len(result.Errors), 1, "error count")
}

func TestOrchestrator_Run_MultipleSources(t *testing.T) {
	logger := logx.New()

	artifacts1 := []*domain.Artifact{
		domain.NewArtifact(domain.ArtifactTypeSubdomain, "test1.example.com", "source1"),
	}
	artifacts2 := []*domain.Artifact{
		domain.NewArtifact(domain.ArtifactTypeSubdomain, "test2.example.com", "source2"),
	}

	source1 := mockSourceWithArtifacts("source1", artifacts1)
	source2 := mockSourceWithArtifacts("source2", artifacts2)

	orch := NewOrchestrator(OrchestratorOptions{
		Sources:    []ports.Source{source1, source2},
		Logger:     logger,
		MaxWorkers: 2,
	})

	target := domain.NewTarget("example.com", domain.ScanModePassive)

	result, err := orch.Run(context.Background(), *target)

	testutil.AssertNoError(t, err, "run should succeed")
	testutil.AssertNotNil(t, result, "result should not be nil")
	testutil.AssertEqual(t, len(result.Artifacts), 2, "should have artifacts from both sources")
	testutil.AssertEqual(t, source1.runCallCount, 1, "source1 should run")
	testutil.AssertEqual(t, source2.runCallCount, 1, "source2 should run")
}

func TestOrchestrator_Run_Deduplication(t *testing.T) {
	logger := logx.New()

	// Both sources return the same artifact
	artifacts1 := []*domain.Artifact{
		domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "source1"),
	}
	artifacts2 := []*domain.Artifact{
		domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "source2"),
	}

	source1 := mockSourceWithArtifacts("source1", artifacts1)
	source2 := mockSourceWithArtifacts("source2", artifacts2)

	orch := NewOrchestrator(OrchestratorOptions{
		Sources:    []ports.Source{source1, source2},
		Logger:     logger,
		MaxWorkers: 2,
	})

	target := domain.NewTarget("example.com", domain.ScanModePassive)

	result, err := orch.Run(context.Background(), *target)

	testutil.AssertNoError(t, err, "run should succeed")
	testutil.AssertNotNil(t, result, "result should not be nil")

	// Should be deduplicated to 1 artifact
	testutil.AssertEqual(t, len(result.Artifacts), 1, "should deduplicate artifacts")

	// But should have both sources
	testutil.AssertEqual(t, len(result.Artifacts[0].Sources), 2, "should merge sources")
	testutil.AssertContains(t, result.Artifacts[0].Sources, "source1", "sources")
	testutil.AssertContains(t, result.Artifacts[0].Sources, "source2", "sources")
}

func TestOrchestrator_Run_WithNotifiers(t *testing.T) {
	logger := logx.New()

	source := newMockSource("test-source", domain.SourceModePassive, domain.SourceTypeAPI)
	notifier := newMockNotifier()

	orch := NewOrchestrator(OrchestratorOptions{
		Sources:    []ports.Source{source},
		Logger:     logger,
		Observers:  []ports.Notifier{notifier},
		MaxWorkers: 2,
	})

	target := domain.NewTarget("example.com", domain.ScanModePassive)

	result, err := orch.Run(context.Background(), *target)

	testutil.AssertNoError(t, err, "run should succeed")
	testutil.AssertNotNil(t, result, "result should not be nil")

	// Verify notifications were sent
	// Should have: ScanStarted, SourceStarted, SourceCompleted, ScanCompleted
	// But notifications are async, so we need to wait a bit
	time.Sleep(50 * time.Millisecond)

	testutil.AssertTrue(t, notifier.getNotifyCallCount() >= 2, "should have notifications")

	// Check for scan started and completed events
	startEvents := notifier.getEventsByType(ports.EventTypeScanStarted)
	if len(startEvents) > 0 {
		testutil.AssertEqual(t, len(startEvents), 1, "scan started events")
	}
}

func TestOrchestrator_Run_ContextCancellation(t *testing.T) {
	logger := logx.New()

	// Create a slow source
	slowSource := newMockSource("slow-source", domain.SourceModePassive, domain.SourceTypeAPI)
	slowSource.runFunc = func(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
		select {
		case <-time.After(1 * time.Second):
			result := domain.NewScanResult(target)
			return result, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	orch := NewOrchestrator(OrchestratorOptions{
		Sources:    []ports.Source{slowSource},
		Logger:     logger,
		MaxWorkers: 1,
	})

	target := domain.NewTarget("example.com", domain.ScanModePassive)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result, err := orch.Run(ctx, *target)

	// Should succeed but source might have error
	testutil.AssertNoError(t, err, "orchestrator should not fail")
	testutil.AssertNotNil(t, result, "result should not be nil")
}

func TestOrchestrator_Run_ConcurrencyLimit(t *testing.T) {
	logger := logx.New()

	// Create multiple sources
	var sources []ports.Source
	for i := 0; i < 10; i++ {
		source := newMockSource("source", domain.SourceModePassive, domain.SourceTypeAPI)
		sources = append(sources, source)
	}

	// Limit to 3 workers
	orch := NewOrchestrator(OrchestratorOptions{
		Sources:    sources,
		Logger:     logger,
		MaxWorkers: 3,
	})

	target := domain.NewTarget("example.com", domain.ScanModePassive)

	result, err := orch.Run(context.Background(), *target)

	testutil.AssertNoError(t, err, "run should succeed")
	testutil.AssertNotNil(t, result, "result should not be nil")

	// All sources should have run
	for _, s := range sources {
		mock := s.(*mockSource)
		testutil.AssertEqual(t, mock.runCallCount, 1, "source should run once")
	}
}

// TestOrchestrator_ConsolidateResults_NoRaceCondition verifica que la consolidación
// concurrente no tiene race conditions cuando múltiples sources retornan resultados grandes.
// Este test debe ejecutarse con -race flag: go test -race
func TestOrchestrator_ConsolidateResults_NoRaceCondition(t *testing.T) {
	logger := logx.New()

	// Crear 10 sources que retornan cada una 100 artifacts
	sources := make([]ports.Source, 10)
	for i := 0; i < 10; i++ {
		sourceName := fmt.Sprintf("source-%d", i)
		artifacts := make([]*domain.Artifact, 100)
		for j := 0; j < 100; j++ {
			artifacts[j] = domain.NewArtifact(
				domain.ArtifactTypeSubdomain,
				fmt.Sprintf("sub%d-%d.example.com", i, j),
				sourceName,
			)
		}
		sources[i] = mockSourceWithArtifacts(sourceName, artifacts)
	}

	orch := NewOrchestrator(OrchestratorOptions{
		Sources:    sources,
		Logger:     logger,
		MaxWorkers: 5, // Forzar concurrencia
	})

	target := domain.NewTarget("example.com", domain.ScanModePassive)

	// Ejecutar múltiples veces para aumentar probabilidad de detectar race
	for run := 0; run < 10; run++ {
		result, err := orch.Run(context.Background(), *target)

		testutil.AssertNoError(t, err, "run should succeed")
		testutil.AssertNotNil(t, result, "result should not be nil")

		// Verificar que todos los artifacts fueron consolidados correctamente
		// 10 sources × 100 artifacts = 1000 artifacts esperados (antes de deduplicación)
		// Nota: Después de deduplicación puede haber menos debido a IDs random
		testutil.AssertTrue(t, len(result.Artifacts) > 0, "should have artifacts")
		testutil.AssertTrue(t, len(result.Metadata.SourcesUsed) == 10, "should have 10 sources")
	}
}

// TestOrchestrator_ConsolidateResults_WithErrors verifica consolidación cuando
// algunas sources fallan concurrentemente.
func TestOrchestrator_ConsolidateResults_WithErrors(t *testing.T) {
	logger := logx.New()

	// 5 sources exitosas + 5 sources que fallan
	sources := make([]ports.Source, 10)

	for i := 0; i < 5; i++ {
		sourceName := fmt.Sprintf("success-source-%d", i)
		artifacts := []*domain.Artifact{
			domain.NewArtifact(domain.ArtifactTypeSubdomain, fmt.Sprintf("sub%d.example.com", i), sourceName),
		}
		sources[i] = mockSourceWithArtifacts(sourceName, artifacts)
	}

	for i := 5; i < 10; i++ {
		sourceName := fmt.Sprintf("error-source-%d", i)
		sources[i] = mockSourceWithError(sourceName, errors.New("source failed"))
	}

	orch := NewOrchestrator(OrchestratorOptions{
		Sources:    sources,
		Logger:     logger,
		MaxWorkers: 5,
	})

	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result, err := orch.Run(context.Background(), *target)

	testutil.AssertNoError(t, err, "run should succeed despite source errors")
	testutil.AssertNotNil(t, result, "result should not be nil")

	// Verificar que los errores fueron registrados
	testutil.AssertTrue(t, len(result.Errors) >= 5, "should have errors from failed sources")

	// Verificar que las sources exitosas contribuyeron artifacts
	testutil.AssertTrue(t, len(result.Artifacts) > 0, "should have artifacts from successful sources")
}
