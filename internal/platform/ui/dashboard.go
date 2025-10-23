// internal/platform/ui/dashboard.go
package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"
)

// LiveDashboard renderiza un dashboard completo con paneles en tiempo real
type LiveDashboard struct {
	mu sync.Mutex

	// State
	scanInfo    ScanInfo
	sources     map[string]*SourceProgress
	discoveries DiscoveryStats
	startTime   time.Time

	// Metrics collectors
	metricsCollector *MetricsCollector
	systemMetrics    *SystemMetrics

	// PTerm components
	area *pterm.AreaPrinter

	// Control
	stopChan chan struct{}
	running  bool
}

// NewLiveDashboard crea una nueva instancia del dashboard
func NewLiveDashboard(metricsCollector *MetricsCollector, systemMetrics *SystemMetrics) *LiveDashboard {
	return &LiveDashboard{
		sources:          make(map[string]*SourceProgress),
		metricsCollector: metricsCollector,
		systemMetrics:    systemMetrics,
		stopChan:         make(chan struct{}),
	}
}

// Start inicia el dashboard
func (d *LiveDashboard) Start(info ScanInfo) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.scanInfo = info
	d.startTime = time.Now()
	d.running = true

	// Crear area printer
	area, _ := pterm.DefaultArea.WithFullscreen(false).Start()
	d.area = area

	// Iniciar loop de actualización
	go d.updateLoop()
}

// Stop detiene el dashboard
func (d *LiveDashboard) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		close(d.stopChan)
		d.running = false
	}

	if d.area != nil {
		d.area.Stop()
	}
}

// UpdateSource actualiza información de un source
func (d *LiveDashboard) UpdateSource(name string, progress *SourceProgress) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.sources[name] = progress
}

// UpdateDiscoveries actualiza estadísticas de descubrimiento
func (d *LiveDashboard) UpdateDiscoveries(discoveries DiscoveryStats) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.discoveries = discoveries
}

// updateLoop es el loop principal de actualización
func (d *LiveDashboard) updateLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.render()
		case <-d.stopChan:
			return
		}
	}
}

// render renderiza el dashboard completo
func (d *LiveDashboard) render() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.area == nil {
		return
	}

	content := d.buildDashboard()
	d.area.Update(content)
}

// buildDashboard construye el contenido completo del dashboard
func (d *LiveDashboard) buildDashboard() string {
	var b strings.Builder

	// Header principal
	b.WriteString(d.buildHeader())
	b.WriteString("\n\n")

	// Panels superiores (Scan Info + Discoveries)
	b.WriteString(d.buildTopPanels())
	b.WriteString("\n\n")

	// Panel de sources con progreso
	b.WriteString(d.buildSourcesPanel())
	b.WriteString("\n\n")

	// Panels inferiores (Performance + System)
	if d.scanInfo.ShowMetrics {
		b.WriteString(d.buildBottomPanels())
		b.WriteString("\n")
	}

	return b.String()
}

// buildHeader construye el header principal
func (d *LiveDashboard) buildHeader() string {
	var b strings.Builder

	b.WriteString(EmberOrange.Sprint(SeparatorHeavy))
	b.WriteString("\n")

	// ASCII art compacto
	b.WriteString(StylePrimary.Sprint("   █████╗ ███████╗████████╗██╗  ██╗ ██████╗ ███╗   ██╗"))
	b.WriteString(EmberOrange.Sprint(" ▲ LIVE\n"))
	b.WriteString(StylePrimary.Sprint("  ██╔══██╗██╔════╝╚══██╔══╝██║  ██║██╔═══██╗████╗  ██║"))

	// Elapsed time
	elapsed := time.Since(d.startTime)
	b.WriteString(StyleSecondary.Sprintf("   %s %s\n", IconTime, formatDuration(elapsed)))

	b.WriteString(StylePrimary.Sprint("  ███████║█████╗     ██║   ███████║██║   ██║██╔██╗ ██║\n"))
	b.WriteString(StylePrimary.Sprint("  ██╔══██║██╔══╝     ██║   ██╔══██║██║   ██║██║╚██╗██║"))

	// Rate global
	globalMetrics := d.metricsCollector.GetGlobalMetrics()
	b.WriteString(StyleAccent.Sprintf("   %s %.1f/s\n", IconStats, globalMetrics.OverallRate))

	b.WriteString(StylePrimary.Sprint("  ██║  ██║███████╗   ██║   ██║  ██║╚██████╔╝██║ ╚████║\n"))
	b.WriteString(StylePrimary.Sprint("  ╚═╝  ╚═╝╚══════╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝\n"))

	b.WriteString(EmberOrange.Sprint(SeparatorHeavy))

	return b.String()
}

