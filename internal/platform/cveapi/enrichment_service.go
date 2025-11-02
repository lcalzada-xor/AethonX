// internal/platform/cveapi/enrichment_service.go
package cveapi

import (
	"context"
	"fmt"
	"sync"
	"time"

	"aethonx/internal/platform/cache"
	"aethonx/internal/platform/logx"
)

// EnrichmentService orchestrates CVE enrichment with multiple providers and caching.
type EnrichmentService struct {
	logger logx.Logger
	cache  cache.Cache

	// Provider chain (primary + fallbacks)
	providers []CVEProvider

	// Configuration
	cacheTTL      time.Duration
	timeout       time.Duration
	maxConcurrent int

	// Stats
	stats      *EnrichmentStats
	statsMutex sync.RWMutex
}

// EnrichmentStats tracks enrichment metrics.
type EnrichmentStats struct {
	TotalRequests    int64
	CacheHits        int64
	CacheMisses      int64
	SuccessfulEnrich int64
	FailedEnrich     int64
	ProviderUsage    map[string]int64 // provider name -> count
}

// EnrichmentConfig contains configuration for the enrichment service.
type EnrichmentConfig struct {
	CacheTTL      time.Duration
	Timeout       time.Duration
	MaxConcurrent int
}

// NewEnrichmentService creates a new CVE enrichment service.
func NewEnrichmentService(config EnrichmentConfig, logger logx.Logger) *EnrichmentService {
	return &EnrichmentService{
		logger:        logger.With("component", "cve-enrichment"),
		cache:         cache.NewMemoryCache(10000), // Cache up to 10k CVEs
		providers:     []CVEProvider{},
		cacheTTL:      config.CacheTTL,
		timeout:       config.Timeout,
		maxConcurrent: config.MaxConcurrent,
		stats: &EnrichmentStats{
			ProviderUsage: make(map[string]int64),
		},
	}
}

// RegisterProvider adds a CVE provider to the service.
// Providers are tried in registration order (first = primary).
func (e *EnrichmentService) RegisterProvider(provider CVEProvider) error {
	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}

	// Health check before registration
	if err := provider.HealthCheck(); err != nil {
		e.logger.Warn("provider health check failed",
			"provider", provider.Name(),
			"error", err.Error(),
		)
		// Don't fail registration, just warn
	}

	e.providers = append(e.providers, provider)
	e.logger.Info("registered CVE provider",
		"provider", provider.Name(),
		"rate_limit", provider.RateLimit(),
	)

	return nil
}

// Enrich retrieves enriched CVE data, using cache if available.
func (e *EnrichmentService) Enrich(ctx context.Context, cveID string) (*EnrichedCVE, error) {
	e.incrementStat("total")

	// Validate CVE ID format
	if !isValidCVEID(cveID) {
		return nil, fmt.Errorf("invalid CVE ID format: %s", cveID)
	}

	// Check cache first
	cacheKey := fmt.Sprintf("cve:%s", cveID)
	if cached, ok := e.cache.Get(cacheKey); ok {
		e.incrementStat("cache_hit")
		e.logger.Debug("cache hit", "cve", cveID)
		return cached.(*EnrichedCVE), nil
	}

	e.incrementStat("cache_miss")

	// Try providers in order
	var lastErr error
	for _, provider := range e.providers {
		enriched, err := e.enrichWithProvider(ctx, provider, cveID)
		if err == nil {
			// Success - cache and return
			e.cache.Set(cacheKey, enriched, e.cacheTTL)
			e.incrementStat("success")
			e.incrementProviderStat(provider.Name())
			e.logger.Debug("enriched successfully",
				"cve", cveID,
				"provider", provider.Name(),
			)
			return enriched, nil
		}

		e.logger.Warn("provider enrichment failed",
			"provider", provider.Name(),
			"cve", cveID,
			"error", err.Error(),
		)
		lastErr = err
	}

	// All providers failed
	e.incrementStat("failed")
	return nil, fmt.Errorf("all providers failed for %s: %w", cveID, lastErr)
}

// enrichWithProvider attempts enrichment with a specific provider.
func (e *EnrichmentService) enrichWithProvider(
	ctx context.Context,
	provider CVEProvider,
	cveID string,
) (*EnrichedCVE, error) {
	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// Execute enrichment with timeout
	resultChan := make(chan *EnrichedCVE, 1)
	errorChan := make(chan error, 1)

	go func() {
		enriched, err := provider.Enrich(cveID)
		if err != nil {
			errorChan <- err
			return
		}
		resultChan <- enriched
	}()

	select {
	case enriched := <-resultChan:
		// Set enrichment metadata
		enriched.EnrichmentSource = provider.Name()
		enriched.EnrichedAt = time.Now()
		return enriched, nil

	case err := <-errorChan:
		return nil, err

	case <-ctx.Done():
		return nil, fmt.Errorf("enrichment timeout for %s", provider.Name())
	}
}

// EnrichBatch enriches multiple CVEs concurrently.
func (e *EnrichmentService) EnrichBatch(
	ctx context.Context,
	cveIDs []string,
) (map[string]*EnrichedCVE, []error) {
	results := make(map[string]*EnrichedCVE)
	errors := make([]error, 0)
	resultsMutex := sync.Mutex{}

	// Use semaphore to limit concurrency
	sem := make(chan struct{}, e.maxConcurrent)
	var wg sync.WaitGroup

	for _, cveID := range cveIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			enriched, err := e.Enrich(ctx, id)

			resultsMutex.Lock()
			defer resultsMutex.Unlock()

			if err != nil {
				errors = append(errors, fmt.Errorf("%s: %w", id, err))
			} else {
				results[id] = enriched
			}
		}(cveID)
	}

	wg.Wait()
	return results, errors
}

// GetStats returns current enrichment statistics.
func (e *EnrichmentService) GetStats() EnrichmentStats {
	e.statsMutex.RLock()
	defer e.statsMutex.RUnlock()

	// Deep copy to prevent race conditions
	stats := *e.stats
	stats.ProviderUsage = make(map[string]int64, len(e.stats.ProviderUsage))
	for k, v := range e.stats.ProviderUsage {
		stats.ProviderUsage[k] = v
	}

	return stats
}

// incrementStat safely increments a stat counter.
func (e *EnrichmentService) incrementStat(stat string) {
	e.statsMutex.Lock()
	defer e.statsMutex.Unlock()

	switch stat {
	case "total":
		e.stats.TotalRequests++
	case "cache_hit":
		e.stats.CacheHits++
	case "cache_miss":
		e.stats.CacheMisses++
	case "success":
		e.stats.SuccessfulEnrich++
	case "failed":
		e.stats.FailedEnrich++
	}
}

// incrementProviderStat safely increments provider usage counter.
func (e *EnrichmentService) incrementProviderStat(providerName string) {
	e.statsMutex.Lock()
	defer e.statsMutex.Unlock()

	e.stats.ProviderUsage[providerName]++
}

// isValidCVEID validates CVE ID format (CVE-YYYY-NNNNN).
func isValidCVEID(cveID string) bool {
	if len(cveID) < 13 {
		return false
	}
	if cveID[0:4] != "CVE-" {
		return false
	}
	// Could add more validation (year range, number format)
	return true
}
