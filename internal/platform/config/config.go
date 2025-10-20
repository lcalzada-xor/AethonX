// internal/platform/config/config.go
package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"aethonx/internal/core/ports"
)

type Config struct {
	// App
	Target       string
	Active       bool
	Workers      int
	TimeoutS     int // segundos (0 = sin timeout)
	PrintVersion bool

	// IO
	OutputDir string

	// Sources: mapa dinámico de configuraciones por source
	// Key = source name (ej: "crtsh", "rdap", "subfinder")
	// Value = configuración específica de esa source
	Sources map[string]ports.SourceConfig

	// Outputs
	Outputs Outputs

	// Streaming
	Streaming Streaming

	// Resilience
	Resilience Resilience

	// Proxy
	ProxyURL string
}

type Outputs struct {
	TableDisabled bool
	// JSON output is ALWAYS generated (required for streaming consolidation)
}

type Streaming struct {
	ArtifactThreshold int // Número de artifacts por source para activar escritura parcial
}

type Resilience struct {
	// Retry configuration
	MaxRetries       int           // Default max retries for sources
	BackoffBase      time.Duration // Base backoff duration (e.g., 1s)
	BackoffMultiplier float64      // Multiplier for exponential backoff (e.g., 2.0)

	// Circuit Breaker configuration
	CircuitBreakerEnabled      bool
	CircuitBreakerThreshold    int           // Failures before opening circuit
	CircuitBreakerTimeout      time.Duration // How long circuit stays open
	CircuitBreakerHalfOpenMax  int           // Max requests in half-open state
}

// DefaultConfig retorna una configuración por defecto.
func DefaultConfig() Config {
	return Config{
		Target:   "",
		Active:   false,
		Workers:  4,
		TimeoutS: 30,

		OutputDir: "aethonx_out",
		ProxyURL:  "",

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
				Timeout:   30 * time.Second,
				Retries:   2,
				RateLimit: 0,
				Priority:  7,
				Custom:    make(map[string]interface{}),
			},
		},

		Outputs: Outputs{
			TableDisabled: false,
		},

		Streaming: Streaming{
			ArtifactThreshold: 1000,
		},

		Resilience: Resilience{
			MaxRetries:                 3,
			BackoffBase:                1 * time.Second,
			BackoffMultiplier:          2.0,
			CircuitBreakerEnabled:      true,
			CircuitBreakerThreshold:    5,
			CircuitBreakerTimeout:      60 * time.Second,
			CircuitBreakerHalfOpenMax:  3,
		},
	}
}

// Load inicializa la configuración: ENV -> defaults, luego FLAGS (flags tienen prioridad).
func Load() (Config, error) {
	cfg := DefaultConfig()

	// Cargar desde ENV
	loadFromEnv(&cfg)

	// Parsear flags (overrides ENV)
	loadFromFlags(&cfg)

	// Normalizar
	normalize(&cfg)

	return cfg, nil
}

