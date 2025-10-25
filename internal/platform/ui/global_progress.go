// internal/platform/ui/global_progress.go
package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"aethonx/internal/platform/ui/terminal"
)

// GlobalProgress maneja una barra de progreso global para todos los sources del stage actual.
// Es thread-safe y se actualiza in-place usando ANSI escape codes.
type GlobalProgress struct {
	totalSources     int
	completedSources int
	currentSource    string
	startTime        time.Time
	isActive         bool
	spinnerFrames    []string
	currentFrame     int
	lineRendered     bool
	barWidth         int
	mu               sync.RWMutex

	// Spinner independiente con goroutine
	spinnerTicker *time.Ticker
	spinnerDone   chan bool

	// Métricas adicionales
	totalArtifacts int
	sourceStartTime time.Time
}

// NewGlobalProgress crea una nueva instancia de GlobalProgress
func NewGlobalProgress() *GlobalProgress {
	return &GlobalProgress{
		// Spinner más suave y visualmente atractivo
		spinnerFrames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		currentFrame:  0,
		lineRendered:  false,
		barWidth:      30, // Ancho de la barra de progreso
		spinnerDone:   make(chan bool),
	}
}

// Start inicializa el progreso global con el número total de sources
func (g *GlobalProgress) Start(totalSources int) {
	g.mu.Lock()
	g.totalSources = totalSources
	g.completedSources = 0
	g.currentSource = ""
	g.startTime = time.Now()
	g.sourceStartTime = time.Now()
	g.isActive = true
	g.currentFrame = 0
	g.lineRendered = false
	g.totalArtifacts = 0
	g.mu.Unlock()

	// Iniciar goroutine del spinner con ticker de 250ms
	g.startSpinner()
}

// UpdateCurrent actualiza el source actual que se está ejecutando
func (g *GlobalProgress) UpdateCurrent(sourceName string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.currentSource = sourceName
	g.sourceStartTime = time.Now() // Reset timer para el source actual
}

// UpdateArtifactCount actualiza el contador total de artifacts
func (g *GlobalProgress) UpdateArtifactCount(count int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.totalArtifacts = count
}

// IncrementCompleted incrementa el contador de sources completados
func (g *GlobalProgress) IncrementCompleted() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.completedSources++
}

// Stop detiene el progreso (marca como inactivo, deja de girar el spinner)
func (g *GlobalProgress) Stop() {
	g.mu.Lock()
	g.isActive = false
	g.mu.Unlock()

	// Detener el spinner goroutine
	g.stopSpinner()
}

// startSpinner inicia la goroutine del spinner independiente
func (g *GlobalProgress) startSpinner() {
	g.spinnerTicker = time.NewTicker(250 * time.Millisecond)

	go func() {
		for {
			select {
			case <-g.spinnerTicker.C:
				g.mu.Lock()
				if g.isActive {
					// Avanzar frame del spinner
					g.currentFrame = (g.currentFrame + 1) % len(g.spinnerFrames)
					// Renderizar solo si estamos activos
					g.renderUnsafe()
				}
				g.mu.Unlock()

			case <-g.spinnerDone:
				g.spinnerTicker.Stop()
				return
			}
		}
	}()
}

// stopSpinner detiene la goroutine del spinner
func (g *GlobalProgress) stopSpinner() {
	if g.spinnerTicker != nil {
		select {
		case g.spinnerDone <- true:
		default:
			// Channel ya cerrado o no disponible, ignorar
		}
	}
}

// Render renderiza la barra de progreso in-place
// Esta función es llamada externamente cuando hay cambios de estado
func (g *GlobalProgress) Render() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.renderUnsafe()
}

