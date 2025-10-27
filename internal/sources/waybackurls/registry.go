package waybackurls

import (
	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/registry"
	"aethonx/internal/platform/urlfilter"
)

// Auto-registro de la source al importar el package
func init() {
	if err := registry.Global().Register(
		"waybackurls",
		func(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
			// Extract custom config
			execPath := "waybackurls"
			withDates := false
			noSubs := false

			if cfg.Custom != nil {
				if v, ok := cfg.Custom["exec_path"].(string); ok && v != "" {
					execPath = v
				}
				if v, ok := cfg.Custom["with_dates"].(bool); ok {
					withDates = v
				}
				if v, ok := cfg.Custom["no_subs"].(bool); ok {
					noSubs = v
				}
			}

			// Use configured timeout or default
			timeout := cfg.Timeout
			if timeout == 0 {
				timeout = defaultTimeout
			}

			// Use default filter config (can be customized via Custom map in the future)
			filterCfg := urlfilter.DefaultConfig()

			return NewWithConfig(logger, execPath, timeout, withDates, noSubs, filterCfg), nil
		},
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
		// Registry will skip this source during Build()
		logx.New().Warn("failed to register waybackurls source", "error", err.Error())
	}
}
