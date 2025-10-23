// internal/platform/ui/symbols.go
package ui

import "github.com/pterm/pterm"

// Status representa el estado de un source o stage
type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusSuccess
	StatusWarning
	StatusError
	StatusSkipped
)

// String convierte el status a string
func (s Status) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusSuccess:
		return "success"
	case StatusWarning:
		return "warning"
	case StatusError:
		return "error"
	case StatusSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

// Symbol retorna el símbolo Unicode para cada estado (temática infernal)
func (s Status) Symbol() string {
	switch s {
	case StatusPending:
		return "○" // Luna nueva (oscuridad, esperando)
	case StatusRunning:
		return "◉" // Brasa ardiendo (será animado)
	case StatusSuccess:
		return "⚡" // Rayo de iluminación
	case StatusWarning:
		return "🔶" // Advertencia flamígera
	case StatusError:
		return "✖" // Cruz de muerte
	case StatusSkipped:
		return "〰" // Río Aqueronte (omitido)
	default:
		return "?"
	}
}

// Style retorna un pterm.RGBStyle configurado para el estado
func (s Status) Style() pterm.RGBStyle {
	switch s {
	case StatusPending:
		return StyleSecondary
	case StatusRunning:
		return StyleActive
	case StatusSuccess:
		return StyleSuccess
	case StatusWarning:
		return StyleWarning
	case StatusError:
		return StyleError
	case StatusSkipped:
		return StyleSecondary
	default:
		return StyleText
	}
}

// Iconos temáticos - Mitología griega + infierno
// Usando Unicode + algunos emojis seguros para máxima compatibilidad
var (
	// Elementos de navegación y estructura
	IconTarget    = "►" // Apuntando al objetivo (Unicode seguro)
	IconStage     = "◈" // Diamante de etapa (Unicode)
	IconInfo      = "ℹ" // Información (Unicode seguro)
	IconWarning   = "⚠" // Advertencia (Unicode universal)
	IconError     = "✖" // Error crítico (Unicode seguro)
	IconSuccess   = "✓" // Éxito (Unicode universal)
	IconStats     = "≡" // Estadísticas (Unicode barras)
	IconTime      = "◷" // Tiempo (Unicode reloj)
	IconArtifacts = "◆" // Tesoros/diamante (Unicode seguro)
	IconSources   = "◉" // Fuentes (Unicode círculo)
	IconWorkers   = "≣" // Workers/procesos (Unicode)
	IconMode      = "⊕" // Modo (Unicode circled plus)
)

// Separadores temáticos con caracteres Unicode dobles
var (
	// SeparatorHeavy - Separador principal (estilo infernal)
	SeparatorHeavy = "════════════════════════════════════════════════════════════"

	// SeparatorLight - Separador secundario
	SeparatorLight = "────────────────────────────────────────────────────────────"

	// SeparatorFlame - Separador con efecto de llama
	SeparatorFlame = "▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰"
)

// Caracteres de borde para boxes (estilo double-line Unicode)
var (
	BorderTopLeft     = "╔"
	BorderTopRight    = "╗"
	BorderBottomLeft  = "╚"
	BorderBottomRight = "╝"
	BorderHorizontal  = "═"
	BorderVertical    = "║"
	BorderTeeDown     = "╦"
	BorderTeeUp       = "╩"
	BorderTeeRight    = "╠"
	BorderTeeLeft     = "╣"
	BorderCross       = "╬"
)

// Progress bar characters
var (
	ProgressFull  = "█"
	ProgressEmpty = "░"
	ProgressTip   = "▶"
)

// Spinner sequences temáticas
var SpinnerSequences = map[string][]string{
	"ember":   {"◉", "◎", "○", "◎"},                     // Brasas pulsantes (default)
	"flame":   {"▰", "▱", "▰", "▱"},                     // Llama oscilante
	"pulse":   {"●", "◉", "○", "◉"},                     // Pulso
	"scroll":  {"◐", "◓", "◑", "◒"},                     // Pergamino girando
}

// GetSpinnerSequence obtiene la secuencia de spinner por nombre
func GetSpinnerSequence(name string) []string {
	if seq, exists := SpinnerSequences[name]; exists {
		return seq
	}
	return SpinnerSequences["ember"] // Default
}
