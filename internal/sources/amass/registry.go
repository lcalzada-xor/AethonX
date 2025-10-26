package amass

import (
	"time"

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
			// Extract custom config
			execPath := "amass"
			timeout := defaultTimeout
			maxDNSQPS := 0
			brute := false
			alts := false
			activeMode := false // Default to passive

			if cfg.Custom != nil {
				if v, ok := cfg.Custom["exec_path"].(string); ok && v != "" {
					execPath = v
				}
				if v, ok := cfg.Custom["max_dns_qps"].(int); ok {
					maxDNSQPS = v
				}
				if v, ok := cfg.Custom["brute"].(bool); ok {
					brute = v
				}
				if v, ok := cfg.Custom["alts"].(bool); ok {
					alts = v
				}
				// Active mode passed via custom map from main.go
				if v, ok := cfg.Custom["active_mode"].(bool); ok {
					activeMode = v
				}
			}

			// Use configured timeout or default
			if cfg.Timeout > 0 {
				timeout = time.Duration(cfg.Timeout) * time.Second
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
			Priority:  15, // Medium-high priority (after crtsh=20, before subfinder=10)
			StageHint: 0,  // Stage 0 explicit
		},
	); err != nil {
		// Log error but don't panic - allow application to start
		// Registry will skip this source during Build()
		logx.New().Warn("failed to register amass source", "error", err.Error())
	}
}
