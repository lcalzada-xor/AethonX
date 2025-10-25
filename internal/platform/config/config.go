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

	"github.com/spf13/pflag"
)

// Config is the main configuration structure organized by functional categories.
type Config struct {
	Core       CoreConfig
	Source     SourceConfig
	Output     OutputConfig
	Streaming  StreamingConfig
	Resilience ResilienceConfig
	Network    NetworkConfig
}

// CoreConfig contains fundamental scan parameters.
type CoreConfig struct {
	Target   string // Target domain (required)
	Active   bool   // Enable active reconnaissance mode
	Workers  int    // Number of concurrent workers
	TimeoutS int    // Global timeout in seconds (0 = no timeout)
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

// DefaultConfig returns a default configuration organized by categories.
func DefaultConfig() Config {
	return Config{
		Core: CoreConfig{
			Target:   "",
			Active:   false,
			Workers:  4,
			TimeoutS: 30,
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
				"httpx": {
					Enabled:   true,
					Timeout:   120 * time.Second, // httpx can be slow with tech detection
					Retries:   2,
					RateLimit: 0,
					Priority:  15, // High priority after passive sources
					Custom: map[string]interface{}{
						"profile":    "full",
						"threads":    50,
						"rate_limit": 150,
						"exec_path":  "httpx",
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
	}
}

// Load initializes configuration: ENV -> defaults, then FLAGS (flags take priority).
func Load(version, commit, date string) (Config, error) {
	cfg := DefaultConfig()

	// Load from ENV
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
				sourceCfg.Custom["threads"] = parseInt(v, 50)
			}
			if v := getenv(prefix+"RATE_LIMIT", ""); v != "" {
				sourceCfg.Custom["rate_limit"] = parseInt(v, 150)
			}
			if v := getenv(prefix+"EXEC_PATH", ""); v != "" {
				sourceCfg.Custom["exec_path"] = v
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

	// === SOURCE FLAGS ===
	for name := range cfg.Source.Sources {
		sourceCfg := cfg.Source.Sources[name]
		pflag.BoolVar(&sourceCfg.Enabled, fmt.Sprintf("src.%s", name), sourceCfg.Enabled,
			fmt.Sprintf("Enable %s source", name))
		pflag.IntVar(&sourceCfg.Priority, fmt.Sprintf("src.%s.priority", name), sourceCfg.Priority,
			fmt.Sprintf("Priority for %s (higher=first)", name))
		cfg.Source.Sources[name] = sourceCfg
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

	// Parse flags
	pflag.Parse()

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
		fmt.Fprintf(os.Stderr, "\n⚠️  WARNING: Suspicious target detected: %q\n", cfg.Core.Target)
		fmt.Fprintf(os.Stderr, "   Did you mean to use --target (double dash) instead of -target (single dash)?\n")
		fmt.Fprintf(os.Stderr, "\n   Common mistake:\n")
		fmt.Fprintf(os.Stderr, "     ❌  aethonx -target example.com    (interprets as: -t -a -r -g -e -t)\n")
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
