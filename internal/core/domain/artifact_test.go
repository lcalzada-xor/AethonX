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
			name:     "normalize URL - lowercase scheme and host",
			artType:  ArtifactTypeURL,
			input:    "HTTPS://EXAMPLE.COM/PATH",
			expected: "https://example.com/PATH",
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
				Type:  tt.artType,
				Value: tt.input,
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
	// Create artifacts with typed metadata
	meta1 := metadata.NewDomainMetadata()
	meta1.Registrar = "Registrar1"

	meta2 := metadata.NewDomainMetadata()
	meta2.Registrar = "Registrar2"

	a1 := NewArtifactWithMetadata(ArtifactTypeSubdomain, "test.example.com", "crtsh", meta1)
	a1.Confidence = 0.8
	a1.AddTag("tag1")

	a2 := NewArtifactWithMetadata(ArtifactTypeSubdomain, "test.example.com", "rdap", meta2)
	a2.Confidence = 0.9
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

	// Verificar TypedMetadata (a1 should keep its own metadata, not overwrite)
	domainMeta := a1.GetDomainMetadata()
	testutil.AssertNotNil(t, domainMeta, "typed metadata should exist")
	testutil.AssertEqual(t, domainMeta.Registrar, "Registrar1", "metadata should not be overwritten")

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

	// Verificar que el metadata tipado se puede recuperar
	domainMeta := a.GetDomainMetadata()
	testutil.AssertNotNil(t, domainMeta, "domain metadata should be retrievable")
	testutil.AssertEqual(t, domainMeta.Registrar, "Test Registrar", "registrar should match")
	testutil.AssertEqual(t, domainMeta.SubdomainLevel, 2, "subdomain level should match")
}

func TestArtifact_TypedMetadataAccess(t *testing.T) {
	a := NewArtifact(ArtifactTypeSubdomain, "test.example.com", "crtsh")

	meta := &metadata.DomainMetadata{
		ResolvedIPs:    []string{"192.0.2.1"},
		Registrar:      "Test Registrar",
		SubdomainLevel: 1,
	}

	a.TypedMetadata = meta

	// Verificar acceso a metadata tipado
	domainMeta := a.GetDomainMetadata()
	testutil.AssertNotNil(t, domainMeta, "domain metadata should be accessible")
	testutil.AssertEqual(t, domainMeta.Registrar, "Test Registrar", "registrar should match")

	// Verificar que ToMap() funciona para serialización
	metaMap := meta.ToMap()
	testutil.AssertEqual(t, metaMap["registrar"], "Test Registrar", "ToMap should work for serialization")
}

// Advanced normalization and validation tests

func TestArtifact_IPNormalization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		valid    bool
	}{
		{
			name:     "valid IPv4",
			input:    "192.168.1.1",
			expected: "192.168.1.1",
			valid:    true,
		},
		{
			name:     "IPv4 with spaces",
			input:    "  192.168.1.1  ",
			expected: "192.168.1.1",
			valid:    true,
		},
		{
			name:     "valid IPv6",
			input:    "2001:0db8:0000:0000:0000:0000:0000:0001",
			expected: "2001:db8::1",
			valid:    true,
		},
		{
			name:     "IPv6 shorthand",
			input:    "2001:db8::1",
			expected: "2001:db8::1",
			valid:    true,
		},
		{
			name:     "invalid IP - letters",
			input:    "192.168.1.abc",
			expected: "",
			valid:    false,
		},
		{
			name:     "invalid IP - out of range",
			input:    "192.168.1.256",
			expected: "",
			valid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewArtifact(ArtifactTypeIP, tt.input, "test")

			if tt.valid {
				testutil.AssertEqual(t, a.Value, tt.expected, "normalized IP")
				testutil.AssertTrue(t, a.IsValid(), "should be valid")
			} else {
				testutil.AssertEqual(t, a.Value, tt.expected, "invalid IP should normalize to empty")
				testutil.AssertFalse(t, a.IsValid(), "should be invalid")
			}
		})
	}
}

