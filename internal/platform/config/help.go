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
  --src.amass              OWASP Amass enumeration (default: enabled)
  --src.httpx              HTTP probing (default: enabled)

  Disable with: --src.<name>=false

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
  aethonx -t example.com                        # Passive scan (pretty UI)
  aethonx -t example.com -a -w 8                # Active scan, 8 workers
  aethonx -t example.com -q                     # Quiet mode (CI/CD)
  aethonx -t example.com --src.httpx=false      # Disable httpx source
  aethonx -t example.com --src.amass=false      # Disable amass source
  aethonx -t example.com --src.subfinder=false  # Disable subfinder
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
