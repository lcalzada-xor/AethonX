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

// Symbol retorna el símbolo Unicode para cada estado
func (s Status) Symbol() string {
	switch s {
	case StatusPending:
		return "⏸"
	case StatusRunning:
		return "⣾"
	case StatusSuccess:
		return "✓"
	case StatusWarning:
		return "⚠"
	case StatusError:
		return "✗"
	case StatusSkipped:
		return "⊘"
	default:
		return "?"
	}
}

// Color retorna el color pterm para cada estado
func (s Status) Color() pterm.Color {
	switch s {
	case StatusPending:
		return pterm.FgGray
	case StatusRunning:
		return pterm.FgCyan
	case StatusSuccess:
		return pterm.FgGreen
	case StatusWarning:
		return pterm.FgYellow
	case StatusError:
		return pterm.FgRed
	case StatusSkipped:
		return pterm.FgGray
	default:
		return pterm.FgDefault
	}
}

// Style retorna un pterm.Style configurado para el estado
func (s Status) Style() *pterm.Style {
	return pterm.NewStyle(s.Color())
}

// Icons globales para diferentes elementos de la UI
var (
	IconTarget    = "🎯"
	IconStage     = "🔄"
	IconInfo      = "ℹ"
	IconWarning   = "⚠"
	IconError     = "✗"
	IconSuccess   = "✓"
	IconStats     = "📊"
	IconTime      = "⏱"
	IconArtifacts = "📦"
	IconSources   = "🔌"
	IconWorkers   = "⚙️"
)

// Separadores y bordes
var (
	SeparatorHeavy = "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	SeparatorLight = "────────────────────────────────────────────"
)
