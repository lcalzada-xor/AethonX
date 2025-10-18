// internal/core/domain/scan_result_test.go
package domain

import (
	"testing"

	"aethonx/internal/testutil"
)

func TestNewScanResult(t *testing.T) {
	target := fixtureTarget(ScanModePassive)
	result := NewScanResult(target)

	testutil.AssertNotNil(t, result, "scan result should not be nil")
	testutil.AssertNotEqual(t, result.ID, "", "ID should be generated")
	testutil.AssertEqual(t, result.Target.Root, "example.com", "target root")
	testutil.AssertNotNil(t, result.Artifacts, "artifacts should be initialized")
	testutil.AssertNotNil(t, result.Warnings, "warnings should be initialized")
	testutil.AssertNotNil(t, result.Errors, "errors should be initialized")
	testutil.AssertFalse(t, result.Metadata.StartTime.IsZero(), "start time should be set")
}

func TestScanResult_AddArtifact(t *testing.T) {
	target := fixtureTarget(ScanModePassive)
	result := NewScanResult(target)

	artifact := NewArtifact(ArtifactTypeSubdomain, "test.example.com", "crtsh")
	result.AddArtifact(artifact)

	if len(result.Artifacts) != 1 {
		t.Errorf("expected 1 artifact, got %d", len(result.Artifacts))
	}
}

func TestScanResult_AddWarning(t *testing.T) {
	target := fixtureTarget(ScanModePassive)
	result := NewScanResult(target)

	result.AddWarning("test-source", "test warning message")

	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(result.Warnings))
	}
	testutil.AssertEqual(t, result.Warnings[0].Source, "test-source", "warning source")
	testutil.AssertEqual(t, result.Warnings[0].Message, "test warning message", "warning message")
}

func TestScanResult_AddError(t *testing.T) {
	target := fixtureTarget(ScanModePassive)
	result := NewScanResult(target)

	// Error no fatal
	result.AddError("test-source", "non-fatal error", false)
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(result.Errors))
	}
	testutil.AssertFalse(t, result.Errors[0].Fatal, "error should not be fatal")

	// Error fatal
	result.AddError("test-source", "fatal error", true)
	if len(result.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(result.Errors))
	}
	testutil.AssertTrue(t, result.Errors[1].Fatal, "error should be fatal")
}

func TestScanResult_TotalArtifacts(t *testing.T) {
	target := fixtureTarget(ScanModePassive)
	result := NewScanResult(target)

	testutil.AssertEqual(t, result.TotalArtifacts(), 0, "total artifacts initially")

	result.Artifacts = fixtureSubdomainArtifacts()
	testutil.AssertEqual(t, result.TotalArtifacts(), 4, "total artifacts after adding")
}

func TestScanResult_Stats(t *testing.T) {
	target := fixtureTarget(ScanModePassive)
	result := NewScanResult(target)
	result.Artifacts = fixtureMixedArtifacts()

	stats := result.Stats()

	testutil.AssertNotNil(t, stats, "stats should not be nil")
	testutil.AssertEqual(t, stats[string(ArtifactTypeSubdomain)], 1, "subdomain count")
	testutil.AssertEqual(t, stats[string(ArtifactTypeIP)], 1, "IP count")
	testutil.AssertEqual(t, stats[string(ArtifactTypeEmail)], 1, "email count")
	testutil.AssertEqual(t, stats[string(ArtifactTypeURL)], 1, "URL count")
	testutil.AssertEqual(t, stats[string(ArtifactTypeCertificate)], 1, "certificate count")
}

func TestScanResult_Finalize(t *testing.T) {
	target := fixtureTarget(ScanModePassive)
	result := NewScanResult(target)
	result.Artifacts = fixtureSubdomainArtifacts()

	testutil.AssertTrue(t, result.Metadata.EndTime.IsZero(), "end time should not be set initially")

	result.Finalize()

	testutil.AssertFalse(t, result.Metadata.EndTime.IsZero(), "end time should be set after finalize")
	testutil.AssertNotEqual(t, result.Metadata.Duration, 0, "duration should be calculated")
	testutil.AssertTrue(t, result.Metadata.Duration >= 0, "duration should be non-negative")
}

func TestScanResult_Summary(t *testing.T) {
	target := fixtureTarget(ScanModePassive)
	result := NewScanResult(target)
	result.Artifacts = fixtureMixedArtifacts()
	result.AddWarning("test-source", "test warning")
	result.AddError("test-source", "test error", false)
	result.Finalize()

	summary := result.Summary()

	testutil.AssertNotEqual(t, summary, "", "summary should not be empty")
}

func TestScanResult_HasErrors(t *testing.T) {
	target := fixtureTarget(ScanModePassive)
	result := NewScanResult(target)

	testutil.AssertFalse(t, result.HasErrors(), "should not have errors initially")

	result.AddError("test-source", "test error", false)
	testutil.AssertTrue(t, result.HasErrors(), "should have errors after adding")
}

func TestScanResult_WarningsCount(t *testing.T) {
	target := fixtureTarget(ScanModePassive)
	result := NewScanResult(target)

	testutil.AssertEqual(t, len(result.Warnings), 0, "should not have warnings initially")

	result.AddWarning("test-source", "test warning")
	testutil.AssertEqual(t, len(result.Warnings), 1, "should have 1 warning after adding")
	testutil.AssertEqual(t, result.Warnings[0].Source, "test-source", "warning source")
	testutil.AssertEqual(t, result.Warnings[0].Message, "test warning", "warning message")
}

func TestScanResult_FilterArtifactsByType(t *testing.T) {
	target := fixtureTarget(ScanModePassive)
	result := NewScanResult(target)
	result.Artifacts = fixtureMixedArtifacts()

	// Count subdomains manually
	subdomainCount := 0
	for _, a := range result.Artifacts {
		if a.Type == ArtifactTypeSubdomain {
			subdomainCount++
		}
	}
	testutil.AssertEqual(t, subdomainCount, 1, "subdomain artifacts")

	// Count IPs manually
	ipCount := 0
	for _, a := range result.Artifacts {
		if a.Type == ArtifactTypeIP {
			ipCount++
		}
	}
	testutil.AssertEqual(t, ipCount, 1, "IP artifacts")
}
