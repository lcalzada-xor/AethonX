package urlfilter

import (
	"testing"

	"aethonx/internal/platform/logx"
)

func TestPriorityScorer_SensitiveFiles(t *testing.T) {
	logger := logx.New()
	scorer := NewPriorityScorer(DefaultScoreWeights(), logger)

	tests := []struct {
		url           string
		expectedScore int
		expectedHighPriority bool
	}{
		{"https://example.com/.env", 1000, true},
		{"https://example.com/config.php", 1000, true},
		{"https://example.com/credentials.json", 1000, true},
		{"https://example.com/id_rsa", 1000, true},
		{"https://example.com/normal.txt", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			scored := scorer.Score(tt.url)

			if tt.expectedHighPriority && scored.Score < 500 {
				t.Errorf("expected high priority (>500), got %d", scored.Score)
			}

			hasReason := false
			for _, reason := range scored.Reasons {
				if reason == "sensitive_file" {
					hasReason = true
					break
				}
			}

			if tt.expectedHighPriority && !hasReason {
				t.Error("expected 'sensitive_file' reason")
			}
		})
	}
}

func TestPriorityScorer_Repository(t *testing.T) {
	logger := logx.New()
	scorer := NewPriorityScorer(DefaultScoreWeights(), logger)

	urls := []string{
		"https://example.com/.git/config",
		"https://example.com/.svn/entries",
		"https://example.com/.hg/store",
	}

	for _, url := range urls {
		t.Run(url, func(t *testing.T) {
			scored := scorer.Score(url)

			if scored.Score < 700 {
				t.Errorf("expected high score for repository, got %d", scored.Score)
			}
		})
	}
}

func TestPriorityScorer_AdminPaths(t *testing.T) {
	logger := logx.New()
	scorer := NewPriorityScorer(DefaultScoreWeights(), logger)

	urls := []string{
		"https://example.com/admin/dashboard",
		"https://example.com/wp-admin/",
		"https://example.com/administrator/",
	}

	for _, url := range urls {
		t.Run(url, func(t *testing.T) {
			scored := scorer.Score(url)

			if scored.Score < 300 {
				t.Errorf("expected medium-high score for admin path, got %d", scored.Score)
			}
		})
	}
}

func TestPriorityScorer_APIEndpoints(t *testing.T) {
	logger := logx.New()
	scorer := NewPriorityScorer(DefaultScoreWeights(), logger)

	urls := []string{
		"https://example.com/api/v1/users",
		"https://example.com/rest/products",
		"https://example.com/graphql",
	}

	for _, url := range urls {
		t.Run(url, func(t *testing.T) {
			scored := scorer.Score(url)

			if scored.Score < 200 {
				t.Errorf("expected positive score for API, got %d", scored.Score)
			}
		})
	}
}

func TestPriorityScorer_StaticAssets(t *testing.T) {
	logger := logx.New()
	scorer := NewPriorityScorer(DefaultScoreWeights(), logger)

	urls := []string{
		"https://example.com/images/logo.png",
		"https://example.com/css/style.css",
		"https://example.com/js/app.js",
	}

	for _, url := range urls {
		t.Run(url, func(t *testing.T) {
			scored := scorer.Score(url)

			if scored.Score >= 0 {
				t.Errorf("expected negative score for static asset, got %d", scored.Score)
			}
		})
	}
}

func TestPriorityScorer_TrackingParameters(t *testing.T) {
	logger := logx.New()
	scorer := NewPriorityScorer(DefaultScoreWeights(), logger)

	url := "https://example.com/page?utm_source=google&utm_campaign=test"
	scored := scorer.Score(url)

	hasReason := false
	for _, reason := range scored.Reasons {
		if reason == "tracking_params" {
			hasReason = true
			break
		}
	}

	if !hasReason {
		t.Error("expected 'tracking_params' reason")
	}
}

func TestPriorityScorer_ScoreBatch(t *testing.T) {
	logger := logx.New()
	scorer := NewPriorityScorer(DefaultScoreWeights(), logger)

	urls := []string{
		"https://example.com/.env",
		"https://example.com/api/users",
		"https://example.com/images/logo.png",
		"https://example.com/page",
	}

	scored := scorer.ScoreBatch(urls)

	if len(scored) != 4 {
		t.Errorf("expected 4 scored URLs, got %d", len(scored))
	}

	// Verify sorted by score (descending)
	for i := 0; i < len(scored)-1; i++ {
		if scored[i].Score < scored[i+1].Score {
			t.Errorf("results not sorted: %d < %d", scored[i].Score, scored[i+1].Score)
		}
	}

	// First should be .env (highest priority)
	if scored[0].Score < 900 {
		t.Errorf("expected .env to have highest score, got %d", scored[0].Score)
	}
}

func BenchmarkPriorityScorer_Score(b *testing.B) {
	logger := logx.New()
	scorer := NewPriorityScorer(DefaultScoreWeights(), logger)
	url := "https://example.com/api/v1/users/123?page=1&limit=10"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = scorer.Score(url)
	}
}
