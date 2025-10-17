// internal/core/domain/metadata/technology.go
package metadata

// TechnologyMetadata contiene información detallada sobre una tecnología detectada.
type TechnologyMetadata struct {
	// Identificación
	Name        string // Nombre canónico (ej: "nginx")
	DisplayName string // Nombre para mostrar (ej: "Nginx")
	Category    string // web-server, framework, cms, cdn, analytics, etc.
	Subcategory string // reverse-proxy, orm, admin-panel, etc.

	// Versión
	Version         string  // Versión exacta detectada (ej: "1.24.0")
	VersionDetected bool    // Si se detectó versión o es inferida
	VersionConfidence float64 // Confianza en la versión [0.0-1.0]
	LatestVersion   string  // Última versión conocida
	Outdated        bool    // Si está desactualizada

	// Detalles de versión
	MajorVersion string
	MinorVersion string
	PatchVersion string
	BuildNumber  string

	// Detección
	DetectionMethod  string // http_header, html_meta, js_library, favicon_hash, etc.
	DetectionPattern string // Patrón que se matcheó
	DetectionLocation string // URL donde se detectó

	// Información adicional
	Vendor  string // Empresa/organización (ej: "F5 Networks")
	Website string // Sitio web oficial
	CPE     string // Common Platform Enumeration
	License string // Tipo de licencia

	// Seguridad
	HasKnownVulns bool     // Si tiene vulnerabilidades conocidas
	CVECount      int      // Cantidad de CVEs
	CVEList       []string // Lista de CVEs
	RiskLevel     string   // low, medium, high, critical

	// Metadatos de uso
	ConfidenceScore float64 // Confianza general en la detección
	PopularityRank  int     // Ranking de popularidad
	FirstRelease    string  // Fecha primera release

	// Módulos/Plugins
	Modules []string // Módulos detectados
	Plugins []string // Plugins detectados

	// Stack relacionado
	Implies  []string // Tecnologías que implica (ej: nginx -> linux, openssl)
	Excludes []string // Tecnologías que excluye (ej: nginx -> apache)

	// URLs relevantes
	IconURL       string
	Documentation string
}

// ToMap convierte TechnologyMetadata a map[string]string.
func (t *TechnologyMetadata) ToMap() map[string]string {
	m := make(map[string]string)

	// Identificación
	SetIfNotEmpty(m, "name", t.Name)
	SetIfNotEmpty(m, "display_name", t.DisplayName)
	SetIfNotEmpty(m, "category", t.Category)
	SetIfNotEmpty(m, "subcategory", t.Subcategory)

	// Versión
	SetIfNotEmpty(m, "version", t.Version)
	SetBool(m, "version_detected", t.VersionDetected)
	if t.VersionConfidence > 0 {
		SetFloat(m, "version_confidence", t.VersionConfidence)
	}
	SetIfNotEmpty(m, "latest_version", t.LatestVersion)
	SetBool(m, "outdated", t.Outdated)

	// Detalles de versión
	SetIfNotEmpty(m, "major_version", t.MajorVersion)
	SetIfNotEmpty(m, "minor_version", t.MinorVersion)
	SetIfNotEmpty(m, "patch_version", t.PatchVersion)
	SetIfNotEmpty(m, "build_number", t.BuildNumber)

	// Detección
	SetIfNotEmpty(m, "detection_method", t.DetectionMethod)
	SetIfNotEmpty(m, "detection_pattern", t.DetectionPattern)
	SetIfNotEmpty(m, "detection_location", t.DetectionLocation)

	// Información adicional
	SetIfNotEmpty(m, "vendor", t.Vendor)
	SetIfNotEmpty(m, "website", t.Website)
	SetIfNotEmpty(m, "cpe", t.CPE)
	SetIfNotEmpty(m, "license", t.License)

	// Seguridad
	SetBool(m, "has_known_vulns", t.HasKnownVulns)
	if t.CVECount > 0 {
		SetInt(m, "cve_count", t.CVECount)
	}
	if len(t.CVEList) > 0 {
		m["cve_list"] = StringSliceToCSV(t.CVEList)
	}
	SetIfNotEmpty(m, "risk_level", t.RiskLevel)

	// Metadatos
	if t.ConfidenceScore > 0 {
		SetFloat(m, "confidence_score", t.ConfidenceScore)
	}
	if t.PopularityRank > 0 {
		SetInt(m, "popularity_rank", t.PopularityRank)
	}
	SetIfNotEmpty(m, "first_release", t.FirstRelease)

	// Módulos/Plugins
	if len(t.Modules) > 0 {
		m["modules"] = StringSliceToCSV(t.Modules)
	}
	if len(t.Plugins) > 0 {
		m["plugins"] = StringSliceToCSV(t.Plugins)
	}

	// Stack
	if len(t.Implies) > 0 {
		m["implies"] = StringSliceToCSV(t.Implies)
	}
	if len(t.Excludes) > 0 {
		m["excludes"] = StringSliceToCSV(t.Excludes)
	}

	// URLs
	SetIfNotEmpty(m, "icon_url", t.IconURL)
	SetIfNotEmpty(m, "documentation", t.Documentation)

	return m
}

