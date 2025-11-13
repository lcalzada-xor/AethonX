// internal/platform/config/config.go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"aethonx/internal/core/ports"

	"github.com/joho/godotenv"
	"github.com/spf13/pflag"
)

// Config is the main configuration structure organized by functional categories.
type Config struct {
	Core           CoreConfig
	Source         SourceConfig
	Output         OutputConfig
	Streaming      StreamingConfig
	Resilience     ResilienceConfig
	Network        NetworkConfig
	Enrichment     EnrichmentConfig
	VulnEnrichment VulnEnrichmentConfig
}

// CoreConfig contains fundamental scan parameters.
type CoreConfig struct {
	Target        string // Target domain (required)
	Active        bool   // Enable active reconnaissance mode
	Workers       int    // Number of concurrent workers
	TimeoutS      int    // Global timeout in seconds (0 = no timeout)
	StageTimeoutS int    // Per-stage timeout in seconds (0 = no per-stage limit, only global timeout applies)
}

// SourceConfig contains source-specific configurations.
type SourceConfig struct {
	// Dynamic map of source configurations
	// Key = source name (e.g., "crtsh", "rdap", "httpx")
	// Value = source-specific configuration
	Sources map[string]ports.SourceConfig
}

// OutputConfig contains output-related settings.
type OutputConfig struct {
	Dir         string // Output directory
	UIMode      string // UI mode: pretty (default), raw
	LogFormat   string // Log format for raw mode: text (default), json
	ShowMetrics bool   // Show system metrics (CPU, memory, etc.)
	ShowPhases  bool   // Show execution phases for each source
}

// StreamingConfig contains memory management settings.
type StreamingConfig struct {
	ArtifactThreshold int // Artifact count threshold for partial disk writes
}

// ResilienceConfig contains fault tolerance settings.
type ResilienceConfig struct {
	// Retry configuration
	MaxRetries        int           // Max retries per source
	BackoffBase       time.Duration // Base backoff duration (e.g., 1s)
	BackoffMultiplier float64       // Multiplier for exponential backoff (e.g., 2.0)

	// Circuit Breaker configuration
	CircuitBreakerEnabled     bool          // Enable circuit breaker
	CircuitBreakerThreshold   int           // Failures before opening circuit
	CircuitBreakerTimeout     time.Duration // How long circuit stays open
	CircuitBreakerHalfOpenMax int           // Max requests in half-open state
}

// NetworkConfig contains network-related settings.
type NetworkConfig struct {
	ProxyURL string // HTTP(S) proxy URL for outbound requests
}

// EnrichmentConfig contains CVE enrichment settings.
type EnrichmentConfig struct {
	Enabled       bool          // Enable CVE enrichment
	Provider      string        // Primary provider: nvd, circl
	NVDAPIKey     string        // NVD API key (optional, increases rate limit)
	CacheTTL      time.Duration // Cache TTL for enriched CVEs
	Timeout       time.Duration // Timeout per CVE enrichment request
	MaxConcurrent int           // Max concurrent enrichment requests
}

// VulnEnrichmentConfig contains Vulners.com service vulnerability enrichment settings.
type VulnEnrichmentConfig struct {
	Enabled       bool          // Enable Vulners vulnerability enrichment for services
	APIKey        string        // Vulners API key (optional, increases rate limit)
	Timeout       time.Duration // Timeout per service enrichment request
	MaxConcurrent int           // Max concurrent enrichment requests
}

