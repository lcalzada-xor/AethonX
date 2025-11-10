package retirejs

import (
	"fmt"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/registry"
)

// Auto-register retirejs source on package import.
func init() {
	err := registry.Global().Register("retirejs", factory, ports.SourceMetadata{
		Name:        "retirejs",
		Description: "retire.js - detect JavaScript libraries with known vulnerabilities (CVE Assessment)",
		Author:      "Erlend Oftedal",
		Version:     "5.3.0",
		Mode:        domain.SourceModeActive,
		Type:        domain.SourceTypeCLI,
		Priority:    25,     // Stage 4 priority
		StageHint:   4,      // Force Stage 4 (CVE Assessment)
		InputArtifacts: []domain.ArtifactType{
			domain.ArtifactTypeJavaScript, // Consume JS artifacts from golinkfinderevo
			domain.ArtifactTypeURL,        // Fallback: consume URLs
		},
		OutputArtifacts: []domain.ArtifactType{
			domain.ArtifactTypeVulnerability, // CVEs detectados
			domain.ArtifactTypeTechnology,    // Librerías identificadas
			domain.ArtifactTypeJavaScript,    // Archivos analizados
		},
	})

	if err != nil {
		logx.New().Warn("failed to register retirejs source", "error", err.Error())
	}
}

// factory creates a new RetireJSSource from SourceConfig using registry helpers.
func factory(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
	// Extract configuration using registry helpers
	execPath := registry.GetStringConfig(cfg.Custom, "exec_path", "retire")
	severity := registry.GetStringConfig(cfg.Custom, "severity", "medium")
	exitCode := registry.GetIntConfig(cfg.Custom, "exit_code", 0)
	deep := registry.GetBoolConfig(cfg.Custom, "deep", false)
	includeOSV := registry.GetBoolConfig(cfg.Custom, "include_osv", false)
	maxFiles := registry.GetIntConfig(cfg.Custom, "max_files", defaultMaxFiles)
	extensions := registry.GetSliceConfig(cfg.Custom, "extensions", []string{"js"})
	cachePath := registry.GetStringConfig(cfg.Custom, "cache_dir", "")
	noCache := registry.GetBoolConfig(cfg.Custom, "no_cache", false)
	downloadTimeout := registry.GetDurationConfig(cfg.Custom, "download_timeout", defaultDownloadTimeout)
	preferLocal := registry.GetBoolConfig(cfg.Custom, "prefer_local", true)
	fallbackDownload := registry.GetBoolConfig(cfg.Custom, "fallback_download", true)

	// Use configured timeout or default
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	// Validation
	if err := registry.ValidateIntRange("exit_code", exitCode, 0, 255); err != nil {
		return nil, err
	}

	if err := registry.ValidatePositiveInt("max_files", maxFiles); err != nil {
		return nil, err
	}

	// Validate severity
	validSeverities := map[string]bool{
		"none": true, "low": true, "medium": true, "high": true, "critical": true,
	}
	if !validSeverities[severity] {
		return nil, fmt.Errorf("invalid severity: %s (valid: none, low, medium, high, critical)", severity)
	}

	// Validate timeout
	if timeout < 10*time.Second {
		logger.Warn("retirejs timeout is very short, may cause incomplete results",
			"timeout", timeout.String(),
			"recommended_minimum", "30s",
		)
	}

	// Create source
	source := NewWithConfig(
		logger,
		execPath,
		timeout,
		severity,
		exitCode,
		deep,
		includeOSV,
		maxFiles,
		extensions,
		cachePath,
		noCache,
		downloadTimeout,
		preferLocal,
		fallbackDownload,
	)

	logger.Debug("retirejs source created via factory",
		"severity", severity,
		"exit_code", exitCode,
		"deep", deep,
		"include_osv", includeOSV,
		"max_files", maxFiles,
		"extensions", extensions,
		"cache_dir", cachePath,
		"no_cache", noCache,
		"download_timeout", downloadTimeout.String(),
		"prefer_local", preferLocal,
		"fallback_download", fallbackDownload,
		"timeout", timeout.String(),
	)

	return source, nil
}
