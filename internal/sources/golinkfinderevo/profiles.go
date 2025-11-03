// Package golinkfinderevo - Scan Profiles
// Defines different scanning profiles with varying levels of depth and resource usage.
package golinkfinderevo

import (
	"time"
)

// ScanProfile represents different scanning strategies.
type ScanProfile string

const (
	// ProfileQuick performs basic endpoint discovery without recursion.
	// Best for: Quick reconnaissance, large-scale scans, limited resources
	ProfileQuick ScanProfile = "quick"

	// ProfileStandard performs full discovery with basic recursion.
	// Best for: Standard security assessments, balanced speed/depth
	ProfileStandard ScanProfile = "standard"

	// ProfileDeep performs deep recursive crawling with all patterns.
	// Best for: Thorough security audits, penetration testing
	ProfileDeep ScanProfile = "deep"
)

// ProfileConfig defines configuration parameters for a scan profile.
type ProfileConfig struct {
	// Workers is the number of concurrent workers
	Workers int

	// Timeout is the maximum execution time per resource
	Timeout time.Duration

	// MaxRecursion is the maximum recursion depth (0 = no recursion)
	MaxRecursion int

	// GFPatterns is the list of gf patterns to execute
	GFPatterns []string

	// Description is a human-readable description of the profile
	Description string

	// MaxScriptFiles limits the number of JavaScript files to process
	MaxScriptFiles int

	// MaxHTMLFiles limits the number of HTML files to process
	MaxHTMLFiles int

	// EnableJSRendering enables headless browser JavaScript rendering
	EnableJSRendering bool
}

// Profiles maps profile names to their configurations.
var Profiles = map[ScanProfile]ProfileConfig{
	ProfileQuick: {
		Workers:           10,
		Timeout:           30 * time.Second,
		MaxRecursion:      0, // No recursion
		GFPatterns:        []string{"endpoints", "api-keys"},
		Description:       "Quick endpoint discovery without recursion (fast, low resource usage)",
		MaxScriptFiles:    25, // Reduced limit for quick scans
		MaxHTMLFiles:      25,
		EnableJSRendering: false,
	},
	ProfileStandard: {
		Workers:           20,
		Timeout:           60 * time.Second,
		MaxRecursion:      1, // 1 level recursion
		GFPatterns:        []string{"all"},
		Description:       "Standard discovery with basic recursion and all gf patterns (balanced)",
		MaxScriptFiles:    50,
		MaxHTMLFiles:      50,
		EnableJSRendering: false,
	},
	ProfileDeep: {
		Workers:           30,
		Timeout:           120 * time.Second,
		MaxRecursion:      2, // 2 levels recursion
		GFPatterns:        []string{"all"},
		Description:       "Deep recursive crawling with full pattern matching (thorough, resource-intensive)",
		MaxScriptFiles:    100, // Higher limits for deep scans
		MaxHTMLFiles:      100,
		EnableJSRendering: true, // Enable JS rendering for dynamic content
	},
}

// GetProfile returns the ProfileConfig for a given profile name.
// Returns the Standard profile if the requested profile doesn't exist.
func GetProfile(profile ScanProfile) ProfileConfig {
	if cfg, exists := Profiles[profile]; exists {
		return cfg
	}
	return Profiles[ProfileStandard]
}

// ValidateProfile checks if a profile name is valid.
func ValidateProfile(profile ScanProfile) bool {
	_, exists := Profiles[profile]
	return exists
}

// AllProfileNames returns a list of all available profile names.
func AllProfileNames() []string {
	return []string{
		string(ProfileQuick),
		string(ProfileStandard),
		string(ProfileDeep),
	}
}

// ProfileSummary returns a formatted summary of all profiles.
func ProfileSummary() map[string]string {
	summary := make(map[string]string)
	for name, config := range Profiles {
		summary[string(name)] = config.Description
	}
	return summary
}