// DefaultConfig returns a default configuration organized by categories.
func DefaultConfig() Config {
	return Config{
		Core: CoreConfig{
			Target:        "",
			Active:        false,
			Workers:       16,
			TimeoutS:      30,
			StageTimeoutS: 0, // 0 = no per-stage limit, only global timeout applies
		},

		Source: SourceConfig{
			Sources: map[string]ports.SourceConfig{
				"crtsh": {
					Enabled:   true,
					Timeout:   30 * time.Second,
					Retries:   2,
					RateLimit: 0,
					Priority:  10,
					Custom:    make(map[string]interface{}),
				},
				"rdap": {
					Enabled:   true,
					Timeout:   30 * time.Second,
					Retries:   2,
					RateLimit: 0,
					Priority:  8,
					Custom:    make(map[string]interface{}),
				},
				"subfinder": {
					Enabled:   true,
					Timeout:   200 * time.Second, // subfinder with all sources
					Retries:   2,
					RateLimit: 0,  // Managed internally by subfinder
					Priority:  10, // High priority - passive discovery
					Custom: map[string]interface{}{
						"all_sources": true,
						"sources":     []string{},
						"threads":     10,
						"rate_limit":  0,
						"exec_path":   "subfinder",
					},
				},
				"dnsx": {
					Enabled:   true,
					Timeout:   60 * time.Second,
					Retries:   2,
					RateLimit: 0,
					Priority:  15, // Stage 2: Same as httpx, run in parallel
					Custom: map[string]interface{}{
						"threads":         100,
						"rate_limit":      -1, // unlimited
						"query_types":     []string{"a", "aaaa", "cname", "mx", "txt", "ns", "soa"},
						"enable_cdn":      true,
						"enable_asn":      true,
						"wildcard_filter": true,
						"exec_path":       "dnsx",
					},
				},
				"httpx": {
					Enabled:   true,
					Timeout:   120 * time.Second, // httpx can be slow with tech detection
					Retries:   2,
					RateLimit: 0,
					Priority:  15, // High priority after passive sources
					Custom: map[string]interface{}{
						"profile":    "full",
						"threads":    75,
						"rate_limit": 150,
						"exec_path":  "httpx",
					},
				},
				"waybackurls": {
					Enabled:   true,
					Timeout:   120 * time.Second, // Wayback Machine can be slow
					Retries:   2,
					RateLimit: 0,
					Priority:  5, // High priority (passive discovery, early execution)
					Custom: map[string]interface{}{
						"with_dates": false,
						"no_subs":    false,
						"exec_path":  "waybackurls",
					},
				},
				"shodan": {
					Enabled:   false, // Disabled by default (requires API key)
					Timeout:   60 * time.Second,
					Retries:   2,
					RateLimit: 1.0, // 1 req/s (free tier)
					Priority:  12,  // After crtsh (10), before subfinder (20)
					Custom: map[string]interface{}{
						"api_key":    "",    // Must be set via env or flag
						"use_cli":    false, // Use API by default
						"rate_limit": 1.0,   // Requests per second
					},
				},
				"golinkfinderevo": {
					Enabled:   true,
					Timeout:   60 * time.Second, // Standard profile timeout
					Retries:   2,
					RateLimit: 0,  // Managed by worker pool
					Priority:  20, // Stage 3 (Crawl) - after httpx
					Custom: map[string]interface{}{
						"profile":           "standard",                         // quick, standard, deep
						"workers":           20,                                 // Concurrent workers
						"max_script_files":  50,                                 // Max JS files to analyze
						"max_html_files":    50,                                 // Max HTML files to analyze
						"exec_path":         "golinkfinder",                     // Binary path
						"gf_templates_path": "./internal/platform/gf_templates", // GF templates directory
						"gf_patterns":       []string{"all"},                    // GF patterns to apply
					},
				},
				"whatweb": {
					Enabled:   true,
					Timeout:   120 * time.Second, // Web fingerprinting can be slow
					Retries:   2,
					RateLimit: 0,  // Managed by threads
					Priority:  16, // Stage 2 - after discovery, before crawl
					Custom: map[string]interface{}{
						"exec_path":  "whatweb",
						"aggression": 1,  // 1-4, stealthy by default
						"threads":    25, // Concurrent scans
						"user_agent": "AethonX/1.0 (Web reconnaissance tool)",
					},
				},
				"ipapi": {
					Enabled:   true,
					Timeout:   30 * time.Second, // IP geolocation lookups
					Retries:   2,
					RateLimit: 0, // Rate limiting handled internally by client (45 req/min free tier)
					Priority:  18,  // Stage 2 - enrichment after httpx/dnsx (15), before golinkfinder (20)
					Custom:    make(map[string]interface{}),
				},
				"abuseipdb": {
					Enabled:   false, // Disabled by default (requires API key)
					Timeout:   10 * time.Second, // IP reputation checks are fast
					Retries:   2,
					RateLimit: 0, // Rate limiting handled internally by client (1000 req/day free tier ~0.01 req/s)
					Priority:  19,  // Stage 2 - enrichment after ipapi (18), before golinkfinder (20)
					Custom: map[string]interface{}{
						"api_key": "", // REQUIRED: Get free API key at https://www.abuseipdb.com/api
					},
				},
				"retirejs": {
					Enabled:   true,
					Timeout:   180 * time.Second, // retire.js can take time scanning large JS files
					Retries:   2,
					RateLimit: 0,  // No rate limiting needed for local analysis
					Priority:  25, // Stage 4 (CVE Assessment) - after golinkfinderevo (20)
					Custom: map[string]interface{}{
						"exec_path":         "retire",
						"severity":          "medium",   // none, low, medium, high, critical
						"exit_code":         0,          // Exit code to use (0 = don't fail on vulns)
						"deep":              false,      // Deep scan mode
						"include_osv":       false,      // Include OSV advisories
						"max_files":         100,        // Max JS files to analyze
						"extensions":        []string{"js"}, // File extensions to scan
						"cache_dir":         "",         // Cache directory (empty = default)
						"no_cache":          false,      // Disable cache
						"download_timeout":  10 * time.Second, // Timeout for downloading JS files (fallback mode)
						"prefer_local":      true,       // Prefer local files from golinkfinderevo
						"fallback_download": true,       // Fallback to download if no local files
					},
				},
			},
		},

		Output: OutputConfig{
			Dir:         "aethonx_out",
			UIMode:      "pretty",
			LogFormat:   "text",
			ShowMetrics: false,
			ShowPhases:  false,
		},

		Streaming: StreamingConfig{
			ArtifactThreshold: 1000,
		},

		Resilience: ResilienceConfig{
			MaxRetries:                3,
			BackoffBase:               1 * time.Second,
			BackoffMultiplier:         2.0,
			CircuitBreakerEnabled:     true,
			CircuitBreakerThreshold:   5,
			CircuitBreakerTimeout:     60 * time.Second,
			CircuitBreakerHalfOpenMax: 3,
		},

		Network: NetworkConfig{
			ProxyURL: "",
		},

		Enrichment: EnrichmentConfig{
			Enabled:       true,
			Provider:      "nvd",
			NVDAPIKey:     "",
			CacheTTL:      168 * time.Hour, // 7 days
			Timeout:       10 * time.Second,
			MaxConcurrent: 5,
		},

		VulnEnrichment: VulnEnrichmentConfig{
			Enabled:       false, // Disabled by default (API has 1000 req/day limit)
			APIKey:        "",    // Optional, increases rate limit
			Timeout:       10 * time.Second,
			MaxConcurrent: 5,
		},
	}
}

