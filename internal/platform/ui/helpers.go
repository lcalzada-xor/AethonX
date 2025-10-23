// internal/platform/ui/helpers.go
package ui

import (
	"fmt"
	"time"
)

// formatDuration formatea una duración de manera legible
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	} else if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
}

// boolToString convierte booleano a string visual
func boolToString(b bool) string {
	if b {
		return StyleSuccess.Sprint("ON")
	}
	return StyleSecondary.Sprint("OFF")
}
