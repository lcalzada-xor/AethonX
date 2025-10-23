// internal/platform/ui/noop_presenter.go
package ui

import "time"

// NoopPresenter es una implementación vacía del Presenter
// que no produce ninguna salida. Útil para modo quiet o headless.
type NoopPresenter struct{}

// NewNoopPresenter crea una instancia del presenter sin salida
func NewNoopPresenter() *NoopPresenter {
	return &NoopPresenter{}
}

// Start no hace nada
func (n *NoopPresenter) Start(info ScanInfo) {}

// StartStage no hace nada
func (n *NoopPresenter) StartStage(stage StageInfo) {}

// FinishStage no hace nada
func (n *NoopPresenter) FinishStage(stageNum int, duration time.Duration) {}

// StartSource no hace nada
func (n *NoopPresenter) StartSource(stageNum int, sourceName string) {}

// UpdateSource no hace nada
func (n *NoopPresenter) UpdateSource(sourceName string, metrics ProgressMetrics) {}

// UpdateSourcePhase no hace nada
func (n *NoopPresenter) UpdateSourcePhase(sourceName string, phase string) {}

// FinishSource no hace nada
func (n *NoopPresenter) FinishSource(sourceName string, status Status, duration time.Duration, artifactCount int) {
}

// UpdateDiscoveries no hace nada
func (n *NoopPresenter) UpdateDiscoveries(discoveries DiscoveryStats) {}

// Info no hace nada
func (n *NoopPresenter) Info(msg string) {}

// Warning no hace nada
func (n *NoopPresenter) Warning(msg string) {}

// Error no hace nada
func (n *NoopPresenter) Error(msg string) {}

// Finish no hace nada
func (n *NoopPresenter) Finish(stats ScanStats) {}

// Close no hace nada
func (n *NoopPresenter) Close() error {
	return nil
}