// Load initializes configuration: .env -> ENV -> defaults, then FLAGS (flags take priority).
// Priority: FLAGS > ENV > .env > defaults
func Load(version, commit, date string) (Config, error) {
	cfg := DefaultConfig()

	// Load .env file if it exists (silently ignore if not found)
	// This allows users to optionally use .env files for configuration
	_ = godotenv.Load() // Ignore error - .env is optional

	// Load from ENV (includes variables from .env)
	loadFromEnv(&cfg)

	// Parse flags (overrides ENV)
	loadFromFlags(&cfg, version, commit, date)

	// Normalize
	normalize(&cfg)

	return cfg, nil
}

// loadFromEnv loads configuration from environment variables.
func loadFromEnv(cfg *Config) {
	// === CORE CONFIG ===
	if v := getenv("AETHONX_TARGET", ""); v != "" {
		cfg.Core.Target = v
	}
	if v := getenv("AETHONX_ACTIVE", ""); v != "" {
		cfg.Core.Active = parseBool(v)
	}
	if v := getenv("AETHONX_WORKERS", ""); v != "" {
		cfg.Core.Workers = parseInt(v, cfg.Core.Workers)
	}
	if v := getenv("AETHONX_TIMEOUT", ""); v != "" {
		cfg.Core.TimeoutS = parseInt(v, cfg.Core.TimeoutS)
	}

	// === OUTPUT CONFIG ===
	if v := getenv("AETHONX_OUTPUT_DIR", ""); v != "" {
		cfg.Output.Dir = v
	}
	if v := getenv("AETHONX_UI_MODE", ""); v != "" {
		cfg.Output.UIMode = v
	}
	if v := getenv("AETHONX_LOG_FORMAT", ""); v != "" {
		cfg.Output.LogFormat = v
	}
	if v := getenv("AETHONX_SHOW_METRICS", ""); v != "" {
		cfg.Output.ShowMetrics = parseBool(v)
	}
	if v := getenv("AETHONX_SHOW_PHASES", ""); v != "" {
		cfg.Output.ShowPhases = parseBool(v)
	}

	// === NETWORK CONFIG ===
	if v := getenv("AETHONX_PROXY_URL", ""); v != "" {
		cfg.Network.ProxyURL = v
	}

	// === ENRICHMENT CONFIG ===
	if v := getenv("AETHONX_ENRICHMENT_ENABLED", ""); v != "" {
		cfg.Enrichment.Enabled = parseBool(v)
	}
	if v := getenv("AETHONX_ENRICHMENT_PROVIDER", ""); v != "" {
		cfg.Enrichment.Provider = v
	}
	if v := getenv("AETHONX_ENRICHMENT_NVD_API_KEY", ""); v != "" {
		cfg.Enrichment.NVDAPIKey = v
	}
	if v := getenv("AETHONX_ENRICHMENT_CACHE_TTL", ""); v != "" {
		if duration, err := time.ParseDuration(v); err == nil {
			cfg.Enrichment.CacheTTL = duration
		}
	}
	if v := getenv("AETHONX_ENRICHMENT_TIMEOUT", ""); v != "" {
		if duration, err := time.ParseDuration(v); err == nil {
			cfg.Enrichment.Timeout = duration
		}
	}
	if v := getenv("AETHONX_ENRICHMENT_MAX_CONCURRENT", ""); v != "" {
		cfg.Enrichment.MaxConcurrent = parseInt(v, cfg.Enrichment.MaxConcurrent)
	}

	// === VULN ENRICHMENT CONFIG ===
	if v := getenv("AETHONX_VULN_ENRICHMENT_ENABLED", ""); v != "" {
		cfg.VulnEnrichment.Enabled = parseBool(v)
	}
	if v := getenv("AETHONX_VULN_ENRICHMENT_API_KEY", ""); v != "" {
		cfg.VulnEnrichment.APIKey = v
	}
	if v := getenv("AETHONX_VULN_ENRICHMENT_TIMEOUT", ""); v != "" {
		if duration, err := time.ParseDuration(v); err == nil {
			cfg.VulnEnrichment.Timeout = duration
		}
	}
	if v := getenv("AETHONX_VULN_ENRICHMENT_MAX_CONCURRENT", ""); v != "" {
		cfg.VulnEnrichment.MaxConcurrent = parseInt(v, cfg.VulnEnrichment.MaxConcurrent)
	}

	// === SOURCE CONFIG ===
	// Format: AETHONX_SOURCES_CRTSH_ENABLED=true
	//         AETHONX_SOURCES_CRTSH_PRIORITY=10
	//         AETHONX_SOURCES_CRTSH_TIMEOUT=60
	for name := range cfg.Source.Sources {
		prefix := fmt.Sprintf("AETHONX_SOURCES_%s_", strings.ToUpper(name))

		sourceCfg := cfg.Source.Sources[name]

		if v := getenv(prefix+"ENABLED", ""); v != "" {
			sourceCfg.Enabled = parseBool(v)
		}
		if v := getenv(prefix+"PRIORITY", ""); v != "" {
			sourceCfg.Priority = parseInt(v, sourceCfg.Priority)
		}
		if v := getenv(prefix+"TIMEOUT", ""); v != "" {
			sourceCfg.Timeout = time.Duration(parseInt(v, int(sourceCfg.Timeout.Seconds()))) * time.Second
		}
		if v := getenv(prefix+"RETRIES", ""); v != "" {
			sourceCfg.Retries = parseInt(v, sourceCfg.Retries)
		}
		if v := getenv(prefix+"RATELIMIT", ""); v != "" {
			sourceCfg.RateLimit = parseInt(v, sourceCfg.RateLimit)
		}

		// HTTPx-specific custom config
		if name == "httpx" {
			if v := getenv(prefix+"PROFILE", ""); v != "" {
				sourceCfg.Custom["profile"] = v
			}
			if v := getenv(prefix+"THREADS", ""); v != "" {
				sourceCfg.Custom["threads"] = parseInt(v, 75)
			}
			if v := getenv(prefix+"RATE_LIMIT", ""); v != "" {
				sourceCfg.Custom["rate_limit"] = parseInt(v, 150)
			}
			if v := getenv(prefix+"EXEC_PATH", ""); v != "" {
				sourceCfg.Custom["exec_path"] = v
			}
		}

		// Subfinder-specific custom config
		if name == "subfinder" {
			if v := getenv(prefix+"ALL_SOURCES", ""); v != "" {
				sourceCfg.Custom["all_sources"] = parseBool(v)
			}
			if v := getenv(prefix+"THREADS", ""); v != "" {
				sourceCfg.Custom["threads"] = parseInt(v, 10)
			}
			if v := getenv(prefix+"RATE_LIMIT", ""); v != "" {
				sourceCfg.Custom["rate_limit"] = parseInt(v, 0)
			}
			if v := getenv(prefix+"EXEC_PATH", ""); v != "" {
				sourceCfg.Custom["exec_path"] = v
			}
		}

		// Shodan-specific custom config
		if name == "shodan" {
			// Try both AETHONX_SOURCES_SHODAN_API_KEY and AETHONX_SRC_SHODAN_API_KEY
			// (backward compatibility with documentation examples)
			apiKey := getenv(prefix+"API_KEY", "")
			if apiKey == "" {
				apiKey = getenv("AETHONX_SRC_SHODAN_API_KEY", "")
			}
			if apiKey != "" {
				sourceCfg.Custom["api_key"] = apiKey
			}

			if v := getenv(prefix+"USE_CLI", ""); v != "" {
				sourceCfg.Custom["use_cli"] = parseBool(v)
			}
			if v := getenv(prefix+"RATE_LIMIT", ""); v != "" {
				// Parse as float for Shodan rate limit
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					sourceCfg.Custom["rate_limit"] = f
				}
			}
		}

		// AbuseIPDB-specific custom config
		if name == "abuseipdb" {
			// Support both formats: AETHONX_SOURCES_ABUSEIPDB_API_KEY and AETHONX_SRC_ABUSEIPDB_API_KEY
			apiKey := getenv(prefix+"API_KEY", "")
			if apiKey == "" {
				apiKey = getenv("AETHONX_SRC_ABUSEIPDB_API_KEY", "")
			}
			if apiKey != "" {
				sourceCfg.Custom["api_key"] = apiKey
			}
		}

		// WhatWeb-specific custom config
		if name == "whatweb" {
			if v := getenv(prefix+"EXEC_PATH", ""); v != "" {
				sourceCfg.Custom["exec_path"] = v
			}
			if v := getenv(prefix+"AGGRESSION", ""); v != "" {
				sourceCfg.Custom["aggression"] = parseInt(v, 1)
			}
			if v := getenv(prefix+"THREADS", ""); v != "" {
				sourceCfg.Custom["threads"] = parseInt(v, 25)
			}
			if v := getenv(prefix+"USER_AGENT", ""); v != "" {
				sourceCfg.Custom["user_agent"] = v
			}
		}

		cfg.Source.Sources[name] = sourceCfg
	}

	// === STREAMING CONFIG ===
	if v := getenv("AETHONX_STREAMING_THRESHOLD", ""); v != "" {
		cfg.Streaming.ArtifactThreshold = parseInt(v, cfg.Streaming.ArtifactThreshold)
	}

	// === RESILIENCE CONFIG ===
	if v := getenv("AETHONX_RESILIENCE_MAX_RETRIES", ""); v != "" {
		cfg.Resilience.MaxRetries = parseInt(v, cfg.Resilience.MaxRetries)
	}
	if v := getenv("AETHONX_RESILIENCE_BACKOFF_BASE", ""); v != "" {
		cfg.Resilience.BackoffBase = time.Duration(parseInt(v, int(cfg.Resilience.BackoffBase.Seconds()))) * time.Second
	}
	if v := getenv("AETHONX_RESILIENCE_CB_ENABLED", ""); v != "" {
		cfg.Resilience.CircuitBreakerEnabled = parseBool(v)
	}
	if v := getenv("AETHONX_RESILIENCE_CB_THRESHOLD", ""); v != "" {
		cfg.Resilience.CircuitBreakerThreshold = parseInt(v, cfg.Resilience.CircuitBreakerThreshold)
	}
}

