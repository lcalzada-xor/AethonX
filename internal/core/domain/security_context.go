// internal/core/domain/security_context.go
package domain

// Severity represents risk severity levels for security findings.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// SeverityWeight returns a numeric weight for severity comparison.
func (s Severity) Weight() int {
	switch s {
	case SeverityCritical:
		return 5
	case SeverityHigh:
		return 4
	case SeverityMedium:
		return 3
	case SeverityLow:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}

// VulnerabilityType represents types of security vulnerabilities.
type VulnerabilityType string

const (
	VulnTypeSQLi         VulnerabilityType = "sqli"
	VulnTypeXSS          VulnerabilityType = "xss"
	VulnTypeCredential   VulnerabilityType = "credential"
	VulnTypeAPIKey       VulnerabilityType = "api_key"
	VulnTypeToken        VulnerabilityType = "token"
	VulnTypeDBConnection VulnerabilityType = "db_connection"
	VulnTypeInternalIP   VulnerabilityType = "internal_ip"
)

// TokenType represents types of authentication tokens.
type TokenType string

const (
	TokenTypeJWT     TokenType = "jwt"
	TokenTypeGitHub  TokenType = "github"
	TokenTypeSlack   TokenType = "slack"
	TokenTypeOAuth2  TokenType = "oauth2"
	TokenTypeAWS     TokenType = "aws"
	TokenTypeGCP     TokenType = "gcp"
	TokenTypeAzure   TokenType = "azure"
	TokenTypeStripe  TokenType = "stripe"
	TokenTypeTwilio  TokenType = "twilio"
)

// CloudProvider represents cloud service providers.
type CloudProvider string

const (
	CloudProviderAWS   CloudProvider = "aws"
	CloudProviderGCP   CloudProvider = "gcp"
	CloudProviderAzure CloudProvider = "azure"
)

// SecurityContext contains security-related classification and assessment information.
type SecurityContext struct {
	// Severity indicates the risk level of this artifact
	Severity Severity `json:"severity,omitempty"`

	// VulnerabilityTypes lists the types of vulnerabilities associated with this artifact
	VulnerabilityTypes []VulnerabilityType `json:"vulnerability_types,omitempty"`

	// TokenType specifies the type of authentication token (if applicable)
	TokenType TokenType `json:"token_type,omitempty"`

	// CloudProvider identifies the cloud service provider (for cloud resources)
	CloudProvider CloudProvider `json:"cloud_provider,omitempty"`

	// ExposureType describes how the artifact is exposed (e.g., "internal_network", "public", "connection_string")
	ExposureType string `json:"exposure_type,omitempty"`

	// IsSensitive flags whether this artifact contains sensitive information
	IsSensitive bool `json:"is_sensitive,omitempty"`
}

// NewSecurityContext creates a new SecurityContext.
func NewSecurityContext() *SecurityContext {
	return &SecurityContext{
		VulnerabilityTypes: []VulnerabilityType{},
	}
}

// WithSeverity sets the severity level.
func (sc *SecurityContext) WithSeverity(severity Severity) *SecurityContext {
	sc.Severity = severity
	return sc
}

// WithVulnerabilityType adds a vulnerability type.
func (sc *SecurityContext) WithVulnerabilityType(vulnType VulnerabilityType) *SecurityContext {
	sc.VulnerabilityTypes = append(sc.VulnerabilityTypes, vulnType)
	return sc
}

// WithTokenType sets the token type.
func (sc *SecurityContext) WithTokenType(tokenType TokenType) *SecurityContext {
	sc.TokenType = tokenType
	return sc
}

// WithCloudProvider sets the cloud provider.
func (sc *SecurityContext) WithCloudProvider(provider CloudProvider) *SecurityContext {
	sc.CloudProvider = provider
	return sc
}

// WithExposureType sets the exposure type.
func (sc *SecurityContext) WithExposureType(exposure string) *SecurityContext {
	sc.ExposureType = exposure
	return sc
}

// WithSensitive marks the artifact as sensitive.
func (sc *SecurityContext) WithSensitive(sensitive bool) *SecurityContext {
	sc.IsSensitive = sensitive
	return sc
}

// AddVulnerabilityType adds a vulnerability type if not already present.
func (sc *SecurityContext) AddVulnerabilityType(vulnType VulnerabilityType) {
	for _, v := range sc.VulnerabilityTypes {
		if v == vulnType {
			return
		}
	}
	sc.VulnerabilityTypes = append(sc.VulnerabilityTypes, vulnType)
}

// HasVulnerabilityType checks if a specific vulnerability type is present.
func (sc *SecurityContext) HasVulnerabilityType(vulnType VulnerabilityType) bool {
	for _, v := range sc.VulnerabilityTypes {
		if v == vulnType {
			return true
		}
	}
	return false
}

// IsEmpty returns true if all fields are empty/zero.
func (sc *SecurityContext) IsEmpty() bool {
	return sc.Severity == "" &&
		len(sc.VulnerabilityTypes) == 0 &&
		sc.TokenType == "" &&
		sc.CloudProvider == "" &&
		sc.ExposureType == "" &&
		!sc.IsSensitive
}
