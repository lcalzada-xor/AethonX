// internal/core/orchestrator.go
package core

import (
	"context"
	"sync"

	"aethonx/internal/domain"
)

type Orchestrator struct {
	Sources []Source
	Limit   int // concurrencia máxima
}

func (o *Orchestrator) Run(ctx context.Context, t Target) (RunResult, error) {
	selected := make([]Source, 0, len(o.Sources))
	for _, s := range o.Sources {
		if t.Active {
			selected = append(selected, s) // activa: todas (pasivas + activas)
			continue
		}
		if s.Mode() == PassiveOnly {
			selected = append(selected, s)
		}
	}

	if o.Limit <= 0 {
		o.Limit = 1
	}
	sem := make(chan struct{}, o.Limit)
	wg := sync.WaitGroup{}
	type sr struct {
		res RunResult
		err error
	}
	out := make(chan sr, len(selected))

	for _, s := range selected {
		s := s
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			res, err := s.Run(ctx, t)
			out <- sr{res: res, err: err}
		}()
	}

	wg.Wait()
	close(out)

	var merged RunResult
	for item := range out {
		if item.err != nil {
			merged.Warnings = append(merged.Warnings, item.err.Error())
			continue
		}
		merged.Artifacts = append(merged.Artifacts, item.res.Artifacts...)
		merged.Warnings = append(merged.Warnings, item.res.Warnings...)
	}

	// 🔹 Normaliza y deduplica antes de devolver
	merged.Artifacts = domain.DedupeAndNormalize(merged.Artifacts)
	return merged, nil
}
