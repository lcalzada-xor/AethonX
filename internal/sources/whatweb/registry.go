package whatweb

import (
	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/registry"
)

// Auto-registration on package import using registry helpers
func init() {
	if err := registry.Global().Register(
		"whatweb",
		factory,
		ports.SourceMetadata{
			Name:         "whatweb",
			Description:  "Web technology fingerprinting via WhatWeb CLI tool",
			Version:      "1.0.0",
			Author:       "AethonX",
			Mode:         domain.SourceModeActive,
			Type:         domain.SourceTypeCLI,
			RequiresAuth: false,
			RateLimit:    0, // Managed by threads parameter

			// Dependency declaration for stage-based execution
			InputArtifacts: []domain.ArtifactType{
				domain.ArtifactTypeSubdomain,
				domain.ArtifactTypeURL,
			},
			OutputArtifacts: []domain.ArtifactType{
				domain.ArtifactTypeTechnology,
				domain.ArtifactTypeService,
				domain.ArtifactTypeIP,
				domain.ArtifactTypeEmail, // Email addresses
			},
			Priority:  16, // Stage 2 - after discovery (10), before crawl (20+)
			StageHint: 2,  // Stage 2: Probing/Enrichment
		},
	); err != nil {
		// Log error but don't panic - allow application to start
		logx.New().Warn("failed to register whatweb source", "error", err.Error())
	}
}

// factory creates a new WhatWebSource from SourceConfig using registry helpers
func factory(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
	// Extract custom config using registry helpers (type-safe, no manual nil checks)
	execPath := registry.GetStringConfig(cfg.Custom, "exec_path", "whatweb")
	aggression := registry.GetIntConfig(cfg.Custom, "aggression", 1)
	threads := registry.GetIntConfig(cfg.Custom, "threads", defaultThreads)
	userAgent := registry.GetStringConfig(cfg.Custom, "user_agent", "AethonX/1.0 (Web reconnaissance tool)")

	// Use configured timeout or default
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	return NewWithConfig(logger, execPath, timeout, aggression, threads, userAgent), nil
}
