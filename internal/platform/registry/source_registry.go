// internal/platform/registry/source_registry.go
package registry

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
)

// SourceRegistry gestiona el registro y construcción de sources.
// Implementa el patrón Registry + Factory para desacoplar la creación
// de sources del código de aplicación.
type SourceRegistry struct {
	mu        sync.RWMutex
	factories map[string]SourceFactory
	metadata  map[string]ports.SourceMetadata
	logger    logx.Logger
}

// SourceFactory es una función que crea una instancia de Source.
type SourceFactory func(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error)

// globalRegistry es la instancia global del registry.
var globalRegistry *SourceRegistry
var once sync.Once

// Global retorna la instancia global del registry.
func Global() *SourceRegistry {
	once.Do(func() {
		globalRegistry = NewSourceRegistry(logx.New())
	})
	return globalRegistry
}

// NewSourceRegistry crea un nuevo registry de sources.
func NewSourceRegistry(logger logx.Logger) *SourceRegistry {
	return &SourceRegistry{
		factories: make(map[string]SourceFactory),
		metadata:  make(map[string]ports.SourceMetadata),
		logger:    logger.With("component", "source-registry"),
	}
}

// Register registra una source factory con su metadata.
// Típicamente llamado desde init() de cada source package.
func (r *SourceRegistry) Register(name string, factory SourceFactory, meta ports.SourceMetadata) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if name == "" {
		return fmt.Errorf("source name cannot be empty")
	}

	if factory == nil {
		return fmt.Errorf("factory cannot be nil for source %s", name)
	}

	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("source %s is already registered", name)
	}

	r.factories[name] = factory
	r.metadata[name] = meta
	r.logger.Debug("source registered", "name", name, "mode", meta.Mode, "type", meta.Type)

	return nil
}


// Build construye todas las sources habilitadas según la configuración.
func (r *SourceRegistry) Build(configs map[string]ports.SourceConfig, logger logx.Logger) ([]ports.Source, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Validación de configuración (fail-fast)
	if configs == nil {
		return nil, fmt.Errorf("configs cannot be nil")
	}

	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	sources := make([]ports.Source, 0, len(configs))
	errors := make([]error, 0)

	// Ordenar por prioridad (mayor a menor)
	type prioritizedSource struct {
		name     string
		config   ports.SourceConfig
		priority int
	}

	prioritized := make([]prioritizedSource, 0, len(configs))
	for name, cfg := range configs {
		if !cfg.Enabled {
			continue
		}

		// Validar que la source esté registrada
		if _, exists := r.factories[name]; !exists {
			r.logger.Warn("source not registered, skipping",
				"source", name,
			)
			errors = append(errors, fmt.Errorf("source %s not registered in registry", name))
			continue
		}

		// Validar prioridad razonable
		if cfg.Priority < 0 {
			r.logger.Warn("invalid priority, using default",
				"source", name,
				"priority", cfg.Priority,
			)
			cfg.Priority = 5 // Default priority
		}

		prioritized = append(prioritized, prioritizedSource{
			name:     name,
			config:   cfg,
			priority: cfg.Priority,
		})
	}

	sort.Slice(prioritized, func(i, j int) bool {
		return prioritized[i].priority > prioritized[j].priority
	})

	// Construir sources
	for _, ps := range prioritized {
		factory := r.factories[ps.name] // Ya validado arriba

		source, err := factory(ps.config, logger)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to build source %s: %w", ps.name, err))
			continue
		}

		// Si implementa AdvancedSource, inicializar
		if advanced, ok := source.(ports.AdvancedSource); ok {
			ctx := context.Background()
			if err := advanced.Initialize(ctx, ps.config); err != nil {
				r.logger.Warn("failed to initialize source",
					"source", ps.name,
					"error", err.Error(),
				)
				// Continuar de todas formas
			}
		}

		sources = append(sources, source)
		r.logger.Debug("source built",
			"name", ps.name,
			"priority", ps.priority,
			"mode", r.metadata[ps.name].Mode,
		)
	}

	if len(errors) > 0 {
		// Log errors pero no fallar completamente
		for _, err := range errors {
			r.logger.Warn("source build error", "error", err.Error())
		}
	}

	if len(sources) == 0 && len(configs) > 0 {
		return nil, fmt.Errorf("no sources could be built")
	}

	// Use the provided logger (respects visual mode) instead of registry's logger
	logger.Info("sources built", "count", len(sources), "requested", len(configs))
	return sources, nil
}

// List retorna los nombres de todas las sources registradas.
func (r *SourceRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetMetadata retorna el metadata de una source.
func (r *SourceRegistry) GetMetadata(name string) (ports.SourceMetadata, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	meta, exists := r.metadata[name]
	return meta, exists
}

// GetAllMetadata retorna el metadata de todas las sources registradas.
func (r *SourceRegistry) GetAllMetadata() map[string]ports.SourceMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Crear copia para evitar race conditions
	result := make(map[string]ports.SourceMetadata, len(r.metadata))
	for name, meta := range r.metadata {
		result[name] = meta
	}

	return result
}

// IsRegistered verifica si una source está registrada.
func (r *SourceRegistry) IsRegistered(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.factories[name]
	return exists
}

// Clear elimina todas las sources registradas (útil para testing).
func (r *SourceRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.factories = make(map[string]SourceFactory)
	r.metadata = make(map[string]ports.SourceMetadata)
}
