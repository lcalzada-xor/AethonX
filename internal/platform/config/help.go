// internal/platform/config/help.go
package config

import (
	"fmt"
	"os"
	"runtime"
)

const helpText = `
AethonX - Modular Reconnaissance Engine

USAGE:
  aethonx -t <domain> [options]

IMPORTANT:
  Use double dash (--) for long flag names: --target, --workers, --active
  Use single dash (-) for short flags: -t, -w, -a

  ❌ WRONG:  aethonx -target example.com
  ✓  RIGHT:  aethonx --target example.com
  ✓  RIGHT:  aethonx -t example.com

CORE OPTIONS:
  -t, --target string      Target domain (required, e.g., example.com)
  -a, --active             Enable active reconnaissance mode (default: false)
  -w, --workers int        Number of concurrent workers (default: 4)
  -T, --timeout int        Global timeout in seconds, 0=no timeout (default: 30)
  -o, --out string         Output directory (default: "aethonx_out")

SOURCE OPTIONS:
  --src.crtsh                  Enable crt.sh certificate transparency source (default: true)
  --src.crtsh.priority int     Set crt.sh priority (default: 10)

  --src.rdap                   Enable RDAP WHOIS source (default: true)
  --src.rdap.priority int      Set RDAP priority (default: 8)

  --src.httpx                  Enable httpx active probing source (default: true)
  --src.httpx.priority int     Set httpx priority (default: 15)

OUTPUT OPTIONS:
  -q, --quiet              Disable table output, JSON only (default: false)

STREAMING OPTIONS:
  -s, --streaming int      Artifact threshold for partial disk writes (default: 1000)
                           Higher values = more memory usage, fewer disk writes

RESILIENCE OPTIONS:
  -r, --retries int        Max retries per source on failure (default: 3)
  --circuit-breaker        Enable circuit breaker for failing sources (default: true)

NETWORK OPTIONS:
  -p, --proxy string       HTTP(S) proxy URL for outbound requests (optional)

INFO:
  -v, --version            Print version information and exit
  -h, --help               Show this help message

EXAMPLES:
  Basic passive scan:
    aethonx -t example.com

  Active scan with custom workers:
    aethonx -t example.com -a -w 8

  Quiet mode (JSON output only):
    aethonx -t example.com -q

  High-volume target with streaming tuning:
    aethonx -t example.com -s 500 -w 8 -T 120

  Using a proxy:
    aethonx -t example.com -p http://proxy.example.com:8080

  Disable specific sources:
    aethonx -t example.com --src.crtsh=false --src.rdap=false

ENVIRONMENT VARIABLES:
  Most flags can be set via environment variables with AETHONX_ prefix:

  AETHONX_TARGET                    Target domain
  AETHONX_ACTIVE=true               Enable active mode
  AETHONX_WORKERS=8                 Number of workers
  AETHONX_TIMEOUT=60                Timeout in seconds
  AETHONX_OUTPUT_DIR=/path          Output directory
  AETHONX_STREAMING_THRESHOLD=500   Streaming threshold
  AETHONX_RESILIENCE_MAX_RETRIES=5  Max retries
  AETHONX_PROXY_URL=http://...      Proxy URL

  Source-specific (replace CRTSH with source name):
  AETHONX_SOURCES_CRTSH_ENABLED=false
  AETHONX_SOURCES_CRTSH_PRIORITY=20

  Note: CLI flags override environment variables.

SCAN MODES:
  Passive Mode (default):
    - No direct contact with target infrastructure
    - Queries public data sources (crt.sh, RDAP, etc.)
    - Safe for stealth reconnaissance

  Active Mode (-a, --active):
    - Direct probing of target infrastructure
    - HTTP requests, port scanning, service detection
    - May trigger IDS/IPS alerts and logging

OUTPUT:
  AethonX generates JSON output in the specified directory:
  - Full scan results with artifacts, metadata, and relationships
  - Partial streaming files for large datasets (auto-cleanup)
  - Table output to stdout (unless --quiet)

For more information and documentation:
  https://github.com/yourusername/aethonx
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
