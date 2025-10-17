// internal/core/ports/source.go
package ports

import (
	"context"
	"time"

	"aethonx/internal/core/domain"
)

// Source es el port primario para todas las fuentes de datos en AethonX.
// Cualquier fuente (API, CLI, builtin) debe implementar esta interfaz.
type Source interface {
	// Name retorna el nombre único de la fuente (ej: "crtsh", "subfinder", "shodan")
	Name() string

	// Mode retorna el modo de operación de la fuente (passive, active, both)
	Mode() domain.SourceMode

	// Type retorna el tipo de implementación (api, cli, builtin, etc.)
	Type() domain.SourceType

	// Run ejecuta la fuente contra el target y retorna los resultados
	Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error)
}

// AdvancedSource extiende Source con capacidades adicionales opcionales.
// Las fuentes pueden implementar esta interfaz mediante type assertion.
type AdvancedSource interface {
	Source

	// Initialize prepara la fuente con configuración específica
	Initialize(ctx context.Context, cfg SourceConfig) error

	// Validate verifica que la fuente esté correctamente configurada
	Validate() error

	// Close libera recursos utilizados por la fuente
	Close() error

	// HealthCheck verifica que la fuente esté operativa
	HealthCheck(ctx context.Context) error
}

// StreamingSource permite a las fuentes emitir artefactos en tiempo real.
type StreamingSource interface {
	Source

	// Stream ejecuta la fuente y emite artefactos a medida que los descubre
	Stream(ctx context.Context, target domain.Target) (<-chan *domain.Artifact, <-chan error)
}

// RateLimitedSource indica que la fuente implementa rate limiting.
type RateLimitedSource interface {
	Source

	// SetRateLimit configura el límite de peticiones por segundo
	SetRateLimit(requestsPerSecond int)

	// GetRateLimit retorna el límite actual
	GetRateLimit() int
}

// SourceConfig contiene la configuración específica de una fuente.
type SourceConfig struct {
	// Enabled indica si la fuente está habilitada
	Enabled bool

	// Timeout tiempo máximo de ejecución
	Timeout time.Duration

	// Retries número de reintentos en caso de fallo
	Retries int

	// RateLimit límite de peticiones por segundo (0 = sin límite)
	RateLimit int

	// Priority prioridad de ejecución (mayor = más prioritario)
	Priority int

	// Custom configuración específica de la fuente (API keys, paths, etc.)
	Custom map[string]interface{}
}

// DefaultSourceConfig retorna una configuración por defecto.
func DefaultSourceConfig() SourceConfig {
	return SourceConfig{
		Enabled:   true,
		Timeout:   30 * time.Second,
		Retries:   2,
		RateLimit: 0,
		Priority:  0,
		Custom:    make(map[string]interface{}),
	}
}

// SourceFactory es una función que crea una instancia de Source.
type SourceFactory func(cfg SourceConfig) (Source, error)

// SourceMetadata contiene metadatos sobre una fuente.
type SourceMetadata struct {
	Name        string
	Description string
	Version     string
	Author      string
	Mode        domain.SourceMode
	Type        domain.SourceType
	RequiresAuth bool
	RateLimit   int // Límite recomendado de requests/segundo
}
