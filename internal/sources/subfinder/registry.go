package subfinder

import (
	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/registry"
)

// Auto-registro de la source al importar el package
func init() {
	if err := registry.Global().Register(
		"subfinder",
		func(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
			// Extract custom config
			execPath := "subfinder"
			allSources := true
			threads := defaultThreads
			rateLimit := 0
			sources := []string{}

			if cfg.Custom != nil {
				if v, ok := cfg.Custom["exec_path"].(string); ok && v != "" {
					execPath = v
				}
				if v, ok := cfg.Custom["all_sources"].(bool); ok {
					allSources = v
				}
				if v, ok := cfg.Custom["threads"].(int); ok {
					threads = v
				}
				if v, ok := cfg.Custom["rate_limit"].(int); ok {
					rateLimit = v
				}
				if v, ok := cfg.Custom["sources"].([]string); ok {
					sources = v
				}
			}

			// Use configured timeout or default
			timeout := cfg.Timeout
			if timeout == 0 {
				timeout = defaultTimeout
			}

			return NewWithConfig(logger, execPath, timeout, threads, rateLimit, allSources, sources), nil
		},
		ports.SourceMetadata{
			Name:         "subfinder",
			Description:  "Multi-source subdomain discovery via Project Discovery's subfinder",
			Version:      "1.0.0",
			Author:       "AethonX",
			Mode:         domain.SourceModePassive,
			Type:         domain.SourceTypeCLI,
			RequiresAuth: false,
			RateLimit:    0, // Managed internally by subfinder

			// Dependency declaration (Stage 0: no inputs)
			InputArtifacts:  []domain.ArtifactType{}, // No inputs = Stage 0
			OutputArtifacts: []domain.ArtifactType{
				domain.ArtifactTypeSubdomain,
			},
			Priority:  10, // High priority (passive discovery, same as crtsh)
			StageHint: 0,  // Stage 0 explicit
		},
	); err != nil {
		// Log error but don't panic - allow application to start
		// Registry will skip this source during Build()
		logx.New().Warn("failed to register subfinder source", "error", err.Error())
	}
}