// FromMap carga TechnologyMetadata desde map[string]string.
func (t *TechnologyMetadata) FromMap(m map[string]string) error {
	// Identificación
	t.Name = GetString(m, "name", "")
	t.DisplayName = GetString(m, "display_name", "")
	t.Category = GetString(m, "category", "")
	t.Subcategory = GetString(m, "subcategory", "")

	// Versión
	t.Version = GetString(m, "version", "")
	t.VersionDetected = GetBool(m, "version_detected", false)
	t.VersionConfidence = GetFloat(m, "version_confidence", 0.0)
	t.LatestVersion = GetString(m, "latest_version", "")
	t.Outdated = GetBool(m, "outdated", false)

	// Detalles de versión
	t.MajorVersion = GetString(m, "major_version", "")
	t.MinorVersion = GetString(m, "minor_version", "")
	t.PatchVersion = GetString(m, "patch_version", "")
	t.BuildNumber = GetString(m, "build_number", "")

	// Detección
	t.DetectionMethod = GetString(m, "detection_method", "")
	t.DetectionPattern = GetString(m, "detection_pattern", "")
	t.DetectionLocation = GetString(m, "detection_location", "")

	// Información adicional
	t.Vendor = GetString(m, "vendor", "")
	t.Website = GetString(m, "website", "")
	t.CPE = GetString(m, "cpe", "")
	t.License = GetString(m, "license", "")

	// Seguridad
	t.HasKnownVulns = GetBool(m, "has_known_vulns", false)
	t.CVECount = GetInt(m, "cve_count", 0)
	t.CVEList = CSVToStringSlice(GetString(m, "cve_list", ""))
	t.RiskLevel = GetString(m, "risk_level", "")

	// Metadatos
	t.ConfidenceScore = GetFloat(m, "confidence_score", 0.0)
	t.PopularityRank = GetInt(m, "popularity_rank", 0)
	t.FirstRelease = GetString(m, "first_release", "")

	// Módulos/Plugins
	t.Modules = CSVToStringSlice(GetString(m, "modules", ""))
	t.Plugins = CSVToStringSlice(GetString(m, "plugins", ""))

	// Stack
	t.Implies = CSVToStringSlice(GetString(m, "implies", ""))
	t.Excludes = CSVToStringSlice(GetString(m, "excludes", ""))

	// URLs
	t.IconURL = GetString(m, "icon_url", "")
	t.Documentation = GetString(m, "documentation", "")

	return nil
}

// IsValid verifica si el metadata tiene datos válidos mínimos.
func (t *TechnologyMetadata) IsValid() bool {
	return t.Name != ""
}

// Type retorna el tipo de metadata.
func (t *TechnologyMetadata) Type() string {
	return "technology"
}

// NewTechnologyMetadata crea un nuevo TechnologyMetadata con valores básicos.
func NewTechnologyMetadata(name, version string) *TechnologyMetadata {
	return &TechnologyMetadata{
		Name:              name,
		DisplayName:       name,
		Version:           version,
		VersionDetected:   version != "",
		VersionConfidence: 1.0,
		ConfidenceScore:   0.8,
		CVEList:           []string{},
		Modules:           []string{},
		Plugins:           []string{},
		Implies:           []string{},
		Excludes:          []string{},
	}
}
