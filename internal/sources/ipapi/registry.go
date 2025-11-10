// internal/sources/ipapi/registry.go
package ipapi

import (
	"fmt"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/registry"
)

// init registers the ipapi source in the global registry.
// This runs automatically when the package is imported.
func init() {
	registry.Global().Register("ipapi", factory, sourceMetadata)
}

// factory creates a new IPAPISource from configuration.
func factory(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
	// Extract timeout from config (with fallback to default)
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Validate timeout is reasonable
	if timeout < 5*time.Second {
		return nil, fmt.Errorf("ipapi timeout too low: %v (minimum 5s)", timeout)
	}

	logger.Info("creating ipapi source",
		"timeout", timeout.String(),
		"priority", cfg.Priority,
	)

	return NewWithTimeout(logger, timeout), nil
}

// sourceMetadata describes the ipapi source capabilities.
var sourceMetadata = ports.SourceMetadata{
	Name:        "ipapi",
	Description: "IP geolocation and network enrichment via ip-api.com (45 req/min free)",
	Type:        domain.SourceTypeAPI,
	Mode:        domain.SourceModePassive,
	StageHint:   2, // Stage 2: Enrichment (after discovery, before crawl)
}
