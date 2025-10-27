package httpx

// ScanProfile defines different scanning strategies for httpx.
type ScanProfile string

const (
	// ProfileBasic performs lightweight host verification with essential metadata.
	ProfileBasic ScanProfile = "basic"

	// ProfileTech performs technology detection and fingerprinting.
	ProfileTech ScanProfile = "tech"

	// ProfileTLS focuses on TLS/SSL certificate analysis.
	ProfileTLS ScanProfile = "tls"

	// ProfileFull combines all probes for comprehensive reconnaissance.
	ProfileFull ScanProfile = "full"

	// ProfileHeadless enables screenshot capture (requires Chrome).
	ProfileHeadless ScanProfile = "headless"

	// ProfileVerification performs ultra-fast liveness verification.
	// Used for mass URL checking (waybackurls artifacts).
	ProfileVerification ScanProfile = "verification"
)

// ProfileConfig defines the httpx flags and metadata for a scan profile.
type ProfileConfig struct {
	Flags       []string
	Description string
	Weight      int // Task weight for worker pool scheduling (0-100)
}

// Profiles maps each ScanProfile to its configuration.
var Profiles = map[ScanProfile]ProfileConfig{
	ProfileBasic: {
		Flags: []string{
			"-sc",     // Status code
			"-title",  // Page title
			"-cl",     // Content length
			"-ct",     // Content type
			"-server", // Web server
			"-rt",     // Response time
			"-ip",     // IP address
			"-cdn",    // CDN detection
			"-method", // HTTP method
			"-probe",  // Probe status
		},
		Description: "Basic host verification with essential metadata",
		Weight:      40,
	},

	ProfileTech: {
		Flags: []string{
			"-sc", "-title", "-cl", "-ct",
			"-td",           // Tech detection (Wappalyzer)
			"-server",       // Web server
			"-jarm",         // JARM fingerprint
			"-hash", "sha256", // Body hash
			"-favicon",      // Favicon hash (MMH3)
			"-ip", "-cname",
			"-asn",          // ASN information
			"-cdn",
		},
		Description: "Technology detection and advanced fingerprinting",
		Weight:      70,
	},

	ProfileTLS: {
		Flags: []string{
			"-sc", "-title",
			"-tls-probe",     // Probe TLS
			"-tls-grab",      // Grab certificates
			"-asn",
			"-cdn",
			"-ip", "-cname",
			"-include-chain", // Redirect chain
		},
		Description: "TLS/SSL certificate analysis",
		Weight:      50,
	},

	ProfileFull: {
		Flags: []string{
			"-sc", "-title", "-cl", "-ct", "-server", "-rt", "-method",
			"-td",           // Tech detection
			"-jarm",         // JARM fingerprint
			"-favicon",
			"-hash", "sha256",
			"-tls-probe", "-tls-grab",
			"-ip", "-cname", "-asn", "-cdn",
			"-include-chain",
			"-extract-fqdn", // Extract FQDNs from response
			"-websocket",    // Websocket detection
			"-pipeline",     // HTTP pipeline support
			"-http2",        // HTTP/2 support
		},
		Description: "Comprehensive scan with all probes enabled",
		Weight:      90,
	},

	ProfileHeadless: {
		Flags: []string{
			"-sc", "-title",
			"-ss",                    // Screenshot
			"-system-chrome",         // Use local Chrome
			"-esb",                   // Exclude screenshot bytes from JSON
			"-screenshot-timeout", "15s",
			"-screenshot-idle", "2s",
		},
		Description: "Visual reconnaissance with headless browser (requires Chrome)",
		Weight:      100,
	},

	ProfileVerification: {
		Flags: []string{
			"-sc",                     // Status code (essential)
			"-silent",                 // No console output
			"-no-color",               // No ANSI colors
			"-timeout", "3",           // 3 second timeout per request
			"-retries", "1",           // Only 1 retry
			"-follow-redirects",       // Follow HTTP redirects
			"-max-redirects", "2",     // Max 2 redirects
		},
		Description: "Ultra-fast liveness verification for mass URL checking",
		Weight:      20, // Lowest weight (fastest)
	},
}

// GetProfile returns the ProfileConfig for a given ScanProfile.
// Returns ProfileBasic if the profile doesn't exist.
func GetProfile(profile ScanProfile) ProfileConfig {
	if cfg, exists := Profiles[profile]; exists {
		return cfg
	}
	return Profiles[ProfileBasic]
}
