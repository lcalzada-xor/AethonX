// internal/platform/ui/terminal/ansi.go
package terminal

import (
	"fmt"
	"strings"
)

// ANSI Escape Codes
const (
	// Reset
	Reset = "\033[0m"

	// Cursor Control
	CursorHide     = "\033[?25l"
	CursorShow     = "\033[?25h"
	CursorSave     = "\033[s"
	CursorRestore  = "\033[u"
	ClearLine      = "\033[2K"
	ClearToLineEnd = "\033[K"

	// Colors (Foreground)
	Black   = "\033[30m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
	Gray    = "\033[90m"

	// Bright Colors
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
	BrightWhite   = "\033[97m"

	// Styles
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Italic    = "\033[3m"
	Underline = "\033[4m"
)

// MoveCursorUp mueve el cursor N líneas arriba
func MoveCursorUp(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("\033[%dA", n)
}

// MoveCursorDown mueve el cursor N líneas abajo
func MoveCursorDown(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("\033[%dB", n)
}

// MoveCursorToColumn mueve el cursor a la columna N
func MoveCursorToColumn(n int) string {
	return fmt.Sprintf("\033[%dG", n)
}

// RGB crea un color RGB (true color)
func RGB(r, g, b int) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
}

// RGBBg crea un color de fondo RGB
func RGBBg(r, g, b int) string {
	return fmt.Sprintf("\033[48;2;%d;%d;%dm", r, g, b)
}

// Colorize aplica un color a un texto
func Colorize(text, color string) string {
	return color + text + Reset
}

// Bold aplica negrita
func BoldText(text string) string {
	return Bold + text + Reset
}

// StripANSI elimina todos los códigos ANSI de un string
func StripANSI(s string) string {
	// Simple implementation - for visual length calculation
	inEscape := false
	var result strings.Builder

	for i := 0; i < len(s); i++ {
		if s[i] == '\033' {
			inEscape = true
			continue
		}

		if inEscape {
			if s[i] == 'm' || s[i] == 'A' || s[i] == 'B' || s[i] == 'G' || s[i] == 'K' {
				inEscape = false
			}
			continue
		}

		result.WriteByte(s[i])
	}

	return result.String()
}

// VisualLength calcula el largo visual de un string (sin ANSI codes)
func VisualLength(s string) int {
	return len(StripANSI(s))
}

// PadRight añade espacios a la derecha hasta width
func PadRight(s string, width int) string {
	visualLen := VisualLength(s)
	if visualLen >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visualLen)
}

// TruncateVisual trunca un string a width visual characters
func TruncateVisual(s string, width int) string {
	visualLen := VisualLength(s)
	if visualLen <= width {
		return s
	}

	// Simple truncation (doesn't preserve ANSI codes perfectly, but good enough)
	stripped := StripANSI(s)
	if len(stripped) > width-3 {
		return stripped[:width-3] + "..."
	}
	return stripped[:width]
}
