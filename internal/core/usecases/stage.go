// internal/core/usecases/stage.go
package usecases

import (
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/ui"
)

// Stage representa una etapa de ejecución en el pipeline.
// Un stage agrupa sources que pueden ejecutarse concurrentemente porque:
// - No tienen dependencias mutuas entre sí
// - Comparten el mismo nivel de dependencia respecto a stages anteriores
type Stage struct {
	// ID único del stage (0 = primer stage, 1 = segundo, etc.)
	ID int

	// Name nombre descriptivo del stage
	Name string

	// Sources lista de sources que se ejecutan en este stage
	Sources []ports.Source

	// Level indica el nivel de profundidad en el grafo de dependencias
	// (mismo concepto que ID, pero más semántico)
	Level int
}

// StageResult encapsula el resultado de ejecución de un stage completo.
type StageResult struct {
	// StageID identifica el stage ejecutado
	StageID int

	// StageName nombre del stage
	StageName string

	// SourceResults resultados individuales de cada source
	SourceResults []SourceExecutionResult

	// ConsolidatedResult artifacts consolidados y deduplicados del stage
	ConsolidatedResult *domain.ScanResult

	// Duration tiempo total de ejecución del stage
	Duration time.Duration

	// Errors errores críticos que ocurrieron durante el stage
	Errors []error

	// Warnings advertencias no críticas
	Warnings []string

	// StreamedToDisk indica si los resultados fueron escritos a disco
	StreamedToDisk bool
}

// SourceExecutionResult resultado de ejecución de una source individual.
type SourceExecutionResult struct {
	// SourceName nombre de la source
	SourceName string

	// Result resultado de la source (nil si hubo error)
	Result *domain.ScanResult

	// Error error de ejecución (nil si exitoso)
	Error error

	// Duration tiempo de ejecución
	Duration time.Duration

	// ArtifactCount número de artifacts producidos
	ArtifactCount int

	// StreamedToDisk indica si el resultado fue escrito a disco
	StreamedToDisk bool

	// Summary resumen informativo del resultado para UI
	Summary *ui.SourceSummary
}

// NewStage crea un nuevo stage.
func NewStage(id int, sources []ports.Source) *Stage {
	return &Stage{
		ID:      id,
		Name:    inferStageName(id, sources),
		Sources: sources,
		Level:   id,
	}
}

// inferStageName infiere un nombre descriptivo para el stage basado en sus sources.
func inferStageName(id int, sources []ports.Source) string {
	if len(sources) == 0 {
		return "Empty Stage"
	}

	// Analizar patrones comunes
	allPassive := true
	allActive := true
	hasAPI := false
	hasCLI := false

	for _, s := range sources {
		mode := s.Mode()
		if mode == domain.SourceModeActive {
			allPassive = false
		}
		if mode == domain.SourceModePassive {
			allActive = false
		}

		sourceType := s.Type()
		if sourceType == domain.SourceTypeAPI {
			hasAPI = true
		}
		if sourceType == domain.SourceTypeCLI {
			hasCLI = true
		}
	}

	// Inferir nombre basado en características
	if id == 0 {
		if allPassive {
			return "Surface Discovery"
		}
		return "Initial Discovery"
	}

	if allActive && hasAPI {
		return "Service Profiling"
	}

	if allActive && hasCLI {
		return "Deep Scanning"
	}

	if !allPassive && !allActive {
		return "Hybrid Enumeration"
	}

	// Fallback genérico
	return "Stage " + string(rune('0'+id))
}

// IsEmpty retorna true si el stage no tiene sources.
func (s *Stage) IsEmpty() bool {
	return len(s.Sources) == 0
}

// SourceCount retorna el número de sources en el stage.
func (s *Stage) SourceCount() int {
	return len(s.Sources)
}

// HasErrors retorna true si el StageResult contiene errores críticos.
func (sr *StageResult) HasErrors() bool {
	return len(sr.Errors) > 0
}

// SuccessfulSources retorna el número de sources que se ejecutaron exitosamente.
func (sr *StageResult) SuccessfulSources() int {
	count := 0
	for _, result := range sr.SourceResults {
		if result.Error == nil {
			count++
		}
	}
	return count
}

// FailedSources retorna el número de sources que fallaron.
func (sr *StageResult) FailedSources() int {
	return len(sr.SourceResults) - sr.SuccessfulSources()
}

// TotalArtifacts retorna el número total de artifacts producidos por el stage.
func (sr *StageResult) TotalArtifacts() int {
	if sr.ConsolidatedResult != nil {
		return len(sr.ConsolidatedResult.Artifacts)
	}
	return 0
}
