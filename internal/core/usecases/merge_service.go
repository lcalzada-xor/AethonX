// internal/core/usecases/merge_service.go
package usecases

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
)

// MergeService consolida resultados parciales de múltiples archivos JSON.
// Lee archivos *_partial_*.json y combina sus artifacts, warnings y errors.
type MergeService struct {
	logger logx.Logger
}

// NewMergeService crea una nueva instancia del servicio de merge.
func NewMergeService(logger logx.Logger) *MergeService {
	return &MergeService{
		logger: logger.With("component", "merge-service"),
	}
}

// PartialScanResult representa un resultado parcial de una source.
// Debe coincidir con output.PartialScanResult pero está aquí para evitar
// dependencia circular (usecases no puede importar adapters).
type PartialScanResult struct {
	Source        string             `json:"source"`
	ScanID        string             `json:"scan_id"`
	Target        string             `json:"target"`
	Artifacts     []*domain.Artifact `json:"artifacts"`
	Warnings      []domain.Warning   `json:"warnings"`
	Errors        []domain.Error     `json:"errors"`
	ArtifactCount int                `json:"artifact_count"`
}

// LoadPartialResults carga todos los resultados parciales que coincidan con el patrón.
func (m *MergeService) LoadPartialResults(dir, pattern string) ([]PartialScanResult, error) {
	// Construir patrón completo
	fullPattern := filepath.Join(dir, pattern)

	// Buscar archivos que coincidan
	files, err := filepath.Glob(fullPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob pattern %s: %w", fullPattern, err)
	}

	if len(files) == 0 {
		m.logger.Debug("no partial files found", "pattern", fullPattern)
		return []PartialScanResult{}, nil
	}

	m.logger.Info("loading partial results", "files", len(files), "pattern", pattern)

	// Cargar cada archivo
	results := make([]PartialScanResult, 0, len(files))
	for _, file := range files {
		partial, err := m.loadPartialFile(file)
		if err != nil {
			m.logger.Warn("failed to load partial file", "file", file, "error", err.Error())
			continue
		}
		results = append(results, partial)
	}

	totalArtifacts := 0
	for _, r := range results {
		totalArtifacts += len(r.Artifacts)
	}

	m.logger.Info("partial results loaded",
		"sources", len(results),
		"total_artifacts", totalArtifacts,
	)

	return results, nil
}

// loadPartialFile carga un archivo parcial individual.
func (m *MergeService) loadPartialFile(filepath string) (PartialScanResult, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return PartialScanResult{}, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	var partial PartialScanResult
	dec := json.NewDecoder(f)
	if err := dec.Decode(&partial); err != nil {
		return PartialScanResult{}, fmt.Errorf("failed to decode JSON: %w", err)
	}

	m.logger.Debug("partial file loaded",
		"source", partial.Source,
		"artifacts", len(partial.Artifacts),
	)

	return partial, nil
}

// ConsolidateIntoResult consolida resultados parciales en un ScanResult.
func (m *MergeService) ConsolidateIntoResult(
	result *domain.ScanResult,
	partials []PartialScanResult,
) {
	for _, partial := range partials {
		// Añadir artifacts
		result.Artifacts = append(result.Artifacts, partial.Artifacts...)

		// Añadir warnings
		result.Warnings = append(result.Warnings, partial.Warnings...)

		// Añadir errors
		result.Errors = append(result.Errors, partial.Errors...)

		m.logger.Debug("consolidated partial result",
			"source", partial.Source,
			"artifacts", len(partial.Artifacts),
		)
	}
}

// ClearPartialFiles elimina archivos parciales después del merge exitoso.
func (m *MergeService) ClearPartialFiles(dir, pattern string) error {
	fullPattern := filepath.Join(dir, pattern)

	files, err := filepath.Glob(fullPattern)
	if err != nil {
		return fmt.Errorf("failed to glob pattern %s: %w", fullPattern, err)
	}

	if len(files) == 0 {
		return nil
	}

	m.logger.Info("clearing partial files", "count", len(files))

	for _, file := range files {
		if err := os.Remove(file); err != nil {
			m.logger.Warn("failed to remove partial file", "file", file, "error", err.Error())
			// Continuar eliminando otros archivos
		}
	}

	return nil
}
