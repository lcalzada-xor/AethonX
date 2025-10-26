// internal/core/domain/confidence.go
package domain

// Confidence levels for artifact discovery.
// Represents how certain we are that an artifact is currently valid/active.
const (
	// ConfidenceLow indicates historical or unverified data.
	// Used for: waybackurls (historical URLs from Wayback Machine)
	ConfidenceLow float64 = 0.3

	// ConfidenceMedium indicates passive discovery without direct verification.
	// Used for: crtsh, subfinder, amass (passive mode)
	ConfidenceMedium float64 = 0.6

	// ConfidenceHigh indicates active discovery with indirect verification.
	// Used for: rdap (official WHOIS data), amass (active mode with DNS validation)
	ConfidenceHigh float64 = 0.8

	// ConfidenceVerified indicates direct verification of liveness/validity.
	// Used for: httpx (HTTP probe confirmed alive)
	ConfidenceVerified float64 = 1.0
)

// GetConfidenceLabel returns a human-readable label for a confidence value.
func GetConfidenceLabel(confidence float64) string {
	switch {
	case confidence >= ConfidenceVerified:
		return "verified"
	case confidence >= ConfidenceHigh:
		return "high"
	case confidence >= ConfidenceMedium:
		return "medium"
	case confidence >= ConfidenceLow:
		return "low"
	default:
		return "unknown"
	}
}

// ShouldUpgradeConfidence determines if confidence should be upgraded based on verification.
func ShouldUpgradeConfidence(currentConfidence float64, isVerified bool) bool {
	return isVerified && currentConfidence < ConfidenceVerified
}
