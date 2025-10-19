// internal/platform/workerpool/schedulers.go
package workerpool

import (
	"sort"
)

// PriorityScheduler ordena tareas por prioridad (mayor primero).
type PriorityScheduler struct{}

// NewPriorityScheduler crea un scheduler basado en prioridad.
func NewPriorityScheduler() *PriorityScheduler {
	return &PriorityScheduler{}
}

// Schedule ordena por prioridad descendente.
func (s *PriorityScheduler) Schedule(tasks []Task) []Task {
	scheduled := make([]Task, len(tasks))
	copy(scheduled, tasks)

	sort.Slice(scheduled, func(i, j int) bool {
		// Mayor prioridad primero
		if scheduled[i].Priority() != scheduled[j].Priority() {
			return scheduled[i].Priority() > scheduled[j].Priority()
		}
		// Si misma prioridad, menor peso primero (tasks rápidas)
		return scheduled[i].Weight() < scheduled[j].Weight()
	})

	return scheduled
}

// Name retorna el nombre del scheduler.
func (s *PriorityScheduler) Name() string {
	return "priority"
}

// WeightedScheduler ordena tareas por peso/costo (menor primero).
// Estrategia: ejecutar tasks rápidas primero para maximizar throughput.
type WeightedScheduler struct{}

// NewWeightedScheduler crea un scheduler basado en peso.
func NewWeightedScheduler() *WeightedScheduler {
	return &WeightedScheduler{}
}

// Schedule ordena por peso ascendente (rápidas primero).
func (s *WeightedScheduler) Schedule(tasks []Task) []Task {
	scheduled := make([]Task, len(tasks))
	copy(scheduled, tasks)

	sort.Slice(scheduled, func(i, j int) bool {
		// Menor peso primero (tasks rápidas)
		if scheduled[i].Weight() != scheduled[j].Weight() {
			return scheduled[i].Weight() < scheduled[j].Weight()
		}
		// Si mismo peso, mayor prioridad primero
		return scheduled[i].Priority() > scheduled[j].Priority()
	})

	return scheduled
}

// Name retorna el nombre del scheduler.
func (s *WeightedScheduler) Name() string {
	return "weighted"
}

// HybridScheduler combina prioridad y peso con un factor de balance.
// BalanceFactor [0.0-1.0]: 0.0 = solo prioridad, 1.0 = solo peso
type HybridScheduler struct {
	BalanceFactor float64
}

// NewHybridScheduler crea un scheduler híbrido.
func NewHybridScheduler(balanceFactor float64) *HybridScheduler {
	if balanceFactor < 0.0 {
		balanceFactor = 0.0
	}
	if balanceFactor > 1.0 {
		balanceFactor = 1.0
	}

	return &HybridScheduler{
		BalanceFactor: balanceFactor,
	}
}

// Schedule ordena por score híbrido.
func (s *HybridScheduler) Schedule(tasks []Task) []Task {
	scheduled := make([]Task, len(tasks))
	copy(scheduled, tasks)

	// Calcular scores
	scores := make(map[Task]float64, len(tasks))
	for _, task := range tasks {
		// Score = (priority * (1 - balance)) - (weight * balance)
		// Mayor score = más prioritario
		priorityScore := float64(task.Priority()) * (1.0 - s.BalanceFactor)
		weightPenalty := float64(task.Weight()) * s.BalanceFactor
		scores[task] = priorityScore - weightPenalty
	}

	sort.Slice(scheduled, func(i, j int) bool {
		return scores[scheduled[i]] > scores[scheduled[j]]
	})

	return scheduled
}

// Name retorna el nombre del scheduler.
func (s *HybridScheduler) Name() string {
	return "hybrid"
}

// FIFOScheduler no reordena (First In First Out).
type FIFOScheduler struct{}

// NewFIFOScheduler crea un scheduler FIFO.
func NewFIFOScheduler() *FIFOScheduler {
	return &FIFOScheduler{}
}

// Schedule retorna tasks en el orden original.
func (s *FIFOScheduler) Schedule(tasks []Task) []Task {
	scheduled := make([]Task, len(tasks))
	copy(scheduled, tasks)
	return scheduled
}

// Name retorna el nombre del scheduler.
func (s *FIFOScheduler) Name() string {
	return "fifo"
}