// renderUnsafe renderiza sin adquirir el mutex (debe ser llamado con lock ya adquirido)
func (g *GlobalProgress) renderUnsafe() {
	// Si ya renderizamos antes, subir cursor y limpiar línea
	if g.lineRendered {
		fmt.Print(terminal.MoveCursorUp(1))
		fmt.Print(terminal.ClearLine)
		fmt.Print(terminal.MoveCursorToColumn(1))
	}

	// Calcular progreso
	percentage := 0
	if g.totalSources > 0 {
		percentage = (g.completedSources * 100) / g.totalSources
	}

	// Calcular tiempo transcurrido
	elapsed := time.Since(g.startTime)
	sourceElapsed := time.Since(g.sourceStartTime)

	// Símbolo del spinner (si activo)
	spinnerSymbol := "✓" // Check mark cuando está completado
	spinnerColor := terminal.BrightGreen
	if g.isActive {
		spinnerSymbol = g.spinnerFrames[g.currentFrame]
		spinnerColor = terminal.BrightCyan
	}

	// Renderizar barra de progreso con caracteres más suaves
	filled := (g.barWidth * g.completedSources) / g.totalSources
	if filled > g.barWidth {
		filled = g.barWidth
	}

	// Barra con transición suave
	bar := strings.Repeat("█", filled) + strings.Repeat("░", g.barWidth-filled)

	// Color basado en progreso con más granularidad
	barColor := terminal.BrightCyan
	if percentage >= 100 {
		barColor = terminal.BrightGreen
	} else if percentage >= 75 {
		barColor = terminal.BrightYellow
	} else if percentage >= 50 {
		barColor = terminal.Yellow
	}

	// Source actual (o "completed" si terminó)
	sourceText := g.currentSource
	sourceColor := terminal.White
	if !g.isActive && percentage >= 100 {
		sourceText = "completed"
		sourceColor = terminal.BrightGreen
	} else if g.currentSource != "" {
		// Resaltar el source actual con bold
		sourceText = terminal.BoldText(g.currentSource)
		sourceColor = terminal.BrightCyan
	}

	// Indicador de source lento (>5s)
	slowIndicator := ""
	if g.isActive && sourceElapsed > 5*time.Second {
		slowIndicator = terminal.Colorize(" ⏱", terminal.Yellow)
	}

	// Calcular ETA (tiempo estimado restante)
	etaText := ""
	if g.completedSources > 0 && g.completedSources < g.totalSources {
		avgTimePerSource := elapsed / time.Duration(g.completedSources)
		remainingSources := g.totalSources - g.completedSources
		eta := avgTimePerSource * time.Duration(remainingSources)
		etaText = terminal.Colorize(fmt.Sprintf(" • ETA %s", formatDuration(eta)), terminal.Gray)
	}

	// Contador de artifacts
	artifactText := ""
	if g.totalArtifacts > 0 {
		artifactText = terminal.Colorize(fmt.Sprintf(" • %d artifacts", g.totalArtifacts), terminal.Gray)
	}

	// Construir línea de progreso mejorada
	// Formato: ⠋ [████████░░] 75% | rdap (2/3 sources) ⏱ • ETA 2s • 42 artifacts | 342ms
	line := fmt.Sprintf("  %s %s %s | %s %s%s%s%s | %s",
		terminal.Colorize(spinnerSymbol, spinnerColor),
		terminal.Colorize("[", terminal.Gray)+terminal.Colorize(bar, barColor)+terminal.Colorize("]", terminal.Gray),
		terminal.Colorize(fmt.Sprintf("%3d%%", percentage), barColor),
		terminal.Colorize(sourceText, sourceColor),
		terminal.Colorize(fmt.Sprintf("(%d/%d)", g.completedSources, g.totalSources), terminal.Gray),
		slowIndicator,
		etaText,
		artifactText,
		terminal.Colorize(formatDuration(elapsed), terminal.Gray),
	)

	fmt.Println(line)

	g.lineRendered = true
}

// Clear limpia la línea de progreso del terminal
func (g *GlobalProgress) Clear() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.lineRendered {
		fmt.Print(terminal.MoveCursorUp(1))
		fmt.Print(terminal.ClearLine)
		fmt.Print(terminal.MoveCursorToColumn(1))
		g.lineRendered = false
	}
}

// GetProgress retorna el progreso actual (útil para tests)
func (g *GlobalProgress) GetProgress() (completed, total int) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.completedSources, g.totalSources
}

// IsActive retorna si el progreso está activo (útil para tests)
func (g *GlobalProgress) IsActive() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.isActive
}
