// internal/core/usecases/source_task.go
package usecases

import (
	"context"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
)

// SourceTask adapta un ports.Source a workerpool.Task.
type SourceTask struct {
	source   ports.Source
	target   domain.Target
	priority int
	weight   int

	// Input artifacts from previous stages (filtered)
	input *domain.ScanResult

	// Result storage
	result *domain.ScanResult
	err    error
}

// NewSourceTask crea una nueva SourceTask sin inputs.
func NewSourceTask(source ports.Source, target domain.Target, priority, weight int) *SourceTask {
	return &SourceTask{
		source:   source,
		target:   target,
		priority: priority,
		weight:   weight,
		input:    nil, // Sin inputs
	}
}

// NewSourceTaskWithInput crea una nueva SourceTask con input artifacts.
func NewSourceTaskWithInput(source ports.Source, target domain.Target, priority, weight int, input *domain.ScanResult) *SourceTask {
	return &SourceTask{
		source:   source,
		target:   target,
		priority: priority,
		weight:   weight,
		input:    input,
	}
}

// Execute ejecuta la source con o sin inputs según corresponda.
func (st *SourceTask) Execute(ctx context.Context) error {
	// Verificar si la source implementa InputConsumer y tiene inputs
	if consumer, ok := st.source.(ports.InputConsumer); ok && st.input != nil {
		st.result, st.err = consumer.RunWithInput(ctx, st.target, st.input)
	} else {
		// Fallback: ejecutar sin inputs
		st.result, st.err = st.source.Run(ctx, st.target)
	}
	return st.err
}

// Priority retorna la prioridad de la tarea.
func (st *SourceTask) Priority() int {
	return st.priority
}

// Weight retorna el peso/costo estimado de la tarea.
func (st *SourceTask) Weight() int {
	return st.weight
}

// Name retorna el nombre de la tarea (nombre de la source).
func (st *SourceTask) Name() string {
	return st.source.Name()
}

// Result retorna el resultado de la ejecución.
func (st *SourceTask) Result() (*domain.ScanResult, error) {
	return st.result, st.err
}

// Source retorna la source subyacente.
func (st *SourceTask) Source() ports.Source {
	return st.source
}

// estimateSourceWeight estima el peso/costo de una source basado en su tipo y modo.
func estimateSourceWeight(source ports.Source) int {
	// Heurística simple basada en tipo y modo
	baseWeight := 50

	switch source.Type() {
	case domain.SourceTypeAPI:
		baseWeight = 30 // APIs suelen ser rápidas
	case domain.SourceTypeCLI:
		baseWeight = 70 // CLI tools más lentas
	case domain.SourceTypeBuiltin:
		baseWeight = 20 // Builtins muy rápidas
	}

	// Ajustar por modo
	switch source.Mode() {
	case domain.SourceModePassive:
		// No ajuste
	case domain.SourceModeActive:
		baseWeight += 20 // Active scans más lentas
	case domain.SourceModeBoth:
		baseWeight += 10 // Hybrid scans
	}

	// Cap entre 10-100
	if baseWeight < 10 {
		baseWeight = 10
	}
	if baseWeight > 100 {
		baseWeight = 100
	}

	return baseWeight
}

// EstimateSourceWeight es la función pública para estimar peso.
func EstimateSourceWeight(source ports.Source) int {
	return estimateSourceWeight(source)
}
