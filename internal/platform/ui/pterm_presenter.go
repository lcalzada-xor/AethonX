// internal/platform/ui/pterm_presenter.go
package ui

import (
	"fmt"
	"sync"
	"time"

	"github.com/pterm/pterm"
)

// PTermPresenter implementa Presenter usando la biblioteca pterm
// para renderizar spinners, colores y símbolos en la terminal.
type PTermPresenter struct {
	mu sync.Mutex

	// Tracking de progreso
	stages        map[int]*StageProgress
	currentStage  int
	totalStages   int
	scanStartTime time.Time

	// Spinners activos por source
	spinners map[string]*pterm.SpinnerPrinter

	// Configuración
	scanInfo ScanInfo

	// Multi printer para manejar múltiples spinners
	multiPrinter *pterm.MultiPrinter
}

// NewPTermPresenter crea una nueva instancia del presenter con pterm
func NewPTermPresenter() *PTermPresenter {
	return &PTermPresenter{
		stages:   make(map[int]*StageProgress),
		spinners: make(map[string]*pterm.SpinnerPrinter),
	}
}

// Start inicia la presentación mostrando el header del escaneo
func (p *PTermPresenter) Start(info ScanInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.scanInfo = info
	p.totalStages = info.TotalStages
	p.scanStartTime = time.Now()

	// Header principal con ASCII art del caballo Aethon
	pterm.Println() // Espacio inicial

	// Banner con colores aplicados correctamente
	EmberOrange.Println("════════════════════════════════════════════════════════════")
	pterm.Println()

	StylePrimary.Println("    █████╗ ███████╗████████╗██╗  ██╗ ██████╗ ███╗   ██╗")
	StylePrimary.Println("   ██╔══██╗██╔════╝╚══██╔══╝██║  ██║██╔═══██╗████╗  ██║")
	StylePrimary.Println("   ███████║█████╗     ██║   ███████║██║   ██║██╔██╗ ██║")
	StylePrimary.Println("   ██╔══██║██╔══╝     ██║   ██╔══██║██║   ██║██║╚██╗██║")
	StylePrimary.Println("   ██║  ██║███████╗   ██║   ██║  ██║╚██████╔╝██║ ╚████║")
	StylePrimary.Println("   ╚═╝  ╚═╝╚══════╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝")
	pterm.Println()

	// Tagline centrado con color ember
	EmberOrange.Println("          Illuminating the Digital Underworld")
	pterm.Println()
	EmberOrange.Println("════════════════════════════════════════════════════════════")
	pterm.Println()

	// Información del scan con diseño limpio
	StylePrimary.Println("▸ SCAN CONFIGURATION")
	pterm.Println()

	targetInfo := fmt.Sprintf("  %s TARGET      %s\n", IconTarget, StyleText.Sprint(info.Target))
	targetInfo += fmt.Sprintf("  %s MODE        %s\n", IconMode, StyleAccent.Sprint(info.Mode))
	targetInfo += fmt.Sprintf("  %s WORKERS     %s\n", IconWorkers, StyleText.Sprintf("%d", info.Workers))
	targetInfo += fmt.Sprintf("  %s TIMEOUT     %s\n", IconTime, StyleText.Sprintf("%ds", info.TimeoutSeconds))
	targetInfo += fmt.Sprintf("  %s STREAMING   %s\n", IconInfo, p.boolToString(info.StreamingOn))
	targetInfo += fmt.Sprintf("  %s STAGES      %s", IconStage, StyleText.Sprintf("%d", info.TotalStages))

	pterm.Println(targetInfo)
	pterm.Println()
	StyleSecondary.Println(SeparatorLight)
	pterm.Println()
}

// StartStage notifica el inicio de un nuevo stage
func (p *PTermPresenter) StartStage(stage StageInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.currentStage = stage.Number

	// Crear tracking para este stage
	stageProgress := &StageProgress{
		Number:    stage.Number,
		Name:      stage.Name,
		Status:    StatusRunning,
		Sources:   make(map[string]*SourceProgress),
		StartTime: time.Now(),
	}

	// Inicializar sources como pending
	for _, sourceName := range stage.Sources {
		stageProgress.Sources[sourceName] = &SourceProgress{
			Name:   sourceName,
			Status: StatusPending,
		}
	}

	p.stages[stage.Number] = stageProgress

	// Mostrar header del stage con nuevo diseño
	stageTitle := fmt.Sprintf("%s STAGE %d/%d: %s",
		IconStage,
		stage.Number,
		stage.TotalStages,
		stage.Name,
	)

	StylePrimary.Println(stageTitle)
	pterm.Println()

	// Mostrar sources del stage
	for _, sourceName := range stage.Sources {
		status := StatusPending
		p.renderSourceLine(sourceName, status, 0, 0)
	}

	pterm.Println()
}

