package subfinder

import (
	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/registry"
)

// Auto-registration on package import using registry helpers
func init() {
	if err := registry.Global().Register(
		"subfinder",
		factory,
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
		logx.New().Warn("failed to register subfinder source", "error", err.Error())
	}
}

// factory creates a new SubfinderSource from SourceConfig using registry helpers
func factory(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
	// Extract custom config using registry helpers (type-safe, no manual nil checks)
	execPath := registry.GetStringConfig(cfg.Custom, "exec_path", "subfinder")
	threads := registry.GetIntConfig(cfg.Custom, "threads", defaultThreads)
	rateLimit := registry.GetIntConfig(cfg.Custom, "rate_limit", 0)
	sources := registry.GetSliceConfig(cfg.Custom, "sources", []string{"alienvault", "anubis", "commoncrawl", "crtsh", "digitorus", "dnsdumpster", "hackertarget", "rapiddns", "sitedossier", "waybackarchive"})

	// Use configured timeout or default
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	return NewWithConfig(logger, execPath, timeout, threads, rateLimit, sources), nil
}
