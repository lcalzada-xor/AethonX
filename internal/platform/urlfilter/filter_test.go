package urlfilter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"aethonx/internal/platform/logx"
)

func TestFilterEngine_BasicFiltering(t *testing.T) {
	logger := logx.New()
	cfg := DefaultConfig()
	cfg.MaxURLs = 1000
	engine := NewFilterEngine(cfg, logger)

	// Create test URLs with duplicates and different priorities
	urls := []string{
		"https://example.com/.env",
		"https://example.com/api/users/1",
		"https://example.com/api/users/2",
		"https://example.com/api/users/3",
		"https://example.com/images/logo.png",
		"https://example.com/images/photo.jpg",
		"https://example.com/.env", // Duplicate
	}

	ctx := context.Background()
	scored, stats, err := engine.Filter(ctx, urls)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have removed duplicate
	if stats.DuplicatesSkipped == 0 {
		t.Error("expected at least one duplicate to be skipped")
	}

	// Output should be less than input
	if stats.OutputURLs >= stats.InputURLs {
		t.Errorf("expected output (%d) < input (%d)", stats.OutputURLs, stats.InputURLs)
	}

	// High priority URLs should be first
	if len(scored) > 0 && scored[0].Score < 500 {
		t.Errorf("expected highest priority URL first, got score %d", scored[0].Score)
	}

	t.Logf("Filter Stats: %s", stats.String())
}

func TestFilterEngine_VolumeControl(t *testing.T) {
	logger := logx.New()
	cfg := DefaultConfig()
	cfg.MaxURLs = 100
	cfg.EnableVolumeControl = true
	engine := NewFilterEngine(cfg, logger)

	// Create more URLs than max
	urls := make([]string, 500)
	for i := 0; i < 500; i++ {
		urls[i] = fmt.Sprintf("https://example.com/page%d", i)
	}

	ctx := context.Background()
	scored, stats, err := engine.Filter(ctx, urls)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have been truncated
	if stats.InputURLs != 100 {
		t.Errorf("expected input to be truncated to 100, got %d", stats.InputURLs)
	}

	if len(scored) > 100 {
		t.Errorf("expected output <= 100, got %d", len(scored))
	}
}

func TestFilterEngine_Clustering(t *testing.T) {
	logger := logx.New()
	cfg := DefaultConfig()
	cfg.EnableClustering = true
	cfg.SimilarityThreshold = 0.85
	cfg.TopKPerCluster = 2
	engine := NewFilterEngine(cfg, logger)

	// Create similar URLs
	urls := make([]string, 20)
	for i := 0; i < 20; i++ {
		urls[i] = fmt.Sprintf("https://example.com/api/users/%d", i)
	}

	ctx := context.Background()
	_, stats, err := engine.Filter(ctx, urls)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have created clusters
	if stats.ClustersMerged == 0 {
		t.Error("expected clusters to be created")
	}

	// Output should be much less than input due to clustering
	if stats.OutputURLs >= stats.InputURLs {
		t.Errorf("expected significant reduction from clustering")
	}

	t.Logf("Clustering: %d URLs -> %d clusters -> %d output",
		stats.InputURLs, stats.ClustersMerged, stats.OutputURLs)
}

func TestFilterEngine_PatternFiltering(t *testing.T) {
	logger := logx.New()
	cfg := DefaultConfig()
	cfg.EnablePatternFilter = true
	cfg.MaxPerPattern = 3
	cfg.MinPatternFrequency = 5
	engine := NewFilterEngine(cfg, logger)

	// Create URLs with clear patterns
	urls := make([]string, 50)
	for i := 0; i < 50; i++ {
		urls[i] = fmt.Sprintf("https://example.com/products/%d/details", i)
	}

	ctx := context.Background()
	_, stats, err := engine.Filter(ctx, urls)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have detected pattern
	if stats.PatternsFound == 0 {
		t.Error("expected patterns to be detected")
	}

	// Should have limited URLs per pattern
	if stats.OutputURLs > cfg.MaxPerPattern*2 {
		t.Errorf("expected max %d URLs per pattern, got %d output",
			cfg.MaxPerPattern, stats.OutputURLs)
	}

	t.Logf("Pattern filtering: %d patterns, %d output", stats.PatternsFound, stats.OutputURLs)
}

func TestFilterEngine_PriorityFiltering(t *testing.T) {
	logger := logx.New()
	cfg := DefaultConfig()
	cfg.MinPriorityScore = 0 // Filter out negative scores
	engine := NewFilterEngine(cfg, logger)

	urls := []string{
		"https://example.com/.env",            // High priority
		"https://example.com/api/users",       // Medium priority
		"https://example.com/images/logo.png", // Low priority (negative)
		"https://example.com/css/style.css",   // Low priority (negative)
	}

	ctx := context.Background()
	scored, stats, err := engine.Filter(ctx, urls)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have filtered low priority
	if stats.LowPrioritySkipped == 0 {
		t.Error("expected low priority URLs to be skipped")
	}

	// All remaining should have score >= 0
	for _, s := range scored {
		if s.Score < 0 {
			t.Errorf("expected all scores >= 0, got %d for %s", s.Score, s.URL)
		}
	}
}