// StartSource notifica el inicio de ejecución de un source
func (p *PTermPresenter) StartSource(stageNum int, sourceName string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	stage, exists := p.stages[stageNum]
	if !exists {
		return
	}

	srcProgress, exists := stage.Sources[sourceName]
	if !exists {
		srcProgress = &SourceProgress{
			Name: sourceName,
		}
		stage.Sources[sourceName] = srcProgress
	}

	srcProgress.Status = StatusRunning
	srcProgress.StartTime = time.Now()

	// Crear spinner dinámico con brasas pulsantes (temática infernal)
	// El texto debe ser plano sin colores pre-aplicados para que el spinner pueda actualizarlo
	spinnerText := fmt.Sprintf("  ▸ Running %s...", sourceName)

	spinner, _ := pterm.DefaultSpinner.
		WithDelay(80 * time.Millisecond).                          // Velocidad más rápida y dinámica
		WithSequence("◉ ", "◎ ", "◉ ", "◎ ", "○ ", "◎ ").         // Brasas pulsantes con espacios
		WithStyle(pterm.NewStyle(pterm.FgLightRed, pterm.Bold)).   // Color rojo brillante + bold
		WithRemoveWhenDone(true).                                  // Limpiar cuando termine
		Start(spinnerText)

	p.spinners[sourceName] = spinner
}

// UpdateSource actualiza el progreso de un source
func (p *PTermPresenter) UpdateSource(sourceName string, artifactCount int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Buscar el source en los stages
	for _, stage := range p.stages {
		if srcProgress, exists := stage.Sources[sourceName]; exists {
			srcProgress.ArtifactCount = artifactCount

			// Actualizar spinner si existe con contador de artifacts (texto plano para que se actualice dinámicamente)
			if spinner, exists := p.spinners[sourceName]; exists {
				spinner.UpdateText(fmt.Sprintf("  ▸ Running %s... %s %d artifacts",
					sourceName,
					IconArtifacts,
					artifactCount,
				))
			}
			break
		}
	}
}

// FinishSource notifica la finalización de un source
func (p *PTermPresenter) FinishSource(sourceName string, status Status, duration time.Duration, artifactCount int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Actualizar progreso del source
	for _, stage := range p.stages {
		if srcProgress, exists := stage.Sources[sourceName]; exists {
			srcProgress.Status = status
			srcProgress.Duration = duration
			srcProgress.ArtifactCount = artifactCount
			break
		}
	}

	// Detener y reemplazar spinner
	if spinner, exists := p.spinners[sourceName]; exists {
		spinner.Stop()
		delete(p.spinners, sourceName)
	}

	// Renderizar línea final con resultado
	p.renderSourceLine(sourceName, status, duration, artifactCount)
}

// FinishStage notifica la finalización de un stage
func (p *PTermPresenter) FinishStage(stageNum int, duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	stage, exists := p.stages[stageNum]
	if !exists {
		return
	}

	stage.Status = StatusSuccess
	stage.Duration = duration

	// Verificar si hubo errores
	for _, src := range stage.Sources {
		if src.Status == StatusError {
			stage.Status = StatusWarning
			break
		}
	}

	pterm.Println()
	completedMsg := fmt.Sprintf("%s Stage %d completed in %s", IconSuccess, stageNum, p.formatDuration(duration))
	StyleSuccess.Println(completedMsg)
	StyleSecondary.Println(SeparatorLight)
	pterm.Println()
}

// Info muestra un mensaje informativo
func (p *PTermPresenter) Info(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	StyleInfo.Printf("%s %s\n", IconInfo, msg)
}

// Warning muestra una advertencia
func (p *PTermPresenter) Warning(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	StyleWarning.Printf("%s %s\n", IconWarning, msg)
}

// Error muestra un error
func (p *PTermPresenter) Error(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	StyleError.Printf("%s %s\n", IconError, msg)
}

