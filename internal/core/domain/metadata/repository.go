// internal/core/domain/metadata/repository.go
package metadata

// RepositoryMetadata contiene información detallada sobre repositorios de código expuestos.
type RepositoryMetadata struct {
	// Tipo de repositorio
	RepoType string // "git", "svn", "mercurial", "cvs"

	// Accesibilidad
	Accessible bool // Si el repo es accesible públicamente
	Protected  bool // Si tiene algún tipo de protección

	// Git específico
	GitURL     string // URL del .git/
	HasConfig  bool   // Si .git/config es accesible
	HasHead    bool   // Si .git/HEAD es accesible
	HasLogs    bool   // Si .git/logs/ es accesible
	HasRefs    bool   // Si .git/refs/ es accesible
	HasObjects bool   // Si .git/objects/ es accesible

	// Información extraída
	Branches    []string // Branches accesibles
	Tags        []string // Tags encontradas
	RemoteURL   string   // URL remota del repo (de .git/config)
	LastCommit  string   // Hash del último commit
	CommitCount int      // Número de commits accesibles

	// Contenido sensible
	HasSecrets  bool     // Si se encontraron secrets en commits
	SecretTypes []string // "api_key", "password", "token", "private_key"

	// Riesgo
	RiskLevel   string // "low", "medium", "high", "critical"
	CanDownload bool   // Si el repo completo es descargable

	// Metadatos adicionales
	Size      int64 // Tamaño estimado en bytes
	FileCount int   // Número de archivos

	// Tools para download
	DownloadTool string // "git-dumper", "dvcs-ripper", "wget"
}

func (r *RepositoryMetadata) ToMap() map[string]string {
	m := make(map[string]string)
	SetIfNotEmpty(m, "repo_type", r.RepoType)
	SetBool(m, "accessible", r.Accessible)
	SetBool(m, "protected", r.Protected)
	SetIfNotEmpty(m, "git_url", r.GitURL)
	SetBool(m, "has_config", r.HasConfig)
	SetBool(m, "has_head", r.HasHead)
	SetBool(m, "has_logs", r.HasLogs)
	SetBool(m, "has_refs", r.HasRefs)
	SetBool(m, "has_objects", r.HasObjects)
	if len(r.Branches) > 0 {
		m["branches"] = StringSliceToCSV(r.Branches)
	}
	if len(r.Tags) > 0 {
		m["tags"] = StringSliceToCSV(r.Tags)
	}
	SetIfNotEmpty(m, "remote_url", r.RemoteURL)
	SetIfNotEmpty(m, "last_commit", r.LastCommit)
	if r.CommitCount > 0 {
		SetInt(m, "commit_count", r.CommitCount)
	}
	SetBool(m, "has_secrets", r.HasSecrets)
	if len(r.SecretTypes) > 0 {
		m["secret_types"] = StringSliceToCSV(r.SecretTypes)
	}
	SetIfNotEmpty(m, "risk_level", r.RiskLevel)
	SetBool(m, "can_download", r.CanDownload)
	if r.Size > 0 {
		SetInt64(m, "size", r.Size)
	}
	if r.FileCount > 0 {
		SetInt(m, "file_count", r.FileCount)
	}
	SetIfNotEmpty(m, "download_tool", r.DownloadTool)
	return m
}

func (r *RepositoryMetadata) FromMap(m map[string]string) error {
	r.RepoType = GetString(m, "repo_type", "")
	r.Accessible = GetBool(m, "accessible", false)
	r.Protected = GetBool(m, "protected", false)
	r.GitURL = GetString(m, "git_url", "")
	r.HasConfig = GetBool(m, "has_config", false)
	r.HasHead = GetBool(m, "has_head", false)
	r.HasLogs = GetBool(m, "has_logs", false)
	r.HasRefs = GetBool(m, "has_refs", false)
	r.HasObjects = GetBool(m, "has_objects", false)
	r.Branches = CSVToStringSlice(GetString(m, "branches", ""))
	r.Tags = CSVToStringSlice(GetString(m, "tags", ""))
	r.RemoteURL = GetString(m, "remote_url", "")
	r.LastCommit = GetString(m, "last_commit", "")
	r.CommitCount = GetInt(m, "commit_count", 0)
	r.HasSecrets = GetBool(m, "has_secrets", false)
	r.SecretTypes = CSVToStringSlice(GetString(m, "secret_types", ""))
	r.RiskLevel = GetString(m, "risk_level", "")
	r.CanDownload = GetBool(m, "can_download", false)
	r.Size = GetInt64(m, "size", 0)
	r.FileCount = GetInt(m, "file_count", 0)
	r.DownloadTool = GetString(m, "download_tool", "")
	return nil
}

func (r *RepositoryMetadata) IsValid() bool { return r.RepoType != "" }
func (r *RepositoryMetadata) Type() string  { return "repository" }

// NewRepositoryMetadata crea una instancia de RepositoryMetadata con valores por defecto.
func NewRepositoryMetadata(repoType string) *RepositoryMetadata {
	return &RepositoryMetadata{
		RepoType: repoType,
	}
}
