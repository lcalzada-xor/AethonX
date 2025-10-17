// internal/core/domain/metadata/api.go
package metadata

// APIMetadata contiene información detallada sobre una API detectada.
type APIMetadata struct {
	// Identificación
	Name       string // Nombre de la API
	APIType    string // "rest", "graphql", "soap", "grpc", "websocket"
	Version    string // "v1", "v2", "2.0"

	// Endpoint base
	BaseURL string // "https://api.example.com/v1"

	// Documentación
	HasDocumentation bool
	DocsURL          string // Swagger/OpenAPI URL
	DocsFormat       string // "swagger", "openapi3", "raml", "graphql_schema"

	// Autenticación
	AuthRequired bool
	AuthMethods  []string // "bearer", "api_key", "oauth2", "basic", "jwt"
	AuthLocation string   // "header", "query", "cookie"

	// GraphQL específico
	IntrospectionEnabled bool   // Si GraphQL introspection está activo
	HasMutations         bool
	HasSubscriptions     bool
	SchemaURL            string

	// REST específico
	Methods        []string // GET, POST, PUT, DELETE, PATCH
	Endpoints      []string // Lista de endpoints descubiertos
	HasRateLimit   bool
	RateLimitValue string // "100 req/min"

	// Seguridad
	HasCORS    bool
	CORSOrigin string // "*", "specific-domain.com"
	HasCSRF    bool
	HTTPSOnly  bool

	// Versioning
	VersioningScheme   string   // "url", "header", "query"
	SupportedVersions  []string
	DeprecatedVersions []string

	// Response format
	ResponseFormat []string // "json", "xml", "protobuf", "msgpack"

	// Errores y comportamiento
	ErrorFormat       string // "json", "xml", "plain"
	HasDetailedErrors bool

	// Technologies
	Framework string // "express", "fastapi", "spring-boot"
	Language  string // "nodejs", "python", "java"
}

func (a *APIMetadata) ToMap() map[string]string {
	m := make(map[string]string)
	SetIfNotEmpty(m, "name", a.Name)
	SetIfNotEmpty(m, "api_type", a.APIType)
	SetIfNotEmpty(m, "version", a.Version)
	SetIfNotEmpty(m, "base_url", a.BaseURL)
	SetBool(m, "has_documentation", a.HasDocumentation)
	SetIfNotEmpty(m, "docs_url", a.DocsURL)
	SetIfNotEmpty(m, "docs_format", a.DocsFormat)
	SetBool(m, "auth_required", a.AuthRequired)
	if len(a.AuthMethods) > 0 {
		m["auth_methods"] = StringSliceToCSV(a.AuthMethods)
	}
	SetIfNotEmpty(m, "auth_location", a.AuthLocation)
	SetBool(m, "introspection_enabled", a.IntrospectionEnabled)
	SetBool(m, "has_mutations", a.HasMutations)
	SetBool(m, "has_subscriptions", a.HasSubscriptions)
	SetIfNotEmpty(m, "schema_url", a.SchemaURL)
	if len(a.Methods) > 0 {
		m["methods"] = StringSliceToCSV(a.Methods)
	}
	if len(a.Endpoints) > 0 {
		m["endpoints"] = StringSliceToCSV(a.Endpoints)
	}
	SetBool(m, "has_rate_limit", a.HasRateLimit)
	SetIfNotEmpty(m, "rate_limit_value", a.RateLimitValue)
	SetBool(m, "has_cors", a.HasCORS)
	SetIfNotEmpty(m, "cors_origin", a.CORSOrigin)
	SetBool(m, "has_csrf", a.HasCSRF)
	SetBool(m, "https_only", a.HTTPSOnly)
	SetIfNotEmpty(m, "versioning_scheme", a.VersioningScheme)
	if len(a.SupportedVersions) > 0 {
		m["supported_versions"] = StringSliceToCSV(a.SupportedVersions)
	}
	if len(a.DeprecatedVersions) > 0 {
		m["deprecated_versions"] = StringSliceToCSV(a.DeprecatedVersions)
	}
	if len(a.ResponseFormat) > 0 {
		m["response_format"] = StringSliceToCSV(a.ResponseFormat)
	}
	SetIfNotEmpty(m, "error_format", a.ErrorFormat)
	SetBool(m, "has_detailed_errors", a.HasDetailedErrors)
	SetIfNotEmpty(m, "framework", a.Framework)
	SetIfNotEmpty(m, "language", a.Language)
	return m
}

func (a *APIMetadata) FromMap(m map[string]string) error {
	a.Name = GetString(m, "name", "")
	a.APIType = GetString(m, "api_type", "")
	a.Version = GetString(m, "version", "")
	a.BaseURL = GetString(m, "base_url", "")
	a.HasDocumentation = GetBool(m, "has_documentation", false)
	a.DocsURL = GetString(m, "docs_url", "")
	a.DocsFormat = GetString(m, "docs_format", "")
	a.AuthRequired = GetBool(m, "auth_required", false)
	a.AuthMethods = CSVToStringSlice(GetString(m, "auth_methods", ""))
	a.AuthLocation = GetString(m, "auth_location", "")
	a.IntrospectionEnabled = GetBool(m, "introspection_enabled", false)
	a.HasMutations = GetBool(m, "has_mutations", false)
	a.HasSubscriptions = GetBool(m, "has_subscriptions", false)
	a.SchemaURL = GetString(m, "schema_url", "")
	a.Methods = CSVToStringSlice(GetString(m, "methods", ""))
	a.Endpoints = CSVToStringSlice(GetString(m, "endpoints", ""))
	a.HasRateLimit = GetBool(m, "has_rate_limit", false)
	a.RateLimitValue = GetString(m, "rate_limit_value", "")
	a.HasCORS = GetBool(m, "has_cors", false)
	a.CORSOrigin = GetString(m, "cors_origin", "")
	a.HasCSRF = GetBool(m, "has_csrf", false)
	a.HTTPSOnly = GetBool(m, "https_only", false)
	a.VersioningScheme = GetString(m, "versioning_scheme", "")
	a.SupportedVersions = CSVToStringSlice(GetString(m, "supported_versions", ""))
	a.DeprecatedVersions = CSVToStringSlice(GetString(m, "deprecated_versions", ""))
	a.ResponseFormat = CSVToStringSlice(GetString(m, "response_format", ""))
	a.ErrorFormat = GetString(m, "error_format", "")
	a.HasDetailedErrors = GetBool(m, "has_detailed_errors", false)
	a.Framework = GetString(m, "framework", "")
	a.Language = GetString(m, "language", "")
	return nil
}

func (a *APIMetadata) IsValid() bool { return a.BaseURL != "" || a.APIType != "" }
func (a *APIMetadata) Type() string  { return "api" }

// NewAPIMetadata crea una instancia de APIMetadata con valores por defecto.
func NewAPIMetadata(apiType, baseURL string) *APIMetadata {
	return &APIMetadata{
		APIType: apiType,
		BaseURL: baseURL,
	}
}
