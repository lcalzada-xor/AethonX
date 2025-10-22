// internal/adapters/output/json.go
package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aethonx/internal/core/domain"
)

// sanitizeDomainName convierte un nombre de dominio en un nombre de carpeta válido.
// Ejemplo: "example.com" -> "example_com"
func sanitizeDomainName(domain string) string {
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

// OutputJSON exporta el resultado en formato JSON.
func OutputJSON(dir string, result *domain.ScanResult) error {
	if dir == "" {
		dir = "."
	}

	// Crear subdirectorio específico para el dominio
	domainDir := sanitizeDomainName(result.Target.Root)
	fullDir := filepath.Join(dir, domainDir)

	// Crear directorio completo si no existe
	if err := os.MkdirAll(fullDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generar nombre de archivo con timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("aethonx_%s_%s.json", result.Target.Root, timestamp)
	filepath := filepath.Join(fullDir, filename)

	// Crear archivo
	f, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	// Codificar JSON con indentación
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// OutputJSONStdout exporta el resultado a stdout en formato JSON.
func OutputJSONStdout(result *domain.ScanResult, pretty bool) error {
	enc := json.NewEncoder(os.Stdout)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(result)
}

// GraphSummary representa un resumen del grafo de relaciones.
type GraphSummary struct {
	TotalArtifacts  int                       `json:"total_artifacts"`
	TotalRelations  int                       `json:"total_relations"`
	RelationsByType map[string]int            `json:"relations_by_type"`
	ArtifactsByType map[string]int            `json:"artifacts_by_type"`
	Timestamp       time.Time                 `json:"timestamp"`
	Target          string                    `json:"target"`
}

// BuildGraphSummary construye un resumen del grafo desde un ScanResult.
func BuildGraphSummary(result *domain.ScanResult) GraphSummary {
	// Convertir RelationsByType de domain.RelationType a string para JSON
	relationsByTypeStr := make(map[string]int)
	for relType, count := range result.Metadata.RelationsByType {
		relationsByTypeStr[string(relType)] = count
	}

	// Contar artifacts por tipo
	artifactsByType := result.Stats()

	return GraphSummary{
		TotalArtifacts:  len(result.Artifacts),
		TotalRelations:  result.Metadata.TotalRelations,
		RelationsByType: relationsByTypeStr,
		ArtifactsByType: artifactsByType,
		Timestamp:       result.Metadata.EndTime,
		Target:          result.Target.Root,
	}
}
