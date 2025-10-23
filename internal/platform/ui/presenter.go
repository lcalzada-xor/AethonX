// internal/platform/ui/presenter.go
package ui

import (
	"time"
)

// UIMode define el modo de visualización
type UIMode string

const (
	UIModeCompact   UIMode = "compact"   // Barras de progreso compactas (default)
	UIModeDashboard UIMode = "dashboard" // Dashboard completo con paneles
	UIModeMinimal   UIMode = "minimal"   // Solo símbolos y contadores
	UIModeQuiet     UIMode = "quiet"     // Sin UI visual
)

// Presenter define la interfaz para presentar el progreso de la ejecución
// del pipeline de reconocimiento de manera visual e interactiva.
type Presenter interface {
	// Start inicia la presentación con información del escaneo
	Start(info ScanInfo)

	// StartStage notifica el inicio de un nuevo stage
	StartStage(stage StageInfo)

	// FinishStage notifica la finalización de un stage
	FinishStage(stageNum int, duration time.Duration)

	// StartSource notifica el inicio de ejecución de un source
	StartSource(stageNum int, sourceName string)

	// UpdateSource actualiza el progreso de un source con métricas completas
	UpdateSource(sourceName string, metrics ProgressMetrics)

	// UpdateSourcePhase actualiza solo la fase de un source
	UpdateSourcePhase(sourceName string, phase string)

	// FinishSource notifica la finalización de un source
	FinishSource(sourceName string, status Status, duration time.Duration, artifactCount int)

	// UpdateDiscoveries actualiza contadores de artifacts por tipo en tiempo real
	UpdateDiscoveries(discoveries DiscoveryStats)

	// Info muestra un mensaje informativo
	Info(msg string)

	// Warning muestra una advertencia
	Warning(msg string)

	// Error muestra un error
	Error(msg string)

	// Finish finaliza la presentación con estadísticas finales
	Finish(stats ScanStats)

	// Close limpia recursos del presenter
	Close() error
}

// ScanInfo contiene información inicial del escaneo
type ScanInfo struct {
	Target         string
	Mode           string
	Workers        int
	TimeoutSeconds int
	StreamingOn    bool
	TotalStages    int
	UIMode         UIMode
	ShowMetrics    bool
	ShowPhases     bool
}

// StageInfo contiene información de un stage
type StageInfo struct {
	Number      int
	TotalStages int
	Name        string
	Sources     []string
}

// ScanStats contiene estadísticas finales del escaneo
type ScanStats struct {
	TotalDuration      time.Duration
	TotalArtifacts     int
	UniqueArtifacts    int
	SourcesSucceeded   int
	SourcesFailed      int
	ArtifactsByType    map[string]int
	RelationshipsBuilt int
}

// DiscoveryStats contiene estadísticas de descubrimiento en tiempo real
type DiscoveryStats struct {
	Subdomains int
	IPs        int
	URLs       int
	Emails     int
	Ports      int
	Total      int
	Unique     int
}

// SourceProgress representa el progreso de un source específico
type SourceProgress struct {
	Name          string
	Status        Status
	ArtifactCount int
	Duration      time.Duration
	StartTime     time.Time
	Metrics       *ProgressMetrics
}

// StageProgress representa el progreso de un stage completo
type StageProgress struct {
	Number    int
	Name      string
	Status    Status
	Sources   map[string]*SourceProgress
	StartTime time.Time
	Duration  time.Duration
}