// buildTopPanels construye los paneles superiores
func (d *LiveDashboard) buildTopPanels() string {
	scanPanel := d.buildScanInfoPanel()
	discoveryPanel := d.buildDiscoveryPanel()

	// Combinar en línea
	return combinePanelsHorizontal(scanPanel, discoveryPanel)
}

// buildScanInfoPanel construye el panel de información del scan
func (d *LiveDashboard) buildScanInfoPanel() string {
	var b strings.Builder

	b.WriteString(BorderTopLeft + strings.Repeat(BorderHorizontal, 32) + " SCAN INFO " + strings.Repeat(BorderHorizontal, 2) + BorderTopRight + "\n")

	b.WriteString(fmt.Sprintf("%s %s Target    %s\n",
		BorderVertical,
		IconTarget,
		StyleText.Sprintf("%-31s", d.scanInfo.Target)+BorderVertical,
	))

	b.WriteString(fmt.Sprintf("%s %s Mode      %-31s%s\n",
		BorderVertical,
		IconMode,
		StyleAccent.Sprint(d.scanInfo.Mode),
		BorderVertical,
	))

	b.WriteString(fmt.Sprintf("%s %s Workers   %-31s%s\n",
		BorderVertical,
		IconWorkers,
		StyleText.Sprintf("%d", d.scanInfo.Workers),
		BorderVertical,
	))

	elapsed := time.Since(d.startTime)
	b.WriteString(fmt.Sprintf("%s %s Elapsed   %-31s%s\n",
		BorderVertical,
		IconTime,
		StyleSuccess.Sprint(formatDuration(elapsed)),
		BorderVertical,
	))

	b.WriteString(BorderBottomLeft + strings.Repeat(BorderHorizontal, 46) + BorderBottomRight)

	return b.String()
}

// buildDiscoveryPanel construye el panel de descubrimientos
func (d *LiveDashboard) buildDiscoveryPanel() string {
	var b strings.Builder

	b.WriteString(BorderTopLeft + strings.Repeat(BorderHorizontal, 28) + " DISCOVERIES " + strings.Repeat(BorderHorizontal, 2) + BorderTopRight + "\n")

	b.WriteString(fmt.Sprintf("%s %s Subdomains  %s%s\n",
		BorderVertical,
		IconArtifacts,
		StyleAccent.Sprintf("%19d", d.discoveries.Subdomains),
		BorderVertical,
	))

	b.WriteString(fmt.Sprintf("%s %s IPs         %s%s\n",
		BorderVertical,
		IconArtifacts,
		StyleAccent.Sprintf("%19d", d.discoveries.IPs),
		BorderVertical,
	))

	b.WriteString(fmt.Sprintf("%s %s URLs        %s%s\n",
		BorderVertical,
		IconArtifacts,
		StyleAccent.Sprintf("%19d", d.discoveries.URLs),
		BorderVertical,
	))

	b.WriteString(fmt.Sprintf("%s %s Emails      %s%s\n",
		BorderVertical,
		IconArtifacts,
		StyleAccent.Sprintf("%19d", d.discoveries.Emails),
		BorderVertical,
	))

	b.WriteString(BorderBottomLeft + strings.Repeat(BorderHorizontal, 44) + BorderBottomRight)

	return b.String()
}

