// parser.go - Parses AbuseIPDB API responses into IPMetadata enrichment
package abuseipdb

import (
	domainmetadata "aethonx/internal/core/domain/metadata"
)

// Parser handles parsing of AbuseIPDB responses into domain metadata.
type Parser struct{}

// NewParser creates a new AbuseIPDB response parser.
func NewParser() *Parser {
	return &Parser{}
}

// ParseToIPMetadata enriches IPMetadata with AbuseIPDB reputation data.
// It adds threat intelligence, reputation scoring, and abuse reporting data.
func (p *Parser) ParseToIPMetadata(resp *AbuseIPDBResponse) *domainmetadata.IPMetadata {
	ipMeta := domainmetadata.NewIPMetadata()

	data := resp.Data

	// Geolocation data (if not already populated)
	if ipMeta.Country == "" {
		ipMeta.Country = data.CountryName
	}
	if ipMeta.CountryCode == "" {
		ipMeta.CountryCode = data.CountryCode
	}

	// ISP/Network data (if not already populated)
	if ipMeta.ISP == "" {
		ipMeta.ISP = data.ISP
	}

	// Threat Intelligence - Abuse Confidence Score (0-100)
	// Interpretation:
	// - 0-25: Low risk (mostly clean)
	// - 26-50: Medium risk (some reports)
	// - 51-75: High risk (frequent reports)
	// - 76-100: Critical risk (very dangerous)
	ipMeta.ThreatScore = float64(data.AbuseConfidenceScore)

	// Reputation categories based on abuse confidence score
	ipMeta.Reputation = calculateReputation(data.AbuseConfidenceScore)

	// Blacklist status (using Blacklisted field)
	ipMeta.Blacklisted = data.AbuseConfidenceScore >= 50 && !data.IsWhitelisted

	// Usage type classification (Data Center, ISP, etc.)
	if data.UsageType != "" {
		ipMeta.IPType = normalizeUsageType(data.UsageType)
	}

	// Hostnames associated with IP (store as PTRRecords)
	if len(data.Hostnames) > 0 {
		ipMeta.PTRRecords = data.Hostnames
	}

	// Note: AbuseIPDB-specific fields (AbuseReports, LastReportedAt, IsWhitelisted)
	// are not stored in IPMetadata as they're too specific.
	// This data is available via custom fields in the artifact if needed.

	return ipMeta
}

// EnrichExistingMetadata enriches an existing IPMetadata with AbuseIPDB data.
// This is useful when combining data from multiple sources (e.g., ipapi + abuseipdb).
func (p *Parser) EnrichExistingMetadata(existing *domainmetadata.IPMetadata, resp *AbuseIPDBResponse) {
	data := resp.Data

	// Only override if empty (preserve existing data)
	if existing.Country == "" && data.CountryName != "" {
		existing.Country = data.CountryName
	}
	if existing.CountryCode == "" && data.CountryCode != "" {
		existing.CountryCode = data.CountryCode
	}
	if existing.ISP == "" && data.ISP != "" {
		existing.ISP = data.ISP
	}

	// Always add threat intelligence (this is AbuseIPDB's specialty)
	existing.ThreatScore = float64(data.AbuseConfidenceScore)
	existing.Reputation = calculateReputation(data.AbuseConfidenceScore)
	existing.Blacklisted = data.AbuseConfidenceScore >= 50 && !data.IsWhitelisted

	// Merge hostnames as PTRRecords (avoid duplicates)
	if len(data.Hostnames) > 0 {
		hostnameSet := make(map[string]bool)
		for _, h := range existing.PTRRecords {
			hostnameSet[h] = true
		}
		for _, h := range data.Hostnames {
			if !hostnameSet[h] {
				existing.PTRRecords = append(existing.PTRRecords, h)
			}
		}
	}

	// Usage type (if not already set)
	if existing.IPType == "" && data.UsageType != "" {
		existing.IPType = normalizeUsageType(data.UsageType)
	}
}

// calculateReputation determines reputation category based on abuse confidence score.
func calculateReputation(score int) string {
	switch {
	case score >= 76:
		return "critical" // Very dangerous IP
	case score >= 51:
		return "high" // High risk IP
	case score >= 26:
		return "medium" // Some abuse reports
	case score >= 1:
		return "low" // Few reports
	default:
		return "clean" // No abuse reports
	}
}

// normalizeUsageType maps AbuseIPDB usage types to standard IPType values.
func normalizeUsageType(usageType string) string {
	switch usageType {
	case "Data Center/Web Hosting/Transit":
		return "hosting"
	case "Commercial":
		return "business"
	case "Reserved":
		return "reserved"
	case "Content Delivery Network":
		return "cdn"
	case "Fixed Line ISP":
		return "residential"
	case "Mobile ISP":
		return "mobile"
	case "University/College/School":
		return "education"
	case "Library":
		return "education"
	default:
		return usageType // Return as-is if unknown
	}
}
