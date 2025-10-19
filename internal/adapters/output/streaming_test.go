// internal/adapters/output/streaming_test.go
package output

import (
	"os"
	"path/filepath"
	"testing"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
	"aethonx/internal/testutil"
)

func TestStreamingWriter_WritePartial(t *testing.T) {
	// Crear directorio temporal
	tmpDir := t.TempDir()

	// Crear writer
	logger := logx.New()
	writer := NewStreamingWriter(tmpDir, "test-scan-123", "example.com", logger)

	// Crear resultado de prueba con artifacts
	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)

	// Añadir algunos artifacts
	artifact1 := domain.NewArtifact(domain.ArtifactTypeSubdomain, "test1.example.com", "crtsh")
	artifact2 := domain.NewArtifact(domain.ArtifactTypeSubdomain, "test2.example.com", "crtsh")
	result.AddArtifact(artifact1)
	result.AddArtifact(artifact2)
	result.AddWarning("crtsh", "test warning")

	// Escribir parcial
	filepath, err := writer.WritePartial("crtsh", result)

	// Verificar que no hubo error
	testutil.AssertNoError(t, err, "WritePartial should succeed")

	// Verificar que el archivo existe
	_, statErr := os.Stat(filepath)
	testutil.AssertNoError(t, statErr, "partial file should exist")

	// Verificar que el archivo contiene datos válidos
	data, readErr := os.ReadFile(filepath)
	testutil.AssertNoError(t, readErr, "should read partial file")
	testutil.AssertTrue(t, len(data) > 0, "partial file should not be empty")
}

func TestStreamingWriter_GeneratePartialFilename(t *testing.T) {
	logger := logx.New()
	writer := NewStreamingWriter("/tmp", "scan-123", "example.com", logger)

	filename := writer.GeneratePartialFilename("crtsh")

	testutil.AssertTrue(t, len(filename) > 0, "filename should not be empty")
	testutil.AssertContains(t, filename, "example.com", "filename should contain target")
	testutil.AssertContains(t, filename, "partial", "filename should contain 'partial'")
	testutil.AssertContains(t, filename, "crtsh", "filename should contain source name")
	testutil.AssertContains(t, filename, ".json", "filename should have .json extension")
}

func TestStreamingWriter_GetPattern(t *testing.T) {
	logger := logx.New()
	writer := NewStreamingWriter("/tmp", "scan-123", "example.com", logger)

	pattern := writer.GetPattern()

	testutil.AssertContains(t, pattern, "example.com", "pattern should contain target")
	testutil.AssertContains(t, pattern, "partial_*.json", "pattern should have wildcard for sources")
}

func TestStreamingWriter_GetFinalFilename(t *testing.T) {
	logger := logx.New()
	writer := NewStreamingWriter("/tmp", "scan-123", "example.com", logger)

	filename := writer.GetFinalFilename()

	testutil.AssertContains(t, filename, "example.com", "filename should contain target")
	testutil.AssertContains(t, filename, ".json", "filename should have .json extension")
	testutil.AssertTrue(t, !testutil.ContainsStr(filename, "partial"), "final filename should not contain 'partial'")
}

func TestStreamingWriter_WritePartial_CreatesDirectory(t *testing.T) {
	// Usar directorio que no existe
	tmpDir := filepath.Join(t.TempDir(), "subdir", "nested")

	logger := logx.New()
	writer := NewStreamingWriter(tmpDir, "scan-123", "example.com", logger)

	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)
	artifact := domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "rdap")
	result.AddArtifact(artifact)

	// Escribir parcial debería crear el directorio
	_, err := writer.WritePartial("rdap", result)

	testutil.AssertNoError(t, err, "WritePartial should create directory and succeed")

	// Verificar que el directorio fue creado
	_, statErr := os.Stat(tmpDir)
	testutil.AssertNoError(t, statErr, "directory should be created")
}
