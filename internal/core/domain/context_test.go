package domain

import (
	"encoding/json"
	"testing"
)

func TestArtifact_ContextFields_JSON(t *testing.T) {
	// Create artifact with all new context fields
	artifact := NewArtifact(ArtifactTypeEndpoint, "/api/users", "golinkfinderevo")

	// Set discovery context
	artifact.SetDiscoveryContext(&DiscoveryContext{
		SourceURL:      "https://example.com/app.js",
		LineNumber:     42,
		Context:        "fetch('/api/users')",
		MatchPattern:   "api-endpoint",
	})

	// Set security context
	artifact.SetSecurityContext(NewSecurityContext().
		WithSeverity(SeverityHigh).
		WithVulnerabilityType(VulnTypeSQLi))

	// Set classification
	artifact.SetClassification(NewClassification().
		WithResourceType(ResourceTypeAPI).
		WithDataType(DataTypeAPIKey))

	// Marshal to JSON
	jsonBytes, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal artifact: %v", err)
	}

	t.Logf("Serialized artifact with context fields:\n%s", string(jsonBytes))

	// Unmarshal back
	var decoded Artifact
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal artifact: %v", err)
	}

	// Verify discovery context
	if decoded.DiscoveryContext == nil {
		t.Error("DiscoveryContext is nil after unmarshal")
	} else {
		if decoded.DiscoveryContext.SourceURL != "https://example.com/app.js" {
			t.Errorf("SourceURL mismatch: got %s", decoded.DiscoveryContext.SourceURL)
		}
		if decoded.DiscoveryContext.LineNumber != 42 {
			t.Errorf("LineNumber mismatch: got %d", decoded.DiscoveryContext.LineNumber)
		}
		if decoded.DiscoveryContext.MatchPattern != "api-endpoint" {
			t.Errorf("MatchPattern mismatch: got %s", decoded.DiscoveryContext.MatchPattern)
		}
	}

	// Verify security context
	if decoded.SecurityContext == nil {
		t.Error("SecurityContext is nil after unmarshal")
	} else {
		if decoded.SecurityContext.Severity != SeverityHigh {
			t.Errorf("Severity mismatch: got %s", decoded.SecurityContext.Severity)
		}
		if len(decoded.SecurityContext.VulnerabilityTypes) != 1 || decoded.SecurityContext.VulnerabilityTypes[0] != VulnTypeSQLi {
			t.Errorf("VulnerabilityTypes mismatch: got %v", decoded.SecurityContext.VulnerabilityTypes)
		}
	}

	// Verify classification
	if decoded.Classification == nil {
		t.Error("Classification is nil after unmarshal")
	} else {
		if decoded.Classification.ResourceType != ResourceTypeAPI {
			t.Errorf("ResourceType mismatch: got %s", decoded.Classification.ResourceType)
		}
		if decoded.Classification.DataType != DataTypeAPIKey {
			t.Errorf("DataType mismatch: got %s", decoded.Classification.DataType)
		}
	}
}

func TestArtifact_ContextFields_Merge(t *testing.T) {
	// Create first artifact with discovery context
	artifact1 := NewArtifact(ArtifactTypeEndpoint, "/api/users", "source1")
	artifact1.SetDiscoveryContext(&DiscoveryContext{
		SourceURL:  "https://example.com/app.js",
		LineNumber: 42,
	})

	// Create second artifact with security context
	artifact2 := NewArtifact(ArtifactTypeEndpoint, "/api/users", "source2")
	artifact2.SetSecurityContext(NewSecurityContext().
		WithSeverity(SeverityCritical).
		WithVulnerabilityType(VulnTypeCredential))

	// Merge
	if err := artifact1.Merge(artifact2); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Verify both contexts are present
	if artifact1.DiscoveryContext == nil {
		t.Error("DiscoveryContext lost after merge")
	}
	if artifact1.SecurityContext == nil {
		t.Error("SecurityContext not merged")
	}
	if artifact1.SecurityContext.Severity != SeverityCritical {
		t.Errorf("Severity not merged: got %s", artifact1.SecurityContext.Severity)
	}
}

func TestSecurityContext_SeverityWeight(t *testing.T) {
	tests := []struct {
		severity Severity
		weight   int
	}{
		{SeverityCritical, 5},
		{SeverityHigh, 4},
		{SeverityMedium, 3},
		{SeverityLow, 2},
		{SeverityInfo, 1},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			if got := tt.severity.Weight(); got != tt.weight {
				t.Errorf("Weight() = %d, want %d", got, tt.weight)
			}
		})
	}
}

func TestDiscoveryContext_IsEmpty(t *testing.T) {
	tests := []struct {
		name    string
		ctx     *DiscoveryContext
		isEmpty bool
	}{
		{
			name:    "completely empty",
			ctx:     &DiscoveryContext{},
			isEmpty: true,
		},
		{
			name:    "with SourceURL",
			ctx:     &DiscoveryContext{SourceURL: "https://example.com"},
			isEmpty: false,
		},
		{
			name:    "with LineNumber",
			ctx:     &DiscoveryContext{LineNumber: 42},
			isEmpty: false,
		},
		{
			name:    "with Context",
			ctx:     &DiscoveryContext{Context: "some context"},
			isEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ctx.IsEmpty(); got != tt.isEmpty {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.isEmpty)
			}
		})
	}
}

func TestClassification_IsEmpty(t *testing.T) {
	tests := []struct {
		name    string
		cls     *Classification
		isEmpty bool
	}{
		{
			name:    "completely empty",
			cls:     &Classification{},
			isEmpty: true,
		},
		{
			name:    "with ResourceType",
			cls:     &Classification{ResourceType: ResourceTypeAPI},
			isEmpty: false,
		},
		{
			name:    "with IsExternal",
			cls:     &Classification{IsExternal: true},
			isEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cls.IsEmpty(); got != tt.isEmpty {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.isEmpty)
			}
		})
	}
}
