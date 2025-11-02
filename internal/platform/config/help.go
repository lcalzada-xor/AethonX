// internal/platform/config/help.go
package config

import (
	"fmt"
	"os"
	"runtime"
)

const helpText = `AethonX - Modular Reconnaissance Engine

USAGE
  aethonx -t <domain> [options]

  Note: Use double dash (--) for long flags, single dash (-) for short flags
        Example: --target or -t (not -target)

CORE OPTIONS
  -t, --target <domain>    Target domain (required)
  -a, --active             Active reconnaissance mode (default: passive)
  -w, --workers <int>      Concurrent workers (default: 16)
  -o, --out <path>         Output directory (default: aethonx_out)
  -q, --quiet              JSON only, no visual UI

SOURCES
  --src.crtsh              Certificate Transparency logs (default: enabled)
  --src.rdap               RDAP/WHOIS queries (default: enabled)
  --src.subfinder          Multi-source subdomain discovery (default: enabled)
  --src.httpx              HTTP probing (default: enabled)
  --src.shodan             Shodan internet database (default: disabled, API key optional)
  --src.waybackurls        Wayback Machine URL history (default: enabled)

  Enable/disable with: --src.<name>=true/false

SHODAN CONFIGURATION (optional API key)
  --src.shodan.api_key <key>         Shodan API key (optional, enables more endpoints)
  --src.shodan.use_cli               Use Shodan CLI instead of API (default: false)
  --src.shodan.use_internetdb        Use free InternetDB for IP enrichment (default: true, no auth)
  --src.shodan.rate_limit <rps>      Requests per second (default: 1.0 for free tier)

  Environment variable: AETHONX_SOURCES_SHODAN_API_KEY

  Note: Works without API key using InternetDB (free IP enrichment)

ADVANCED
  -T, --timeout <sec>      Global timeout in seconds (default: 30, 0=none)
  -s, --streaming <int>    Memory threshold for disk writes (default: 1000)
  -r, --retries <int>      Max retries per source (default: 3)
  -p, --proxy <url>        HTTP/S proxy URL
      --no-ui              Disable visual UI, use plain logs
      --circuit-breaker    Enable circuit breaker (default: true)

UI OPTIONS
      --ui-mode <mode>     UI mode: pretty (default), raw

INFO
  -h, --help               Show this help
  -v, --version            Version information

EXAMPLES
  # Basic scans
  aethonx -t example.com                        # Passive scan (pretty UI)
  aethonx -t example.com -a -w 8                # Active scan, 8 workers
  aethonx -t example.com -q                     # Quiet mode (CI/CD)

  # Source management
  aethonx -t example.com --src.httpx=false      # Disable httpx source
  aethonx -t example.com --src.subfinder=false  # Disable subfinder

  # Shodan integration
  export AETHONX_SOURCES_SHODAN_API_KEY="your-api-key"
  aethonx -t example.com --src.shodan           # Enable Shodan with API
  aethonx -t example.com --src.shodan --src.shodan.use_cli  # Use CLI mode

  # UI modes
  aethonx -t example.com --ui-mode=raw          # Raw logs (for debugging)

ENVIRONMENT VARIABLES
  All flags support AETHONX_ prefix: AETHONX_TARGET, AETHONX_ACTIVE, etc.
  CLI flags override environment variables.
`

// PrintHelp prints the custom help message and exits.
func PrintHelp() {
	fmt.Fprint(os.Stdout, helpText)
	os.Exit(0)
}

// PrintVersion prints version information and exits.
func PrintVersion(version, commit, date string) {
	fmt.Printf("AethonX %s\n", version)
	fmt.Printf("  Commit:  %s\n", commit)
	fmt.Printf("  Built:   %s\n", date)
	fmt.Printf("  Go:      %s\n", getGoVersion())
	os.Exit(0)
}

func getGoVersion() string {
	return runtime.Version()
}
