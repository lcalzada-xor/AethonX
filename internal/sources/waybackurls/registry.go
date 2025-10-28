package waybackurls

import (
	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/registry"
	"aethonx/internal/platform/urlfilter"
)

// Auto-registration on package import using registry helpers
func init() {
	if err := registry.Global().Register(
		"waybackurls",
		factory,
		ports.SourceMetadata{
			Name:         "waybackurls",
			Description:  "Historical URL discovery via Wayback Machine (Internet Archive)",
			Version:      "1.0.0",
			Author:       "AethonX",
			Mode:         domain.SourceModePassive,
			Type:         domain.SourceTypeCLI,
			RequiresAuth: false,
			RateLimit:    0, // Wayback Machine has no strict rate limits

			// Dependency declaration (Stage 0: no inputs)
			InputArtifacts: []domain.ArtifactType{}, // No inputs = Stage 0
			OutputArtifacts: []domain.ArtifactType{
				domain.ArtifactTypeURL,
				domain.ArtifactTypeSubdomain,
				domain.ArtifactTypeEndpoint,
				domain.ArtifactTypeParameter,
				domain.ArtifactTypeJavaScript,
				domain.ArtifactTypeSensitiveFile,
				domain.ArtifactTypeBackupFile,
				domain.ArtifactTypeRepository,
				domain.ArtifactTypeAPI,
				domain.ArtifactTypeTechnology,
			},
			Priority:  5, // High priority (passive discovery, early execution)
			StageHint: 0, // Stage 0 explicit
		},
	); err != nil {
		// Log error but don't panic - allow application to start
		logx.New().Warn("failed to register waybackurls source", "error", err.Error())
	}
}

// factory creates a new WaybackurlsSource from SourceConfig using registry helpers
func factory(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
	// Extract custom config using registry helpers (type-safe, no manual nil checks)
	execPath := registry.GetStringConfig(cfg.Custom, "exec_path", "waybackurls")
	withDates := registry.GetBoolConfig(cfg.Custom, "with_dates", false)
	noSubs := registry.GetBoolConfig(cfg.Custom, "no_subs", false)

	// Use configured timeout or default
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	// Use default filter config (can be customized via Custom map in the future)
	filterCfg := urlfilter.DefaultConfig()

	return NewWithConfig(logger, execPath, timeout, withDates, noSubs, filterCfg), nil
}
