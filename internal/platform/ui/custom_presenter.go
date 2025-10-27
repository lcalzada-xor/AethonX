// internal/platform/ui/custom_presenter.go
package ui

import (
	"fmt"
	"sync"
	"time"

	"aethonx/internal/platform/ui/terminal"
)

// CustomPresenter implementa Presenter con sistema de renderizado simple
type CustomPresenter struct {
	mu             sync.RWMutex
	stages         map[int]*StageProgress
	scanInfo       ScanInfo
	startTime      time.Time
	currentResults []sourceResult  // Acumular resultados del stage actual
	globalProgress *GlobalProgress // Barra de progreso global
}

// sourceResult almacena el resultado de un source para mostrar al final del stage
type sourceResult struct {
	name     string
	status   Status
	duration time.Duration
	count    int
	summary  *SourceSummary
}

// NewCustomPresenter crea un nuevo presenter custom
func NewCustomPresenter() *CustomPresenter {
	return &CustomPresenter{
		stages:         make(map[int]*StageProgress),
		currentResults: make([]sourceResult, 0),
		globalProgress: NewGlobalProgress(),
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

	// Inicializar lista de sources en GlobalProgress
	c.globalProgress.InitializeSources(stage.Sources)

	// Iniciar GlobalProgress con el número total de sources
	c.globalProgress.Start(len(stage.Sources))

	// Renderizar línea inicial de progreso
	c.globalProgress.Render()
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

	// Copiar resultados para impresión
	results := make([]sourceResult, len(c.currentResults))
	copy(results, c.currentResults)

	// Limpiar para siguiente stage
	c.currentResults = nil

	c.mu.Unlock()

	// Detener y limpiar GlobalProgress
	c.globalProgress.Stop()
	c.globalProgress.Render() // Renderizar estado final (100%)
	time.Sleep(400 * time.Millisecond) // Dar tiempo para ver el 100%
	c.globalProgress.Clear()

	// Mostrar TODOS los resultados acumulados
	fmt.Println()
	for _, result := range results {
		symbol := result.status.Symbol()
		color := result.status.Color()

		// Construir línea base
		line := fmt.Sprintf("  %s %s %s ◆ %d artifacts",
			terminal.Colorize(symbol, color),
			terminal.Colorize(result.name, terminal.BrightCyan),
			terminal.Colorize(fmt.Sprintf("(%s)", formatDuration(result.duration)), terminal.Gray),
			result.count,
		)

		// Añadir summary si existe
		if result.summary != nil && result.summary.Summary != "" {
			summaryColor := terminal.Gray
			if result.status == StatusError {
				summaryColor = terminal.BrightRed
			}
			line += fmt.Sprintf(" | %s", terminal.Colorize(result.summary.Summary, summaryColor))
		}

		fmt.Println(line)
	}

	// Mostrar resumen del stage
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

	// Buscar en qué stage está este source (ignorar stageNum que puede ser incorrecto)
	var stage *StageProgress
	for _, s := range c.stages {
		if _, exists := s.Sources[sourceName]; exists {
			stage = s
			break
		}
	}

	if stage == nil {
		c.mu.Unlock()
		// Aún así actualizar GlobalProgress para que se vea el spinner
		c.globalProgress.UpdateCurrent(sourceName)
		return
	}

	srcProgress := stage.Sources[sourceName]
	srcProgress.Status = StatusRunning
	srcProgress.StartTime = time.Now()

	c.mu.Unlock()

	// Actualizar GlobalProgress con el source actual (UpdateCurrent ya renderiza)
	c.globalProgress.UpdateCurrent(sourceName)
}

// UpdateSource actualiza el progreso de un source
func (c *CustomPresenter) UpdateSource(sourceName string, metrics ProgressMetrics) {
	c.mu.Lock()

	// Actualizar métricas en state
	totalArtifacts := 0
	for _, stage := range c.stages {
		if srcProgress, exists := stage.Sources[sourceName]; exists {
			srcProgress.ArtifactCount = metrics.Current
			if srcProgress.Metrics == nil {
				srcProgress.Metrics = &ProgressMetrics{}
			}
			*srcProgress.Metrics = metrics
		}
		// Sumar artifacts de todas las sources
		for _, src := range stage.Sources {
			totalArtifacts += src.ArtifactCount
		}
	}

	c.mu.Unlock()

	// Actualizar contador de artifacts en GlobalProgress
	c.globalProgress.UpdateArtifactCount(totalArtifacts)
	// No llamar Render() aquí porque el spinner ya lo hace cada 200ms
}

// UpdateSourcePhase actualiza solo la fase de un source
func (c *CustomPresenter) UpdateSourcePhase(sourceName string, phase string) {
	// No-op en custom presenter (simplificado)
}

// FinishSource finaliza un source
func (c *CustomPresenter) FinishSource(sourceName string, status Status, duration time.Duration, artifactCount int, summary *SourceSummary) {
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

	// Acumular resultado para mostrar al final del stage
	c.currentResults = append(c.currentResults, sourceResult{
		name:     sourceName,
		status:   status,
		duration: duration,
		count:    artifactCount,
		summary:  summary,
	})

	c.mu.Unlock()

	// Actualizar status del source en GlobalProgress
	c.globalProgress.UpdateSourceStatus(sourceName, status)

	// Incrementar contador (UpdateSourceStatus ya renderizó)
	c.globalProgress.IncrementCompleted()
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
	// Limpiar GlobalProgress si hay línea renderizada
	if c.globalProgress != nil {
		c.globalProgress.Clear()
	}
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
