package urlfilter

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Verify basic settings
	if cfg.MaxURLs != 100000 {
		t.Errorf("expected MaxURLs=100000, got %d", cfg.MaxURLs)
	}

	if cfg.SimilarityThreshold != 0.85 {
		t.Errorf("expected SimilarityThreshold=0.85, got %.2f", cfg.SimilarityThreshold)
	}

	if cfg.MinPriorityScore != -100 {
		t.Errorf("expected MinPriorityScore=-100, got %d", cfg.MinPriorityScore)
	}

	// Verify validation passes
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config should be valid: %v", err)
	}
}

func TestFastConfig(t *testing.T) {
	cfg := FastConfig()

	if cfg.MaxURLs != 10000 {
		t.Errorf("expected MaxURLs=10000, got %d", cfg.MaxURLs)
	}

	if cfg.SimilarityThreshold != 0.90 {
		t.Errorf("expected SimilarityThreshold=0.90, got %.2f", cfg.SimilarityThreshold)
	}

	if cfg.TopKPerCluster != 1 {
		t.Errorf("expected TopKPerCluster=1, got %d", cfg.TopKPerCluster)
	}
}

func TestThoroughConfig(t *testing.T) {
	cfg := ThoroughConfig()

	if cfg.MaxURLs != 500000 {
		t.Errorf("expected MaxURLs=500000, got %d", cfg.MaxURLs)
	}

	if cfg.SimilarityThreshold != 0.75 {
		t.Errorf("expected SimilarityThreshold=0.75, got %.2f", cfg.SimilarityThreshold)
	}

	if cfg.TopKPerCluster != 5 {
		t.Errorf("expected TopKPerCluster=5, got %d", cfg.TopKPerCluster)
	}
}

func TestFilterConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     FilterConfig
		wantErr bool
	}{
		{
			name:    "valid default config",
			cfg:     DefaultConfig(),
			wantErr: false,
		},
		{
			name: "negative max urls",
			cfg: FilterConfig{
				MaxURLs:             -1,
				SimilarityThreshold: 0.85,
				TopKPerCluster:      3,
				MaxPerPattern:       50,
				MinPatternFrequency: 3,
				BloomFPRate:         0.01,
				MaxConcurrency:      4,
				Timeout:             time.Minute,
			},
			wantErr: true,
		},
		{
			name: "invalid similarity threshold (too high)",
			cfg: FilterConfig{
				MaxURLs:             100000,
				SimilarityThreshold: 1.5,
				TopKPerCluster:      3,
				MaxPerPattern:       50,
				MinPatternFrequency: 3,
				BloomFPRate:         0.01,
				MaxConcurrency:      4,
				Timeout:             time.Minute,
			},
			wantErr: true,
		},
		{
			name: "invalid similarity threshold (negative)",
			cfg: FilterConfig{
				MaxURLs:             100000,
				SimilarityThreshold: -0.1,
				TopKPerCluster:      3,
				MaxPerPattern:       50,
				MinPatternFrequency: 3,
				BloomFPRate:         0.01,
				MaxConcurrency:      4,
				Timeout:             time.Minute,
			},
			wantErr: true,
		},
		{
			name: "invalid top k per cluster",
			cfg: FilterConfig{
				MaxURLs:             100000,
				SimilarityThreshold: 0.85,
				TopKPerCluster:      0,
				MaxPerPattern:       50,
				MinPatternFrequency: 3,
				BloomFPRate:         0.01,
				MaxConcurrency:      4,
				Timeout:             time.Minute,
			},
			wantErr: true,
		},
		{
			name: "invalid bloom fp rate (too high)",
			cfg: FilterConfig{
				MaxURLs:             100000,
				SimilarityThreshold: 0.85,
				TopKPerCluster:      3,
				MaxPerPattern:       50,
				MinPatternFrequency: 3,
				BloomFPRate:         1.0,
				MaxConcurrency:      4,
				Timeout:             time.Minute,
			},
			wantErr: true,
		},
		{
			name: "invalid bloom fp rate (zero)",
			cfg: FilterConfig{
				MaxURLs:             100000,
				SimilarityThreshold: 0.85,
				TopKPerCluster:      3,
				MaxPerPattern:       50,
				MinPatternFrequency: 3,
				BloomFPRate:         0.0,
				MaxConcurrency:      4,
				Timeout:             time.Minute,
			},
			wantErr: true,
		},
		{
			name: "invalid max concurrency",
			cfg: FilterConfig{
				MaxURLs:             100000,
				SimilarityThreshold: 0.85,
				TopKPerCluster:      3,
				MaxPerPattern:       50,
				MinPatternFrequency: 3,
				BloomFPRate:         0.01,
				MaxConcurrency:      0,
				Timeout:             time.Minute,
			},
			wantErr: true,
		},
		{
			name: "invalid timeout",
			cfg: FilterConfig{
				MaxURLs:             100000,
				SimilarityThreshold: 0.85,
				TopKPerCluster:      3,
				MaxPerPattern:       50,
				MinPatternFrequency: 3,
				BloomFPRate:         0.01,
				MaxConcurrency:      4,
				Timeout:             0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFilterStats_ReductionRatio(t *testing.T) {
	tests := []struct {
		name      string
		stats     FilterStats
		wantRatio float64
	}{
		{
			name: "50% reduction",
			stats: FilterStats{
				InputURLs:  1000,
				OutputURLs: 500,
			},
			wantRatio: 50.0,
		},
		{
			name: "90% reduction",
			stats: FilterStats{
				InputURLs:  1000,
				OutputURLs: 100,
			},
			wantRatio: 90.0,
		},
		{
			name: "no reduction",
			stats: FilterStats{
				InputURLs:  1000,
				OutputURLs: 1000,
			},
			wantRatio: 0.0,
		},
		{
			name: "zero input",
			stats: FilterStats{
				InputURLs:  0,
				OutputURLs: 0,
			},
			wantRatio: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio := tt.stats.ReductionRatio()
			if ratio != tt.wantRatio {
				t.Errorf("ReductionRatio() = %.2f, want %.2f", ratio, tt.wantRatio)
			}
		})
	}
}

func TestFilterStats_ThroughputURLsPerSecond(t *testing.T) {
	tests := []struct {
		name           string
		stats          FilterStats
		wantThroughput float64
	}{
		{
			name: "1000 URLs in 1 second",
			stats: FilterStats{
				InputURLs: 1000,
				Duration:  time.Second,
			},
			wantThroughput: 1000.0,
		},
		{
			name: "5000 URLs in 2 seconds",
			stats: FilterStats{
				InputURLs: 5000,
				Duration:  2 * time.Second,
			},
			wantThroughput: 2500.0,
		},
		{
			name: "zero duration",
			stats: FilterStats{
				InputURLs: 1000,
				Duration:  0,
			},
			wantThroughput: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			throughput := tt.stats.ThroughputURLsPerSecond()
			if throughput != tt.wantThroughput {
				t.Errorf("ThroughputURLsPerSecond() = %.2f, want %.2f", throughput, tt.wantThroughput)
			}
		})
	}
}

func TestNormalizationStrategy_String(t *testing.T) {
	tests := []struct {
		strategy NormalizationStrategy
		want     string
	}{
		{NormBasic, "basic"},
		{NormStructural, "structural"},
		{NormParametric, "parametric"},
		{NormExtensionless, "extensionless"},
		{NormAggressive, "aggressive"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.strategy.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSimilarityAlgorithm_String(t *testing.T) {
	tests := []struct {
		algorithm SimilarityAlgorithm
		want      string
	}{
		{SimLevenshtein, "levenshtein"},
		{SimJaccard, "jaccard"},
		{SimTemplate, "template"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.algorithm.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}
