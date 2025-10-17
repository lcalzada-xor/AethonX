// internal/platform/config/config.go
package config

import (
	"flag"
	"os"
	"strconv"
	"strings"
	"time"
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

	// Toggles
	Sources Sources
	Outputs Outputs

	// Futuro: proxy, cache dir, etc.
	ProxyURL string
}

type Sources struct {
	CRTSHEnabled bool
	RDAPEnabled  bool
	// Ej: SubfinderEnabled bool
	//     AmassEnabled     bool
}

type Outputs struct {
	JSONEnabled   bool
	TableDisabled bool
	// Ej: NDJSONEnabled bool
}

// Load inicializa la configuración: ENV -> defaults, luego FLAGS (flags tienen prioridad).
func Load() (Config, error) {
	// Defaults desde ENV
	def := Config{
		Target:   getenv("AETHONX_TARGET", ""),
		Active:   parseBool(getenv("AETHONX_ACTIVE", "false")),
		Workers:  parseInt(getenv("AETHONX_WORKERS", "4"), 4),
		TimeoutS: parseInt(getenv("AETHONX_TIMEOUT", "30"), 30),

		OutputDir: getenv("AETHONX_OUTPUT_DIR", "aethonx_out"),
		ProxyURL:  getenv("AETHONX_PROXY_URL", ""),

		Sources: Sources{
			CRTSHEnabled: parseBool(getenv("AETHONX_SOURCES_CRTSH", "true")),
			RDAPEnabled:  parseBool(getenv("AETHONX_SOURCES_RDAP", "true")),
		},
		Outputs: Outputs{
			JSONEnabled:   parseBool(getenv("AETHONX_OUTPUTS_JSON", "false")),
			TableDisabled: parseBool(getenv("AETHONX_OUTPUTS_TABLE_DISABLED", "false")),
		},
	}

	// Definir flags con esos defaults
	var cfg Config
	cfg = def // copia inicial

	flag.StringVar(&cfg.Target, "target", def.Target, "Dominio objetivo (e.g., example.com)")
	flag.BoolVar(&cfg.Active, "active", def.Active, "Habilitar fase activa")
	flag.IntVar(&cfg.Workers, "workers", def.Workers, "Concurrencia máxima de fuentes")
	flag.IntVar(&cfg.TimeoutS, "timeout", def.TimeoutS, "Timeout global en segundos (0 = sin timeout)")

	flag.StringVar(&cfg.OutputDir, "out", def.OutputDir, "Directorio de salida")
	flag.BoolVar(&cfg.PrintVersion, "version", false, "Imprimir versión y salir")

	// Toggles de fuentes
	flag.BoolVar(&cfg.Sources.CRTSHEnabled, "src.crtsh", def.Sources.CRTSHEnabled, "Habilitar fuente crt.sh")
	flag.BoolVar(&cfg.Sources.RDAPEnabled, "src.rdap", def.Sources.RDAPEnabled, "Habilitar fuente RDAP")

	// Toggles de salida
	flag.BoolVar(&cfg.Outputs.JSONEnabled, "out.json", def.Outputs.JSONEnabled, "Emitir salida JSON")
	flag.BoolVar(&cfg.Outputs.TableDisabled, "out.no-table", def.Outputs.TableDisabled, "Desactivar salida en tabla")

	// Infra
	flag.StringVar(&cfg.ProxyURL, "proxy", def.ProxyURL, "Proxy HTTP(S) para peticiones salientes (opcional)")

	// Permite que otras librerías definan sus flags antes de parsear
	// En main.go ya hacemos flag.Parse() implícitamente aquí:
	flag.Parse()

	normalize(&cfg)
	return cfg, nil
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

// Timeout devuelve un time.Duration útil si prefieres trabajar con duración.
func (c Config) Timeout() time.Duration {
	if c.TimeoutS <= 0 {
		return 0
	}
	return time.Duration(c.TimeoutS) * time.Second
}
