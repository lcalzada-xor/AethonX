// internal/platform/ui/presenter.go
package ui

import (
	"time"
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

	// UpdateSource actualiza el progreso de un source
	UpdateSource(sourceName string, artifactCount int)

	// FinishSource notifica la finalización de un source
	FinishSource(sourceName string, status Status, duration time.Duration, artifactCount int)

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
	TotalDuration     time.Duration
	TotalArtifacts    int
	UniqueArtifacts   int
	SourcesSucceeded  int
	SourcesFailed     int
	ArtifactsByType   map[string]int
	RelationshipsBuilt int
}

// SourceProgress representa el progreso de un source específico
type SourceProgress struct {
	Name          string
	Status        Status
	ArtifactCount int
	Duration      time.Duration
	StartTime     time.Time
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
