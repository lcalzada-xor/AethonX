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
	totalArtifacts    int
	previousArtifacts int       // Para calcular velocidad
	lastUpdateTime    time.Time // Para calcular velocidad
	sourceStartTime   time.Time

	// Animación de la barra
	growingEdgeFrames []string
	growingEdgeFrame  int

	// Status por source
	sourceNames   []string           // Lista ordenada de nombres de sources
	sourceStatus  map[string]Status  // Estado de cada source
	sourceSpinner map[string]int     // Frame del spinner de cada source
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

		// Animación del borde de crecimiento de la barra
		growingEdgeFrames: []string{"▓", "▒", "░"},
		growingEdgeFrame:  0,

		// Status tracking por source
		sourceNames:   make([]string, 0),
		sourceStatus:  make(map[string]Status),
		sourceSpinner: make(map[string]int),
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
	g.lastUpdateTime = time.Now()
	g.isActive = true
	g.currentFrame = 0
	g.growingEdgeFrame = 0
	g.lineRendered = false
	g.totalArtifacts = 0
	g.previousArtifacts = 0
	g.mu.Unlock()

	// Iniciar goroutine del spinner con ticker de 200ms
	g.startSpinner()
}

// InitializeSources configura la lista de sources a monitorear
func (g *GlobalProgress) InitializeSources(sourceNames []string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.sourceNames = sourceNames
	g.sourceStatus = make(map[string]Status)
	g.sourceSpinner = make(map[string]int)

	// Inicializar todos como pending
	for _, name := range sourceNames {
		g.sourceStatus[name] = StatusPending
		g.sourceSpinner[name] = 0
	}
}

// UpdateCurrent actualiza el source actual que se está ejecutando
func (g *GlobalProgress) UpdateCurrent(sourceName string) {
	g.mu.Lock()

	g.currentSource = sourceName
	g.sourceStartTime = time.Now() // Reset timer para el source actual

	// Actualizar status a Running
	if _, exists := g.sourceStatus[sourceName]; exists {
		g.sourceStatus[sourceName] = StatusRunning
	}

	// Renderizar inmediatamente para mostrar el cambio de status
	g.renderUnsafe()
	g.mu.Unlock()
}

// UpdateSourceStatus actualiza el estado de un source específico
func (g *GlobalProgress) UpdateSourceStatus(sourceName string, status Status) {
	g.mu.Lock()

	g.sourceStatus[sourceName] = status

	// Renderizar inmediatamente para mostrar el cambio de status
	g.renderUnsafe()
	g.mu.Unlock()
}

