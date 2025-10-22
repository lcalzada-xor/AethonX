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

	// Header principal
	pterm.DefaultHeader.
		WithBackgroundStyle(pterm.NewStyle(pterm.BgCyan)).
		WithTextStyle(pterm.NewStyle(pterm.FgBlack)).
		Println("AethonX - Reconnaissance Pipeline")

	pterm.Println()

	// Información del scan
	pterm.DefaultSection.Println("Scan Configuration")

	infoPanel := pterm.DefaultBox.
		WithTitle("Target Information").
		WithTitleTopCenter().
		WithRightPadding(4).
		WithLeftPadding(4).
		WithBoxStyle(pterm.NewStyle(pterm.FgCyan))

	targetInfo := fmt.Sprintf("%s Target: %s\n", IconTarget, pterm.Cyan(info.Target))
	targetInfo += fmt.Sprintf("   Mode: %s\n", pterm.Yellow(info.Mode))
	targetInfo += fmt.Sprintf("%s Workers: %d\n", IconWorkers, info.Workers)
	targetInfo += fmt.Sprintf("%s Timeout: %ds\n", IconTime, info.TimeoutSeconds)
	targetInfo += fmt.Sprintf("   Streaming: %s\n", p.boolToString(info.StreamingOn))
	targetInfo += fmt.Sprintf("%s Total Stages: %d", IconStage, info.TotalStages)

	infoPanel.Println(targetInfo)

	pterm.Println()
	pterm.Println(pterm.LightBlue(SeparatorHeavy))
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

	// Mostrar header del stage
	stageTitle := fmt.Sprintf("%s Stage %d/%d: %s",
		IconStage,
		stage.Number,
		stage.TotalStages,
		pterm.Cyan(stage.Name),
	)

	pterm.DefaultSection.WithLevel(2).Println(stageTitle)

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

	// Crear spinner para este source
	spinner, _ := pterm.DefaultSpinner.
		WithStyle(pterm.NewStyle(pterm.FgCyan)).
		WithSequence("⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷").
		Start(fmt.Sprintf("  %s Running %s...",
			StatusRunning.Symbol(),
			pterm.Cyan(sourceName),
		))

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

			// Actualizar spinner si existe
			if spinner, exists := p.spinners[sourceName]; exists {
				spinner.UpdateText(fmt.Sprintf("  %s Running %s... (%s %d artifacts)",
					StatusRunning.Symbol(),
					pterm.Cyan(sourceName),
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
	pterm.Info.Printf("Stage %d completed in %s\n", stageNum, p.formatDuration(duration))
	pterm.Println(pterm.Gray(SeparatorLight))
	pterm.Println()
}

// Info muestra un mensaje informativo
func (p *PTermPresenter) Info(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	pterm.Info.Println(msg)
}

// Warning muestra una advertencia
func (p *PTermPresenter) Warning(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	pterm.Warning.Println(msg)
}

// Error muestra un error
func (p *PTermPresenter) Error(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	pterm.Error.Println(msg)
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
	pterm.Println(pterm.LightBlue(SeparatorHeavy))
	pterm.Println()

	// Header de resultados
	pterm.DefaultHeader.
		WithBackgroundStyle(pterm.NewStyle(pterm.BgGreen)).
		WithTextStyle(pterm.NewStyle(pterm.FgBlack)).
		Println("Scan Completed")

	pterm.Println()

	// Panel de estadísticas
	statsPanel := pterm.DefaultBox.
		WithTitle("Scan Statistics").
		WithTitleTopCenter().
		WithRightPadding(4).
		WithLeftPadding(4).
		WithBoxStyle(pterm.NewStyle(pterm.FgGreen))

	statsContent := fmt.Sprintf("%s Total Duration: %s\n",
		IconTime,
		pterm.Green(p.formatDuration(stats.TotalDuration)),
	)
	statsContent += fmt.Sprintf("%s Total Artifacts: %s\n",
		IconArtifacts,
		pterm.Cyan(fmt.Sprintf("%d", stats.TotalArtifacts)),
	)
	statsContent += fmt.Sprintf("   Unique Artifacts: %s\n",
		pterm.Yellow(fmt.Sprintf("%d", stats.UniqueArtifacts)),
	)
	statsContent += fmt.Sprintf("%s Sources Succeeded: %s\n",
		IconSuccess,
		pterm.Green(fmt.Sprintf("%d", stats.SourcesSucceeded)),
	)

	if stats.SourcesFailed > 0 {
		statsContent += fmt.Sprintf("%s Sources Failed: %s\n",
			IconError,
			pterm.Red(fmt.Sprintf("%d", stats.SourcesFailed)),
		)
	}

	if stats.RelationshipsBuilt > 0 {
		statsContent += fmt.Sprintf("   Relationships: %s",
			pterm.Magenta(fmt.Sprintf("%d", stats.RelationshipsBuilt)),
		)
	}

	statsPanel.Println(statsContent)

	// Tabla de artifacts por tipo (si hay datos)
	if len(stats.ArtifactsByType) > 0 {
		pterm.Println()
		pterm.DefaultSection.WithLevel(2).Println("Artifacts by Type")

		tableData := pterm.TableData{
			{"Type", "Count"},
		}

		for artifactType, count := range stats.ArtifactsByType {
			tableData = append(tableData, []string{
				artifactType,
				fmt.Sprintf("%d", count),
			})
		}

		pterm.DefaultTable.
			WithHasHeader().
			WithBoxed().
			WithData(tableData).
			Render()
	}

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
	styledName := status.Style().Sprint(sourceName)

	line := fmt.Sprintf("  %s %s", symbol, styledName)

	if status == StatusRunning {
		line += " (running...)"
	} else if status == StatusPending {
		line += status.Style().Sprint(" (pending...)")
	} else {
		// Completado con detalles
		if duration > 0 {
			line += fmt.Sprintf(" (%s)", p.formatDuration(duration))
		}

		if artifactCount > 0 {
			line += fmt.Sprintf(" %s %s artifacts",
				IconArtifacts,
				pterm.Cyan(fmt.Sprintf("%d", artifactCount)),
			)
		}
	}

	// Usar el color apropiado
	status.Style().Println(line)
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
		return pterm.Green("ON")
	}
	return pterm.Gray("OFF")
}
