package urlfilter

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"aethonx/internal/platform/logx"
)

// FilterEngine is the main URL filtering engine that orchestrates all components.
type FilterEngine struct {
	config     FilterConfig
	normalizer *URLNormalizer
	similarity *SimilarityDetector
	scorer     *PriorityScorer
	patterns   *PatternExtractor
	bloom      *BloomFilter
	logger     logx.Logger
}

// NewFilterEngine creates a new filter engine with given configuration.
func NewFilterEngine(config FilterConfig, logger logx.Logger) *FilterEngine {
	// Validate configuration
	if err := config.Validate(); err != nil {
		logger.Warn("invalid filter config, using defaults", "error", err.Error())
		config = DefaultConfig()
	}

	// Initialize components
	engine := &FilterEngine{
		config:     config,
		normalizer: NewURLNormalizer(config.NormStrategy, logger),
		similarity: NewSimilarityDetector(config.SimilarityThreshold, config.SimilarityAlgorithm, logger),
		scorer:     NewPriorityScorer(DefaultScoreWeights(), logger),
		patterns:   NewPatternExtractor(config.MinPatternFrequency, logger),
		logger:     logger.With("component", "filter_engine"),
	}

	// Initialize Bloom filter if enabled
	if config.UseBloomFilter {
		// Estimate expected elements (use MaxURLs or a reasonable default)
		expectedElements := config.MaxURLs
		if expectedElements <= 0 {
			expectedElements = 100000
		}
		engine.bloom = NewBloomFilter(expectedElements, config.BloomFPRate, logger)
	}

	logger.Info("initialized filter engine",
		"norm_strategy", config.NormStrategy.String(),
		"similarity_threshold", config.SimilarityThreshold,
		"similarity_algorithm", config.SimilarityAlgorithm.String(),
		"max_urls", config.MaxURLs,
		"use_bloom", config.UseBloomFilter,
	)

	return engine
}

// Filter processes URLs and returns deduplicated, prioritized results.
func (f *FilterEngine) Filter(ctx context.Context, urls []string) ([]ScoredURL, FilterStats, error) {
	startTime := time.Now()
	stats := FilterStats{
		InputURLs: len(urls),
	}

	f.logger.Info("starting URL filtering", "input_urls", len(urls))

	// Track memory before processing
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// Step 1: Apply volume control
	if f.config.EnableVolumeControl && f.config.MaxURLs > 0 && len(urls) > f.config.MaxURLs {
		f.logger.Warn("input exceeds max URLs, truncating",
			"input", len(urls),
			"max", f.config.MaxURLs,
		)
		urls = urls[:f.config.MaxURLs]
		stats.InputURLs = len(urls)
	}

	// Step 2: Normalize URLs and deduplicate using Bloom filter
	normalized := f.normalizeAndDeduplicate(ctx, urls, &stats)
	if len(normalized) == 0 {
		return nil, stats, fmt.Errorf("no valid URLs after normalization")
	}

	f.logger.Debug("normalized and deduplicated",
		"input", len(urls),
		"output", len(normalized),
		"duplicates", stats.DuplicatesSkipped,
	)

	// Step 3: Score URLs by priority
	scored := f.scoreURLs(ctx, normalized, &stats)
	f.logger.Debug("scored URLs", "count", len(scored))

	// Step 4: Apply clustering (if enabled)
	if f.config.EnableClustering {
		scored = f.clusterURLs(ctx, scored, &stats)
		f.logger.Debug("clustered URLs",
			"input", len(normalized),
			"clusters", stats.ClustersMerged,
			"output", len(scored),
		)
	}

	// Step 5: Apply pattern-based filtering (if enabled)
	if f.config.EnablePatternFilter && f.config.MaxPerPattern > 0 {
		scored = f.filterByPattern(ctx, scored, &stats)
		f.logger.Debug("filtered by pattern",
			"patterns", stats.PatternsFound,
			"output", len(scored),
		)
	}

	// Step 6: Filter by minimum priority score
	scored = f.filterByScore(scored, &stats)
	f.logger.Debug("filtered by score",
		"min_score", f.config.MinPriorityScore,
		"output", len(scored),
	)

	// Final statistics
	stats.OutputURLs = len(scored)
	stats.Duration = time.Since(startTime)
	stats.DurationMs = stats.Duration.Milliseconds()

	// Track memory after processing
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)
	stats.MemoryBytes = int64(memAfter.Alloc - memBefore.Alloc)

	// Add Bloom filter stats if enabled
	if f.bloom != nil {
		bloomStats := f.bloom.Stats()
		stats.BloomFilterSize = bloomStats.Size
		stats.BloomFilterChecks = bloomStats.Checks
		stats.BloomFilterHits = bloomStats.Hits
		stats.BloomFilterFPRate = bloomStats.EstimatedFPRate
	}

	f.logger.Info("filtering complete", "stats", stats.String())

	return scored, stats, nil
}

// normalizeAndDeduplicate normalizes URLs and removes duplicates.
func (f *FilterEngine) normalizeAndDeduplicate(ctx context.Context, urls []string, stats *FilterStats) []string {
	seen := make(map[string]bool, len(urls))
	normalized := make([]string, 0, len(urls))

	for _, rawURL := range urls {
		// Check context cancellation
		select {
		case <-ctx.Done():
			f.logger.Warn("normalization cancelled", "processed", len(normalized))
			return normalized
		default:
		}

		// Normalize URL
		result, err := f.normalizer.Normalize(rawURL)
		if err != nil {
			stats.InvalidURLsSkipped++
			continue
		}

		// Deduplicate using Bloom filter (if enabled)
		if f.bloom != nil {
			if f.bloom.AddAndCheck(result.Signature) {
				// Already seen (might be false positive, but acceptable)
				stats.DuplicatesSkipped++
				continue
			}
		} else {
			// Fallback to map-based deduplication
			if seen[result.Signature] {
				stats.DuplicatesSkipped++
				continue
			}
			seen[result.Signature] = true
		}

		normalized = append(normalized, result.Original)
	}

	return normalized
}

