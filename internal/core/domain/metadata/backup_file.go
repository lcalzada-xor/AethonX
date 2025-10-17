// internal/core/domain/metadata/backup_file.go
package metadata

// BackupFileMetadata contiene información detallada sobre archivos de backup expuestos.
type BackupFileMetadata struct {
	// Identificación del backup
	Filename  string // "database.sql.bak", "site-backup.zip"
	Extension string // ".bak", ".old", ".zip", ".sql", ".tar.gz"

	// Tipo de backup
	BackupType string // "database", "source_code", "config", "full_site", "unknown"
	Format     string // "sql", "zip", "tar", "gz", "7z", "rar"

	// Ubicación
	URL  string // URL completa del archivo
	Path string // Path relativo

	// Tamaño y fecha
	Size         int64  // Tamaño en bytes
	SizeHuman    string // "45.2 MB"
	LastModified string // Fecha de última modificación

	// Accesibilidad
	Accessible   bool // Si es accesible públicamente
	StatusCode   int  // 200, 403, etc.
	RequiresAuth bool // Si requiere autenticación

	// Contenido (si se pudo analizar)
	IsCompressed   bool
	ContainsSQL    bool // Si contiene dumps SQL
	ContainsCode   bool // Si contiene código fuente
	ContainsConfig bool // Si contiene configs (.env, etc.)

	// Información sensible detectada
	HasPasswords   bool
	HasAPIKeys     bool
	HasCredentials bool
	HasPII         bool // Personally Identifiable Information

	// Metadata del archivo
	CreatedBy        string  // Software que lo creó (si detectable)
	CompressionRatio float64

	// Riesgo
	RiskLevel string // "low", "medium", "high", "critical"
	Severity  string // Impacto potencial

	// Descarga
	Downloadable bool
	Downloaded   bool   // Si ya fue descargado
	LocalPath    string // Path local si fue descargado

	// Hashes (para deduplicación)
	MD5    string
	SHA256 string
}

func (b *BackupFileMetadata) ToMap() map[string]string {
	m := make(map[string]string)
	SetIfNotEmpty(m, "filename", b.Filename)
	SetIfNotEmpty(m, "extension", b.Extension)
	SetIfNotEmpty(m, "backup_type", b.BackupType)
	SetIfNotEmpty(m, "format", b.Format)
	SetIfNotEmpty(m, "url", b.URL)
	SetIfNotEmpty(m, "path", b.Path)
	if b.Size > 0 {
		SetInt64(m, "size", b.Size)
	}
	SetIfNotEmpty(m, "size_human", b.SizeHuman)
	SetIfNotEmpty(m, "last_modified", b.LastModified)
	SetBool(m, "accessible", b.Accessible)
	if b.StatusCode > 0 {
		SetInt(m, "status_code", b.StatusCode)
	}
	SetBool(m, "requires_auth", b.RequiresAuth)
	SetBool(m, "is_compressed", b.IsCompressed)
	SetBool(m, "contains_sql", b.ContainsSQL)
	SetBool(m, "contains_code", b.ContainsCode)
	SetBool(m, "contains_config", b.ContainsConfig)
	SetBool(m, "has_passwords", b.HasPasswords)
	SetBool(m, "has_api_keys", b.HasAPIKeys)
	SetBool(m, "has_credentials", b.HasCredentials)
	SetBool(m, "has_pii", b.HasPII)
	SetIfNotEmpty(m, "created_by", b.CreatedBy)
	if b.CompressionRatio > 0 {
		SetFloat(m, "compression_ratio", b.CompressionRatio)
	}
	SetIfNotEmpty(m, "risk_level", b.RiskLevel)
	SetIfNotEmpty(m, "severity", b.Severity)
	SetBool(m, "downloadable", b.Downloadable)
	SetBool(m, "downloaded", b.Downloaded)
	SetIfNotEmpty(m, "local_path", b.LocalPath)
	SetIfNotEmpty(m, "md5", b.MD5)
	SetIfNotEmpty(m, "sha256", b.SHA256)
	return m
}

func (b *BackupFileMetadata) FromMap(m map[string]string) error {
	b.Filename = GetString(m, "filename", "")
	b.Extension = GetString(m, "extension", "")
	b.BackupType = GetString(m, "backup_type", "")
	b.Format = GetString(m, "format", "")
	b.URL = GetString(m, "url", "")
	b.Path = GetString(m, "path", "")
	b.Size = GetInt64(m, "size", 0)
	b.SizeHuman = GetString(m, "size_human", "")
	b.LastModified = GetString(m, "last_modified", "")
	b.Accessible = GetBool(m, "accessible", false)
	b.StatusCode = GetInt(m, "status_code", 0)
	b.RequiresAuth = GetBool(m, "requires_auth", false)
	b.IsCompressed = GetBool(m, "is_compressed", false)
	b.ContainsSQL = GetBool(m, "contains_sql", false)
	b.ContainsCode = GetBool(m, "contains_code", false)
	b.ContainsConfig = GetBool(m, "contains_config", false)
	b.HasPasswords = GetBool(m, "has_passwords", false)
	b.HasAPIKeys = GetBool(m, "has_api_keys", false)
	b.HasCredentials = GetBool(m, "has_credentials", false)
	b.HasPII = GetBool(m, "has_pii", false)
	b.CreatedBy = GetString(m, "created_by", "")
	b.CompressionRatio = GetFloat(m, "compression_ratio", 0)
	b.RiskLevel = GetString(m, "risk_level", "")
	b.Severity = GetString(m, "severity", "")
	b.Downloadable = GetBool(m, "downloadable", false)
	b.Downloaded = GetBool(m, "downloaded", false)
	b.LocalPath = GetString(m, "local_path", "")
	b.MD5 = GetString(m, "md5", "")
	b.SHA256 = GetString(m, "sha256", "")
	return nil
}

func (b *BackupFileMetadata) IsValid() bool { return b.Filename != "" }
func (b *BackupFileMetadata) Type() string  { return "backup_file" }

// NewBackupFileMetadata crea una instancia de BackupFileMetadata con valores por defecto.
func NewBackupFileMetadata(filename string) *BackupFileMetadata {
	return &BackupFileMetadata{
		Filename: filename,
	}
}
