// internal/platform/vulners/parser.go
package vulners

import (
	"strings"
)

// CVEInfo represents processed CVE information.
type CVEInfo struct {
	ID          string   // CVE ID (e.g., "CVE-2021-23017")
	CVSS        float64  // CVSS score (0.0-10.0)
	CVSSVector  string   // CVSS vector string
	CVSSVersion string   // CVSS version
	Severity    string   // Calculated severity: critical, high, medium, low
	Description string   // Vulnerability description
	Published   string   // Publication date
	Modified    string   // Last modification date
	CWE         []string // CWE identifiers
	References  []string // External references
	HasExploit  bool     // Whether a public exploit exists
}

// calculateSeverity determines severity level from CVSS score.
// Follows CVSS v3.x severity ratings:
// - 9.0-10.0: Critical
// - 7.0-8.9: High
// - 4.0-6.9: Medium
// - 0.1-3.9: Low
// - 0.0: None
func calculateSeverity(cvss float64) string {
	if cvss >= 9.0 {
		return "critical"
	} else if cvss >= 7.0 {
		return "high"
	} else if cvss >= 4.0 {
		return "medium"
	} else if cvss > 0.0 {
		return "low"
	}
	return "none"
}

// calculateRiskLevel determines overall risk from a list of CVEs.
// Returns the highest severity found.
func calculateRiskLevel(cves []CVEInfo) string {
	if len(cves) == 0 {
		return "none"
	}

	maxSeverity := "none"
	severityRank := map[string]int{
		"none":     0,
		"low":      1,
		"medium":   2,
		"high":     3,
		"critical": 4,
	}

	for _, cve := range cves {
		if severityRank[cve.Severity] > severityRank[maxSeverity] {
			maxSeverity = cve.Severity
		}
	}

	return maxSeverity
}

// extractCVEIDs extracts just the CVE IDs from a list of CVEInfo.
func extractCVEIDs(cves []CVEInfo) []string {
	ids := make([]string, 0, len(cves))
	for _, cve := range cves {
		ids = append(ids, cve.ID)
	}
	return ids
}

// normalizeProduct normalizes product names for better API matching.
// Examples:
// - "nginx/1.18.0" -> "nginx"
// - "Apache/2.4.41" -> "apache"
// - "MySQL Server" -> "mysql"
func normalizeProduct(product string) string {
	product = strings.TrimSpace(product)
	product = strings.ToLower(product)

	// Remove version suffixes (e.g., "nginx/1.18.0" -> "nginx")
	if idx := strings.Index(product, "/"); idx != -1 {
		product = product[:idx]
	}

	// Remove common prefixes/suffixes
	product = strings.TrimPrefix(product, "apache ")
	product = strings.TrimSuffix(product, " server")
	product = strings.TrimSuffix(product, " httpd")

	// Common product name mappings
	productMappings := map[string]string{
		"httpd":           "apache",
		"apache2":         "apache",
		"apache httpd":    "apache",
		"mysql server":    "mysql",
		"mariadb server":  "mariadb",
		"postgresql":      "postgres",
		"microsoft-iis":   "iis",
		"microsoft iis":   "iis",
		"openssh":         "openssh",
		"openssl":         "openssl",
		"redis server":    "redis",
		"mongodb server":  "mongodb",
		"tomcat":          "apache_tomcat",
		"apache tomcat":   "apache_tomcat",
	}

	if normalized, ok := productMappings[product]; ok {
		return normalized
	}

	return product
}

// normalizeVersion normalizes version strings for API queries.
// Examples:
// - "v1.18.0" -> "1.18.0"
// - "2.4.41-Ubuntu" -> "2.4.41"
// - "8.0.23 (Debian)" -> "8.0.23"
func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)

	// Remove "v" prefix
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "V")

	// Remove distribution suffixes
	// Examples: "2.4.41-Ubuntu", "8.0.23 (Debian)", "1.18.0-1ubuntu1.4"
	separators := []string{"-", " ", "(", "_"}
	for _, sep := range separators {
		if idx := strings.Index(version, sep); idx != -1 {
			// Only split if what comes after looks like distro/build info
			suffix := strings.ToLower(version[idx:])
			if containsDistroKeyword(suffix) {
				version = version[:idx]
				break
			}
		}
	}

	return strings.TrimSpace(version)
}

// containsDistroKeyword checks if a string contains distribution-related keywords.
func containsDistroKeyword(s string) bool {
	keywords := []string{
		"ubuntu", "debian", "centos", "rhel", "fedora", "alpine",
		"el7", "el8", "el9", // Red Hat Enterprise Linux versions
		"deb", "rpm",
		"build", "git", "snapshot",
	}

	s = strings.ToLower(s)
	for _, keyword := range keywords {
		if strings.Contains(s, keyword) {
			return true
		}
	}

	return false
}

// filterCriticalCVEs returns only CVEs with critical or high severity.
func filterCriticalCVEs(cves []CVEInfo) []CVEInfo {
	critical := make([]CVEInfo, 0)
	for _, cve := range cves {
		if cve.Severity == "critical" || cve.Severity == "high" {
			critical = append(critical, cve)
		}
	}
	return critical
}

// filterWithExploits returns only CVEs that have public exploits.
func filterWithExploits(cves []CVEInfo) []CVEInfo {
	withExploits := make([]CVEInfo, 0)
	for _, cve := range cves {
		if cve.HasExploit {
			withExploits = append(withExploits, cve)
		}
	}
	return withExploits
}

// summarizeCVEs creates a human-readable summary of CVEs.
func summarizeCVEs(cves []CVEInfo) string {
	if len(cves) == 0 {
		return "No vulnerabilities found"
	}

	counts := map[string]int{
		"critical": 0,
		"high":     0,
		"medium":   0,
		"low":      0,
	}

	exploitsCount := 0

	for _, cve := range cves {
		counts[cve.Severity]++
		if cve.HasExploit {
			exploitsCount++
		}
	}

	summary := ""
	if counts["critical"] > 0 {
		summary += "Critical: " + string(rune(counts["critical"]))
	}
	if counts["high"] > 0 {
		if summary != "" {
			summary += ", "
		}
		summary += "High: " + string(rune(counts["high"]))
	}
	if counts["medium"] > 0 {
		if summary != "" {
			summary += ", "
		}
		summary += "Medium: " + string(rune(counts["medium"]))
	}
	if counts["low"] > 0 {
		if summary != "" {
			summary += ", "
		}
		summary += "Low: " + string(rune(counts["low"]))
	}

	if exploitsCount > 0 {
		summary += " | Exploits: " + string(rune(exploitsCount))
	}

	return summary
}