// loadFromFlags parses CLI flags with pflag (supports short aliases and categories).
func loadFromFlags(cfg *Config, version, commit, date string) {
	// Custom help flag handling
	showHelp := pflag.BoolP("help", "h", false, "Show help message")
	showVersion := pflag.BoolP("version", "v", false, "Print version information")

	// === CORE FLAGS ===
	pflag.StringVarP(&cfg.Core.Target, "target", "t", cfg.Core.Target, "Target domain (required)")
	pflag.BoolVarP(&cfg.Core.Active, "active", "a", cfg.Core.Active, "Enable active reconnaissance")
	pflag.IntVarP(&cfg.Core.Workers, "workers", "w", cfg.Core.Workers, "Concurrent workers")
	pflag.IntVarP(&cfg.Core.TimeoutS, "timeout", "T", cfg.Core.TimeoutS, "Global timeout in seconds (0=none)")
	pflag.IntVar(&cfg.Core.StageTimeoutS, "stage-timeout", cfg.Core.StageTimeoutS, "Per-stage timeout in seconds (0=no limit, only global timeout)")

	// === SOURCE FLAGS ===
	// NOTE: We need to store pointers to enabled/priority flags for each source
	// because pflag.BoolVar requires a pointer to the actual variable, not a copy.
	// We'll apply these values back to the config after pflag.Parse().
	sourceEnabledFlags := make(map[string]*bool)
	sourcePriorityFlags := make(map[string]*int)

	for name := range cfg.Source.Sources {
		sourceCfg := cfg.Source.Sources[name]

		// Create temporary variables to hold flag values
		enabled := sourceCfg.Enabled
		priority := sourceCfg.Priority

		// Register flags with pointers to temporary variables
		pflag.BoolVar(&enabled, fmt.Sprintf("src.%s", name), sourceCfg.Enabled,
			fmt.Sprintf("Enable %s source", name))
		pflag.IntVar(&priority, fmt.Sprintf("src.%s.priority", name), sourceCfg.Priority,
			fmt.Sprintf("Priority for %s (higher=first)", name))

		// Store pointers for later application
		sourceEnabledFlags[name] = &enabled
		sourcePriorityFlags[name] = &priority
	}

	// === SHODAN-SPECIFIC FLAGS ===
	// Note: Using temporary variables since we can't directly modify Custom map via pflag
	var shodanAPIKey string
	var shodanUseCLI bool
	var shodanRateLimit float64

	if shodanCfg, ok := cfg.Source.Sources["shodan"]; ok {
		if v, ok := shodanCfg.Custom["api_key"].(string); ok {
			shodanAPIKey = v
		}
		if v, ok := shodanCfg.Custom["use_cli"].(bool); ok {
			shodanUseCLI = v
		}
		if v, ok := shodanCfg.Custom["rate_limit"].(float64); ok {
			shodanRateLimit = v
		} else {
			shodanRateLimit = 1.0 // Default
		}

		pflag.StringVar(&shodanAPIKey, "src.shodan.api_key", shodanAPIKey,
			"Shodan API key (required for API mode)")
		pflag.BoolVar(&shodanUseCLI, "src.shodan.use_cli", shodanUseCLI,
			"Use Shodan CLI instead of API (default: false)")
		pflag.Float64Var(&shodanRateLimit, "src.shodan.rate_limit", shodanRateLimit,
			"Shodan API rate limit in requests/second (default: 1.0 for free tier)")
	}

	// AbuseIPDB flags
	var abuseipdbAPIKey string

	if abuseipdbCfg, ok := cfg.Source.Sources["abuseipdb"]; ok {
		if v, ok := abuseipdbCfg.Custom["api_key"].(string); ok {
			abuseipdbAPIKey = v
		}

		pflag.StringVar(&abuseipdbAPIKey, "src.abuseipdb.api_key", abuseipdbAPIKey,
			"AbuseIPDB API key (required, get free key at https://www.abuseipdb.com/api)")
	}

	// GoLinkfinderEVO flags
	var golinkfinderevoProfile string
	var golinkfinderevoWorkers int
	var golinkfinderevoMaxScriptFiles int
	var golinkfinderevoMaxHTMLFiles int
	var golinkfinderevoExecPath string
	var golinkfinderevoGFTemplatesPath string
	var golinkfinderevoGFPatterns string

	if glCfg, ok := cfg.Source.Sources["golinkfinderevo"]; ok {
		if v, ok := glCfg.Custom["profile"].(string); ok {
			golinkfinderevoProfile = v
		} else {
			golinkfinderevoProfile = "standard"
		}
		if v, ok := glCfg.Custom["workers"].(int); ok {
			golinkfinderevoWorkers = v
		} else {
			golinkfinderevoWorkers = 20
		}
		if v, ok := glCfg.Custom["max_script_files"].(int); ok {
			golinkfinderevoMaxScriptFiles = v
		} else {
			golinkfinderevoMaxScriptFiles = 50
		}
		if v, ok := glCfg.Custom["max_html_files"].(int); ok {
			golinkfinderevoMaxHTMLFiles = v
		} else {
			golinkfinderevoMaxHTMLFiles = 50
		}
		if v, ok := glCfg.Custom["exec_path"].(string); ok {
			golinkfinderevoExecPath = v
		} else {
			golinkfinderevoExecPath = "golinkfinder"
		}
		if v, ok := glCfg.Custom["gf_templates_path"].(string); ok {
			golinkfinderevoGFTemplatesPath = v
		} else {
			golinkfinderevoGFTemplatesPath = "./internal/platform/gf_templates"
		}
		// Handle gf_patterns as slice or string
		if v, ok := glCfg.Custom["gf_patterns"].([]string); ok && len(v) > 0 {
			golinkfinderevoGFPatterns = strings.Join(v, ",")
		} else {
			golinkfinderevoGFPatterns = "all"
		}

		pflag.StringVar(&golinkfinderevoProfile, "src.golinkfinderevo.profile", golinkfinderevoProfile,
			"GoLinkfinderEVO scan profile: quick, standard (default), deep")
		pflag.IntVar(&golinkfinderevoWorkers, "src.golinkfinderevo.workers", golinkfinderevoWorkers,
			"Number of concurrent workers (default: 20)")
		pflag.IntVar(&golinkfinderevoMaxScriptFiles, "src.golinkfinderevo.max-script-files", golinkfinderevoMaxScriptFiles,
			"Maximum JavaScript files to analyze (default: 50)")
		pflag.IntVar(&golinkfinderevoMaxHTMLFiles, "src.golinkfinderevo.max-html-files", golinkfinderevoMaxHTMLFiles,
			"Maximum HTML files to analyze (default: 50)")
		pflag.StringVar(&golinkfinderevoExecPath, "src.golinkfinderevo.exec-path", golinkfinderevoExecPath,
			"Path to golinkfinder binary (default: golinkfinder)")
		pflag.StringVar(&golinkfinderevoGFTemplatesPath, "src.golinkfinderevo.gf-templates-path", golinkfinderevoGFTemplatesPath,
			"Path to gf templates directory (default: ./internal/platform/gf_templates)")
		pflag.StringVar(&golinkfinderevoGFPatterns, "src.golinkfinderevo.gf-patterns", golinkfinderevoGFPatterns,
			"GF patterns to apply (comma-separated, default: all)")
	}

	// RetireJS flags
	var retirejsExecPath string
	var retirejsSeverity string
	var retirejsDeep bool
	var retirejsIncludeOSV bool
	var retirejsMaxFiles int
	var retirejsPreferLocal bool
	var retirejsFallbackDownload bool

	if retirejsCfg, ok := cfg.Source.Sources["retirejs"]; ok {
		if v, ok := retirejsCfg.Custom["exec_path"].(string); ok {
			retirejsExecPath = v
		} else {
			retirejsExecPath = "retire"
		}
		if v, ok := retirejsCfg.Custom["severity"].(string); ok {
			retirejsSeverity = v
		} else {
			retirejsSeverity = "medium"
		}
		if v, ok := retirejsCfg.Custom["deep"].(bool); ok {
			retirejsDeep = v
		} else {
			retirejsDeep = false
		}
		if v, ok := retirejsCfg.Custom["include_osv"].(bool); ok {
			retirejsIncludeOSV = v
		} else {
			retirejsIncludeOSV = false
		}
		if v, ok := retirejsCfg.Custom["max_files"].(int); ok {
			retirejsMaxFiles = v
		} else {
			retirejsMaxFiles = 100
		}
		if v, ok := retirejsCfg.Custom["prefer_local"].(bool); ok {
			retirejsPreferLocal = v
		} else {
			retirejsPreferLocal = true
		}
		if v, ok := retirejsCfg.Custom["fallback_download"].(bool); ok {
			retirejsFallbackDownload = v
		} else {
			retirejsFallbackDownload = true
		}

		pflag.StringVar(&retirejsExecPath, "src.retirejs.exec-path", retirejsExecPath,
			"Path to retire binary (default: retire)")
		pflag.StringVar(&retirejsSeverity, "src.retirejs.severity", retirejsSeverity,
			"Minimum severity to report: none, low, medium (default), high, critical")
		pflag.BoolVar(&retirejsDeep, "src.retirejs.deep", retirejsDeep,
			"Enable deep scan mode (default: false)")
		pflag.BoolVar(&retirejsIncludeOSV, "src.retirejs.include-osv", retirejsIncludeOSV,
			"Include OSV advisories (default: false)")
		pflag.IntVar(&retirejsMaxFiles, "src.retirejs.max-files", retirejsMaxFiles,
			"Maximum JavaScript files to analyze (default: 100)")
		pflag.BoolVar(&retirejsPreferLocal, "src.retirejs.prefer-local", retirejsPreferLocal,
			"Prefer local files from golinkfinderevo (default: true)")
		pflag.BoolVar(&retirejsFallbackDownload, "src.retirejs.fallback-download", retirejsFallbackDownload,
			"Fallback to download if no local files (default: true)")
	}

	// WhatWeb flags
	var whatwebExecPath string
	var whatwebAggression int
	var whatwebThreads int
	var whatwebUserAgent string

	if whatwebCfg, ok := cfg.Source.Sources["whatweb"]; ok {
		if v, ok := whatwebCfg.Custom["exec_path"].(string); ok {
			whatwebExecPath = v
		} else {
			whatwebExecPath = "whatweb"
		}
		if v, ok := whatwebCfg.Custom["aggression"].(int); ok {
			whatwebAggression = v
		} else {
			whatwebAggression = 1
		}
		if v, ok := whatwebCfg.Custom["threads"].(int); ok {
			whatwebThreads = v
		} else {
			whatwebThreads = 25
		}
		if v, ok := whatwebCfg.Custom["user_agent"].(string); ok {
			whatwebUserAgent = v
		} else {
			whatwebUserAgent = "AethonX/1.0 (Web reconnaissance tool)"
		}

		pflag.StringVar(&whatwebExecPath, "src.whatweb.exec-path", whatwebExecPath,
			"Path to whatweb binary (default: whatweb)")
		pflag.IntVar(&whatwebAggression, "src.whatweb.aggression", whatwebAggression,
			"Aggression level 1-4 (1=stealthy, 4=heavy, default: 1)")
		pflag.IntVar(&whatwebThreads, "src.whatweb.threads", whatwebThreads,
			"Number of concurrent threads (default: 25)")
		pflag.StringVar(&whatwebUserAgent, "src.whatweb.user-agent", whatwebUserAgent,
			"Custom user agent string")
	}

	// === OUTPUT FLAGS ===
	pflag.StringVarP(&cfg.Output.Dir, "out", "o", cfg.Output.Dir, "Output directory")
	pflag.StringVar(&cfg.Output.UIMode, "ui-mode", cfg.Output.UIMode,
		"UI mode: pretty (default, visual), raw (plain logs)")
	pflag.StringVar(&cfg.Output.LogFormat, "log-format", cfg.Output.LogFormat,
		"Log format for raw mode: text (default, logfmt), json (structured)")
	pflag.BoolVar(&cfg.Output.ShowMetrics, "show-metrics", cfg.Output.ShowMetrics,
		"Show system metrics (CPU, memory, goroutines)")
	pflag.BoolVar(&cfg.Output.ShowPhases, "show-phases", cfg.Output.ShowPhases,
		"Show execution phases for each source")

	// === STREAMING FLAGS ===
	pflag.IntVarP(&cfg.Streaming.ArtifactThreshold, "streaming", "s", cfg.Streaming.ArtifactThreshold,
		"Artifact threshold for streaming")

	// === RESILIENCE FLAGS ===
	pflag.IntVarP(&cfg.Resilience.MaxRetries, "retries", "r", cfg.Resilience.MaxRetries,
		"Max retries per source")
	pflag.BoolVar(&cfg.Resilience.CircuitBreakerEnabled, "circuit-breaker", cfg.Resilience.CircuitBreakerEnabled,
		"Enable circuit breaker")

	// === NETWORK FLAGS ===
	pflag.StringVarP(&cfg.Network.ProxyURL, "proxy", "p", cfg.Network.ProxyURL, "HTTP(S) proxy URL")

	// === ENRICHMENT FLAGS ===
	pflag.BoolVar(&cfg.Enrichment.Enabled, "enrich", cfg.Enrichment.Enabled,
		"Enable CVE enrichment (default: true)")
	pflag.BoolVar(&cfg.Enrichment.Enabled, "no-enrich", !cfg.Enrichment.Enabled,
		"Disable CVE enrichment")
	pflag.StringVar(&cfg.Enrichment.Provider, "enrich-provider", cfg.Enrichment.Provider,
		"CVE enrichment provider: nvd, circl (default: nvd)")
	pflag.StringVar(&cfg.Enrichment.NVDAPIKey, "enrich-nvd-api-key", cfg.Enrichment.NVDAPIKey,
		"NVD API key (optional, increases rate limit from 0.6/s to 50/s)")

	// === VULN ENRICHMENT FLAGS ===
	pflag.BoolVar(&cfg.VulnEnrichment.Enabled, "vuln-enrich", cfg.VulnEnrichment.Enabled,
		"Enable Vulners.com service vulnerability enrichment (default: false)")
	pflag.StringVar(&cfg.VulnEnrichment.APIKey, "vuln-enrich-api-key", cfg.VulnEnrichment.APIKey,
		"Vulners.com API key (optional, increases rate limit from 1000/day)")

	// Parse flags
	pflag.Parse()

	// Apply source-specific flags back to config
	for name := range cfg.Source.Sources {
		sourceCfg := cfg.Source.Sources[name]

		// Apply enabled flag if it was set
		if enabledPtr, ok := sourceEnabledFlags[name]; ok && enabledPtr != nil {
			sourceCfg.Enabled = *enabledPtr
		}

		// Apply priority flag if it was set
		if priorityPtr, ok := sourcePriorityFlags[name]; ok && priorityPtr != nil {
			sourceCfg.Priority = *priorityPtr
		}

		cfg.Source.Sources[name] = sourceCfg
	}

	// Apply Shodan-specific flags back to config
	if shodanCfg, ok := cfg.Source.Sources["shodan"]; ok {
		shodanCfg.Custom["api_key"] = shodanAPIKey
		shodanCfg.Custom["use_cli"] = shodanUseCLI
		shodanCfg.Custom["rate_limit"] = shodanRateLimit
		cfg.Source.Sources["shodan"] = shodanCfg
	}

	// Apply AbuseIPDB-specific flags back to config
	if abuseipdbCfg, ok := cfg.Source.Sources["abuseipdb"]; ok {
		abuseipdbCfg.Custom["api_key"] = abuseipdbAPIKey
		cfg.Source.Sources["abuseipdb"] = abuseipdbCfg
	}

	// Apply GoLinkfinderEVO-specific flags back to config
	if glCfg, ok := cfg.Source.Sources["golinkfinderevo"]; ok {
		glCfg.Custom["profile"] = golinkfinderevoProfile
		glCfg.Custom["workers"] = golinkfinderevoWorkers
		glCfg.Custom["max_script_files"] = golinkfinderevoMaxScriptFiles
		glCfg.Custom["max_html_files"] = golinkfinderevoMaxHTMLFiles
		glCfg.Custom["exec_path"] = golinkfinderevoExecPath
		glCfg.Custom["gf_templates_path"] = golinkfinderevoGFTemplatesPath
		// Parse comma-separated patterns back to slice
		if golinkfinderevoGFPatterns != "" {
			glCfg.Custom["gf_patterns"] = strings.Split(golinkfinderevoGFPatterns, ",")
		}
		cfg.Source.Sources["golinkfinderevo"] = glCfg
	}

	// Apply RetireJS-specific flags back to config
	if retirejsCfg, ok := cfg.Source.Sources["retirejs"]; ok {
		retirejsCfg.Custom["exec_path"] = retirejsExecPath
		retirejsCfg.Custom["severity"] = retirejsSeverity
		retirejsCfg.Custom["deep"] = retirejsDeep
		retirejsCfg.Custom["include_osv"] = retirejsIncludeOSV
		retirejsCfg.Custom["max_files"] = retirejsMaxFiles
		retirejsCfg.Custom["prefer_local"] = retirejsPreferLocal
		retirejsCfg.Custom["fallback_download"] = retirejsFallbackDownload
		cfg.Source.Sources["retirejs"] = retirejsCfg
	}

	// Apply WhatWeb-specific flags back to config
	if whatwebCfg, ok := cfg.Source.Sources["whatweb"]; ok {
		whatwebCfg.Custom["exec_path"] = whatwebExecPath
		whatwebCfg.Custom["aggression"] = whatwebAggression
		whatwebCfg.Custom["threads"] = whatwebThreads
		whatwebCfg.Custom["user_agent"] = whatwebUserAgent
		cfg.Source.Sources["whatweb"] = whatwebCfg
	}

	// Handle help and version flags
	if *showHelp {
		PrintHelp()
	}

	if *showVersion {
		PrintVersion(version, commit, date)
	}

	// Detect common mistake: user typed "-target" instead of "--target" or "-t"
	// This happens because "-target" is interpreted as "-t -a -r -g -e -t"
	detectCommonFlagMistakes(cfg)
}

