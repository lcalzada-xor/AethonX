// internal/core/domain/enums_test.go
package domain

import (
	"testing"

	"aethonx/internal/testutil"
)

func TestScanMode_String(t *testing.T) {
	tests := []struct {
		mode     ScanMode
		expected string
	}{
		{ScanModePassive, "passive"},
		{ScanModeActive, "active"},
		{ScanModeHybrid, "hybrid"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			testutil.AssertEqual(t, string(tt.mode), tt.expected, "scan mode string")
		})
	}
}

func TestSourceMode_CompatibleWith(t *testing.T) {
	tests := []struct {
		name       string
		sourceMode SourceMode
		scanMode   ScanMode
		compatible bool
	}{
		{
			name:       "passive source compatible with passive scan",
			sourceMode: SourceModePassive,
			scanMode:   ScanModePassive,
			compatible: true,
		},
		{
			name:       "passive source NOT compatible with active scan",
			sourceMode: SourceModePassive,
			scanMode:   ScanModeActive,
			compatible: false,
		},
		{
			name:       "passive source compatible with hybrid scan",
			sourceMode: SourceModePassive,
			scanMode:   ScanModeHybrid,
			compatible: true,
		},
		{
			name:       "active source NOT compatible with passive scan",
			sourceMode: SourceModeActive,
			scanMode:   ScanModePassive,
			compatible: false,
		},
		{
			name:       "active source compatible with active scan",
			sourceMode: SourceModeActive,
			scanMode:   ScanModeActive,
			compatible: true,
		},
		{
			name:       "active source compatible with hybrid scan",
			sourceMode: SourceModeActive,
			scanMode:   ScanModeHybrid,
			compatible: true,
		},
		{
			name:       "both mode compatible with passive scan",
			sourceMode: SourceModeBoth,
			scanMode:   ScanModePassive,
			compatible: true,
		},
		{
			name:       "both mode compatible with active scan",
			sourceMode: SourceModeBoth,
			scanMode:   ScanModeActive,
			compatible: true,
		},
		{
			name:       "both mode compatible with hybrid scan",
			sourceMode: SourceModeBoth,
			scanMode:   ScanModeHybrid,
			compatible: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.sourceMode.CompatibleWith(tt.scanMode)
			if tt.compatible {
				testutil.AssertTrue(t, result, "should be compatible")
			} else {
				testutil.AssertFalse(t, result, "should not be compatible")
			}
		})
	}
}

func TestArtifactType_IsValid(t *testing.T) {
	validTypes := []ArtifactType{
		ArtifactTypeDomain,
		ArtifactTypeSubdomain,
		ArtifactTypeIP,
		ArtifactTypeEmail,
		ArtifactTypeURL,
		ArtifactTypeCertificate,
	}

	for _, artType := range validTypes {
		t.Run(string(artType), func(t *testing.T) {
			testutil.AssertTrue(t, artType.IsValid(), "should be valid artifact type")
		})
	}

	// Tipo inválido
	invalidType := ArtifactType("invalid-type")
	testutil.AssertFalse(t, invalidType.IsValid(), "should be invalid artifact type")
}

func TestArtifactType_Category(t *testing.T) {
	tests := []struct {
		artifactType ArtifactType
		category     string
	}{
		{ArtifactTypeDomain, "infrastructure"},
		{ArtifactTypeSubdomain, "infrastructure"},
		{ArtifactTypeIP, "infrastructure"},
		{ArtifactTypeIPv6, "infrastructure"},
		{ArtifactTypeURL, "web"},
		{ArtifactTypeEndpoint, "web"},
		{ArtifactTypeAPI, "web"},
		{ArtifactTypeCertificate, "security"},
		{ArtifactTypeVulnerability, "security"},
		{ArtifactTypeEmail, "contact"},
		{ArtifactTypeCloudResource, "cloud"},
		{ArtifactTypeCredential, "data"},
	}

	for _, tt := range tests {
		t.Run(string(tt.artifactType), func(t *testing.T) {
			testutil.AssertEqual(t, tt.artifactType.Category(), tt.category, "artifact category")
		})
	}
}
