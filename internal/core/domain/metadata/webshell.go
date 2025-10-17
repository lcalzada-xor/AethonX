// internal/core/domain/metadata/webshell.go
package metadata

import (
	"strconv"
)

// WebshellMetadata contiene información detallada sobre webshells detectadas.
type WebshellMetadata struct {
	// Identificación
	Name         string // "c99", "r57", "b374k", "wso", "china_chopper"
	WebshellType string // "php", "jsp", "asp", "aspx", "perl"
	Variant      string // Variante específica

	// Ubicación
	URL  string // URL del webshell
	Path string // Path del archivo

	// Detección
	DetectionMethod string  // "signature", "behavior", "static_analysis"
	Confidence      float64 // 0.0-1.0
	Signature       string  // Firma que matcheó

	// Características
	HasFileUpload   bool
	HasFileDownload bool
	HasCommandExec  bool
	HasSQLClient    bool
	HasPortScanner  bool
	HasBackconnect  bool
	HasBruteForce   bool

	// Funcionalidades avanzadas
	Obfuscated        bool // Si está ofuscado
	Encrypted         bool // Si usa encriptación
	HasPassword       bool // Si tiene password
	PasswordProtected bool

	// Timestamps
	FileCreated  string // Fecha de creación
	FileModified string // Última modificación
	LastAccessed string // Último acceso

	// Hash
	MD5    string
	SHA256 string

	// Metadatos del archivo
	Size        int64  // Tamaño en bytes
	Permissions string // "755", "644", etc.
	Owner       string // Usuario propietario

	// Indicadores de compromiso
	IOCs      []string // IPs, dominios, strings únicos
	C2Servers []string // Servidores C2 si los hay

	// Severidad
	RiskLevel   string // "high", "critical"
	ThreatLevel string // Nivel de amenaza

	// Remediación
	RemediationSteps []string
}

func (w *WebshellMetadata) ToMap() map[string]string {
	m := make(map[string]string)
	SetIfNotEmpty(m, "name", w.Name)
	SetIfNotEmpty(m, "webshell_type", w.WebshellType)
	SetIfNotEmpty(m, "variant", w.Variant)
	SetIfNotEmpty(m, "url", w.URL)
	SetIfNotEmpty(m, "path", w.Path)
	SetIfNotEmpty(m, "detection_method", w.DetectionMethod)
	if w.Confidence > 0 {
		m["confidence"] = strconv.FormatFloat(w.Confidence, 'f', 2, 64)
	}
	SetIfNotEmpty(m, "signature", w.Signature)
	SetBool(m, "has_file_upload", w.HasFileUpload)
	SetBool(m, "has_file_download", w.HasFileDownload)
	SetBool(m, "has_command_exec", w.HasCommandExec)
	SetBool(m, "has_sql_client", w.HasSQLClient)
	SetBool(m, "has_port_scanner", w.HasPortScanner)
	SetBool(m, "has_backconnect", w.HasBackconnect)
	SetBool(m, "has_brute_force", w.HasBruteForce)
	SetBool(m, "obfuscated", w.Obfuscated)
	SetBool(m, "encrypted", w.Encrypted)
	SetBool(m, "has_password", w.HasPassword)
	SetBool(m, "password_protected", w.PasswordProtected)
	SetIfNotEmpty(m, "file_created", w.FileCreated)
	SetIfNotEmpty(m, "file_modified", w.FileModified)
	SetIfNotEmpty(m, "last_accessed", w.LastAccessed)
	SetIfNotEmpty(m, "md5", w.MD5)
	SetIfNotEmpty(m, "sha256", w.SHA256)
	if w.Size > 0 {
		SetInt64(m, "size", w.Size)
	}
	SetIfNotEmpty(m, "permissions", w.Permissions)
	SetIfNotEmpty(m, "owner", w.Owner)
	if len(w.IOCs) > 0 {
		m["iocs"] = StringSliceToCSV(w.IOCs)
	}
	if len(w.C2Servers) > 0 {
		m["c2_servers"] = StringSliceToCSV(w.C2Servers)
	}
	SetIfNotEmpty(m, "risk_level", w.RiskLevel)
	SetIfNotEmpty(m, "threat_level", w.ThreatLevel)
	if len(w.RemediationSteps) > 0 {
		m["remediation_steps"] = StringSliceToCSV(w.RemediationSteps)
	}
	return m
}

func (w *WebshellMetadata) FromMap(m map[string]string) error {
	w.Name = GetString(m, "name", "")
	w.WebshellType = GetString(m, "webshell_type", "")
	w.Variant = GetString(m, "variant", "")
	w.URL = GetString(m, "url", "")
	w.Path = GetString(m, "path", "")
	w.DetectionMethod = GetString(m, "detection_method", "")
	confStr := GetString(m, "confidence", "0")
	if conf, err := strconv.ParseFloat(confStr, 64); err == nil {
		w.Confidence = conf
	}
	w.Signature = GetString(m, "signature", "")
	w.HasFileUpload = GetBool(m, "has_file_upload", false)
	w.HasFileDownload = GetBool(m, "has_file_download", false)
	w.HasCommandExec = GetBool(m, "has_command_exec", false)
	w.HasSQLClient = GetBool(m, "has_sql_client", false)
	w.HasPortScanner = GetBool(m, "has_port_scanner", false)
	w.HasBackconnect = GetBool(m, "has_backconnect", false)
	w.HasBruteForce = GetBool(m, "has_brute_force", false)
	w.Obfuscated = GetBool(m, "obfuscated", false)
	w.Encrypted = GetBool(m, "encrypted", false)
	w.HasPassword = GetBool(m, "has_password", false)
	w.PasswordProtected = GetBool(m, "password_protected", false)
	w.FileCreated = GetString(m, "file_created", "")
	w.FileModified = GetString(m, "file_modified", "")
	w.LastAccessed = GetString(m, "last_accessed", "")
	w.MD5 = GetString(m, "md5", "")
	w.SHA256 = GetString(m, "sha256", "")
	w.Size = GetInt64(m, "size", 0)
	w.Permissions = GetString(m, "permissions", "")
	w.Owner = GetString(m, "owner", "")
	w.IOCs = CSVToStringSlice(GetString(m, "iocs", ""))
	w.C2Servers = CSVToStringSlice(GetString(m, "c2_servers", ""))
	w.RiskLevel = GetString(m, "risk_level", "")
	w.ThreatLevel = GetString(m, "threat_level", "")
	w.RemediationSteps = CSVToStringSlice(GetString(m, "remediation_steps", ""))
	return nil
}

func (w *WebshellMetadata) IsValid() bool { return w.Name != "" }
func (w *WebshellMetadata) Type() string  { return "webshell" }

// NewWebshellMetadata crea una instancia de WebshellMetadata con valores por defecto.
func NewWebshellMetadata(name, shellType string) *WebshellMetadata {
	return &WebshellMetadata{
		Name:         name,
		WebshellType: shellType,
		Confidence:   1.0,
	}
}
