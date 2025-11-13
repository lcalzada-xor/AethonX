// internal/sources/abuseipdb/registry.go
package abuseipdb

import (
	"fmt"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/registry"
)

// init registers the abuseipdb source in the global registry.
// This runs automatically when the package is imported.
func init() {
	registry.Global().Register("abuseipdb", factory, sourceMetadata)
}

// factory creates a new AbuseIPDBSource from configuration.
func factory(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
	// Extract API key from config (REQUIRED)
	apiKey := registry.GetStringConfig(cfg.Custom, "api_key", "")

	// Validate API key is present
	if err := registry.ValidateRequiredString(apiKey, "api_key"); err != nil {
		return nil, fmt.Errorf("abuseipdb: %w (get your free API key at https://www.abuseipdb.com/api)", err)
	}

	// Extract timeout from config (with fallback to default)
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	// Validate timeout is reasonable
	if timeout < 5*time.Second {
		return nil, fmt.Errorf("abuseipdb timeout too low: %v (minimum 5s)", timeout)
	}

	logger.Info("creating abuseipdb source",
		"timeout", timeout.String(),
		"priority", cfg.Priority,
		"api_key_present", apiKey != "",
	)

	return NewWithTimeout(logger, apiKey, timeout), nil
}

// sourceMetadata describes the abuseipdb source capabilities.
var sourceMetadata = ports.SourceMetadata{
	Name:        "abuseipdb",
	Description: "IP reputation and threat intelligence via abuseipdb.com (1000 req/day free, API key required)",
	Type:        domain.SourceTypeAPI,
	Mode:        domain.SourceModePassive,
	StageHint:   2, // Stage 2: Enrichment (after discovery, before crawl)
}
