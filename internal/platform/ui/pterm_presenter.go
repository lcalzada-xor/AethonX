// internal/platform/ui/pterm_presenter.go
package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"
)

// PTermPresenter implementa Presenter usando pterm con barras de progreso
// y métricas en tiempo real
type PTermPresenter struct {
	mu sync.Mutex

	// Configuration
	scanInfo ScanInfo
	uiMode   UIMode

	// Tracking
	stages       map[int]*StageProgress
	currentStage int
	startTime    time.Time

	// Metrics
	metricsCollector *MetricsCollector
	systemMetrics    *SystemMetrics
	discoveries      DiscoveryStats

	// Dashboard (solo para modo dashboard)
	dashboard *LiveDashboard

	// Progress bars (para modo compact)
	progressBars map[string]*pterm.ProgressbarPrinter
}

// NewPTermPresenter crea una nueva instancia del presenter
func NewPTermPresenter() *PTermPresenter {
	metricsCollector := NewMetricsCollector()
	systemMetrics := NewSystemMetrics(2 * time.Second)

	return &PTermPresenter{
		stages:           make(map[int]*StageProgress),
		metricsCollector: metricsCollector,
		systemMetrics:    systemMetrics,
		progressBars:     make(map[string]*pterm.ProgressbarPrinter),
	}
}

// Start inicia la presentación
func (p *PTermPresenter) Start(info ScanInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.scanInfo = info
	p.uiMode = info.UIMode
	if p.uiMode == "" {
		p.uiMode = UIModeCompact
	}
	p.startTime = time.Now()

	// Iniciar monitoreo de sistema
	p.systemMetrics.Start()

	// Renderizar header
	p.renderHeader()

	// Iniciar dashboard si está en modo dashboard
	if p.uiMode == UIModeDashboard {
		p.dashboard = NewLiveDashboard(p.metricsCollector, p.systemMetrics)
		p.dashboard.Start(info)
	}
}

// StartStage inicia un nuevo stage
func (p *PTermPresenter) StartStage(stage StageInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.currentStage = stage.Number

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

	p.stages[stage.Number] = stageProgress

	// Renderizar según modo
	if p.uiMode != UIModeDashboard {
		p.renderStageHeader(stage)
	}
}

// FinishStage finaliza un stage
func (p *PTermPresenter) FinishStage(stageNum int, duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	stage, exists := p.stages[stageNum]
	if !exists {
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

	if p.uiMode != UIModeDashboard {
		pterm.Println()
		fmt.Printf("%s Stage %d completed in %s\n",
			StyleSuccess.Sprint(IconSuccess),
			stageNum,
			formatDuration(duration))
		StyleSecondary.Println(SeparatorLight)
		pterm.Println()
	}
}

// StartSource inicia la ejecución de un source
func (p *PTermPresenter) StartSource(stageNum int, sourceName string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Registrar en metrics collector
	p.metricsCollector.RegisterSource(sourceName)

	// Actualizar state
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

	// Renderizar según modo
	switch p.uiMode {
	case UIModeDashboard:
		// Dashboard se actualiza automáticamente
		if p.dashboard != nil {
			p.dashboard.UpdateSource(sourceName, srcProgress)
		}

	case UIModeCompact:
		// Crear progress bar
		pb, _ := pterm.DefaultProgressbar.
			WithTotal(100).
			WithTitle(fmt.Sprintf("  %s %s", StatusRunning.Symbol(), sourceName)).
			WithTitleStyle(pterm.NewStyle(pterm.FgLightRed, pterm.Bold)).
			WithBarStyle(pterm.NewStyle(pterm.FgLightRed)).
			WithShowCount(false).
			WithShowPercentage(true).
			WithRemoveWhenDone(false).
			Start()

		p.progressBars[sourceName] = pb

	case UIModeMinimal:
		pterm.Printf("  %s %s\n", StatusRunning.Symbol(), sourceName)
	}
}

// UpdateSource actualiza el progreso de un source
func (p *PTermPresenter) UpdateSource(sourceName string, metrics ProgressMetrics) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Actualizar metrics collector
	p.metricsCollector.UpdateSource(sourceName, metrics.Current, metrics.Phase)
	if metrics.Total > 0 {
		p.metricsCollector.UpdateSourceProgress(sourceName, metrics.Current, metrics.Total)
	}

	// Actualizar progreso en stage
	for _, stage := range p.stages {
		if srcProgress, exists := stage.Sources[sourceName]; exists {
			srcProgress.ArtifactCount = metrics.Current
			srcProgress.Metrics = &metrics

			// Actualizar UI según modo
			switch p.uiMode {
			case UIModeDashboard:
				if p.dashboard != nil {
					p.dashboard.UpdateSource(sourceName, srcProgress)
				}

			case UIModeCompact:
				if pb, exists := p.progressBars[sourceName]; exists {
					if metrics.Percentage >= 0 {
						pb.UpdateTitle(fmt.Sprintf("  %s %s  %s %d  %s %.1f/s",
							StatusRunning.Symbol(),
							sourceName,
							IconArtifacts,
							metrics.Current,
							IconStats,
							metrics.Rate,
						))
						pb.Add(int(metrics.Percentage) - pb.Current)
					} else {
						// Modo indeterminado
						pb.UpdateTitle(fmt.Sprintf("  %s %s  %s %d  %s %.1f/s",
							StatusRunning.Symbol(),
							sourceName,
							IconArtifacts,
							metrics.Current,
							IconStats,
							metrics.Rate,
						))
					}
				}
			}

			break
		}
	}
}

