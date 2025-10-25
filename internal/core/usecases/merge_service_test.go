// internal/core/usecases/merge_service_test.go
package usecases

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
	"aethonx/internal/testutil"
)

func TestMergeService_LoadPartialResults(t *testing.T) {
	// Crear directorio temporal
	tmpDir := t.TempDir()

	// Crear subdirectorio para el dominio (como lo hace OutputJSON)
	domainDir := filepath.Join(tmpDir, "example_com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("failed to create domain subdirectory: %v", err)
	}

	// Crear algunos archivos parciales de prueba
	partial1 := PartialScanResult{
		Source: "crtsh",
		ScanID: "scan-123",
		Target: "example.com",
		Artifacts: []*domain.Artifact{
			domain.NewArtifact(domain.ArtifactTypeSubdomain, "test1.example.com", "crtsh"),
			domain.NewArtifact(domain.ArtifactTypeSubdomain, "test2.example.com", "crtsh"),
		},
		ArtifactCount: 2,
	}

	partial2 := PartialScanResult{
		Source: "rdap",
		ScanID: "scan-123",
		Target: "example.com",
		Artifacts: []*domain.Artifact{
			domain.NewArtifact(domain.ArtifactTypeDomain, "example.com", "rdap"),
		},
		ArtifactCount: 1,
	}

	// Escribir archivos en el subdirectorio
	writePartialFile(t, domainDir, "aethonx_example.com_20250119_partial_crtsh.json", partial1)
	writePartialFile(t, domainDir, "aethonx_example.com_20250119_partial_rdap.json", partial2)

	// Crear servicio
	logger := logx.New()
	merger := NewMergeService(logger)

	// Cargar resultados parciales
	results, err := merger.LoadPartialResults(tmpDir, "aethonx_example.com_20250119_partial_*.json")

	// Verificar
	testutil.AssertNoError(t, err, "LoadPartialResults should succeed")
	testutil.AssertEqual(t, len(results), 2, "should load 2 partial results")

	// Verificar contenido
	totalArtifacts := 0
	for _, r := range results {
		totalArtifacts += len(r.Artifacts)
	}
	testutil.AssertEqual(t, totalArtifacts, 3, "should have 3 total artifacts")
}

func TestMergeService_LoadPartialResults_NoFiles(t *testing.T) {
	tmpDir := t.TempDir()

	logger := logx.New()
	merger := NewMergeService(logger)

	// Intentar cargar de directorio vacío
	results, err := merger.LoadPartialResults(tmpDir, "*.json")

	testutil.AssertNoError(t, err, "should not error on empty directory")
	testutil.AssertEqual(t, len(results), 0, "should return empty slice")
}

func TestMergeService_ConsolidateIntoResult(t *testing.T) {
	// Crear resultado base
	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)

	// Crear resultados parciales
	partials := []PartialScanResult{
		{
			Source: "crtsh",
			Artifacts: []*domain.Artifact{
				domain.NewArtifact(domain.ArtifactTypeSubdomain, "test1.example.com", "crtsh"),
			},
			Warnings: []domain.Warning{
				{Source: "crtsh", Message: "warning 1"},
			},
		},
		{
			Source: "rdap",
			Artifacts: []*domain.Artifact{
				domain.NewArtifact(domain.ArtifactTypeDomain, "example.com", "rdap"),
			},
			Errors: []domain.Error{
				{Source: "rdap", Message: "error 1", Fatal: false},
			},
		},
	}

	// Consolidar
	logger := logx.New()
	merger := NewMergeService(logger)
	err := merger.ConsolidateIntoResult(result, partials)

	// Verificar
	testutil.AssertNoError(t, err, "ConsolidateIntoResult should succeed")
	testutil.AssertEqual(t, len(result.Artifacts), 2, "should have 2 artifacts")
	testutil.AssertEqual(t, len(result.Warnings), 1, "should have 1 warning")
	testutil.AssertEqual(t, len(result.Errors), 1, "should have 1 error")
}

func TestMergeService_LoadPartialResults_InvalidDir(t *testing.T) {
	logger := logx.New()
	merger := NewMergeService(logger)

	results, err := merger.LoadPartialResults("", "*.json")

	testutil.AssertTrue(t, err != nil, "should fail with empty directory")
	testutil.AssertTrue(t, results == nil, "results should be nil on error")
}

func TestMergeService_LoadPartialResults_InvalidPattern(t *testing.T) {
	logger := logx.New()
	merger := NewMergeService(logger)

	results, err := merger.LoadPartialResults("/tmp", "")

	testutil.AssertTrue(t, err != nil, "should fail with empty pattern")
	testutil.AssertTrue(t, results == nil, "results should be nil on error")
}

func TestMergeService_ConsolidateIntoResult_NilResult(t *testing.T) {
	logger := logx.New()
	merger := NewMergeService(logger)

	partials := []PartialScanResult{{Source: "test"}}
	err := merger.ConsolidateIntoResult(nil, partials)

	testutil.AssertTrue(t, err != nil, "should fail with nil result")
}

func TestMergeService_ClearPartialFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Crear subdirectorio para el dominio (como lo hace OutputJSON)
	domainDir := filepath.Join(tmpDir, "example_com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("failed to create domain subdirectory: %v", err)
	}

	// Crear algunos archivos parciales en el subdirectorio
	file1 := filepath.Join(domainDir, "aethonx_example.com_20250119_partial_crtsh.json")
	file2 := filepath.Join(domainDir, "aethonx_example.com_20250119_partial_rdap.json")

	_ = os.WriteFile(file1, []byte(`{"source":"crtsh"}`), 0o644)
	_ = os.WriteFile(file2, []byte(`{"source":"rdap"}`), 0o644)

	// Verificar que existen
	_, err1 := os.Stat(file1)
	_, err2 := os.Stat(file2)
	testutil.AssertNoError(t, err1, "file1 should exist before clearing")
	testutil.AssertNoError(t, err2, "file2 should exist before clearing")

	// Limpiar archivos
	logger := logx.New()
	merger := NewMergeService(logger)
	err := merger.ClearPartialFiles(tmpDir, "aethonx_example.com_20250119_partial_*.json")

	testutil.AssertNoError(t, err, "ClearPartialFiles should succeed")

	// Verificar que fueron eliminados
	_, err1 = os.Stat(file1)
	_, err2 = os.Stat(file2)
	testutil.AssertTrue(t, os.IsNotExist(err1), "file1 should be deleted")
	testutil.AssertTrue(t, os.IsNotExist(err2), "file2 should be deleted")
}

// Helper para escribir archivos parciales en tests
func writePartialFile(t *testing.T, dir, filename string, partial PartialScanResult) {
	t.Helper()

	filepath := filepath.Join(dir, filename)
	data, err := json.MarshalIndent(partial, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal partial: %v", err)
	}

	err = os.WriteFile(filepath, data, 0o644)
	if err != nil {
		t.Fatalf("failed to write partial file: %v", err)
	}
}