// UpdateArtifactCount actualiza el contador total de artifacts
func (g *GlobalProgress) UpdateArtifactCount(count int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.previousArtifacts = g.totalArtifacts
	g.totalArtifacts = count
	g.lastUpdateTime = time.Now()
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
	g.spinnerTicker = time.NewTicker(200 * time.Millisecond)

	go func() {
		for {
			select {
			case <-g.spinnerTicker.C:
				g.mu.Lock()
				if g.isActive {
					// Avanzar frame del spinner principal
					g.currentFrame = (g.currentFrame + 1) % len(g.spinnerFrames)
					// Avanzar frame del borde de crecimiento
					g.growingEdgeFrame = (g.growingEdgeFrame + 1) % len(g.growingEdgeFrames)

					// Avanzar frames de spinners por source (solo los que están running)
					for name, status := range g.sourceStatus {
						if status == StatusRunning {
							g.sourceSpinner[name] = (g.sourceSpinner[name] + 1) % len(g.spinnerFrames)
						}
					}

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

	// Renderizar barra de progreso con borde animado
	var filled int
	if g.totalSources > 0 {
		filled = (g.barWidth * g.completedSources) / g.totalSources
		if filled > g.barWidth {
			filled = g.barWidth
		}
	}

	// Barra con transición suave y borde de crecimiento animado
	var bar string
	if g.isActive && filled > 0 && filled < g.barWidth {
		// Barra llena + carácter animado en el borde + vacío
		filledPart := strings.Repeat("█", filled-1)
		growingEdge := g.growingEdgeFrames[g.growingEdgeFrame]
		emptyPart := strings.Repeat("░", g.barWidth-filled)
		bar = filledPart + growingEdge + emptyPart
	} else {
		// Barra normal (sin animación en 0% o 100%)
		bar = strings.Repeat("█", filled) + strings.Repeat("░", g.barWidth-filled)
	}

	// Color basado en progreso con más granularidad
	barColor := terminal.BrightCyan
	if percentage >= 100 {
		barColor = terminal.BrightGreen
	} else if percentage >= 75 {
		barColor = terminal.BrightYellow
	} else if percentage >= 50 {
		barColor = terminal.Yellow
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

	// Contador de artifacts con velocidad
	artifactText := ""
	if g.totalArtifacts > 0 {
		// Calcular velocidad (artifacts/segundo)
		velocity := g.calculateVelocity()
		if velocity > 0 {
			artifactText = terminal.Colorize(
				fmt.Sprintf(" • %d artifacts (%d/s)", g.totalArtifacts, velocity),
				terminal.Gray,
			)
		} else {
			artifactText = terminal.Colorize(
				fmt.Sprintf(" • %d artifacts", g.totalArtifacts),
				terminal.Gray,
			)
		}
	}

	// Construir dashboard de sources (a la derecha)
	sourceDashboard := g.buildSourceDashboard()

	// Construir línea de progreso mejorada con dashboard de sources
	// Formato: ⠋ [████████░░] 75% | (2/3)%s%s%s | 342ms | [httpx ⠋] [rdap ✓] [crtsh ✖]
	line := fmt.Sprintf("  %s %s %3s | %s%s%s%s | %s%s",
		terminal.Colorize(spinnerSymbol, spinnerColor),
		terminal.Colorize("[", terminal.Gray)+terminal.Colorize(bar, barColor)+terminal.Colorize("]", terminal.Gray),
		terminal.Colorize(fmt.Sprintf("%d%%", percentage), barColor),
		terminal.Colorize(fmt.Sprintf("(%d/%d)", g.completedSources, g.totalSources), terminal.Gray),
		slowIndicator,
		etaText,
		artifactText,
		terminal.Colorize(formatDuration(elapsed), terminal.Gray),
		sourceDashboard,
	)

	fmt.Println(line)

	g.lineRendered = true
}

// buildSourceDashboard construye el mini-dashboard de sources
// Formato: | [httpx ⠋] [rdap ✓] [crtsh ✖]
func (g *GlobalProgress) buildSourceDashboard() string {
	if len(g.sourceNames) == 0 {
		return ""
	}

	var parts []string
	for _, name := range g.sourceNames {
		status := g.sourceStatus[name]

		// Símbolo según el estado
		var symbol string
		var color string

		switch status {
		case StatusRunning:
			// Spinner animado para sources en ejecución
			frame := g.sourceSpinner[name]
			symbol = g.spinnerFrames[frame]
			color = terminal.BrightCyan
		case StatusSuccess:
			symbol = "✓"
			color = terminal.BrightGreen
		case StatusError:
			symbol = "✖"
			color = terminal.BrightRed
		case StatusWarning:
			symbol = "⚠"
			color = terminal.BrightYellow
		case StatusPending:
			symbol = "○" // Círculo vacío para pending
			color = terminal.Gray
		default:
			symbol = "?"
			color = terminal.Gray
		}

		// Construir parte: [name symbol]
		part := fmt.Sprintf("[%s %s]",
			terminal.Colorize(name, terminal.White),
			terminal.Colorize(symbol, color),
		)
		parts = append(parts, part)
	}

	if len(parts) > 0 {
		return " | " + strings.Join(parts, " ")
	}
	return ""
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

// calculateVelocity calcula la velocidad de descubrimiento de artifacts (artifacts/segundo)
// NOTA: Debe ser llamado con el mutex ya adquirido
func (g *GlobalProgress) calculateVelocity() int {
	if g.totalArtifacts == 0 {
		return 0
	}

	// Calcular velocidad global (desde el inicio del scan)
	elapsed := time.Since(g.startTime)
	if elapsed.Seconds() < 0.5 {
		// Evitar divisiones por cero y valores erráticos al inicio
		return 0
	}

	velocity := float64(g.totalArtifacts) / elapsed.Seconds()
	return int(velocity)
}
