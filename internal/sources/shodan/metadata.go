// internal/sources/shodan/metadata.go
package shodan

import (
	"strconv"
	"strings"

	"aethonx/internal/core/domain/metadata"
)

// VulnerabilityMetadata contains detailed information about a security vulnerability.
type VulnerabilityMetadata struct {
	CVE           string   // CVE identifier (e.g., "CVE-2021-44228")
	Severity      string   // critical, high, medium, low
	CVSSScore     float64  // CVSS score (0.0-10.0)
	Description   string   // Vulnerability description
	References    []string // URLs to advisories
	AffectedPorts []int    // Ports where vulnerability was detected
	DiscoveryTool string   // Tool that discovered the vuln (e.g., "shodan")
}

// ToMap converts VulnerabilityMetadata to map[string]string.
func (v *VulnerabilityMetadata) ToMap() map[string]string {
	m := make(map[string]string)
	metadata.SetIfNotEmpty(m, "cve", v.CVE)
	metadata.SetIfNotEmpty(m, "severity", v.Severity)
	if v.CVSSScore > 0 {
		metadata.SetFloat(m, "cvss_score", v.CVSSScore)
	}
	metadata.SetIfNotEmpty(m, "description", v.Description)
	if len(v.References) > 0 {
		m["references"] = metadata.StringSliceToCSV(v.References)
	}
	if len(v.AffectedPorts) > 0 {
		m["affected_ports"] = metadata.IntSliceToCSV(v.AffectedPorts)
	}
	metadata.SetIfNotEmpty(m, "discovery_tool", v.DiscoveryTool)
	return m
}

// FromMap loads VulnerabilityMetadata from map[string]string.
func (v *VulnerabilityMetadata) FromMap(m map[string]string) error {
	v.CVE = metadata.GetString(m, "cve", "")
	v.Severity = metadata.GetString(m, "severity", "")
	v.CVSSScore = metadata.GetFloat(m, "cvss_score", 0.0)
	v.Description = metadata.GetString(m, "description", "")
	v.References = metadata.CSVToStringSlice(metadata.GetString(m, "references", ""))
	v.AffectedPorts = metadata.CSVToIntSlice(metadata.GetString(m, "affected_ports", ""))
	v.DiscoveryTool = metadata.GetString(m, "discovery_tool", "")
	return nil
}

// IsValid verifies if the metadata has valid minimum data.
func (v *VulnerabilityMetadata) IsValid() bool {
	return v.CVE != ""
}

// Type returns the metadata type.
func (v *VulnerabilityMetadata) Type() string {
	return "vulnerability"
}

// NewVulnerabilityMetadata creates a new VulnerabilityMetadata instance.
func NewVulnerabilityMetadata(cve string) *VulnerabilityMetadata {
	return &VulnerabilityMetadata{
		CVE:           cve,
		References:    []string{},
		AffectedPorts: []int{},
		DiscoveryTool: "shodan",
	}
}

// CloudMetadata contains information about cloud infrastructure.
type CloudMetadata struct {
	Provider string   // aws, azure, gcp, digitalocean, etc.
	Service  string   // ec2, s3, compute-engine, etc.
	Region   string   // us-east-1, westeurope, etc.
	Tags     []string // Additional tags
}

// ToMap converts CloudMetadata to map[string]string.
func (c *CloudMetadata) ToMap() map[string]string {
	m := make(map[string]string)
	metadata.SetIfNotEmpty(m, "provider", c.Provider)
	metadata.SetIfNotEmpty(m, "service", c.Service)
	metadata.SetIfNotEmpty(m, "region", c.Region)
	if len(c.Tags) > 0 {
		m["tags"] = metadata.StringSliceToCSV(c.Tags)
	}
	return m
}

// FromMap loads CloudMetadata from map[string]string.
func (c *CloudMetadata) FromMap(m map[string]string) error {
	c.Provider = metadata.GetString(m, "provider", "")
	c.Service = metadata.GetString(m, "service", "")
	c.Region = metadata.GetString(m, "region", "")
	c.Tags = metadata.CSVToStringSlice(metadata.GetString(m, "tags", ""))
	return nil
}

// IsValid verifies if the metadata has valid minimum data.
func (c *CloudMetadata) IsValid() bool {
	return c.Provider != ""
}

// Type returns the metadata type.
func (c *CloudMetadata) Type() string {
	return "cloud"
}

// NewCloudMetadata creates a new CloudMetadata instance.
func NewCloudMetadata(provider string) *CloudMetadata {
	return &CloudMetadata{
		Provider: provider,
		Tags:     []string{},
	}
}

// InferSeverityFromCVE attempts to extract severity from CVE identifier.
// This is a best-effort heuristic until we integrate with CVE databases.
func InferSeverityFromCVE(cve string) string {
	// Common patterns: CVE-YEAR-NUMBER
	// High-profile CVEs (known critical vulnerabilities)
	criticalCVEs := map[string]bool{
		"CVE-2021-44228": true, // Log4Shell
		"CVE-2021-45046": true, // Log4Shell bypass
		"CVE-2014-0160":  true, // Heartbleed
		"CVE-2017-5638":  true, // Apache Struts
		"CVE-2017-0144":  true, // EternalBlue
		"CVE-2020-1472":  true, // Zerologon
		"CVE-2021-26855": true, // Exchange ProxyLogon
	}

	if criticalCVEs[cve] {
		return "critical"
	}

	// Default to unknown - should be enriched from CVE API
	return "unknown"
}

// NormalizeSeverity converts various severity formats to standard levels.
func NormalizeSeverity(severity string) string {
	severity = strings.ToLower(strings.TrimSpace(severity))

	switch severity {
	case "critical", "crit":
		return "critical"
	case "high":
		return "high"
	case "medium", "moderate", "med":
		return "medium"
	case "low":
		return "low"
	case "info", "informational", "none":
		return "info"
	default:
		return "unknown"
	}
}

// CVSSScoreToSeverity converts CVSS score to severity level.
func CVSSScoreToSeverity(score float64) string {
	switch {
	case score >= 9.0:
		return "critical"
	case score >= 7.0:
		return "high"
	case score >= 4.0:
		return "medium"
	case score >= 0.1:
		return "low"
	default:
		return "info"
	}
}

// ParseCVSSScore safely parses CVSS score from string.
func ParseCVSSScore(scoreStr string) float64 {
	score, err := strconv.ParseFloat(scoreStr, 64)
	if err != nil {
		return 0.0
	}
	// Clamp to valid range (0.0-10.0)
	if score < 0.0 {
		return 0.0
	}
	if score > 10.0 {
		return 10.0
	}
	return score
}
