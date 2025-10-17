// internal/core/ports/exporter.go
package ports

import (
	"io"

	"aethonx/internal/core/domain"
)

// Exporter es el port para exportar resultados en diferentes formatos.
type Exporter interface {
	// Name retorna el nombre del exporter (ej: "json", "markdown", "html")
	Name() string

	// SupportedFormats retorna los formatos soportados por el exporter
	SupportedFormats() []string

	// Export exporta el resultado en el formato especificado
	Export(result *domain.ScanResult, opts ExportOptions) error
}

// StreamExporter permite exportar resultados de forma streaming.
type StreamExporter interface {
	Exporter

	// ExportStream exporta artifacts en tiempo real a medida que llegan
	ExportStream(artifacts <-chan *domain.Artifact, opts ExportOptions) error
}

// WriterExporter permite exportar a cualquier io.Writer.
type WriterExporter interface {
	Exporter

	// ExportToWriter exporta el resultado a un Writer personalizado
	ExportToWriter(result *domain.ScanResult, writer io.Writer, opts ExportOptions) error
}

// ExportOptions configura las opciones de exportación.
type ExportOptions struct {
	// OutputPath ruta donde guardar el resultado (puede ser vacío para stdout)
	OutputPath string

	// Format formato específico (json, ndjson, yaml, etc.)
	Format string

	// Pretty indica si el output debe ser formateado para legibilidad humana
	Pretty bool

	// IncludeMetadata si se debe incluir metadata del escaneo
	IncludeMetadata bool

	// IncludeWarnings si se deben incluir advertencias
	IncludeWarnings bool

	// IncludeErrors si se deben incluir errores
	IncludeErrors bool

	// FilterByType filtra artifacts por tipo (vacío = todos)
	FilterByType []domain.ArtifactType

	// MinConfidence confianza mínima para incluir artifacts (0.0-1.0)
	MinConfidence float64

	// Metadata adicional para el export
	Metadata map[string]string

	// Template ruta a template personalizado (para exporters que lo soporten)
	Template string
}

// DefaultExportOptions retorna opciones por defecto.
func DefaultExportOptions() ExportOptions {
	return ExportOptions{
		OutputPath:      "",
		Format:          "json",
		Pretty:          true,
		IncludeMetadata: true,
		IncludeWarnings: true,
		IncludeErrors:   true,
		FilterByType:    []domain.ArtifactType{},
		MinConfidence:   0.0,
		Metadata:        make(map[string]string),
		Template:        "",
	}
}

// ExporterFactory es una función que crea una instancia de Exporter.
type ExporterFactory func() (Exporter, error)
