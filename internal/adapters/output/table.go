// internal/adapters/output/table.go
package output

import (
	"fmt"
	"os"
	"text/tabwriter"

	"aethonx/internal/core"
)

// OutputTable imprime una tabla legible en terminal.
func OutputTable(res core.RunResult) error {
	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tVALUE\tSOURCE(S)")
	for _, a := range res.Artifacts {
		src := a.Source
		if s, ok := a.Meta["sources"]; ok && s != "" {
			src = s
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", a.Type, a.Value, src)
	}
	if err := w.Flush(); err != nil {
		return err
	}

	if len(res.Warnings) > 0 {
		fmt.Fprintln(os.Stdout, "\nWarnings:")
		for _, w := range res.Warnings {
			fmt.Fprintln(os.Stdout, " -", w)
		}
	}
	return nil
}
