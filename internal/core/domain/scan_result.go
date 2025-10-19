// internal/core/domain/scan_result.go
package domain

import (
	"fmt"
	"time"
)

// ScanResult representa el resultado completo de un escaneo.
type ScanResult struct {
	// ID identificador único del escaneo
	ID string

	// Target objetivo del escaneo
	Target Target

	// Artifacts datos descubiertos
	Artifacts []*Artifact

	// Metadata información sobre el escaneo
	Metadata ScanMetadata

	// Warnings advertencias no críticas durante el escaneo
	Warnings []Warning

	// Errors errores ocurridos durante el escaneo
	Errors []Error
}

// ScanMetadata contiene información sobre la ejecución del escaneo.
type ScanMetadata struct {
	// StartTime momento de inicio del escaneo
	StartTime time.Time

	// EndTime momento de finalización del escaneo
	EndTime time.Time

	// Duration duración total del escaneo
	Duration time.Duration

	// SourcesUsed lista de fuentes que fueron ejecutadas
	SourcesUsed []string

	// TotalSources número total de fuentes disponibles
	TotalSources int

	// TotalRelations número total de relaciones en el grafo
	TotalRelations int

	// RelationsByType cuenta de relaciones agrupadas por tipo
	RelationsByType map[RelationType]int

	// Version versión de AethonX utilizada
	Version string

	// Environment información del entorno (opcional)
	Environment map[string]string
}

// Warning representa una advertencia no crítica durante el escaneo.
type Warning struct {
	// Source fuente que generó la advertencia
	Source string

	// Message descripción de la advertencia
	Message string

	// Timestamp momento de la advertencia
	Timestamp time.Time

	// Context contexto adicional
	Context map[string]string
}

// Error representa un error ocurrido durante el escaneo.
type Error struct {
	// Source fuente que generó el error
	Source string

	// Message descripción del error
	Message string

	// Fatal indica si el error es fatal (detiene el escaneo)
	Fatal bool

	// Timestamp momento del error
	Timestamp time.Time

	// Context contexto adicional
	Context map[string]string
}

// NewScanResult crea un nuevo resultado de escaneo.
func NewScanResult(target Target) *ScanResult {
	return &ScanResult{
		ID:        generateScanID(),
		Target:    target,
		Artifacts: []*Artifact{},
		Metadata: ScanMetadata{
			StartTime:   time.Now(),
			Environment: make(map[string]string),
		},
		Warnings: []Warning{},
		Errors:   []Error{},
	}
}

// AddArtifact añade un artefacto al resultado.
func (r *ScanResult) AddArtifact(artifact *Artifact) {
	if artifact != nil && artifact.IsValid() {
		r.Artifacts = append(r.Artifacts, artifact)
	}
}

// AddArtifacts añade múltiples artefactos al resultado.
func (r *ScanResult) AddArtifacts(artifacts ...*Artifact) {
	for _, a := range artifacts {
		r.AddArtifact(a)
	}
}

// AddWarning añade una advertencia al resultado.
func (r *ScanResult) AddWarning(source, message string) {
	r.Warnings = append(r.Warnings, Warning{
		Source:    source,
		Message:   message,
		Timestamp: time.Now(),
		Context:   make(map[string]string),
	})
}

// AddError añade un error al resultado.
func (r *ScanResult) AddError(source, message string, fatal bool) {
	r.Errors = append(r.Errors, Error{
		Source:    source,
		Message:   message,
		Fatal:     fatal,
		Timestamp: time.Now(),
		Context:   make(map[string]string),
	})
}

// Finalize marca el escaneo como completado y calcula estadísticas finales.
func (r *ScanResult) Finalize() {
	r.Metadata.EndTime = time.Now()
	r.Metadata.Duration = r.Metadata.EndTime.Sub(r.Metadata.StartTime)
}

// Stats retorna estadísticas del escaneo agrupadas por tipo de artefacto.
func (r *ScanResult) Stats() map[string]int {
	stats := make(map[string]int)
	for _, a := range r.Artifacts {
		stats[string(a.Type)]++
	}
	return stats
}

// TotalArtifacts retorna el número total de artefactos descubiertos.
func (r *ScanResult) TotalArtifacts() int {
	return len(r.Artifacts)
}

// HasErrors indica si hubo errores durante el escaneo.
func (r *ScanResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// HasFatalErrors indica si hubo errores fatales durante el escaneo.
func (r *ScanResult) HasFatalErrors() bool {
	for _, err := range r.Errors {
		if err.Fatal {
			return true
		}
	}
	return false
}

// Summary retorna un resumen legible del resultado.
func (r *ScanResult) Summary() string {
	return fmt.Sprintf(
		"ScanResult{target=%s, artifacts=%d, warnings=%d, errors=%d, duration=%s}",
		r.Target.Root,
		len(r.Artifacts),
		len(r.Warnings),
		len(r.Errors),
		r.Metadata.Duration,
	)
}

// generateScanID genera un ID único para el escaneo basado en timestamp.
func generateScanID() string {
	return fmt.Sprintf("scan-%d", time.Now().UnixNano())
}
