// internal/core/usecases/dedupe_service_test.go
package usecases

import (
	"testing"

	"aethonx/internal/core/domain"
	"aethonx/internal/testutil"
)

func TestNewDedupeService(t *testing.T) {
	svc := NewDedupeService()
	testutil.AssertNotNil(t, svc, "service should not be nil")
}

func TestDedupeService_Deduplicate(t *testing.T) {
	svc := NewDedupeService()

	tests := []struct {
		name     string
		input    []*domain.Artifact
		expected int
	}{
		{
			name:     "empty list",
			input:    []*domain.Artifact{},
			expected: 0,
		},
		{
			name: "no duplicates",
			input: []*domain.Artifact{
				domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh"),
				domain.NewArtifact(domain.ArtifactTypeSubdomain, "api.example.com", "crtsh"),
			},
			expected: 2,
		},
		{
			name: "exact duplicates",
			input: []*domain.Artifact{
				domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh"),
				domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "rdap"),
			},
			expected: 1,
		},
		{
			name: "case insensitive duplicates",
			input: []*domain.Artifact{
				domain.NewArtifact(domain.ArtifactTypeSubdomain, "TEST.example.com", "crtsh"),
				domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "rdap"),
			},
			expected: 1,
		},
		{
			name: "nil artifacts filtered out",
			input: []*domain.Artifact{
				nil,
				domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh"),
				nil,
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.Deduplicate(tt.input)
			if len(result) != tt.expected {
				t.Errorf("expected %d artifacts, got %d", tt.expected, len(result))
			}
		})
	}
}

func TestDedupeService_Deduplicate_MergesSources(t *testing.T) {
	svc := NewDedupeService()

	artifacts := []*domain.Artifact{
		domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh"),
		domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "rdap"),
		domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "dnsx"),
	}

	result := svc.Deduplicate(artifacts)

	testutil.AssertEqual(t, len(result), 1, "should have 1 deduplicated artifact")
	testutil.AssertEqual(t, len(result[0].Sources), 3, "should have merged 3 sources")
	testutil.AssertContains(t, result[0].Sources, "crtsh", "sources")
	testutil.AssertContains(t, result[0].Sources, "rdap", "sources")
	testutil.AssertContains(t, result[0].Sources, "dnsx", "sources")
}

func TestDedupeService_Deduplicate_SortsOutput(t *testing.T) {
	svc := NewDedupeService()

	artifacts := []*domain.Artifact{
		domain.NewArtifact(domain.ArtifactTypeEmail, "admin@example.com", "whois"),
		domain.NewArtifact(domain.ArtifactTypeSubdomain, "www.example.com", "crtsh"),
		domain.NewArtifact(domain.ArtifactTypeSubdomain, "api.example.com", "crtsh"),
		domain.NewArtifact(domain.ArtifactTypeIP, "192.168.1.1", "dns"),
	}

	result := svc.Deduplicate(artifacts)

	// Verify sorted by type first, then by value
	if len(result) < 2 {
		t.Fatal("expected at least 2 artifacts")
	}

	// Email comes before IP alphabetically
	// Within same type, sorted by value
	testutil.AssertEqual(t, result[0].Type, domain.ArtifactTypeEmail, "first artifact type")
	testutil.AssertEqual(t, result[1].Type, domain.ArtifactTypeIP, "second artifact type")

	// Find subdomain artifacts and verify they're sorted
	var subdomains []*domain.Artifact
	for _, a := range result {
		if a.Type == domain.ArtifactTypeSubdomain {
			subdomains = append(subdomains, a)
		}
	}

	if len(subdomains) >= 2 {
		// api.example.com should come before www.example.com
		testutil.AssertEqual(t, subdomains[0].Value, "api.example.com", "first subdomain")
		testutil.AssertEqual(t, subdomains[1].Value, "example.com", "second subdomain (www removed)")
	}
}

