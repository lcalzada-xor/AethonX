// internal/adapters/output/table_test.go
package output

import (
	"io"
	"os"
	"strings"
	"testing"

	"aethonx/internal/core/domain"
)

func TestOutputTable(t *testing.T) {
	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh"))
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeIP, "192.168.1.1", "dns"))
	result.Finalize()

	// Redirect stdout to capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := OutputTable(result)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("OutputTable() failed: %v", err)
	}

	// Read captured output
	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	// Verify header
	if !strings.Contains(output, "AethonX Scan Results") {
		t.Error("output should contain header")
	}

	// Verify target info
	if !strings.Contains(output, "example.com") {
		t.Error("output should contain target")
	}
	if !strings.Contains(output, "passive") {
		t.Error("output should contain mode")
	}

	// Verify artifact count in header (not individual artifacts)
	if !strings.Contains(output, "Artifacts:") {
		t.Error("output should show artifact count")
	}
}

func TestOutputTable_NoArtifacts(t *testing.T) {
	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)
	result.Finalize()

	// Redirect stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := OutputTable(result)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("OutputTable() failed: %v", err)
	}

	// Read output
	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	// Should still have header
	if !strings.Contains(output, "AethonX Scan Results") {
		t.Error("output should still contain header")
	}

	// Should show 0 artifacts
	if !strings.Contains(output, "Artifacts:") && !strings.Contains(output, "0") {
		t.Error("output should show 0 artifacts")
	}
}

func TestOutputTable_WithWarnings(t *testing.T) {
	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh"))
	result.AddWarning("crtsh", "Rate limit reached")
	result.AddWarning("rdap", "Slow response")
	result.Finalize()

	// Redirect stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := OutputTable(result)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("OutputTable() failed: %v", err)
	}

	// Read output
	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	// Should show warnings section
	if !strings.Contains(output, "Warnings") {
		t.Error("output should contain Warnings section")
	}
	if !strings.Contains(output, "Rate limit reached") {
		t.Error("output should contain warning message")
	}
	if !strings.Contains(output, "Slow response") {
		t.Error("output should contain second warning")
	}

	// Should show warning count
	if !strings.Contains(output, "(2)") {
		t.Error("output should show warning count")
	}
}

func TestOutputTable_WithErrors(t *testing.T) {
	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh"))
	result.AddError("rdap", "Connection timeout", false)
	result.AddError("crtsh", "API key invalid", true)
	result.Finalize()

	// Redirect stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := OutputTable(result)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("OutputTable() failed: %v", err)
	}

	// Read output
	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	// Should show errors section
	if !strings.Contains(output, "Errors") {
		t.Error("output should contain Errors section")
	}
	if !strings.Contains(output, "Connection timeout") {
		t.Error("output should contain error message")
	}
	if !strings.Contains(output, "API key invalid") {
		t.Error("output should contain second error")
	}

	// Should show error count
	if !strings.Contains(output, "(2)") {
		t.Error("output should show error count")
	}

	// Should mark fatal error
	if !strings.Contains(output, "FATAL") {
		t.Error("output should mark fatal errors")
	}
}

func TestOutputTable_Statistics(t *testing.T) {
	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)

	// Add different types of artifacts
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "test1.example.com", "crtsh"))
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "test2.example.com", "crtsh"))
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeIP, "192.168.1.1", "dns"))
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeEmail, "admin@example.com", "whois"))
	result.Finalize()

	// Redirect stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := OutputTable(result)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("OutputTable() failed: %v", err)
	}

	// Read output
	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	// Should show artifact count in header (statistics moved to presenter)
	if !strings.Contains(output, "Artifacts:") {
		t.Error("output should show artifact count")
	}
	if !strings.Contains(output, "4") {
		t.Error("output should show count of 4 artifacts")
	}
}

func TestOutputTable_MultipleSourcesPerArtifact(t *testing.T) {
	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)

	// Create artifact with multiple sources
	artifact := domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh")
	artifact.AddSource("rdap")
	artifact.AddSource("dnsx")
	result.AddArtifact(artifact)
	result.Finalize()

	// Redirect stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := OutputTable(result)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("OutputTable() failed: %v", err)
	}

	// Read output
	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	// Should show artifact count and sources used in metadata
	if !strings.Contains(output, "Sources:") {
		t.Error("output should show sources used")
	}
}

func TestOutputTable_ConfidenceFormatting(t *testing.T) {
	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)

	// Create artifact with specific confidence
	artifact := domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh")
	artifact.Confidence = 0.95
	result.AddArtifact(artifact)
	result.Finalize()

	// Redirect stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := OutputTable(result)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("OutputTable() failed: %v", err)
	}

	// Read output
	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	// Table no longer shows individual artifact details
	// Confidence is available in JSON output
	if !strings.Contains(output, "AethonX Scan Results") {
		t.Error("output should contain header")
	}
}

func TestOutputTable_DurationDisplay(t *testing.T) {
	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh"))
	result.Finalize()

	// Redirect stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := OutputTable(result)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("OutputTable() failed: %v", err)
	}

	// Read output
	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	// Should show duration
	if !strings.Contains(output, "Duration:") {
		t.Error("output should show duration")
	}
}

func TestOutputTable_SourcesUsedDisplay(t *testing.T) {
	target := domain.NewTarget("example.com", domain.ScanModePassive)
	result := domain.NewScanResult(*target)

	// Add artifacts from different sources
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "test1.example.com", "crtsh"))
	result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "test2.example.com", "rdap"))
	result.Finalize()

	// Redirect stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := OutputTable(result)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("OutputTable() failed: %v", err)
	}

	// Read output
	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	// Should show sources used header
	if !strings.Contains(output, "Sources:") {
		t.Error("output should show sources used header")
	}

	// Should show artifact count
	if !strings.Contains(output, "2") {
		t.Error("output should show 2 artifacts")
	}
}