func TestFilterEngine_BloomFilterEnabled(t *testing.T) {
	logger := logx.New()
	cfg := DefaultConfig()
	cfg.UseBloomFilter = true
	engine := NewFilterEngine(cfg, logger)

	// Add duplicates
	urls := []string{
		"https://example.com/page1",
		"https://example.com/page2",
		"https://example.com/page1", // Duplicate
		"https://example.com/page3",
		"https://example.com/page2", // Duplicate
	}

	ctx := context.Background()
	_, stats, err := engine.Filter(ctx, urls)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have used bloom filter
	if stats.BloomFilterChecks == 0 {
		t.Error("expected bloom filter to be used")
	}

	// Should have detected duplicates
	if stats.DuplicatesSkipped < 2 {
		t.Errorf("expected at least 2 duplicates, got %d", stats.DuplicatesSkipped)
	}
}

func TestFilterEngine_ContextCancellation(t *testing.T) {
	logger := logx.New()
	cfg := DefaultConfig()
	engine := NewFilterEngine(cfg, logger)

	urls := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		urls[i] = fmt.Sprintf("https://example.com/page%d", i)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Should handle cancellation gracefully
	_, _, err := engine.Filter(ctx, urls)

	// May or may not error depending on timing, but should not panic
	if err != nil {
		t.Logf("context cancelled as expected: %v", err)
	}
}

func TestFilterEngine_InvalidURLs(t *testing.T) {
	logger := logx.New()
	cfg := DefaultConfig()
	engine := NewFilterEngine(cfg, logger)

	urls := []string{
		"https://example.com/valid",
		"not a url",
		"://invalid",
		"https://example.com/another-valid",
	}

	ctx := context.Background()
	scored, stats, err := engine.Filter(ctx, urls)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have skipped invalid URLs
	if stats.InvalidURLsSkipped != 2 {
		t.Errorf("expected 2 invalid URLs, got %d", stats.InvalidURLsSkipped)
	}

	// Should still process valid URLs
	if len(scored) != 2 {
		t.Errorf("expected 2 valid URLs, got %d", len(scored))
	}
}

func TestFilterEngine_EmptyInput(t *testing.T) {
	logger := logx.New()
	cfg := DefaultConfig()
	engine := NewFilterEngine(cfg, logger)

	ctx := context.Background()
	_, _, err := engine.Filter(ctx, []string{})

	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestFilterEngine_Config(t *testing.T) {
	logger := logx.New()
	cfg := DefaultConfig()
	cfg.MaxURLs = 12345
	engine := NewFilterEngine(cfg, logger)

	returnedCfg := engine.Config()

	if returnedCfg.MaxURLs != 12345 {
		t.Errorf("expected MaxURLs=12345, got %d", returnedCfg.MaxURLs)
	}
}

func TestFilterEngine_Clear(t *testing.T) {
	logger := logx.New()
	cfg := DefaultConfig()
	cfg.UseBloomFilter = true
	engine := NewFilterEngine(cfg, logger)

	urls := []string{"https://example.com/page1"}
	ctx := context.Background()
	engine.Filter(ctx, urls)

	// Should not panic
	engine.Clear()

	// After clear, should work again
	_, _, err := engine.Filter(ctx, urls)
	if err != nil {
		t.Errorf("filter should work after clear: %v", err)
	}
}

func TestFilterEngine_LargeDataset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large dataset test in short mode")
	}

	logger := logx.New()
	cfg := FastConfig() // Use fast config for speed
	engine := NewFilterEngine(cfg, logger)

	// Create 10K URLs with patterns and duplicates
	urls := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		switch i % 10 {
		case 0:
			urls[i] = fmt.Sprintf("https://example.com/api/users/%d", i/10)
		case 1:
			urls[i] = fmt.Sprintf("https://example.com/api/products/%d", i/10)
		case 2:
			urls[i] = "https://example.com/images/logo.png" // Duplicate static asset
		default:
			urls[i] = fmt.Sprintf("https://example.com/pages/%d", i)
		}
	}

	ctx := context.Background()
	start := time.Now()
	scored, stats, err := engine.Filter(ctx, urls)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("Large dataset filtering:")
	t.Logf("  Input: %d URLs", stats.InputURLs)
	t.Logf("  Output: %d URLs", stats.OutputURLs)
	t.Logf("  Reduction: %.1f%%", stats.ReductionRatio())
	t.Logf("  Duration: %v", duration)
	t.Logf("  Throughput: %.0f URLs/s", stats.ThroughputURLsPerSecond())
	t.Logf("  Duplicates: %d", stats.DuplicatesSkipped)
	t.Logf("  Clusters: %d", stats.ClustersMerged)

	// Verify significant reduction
	if stats.ReductionRatio() < 50.0 {
		t.Errorf("expected at least 50%% reduction, got %.1f%%", stats.ReductionRatio())
	}

	// Verify output is reasonable
	if len(scored) == 0 {
		t.Error("expected some output URLs")
	}

	// Verify performance (should process >10K URLs/s)
	if stats.ThroughputURLsPerSecond() < 1000 {
		t.Errorf("throughput too low: %.0f URLs/s", stats.ThroughputURLsPerSecond())
	}
}

func BenchmarkFilterEngine_Filter(b *testing.B) {
	logger := logx.New()
	cfg := FastConfig()
	engine := NewFilterEngine(cfg, logger)

	urls := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		urls[i] = fmt.Sprintf("https://example.com/page%d", i)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Clear()
		_, _, _ = engine.Filter(ctx, urls)
	}
}
