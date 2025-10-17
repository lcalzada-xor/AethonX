// internal/core/pipeline.go
package core

import "context"

type SourceMode int

const (
	PassiveOnly SourceMode = iota
	ActiveCapable
)

type Target struct {
	RootDomain string
	Active     bool
}

type Artifact struct {
	Type   string            // "domain","subdomain","ip","cert","email", etc.
	Value  string            // valor normalizado
	Source string            // nombre de la fuente
	Meta   map[string]string // datos adicionales
}

type RunResult struct {
	Artifacts []Artifact
	Warnings  []string
}

type Source interface {
	Name() string
	Mode() SourceMode
	Run(ctx context.Context, t Target) (RunResult, error)
}