// UpdateSourcePhase actualiza solo la fase de un source
func (p *PTermPresenter) UpdateSourcePhase(sourceName string, phase string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.metricsCollector.UpdateSource(sourceName, 0, phase)
}

// FinishSource finaliza un source
func (p *PTermPresenter) FinishSource(sourceName string, status Status, duration time.Duration, artifactCount int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Actualizar state
	for _, stage := range p.stages {
		if srcProgress, exists := stage.Sources[sourceName]; exists {
			srcProgress.Status = status
			srcProgress.Duration = duration
			srcProgress.ArtifactCount = artifactCount
			break
		}
	}

	// Renderizar según modo
	switch p.uiMode {
	case UIModeDashboard:
		// Dashboard se actualiza automáticamente
		// No hacer nada aquí

	case UIModeCompact:
		if pb, exists := p.progressBars[sourceName]; exists {
			if status == StatusSuccess {
				pb.Add(100 - pb.Current) // Completar
			}
			pb.Stop()
			delete(p.progressBars, sourceName)
		}

		// Renderizar línea final
		p.renderSourceCompleteLine(sourceName, status, duration, artifactCount)

	case UIModeMinimal:
		p.renderSourceCompleteLine(sourceName, status, duration, artifactCount)
	}
}

// UpdateDiscoveries actualiza estadísticas de descubrimiento
func (p *PTermPresenter) UpdateDiscoveries(discoveries DiscoveryStats) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.discoveries = discoveries

	// Actualizar metrics globales
	artifactsByType := make(map[string]int)
	artifactsByType["subdomain"] = discoveries.Subdomains
	artifactsByType["ip"] = discoveries.IPs
	artifactsByType["url"] = discoveries.URLs
	artifactsByType["email"] = discoveries.Emails

	p.metricsCollector.UpdateGlobalMetrics(discoveries.Total, discoveries.Unique, artifactsByType)

	// Actualizar dashboard
	if p.uiMode == UIModeDashboard && p.dashboard != nil {
		p.dashboard.UpdateDiscoveries(discoveries)
	}
}

// Info muestra un mensaje informativo
func (p *PTermPresenter) Info(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.uiMode != UIModeDashboard {
		StyleInfo.Printf("%s %s\n", IconInfo, msg)
	}
}

// Warning muestra una advertencia
func (p *PTermPresenter) Warning(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.uiMode != UIModeDashboard {
		StyleWarning.Printf("%s %s\n", IconWarning, msg)
	}
}

// Error muestra un error
func (p *PTermPresenter) Error(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.uiMode != UIModeDashboard {
		StyleError.Printf("%s %s\n", IconError, msg)
	}
}

// Finish finaliza la presentación
func (p *PTermPresenter) Finish(stats ScanStats) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Detener dashboard
	if p.dashboard != nil {
		p.dashboard.Stop()
	}

	// Detener todas las progress bars
	for _, pb := range p.progressBars {
		pb.Stop()
	}

	// Renderizar resumen final
	p.renderSummary(stats)
}

// Close limpia recursos
func (p *PTermPresenter) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Detener system metrics
	p.systemMetrics.Stop()

	// Detener dashboard
	if p.dashboard != nil {
		p.dashboard.Stop()
	}

	// Detener progress bars
	for _, pb := range p.progressBars {
		pb.Stop()
	}

	p.progressBars = make(map[string]*pterm.ProgressbarPrinter)

	return nil
}

// Render helpers

