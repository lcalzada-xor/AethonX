// internal/adapters/output/json.go
package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"aethonx/internal/core/domain"
)

// OutputJSON exporta el resultado en formato JSON.
func OutputJSON(dir string, result *domain.ScanResult) error {
	if dir == "" {
		dir = "."
	}

	// Crear directorio si no existe
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generar nombre de archivo con timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("aethonx_%s_%s.json", result.Target.Root, timestamp)
	filepath := filepath.Join(dir, filename)

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
