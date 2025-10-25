// internal/platform/ui/global_progress_test.go
package ui

import (
	"sync"
	"testing"
	"time"
)

func TestGlobalProgress_Start(t *testing.T) {
	gp := NewGlobalProgress()

	gp.Start(5)

	if gp.totalSources != 5 {
		t.Errorf("Expected totalSources=5, got %d", gp.totalSources)
	}

	if gp.completedSources != 0 {
		t.Errorf("Expected completedSources=0, got %d", gp.completedSources)
	}

	if !gp.isActive {
		t.Error("Expected isActive=true")
	}

	if gp.lineRendered {
		t.Error("Expected lineRendered=false initially")
	}
}

func TestGlobalProgress_UpdateCurrent(t *testing.T) {
	gp := NewGlobalProgress()
	gp.Start(3)

	gp.UpdateCurrent("crtsh")

	if gp.currentSource != "crtsh" {
		t.Errorf("Expected currentSource='crtsh', got '%s'", gp.currentSource)
	}
}

func TestGlobalProgress_IncrementCompleted(t *testing.T) {
	gp := NewGlobalProgress()
	gp.Start(3)

	gp.IncrementCompleted()
	if gp.completedSources != 1 {
		t.Errorf("Expected completedSources=1, got %d", gp.completedSources)
	}

	gp.IncrementCompleted()
	if gp.completedSources != 2 {
		t.Errorf("Expected completedSources=2, got %d", gp.completedSources)
	}
}

func TestGlobalProgress_Stop(t *testing.T) {
	gp := NewGlobalProgress()
	gp.Start(3)

	if !gp.isActive {
		t.Error("Expected isActive=true after Start")
	}

	gp.Stop()

	if gp.isActive {
		t.Error("Expected isActive=false after Stop")
	}
}

func TestGlobalProgress_GetProgress(t *testing.T) {
	gp := NewGlobalProgress()
	gp.Start(5)

	completed, total := gp.GetProgress()
	if completed != 0 || total != 5 {
		t.Errorf("Expected (0, 5), got (%d, %d)", completed, total)
	}

	gp.IncrementCompleted()
	gp.IncrementCompleted()

	completed, total = gp.GetProgress()
	if completed != 2 || total != 5 {
		t.Errorf("Expected (2, 5), got (%d, %d)", completed, total)
	}
}

func TestGlobalProgress_IsActive(t *testing.T) {
	gp := NewGlobalProgress()

	// Antes de Start
	if gp.IsActive() {
		t.Error("Expected IsActive=false before Start")
	}

	gp.Start(3)

	// Después de Start
	if !gp.IsActive() {
		t.Error("Expected IsActive=true after Start")
	}

	gp.Stop()

	// Después de Stop
	if gp.IsActive() {
		t.Error("Expected IsActive=false after Stop")
	}
}

func TestGlobalProgress_SpinnerAdvances(t *testing.T) {
	gp := NewGlobalProgress()
	gp.Start(3)
	defer gp.Stop()

	initialFrame := gp.currentFrame

	// Esperar a que el ticker del spinner avance el frame (250ms)
	time.Sleep(300 * time.Millisecond)

	gp.mu.RLock()
	firstFrame := gp.currentFrame
	gp.mu.RUnlock()

	if firstFrame == initialFrame {
		t.Error("Expected spinner frame to advance after 300ms")
	}

	// Esperar otro tick
	time.Sleep(300 * time.Millisecond)

	gp.mu.RLock()
	secondFrame := gp.currentFrame
	gp.mu.RUnlock()

	if secondFrame == firstFrame {
		t.Error("Expected spinner frame to advance after second tick")
	}
}

func TestGlobalProgress_ThreadSafety(t *testing.T) {
	gp := NewGlobalProgress()
	gp.Start(100)

	var wg sync.WaitGroup

	// Goroutines concurrentes incrementando completed
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			gp.IncrementCompleted()
		}()
	}

	// Goroutines concurrentes actualizando current source
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			gp.UpdateCurrent("source-" + string(rune(id)))
		}(i)
	}

	// Goroutines leyendo progreso
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = gp.GetProgress()
			_ = gp.IsActive()
		}()
	}

	wg.Wait()

	// Verificar que completedSources llegó a 50
	completed, _ := gp.GetProgress()
	if completed != 50 {
		t.Errorf("Expected completedSources=50 after concurrent increments, got %d", completed)
	}
}

func TestGlobalProgress_ClearResetsLineRendered(t *testing.T) {
	gp := NewGlobalProgress()
	gp.Start(3)

	// Simular que ya renderizamos
	gp.lineRendered = true

	gp.Clear()

	if gp.lineRendered {
		t.Error("Expected lineRendered=false after Clear")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{100 * time.Millisecond, "100ms"},
		{500 * time.Millisecond, "500ms"},
		{1500 * time.Millisecond, "1.5s"},
		{5 * time.Second, "5.0s"},
		{65 * time.Second, "1m5s"},
		{125 * time.Second, "2m5s"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %s, expected %s", tt.duration, result, tt.expected)
		}
	}
}
