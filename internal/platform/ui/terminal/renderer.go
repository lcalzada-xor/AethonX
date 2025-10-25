// internal/platform/ui/terminal/renderer.go
package terminal

import (
	"fmt"
	"sync"
	"time"
)

// Renderer maneja el renderizado en tiempo real de múltiples líneas
type Renderer struct {
	lines      map[int]*ProgressBar // lineID -> ProgressBar
	lineOrder  []int                 // Orden de líneas para renderizado
	mu         sync.RWMutex
	ticker     *time.Ticker
	stopCh     chan struct{}
	running    bool
	lastHeight int // Altura de la última renderización
}

// NewRenderer crea un nuevo renderer
func NewRenderer() *Renderer {
	return &Renderer{
		lines:     make(map[int]*ProgressBar),
		lineOrder: []int{},
		stopCh:    make(chan struct{}),
		running:   false,
	}
}

// Start inicia el loop de renderizado
func (r *Renderer) Start(interval time.Duration) {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}

	r.ticker = time.NewTicker(interval)
	r.running = true
	r.mu.Unlock()

	// Ocultar cursor
	fmt.Print(CursorHide)

	go r.renderLoop()
}

// Stop detiene el renderer
func (r *Renderer) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return
	}

	close(r.stopCh)
	r.ticker.Stop()
	r.running = false

	// Mostrar cursor
	fmt.Print(CursorShow)
}

// RegisterLine registra una nueva progress bar en una línea
func (r *Renderer) RegisterLine(lineID int, pb *ProgressBar) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.lines[lineID]; !exists {
		r.lineOrder = append(r.lineOrder, lineID)
	}

	r.lines[lineID] = pb
}

// UnregisterLine elimina una línea
func (r *Renderer) UnregisterLine(lineID int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.lines, lineID)

	// Remover de lineOrder
	for i, id := range r.lineOrder {
		if id == lineID {
			r.lineOrder = append(r.lineOrder[:i], r.lineOrder[i+1:]...)
			break
		}
	}
}

// renderLoop es el loop principal de renderizado
func (r *Renderer) renderLoop() {
	for {
		select {
		case <-r.ticker.C:
			r.render()
		case <-r.stopCh:
			return
		}
	}
}

// render renderiza todas las líneas activas
func (r *Renderer) render() {
	r.mu.RLock()

	// Copiar datos necesarios
	lineCount := len(r.lineOrder)
	if lineCount == 0 {
		r.mu.RUnlock()
		return
	}

	// Renderizar cada línea
	output := make([]string, lineCount)
	for i, lineID := range r.lineOrder {
		if pb, exists := r.lines[lineID]; exists {
			output[i] = pb.Render()
		}
	}

	r.mu.RUnlock()

	// Mover cursor arriba si ya habíamos renderizado antes
	if r.lastHeight > 0 {
		fmt.Print(MoveCursorUp(r.lastHeight))
	}

	// Renderizar todas las líneas
	for _, line := range output {
		fmt.Print(ClearLine)
		fmt.Print(MoveCursorToColumn(1))
		fmt.Println(line)
	}

	r.lastHeight = lineCount
}

// Clear limpia la pantalla de todas las líneas renderizadas
func (r *Renderer) Clear() {
	r.mu.RLock()
	height := r.lastHeight
	r.mu.RUnlock()

	if height > 0 {
		fmt.Print(MoveCursorUp(height))
		for i := 0; i < height; i++ {
			fmt.Print(ClearLine)
			fmt.Print(MoveCursorToColumn(1))
			fmt.Println()
		}
		fmt.Print(MoveCursorUp(height))
	}
}

// RenderFinal renderiza el estado final de todas las líneas (sin clear)
func (r *Renderer) RenderFinal() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, lineID := range r.lineOrder {
		if pb, exists := r.lines[lineID]; exists {
			fmt.Println(pb.Render())
		}
	}
}
