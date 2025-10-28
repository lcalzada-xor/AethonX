package httpx

import (
	"fmt"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/registry"
)

// Auto-register httpx source on package import.
func init() {
	err := registry.Global().Register("httpx", factory, ports.SourceMetadata{
		Name:        "httpx",
		Description: "Project Discovery httpx - fast HTTP probing and fingerprinting tool",
		Author:      "Project Discovery",
		Version:     "1.6.9",
		Mode:        domain.SourceModeActive,
		Type:        domain.SourceTypeCLI,
		Priority:    15, // High priority (runs after passive sources)
		InputArtifacts: []domain.ArtifactType{
			domain.ArtifactTypeSubdomain, // Consume subdomains from crtsh
			domain.ArtifactTypeDomain,    // Consume domains from rdap
			domain.ArtifactTypeURL,       // Consume URLs if exist
		},
		OutputArtifacts: []domain.ArtifactType{
			domain.ArtifactTypeURL,         // Probed URLs
			domain.ArtifactTypeIP,          // Resolved IPs
			domain.ArtifactTypeTechnology,  // Detected technologies
			domain.ArtifactTypeCertificate, // SSL certificates
			domain.ArtifactTypeSubdomain,   // Subdomains from SANs
		},
	})

	if err != nil {
		// Log warning but don't panic - allows application to continue
		logx.New().Warn("failed to register httpx source", "error", err.Error())
	}
}

// factory creates a new HTTPXSource from SourceConfig using registry helpers.
func factory(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
	// Extract custom configuration using registry helpers
	execPath := registry.GetStringConfig(cfg.Custom, "exec_path", "httpx")
	profileStr := registry.GetStringConfig(cfg.Custom, "profile", string(ProfileFull))
	threads := registry.GetIntConfig(cfg.Custom, "threads", defaultThreads)
	rateLimit := registry.GetIntConfig(cfg.Custom, "rate_limit", defaultRateLimit)

	// Parse profile
	profile := ScanProfile(profileStr)
	if _, exists := Profiles[profile]; !exists {
		return nil, fmt.Errorf("invalid httpx profile: %s (valid: basic, tech, tls, full, headless)", profileStr)
	}

	// Use configured timeout or default
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	// Validate configuration
	if threads <= 0 || threads > 1000 {
		return nil, fmt.Errorf("httpx threads must be between 1 and 1000, got %d", threads)
	}

	if rateLimit < 0 {
		return nil, fmt.Errorf("httpx rate_limit cannot be negative, got %d", rateLimit)
	}

	// Create source
	source := NewWithConfig(logger, execPath, profile, timeout, threads, rateLimit)

	// Set custom flags if provided
	if customFlags, ok := cfg.Custom["custom_flags"].([]string); ok {
		source.SetCustomFlags(customFlags)
	}

	logger.Debug("httpx source created via factory",
		"profile", profile,
		"threads", threads,
		"rate_limit", rateLimit,
		"timeout", timeout.String(),
	)

	return source, nil
}