func TestDedupeService_FilterByType(t *testing.T) {
	svc := NewDedupeService()

	artifacts := []*domain.Artifact{
		domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh"),
		domain.NewArtifact(domain.ArtifactTypeIP, "192.168.1.1", "dns"),
		domain.NewArtifact(domain.ArtifactTypeEmail, "admin@example.com", "whois"),
		domain.NewArtifact(domain.ArtifactTypeSubdomain, "api.example.com", "crtsh"),
	}

	tests := []struct {
		name     string
		types    []domain.ArtifactType
		expected int
	}{
		{
			name:     "filter subdomains",
			types:    []domain.ArtifactType{domain.ArtifactTypeSubdomain},
			expected: 2,
		},
		{
			name:     "filter IPs",
			types:    []domain.ArtifactType{domain.ArtifactTypeIP},
			expected: 1,
		},
		{
			name:     "filter multiple types",
			types:    []domain.ArtifactType{domain.ArtifactTypeSubdomain, domain.ArtifactTypeEmail},
			expected: 3,
		},
		{
			name:     "no filter (empty types)",
			types:    []domain.ArtifactType{},
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.FilterByType(artifacts, tt.types...)
			if len(result) != tt.expected {
				t.Errorf("expected %d artifacts, got %d", tt.expected, len(result))
			}
		})
	}
}

func TestDedupeService_FilterByConfidence(t *testing.T) {
	svc := NewDedupeService()

	artifacts := []*domain.Artifact{
		createArtifactWithConfidence(domain.ArtifactTypeSubdomain, "test1.example.com", "crtsh", 0.9),
		createArtifactWithConfidence(domain.ArtifactTypeSubdomain, "test2.example.com", "crtsh", 0.7),
		createArtifactWithConfidence(domain.ArtifactTypeSubdomain, "test3.example.com", "crtsh", 0.5),
		createArtifactWithConfidence(domain.ArtifactTypeSubdomain, "test4.example.com", "crtsh", 0.3),
	}

	tests := []struct {
		name          string
		minConfidence float64
		expected      int
	}{
		{
			name:          "very high confidence",
			minConfidence: 0.9,
			expected:      1,
		},
		{
			name:          "high confidence",
			minConfidence: 0.7,
			expected:      2,
		},
		{
			name:          "medium confidence",
			minConfidence: 0.5,
			expected:      3,
		},
		{
			name:          "any confidence",
			minConfidence: 0.0,
			expected:      4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.FilterByConfidence(artifacts, tt.minConfidence)
			if len(result) != tt.expected {
				t.Errorf("expected %d artifacts, got %d", tt.expected, len(result))
			}
		})
	}
}

func TestDedupeService_FilterBySource(t *testing.T) {
	svc := NewDedupeService()

	a1 := domain.NewArtifact(domain.ArtifactTypeSubdomain, "test1.example.com", "crtsh")
	a2 := domain.NewArtifact(domain.ArtifactTypeSubdomain, "test2.example.com", "rdap")
	a3 := domain.NewArtifact(domain.ArtifactTypeSubdomain, "test3.example.com", "crtsh")
	a3.AddSource("rdap") // Multi-source artifact

	artifacts := []*domain.Artifact{a1, a2, a3}

	tests := []struct {
		name     string
		source   string
		expected int
	}{
		{
			name:     "filter by crtsh",
			source:   "crtsh",
			expected: 2,
		},
		{
			name:     "filter by rdap",
			source:   "rdap",
			expected: 2,
		},
		{
			name:     "non-existent source",
			source:   "nonexistent",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.FilterBySource(artifacts, tt.source)
			if len(result) != tt.expected {
				t.Errorf("expected %d artifacts, got %d", tt.expected, len(result))
			}
		})
	}
}

func TestDedupeService_GroupByType(t *testing.T) {
	svc := NewDedupeService()

	artifacts := []*domain.Artifact{
		domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh"),
		domain.NewArtifact(domain.ArtifactTypeSubdomain, "api.example.com", "crtsh"),
		domain.NewArtifact(domain.ArtifactTypeIP, "192.168.1.1", "dns"),
		domain.NewArtifact(domain.ArtifactTypeEmail, "admin@example.com", "whois"),
	}

	groups := svc.GroupByType(artifacts)

	testutil.AssertEqual(t, len(groups), 3, "should have 3 groups")
	testutil.AssertEqual(t, len(groups[domain.ArtifactTypeSubdomain]), 2, "subdomain group size")
	testutil.AssertEqual(t, len(groups[domain.ArtifactTypeIP]), 1, "IP group size")
	testutil.AssertEqual(t, len(groups[domain.ArtifactTypeEmail]), 1, "email group size")
}

// Helper function for tests
func createArtifactWithConfidence(artifactType domain.ArtifactType, value, source string, confidence float64) *domain.Artifact {
	a := domain.NewArtifact(artifactType, value, source)
	a.Confidence = confidence
	return a
}
