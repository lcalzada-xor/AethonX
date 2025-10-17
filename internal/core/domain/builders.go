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

// NewServiceArtifact crea un artifact de servicio con metadata tipado.
func NewServiceArtifact(name string, port int, source string) *Artifact {
	meta := metadata.NewServiceMetadata(name, port)

	return NewArtifactWithMetadata(
		ArtifactTypeService,
		name,
		source,
		meta,
	)
}

// NewWAFArtifact crea un artifact de WAF con metadata tipado.
func NewWAFArtifact(name, source string) *Artifact {
	meta := metadata.NewWAFMetadata(name)

	return NewArtifactWithMetadata(
		ArtifactTypeWAF,
		name,
		source,
		meta,
	)
}

// NewAPIArtifact crea un artifact de API con metadata tipado.
func NewAPIArtifact(apiType, baseURL, source string) *Artifact {
	meta := metadata.NewAPIMetadata(apiType, baseURL)

	return NewArtifactWithMetadata(
		ArtifactTypeAPI,
		baseURL,
		source,
		meta,
	)
}

// NewRepositoryArtifact crea un artifact de repositorio con metadata tipado.
func NewRepositoryArtifact(repoType, url, source string) *Artifact {
	meta := metadata.NewRepositoryMetadata(repoType)

	return NewArtifactWithMetadata(
		ArtifactTypeRepository,
		url,
		source,
		meta,
	)
}

// NewBackupFileArtifact crea un artifact de backup file con metadata tipado.
func NewBackupFileArtifact(filename, source string) *Artifact {
	meta := metadata.NewBackupFileMetadata(filename)

	return NewArtifactWithMetadata(
		ArtifactTypeBackupFile,
		filename,
		source,
		meta,
	)
}

// NewStorageBucketArtifact crea un artifact de storage bucket con metadata tipado.
func NewStorageBucketArtifact(provider, bucketName, source string) *Artifact {
	meta := metadata.NewStorageBucketMetadata(provider, bucketName)

	return NewArtifactWithMetadata(
		ArtifactTypeStorageBucket,
		bucketName,
		source,
		meta,
	)
}

// NewWebshellArtifact crea un artifact de webshell con metadata tipado.
func NewWebshellArtifact(name, shellType, source string) *Artifact {
	meta := metadata.NewWebshellMetadata(name, shellType)

	return NewArtifactWithMetadata(
		ArtifactTypeWebshell,
		name,
		source,
		meta,
	)
}

// GetServiceMetadata retorna el metadata de servicio si existe.
func (a *Artifact) GetServiceMetadata() *metadata.ServiceMetadata {
	if meta, ok := a.TypedMetadata.(*metadata.ServiceMetadata); ok {
		return meta
	}
	return nil
}

// GetWAFMetadata retorna el metadata de WAF si existe.
func (a *Artifact) GetWAFMetadata() *metadata.WAFMetadata {
	if meta, ok := a.TypedMetadata.(*metadata.WAFMetadata); ok {
		return meta
	}
	return nil
}

// GetAPIMetadata retorna el metadata de API si existe.
func (a *Artifact) GetAPIMetadata() *metadata.APIMetadata {
	if meta, ok := a.TypedMetadata.(*metadata.APIMetadata); ok {
		return meta
	}
	return nil
}

// GetRepositoryMetadata retorna el metadata de repositorio si existe.
func (a *Artifact) GetRepositoryMetadata() *metadata.RepositoryMetadata {
	if meta, ok := a.TypedMetadata.(*metadata.RepositoryMetadata); ok {
		return meta
	}
	return nil
}

// GetBackupFileMetadata retorna el metadata de backup file si existe.
func (a *Artifact) GetBackupFileMetadata() *metadata.BackupFileMetadata {
	if meta, ok := a.TypedMetadata.(*metadata.BackupFileMetadata); ok {
		return meta
	}
	return nil
}

// GetStorageBucketMetadata retorna el metadata de storage bucket si existe.
func (a *Artifact) GetStorageBucketMetadata() *metadata.StorageBucketMetadata {
	if meta, ok := a.TypedMetadata.(*metadata.StorageBucketMetadata); ok {
		return meta
	}
	return nil
}

// GetWebshellMetadata retorna el metadata de webshell si existe.
func (a *Artifact) GetWebshellMetadata() *metadata.WebshellMetadata {
	if meta, ok := a.TypedMetadata.(*metadata.WebshellMetadata); ok {
		return meta
	}
	return nil
}

// SetServiceMetadata establece metadata de servicio en un artifact existente.
func (a *Artifact) SetServiceMetadata(meta *metadata.ServiceMetadata) {
	a.TypedMetadata = meta
	a.SyncMetadata()
}

// SetWAFMetadata establece metadata de WAF en un artifact existente.
func (a *Artifact) SetWAFMetadata(meta *metadata.WAFMetadata) {
	a.TypedMetadata = meta
	a.SyncMetadata()
}

// SetAPIMetadata establece metadata de API en un artifact existente.
func (a *Artifact) SetAPIMetadata(meta *metadata.APIMetadata) {
	a.TypedMetadata = meta
	a.SyncMetadata()
}

// SetRepositoryMetadata establece metadata de repositorio en un artifact existente.
func (a *Artifact) SetRepositoryMetadata(meta *metadata.RepositoryMetadata) {
	a.TypedMetadata = meta
	a.SyncMetadata()
}

// SetBackupFileMetadata establece metadata de backup file en un artifact existente.
func (a *Artifact) SetBackupFileMetadata(meta *metadata.BackupFileMetadata) {
	a.TypedMetadata = meta
	a.SyncMetadata()
}

// SetStorageBucketMetadata establece metadata de storage bucket en un artifact existente.
func (a *Artifact) SetStorageBucketMetadata(meta *metadata.StorageBucketMetadata) {
	a.TypedMetadata = meta
	a.SyncMetadata()
}

// SetWebshellMetadata establece metadata de webshell en un artifact existente.
func (a *Artifact) SetWebshellMetadata(meta *metadata.WebshellMetadata) {
	a.TypedMetadata = meta
	a.SyncMetadata()
}
