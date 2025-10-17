// internal/core/domain/builders.go
package domain

import (
	"aethonx/internal/core/domain/metadata"
)

// NewTechnologyArtifact crea un artifact de tecnología con metadata tipado.
func NewTechnologyArtifact(name, version, source string) *Artifact {
	meta := metadata.NewTechnologyMetadata(name, version)

	return NewArtifactWithMetadata(
		ArtifactTypeTechnology,
		name+"@"+version,
		source,
		meta,
	)
}

// NewDomainArtifact crea un artifact de dominio con metadata tipado.
func NewDomainArtifact(domain, source string) *Artifact {
	meta := metadata.NewDomainMetadata()

	return NewArtifactWithMetadata(
		ArtifactTypeDomain,
		domain,
		source,
		meta,
	)
}

// NewSubdomainArtifact crea un artifact de subdominio con metadata tipado.
func NewSubdomainArtifact(subdomain, source string) *Artifact {
	meta := metadata.NewDomainMetadata()

	return NewArtifactWithMetadata(
		ArtifactTypeSubdomain,
		subdomain,
		source,
		meta,
	)
}

// NewIPArtifact crea un artifact de IP con metadata tipado.
func NewIPArtifact(ip, source string) *Artifact {
	meta := metadata.NewIPMetadata()

	return NewArtifactWithMetadata(
		ArtifactTypeIP,
		ip,
		source,
		meta,
	)
}

// SetTechnologyMetadata establece metadata de tecnología en un artifact existente.
func (a *Artifact) SetTechnologyMetadata(meta *metadata.TechnologyMetadata) {
	a.TypedMetadata = meta
	a.SyncMetadata()
}

// SetDomainMetadata establece metadata de dominio en un artifact existente.
func (a *Artifact) SetDomainMetadata(meta *metadata.DomainMetadata) {
	a.TypedMetadata = meta
	a.SyncMetadata()
}

// SetIPMetadata establece metadata de IP en un artifact existente.
func (a *Artifact) SetIPMetadata(meta *metadata.IPMetadata) {
	a.TypedMetadata = meta
	a.SyncMetadata()
}

// GetTechnologyMetadata retorna el metadata de tecnología si existe.
func (a *Artifact) GetTechnologyMetadata() *metadata.TechnologyMetadata {
	if meta, ok := a.TypedMetadata.(*metadata.TechnologyMetadata); ok {
		return meta
	}
	return nil
}

// GetDomainMetadata retorna el metadata de dominio si existe.
func (a *Artifact) GetDomainMetadata() *metadata.DomainMetadata {
	if meta, ok := a.TypedMetadata.(*metadata.DomainMetadata); ok {
		return meta
	}
	return nil
}

// GetIPMetadata retorna el metadata de IP si existe.
func (a *Artifact) GetIPMetadata() *metadata.IPMetadata {
	if meta, ok := a.TypedMetadata.(*metadata.IPMetadata); ok {
		return meta
	}
	return nil
}
