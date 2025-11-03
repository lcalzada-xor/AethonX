// Package dnsx - Auto-registration with AethonX registry
package dnsx

import (
	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/registry"
)

// Auto-registration on package import using registry helpers
func init() {
	if err := registry.Global().Register(
		sourceName,
		factory,
		ports.SourceMetadata{
			Name:         sourceName,
			Description:  "DNS resolution and enumeration with ProjectDiscovery dnsx",
			Version:      "1.0.0",
			Author:       "AethonX",
			Mode:         domain.SourceModeBoth, // Hybrid: Works in both passive and active modes
			Type:         domain.SourceTypeCLI,
			RequiresAuth: false,
			RateLimit:    0, // Managed internally by dnsx

			// Dependency declaration (Stage 2: consumes subdomain from Stage 1)
			InputArtifacts: []domain.ArtifactType{
				domain.ArtifactTypeSubdomain,
				domain.ArtifactTypeDomain,
			},
			OutputArtifacts: []domain.ArtifactType{
				// NOTE: Subdomain removed from outputs to avoid circular dependency
				// Validated subdomains are still produced but not declared as output
				domain.ArtifactTypeIP,
				domain.ArtifactTypeNameserver,
				domain.ArtifactTypeService, // MX records
				domain.ArtifactTypeTechnology, // TXT records
			},
			Priority:  15, // Same as httpx - both run in Stage 2 in parallel
			StageHint: 0,  // Auto-detect stage based on dependencies
		},
	); err != nil {
		// Log error but don't panic - allow application to start
		logx.New().Warn("failed to register dnsx source", "error", err.Error())
	}
}

// factory creates a new DNSXSource from SourceConfig using registry helpers
func factory(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
	config := DefaultDNSXConfig()

	// Extract custom config using registry helpers (type-safe, no manual nil checks)
	config.Threads = registry.GetIntConfig(cfg.Custom, "threads", config.Threads)
	config.RateLimit = registry.GetIntConfig(cfg.Custom, "rate_limit", config.RateLimit)
	config.QueryTypes = registry.GetSliceConfig(cfg.Custom, "query_types", config.QueryTypes)
	config.EnableCDN = registry.GetBoolConfig(cfg.Custom, "enable_cdn", config.EnableCDN)
	config.EnableASN = registry.GetBoolConfig(cfg.Custom, "enable_asn", config.EnableASN)
	config.WildcardFilter = registry.GetBoolConfig(cfg.Custom, "wildcard_filter", config.WildcardFilter)
	config.ExecPath = registry.GetStringConfig(cfg.Custom, "exec_path", config.ExecPath)

	// Use configured timeout or default
	if cfg.Timeout > 0 {
		config.Timeout = cfg.Timeout
	}

	// Validate configuration
	if err := registry.ValidatePositiveInt("threads", config.Threads); err != nil {
		return nil, err
	}

	if config.RateLimit < -1 {
		logger.Warn("invalid rate_limit, using unlimited", "rate_limit", config.RateLimit)
		config.RateLimit = -1
	}

	if err := registry.ValidateNonEmptySlice("query_types", config.QueryTypes); err != nil {
		return nil, err
	}

	logger.Info("creating dnsx source",
		"threads", config.Threads,
		"rate_limit", config.RateLimit,
		"query_types", config.QueryTypes,
		"enable_cdn", config.EnableCDN,
		"enable_asn", config.EnableASN,
		"wildcard_filter", config.WildcardFilter,
		"timeout", config.Timeout.String(),
	)

	return NewWithConfig(logger, config), nil
}
