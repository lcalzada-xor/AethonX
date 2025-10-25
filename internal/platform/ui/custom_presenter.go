// internal/platform/ui/custom_presenter.go
package ui

import (
	"fmt"
	"sync"
	"time"

	"aethonx/internal/platform/ui/terminal"
)

// CustomPresenter implementa Presenter con sistema de renderizado custom
type CustomPresenter struct {
	renderer   *terminal.Renderer
	progressBars map[string]*terminal.ProgressBar
	spinners     map[string]*terminal.AnimatedSpinner
	lineCounter  int

	mu         sync.RWMutex
	stages     map[int]*StageProgress
	scanInfo   ScanInfo
	startTime  time.Time
}

// NewCustomPresenter crea un nuevo presenter custom
func NewCustomPresenter() *CustomPresenter {
	return &CustomPresenter{
		renderer:     terminal.NewRenderer(),
		progressBars: make(map[string]*terminal.ProgressBar),
		spinners:     make(map[string]*terminal.AnimatedSpinner),
		stages:       make(map[int]*StageProgress),
		lineCounter:  0,
	}
}

// Start inicia la presentación
func (c *CustomPresenter) Start(info ScanInfo) {
	c.mu.Lock()
	c.scanInfo = info
	c.startTime = time.Now()
	c.mu.Unlock()

	// Renderizar header
	c.renderHeader(info)

	// Iniciar renderer
	c.renderer.Start(100 * time.Millisecond)
}

// StartStage inicia un nuevo stage
func (c *CustomPresenter) StartStage(stage StageInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stageProgress := &StageProgress{
		Number:    stage.Number,
		Name:      stage.Name,
		Status:    StatusRunning,
		Sources:   make(map[string]*SourceProgress),
		StartTime: time.Now(),
	}

	for _, sourceName := range stage.Sources {
		stageProgress.Sources[sourceName] = &SourceProgress{
			Name:   sourceName,
			Status: StatusPending,
		}
	}

	c.stages[stage.Number] = stageProgress

	// Renderizar stage header
	fmt.Println()
	fmt.Printf("%s %s %d/%d: %s%s\n",
		terminal.Colorize(IconStage, terminal.RGB(255, 107, 53)),
		terminal.BoldText("STAGE"),
		stage.Number,
		stage.TotalStages,
		stage.Name,
		terminal.Reset,
	)
	fmt.Println()
}

// FinishStage finaliza un stage
func (c *CustomPresenter) FinishStage(stageNum int, duration time.Duration) {
	c.mu.Lock()
	stage, exists := c.stages[stageNum]
	if !exists {
		c.mu.Unlock()
		return
	}

	stage.Status = StatusSuccess
	stage.Duration = duration

	// Verificar errores
	for _, src := range stage.Sources {
		if src.Status == StatusError {
			stage.Status = StatusWarning
			break
		}
	}
	c.mu.Unlock()

	fmt.Println()
	fmt.Printf("%s Stage %d completed in %s\n",
		terminal.Colorize(IconSuccess, terminal.BrightGreen),
		stageNum,
		formatDuration(duration),
	)
	fmt.Println(terminal.Colorize(SeparatorLight, terminal.Gray))
	fmt.Println()
}

// StartSource inicia la ejecución de un source
func (c *CustomPresenter) StartSource(stageNum int, sourceName string) {
	c.mu.Lock()

	// Actualizar state
	stage, exists := c.stages[stageNum]
	if !exists {
		c.mu.Unlock()
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

	// Crear spinner animado
	spinner := terminal.NewAnimatedSpinner("ember")
	spinner.Start(50 * time.Millisecond)
	c.spinners[sourceName] = spinner

	// Crear progress bar
	pb := terminal.NewProgressBar(sourceName, 100)
	pb.SetSpinner(spinner)
	c.progressBars[sourceName] = pb

	lineID := c.lineCounter
	c.lineCounter++

	c.mu.Unlock()

	// Registrar con renderer (sin lock)
	c.renderer.RegisterLine(lineID, pb)
}

// UpdateSource actualiza el progreso de un source
func (c *CustomPresenter) UpdateSource(sourceName string, metrics ProgressMetrics) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Actualizar progress bar si existe
	if pb, exists := c.progressBars[sourceName]; exists {
		// Actualizar progreso basado en current
		if metrics.Total > 0 {
			percentage := (metrics.Current * 100) / metrics.Total
			pb.Update(percentage)
		}
	}

	// Actualizar métricas en state
	for _, stage := range c.stages {
		if srcProgress, exists := stage.Sources[sourceName]; exists {
			srcProgress.ArtifactCount = metrics.Current
			if srcProgress.Metrics == nil {
				srcProgress.Metrics = &ProgressMetrics{}
			}
			*srcProgress.Metrics = metrics
			break
		}
	}
}