// buildSourcesPanel construye el panel de sources con barras de progreso
func (d *LiveDashboard) buildSourcesPanel() string {
	var b strings.Builder

	width := 76
	b.WriteString(BorderTopLeft + strings.Repeat(BorderHorizontal, width-18) + " SOURCES PROGRESS " + strings.Repeat(BorderHorizontal, 0) + BorderTopRight + "\n")
	b.WriteString(BorderVertical + strings.Repeat(" ", width) + BorderVertical + "\n")

	// Renderizar cada source
	for _, src := range d.sources {
		metrics := d.metricsCollector.GetProgressMetrics(src.Name)
		b.WriteString(d.buildSourceLine(src, metrics, width))
	}

	b.WriteString(BorderVertical + strings.Repeat(" ", width) + BorderVertical + "\n")
	b.WriteString(BorderBottomLeft + strings.Repeat(BorderHorizontal, width) + BorderBottomRight)

	return b.String()
}

// buildSourceLine construye una línea de progreso para un source
func (d *LiveDashboard) buildSourceLine(src *SourceProgress, metrics ProgressMetrics, width int) string {
	var b strings.Builder

	// Línea principal con barra de progreso
	b.WriteString(BorderVertical + "  ")

	// Símbolo de estado
	symbol := src.Status.Symbol()
	style := src.Status.Style()
	b.WriteString(style.Sprint(symbol) + " ")

	// Nombre del source (10 caracteres)
	b.WriteString(fmt.Sprintf("%-10s ", src.Name))

	// Barra de progreso (20 caracteres)
	progressBar := buildProgressBar(metrics.Percentage, 20)
	b.WriteString(progressBar + " ")

	// Porcentaje (4 caracteres)
	if metrics.Percentage >= 0 {
		b.WriteString(StyleAccent.Sprintf("%3.0f%% ", metrics.Percentage))
	} else {
		b.WriteString(StyleSecondary.Sprint(" --  "))
	}

	// Separador
	b.WriteString(BorderVertical + " ")

	// Artifacts (4 caracteres)
	b.WriteString(StyleText.Sprintf("%4d ", src.ArtifactCount))

	// Separador
	b.WriteString(BorderVertical + " ")

	// Rate (8 caracteres)
	b.WriteString(StyleInfo.Sprintf("%6.1f/s ", metrics.Rate))

	// Separador
	b.WriteString(BorderVertical + " ")

	// ETA (6 caracteres)
	if metrics.EstimatedTime > 0 {
		b.WriteString(StyleWarning.Sprintf("%5s ", formatDuration(metrics.EstimatedTime)))
	} else {
		b.WriteString(StyleSecondary.Sprint("  --  "))
	}

	b.WriteString(BorderVertical + "\n")

	// Línea secundaria con fase (si ShowPhases está habilitado)
	if d.scanInfo.ShowPhases && metrics.Phase != "" {
		b.WriteString(BorderVertical + "     ")
		b.WriteString(StyleSecondary.Sprintf("Phase: %s", metrics.Phase))
		padding := width - len("     Phase: "+metrics.Phase) - 1
		if padding > 0 {
			b.WriteString(strings.Repeat(" ", padding))
		}
		b.WriteString(BorderVertical + "\n")
		b.WriteString(BorderVertical + strings.Repeat(" ", width) + BorderVertical + "\n")
	}

	return b.String()
}

// buildBottomPanels construye los paneles inferiores (Performance + System)
func (d *LiveDashboard) buildBottomPanels() string {
	perfPanel := d.buildPerformancePanel()
	sysPanel := d.buildSystemPanel()

	return combinePanelsHorizontal(perfPanel, sysPanel)
}

