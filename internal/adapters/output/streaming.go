// internal/adapters/output/streaming.go
package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
)

// sanitizeDomainNameForStreaming convierte un nombre de dominio en un nombre de carpeta válido.
// Ejemplo: "example.com" -> "example_com"
func sanitizeDomainNameForStreaming(domain string) string {
	// Reemplazar puntos por guiones bajos
	sanitized := strings.ReplaceAll(domain, ".", "_")
	// Remover cualquier otro carácter que no sea alfanumérico, guión bajo o guión
	sanitized = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, sanitized)
	return sanitized
}

// StreamingWriter maneja la escritura incremental de resultados parciales por source.
// Cada source que completa su ejecución escribe un archivo JSON parcial,
// permitiendo liberar memoria inmediatamente.
type StreamingWriter struct {
	baseDir    string
	scanID     string
	targetRoot string
	timestamp  string
	logger     logx.Logger
}

// NewStreamingWriter crea un nuevo writer de streaming.
func NewStreamingWriter(baseDir, scanID, targetRoot string, logger logx.Logger) *StreamingWriter {
	return &StreamingWriter{
		baseDir:    baseDir,
		scanID:     scanID,
		targetRoot: targetRoot,
		timestamp:  time.Now().Format("20060102_150405"),
		logger:     logger.With("component", "streaming-writer"),
	}
}

// WritePartial escribe un resultado parcial de una source a disco.
// Formato: aethonx_{target}_{timestamp}_partial_{source}.json
func (w *StreamingWriter) WritePartial(sourceName string, result *domain.ScanResult) (string, error) {
	// Crear subdirectorio específico para el dominio
	domainDir := sanitizeDomainNameForStreaming(w.targetRoot)
	fullDir := filepath.Join(w.baseDir, domainDir)

	// Asegurar que el directorio completo existe
	if err := os.MkdirAll(fullDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generar nombre de archivo parcial
	filename := w.GeneratePartialFilename(sourceName)
	filepath := filepath.Join(fullDir, filename)

	// Crear archivo
	f, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to create partial file: %w", err)
	}
	defer f.Close()

	// Estructura de datos para archivo parcial
	partialData := PartialScanResult{
		Source:       sourceName,
		ScanID:       w.scanID,
		Target:       result.Target.Root,
		Artifacts:    result.Artifacts,
		Warnings:     result.Warnings,
		Errors:       result.Errors,
		WrittenAt:    time.Now(),
		ArtifactCount: len(result.Artifacts),
	}

	// Codificar JSON con indentación
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(partialData); err != nil {
		return "", fmt.Errorf("failed to encode partial JSON: %w", err)
	}

	w.logger.Debug("partial result written",
		"source", sourceName,
		"artifacts", len(result.Artifacts),
		"file", filename,
	)

	return filepath, nil
}

// GeneratePartialFilename genera el nombre de archivo para un resultado parcial.
func (w *StreamingWriter) GeneratePartialFilename(sourceName string) string {
	return fmt.Sprintf("aethonx_%s_%s_partial_%s.json",
		w.targetRoot,
		w.timestamp,
		sourceName,
	)
}

// GetPattern retorna el patrón glob para encontrar archivos parciales de este scan.
func (w *StreamingWriter) GetPattern() string {
	return fmt.Sprintf("aethonx_%s_%s_partial_*.json", w.targetRoot, w.timestamp)
}

// GetFinalFilename retorna el nombre del archivo final consolidado.
func (w *StreamingWriter) GetFinalFilename() string {
	return fmt.Sprintf("aethonx_%s_%s.json", w.targetRoot, w.timestamp)
}

// PartialScanResult representa un resultado parcial de una source individual.
type PartialScanResult struct {
	Source        string             `json:"source"`
	ScanID        string             `json:"scan_id"`
	Target        string             `json:"target"`
	Artifacts     []*domain.Artifact `json:"artifacts"`
	Warnings      []domain.Warning   `json:"warnings"`
	Errors        []domain.Error     `json:"errors"`
	WrittenAt     time.Time          `json:"written_at"`
	ArtifactCount int                `json:"artifact_count"`
}