func (p *PTermPresenter) renderHeader() {
	pterm.Println()
	EmberOrange.Println(SeparatorHeavy)
	pterm.Println()

	StylePrimary.Println("    █████╗ ███████╗████████╗██╗  ██╗ ██████╗ ███╗   ██╗")
	StylePrimary.Println("   ██╔══██╗██╔════╝╚══██╔══╝██║  ██║██╔═══██╗████╗  ██║")
	StylePrimary.Println("   ███████║█████╗     ██║   ███████║██║   ██║██╔██╗ ██║")
	StylePrimary.Println("   ██╔══██║██╔══╝     ██║   ██╔══██║██║   ██║██║╚██╗██║")
	StylePrimary.Println("   ██║  ██║███████╗   ██║   ██║  ██║╚██████╔╝██║ ╚████║")
	StylePrimary.Println("   ╚═╝  ╚═╝╚══════╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝")
	pterm.Println()

	EmberOrange.Println("          Illuminating the Digital Underworld")
	pterm.Println()
	EmberOrange.Println(SeparatorHeavy)
	pterm.Println()

	StylePrimary.Println("▸ SCAN CONFIGURATION")
	pterm.Println()

	fmt.Printf("  %s TARGET      %s\n", IconTarget, StyleText.Sprint(p.scanInfo.Target))
	fmt.Printf("  %s MODE        %s\n", IconMode, StyleAccent.Sprint(p.scanInfo.Mode))
	fmt.Printf("  %s WORKERS     %s\n", IconWorkers, StyleText.Sprintf("%d", p.scanInfo.Workers))
	fmt.Printf("  %s TIMEOUT     %s\n", IconTime, StyleText.Sprintf("%ds", p.scanInfo.TimeoutSeconds))
	fmt.Printf("  %s STREAMING   %s\n", IconInfo, boolToString(p.scanInfo.StreamingOn))
	fmt.Printf("  %s UI MODE     %s\n", IconMode, StyleAccent.Sprint(p.scanInfo.UIMode))

	pterm.Println()
	StyleSecondary.Println(SeparatorLight)
	pterm.Println()
}

func (p *PTermPresenter) renderStageHeader(stage StageInfo) {
	stageTitle := fmt.Sprintf("%s STAGE %d/%d: %s",
		IconStage,
		stage.Number,
		stage.TotalStages,
		stage.Name,
	)

	StylePrimary.Println(stageTitle)
	pterm.Println()
}

func (p *PTermPresenter) renderSourceCompleteLine(sourceName string, status Status, duration time.Duration, artifactCount int) {
	symbol := status.Symbol()
	style := status.Style()

	line := fmt.Sprintf("  %s %s", symbol, style.Sprint(sourceName))

	if duration > 0 {
		line += fmt.Sprintf(" %s", StyleSecondary.Sprintf("(%s)", formatDuration(duration)))
	}

	if artifactCount > 0 {
		line += fmt.Sprintf(" %s %s",
			IconArtifacts,
			StyleAccent.Sprintf("%d artifacts", artifactCount),
		)
	}

	pterm.Println(line)
}

func (p *PTermPresenter) renderSummary(stats ScanStats) {
	pterm.Println()
	EmberOrange.Println(SeparatorHeavy)
	pterm.Println()

	StyleSuccess.Print("  ⚡ SCAN COMPLETE")
	pterm.Println()
	pterm.Println()

	StylePrimary.Println("▸ SCAN RESULTS")
	pterm.Println()

	fmt.Printf("  %s DURATION       %s\n",
		IconTime,
		StyleSuccess.Sprint(formatDuration(stats.TotalDuration)),
	)

	fmt.Printf("  %s ARTIFACTS     %s total, %s unique\n",
		IconArtifacts,
		StyleAccent.Sprintf("%d", stats.TotalArtifacts),
		StyleWarning.Sprintf("%d", stats.UniqueArtifacts),
	)

	successMsg := fmt.Sprintf("%s SOURCES       %s succeeded",
		IconSources,
		StyleSuccess.Sprintf("%d", stats.SourcesSucceeded),
	)

	if stats.SourcesFailed > 0 {
		successMsg += fmt.Sprintf(", %s failed",
			StyleError.Sprintf("%d", stats.SourcesFailed),
		)
	}

	fmt.Println("  " + successMsg)

	if stats.RelationshipsBuilt > 0 {
		fmt.Printf("  %s RELATIONS     %s\n",
			IconInfo,
			StyleInfo.Sprintf("%d", stats.RelationshipsBuilt),
		)
	}

	pterm.Println()

	// Tabla de artifacts por tipo
	if len(stats.ArtifactsByType) > 0 {
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

	StyleSecondary.Print("  AethonX ")
	StyleSecondary.Print("— Illuminating the Digital Underworld ")
	StylePrimary.Println("▲")
	pterm.Println()
}

// Helper functions

// stripANSI removes ANSI color codes from a string
func stripANSI(str string) string {
	// Simple implementation - in production use a proper ANSI stripper
	result := strings.Builder{}
	inEscape := false

	for _, r := range str {
		if r == '\033' {
			inEscape = true
			continue
		}

		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}

		result.WriteRune(r)
	}

	return result.String()
}
