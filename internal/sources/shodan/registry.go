// internal/sources/shodan/registry.go
package shodan

import (
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/registry"
)

// Auto-registration: This init() function is called when the package is imported.
// It registers the Shodan source with the global registry.
func init() {
	if err := registry.Global().Register(
		"shodan",
		factory,
		ports.SourceMetadata{
			Name:        "shodan",
			Description: "Internet-wide asset discovery via Shodan search engine",
			Version:     "1.0.0",
			Author:      "AethonX",
			Mode:        domain.SourceModePassive,
			Type:        domain.SourceTypeAPI, // Primary type (can fallback to CLI)
			RequiresAuth: true,                 // API key required for API mode

			// Rate limiting
			RateLimit: 60, // 60 queries/min (free tier: ~1 query/sec)

			// Dependencies
			InputArtifacts: []domain.ArtifactType{}, // Stage 0: No input dependencies
			OutputArtifacts: []domain.ArtifactType{
				domain.ArtifactTypeIP,
				domain.ArtifactTypeSubdomain,
				domain.ArtifactTypePort,
				domain.ArtifactTypeService,
				domain.ArtifactTypeVulnerability,
				domain.ArtifactTypeCertificate,
				domain.ArtifactTypeTechnology,
				domain.ArtifactTypeASN,
				domain.ArtifactTypeCloudResource,
			},

			// Priority: After crtsh (10), before subfinder (20)
			Priority:  12,
			StageHint: 0, // Stage 0: Early passive reconnaissance
		},
	); err != nil {
		// Log error but don't panic - allow application to start
		logx.New().Warn("failed to register shodan source", "error", err.Error())
	}
}

// factory creates a new Shodan source instance from configuration.
// It uses registry helpers for type-safe config extraction.
func factory(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
	// Extract configuration using type-safe registry helpers
	apiKey := registry.GetStringConfig(cfg.Custom, "api_key", "")
	useCLI := registry.GetBoolConfig(cfg.Custom, "use_cli", false)
	timeout := registry.GetDurationConfig(cfg.Custom, "timeout", 60*time.Second)
	rateLimit := registry.GetFloat64Config(cfg.Custom, "rate_limit", 1.0)

	logger.Debug("creating shodan source",
		"use_cli", useCLI,
		"timeout", timeout.String(),
		"rate_limit", rateLimit,
		"api_key_provided", apiKey != "",
	)

	// Create source with configuration
	source := NewWithConfig(logger, apiKey, useCLI, timeout, rateLimit)

	return source, nil
}