// UpdateSourcePhase actualiza solo la fase de un source
func (c *CustomPresenter) UpdateSourcePhase(sourceName string, phase string) {
	// No-op en custom presenter (simplificado)
}

// FinishSource finaliza un source
func (c *CustomPresenter) FinishSource(sourceName string, status Status, duration time.Duration, artifactCount int) {
	c.mu.Lock()

	// Actualizar state
	for _, stage := range c.stages {
		if srcProgress, exists := stage.Sources[sourceName]; exists {
			srcProgress.Status = status
			srcProgress.Duration = duration
			srcProgress.ArtifactCount = artifactCount
			break
		}
	}

	// Completar progress bar
	if pb, exists := c.progressBars[sourceName]; exists {
		pb.Complete()
	}

	// Detener spinner
	if spinner, exists := c.spinners[sourceName]; exists {
		spinner.Stop()
		delete(c.spinners, sourceName)
	}

	c.mu.Unlock()

	// Esperar a que renderer muestre el estado final
	time.Sleep(150 * time.Millisecond)

	// Limpiar línea del renderer
	c.renderer.Clear()

	// Renderizar línea final
	symbol := status.Symbol()
	color := status.Color()

	fmt.Printf("  %s %s %s ◆ %s%d artifacts%s\n",
		terminal.Colorize(symbol, color),
		terminal.Colorize(sourceName, terminal.BrightCyan),
		terminal.Colorize(fmt.Sprintf("(%s)", formatDuration(duration)), terminal.Gray),
		terminal.Colorize("", color),
		artifactCount,
		terminal.Reset,
	)
}

// UpdateDiscoveries actualiza estadísticas de descubrimiento
func (c *CustomPresenter) UpdateDiscoveries(discoveries DiscoveryStats) {
	// No-op en modo simple
}

// Info muestra un mensaje informativo
func (c *CustomPresenter) Info(msg string) {
	fmt.Printf("%s %s\n", terminal.Colorize(IconInfo, terminal.BrightCyan), msg)
}

// Warning muestra una advertencia
func (c *CustomPresenter) Warning(msg string) {
	fmt.Printf("%s %s\n", terminal.Colorize(IconWarning, terminal.BrightYellow), msg)
}

// Error muestra un error
func (c *CustomPresenter) Error(msg string) {
	fmt.Printf("%s %s\n", terminal.Colorize(IconError, terminal.BrightRed), msg)
}