// scoreURLs assigns priority scores to URLs.
func (f *FilterEngine) scoreURLs(ctx context.Context, urls []string, stats *FilterStats) []ScoredURL {
	scored := make([]ScoredURL, 0, len(urls))

	for _, rawURL := range urls {
		// Check context cancellation
		select {
		case <-ctx.Done():
			f.logger.Warn("scoring cancelled", "processed", len(scored))
			return scored
		default:
		}

		scoredURL := f.scorer.Score(rawURL)
		scored = append(scored, scoredURL)
	}

	return scored
}

// clusterURLs groups similar URLs and selects representatives.
func (f *FilterEngine) clusterURLs(ctx context.Context, scored []ScoredURL, stats *FilterStats) []ScoredURL {
	// Extract URLs from scored results
	urls := make([]string, len(scored))
	scoreMap := make(map[string]ScoredURL, len(scored))
	for i, s := range scored {
		urls[i] = s.URL
		scoreMap[s.URL] = s
	}

	// Cluster URLs
	clusters := f.similarity.Cluster(urls)
	stats.ClustersMerged = len(clusters)

	// Select top-K from each cluster
	selectedURLs := f.similarity.SelectTopK(clusters, f.config.TopKPerCluster)

	// Rebuild scored results with selected URLs
	clustered := make([]ScoredURL, 0, len(selectedURLs))
	for _, url := range selectedURLs {
		if s, exists := scoreMap[url]; exists {
			clustered = append(clustered, s)
		}
	}

	return clustered
}

// filterByPattern filters URLs using pattern detection.
func (f *FilterEngine) filterByPattern(ctx context.Context, scored []ScoredURL, stats *FilterStats) []ScoredURL {
	// Extract URLs
	urls := make([]string, len(scored))
	scoreMap := make(map[string]ScoredURL, len(scored))
	for i, s := range scored {
		urls[i] = s.URL
		scoreMap[s.URL] = s
	}

	// Extract patterns
	patterns := f.patterns.ExtractPatterns(urls)
	stats.PatternsFound = len(patterns)

	// Select representatives (max per pattern)
	selectedURLs := f.patterns.SelectRepresentatives(patterns, f.config.MaxPerPattern)

	// Rebuild scored results
	filtered := make([]ScoredURL, 0, len(selectedURLs))
	for _, url := range selectedURLs {
		if s, exists := scoreMap[url]; exists {
			filtered = append(filtered, s)
		}
	}

	return filtered
}

// filterByScore filters out URLs below minimum priority score.
func (f *FilterEngine) filterByScore(scored []ScoredURL, stats *FilterStats) []ScoredURL {
	filtered := make([]ScoredURL, 0, len(scored))

	for _, s := range scored {
		if s.Score >= f.config.MinPriorityScore {
			filtered = append(filtered, s)
		} else {
			stats.LowPrioritySkipped++
		}
	}

	return filtered
}

// FilterStream processes URLs incrementally for streaming sources.
// This method is suitable for processing waybackurls output line-by-line.
func (f *FilterEngine) FilterStream(ctx context.Context, urlChan <-chan string) (<-chan ScoredURL, <-chan error) {
	outChan := make(chan ScoredURL, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(outChan)
		defer close(errChan)

		batch := make([]string, 0, 1000)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		processBatch := func() {
			if len(batch) == 0 {
				return
			}

			scored, _, err := f.Filter(ctx, batch)
			if err != nil {
				f.logger.Warn("batch filter error", "error", err.Error())
				return
			}

			for _, s := range scored {
				select {
				case outChan <- s:
				case <-ctx.Done():
					return
				}
			}

			batch = batch[:0] // Clear batch
		}

		for {
			select {
			case url, ok := <-urlChan:
				if !ok {
					// Input channel closed, process final batch
					processBatch()
					return
				}

				batch = append(batch, url)

				// Process when batch is full
				if len(batch) >= 1000 {
					processBatch()
				}

			case <-ticker.C:
				// Process batch periodically
				processBatch()

			case <-ctx.Done():
				f.logger.Warn("stream filtering cancelled")
				errChan <- ctx.Err()
				return
			}
		}
	}()

	return outChan, errChan
}

// Config returns the current filter configuration.
func (f *FilterEngine) Config() FilterConfig {
	return f.config
}

// Stats returns current engine statistics.
func (f *FilterEngine) Stats() FilterStats {
	stats := FilterStats{}

	if f.bloom != nil {
		bloomStats := f.bloom.Stats()
		stats.BloomFilterSize = bloomStats.Size
		stats.BloomFilterChecks = bloomStats.Checks
		stats.BloomFilterHits = bloomStats.Hits
		stats.BloomFilterFPRate = bloomStats.EstimatedFPRate
		stats.MemoryBytes = bloomStats.MemoryBytes
	}

	return stats
}

// Clear resets internal state (useful for testing).
func (f *FilterEngine) Clear() {
	if f.bloom != nil {
		f.bloom.Clear()
	}
	f.logger.Debug("filter engine cleared")
}
