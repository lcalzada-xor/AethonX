// internal/adapters/output/json.go
package output

import (
	"encoding/json"
	"os"
	"path/filepath"

	"aethonx/internal/core"
)

func OutputJSON(dir string, res core.RunResult) error {
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	fp := filepath.Join(dir, "aethonx.json")
	f, err := os.Create(fp)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}
