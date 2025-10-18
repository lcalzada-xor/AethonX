// internal/core/domain/artifact_test.go
package domain

import (
	"testing"

	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/testutil"
)

func TestNewArtifact(t *testing.T) {
	a := NewArtifact(ArtifactTypeSubdomain, "test.example.com", "crtsh")

	testutil.AssertNotNil(t, a, "artifact should not be nil")
	testutil.AssertEqual(t, a.Type, ArtifactTypeSubdomain, "type")
	testutil.AssertEqual(t, a.Value, "test.example.com", "value")
	testutil.AssertLen(t, a.Sources, 1, "sources length")
	testutil.AssertContains(t, a.Sources, "crtsh", "sources")
	testutil.AssertEqual(t, a.Confidence, 1.0, "default confidence")
	testutil.AssertNotEqual(t, a.ID, "", "ID should be generated")
	testutil.AssertNotNil(t, a.Metadata, "metadata should be initialized")
}

func TestArtifact_Normalize(t *testing.T) {
	tests := []struct {
		name     string
		artType  ArtifactType
		input    string
		expected string
	}{
		{
			name:     "normalize domain - lowercase",
			artType:  ArtifactTypeDomain,
			input:    "EXAMPLE.COM",
			expected: "example.com",
		},
		{
			name:     "normalize domain - remove trailing dot",
			artType:  ArtifactTypeDomain,
			input:    "example.com.",
			expected: "example.com",
		},
		{
			name:     "normalize subdomain - remove wildcard",
			artType:  ArtifactTypeSubdomain,
			input:    "*.example.com",
			expected: "example.com",
		},
		{
			name:     "normalize subdomain - remove www",
			artType:  ArtifactTypeSubdomain,
			input:    "www.example.com",
			expected: "example.com",
		},
		{
			name:     "normalize email - lowercase",
			artType:  ArtifactTypeEmail,
			input:    "Admin@Example.COM",
			expected: "admin@example.com",
		},
		{
			name:     "normalize URL - lowercase",
			artType:  ArtifactTypeURL,
			input:    "HTTPS://EXAMPLE.COM/PATH",
			expected: "https://example.com/path",
		},
		{
			name:     "normalize IP - trim spaces",
			artType:  ArtifactTypeIP,
			input:    "  192.168.1.1  ",
			expected: "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Artifact{
				Type:     tt.artType,
				Value:    tt.input,
				Metadata: make(map[string]string),
			}
			a.Normalize()
			testutil.AssertEqual(t, a.Value, tt.expected, "normalized value")
		})
	}
}

func TestArtifact_GenerateID(t *testing.T) {
	a1 := NewArtifact(ArtifactTypeSubdomain, "test.example.com", "crtsh")
	a2 := NewArtifact(ArtifactTypeSubdomain, "test.example.com", "rdap")
	a3 := NewArtifact(ArtifactTypeSubdomain, "different.example.com", "crtsh")

	// Mismo tipo y valor = mismo ID
	testutil.AssertEqual(t, a1.ID, a2.ID, "same domain should have same ID")

	// Diferente valor = diferente ID
	testutil.AssertNotEqual(t, a1.ID, a3.ID, "different domain should have different ID")

	// ID debe tener longitud 16 (truncated SHA256)
	testutil.AssertEqual(t, len(a1.ID), 16, "ID length")
}

func TestArtifact_Key(t *testing.T) {
	a := NewArtifact(ArtifactTypeSubdomain, "test.example.com", "crtsh")
	key := a.Key()

	testutil.AssertEqual(t, key, "subdomain:test.example.com", "artifact key")
}

func TestArtifact_AddSource(t *testing.T) {
	a := NewArtifact(ArtifactTypeSubdomain, "test.example.com", "crtsh")

	// Añadir nueva source
	a.AddSource("rdap")
	testutil.AssertLen(t, a.Sources, 2, "sources after adding")
	testutil.AssertContains(t, a.Sources, "rdap", "sources")

	// Añadir duplicada (no debería agregarse)
	a.AddSource("crtsh")
	testutil.AssertLen(t, a.Sources, 2, "sources should not have duplicates")

	// Añadir source vacía (no debería agregarse)
	a.AddSource("")
	testutil.AssertLen(t, a.Sources, 2, "empty source should not be added")
}

func TestArtifact_AddTag(t *testing.T) {
	a := NewArtifact(ArtifactTypeSubdomain, "test.example.com", "crtsh")

	// Añadir nuevo tag
	a.AddTag("wildcard")
	testutil.AssertLen(t, a.Tags, 1, "tags after adding")
	testutil.AssertContains(t, a.Tags, "wildcard", "tags")

	// Añadir duplicado (no debería agregarse)
	a.AddTag("wildcard")
	testutil.AssertLen(t, a.Tags, 1, "tags should not have duplicates")

	// Añadir tag vacío (no debería agregarse)
	a.AddTag("")
	testutil.AssertLen(t, a.Tags, 1, "empty tag should not be added")
}

