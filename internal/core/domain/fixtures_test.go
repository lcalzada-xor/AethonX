// internal/core/domain/fixtures_test.go
package domain

// Helper functions for tests in this package only

import "aethonx/internal/core/domain/metadata"

// fixtureTarget crea un target de prueba.
func fixtureTarget(mode ScanMode) Target {
	target := NewTarget("example.com", mode)
	target.Scope.IncludeSubdomains = true
	target.Scope.MaxDepth = 3
	return *target
}

// fixtureSubdomainArtifacts retorna artifacts de subdominios de prueba.
func fixtureSubdomainArtifacts() []*Artifact {
	return []*Artifact{
		NewArtifact(ArtifactTypeSubdomain, "test.example.com", "crtsh"),
		NewArtifact(ArtifactTypeSubdomain, "api.example.com", "crtsh"),
		NewArtifact(ArtifactTypeSubdomain, "www.example.com", "crtsh"),
		NewArtifact(ArtifactTypeSubdomain, "mail.example.com", "rdap"),
	}
}

// fixtureMixedArtifacts retorna artifacts de varios tipos.
func fixtureMixedArtifacts() []*Artifact {
	return []*Artifact{
		NewArtifact(ArtifactTypeSubdomain, "test.example.com", "crtsh"),
		NewArtifact(ArtifactTypeIP, "192.168.1.1", "dns"),
		NewArtifact(ArtifactTypeEmail, "admin@example.com", "whois"),
		NewArtifact(ArtifactTypeURL, "https://example.com", "crawler"),
		NewArtifact(ArtifactTypeCertificate, "*.example.com", "crtsh"),
	}
}

// fixtureDuplicateArtifacts retorna artifacts duplicados para tests de deduplicación.
func fixtureDuplicateArtifacts() []*Artifact {
	return []*Artifact{
		NewArtifact(ArtifactTypeSubdomain, "test.example.com", "crtsh"),
		NewArtifact(ArtifactTypeSubdomain, "test.example.com", "rdap"),
		NewArtifact(ArtifactTypeSubdomain, "api.example.com", "crtsh"),
		NewArtifact(ArtifactTypeSubdomain, "api.example.com", "dnsx"),
	}
}

// fixtureDomainMetadata retorna metadata de dominio de prueba.
func fixtureDomainMetadata() *metadata.DomainMetadata {
	return &metadata.DomainMetadata{
		ResolvedIPs:    []string{"192.0.2.1"},
		DNSRecords:     []string{"A", "AAAA"},
		Registrar:      "Example Registrar Inc",
		SubdomainLevel: 2,
	}
}