func TestArtifact_URLNormalization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "uppercase scheme and host",
			input:    "HTTP://EXAMPLE.COM/Path",
			expected: "http://example.com/Path",
		},
		{
			name:     "remove default HTTP port",
			input:    "http://example.com:80/path",
			expected: "http://example.com/path",
		},
		{
			name:     "remove default HTTPS port",
			input:    "https://example.com:443/path",
			expected: "https://example.com/path",
		},
		{
			name:     "keep non-default port",
			input:    "http://example.com:8080/path",
			expected: "http://example.com:8080/path",
		},
		{
			name:     "remove trailing slash on root",
			input:    "https://example.com/",
			expected: "https://example.com",
		},
		{
			name:     "keep trailing slash on path",
			input:    "https://example.com/path/",
			expected: "https://example.com/path/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewArtifact(ArtifactTypeURL, tt.input, "test")
			testutil.AssertEqual(t, a.Value, tt.expected, "normalized URL")
			testutil.AssertTrue(t, a.IsValid(), "should be valid")
		})
	}
}

func TestArtifact_EmailValidation(t *testing.T) {
	tests := []struct {
		name  string
		email string
		valid bool
	}{
		{
			name:  "valid email",
			email: "user@example.com",
			valid: true,
		},
		{
			name:  "valid email with subdomain",
			email: "user@mail.example.com",
			valid: true,
		},
		{
			name:  "valid email with plus",
			email: "user+tag@example.com",
			valid: true,
		},
		{
			name:  "invalid - no @",
			email: "userexample.com",
			valid: false,
		},
		{
			name:  "invalid - no domain",
			email: "user@",
			valid: false,
		},
		{
			name:  "invalid - no TLD",
			email: "user@example",
			valid: false,
		},
		{
			name:  "invalid - too short",
			email: "a@b",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewArtifact(ArtifactTypeEmail, tt.email, "test")

			if tt.valid {
				testutil.AssertTrue(t, a.IsValid(), "should be valid")
			} else {
				testutil.AssertFalse(t, a.IsValid(), "should be invalid")
			}
		})
	}
}

func TestArtifact_PortValidation(t *testing.T) {
	tests := []struct {
		name  string
		port  string
		valid bool
	}{
		{
			name:  "valid port - 80",
			port:  "80",
			valid: true,
		},
		{
			name:  "valid port - 443",
			port:  "443",
			valid: true,
		},
		{
			name:  "valid port - 8080",
			port:  "8080",
			valid: true,
		},
		{
			name:  "valid port - max",
			port:  "65535",
			valid: true,
		},
		{
			name:  "invalid port - 0",
			port:  "0",
			valid: false,
		},
		{
			name:  "invalid port - negative",
			port:  "-1",
			valid: false,
		},
		{
			name:  "invalid port - too large",
			port:  "65536",
			valid: false,
		},
		{
			name:  "invalid port - not a number",
			port:  "abc",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewArtifact(ArtifactTypePort, tt.port, "test")

			if tt.valid {
				testutil.AssertTrue(t, a.IsValid(), "should be valid")
			} else {
				testutil.AssertFalse(t, a.IsValid(), "should be invalid")
			}
		})
	}
}

func TestArtifact_CertificateSerialValidation(t *testing.T) {
	tests := []struct {
		name   string
		serial string
		valid  bool
	}{
		{
			name:   "valid hex serial",
			serial: "00d389b7d7936a9a5efbd697c8af3ecbf9",
			valid:  true,
		},
		{
			name:   "valid hex serial uppercase",
			serial: "00D389B7D7936A9A5EFBD697C8AF3ECBF9",
			valid:  true,
		},
		{
			name:   "valid hex serial with colons",
			serial: "00:d3:89:b7:d7",
			valid:  true,
		},
		{
			name:   "valid hex serial with spaces",
			serial: "00 d3 89 b7",
			valid:  true,
		},
		{
			name:   "invalid serial - empty",
			serial: "",
			valid:  false,
		},
		{
			name:   "invalid serial - non-hex characters",
			serial: "00g389",
			valid:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewArtifact(ArtifactTypeCertificate, tt.serial, "test")

			if tt.valid {
				testutil.AssertTrue(t, a.IsValid(), "should be valid")
			} else {
				testutil.AssertFalse(t, a.IsValid(), "should be invalid")
			}
		})
	}
}
