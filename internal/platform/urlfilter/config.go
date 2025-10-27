// Package urlfilter provides intelligent URL filtering and deduplication.
// It reduces false positives and noise from high-volume URL sources like waybackurls.
package urlfilter

import (
	"fmt"
	"time"
)

// NormalizationStrategy defines how URLs should be normalized.
type NormalizationStrategy int

const (
	// NormBasic performs basic normalization: lowercase, trim, sort params.
	NormBasic NormalizationStrategy = iota

	// NormStructural replaces dynamic segments (IDs, UUIDs, hashes) with placeholders.
	// Example: /api/users/12345 → /api/users/{id}
	NormStructural

	// NormParametric removes parameter values, keeps only keys.
	// Example: ?id=123&sort=asc → ?id=&sort=
	NormParametric

	// NormExtensionless ignores file extensions for assets.
	// Example: photo.jpg, photo.png → photo.{ext}
	NormExtensionless

	// NormAggressive combines all normalization strategies.
	NormAggressive
)

// String returns string representation of normalization strategy.
func (n NormalizationStrategy) String() string {
	switch n {
	case NormBasic:
		return "basic"
	case NormStructural:
		return "structural"
	case NormParametric:
		return "parametric"
	case NormExtensionless:
		return "extensionless"
	case NormAggressive:
		return "aggressive"
	default:
		return "unknown"
	}
}

// SimilarityAlgorithm defines clustering algorithm.
type SimilarityAlgorithm int

const (
	// SimLevenshtein uses edit distance for path similarity.
	SimLevenshtein SimilarityAlgorithm = iota

	// SimJaccard uses token-based similarity (path segments).
	SimJaccard

	// SimTemplate extracts templates from URL patterns.
	SimTemplate
)

// String returns string representation of similarity algorithm.
func (s SimilarityAlgorithm) String() string {
	switch s {
	case SimLevenshtein:
		return "levenshtein"
	case SimJaccard:
		return "jaccard"
	case SimTemplate:
		return "template"
	default:
		return "unknown"
	}
}

// FilterConfig configures the URL filter engine.
type FilterConfig struct {
	// Volume control
	MaxURLs             int  // Hard limit on URLs to process (0 = no limit)
	EnableVolumeControl bool // Apply volume limits

	// Normalization
	NormStrategy NormalizationStrategy // Normalization strategy to use

	// Similarity clustering
	EnableClustering    bool    // Enable similarity-based clustering
	SimilarityThreshold float64 // Similarity threshold (0.0-1.0)
	SimilarityAlgorithm SimilarityAlgorithm

	// Priority filtering
	MinPriorityScore int // Discard URLs below this score
	TopKPerCluster   int // Keep only top K URLs per cluster

	// Pattern-based filtering
	EnablePatternFilter bool // Enable pattern detection
	MaxPerPattern       int  // Max URLs per pattern (0 = no limit)
	MinPatternFrequency int  // Min occurrences to consider valid pattern

	// Memory optimization
	UseBloomFilter bool    // Use Bloom filter for deduplication
	BloomFPRate    float64 // Bloom filter false positive rate

	// Performance
	MaxConcurrency int           // Max concurrent workers
	Timeout        time.Duration // Processing timeout
}

// DefaultConfig returns a balanced default configuration.
func DefaultConfig() FilterConfig {
	return FilterConfig{
		// Volume control
		MaxURLs:             100000,
		EnableVolumeControl: true,

		// Normalization
		NormStrategy: NormAggressive,

		// Similarity clustering
		EnableClustering:    true,
		SimilarityThreshold: 0.85,
		SimilarityAlgorithm: SimJaccard,

		// Priority filtering
		MinPriorityScore: -100,
		TopKPerCluster:   3,

		// Pattern-based filtering
		EnablePatternFilter: true,
		MaxPerPattern:       50,
		MinPatternFrequency: 3,

		// Memory optimization
		UseBloomFilter: true,
		BloomFPRate:    0.01, // 1% false positive rate

		// Performance
		MaxConcurrency: 4,
		Timeout:        5 * time.Minute,
	}
}

// FastConfig returns configuration optimized for speed.
func FastConfig() FilterConfig {
	cfg := DefaultConfig()
	cfg.MaxURLs = 10000
	cfg.SimilarityThreshold = 0.90
	cfg.MinPriorityScore = 0
	cfg.TopKPerCluster = 1
	cfg.MaxPerPattern = 20
	cfg.EnablePatternFilter = false // Skip pattern detection for speed
	cfg.MaxConcurrency = 8
	return cfg
}

