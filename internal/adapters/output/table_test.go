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

	// Verify table headers
	if !strings.Contains(output, "TYPE") {
		t.Error("output should contain TYPE header")
	}
	if !strings.Contains(output, "VALUE") {
		t.Error("output should contain VALUE header")
	}
	if !strings.Contains(output, "SOURCES") {
		t.Error("output should contain SOURCES header")
	}
	if !strings.Contains(output, "CONFIDENCE") {
		t.Error("output should contain CONFIDENCE header")
	}

	// Verify artifacts appear
	if !strings.Contains(output, "test.example.com") {
		t.Error("output should contain artifact value")
	}
	if !strings.Contains(output, "192.168.1.1") {
		t.Error("output should contain IP artifact")
	}

	// Verify sources appear
	if !strings.Contains(output, "crtsh") {
		t.Error("output should contain source name")
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

	// Should show "No artifacts discovered"
	if !strings.Contains(output, "No artifacts discovered") {
		t.Error("output should indicate no artifacts found")
	}

	// Should still have header
	if !strings.Contains(output, "AethonX Scan Results") {
		t.Error("output should still contain header")
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

	// Should show statistics section
	if !strings.Contains(output, "Statistics by Type") {
		t.Error("output should contain Statistics section")
	}

	// Should show counts for each type
	if !strings.Contains(output, "subdomain") {
		t.Error("output should show subdomain type in stats")
	}
	if !strings.Contains(output, "ip") {
		t.Error("output should show ip type in stats")
	}
	if !strings.Contains(output, "email") {
		t.Error("output should show email type in stats")
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

	// Should show all sources (comma-separated)
	if !strings.Contains(output, "crtsh") {
		t.Error("output should contain first source")
	}
	if !strings.Contains(output, "rdap") {
		t.Error("output should contain second source")
	}
	if !strings.Contains(output, "dnsx") {
		t.Error("output should contain third source")
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

	// Should show confidence with 2 decimal places
	if !strings.Contains(output, "0.95") {
		t.Error("output should show confidence value 0.95")
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

	// Should show sources used
	if !strings.Contains(output, "Sources:") {
		t.Error("output should show sources used")
	}

	// Should list the sources
	if !strings.Contains(output, "crtsh") {
		t.Error("output should list crtsh source")
	}
	if !strings.Contains(output, "rdap") {
		t.Error("output should list rdap source")
	}
}
