package amass

import (
	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/registry"
)

// Auto-registro de la source al importar el package
func init() {
	if err := registry.Global().Register(
		"amass",
		func(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
			// Extract custom config using registry helpers
			execPath := registry.GetStringConfig(cfg.Custom, "exec_path", "amass")
			maxDNSQPS := registry.GetIntConfig(cfg.Custom, "max_dns_qps", 0)
			brute := registry.GetBoolConfig(cfg.Custom, "brute", false)
			alts := registry.GetBoolConfig(cfg.Custom, "alts", false)
			activeMode := registry.GetBoolConfig(cfg.Custom, "active_mode", false)

			// Use configured timeout or default
			timeout := cfg.Timeout
			if timeout == 0 {
				timeout = defaultTimeout
			}

			amassConfig := AmassConfig{
				ExecPath:   execPath,
				Timeout:    timeout,
				ActiveMode: activeMode,
				MaxDNSQPS:  maxDNSQPS,
				Brute:      brute,
				Alts:       alts,
			}

			return NewWithConfig(logger, amassConfig), nil
		},
		ports.SourceMetadata{
			Name:         "amass",
			Description:  "OWASP Amass - In-depth subdomain enumeration and network mapping",
			Version:      "1.0.0",
			Author:       "AethonX",
			Mode:         domain.SourceModeBoth, // Hybrid: passive by default, active with --active flag
			Type:         domain.SourceTypeCLI,
			RequiresAuth: false,
			RateLimit:    0, // Managed via -dns-qps flag

			// Dependency declaration (Stage 0: no inputs required)
			InputArtifacts: []domain.ArtifactType{}, // No inputs = Stage 0
			OutputArtifacts: []domain.ArtifactType{
				domain.ArtifactTypeSubdomain,
				domain.ArtifactTypeIP,
				domain.ArtifactTypeCIDR,
				domain.ArtifactTypeASN,
			},
			Priority:  15, // Medium-high priority (runs after passive sources like waybackurls=5, rdap=8, crtsh=10, subfinder=10)
			StageHint: 0,  // Stage 0 explicit
		},
	); err != nil {
		// Log error but don't panic - allow application to start
		// Registry will skip this source during Build()
		logx.New().Warn("failed to register amass source", "error", err.Error())
	}
}
