// internal/core/domain/metadata/storage_bucket.go
package metadata

// StorageBucketMetadata contiene información detallada sobre buckets de almacenamiento en la nube.
type StorageBucketMetadata struct {
	// Proveedor
	Provider string // "aws_s3", "azure_blob", "gcp_storage", "digitalocean_spaces"

	// Identificación
	BucketName string // Nombre del bucket
	BucketURL  string // URL del bucket
	Region     string // us-east-1, eu-west-1, etc.

	// Acceso
	PublicAccess bool     // Si tiene acceso público
	Permissions  []string // "read", "write", "list", "delete"
	RequiresAuth bool
	AuthMethod   string // "none", "api_key", "iam", "sas"

	// Listado
	IsListable bool  // Si se puede listar contenido
	FileCount  int   // Número de archivos (si listable)
	TotalSize  int64 // Tamaño total en bytes

	// Contenido detectado
	FileTypes  []string // Extensiones encontradas
	HasHTML    bool
	HasJS      bool
	HasImages  bool
	HasBackups bool
	HasLogs    bool
	HasConfigs bool

	// Información sensible
	HasSecrets  bool
	SecretTypes []string // "api_key", "password", "certificate"

	// Configuración
	Versioning bool // Si tiene versioning habilitado
	Encryption bool // Si tiene encryption
	Logging    bool // Si tiene logging
	Website    bool // Si está configurado como website

	// CORS
	HasCORS    bool
	CORSPolicy string // Resumen de política CORS

	// Riesgo
	RiskLevel     string // "low", "medium", "high", "critical"
	Misconfigured bool   // Si está mal configurado

	// Metadatos AWS específicos
	S3ACL    string // "public-read", "private", etc.
	S3Policy string // Bucket policy (si accesible)

	// Metadatos Azure específicos
	AzureContainer string // Nombre del container
	AzureSAS       bool   // Si usa SAS tokens

	// Detection
	DetectionMethod string // "dns", "permutation", "google_dork"
}

func (s *StorageBucketMetadata) ToMap() map[string]string {
	m := make(map[string]string)
	SetIfNotEmpty(m, "provider", s.Provider)
	SetIfNotEmpty(m, "bucket_name", s.BucketName)
	SetIfNotEmpty(m, "bucket_url", s.BucketURL)
	SetIfNotEmpty(m, "region", s.Region)
	SetBool(m, "public_access", s.PublicAccess)
	if len(s.Permissions) > 0 {
		m["permissions"] = StringSliceToCSV(s.Permissions)
	}
	SetBool(m, "requires_auth", s.RequiresAuth)
	SetIfNotEmpty(m, "auth_method", s.AuthMethod)
	SetBool(m, "is_listable", s.IsListable)
	if s.FileCount > 0 {
		SetInt(m, "file_count", s.FileCount)
	}
	if s.TotalSize > 0 {
		SetInt64(m, "total_size", s.TotalSize)
	}
	if len(s.FileTypes) > 0 {
		m["file_types"] = StringSliceToCSV(s.FileTypes)
	}
	SetBool(m, "has_html", s.HasHTML)
	SetBool(m, "has_js", s.HasJS)
	SetBool(m, "has_images", s.HasImages)
	SetBool(m, "has_backups", s.HasBackups)
	SetBool(m, "has_logs", s.HasLogs)
	SetBool(m, "has_configs", s.HasConfigs)
	SetBool(m, "has_secrets", s.HasSecrets)
	if len(s.SecretTypes) > 0 {
		m["secret_types"] = StringSliceToCSV(s.SecretTypes)
	}
	SetBool(m, "versioning", s.Versioning)
	SetBool(m, "encryption", s.Encryption)
	SetBool(m, "logging", s.Logging)
	SetBool(m, "website", s.Website)
	SetBool(m, "has_cors", s.HasCORS)
	SetIfNotEmpty(m, "cors_policy", s.CORSPolicy)
	SetIfNotEmpty(m, "risk_level", s.RiskLevel)
	SetBool(m, "misconfigured", s.Misconfigured)
	SetIfNotEmpty(m, "s3_acl", s.S3ACL)
	SetIfNotEmpty(m, "s3_policy", s.S3Policy)
	SetIfNotEmpty(m, "azure_container", s.AzureContainer)
	SetBool(m, "azure_sas", s.AzureSAS)
	SetIfNotEmpty(m, "detection_method", s.DetectionMethod)
	return m
}

func (s *StorageBucketMetadata) FromMap(m map[string]string) error {
	s.Provider = GetString(m, "provider", "")
	s.BucketName = GetString(m, "bucket_name", "")
	s.BucketURL = GetString(m, "bucket_url", "")
	s.Region = GetString(m, "region", "")
	s.PublicAccess = GetBool(m, "public_access", false)
	s.Permissions = CSVToStringSlice(GetString(m, "permissions", ""))
	s.RequiresAuth = GetBool(m, "requires_auth", false)
	s.AuthMethod = GetString(m, "auth_method", "")
	s.IsListable = GetBool(m, "is_listable", false)
	s.FileCount = GetInt(m, "file_count", 0)
	s.TotalSize = GetInt64(m, "total_size", 0)
	s.FileTypes = CSVToStringSlice(GetString(m, "file_types", ""))
	s.HasHTML = GetBool(m, "has_html", false)
	s.HasJS = GetBool(m, "has_js", false)
	s.HasImages = GetBool(m, "has_images", false)
	s.HasBackups = GetBool(m, "has_backups", false)
	s.HasLogs = GetBool(m, "has_logs", false)
	s.HasConfigs = GetBool(m, "has_configs", false)
	s.HasSecrets = GetBool(m, "has_secrets", false)
	s.SecretTypes = CSVToStringSlice(GetString(m, "secret_types", ""))
	s.Versioning = GetBool(m, "versioning", false)
	s.Encryption = GetBool(m, "encryption", false)
	s.Logging = GetBool(m, "logging", false)
	s.Website = GetBool(m, "website", false)
	s.HasCORS = GetBool(m, "has_cors", false)
	s.CORSPolicy = GetString(m, "cors_policy", "")
	s.RiskLevel = GetString(m, "risk_level", "")
	s.Misconfigured = GetBool(m, "misconfigured", false)
	s.S3ACL = GetString(m, "s3_acl", "")
	s.S3Policy = GetString(m, "s3_policy", "")
	s.AzureContainer = GetString(m, "azure_container", "")
	s.AzureSAS = GetBool(m, "azure_sas", false)
	s.DetectionMethod = GetString(m, "detection_method", "")
	return nil
}

func (s *StorageBucketMetadata) IsValid() bool { return s.BucketName != "" }
func (s *StorageBucketMetadata) Type() string  { return "storage_bucket" }

// NewStorageBucketMetadata crea una instancia de StorageBucketMetadata con valores por defecto.
func NewStorageBucketMetadata(provider, bucketName string) *StorageBucketMetadata {
	return &StorageBucketMetadata{
		Provider:   provider,
		BucketName: bucketName,
	}
}
