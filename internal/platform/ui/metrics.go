// internal/platform/ui/metrics.go
package ui

import (
	"sync"
	"time"
)

// MetricsCollector recopila y calcula métricas en tiempo real
type MetricsCollector struct {
	sources map[string]*SourceMetrics
	global  *GlobalMetrics
	mu      sync.RWMutex
}

// NewMetricsCollector crea una nueva instancia del collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		sources: make(map[string]*SourceMetrics),
		global: &GlobalMetrics{
			StartTime: time.Now(),
		},
	}
}

// SourceMetrics almacena métricas de una fuente individual
type SourceMetrics struct {
	Name          string
	StartTime     time.Time
	LastUpdate    time.Time
	ArtifactCount int
	RequestCount  int
	ErrorCount    int
	Phase         string

	// Para calcular rates
	lastRateCheck    time.Time
	lastArtifactCount int
	currentRate      float64

	// Progress tracking
	CurrentProgress  int
	TotalProgress    int // 0 = indeterminado
	EstimatedTimeLeft time.Duration
}

// GlobalMetrics almacena métricas globales del scan
type GlobalMetrics struct {
	StartTime         time.Time
	TotalArtifacts    int
	UniqueArtifacts   int
	TotalRequests     int
	TotalErrors       int
	ArtifactsByType   map[string]int

	// Rates calculados
	OverallRate       float64
	SuccessRate       float64
}

// ProgressMetrics representa métricas de progreso para UI
type ProgressMetrics struct {
	Current       int
	Total         int       // 0 = indeterminado
	Percentage    float64
	Rate          float64   // Artifacts/segundo
	EstimatedTime time.Duration
	Phase         string
	Latency       time.Duration
}

// RegisterSource registra una nueva fuente para tracking
func (mc *MetricsCollector) RegisterSource(name string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()
	mc.sources[name] = &SourceMetrics{
		Name:             name,
		StartTime:        now,
		LastUpdate:       now,
		lastRateCheck:    now,
		lastArtifactCount: 0,
		currentRate:      0,
	}
}

// UpdateSource actualiza las métricas de una fuente
func (mc *MetricsCollector) UpdateSource(name string, artifactCount int, phase string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	src, exists := mc.sources[name]
	if !exists {
		return
	}

	now := time.Now()
	src.LastUpdate = now
	src.ArtifactCount = artifactCount
	src.Phase = phase

	// Calcular rate (artifacts/segundo)
	timeSinceLastCheck := now.Sub(src.lastRateCheck).Seconds()
	if timeSinceLastCheck >= 1.0 { // Actualizar rate cada segundo
		artifactsDiff := artifactCount - src.lastArtifactCount
		src.currentRate = float64(artifactsDiff) / timeSinceLastCheck
		src.lastRateCheck = now
		src.lastArtifactCount = artifactCount
	}

	// Calcular ETA si tenemos total
	if src.TotalProgress > 0 && src.currentRate > 0 {
		remaining := src.TotalProgress - src.CurrentProgress
		src.EstimatedTimeLeft = time.Duration(float64(remaining)/src.currentRate) * time.Second
	}
}

// UpdateSourceProgress actualiza el progreso de una fuente
func (mc *MetricsCollector) UpdateSourceProgress(name string, current, total int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	src, exists := mc.sources[name]
	if !exists {
		return
	}

	src.CurrentProgress = current
	src.TotalProgress = total
}

// IncrementSourceRequests incrementa el contador de requests
func (mc *MetricsCollector) IncrementSourceRequests(name string, count int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if src, exists := mc.sources[name]; exists {
		src.RequestCount += count
	}
}

// IncrementSourceErrors incrementa el contador de errores
func (mc *MetricsCollector) IncrementSourceErrors(name string, count int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if src, exists := mc.sources[name]; exists {
		src.ErrorCount += count
	}
}

// GetSourceMetrics obtiene las métricas de una fuente
func (mc *MetricsCollector) GetSourceMetrics(name string) *SourceMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if src, exists := mc.sources[name]; exists {
		// Retornar copia para evitar race conditions
		copy := *src
		return &copy
	}
	return nil
}

// GetAllSourceMetrics obtiene todas las métricas de fuentes
func (mc *MetricsCollector) GetAllSourceMetrics() map[string]*SourceMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make(map[string]*SourceMetrics, len(mc.sources))
	for name, src := range mc.sources {
		copy := *src
		result[name] = &copy
	}
	return result
}

// UpdateGlobalMetrics actualiza las métricas globales
func (mc *MetricsCollector) UpdateGlobalMetrics(total, unique int, byType map[string]int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.global.TotalArtifacts = total
	mc.global.UniqueArtifacts = unique
	mc.global.ArtifactsByType = byType

	// Calcular rate global
	elapsed := time.Since(mc.global.StartTime).Seconds()
	if elapsed > 0 {
		mc.global.OverallRate = float64(total) / elapsed
	}

	// Calcular success rate
	totalRequests := 0
	totalErrors := 0
	for _, src := range mc.sources {
		totalRequests += src.RequestCount
		totalErrors += src.ErrorCount
	}
	mc.global.TotalRequests = totalRequests
	mc.global.TotalErrors = totalErrors

	if totalRequests > 0 {
		mc.global.SuccessRate = float64(totalRequests-totalErrors) / float64(totalRequests) * 100
	}
}

// GetGlobalMetrics obtiene las métricas globales
func (mc *MetricsCollector) GetGlobalMetrics() *GlobalMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	copy := *mc.global
	if mc.global.ArtifactsByType != nil {
		copy.ArtifactsByType = make(map[string]int, len(mc.global.ArtifactsByType))
		for k, v := range mc.global.ArtifactsByType {
			copy.ArtifactsByType[k] = v
		}
	}
	return &copy
}

// GetProgressMetrics obtiene métricas de progreso para UI
func (mc *MetricsCollector) GetProgressMetrics(name string) ProgressMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	src, exists := mc.sources[name]
	if !exists {
		return ProgressMetrics{}
	}

	metrics := ProgressMetrics{
		Current:       src.CurrentProgress,
		Total:         src.TotalProgress,
		Rate:          src.currentRate,
		EstimatedTime: src.EstimatedTimeLeft,
		Phase:         src.Phase,
		Latency:       time.Since(src.LastUpdate),
	}

	// Calcular porcentaje
	if src.TotalProgress > 0 {
		metrics.Percentage = float64(src.CurrentProgress) / float64(src.TotalProgress) * 100
	} else if src.ArtifactCount > 0 {
		// Para sources indeterminados, mostrar progreso basado en artifacts
		metrics.Current = src.ArtifactCount
		metrics.Percentage = -1 // Indica indeterminado
	}

	return metrics
}

// Reset reinicia todas las métricas
func (mc *MetricsCollector) Reset() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.sources = make(map[string]*SourceMetrics)
	mc.global = &GlobalMetrics{
		StartTime: time.Now(),
	}
}
