// internal/adapters/output/json_test.go
package output

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aethonx/internal/core/domain"
)

func TestOutputJSON(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create test data
	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh"))
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeIP, "192.168.1.1", "dns"))
	result.Finalize()

	// Execute
	err := OutputJSON(tmpDir, result)
	if err != nil {
		t.Fatalf("OutputJSON() failed: %v", err)
	}

	// Verify subdirectory was created
	domainDir := filepath.Join(tmpDir, "example_com")
	files, err := os.ReadDir(domainDir)
	if err != nil {
		t.Fatalf("failed to read domain subdirectory: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file in subdirectory, got %d", len(files))
	}

	// Verify filename format
	filename := files[0].Name()
	if !strings.HasPrefix(filename, "aethonx_example.com_") {
		t.Errorf("filename should start with 'aethonx_example.com_', got %q", filename)
	}
	if !strings.HasSuffix(filename, ".json") {
		t.Errorf("filename should end with '.json', got %q", filename)
	}

	// Verify file content
	filePath := filepath.Join(domainDir, filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	// Verify JSON is valid
	var decodedResult domain.ScanResult
	if err := json.Unmarshal(data, &decodedResult); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	// Verify content matches
	if decodedResult.Target.Root != "example.com" {
		t.Errorf("Target.Root: expected %q, got %q", "example.com", decodedResult.Target.Root)
	}
	if len(decodedResult.Artifacts) != 2 {
		t.Errorf("Artifacts: expected 2, got %d", len(decodedResult.Artifacts))
	}

	// Verify JSON is indented (pretty-printed)
	if !strings.Contains(string(data), "\n") || !strings.Contains(string(data), "  ") {
		t.Error("JSON should be pretty-printed with indentation")
	}
}

func TestOutputJSON_EmptyDir(t *testing.T) {
	// Create test data
	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)

	// Execute with empty dir (should use current directory)
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	os.Chdir(tmpDir)

	err := OutputJSON("", result)
	if err != nil {
		t.Fatalf("OutputJSON() with empty dir failed: %v", err)
	}

	// Verify subdirectory was created in current directory
	domainDir := "./example_com"
	files, err := os.ReadDir(domainDir)
	if err != nil {
		t.Fatalf("failed to read domain subdirectory in current dir: %v", err)
	}

	found := false
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "aethonx_") && strings.HasSuffix(file.Name(), ".json") {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected JSON file to be created in current directory subdirectory")
	}
}

func TestOutputJSON_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "nested", "output", "dir")

	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)

	err := OutputJSON(outputDir, result)
	if err != nil {
		t.Fatalf("OutputJSON() failed to create nested directory: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Error("output directory should be created")
	}

	// Verify file exists
	files, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("failed to read output dir: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
}

func TestOutputJSON_InvalidDirectory(t *testing.T) {
	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)

	// Try to write to a file as if it were a directory
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "file.txt")
	os.WriteFile(invalidPath, []byte("test"), 0644)

	err := OutputJSON(filepath.Join(invalidPath, "subdir"), result)
	if err == nil {
		t.Error("OutputJSON() should fail with invalid directory path")
	}
}

func TestOutputJSON_TimestampFormat(t *testing.T) {
	tmpDir := t.TempDir()

	target := domain.NewTarget("test.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)

	err := OutputJSON(tmpDir, result)
	if err != nil {
		t.Fatalf("OutputJSON() failed: %v", err)
	}

	// Read subdirectory
	domainDir := filepath.Join(tmpDir, "test_com")
	files, err := os.ReadDir(domainDir)
	if err != nil {
		t.Fatalf("failed to read domain subdirectory: %v", err)
	}

	filename := files[0].Name()

	// Extract timestamp from filename: aethonx_test.com_20060102_150405.json
	parts := strings.Split(filename, "_")
	if len(parts) < 3 {
		t.Fatalf("unexpected filename format: %q", filename)
	}

	// Verify timestamp format (YYYYMMDD_HHMMSS)
	timestampPart := strings.TrimSuffix(strings.Join(parts[2:], "_"), ".json")

	// Parse timestamp to verify format
	_, err = time.Parse("20060102_150405", timestampPart)
	if err != nil {
		t.Errorf("timestamp format is invalid: %q, error: %v", timestampPart, err)
	}
}

func TestOutputJSON_WithComplexData(t *testing.T) {
	tmpDir := t.TempDir()

	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)

	// Add various artifacts
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "api.example.com", "crtsh"))
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeIP, "192.168.1.1", "dns"))
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeEmail, "admin@example.com", "whois"))

	// Add warnings
	result.AddWarning("crtsh", "Rate limit reached")

	// Add errors
	result.AddError("rdap", "Connection timeout", false)

	result.Finalize()

	err := OutputJSON(tmpDir, result)
	if err != nil {
		t.Fatalf("OutputJSON() failed: %v", err)
	}

	// Read and decode from subdirectory
	domainDir := filepath.Join(tmpDir, "example_com")
	files, err := os.ReadDir(domainDir)
	if err != nil {
		t.Fatalf("failed to read domain subdirectory: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(domainDir, files[0].Name()))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var decoded domain.ScanResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	// Verify all data is present
	if len(decoded.Artifacts) != 3 {
		t.Errorf("Artifacts: expected 3, got %d", len(decoded.Artifacts))
	}
	if len(decoded.Warnings) != 1 {
		t.Errorf("Warnings: expected 1, got %d", len(decoded.Warnings))
	}
	if len(decoded.Errors) != 1 {
		t.Errorf("Errors: expected 1, got %d", len(decoded.Errors))
	}
}

func TestOutputJSONStdout_Pretty(t *testing.T) {
	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh"))
	result.Finalize()

	// Redirect stdout to capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := OutputJSONStdout(result, true)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("OutputJSONStdout() failed: %v", err)
	}

	// Read captured output
	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	// Verify JSON is valid
	var decoded domain.ScanResult
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	// Verify pretty printing (should have indentation)
	if !strings.Contains(output, "\n") || !strings.Contains(output, "  ") {
		t.Error("JSON should be pretty-printed when pretty=true")
	}

	if decoded.Target.Root != "example.com" {
		t.Errorf("Target.Root: expected %q, got %q", "example.com", decoded.Target.Root)
	}
}

func TestOutputJSONStdout_Compact(t *testing.T) {
	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh"))
	result.Finalize()

	// Redirect stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := OutputJSONStdout(result, false)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("OutputJSONStdout() failed: %v", err)
	}

	// Read captured output
	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	// Verify JSON is valid
	var decoded domain.ScanResult
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	// Should be more compact (single line or fewer newlines)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) > 3 {
		// Compact JSON should have very few lines
		t.Logf("Compact JSON has %d lines, expected fewer for compact mode", len(lines))
	}
}

func TestOutputJSONStdout_EmptyResult(t *testing.T) {
	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)
	result.Finalize()

	// Redirect stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := OutputJSONStdout(result, true)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("OutputJSONStdout() failed: %v", err)
	}

	// Read output
	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	// Verify JSON is valid
	var decoded domain.ScanResult
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	// Should have no artifacts
	if len(decoded.Artifacts) != 0 {
		t.Errorf("Artifacts: expected 0, got %d", len(decoded.Artifacts))
	}
}