// ThoroughConfig returns configuration for maximum coverage.
func ThoroughConfig() FilterConfig {
	cfg := DefaultConfig()
	cfg.MaxURLs = 500000
	cfg.SimilarityThreshold = 0.75
	cfg.MinPriorityScore = -200
	cfg.TopKPerCluster = 5
	cfg.MaxPerPattern = 100
	cfg.MinPatternFrequency = 2
	cfg.MaxConcurrency = 2 // More thorough = less concurrent
	cfg.Timeout = 15 * time.Minute
	return cfg
}

// Validate checks if configuration is valid.
func (c FilterConfig) Validate() error {
	if c.MaxURLs < 0 {
		return fmt.Errorf("max_urls must be >= 0, got %d", c.MaxURLs)
	}

	if c.SimilarityThreshold < 0.0 || c.SimilarityThreshold > 1.0 {
		return fmt.Errorf("similarity_threshold must be in [0.0, 1.0], got %.2f", c.SimilarityThreshold)
	}

	if c.TopKPerCluster < 1 {
		return fmt.Errorf("top_k_per_cluster must be >= 1, got %d", c.TopKPerCluster)
	}

	if c.MaxPerPattern < 0 {
		return fmt.Errorf("max_per_pattern must be >= 0, got %d", c.MaxPerPattern)
	}

	if c.MinPatternFrequency < 1 {
		return fmt.Errorf("min_pattern_frequency must be >= 1, got %d", c.MinPatternFrequency)
	}

	if c.BloomFPRate <= 0.0 || c.BloomFPRate >= 1.0 {
		return fmt.Errorf("bloom_fp_rate must be in (0.0, 1.0), got %.4f", c.BloomFPRate)
	}

	if c.MaxConcurrency < 1 {
		return fmt.Errorf("max_concurrency must be >= 1, got %d", c.MaxConcurrency)
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got %v", c.Timeout)
	}

	return nil
}

// FilterStats tracks filtering statistics.
type FilterStats struct {
	// Input/Output
	InputURLs  int `json:"input_urls"`
	OutputURLs int `json:"output_urls"`

	// Deduplication
	DuplicatesSkipped   int `json:"duplicates_skipped"`
	LowPrioritySkipped  int `json:"low_priority_skipped"`
	OutOfScopeSkipped   int `json:"out_of_scope_skipped"`
	InvalidURLsSkipped  int `json:"invalid_urls_skipped"`

	// Clustering
	ClustersMerged int `json:"clusters_merged"`
	PatternsFound  int `json:"patterns_found"`

	// Performance
	MemoryBytes int64         `json:"memory_bytes"`
	DurationMs  int64         `json:"duration_ms"`
	Duration    time.Duration `json:"-"`

	// Bloom filter stats
	BloomFilterSize   uint64  `json:"bloom_filter_size"`
	BloomFilterChecks uint64  `json:"bloom_filter_checks"`
	BloomFilterHits   uint64  `json:"bloom_filter_hits"`
	BloomFilterFPRate float64 `json:"bloom_filter_fp_rate"`
}

// ReductionRatio returns percentage of URLs filtered out.
func (s FilterStats) ReductionRatio() float64 {
	if s.InputURLs == 0 {
		return 0.0
	}
	return float64(s.InputURLs-s.OutputURLs) / float64(s.InputURLs) * 100.0
}

// ThroughputURLsPerSecond returns processing throughput.
func (s FilterStats) ThroughputURLsPerSecond() float64 {
	if s.Duration == 0 {
		return 0.0
	}
	return float64(s.InputURLs) / s.Duration.Seconds()
}

// String returns human-readable statistics.
func (s FilterStats) String() string {
	return fmt.Sprintf(
		"FilterStats{input: %d, output: %d, reduction: %.1f%%, duplicates: %d, low_priority: %d, clusters: %d, patterns: %d, duration: %v, throughput: %.0f urls/s}",
		s.InputURLs,
		s.OutputURLs,
		s.ReductionRatio(),
		s.DuplicatesSkipped,
		s.LowPrioritySkipped,
		s.ClustersMerged,
		s.PatternsFound,
		s.Duration,
		s.ThroughputURLsPerSecond(),
	)
}
