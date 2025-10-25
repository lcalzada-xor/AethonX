// internal/platform/ui/terminal/progress.go
package terminal

import (
	"fmt"
	"strings"
	"time"
)

// ProgressBar representa una barra de progreso custom
type ProgressBar struct {
	title       string
	current     int
	total       int
	width       int
	spinner     *AnimatedSpinner
	startTime   time.Time
	completed   bool

	// Visual customization
	fillChar    string
	emptyChar   string
	titleColor  string
	barColor    string
}

// NewProgressBar crea una nueva progress bar
func NewProgressBar(title string, total int) *ProgressBar {
	return &ProgressBar{
		title:      title,
		current:    0,
		total:      total,
		width:      40,
		startTime:  time.Now(),
		completed:  false,
		fillChar:   "█",
		emptyChar:  "░",
		titleColor: BrightRed,
		barColor:   BrightRed,
	}
}

// SetSpinner asigna un spinner animado a la progress bar
func (pb *ProgressBar) SetSpinner(s *AnimatedSpinner) {
	pb.spinner = s
}

// Update actualiza el progreso actual
func (pb *ProgressBar) Update(current int) {
	if current > pb.total {
		current = pb.total
	}
	pb.current = current
}

// Complete marca la progress bar como completada
func (pb *ProgressBar) Complete() {
	pb.completed = true
	pb.current = pb.total
	if pb.spinner != nil {
		pb.spinner.Stop()
	}
}

// SetColors personaliza los colores
func (pb *ProgressBar) SetColors(titleColor, barColor string) {
	pb.titleColor = titleColor
	pb.barColor = barColor
}

// Render genera la representación visual de la progress bar
func (pb *ProgressBar) Render() string {
	// Calcular porcentaje
	percentage := 0
	if pb.total > 0 {
		percentage = (pb.current * 100) / pb.total
	}

	// Calcular barra
	filled := (pb.width * pb.current) / pb.total
	if filled > pb.width {
		filled = pb.width
	}

	bar := strings.Repeat(pb.fillChar, filled) + strings.Repeat(pb.emptyChar, pb.width-filled)

	// Símbolo del spinner (si existe y está corriendo)
	spinnerSymbol := "◉"
	if pb.spinner != nil && pb.spinner.IsRunning() {
		spinnerSymbol = pb.spinner.Current()
	} else if pb.completed {
		spinnerSymbol = "✓"
	}

	// Duración
	duration := time.Since(pb.startTime)
	durationStr := formatDuration(duration)

	// Color basado en estado
	color := pb.titleColor
	barColorCode := pb.barColor

	if pb.completed {
		color = BrightGreen
		barColorCode = BrightGreen
	}

	// Construir línea
	titlePart := fmt.Sprintf("  %s %s", Colorize(spinnerSymbol, color), Colorize(pb.title, color))

	barPart := Colorize("[", Gray) + Colorize(bar, barColorCode) + Colorize("]", Gray)

	percentageColor := Red
	if percentage >= 100 {
		percentageColor = BrightGreen
	} else if percentage >= 50 {
		percentageColor = Yellow
	}

	percentagePart := Colorize(fmt.Sprintf("%3d%%", percentage), percentageColor)

	timePart := Colorize(durationStr, Gray)

	return fmt.Sprintf("%s %s %s | %s", titlePart, barPart, percentagePart, timePart)
}

// formatDuration formatea una duración de forma legible
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}
