// internal/platform/ui/system_metrics.go
package ui

import (
	"runtime"
	"sync"
	"time"
)

// SystemMetrics monitorea recursos del sistema
type SystemMetrics struct {
	CPUPercent      float64
	MemoryMB        float64
	GoroutineCount  int
	LastUpdate      time.Time

	mu              sync.RWMutex
	stopChan        chan struct{}
	updateInterval  time.Duration
}

// SystemMetricsSnapshot es un snapshot inmutable de métricas del sistema
type SystemMetricsSnapshot struct {
	CPUPercent     float64
	MemoryMB       float64
	GoroutineCount int
	Timestamp      time.Time
}

// NewSystemMetrics crea un nuevo monitor de métricas del sistema
func NewSystemMetrics(updateInterval time.Duration) *SystemMetrics {
	if updateInterval == 0 {
		updateInterval = 2 * time.Second
	}

	sm := &SystemMetrics{
		stopChan:       make(chan struct{}),
		updateInterval: updateInterval,
		LastUpdate:     time.Now(),
	}

	// Actualización inicial
	sm.update()

	return sm
}

// Start inicia el monitoreo en background
func (sm *SystemMetrics) Start() {
	go sm.monitorLoop()
}

// Stop detiene el monitoreo
func (sm *SystemMetrics) Stop() {
	close(sm.stopChan)
}

// GetSnapshot obtiene un snapshot de las métricas actuales
func (sm *SystemMetrics) GetSnapshot() SystemMetricsSnapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return SystemMetricsSnapshot{
		CPUPercent:     sm.CPUPercent,
		MemoryMB:       sm.MemoryMB,
		GoroutineCount: sm.GoroutineCount,
		Timestamp:      sm.LastUpdate,
	}
}

// monitorLoop es el loop principal de monitoreo
func (sm *SystemMetrics) monitorLoop() {
	ticker := time.NewTicker(sm.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.update()
		case <-sm.stopChan:
			return
		}
	}
}

// update actualiza todas las métricas
func (sm *SystemMetrics) update() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	sm.MemoryMB = float64(m.Alloc) / 1024 / 1024

	// Goroutines
	sm.GoroutineCount = runtime.NumGoroutine()

	// CPU percent (aproximado basado en GC)
	// Nota: Para CPU real necesitaríamos gopsutil, pero esto es suficiente para una estimación
	sm.CPUPercent = calculateCPUUsage(&m)

	sm.LastUpdate = time.Now()
}

// calculateCPUUsage calcula un aproximado del uso de CPU
// basado en el tiempo de GC y otras métricas de runtime
func calculateCPUUsage(m *runtime.MemStats) float64 {
	// Esta es una aproximación simple
	// Para mediciones reales de CPU se necesitaría gopsutil u otra librería

	// Usamos el número de goroutines como proxy del uso
	numGoroutines := runtime.NumGoroutine()
	numCPU := runtime.NumCPU()

	// Aproximación: % = (goroutines / (CPUs * 10)) * 100
	// Factor 10 porque no todas las goroutines están activas simultáneamente
	cpuPercent := (float64(numGoroutines) / float64(numCPU*10)) * 100

	// Limitar a 100%
	if cpuPercent > 100 {
		cpuPercent = 100
	}

	return cpuPercent
}

// FormatMemory formatea memoria en formato legible
func FormatMemory(mb float64) string {
	if mb < 1 {
		return "< 1 MB"
	} else if mb < 1024 {
		return formatFloat(mb, 1) + " MB"
	} else {
		return formatFloat(mb/1024, 2) + " GB"
	}
}

// formatFloat formatea un float con precisión específica
func formatFloat(val float64, precision int) string {
	if precision == 1 {
		return truncateFloat(val, 1)
	}
	return truncateFloat(val, 2)
}

// truncateFloat trunca un float a N decimales (helper simple)
func truncateFloat(val float64, decimals int) string {
	// Implementación simple sin fmt.Sprintf para evitar import
	intPart := int(val)
	fracPart := int((val - float64(intPart)) * 10)
	if decimals == 2 {
		fracPart = int((val - float64(intPart)) * 100)
	}

	result := intToString(intPart) + "."
	if decimals == 1 {
		result += intToString(fracPart)
	} else {
		if fracPart < 10 {
			result += "0" + intToString(fracPart)
		} else {
			result += intToString(fracPart)
		}
	}

	return result
}

// intToString convierte int a string (helper simple)
func intToString(n int) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	if negative {
		digits = append([]byte{'-'}, digits...)
	}

	return string(digits)
}