// Finish finaliza la presentación
func (c *CustomPresenter) Finish(stats ScanStats) {
	// Detener renderer
	c.renderer.Stop()

	fmt.Println()
	fmt.Println(terminal.Colorize(SeparatorHeavy, terminal.RGB(255, 107, 53)))
	fmt.Printf("%s  %s\n", terminal.Colorize("⚡", terminal.BrightCyan), terminal.BoldText("SCAN COMPLETE"))
	fmt.Println()

	// Estadísticas
	fmt.Printf("%s %s\n\n", terminal.Colorize(IconStats, terminal.RGB(255, 107, 53)), terminal.BoldText("SCAN RESULTS"))

	fmt.Printf("  %s DURATION       %s\n", IconTime, terminal.Colorize(formatDuration(stats.TotalDuration), terminal.BrightCyan))
	fmt.Printf("  %s ARTIFACTS     %s total, %s unique\n",
		IconArtifacts,
		terminal.Colorize(fmt.Sprintf("%d", stats.TotalArtifacts), terminal.BrightCyan),
		terminal.Colorize(fmt.Sprintf("%d", stats.UniqueArtifacts), terminal.BrightYellow),
	)
	fmt.Printf("  %s SOURCES       %s succeeded\n",
		IconSources,
		terminal.Colorize(fmt.Sprintf("%d", stats.SourcesSucceeded), terminal.BrightCyan),
	)

	if stats.RelationshipsBuilt > 0 {
		fmt.Printf("  ℹ RELATIONS     %s\n",
			terminal.Colorize(fmt.Sprintf("%d", stats.RelationshipsBuilt), terminal.RGB(75, 0, 130)),
		)
	}

	// Artifacts por tipo
	if len(stats.ArtifactsByType) > 0 {
		fmt.Printf("\n%s %s\n\n", terminal.Colorize(IconStats, terminal.RGB(255, 107, 53)), terminal.BoldText("ARTIFACTS BY TYPE"))
		for aType, count := range stats.ArtifactsByType {
			fmt.Printf("  %s: %s\n",
				terminal.Colorize(aType, terminal.White),
				terminal.Colorize(fmt.Sprintf("%d", count), terminal.BrightCyan),
			)
		}
	}

	fmt.Println()
	fmt.Println(terminal.Colorize(SeparatorLight, terminal.Gray))
	fmt.Println()
}

// Close limpia recursos
func (c *CustomPresenter) Close() error {
	c.renderer.Stop()

	// Detener todos los spinners
	c.mu.Lock()
	for _, spinner := range c.spinners {
		spinner.Stop()
	}
	c.mu.Unlock()

	return nil
}

// renderHeader renderiza el header del scan
func (c *CustomPresenter) renderHeader(info ScanInfo) {
	fmt.Println()
	fmt.Println(terminal.Colorize(SeparatorHeavy, terminal.RGB(255, 107, 53)))

	// ASCII Art simplificado
	art := `    █████╗ ███████╗████████╗██╗  ██╗ ██████╗ ███╗   ██╗
   ██╔══██╗██╔════╝╚══██╔══╝██║  ██║██╔═══██╗████╗  ██║
   ███████║█████╗     ██║   ███████║██║   ██║██╔██╗ ██║
   ██╔══██║██╔══╝     ██║   ██╔══██║██║   ██║██║╚██╗██║
   ██║  ██║███████╗   ██║   ██║  ██║╚██████╔╝██║ ╚████║
   ╚═╝  ╚═╝╚══════╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝`

	fmt.Println(terminal.Colorize(art, terminal.RGB(255, 107, 53)))
	fmt.Println()
	fmt.Println(terminal.Colorize("          Illuminating the Digital Underworld", terminal.RGB(255, 107, 53)))
	fmt.Println(terminal.Colorize(SeparatorHeavy, terminal.RGB(255, 107, 53)))

	// Configuración
	fmt.Printf("%s %s\n\n", terminal.Colorize(IconTarget, terminal.RGB(255, 107, 53)), terminal.BoldText("SCAN CONFIGURATION"))

	fmt.Printf("  %s TARGET      %s\n", IconTarget, terminal.Colorize(info.Target, terminal.White))
	fmt.Printf("  %s MODE        %s\n", IconMode, terminal.Colorize(info.Mode, terminal.BrightCyan))
	fmt.Printf("  %s WORKERS     %s\n", IconWorkers, terminal.Colorize(fmt.Sprintf("%d", info.Workers), terminal.White))

	timeoutStr := "∞"
	if info.TimeoutSeconds > 0 {
		timeoutStr = fmt.Sprintf("%ds", info.TimeoutSeconds)
	}
	fmt.Printf("  %s TIMEOUT     %s\n", IconTime, terminal.Colorize(timeoutStr, terminal.White))

	streamingStatus := "OFF"
	if info.StreamingOn {
		streamingStatus = "ON"
	}
	fmt.Printf("  ℹ STREAMING   %s\n", terminal.Colorize(streamingStatus, terminal.BrightCyan))
	fmt.Printf("  %s UI MODE     %s\n", IconMode, terminal.Colorize(string(info.UIMode), terminal.BrightCyan))

	fmt.Println()
	fmt.Println(terminal.Colorize(SeparatorLight, terminal.Gray))
}

// formatDuration formatea una duración
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
