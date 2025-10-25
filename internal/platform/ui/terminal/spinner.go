// internal/platform/ui/terminal/spinner.go
package terminal

import (
	"sync"
	"time"
)

// SpinnerSequences define las secuencias temáticas de spinners
var SpinnerSequences = map[string][]string{
	"ember":  {"◉", "◎", "○", "◎"}, // Brasas pulsantes (default)
	"flame":  {"▰", "▱", "▰", "▱"}, // Llama oscilante
	"pulse":  {"●", "◉", "○", "◉"}, // Pulso
	"scroll": {"◐", "◓", "◑", "◒"}, // Pergamino girando
}

// AnimatedSpinner proporciona un spinner que rota automáticamente
// en un goroutine independiente para dar feedback visual continuo.
type AnimatedSpinner struct {
	sequence   []string      // Secuencia de símbolos a rotar
	currentIdx int           // Índice actual en la secuencia
	ticker     *time.Ticker  // Ticker para animación
	stop       chan struct{} // Canal para detener animación
	mu         sync.RWMutex  // Protección thread-safe
	running    bool          // Estado de ejecución
}

// NewAnimatedSpinner crea un nuevo spinner con la secuencia especificada.
// theme puede ser: "ember", "flame", "pulse", "scroll"
func NewAnimatedSpinner(theme string) *AnimatedSpinner {
	sequence, exists := SpinnerSequences[theme]
	if !exists {
		// Default a "ember" si el tema no existe
		sequence = SpinnerSequences["ember"]
	}

	return &AnimatedSpinner{
		sequence:   sequence,
		currentIdx: 0,
		stop:       make(chan struct{}),
		running:    false,
	}
}

// NewDefaultSpinner crea un spinner con el tema por defecto ("ember")
func NewDefaultSpinner() *AnimatedSpinner {
	return NewAnimatedSpinner("ember")
}

// Start inicia la animación del spinner en un goroutine.
// interval define la velocidad de rotación (recomendado: 80-150ms)
func (s *AnimatedSpinner) Start(interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return // Ya está corriendo
	}

	s.ticker = time.NewTicker(interval)
	s.running = true

	go s.animate()
}

// animate es el loop interno que rota el spinner
func (s *AnimatedSpinner) animate() {
	for {
		select {
		case <-s.ticker.C:
			s.mu.Lock()
			s.currentIdx = (s.currentIdx + 1) % len(s.sequence)
			s.mu.Unlock()

		case <-s.stop:
			s.ticker.Stop()
			return
		}
	}
}

// Stop detiene la animación del spinner
func (s *AnimatedSpinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	close(s.stop)
	s.running = false
}

// Current devuelve el símbolo actual del spinner (thread-safe)
func (s *AnimatedSpinner) Current() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.sequence) == 0 {
		return "◉" // Fallback
	}

	return s.sequence[s.currentIdx]
}

// IsRunning devuelve si el spinner está animándose
func (s *AnimatedSpinner) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Reset reinicia el spinner al primer símbolo
func (s *AnimatedSpinner) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentIdx = 0
}

// SetTheme cambia la secuencia del spinner en tiempo real
func (s *AnimatedSpinner) SetTheme(theme string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sequence, exists := SpinnerSequences[theme]; exists {
		s.sequence = sequence
		s.currentIdx = 0
	}
}