func TestArtifact_Merge(t *testing.T) {
	a1 := NewArtifact(ArtifactTypeSubdomain, "test.example.com", "crtsh")
	a1.Confidence = 0.8
	a1.Metadata["key1"] = "value1"
	a1.AddTag("tag1")

	a2 := NewArtifact(ArtifactTypeSubdomain, "test.example.com", "rdap")
	a2.Confidence = 0.9
	a2.Metadata["key2"] = "value2"
	a2.AddTag("tag2")

	err := a1.Merge(a2)
	testutil.AssertNoError(t, err, "merge should succeed")

	// Verificar sources combinadas
	testutil.AssertLen(t, a1.Sources, 2, "sources after merge")
	testutil.AssertContains(t, a1.Sources, "crtsh", "sources")
	testutil.AssertContains(t, a1.Sources, "rdap", "sources")

	// Verificar tags combinados
	testutil.AssertLen(t, a1.Tags, 2, "tags after merge")
	testutil.AssertContains(t, a1.Tags, "tag1", "tags")
	testutil.AssertContains(t, a1.Tags, "tag2", "tags")

	// Verificar metadata combinado
	testutil.AssertEqual(t, a1.Metadata["key1"], "value1", "metadata key1")
	testutil.AssertEqual(t, a1.Metadata["key2"], "value2", "metadata key2")

	// Verificar confianza máxima
	testutil.AssertEqual(t, a1.Confidence, 0.9, "confidence should be max")
}

func TestArtifact_MergeIncompatible(t *testing.T) {
	a1 := NewArtifact(ArtifactTypeSubdomain, "test.example.com", "crtsh")
	a2 := NewArtifact(ArtifactTypeSubdomain, "different.example.com", "rdap")

	err := a1.Merge(a2)
	testutil.AssertError(t, err, "merge should fail for different keys")
}

func TestArtifact_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		artifact *Artifact
		valid    bool
	}{
		{
			name:     "valid artifact",
			artifact: NewArtifact(ArtifactTypeSubdomain, "test.example.com", "crtsh"),
			valid:    true,
		},
		{
			name: "empty type",
			artifact: &Artifact{
				Type:  "",
				Value: "test.example.com",
			},
			valid: false,
		},
		{
			name: "empty value",
			artifact: &Artifact{
				Type:  ArtifactTypeSubdomain,
				Value: "",
			},
			valid: false,
		},
		{
			name: "invalid confidence - too low",
			artifact: &Artifact{
				Type:       ArtifactTypeSubdomain,
				Value:      "test.example.com",
				Confidence: -0.1,
			},
			valid: false,
		},
		{
			name: "invalid confidence - too high",
			artifact: &Artifact{
				Type:       ArtifactTypeSubdomain,
				Value:      "test.example.com",
				Confidence: 1.5,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				testutil.AssertTrue(t, tt.artifact.IsValid(), "artifact should be valid")
			} else {
				testutil.AssertFalse(t, tt.artifact.IsValid(), "artifact should be invalid")
			}
		})
	}
}

func TestNewArtifactWithMetadata(t *testing.T) {
	meta := &metadata.DomainMetadata{
		ResolvedIPs:    []string{"192.0.2.1"},
		DNSRecords:     []string{"A"},
		Registrar:      "Test Registrar",
		SubdomainLevel: 2,
	}

	a := NewArtifactWithMetadata(ArtifactTypeSubdomain, "test.example.com", "crtsh", meta)

	testutil.AssertNotNil(t, a.TypedMetadata, "typed metadata should be set")
	testutil.AssertNotNil(t, a.Metadata, "metadata map should be synchronized")

	// Verificar sincronización
	testutil.AssertEqual(t, a.Metadata["registrar"], "Test Registrar", "metadata should be synced")
	testutil.AssertEqual(t, a.Metadata["subdomain_level"], "2", "metadata should be synced")
}

func TestArtifact_SyncMetadata(t *testing.T) {
	a := NewArtifact(ArtifactTypeSubdomain, "test.example.com", "crtsh")

	meta := &metadata.DomainMetadata{
		ResolvedIPs: []string{"192.0.2.1"},
		Registrar:   "Test Registrar",
		SubdomainLevel: 1,
	}

	a.TypedMetadata = meta
	a.SyncMetadata()

	testutil.AssertNotNil(t, a.Metadata, "metadata map should exist")
	testutil.AssertEqual(t, a.Metadata["registrar"], "Test Registrar", "metadata synced")
}