// Finish finaliza la presentación con estadísticas finales
func (p *PTermPresenter) Finish(stats ScanStats) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Detener todos los spinners activos
	for _, spinner := range p.spinners {
		spinner.Stop()
	}

	pterm.Println()
	EmberOrange.Println(SeparatorHeavy)
	pterm.Println()

	// Header de resultados con diseño impactante
	StyleSuccess.Print("  ⚡ SCAN COMPLETE")
	pterm.Println()
	pterm.Println()

	// Estadísticas con diseño limpio
	StylePrimary.Println("▸ SCAN RESULTS")
	pterm.Println()

	statsContent := fmt.Sprintf("  %s DURATION       %s\n",
		IconTime,
		StyleSuccess.Sprint(p.formatDuration(stats.TotalDuration)),
	)
	statsContent += fmt.Sprintf("  %s ARTIFACTS     %s total, %s unique\n",
		IconArtifacts,
		StyleAccent.Sprintf("%d", stats.TotalArtifacts),
		StyleWarning.Sprintf("%d", stats.UniqueArtifacts),
	)
	statsContent += fmt.Sprintf("  %s SOURCES       %s succeeded",
		IconSources,
		StyleSuccess.Sprintf("%d", stats.SourcesSucceeded),
	)

	if stats.SourcesFailed > 0 {
		statsContent += fmt.Sprintf(", %s failed",
			StyleError.Sprintf("%d", stats.SourcesFailed),
		)
	}
	statsContent += "\n"

	if stats.RelationshipsBuilt > 0 {
		statsContent += fmt.Sprintf("  %s RELATIONS     %s\n",
			IconInfo,
			StyleInfo.Sprintf("%d", stats.RelationshipsBuilt),
		)
	}

	pterm.Println(statsContent)

	// Tabla de artifacts por tipo (si hay datos)
	if len(stats.ArtifactsByType) > 0 {
		pterm.Println()
		StylePrimary.Println("▸ ARTIFACTS BY TYPE")
		pterm.Println()

		tableData := pterm.TableData{
			{"Type", "Count"},
		}

		for artifactType, count := range stats.ArtifactsByType {
			tableData = append(tableData, []string{
				StyleText.Sprint(artifactType),
				StyleAccent.Sprintf("%d", count),
			})
		}

		pterm.DefaultTable.
			WithHasHeader().
			WithBoxed().
			WithHeaderStyle(pterm.NewStyle(pterm.FgLightRed)).
			WithData(tableData).
			Render()
	}

	pterm.Println()
	StyleSecondary.Println(SeparatorLight)
	pterm.Println()

	// Footer con identidad temática
	StyleSecondary.Print("  AethonX ")
	StyleSecondary.Print("— Illuminating the Digital Underworld ")
	StylePrimary.Println("▲")
	pterm.Println()
}

// Close limpia recursos del presenter
func (p *PTermPresenter) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Detener todos los spinners activos
	for _, spinner := range p.spinners {
		spinner.Stop()
	}

	p.spinners = make(map[string]*pterm.SpinnerPrinter)
	return nil
}

// renderSourceLine renderiza una línea con el estado de un source
func (p *PTermPresenter) renderSourceLine(sourceName string, status Status, duration time.Duration, artifactCount int) {
	symbol := status.Symbol()
	style := status.Style()

	line := fmt.Sprintf("  %s %s", symbol, style.Sprint(sourceName))

	if status == StatusRunning {
		line += style.Sprint(" (running...)")
	} else if status == StatusPending {
		line += style.Sprint(" (pending...)")
	} else {
		// Completado con detalles
		if duration > 0 {
			line += fmt.Sprintf(" %s", StyleSecondary.Sprintf("(%s)", p.formatDuration(duration)))
		}

		if artifactCount > 0 {
			line += fmt.Sprintf(" %s %s",
				IconArtifacts,
				StyleAccent.Sprintf("%d artifacts", artifactCount),
			)
		}
	}

	pterm.Println(line)
}

// formatDuration formatea una duración de manera legible
func (p *PTermPresenter) formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// boolToString convierte booleano a string visual
func (p *PTermPresenter) boolToString(b bool) string {
	if b {
		return StyleSuccess.Sprint("ON")
	}
	return StyleSecondary.Sprint("OFF")
}
