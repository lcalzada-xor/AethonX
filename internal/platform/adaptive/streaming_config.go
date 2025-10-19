// internal/platform/adaptive/streaming_config.go
package adaptive

import (
	"runtime"
	"sync"
	"time"

	"aethonx/internal/platform/logx"
)

// AdaptiveStreamingConfig calcula el threshold de streaming dinámicamente
// basándose en memoria disponible y uso actual.
type AdaptiveStreamingConfig struct {
	mu sync.RWMutex

	// Config base
	maxMemoryMB       int64 // Límite máximo de memoria (MB)
	avgArtifactSizeKB int   // Tamaño promedio estimado por artifact (KB)
	minThreshold      int   // Threshold mínimo
	maxThreshold      int   // Threshold máximo

	// Runtime
	currentThreshold int
	logger           logx.Logger

	// Monitoring
	lastUpdate    time.Time
	updateInterval time.Duration
}

// AdaptiveStreamingOptions configura el adaptive streaming.
type AdaptiveStreamingOptions struct {
	MaxMemoryMB       int64         // Default: 512MB
	AvgArtifactSizeKB int           // Default: 5KB
	MinThreshold      int           // Default: 100
	MaxThreshold      int           // Default: 10000
	UpdateInterval    time.Duration // Default: 10s
	Logger            logx.Logger
}

// NewAdaptiveStreamingConfig crea una nueva configuración adaptativa.
func NewAdaptiveStreamingConfig(opts AdaptiveStreamingOptions) *AdaptiveStreamingConfig {
	if opts.MaxMemoryMB <= 0 {
		opts.MaxMemoryMB = 512
	}
	if opts.AvgArtifactSizeKB <= 0 {
		opts.AvgArtifactSizeKB = 5
	}
	if opts.MinThreshold <= 0 {
		opts.MinThreshold = 100
	}
	if opts.MaxThreshold <= 0 {
		opts.MaxThreshold = 10000
	}
	if opts.UpdateInterval <= 0 {
		opts.UpdateInterval = 10 * time.Second
	}
	if opts.Logger == nil {
		opts.Logger = logx.New()
	}

	config := &AdaptiveStreamingConfig{
		maxMemoryMB:       opts.MaxMemoryMB,
		avgArtifactSizeKB: opts.AvgArtifactSizeKB,
		minThreshold:      opts.MinThreshold,
		maxThreshold:      opts.MaxThreshold,
		updateInterval:    opts.UpdateInterval,
		logger:            opts.Logger.With("component", "adaptive-streaming"),
	}

	// Calcular threshold inicial
	config.currentThreshold = config.calculateThreshold()
	config.lastUpdate = time.Now()

	config.logger.Info("adaptive streaming initialized",
		"max_memory_mb", config.maxMemoryMB,
		"avg_artifact_kb", config.avgArtifactSizeKB,
		"min_threshold", config.minThreshold,
		"max_threshold", config.maxThreshold,
		"initial_threshold", config.currentThreshold,
	)

	return config
}

// GetThreshold retorna el threshold actual, recalculando si es necesario.
func (c *AdaptiveStreamingConfig) GetThreshold() int {
	c.mu.RLock()
	shouldUpdate := time.Since(c.lastUpdate) > c.updateInterval
	c.mu.RUnlock()

	if shouldUpdate {
		c.mu.Lock()
		c.currentThreshold = c.calculateThreshold()
		c.lastUpdate = time.Now()
		c.mu.Unlock()

		c.logger.Debug("threshold updated",
			"new_threshold", c.currentThreshold,
		)
	}

	c.mu.RLock()
	threshold := c.currentThreshold
	c.mu.RUnlock()

	return threshold
}

// calculateThreshold calcula el threshold basado en memoria disponible.
func (c *AdaptiveStreamingConfig) calculateThreshold() int {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Memoria en uso (MB)
	allocMB := int64(m.Alloc / 1024 / 1024)
	sysMB := int64(m.Sys / 1024 / 1024)

	// Memoria disponible estimada (max - en uso)
	availableMB := c.maxMemoryMB - allocMB

	// Si memoria disponible < 20%, usar threshold mínimo (stream agresivamente)
	usagePercent := float64(allocMB) / float64(c.maxMemoryMB) * 100
	if usagePercent > 80 {
		c.logger.Warn("high memory usage, using min threshold",
			"usage_percent", usagePercent,
			"alloc_mb", allocMB,
			"threshold", c.minThreshold,
		)
		return c.minThreshold
	}

	// Calcular cuántos artifacts caben en memoria disponible
	// availableMB * 1024 KB / avgArtifactSizeKB
	artifactsInMemory := (availableMB * 1024) / int64(c.avgArtifactSizeKB)

	// Usar 50% del espacio disponible para threshold (margen de seguridad)
	threshold := int(artifactsInMemory / 2)

	// Aplicar límites
	if threshold < c.minThreshold {
		threshold = c.minThreshold
	}
	if threshold > c.maxThreshold {
		threshold = c.maxThreshold
	}

	c.logger.Debug("threshold calculated",
		"alloc_mb", allocMB,
		"sys_mb", sysMB,
		"available_mb", availableMB,
		"usage_percent", usagePercent,
		"threshold", threshold,
	)

	return threshold
}

// Stats retorna estadísticas de memoria y threshold.
func (c *AdaptiveStreamingConfig) Stats() AdaptiveStreamingStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return AdaptiveStreamingStats{
		CurrentThreshold: c.currentThreshold,
		AllocMB:          int64(m.Alloc / 1024 / 1024),
		SysMB:            int64(m.Sys / 1024 / 1024),
		MaxMemoryMB:      c.maxMemoryMB,
		UsagePercent:     float64(m.Alloc/1024/1024) / float64(c.maxMemoryMB) * 100,
		LastUpdate:       c.lastUpdate,
	}
}

// AdaptiveStreamingStats contiene estadísticas del streaming adaptativo.
type AdaptiveStreamingStats struct {
	CurrentThreshold int
	AllocMB          int64
	SysMB            int64
	MaxMemoryMB      int64
	UsagePercent     float64
	LastUpdate       time.Time
}