// detectCommonFlagMistakes warns users about common CLI flag errors.
func detectCommonFlagMistakes(cfg *Config) {
	// Check if target looks truncated (common sign of "-target" mistake)
	// When user types "-target example.com", the result is often a truncated domain
	// like "arget" from "target" or "ample.com" from "example.com"

	target := cfg.Core.Target

	// Heuristics for detecting "-target" mistake:
	// 1. Target has no dot AND is short (< 8 chars) - likely truncated
	// 2. Target starts with invalid chars (common when flags are mangled)
	//
	// NOTE: We don't check for active flag + short domain anymore because
	// legitimate short domains like "uvesa.es" (8 chars) would trigger false positives
	suspiciousTruncated := target != "" && len(target) < 8 && !strings.Contains(target, ".")

	// Check if target starts with common flag prefixes that got mangled
	// (e.g., "arget", "ctive", "orkers") - these are clear mistakes
	suspiciousPrefix := target != "" && !strings.Contains(target, ".") &&
		(strings.HasPrefix(target, "arget") ||
			strings.HasPrefix(target, "ctive") ||
			strings.HasPrefix(target, "orkers"))

	if suspiciousTruncated || suspiciousPrefix {
		fmt.Fprintf(os.Stderr, "\n⚠ WARNING: Suspicious target detected: %q\n", cfg.Core.Target)
		fmt.Fprintf(os.Stderr, "   Did you mean to use --target (double dash) instead of -target (single dash)?\n")
		fmt.Fprintf(os.Stderr, "\n   Common mistake:\n")
		fmt.Fprintf(os.Stderr, "     ✖  aethonx -target example.com    (interprets as: -t -a -r -g -e -t)\n")
		fmt.Fprintf(os.Stderr, "\n   Correct usage:\n")
		fmt.Fprintf(os.Stderr, "     ✓  aethonx --target example.com   (double dash for long flags)\n")
		fmt.Fprintf(os.Stderr, "     ✓  aethonx -t example.com         (single dash for short flags)\n\n")
		os.Exit(2)
	}
}

// normalize sanitizes and validates configuration values.
func normalize(c *Config) {
	// Core normalization
	c.Core.Target = strings.TrimSpace(strings.ToLower(strings.TrimSuffix(c.Core.Target, ".")))
	if c.Core.Workers < 1 {
		c.Core.Workers = 1
	}
	if c.Core.TimeoutS < 0 {
		c.Core.TimeoutS = 0
	}

	// Output normalization
	if c.Output.Dir == "" {
		c.Output.Dir = "aethonx_out"
	}

	// Resilience normalization
	if c.Resilience.BackoffBase < 0 {
		c.Resilience.BackoffBase = 1 * time.Second
	}
	if c.Resilience.BackoffMultiplier < 1.0 {
		c.Resilience.BackoffMultiplier = 2.0
	}
}

// ToJSON serializa la configuración a JSON (útil para debugging).
func (c Config) ToJSON() (string, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Timeout returns global timeout as time.Duration.
func (c Config) Timeout() time.Duration {
	if c.Core.TimeoutS <= 0 {
		return 0
	}
	return time.Duration(c.Core.TimeoutS) * time.Second
}

// Helpers

func getenv(k, def string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return def
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	default:
		return false
	}
}

func parseInt(v string, def int) int {
	i, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return i
}
