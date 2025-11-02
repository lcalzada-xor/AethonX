// internal/platform/cveapi/types.go
package cveapi

import "time"

// EnrichedCVE represents fully enriched CVE data from external sources.
type EnrichedCVE struct {
	// Core identification
	CVEID            string
	SourceIdentifier string
	VulnStatus       string // Analyzed, Modified, Deferred, Rejected

	// Dates
	PublishedDate    time.Time
	LastModifiedDate time.Time

	// Description (multi-language support)
	Descriptions []Description

	// Classification
	CWEs []string // CWE identifiers
	CPEs []string // CPE 2.3 format

	// CVSS Metrics
	CVSSv2 *CVSSMetrics
	CVSSv3 *CVSSMetrics
	CVSSv4 *CVSSMetrics

	// References
	References []Reference

	// Metadata
	EnrichmentSource string    // nvd, circl, vulners
	EnrichedAt       time.Time
}

// Description represents a CVE description in a specific language.
type Description struct {
	Lang  string // en, es, fr, etc.
	Value string
}

// CVSSMetrics contains CVSS scoring information.
type CVSSMetrics struct {
	Version string  // 2.0, 3.0, 3.1, 4.0
	Vector  string  // Full CVSS vector string
	Score   float64 // Base score

	// Base metrics
	AttackVector         string // NETWORK, ADJACENT, LOCAL, PHYSICAL
	AttackComplexity     string // LOW, HIGH (v2: LOW, MEDIUM, HIGH)
	PrivilegesRequired   string // NONE, LOW, HIGH (v3+)
	UserInteraction      string // NONE, REQUIRED (v3+)
	Scope                string // UNCHANGED, CHANGED (v3+)
	ConfidentialityImpact string // NONE, LOW, HIGH (PARTIAL, COMPLETE in v2)
	IntegrityImpact      string // NONE, LOW, HIGH
	AvailabilityImpact   string // NONE, LOW, HIGH

	// Exploitability metrics
	ExploitabilityScore float64
	ImpactScore         float64

	// Severity (derived from score)
	Severity string // CRITICAL, HIGH, MEDIUM, LOW, NONE
}

// Reference represents an external reference link.
type Reference struct {
	URL    string
	Source string   // Source of reference (MITRE, vendor, etc.)
	Tags   []string // Patch, Exploit, Vendor Advisory, Third Party Advisory
}

// CVEProvider defines the interface that all CVE data providers must implement.
type CVEProvider interface {
	// Name returns the provider identifier (e.g., "nvd", "circl")
	Name() string

	// Enrich fetches and enriches CVE data from the provider
	Enrich(cveID string) (*EnrichedCVE, error)

	// HealthCheck verifies the provider is accessible
	HealthCheck() error

	// RateLimit returns the provider's rate limit (requests per second)
	RateLimit() float64
}
