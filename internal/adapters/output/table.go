// internal/adapters/output/table.go
package output

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"aethonx/internal/core/domain"
)

// OutputTable imprime una tabla legible en terminal.
func OutputTable(result *domain.ScanResult) error {
	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)

	// Header con información del scan
	fmt.Fprintf(w, "\n=== AethonX Scan Results ===\n")
	fmt.Fprintf(w, "Target:\t%s\n", result.Target.Root)
	fmt.Fprintf(w, "Mode:\t%s\n", result.Target.Mode)
	fmt.Fprintf(w, "Duration:\t%s\n", result.Metadata.Duration)
	fmt.Fprintf(w, "Artifacts:\t%d\n", len(result.Artifacts))
	fmt.Fprintf(w, "Sources:\t%s\n\n", strings.Join(result.Metadata.SourcesUsed, ", "))

	// Tabla de artifacts
	if len(result.Artifacts) > 0 {
		fmt.Fprintln(w, "TYPE\tVALUE\tSOURCES\tCONFIDENCE")
		fmt.Fprintln(w, "----\t-----\t-------\t----------")

		for _, a := range result.Artifacts {
			sources := strings.Join(a.Sources, ",")
			confidence := fmt.Sprintf("%.2f", a.Confidence)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				a.Type,
				a.Value,
				sources,
				confidence,
			)
		}
	} else {
		fmt.Fprintln(w, "No artifacts discovered.")
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush table: %w", err)
	}

	// Warnings
	if len(result.Warnings) > 0 {
		fmt.Fprintf(os.Stdout, "\n⚠️  Warnings (%d):\n", len(result.Warnings))
		for i, warning := range result.Warnings {
			fmt.Fprintf(os.Stdout, "  %d. [%s] %s\n", i+1, warning.Source, warning.Message)
		}
	}

	// Errors
	if len(result.Errors) > 0 {
		fmt.Fprintf(os.Stdout, "\n❌ Errors (%d):\n", len(result.Errors))
		for i, err := range result.Errors {
			fatal := ""
			if err.Fatal {
				fatal = " (FATAL)"
			}
			fmt.Fprintf(os.Stdout, "  %d. [%s] %s%s\n", i+1, err.Source, err.Message, fatal)
		}
	}

	// Stats summary
	if len(result.Artifacts) > 0 {
		fmt.Fprintln(os.Stdout, "\n📊 Statistics by Type:")
		stats := result.Stats()
		for artifactType, count := range stats {
			fmt.Fprintf(os.Stdout, "  - %s: %d\n", artifactType, count)
		}
	}

	fmt.Fprintln(os.Stdout)
	return nil
}