// buildPerformancePanel construye el panel de performance
func (d *LiveDashboard) buildPerformancePanel() string {
	var b strings.Builder

	globalMetrics := d.metricsCollector.GetGlobalMetrics()

	b.WriteString(BorderTopLeft + strings.Repeat(BorderHorizontal, 30) + " PERFORMANCE " + strings.Repeat(BorderHorizontal, 2) + BorderTopRight + "\n")

	b.WriteString(fmt.Sprintf("%s %s Rate        %s%s\n",
		BorderVertical,
		IconStats,
		StyleSuccess.Sprintf("%22.1f/s", globalMetrics.OverallRate),
		BorderVertical,
	))

	// Success rate
	b.WriteString(fmt.Sprintf("%s %s Success     %s%s\n",
		BorderVertical,
		IconSuccess,
		StyleSuccess.Sprintf("%21.1f%%", globalMetrics.SuccessRate),
		BorderVertical,
	))

	// Unique ratio
	uniqueRatio := 0.0
	if globalMetrics.TotalArtifacts > 0 {
		uniqueRatio = float64(globalMetrics.UniqueArtifacts) / float64(globalMetrics.TotalArtifacts) * 100
	}
	b.WriteString(fmt.Sprintf("%s %s Unique      %s%s\n",
		BorderVertical,
		IconArtifacts,
		StyleAccent.Sprintf("%21.1f%%", uniqueRatio),
		BorderVertical,
	))

	b.WriteString(BorderBottomLeft + strings.Repeat(BorderHorizontal, 46) + BorderBottomRight)

	return b.String()
}

// buildSystemPanel construye el panel de métricas del sistema
func (d *LiveDashboard) buildSystemPanel() string {
	var b strings.Builder

	sysMetrics := d.systemMetrics.GetSnapshot()

	b.WriteString(BorderTopLeft + strings.Repeat(BorderHorizontal, 32) + " SYSTEM " + strings.Repeat(BorderHorizontal, 7) + BorderTopRight + "\n")

	b.WriteString(fmt.Sprintf("%s %s CPU         %s%s\n",
		BorderVertical,
		IconMode,
		StyleInfo.Sprintf("%21.1f%%", sysMetrics.CPUPercent),
		BorderVertical,
	))

	b.WriteString(fmt.Sprintf("%s %s Memory      %s%s\n",
		BorderVertical,
		IconMode,
		StyleInfo.Sprintf("%17s MB", fmt.Sprintf("%.1f", sysMetrics.MemoryMB)),
		BorderVertical,
	))

	b.WriteString(fmt.Sprintf("%s %s Goroutines  %s%s\n",
		BorderVertical,
		IconWorkers,
		StyleText.Sprintf("%22d", sysMetrics.GoroutineCount),
		BorderVertical,
	))

	b.WriteString(BorderBottomLeft + strings.Repeat(BorderHorizontal, 46) + BorderBottomRight)

	return b.String()
}

// Helper functions

// buildProgressBar construye una barra de progreso visual
func buildProgressBar(percentage float64, width int) string {
	if percentage < 0 {
		// Modo indeterminado
		return StyleSecondary.Sprint("[") +
			strings.Repeat(ProgressEmpty, width) +
			StyleSecondary.Sprint("]")
	}

	filled := int(percentage / 100 * float64(width))
	if filled > width {
		filled = width
	}

	bar := StyleSecondary.Sprint("[")
	bar += StyleSuccess.Sprint(strings.Repeat(ProgressFull, filled))
	bar += StyleSecondary.Sprint(strings.Repeat(ProgressEmpty, width-filled))
	bar += StyleSecondary.Sprint("]")

	return bar
}

// combinePanelsHorizontal combina dos paneles lado a lado
func combinePanelsHorizontal(left, right string) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")

	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}

	var b strings.Builder
	for i := 0; i < maxLines; i++ {
		if i < len(leftLines) {
			b.WriteString(leftLines[i])
		}
		b.WriteString("  ") // Espaciado entre paneles
		if i < len(rightLines) {
			b.WriteString(rightLines[i])
		}
		b.WriteString("\n")
	}

	return b.String()
}
