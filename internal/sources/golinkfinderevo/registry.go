package golinkfinderevo

import (
	"fmt"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/registry"
)

// Auto-register golinkfinderevo source on package import.
func init() {
	err := registry.Global().Register("golinkfinderevo", factory, ports.SourceMetadata{
		Name:        "golinkfinderevo",
		Description: "GoLinkfinderEVO - discover endpoints, API routes, and secrets in JavaScript/HTML",
		Author:      "lcalzada-xor",
		Version:     "1.0.0",
		Mode:        domain.SourceModeActive,
		Type:        domain.SourceTypeCLI,
		Priority:    20,     // High priority in Stage 3
		StageHint:   3,      // Force Stage 3 (Crawl)
		InputArtifacts: []domain.ArtifactType{
			domain.ArtifactTypeURL, // Consume URLs from httpx
		},
		OutputArtifacts: []domain.ArtifactType{
			domain.ArtifactTypeEndpoint,       // Discovered endpoints
			domain.ArtifactTypeParameter,      // URL parameters
			domain.ArtifactTypeCredential,     // Credentials, API keys, tokens
			domain.ArtifactTypeSensitiveFile,  // Sensitive file paths
			domain.ArtifactTypeAPI,            // API definitions
			domain.ArtifactTypeStorageBucket,  // Cloud storage buckets
			domain.ArtifactTypeJavaScript,     // JavaScript files for Stage 4 (retire.js)
		},
	})

	if err != nil {
		// Log warning but don't panic - allows application to continue
		logx.New().Warn("failed to register golinkfinderevo source", "error", err.Error())
	}
}

// factory creates a new GoLinkfinderEvoSource from SourceConfig using registry helpers.
func factory(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
	// Extract custom configuration using registry helpers
	execPath := registry.GetStringConfig(cfg.Custom, "exec_path", "golinkfinder")
	profileStr := registry.GetStringConfig(cfg.Custom, "profile", string(ProfileStandard))
	workers := registry.GetIntConfig(cfg.Custom, "workers", 20)
	maxScriptFiles := registry.GetIntConfig(cfg.Custom, "max_script_files", defaultMaxScriptFiles)
	maxHTMLFiles := registry.GetIntConfig(cfg.Custom, "max_html_files", defaultMaxHTMLFiles)
	gfTemplatesPath := registry.GetStringConfig(cfg.Custom, "gf_templates_path", "./internal/platform/gf_templates")
	gfPatterns := registry.GetSliceConfig(cfg.Custom, "gf_patterns", []string{"all"})

	// Parse profile
	profile := ScanProfile(profileStr)
	if !ValidateProfile(profile) {
		return nil, fmt.Errorf("invalid golinkfinderevo profile: %s (valid: %v)", profileStr, AllProfileNames())
	}

	// Use configured timeout or default
	timeout := cfg.Timeout
	if timeout == 0 {
		profileCfg := GetProfile(profile)
		timeout = profileCfg.Timeout
	}

	// Validate configuration
	if workers <= 0 || workers > 1000 {
		return nil, fmt.Errorf("workers must be between 1 and 1000, got %d", workers)
	}

	if maxScriptFiles < 0 {
		return nil, fmt.Errorf("max_script_files cannot be negative, got %d", maxScriptFiles)
	}

	if maxHTMLFiles < 0 {
		return nil, fmt.Errorf("max_html_files cannot be negative, got %d", maxHTMLFiles)
	}

	// Validate timeout
	if timeout < 10*time.Second {
		logger.Warn("golinkfinderevo timeout is very short, may cause incomplete results",
			"timeout", timeout.String(),
			"recommended_minimum", "30s",
		)
	}

	// Create source
	source := NewWithConfig(
		logger,
		execPath,
		profile,
		timeout,
		workers,
		maxScriptFiles,
		maxHTMLFiles,
		gfTemplatesPath,
		gfPatterns,
	)

	// Set custom flags if provided
	if customFlags, ok := cfg.Custom["custom_flags"].([]string); ok {
		source.SetCustomFlags(customFlags)
	}

	logger.Debug("golinkfinderevo source created via factory",
		"profile", profile,
		"workers", workers,
		"max_script_files", maxScriptFiles,
		"max_html_files", maxHTMLFiles,
		"gf_templates_path", gfTemplatesPath,
		"gf_patterns", gfPatterns,
		"timeout", timeout.String(),
	)

	return source, nil
}