// loadFromEnv carga configuración desde variables de entorno.
func loadFromEnv(cfg *Config) {
	if v := getenv("AETHONX_TARGET", ""); v != "" {
		cfg.Target = v
	}
	if v := getenv("AETHONX_ACTIVE", ""); v != "" {
		cfg.Active = parseBool(v)
	}
	if v := getenv("AETHONX_WORKERS", ""); v != "" {
		cfg.Workers = parseInt(v, cfg.Workers)
	}
	if v := getenv("AETHONX_TIMEOUT", ""); v != "" {
		cfg.TimeoutS = parseInt(v, cfg.TimeoutS)
	}
	if v := getenv("AETHONX_OUTPUT_DIR", ""); v != "" {
		cfg.OutputDir = v
	}
	if v := getenv("AETHONX_PROXY_URL", ""); v != "" {
		cfg.ProxyURL = v
	}

	// Sources config desde ENV
	// Formato: AETHONX_SOURCES_CRTSH_ENABLED=true
	//          AETHONX_SOURCES_CRTSH_PRIORITY=10
	//          AETHONX_SOURCES_CRTSH_TIMEOUT=60
	for name := range cfg.Sources {
		prefix := fmt.Sprintf("AETHONX_SOURCES_%s_", strings.ToUpper(name))

		sourceCfg := cfg.Sources[name]

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

		cfg.Sources[name] = sourceCfg
	}

	// Outputs
	if v := getenv("AETHONX_OUTPUTS_TABLE_DISABLED", ""); v != "" {
		cfg.Outputs.TableDisabled = parseBool(v)
	}

	// Streaming
	if v := getenv("AETHONX_STREAMING_THRESHOLD", ""); v != "" {
		cfg.Streaming.ArtifactThreshold = parseInt(v, cfg.Streaming.ArtifactThreshold)
	}

	// Resilience
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

// loadFromFlags parsea flags de CLI.
func loadFromFlags(cfg *Config) {
	flag.StringVar(&cfg.Target, "target", cfg.Target, "Dominio objetivo (e.g., example.com)")
	flag.BoolVar(&cfg.Active, "active", cfg.Active, "Habilitar fase activa")
	flag.IntVar(&cfg.Workers, "workers", cfg.Workers, "Concurrencia máxima de fuentes")
	flag.IntVar(&cfg.TimeoutS, "timeout", cfg.TimeoutS, "Timeout global en segundos (0 = sin timeout)")

	flag.StringVar(&cfg.OutputDir, "out", cfg.OutputDir, "Directorio de salida")
	flag.BoolVar(&cfg.PrintVersion, "version", false, "Imprimir versión y salir")

	// Source configs (solo enabled y priority via flags, el resto via ENV o defaults)
	for name := range cfg.Sources {
		sourceCfg := cfg.Sources[name]
		flag.BoolVar(&sourceCfg.Enabled, fmt.Sprintf("src.%s", name), sourceCfg.Enabled,
			fmt.Sprintf("Habilitar fuente %s", name))
		flag.IntVar(&sourceCfg.Priority, fmt.Sprintf("src.%s.priority", name), sourceCfg.Priority,
			fmt.Sprintf("Prioridad de fuente %s (mayor = más prioritario)", name))
		cfg.Sources[name] = sourceCfg
	}

	// Outputs
	flag.BoolVar(&cfg.Outputs.TableDisabled, "out.no-table", cfg.Outputs.TableDisabled,
		"Desactivar salida en tabla (JSON siempre se genera)")

	// Streaming
	flag.IntVar(&cfg.Streaming.ArtifactThreshold, "streaming.threshold", cfg.Streaming.ArtifactThreshold,
		"Threshold de artifacts para activar escritura parcial por source")

	// Resilience
	flag.IntVar(&cfg.Resilience.MaxRetries, "resilience.retries", cfg.Resilience.MaxRetries,
		"Número máximo de reintentos por source")
	flag.BoolVar(&cfg.Resilience.CircuitBreakerEnabled, "resilience.cb", cfg.Resilience.CircuitBreakerEnabled,
		"Habilitar circuit breaker")

	// Proxy
	flag.StringVar(&cfg.ProxyURL, "proxy", cfg.ProxyURL, "Proxy HTTP(S) para peticiones salientes (opcional)")

	flag.Parse()
}

func normalize(c *Config) {
	c.Target = strings.TrimSpace(strings.ToLower(strings.TrimSuffix(c.Target, ".")))
	if c.Workers < 1 {
		c.Workers = 1
	}
	if c.TimeoutS < 0 {
		c.TimeoutS = 0
	}
	if c.OutputDir == "" {
		c.OutputDir = "aethonx_out"
	}
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

// Timeout devuelve un time.Duration útil si prefieres trabajar con duración.
func (c Config) Timeout() time.Duration {
	if c.TimeoutS <= 0 {
		return 0
	}
	return time.Duration(c.TimeoutS) * time.Second
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
