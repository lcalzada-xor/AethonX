package urlfilter

import (
	"fmt"
	"testing"

	"aethonx/internal/platform/logx"
)

func TestBloomFilter_Basic(t *testing.T) {
	logger := logx.New()
	bf := NewBloomFilter(100, 0.01, logger)

	// Add some elements
	bf.Add("https://example.com/page1")
	bf.Add("https://example.com/page2")
	bf.Add("https://example.com/page3")

	// Test contains (should return true)
	if !bf.MayContain("https://example.com/page1") {
		t.Error("expected page1 to be in filter")
	}
	if !bf.MayContain("https://example.com/page2") {
		t.Error("expected page2 to be in filter")
	}
	if !bf.MayContain("https://example.com/page3") {
		t.Error("expected page3 to be in filter")
	}

	// Test not contains (should return false - no false negatives)
	if bf.MayContain("https://example.com/page4") {
		t.Log("page4 returned true (might be false positive, acceptable)")
	}
}

func TestBloomFilter_AddAndCheck(t *testing.T) {
	logger := logx.New()
	bf := NewBloomFilter(100, 0.01, logger)

	// First add should return false (not present)
	if bf.AddAndCheck("https://example.com/page1") {
		t.Error("first add should return false (not present)")
	}

	// Second add should return true (already present)
	if !bf.AddAndCheck("https://example.com/page1") {
		t.Error("second add should return true (already present)")
	}
}

func TestBloomFilter_Count(t *testing.T) {
	logger := logx.New()
	bf := NewBloomFilter(100, 0.01, logger)

	if bf.Count() != 0 {
		t.Errorf("expected count=0, got %d", bf.Count())
	}

	bf.Add("url1")
	bf.Add("url2")
	bf.Add("url3")

	if bf.Count() != 3 {
		t.Errorf("expected count=3, got %d", bf.Count())
	}
}

func TestBloomFilter_Clear(t *testing.T) {
	logger := logx.New()
	bf := NewBloomFilter(100, 0.01, logger)

	bf.Add("url1")
	bf.Add("url2")

	if bf.Count() != 2 {
		t.Errorf("expected count=2, got %d", bf.Count())
	}

	bf.Clear()

	if bf.Count() != 0 {
		t.Errorf("expected count=0 after clear, got %d", bf.Count())
	}

	// Should not contain previously added elements
	if bf.MayContain("url1") {
		t.Error("should not contain url1 after clear")
	}
}

func TestBloomFilter_FalsePositiveRate(t *testing.T) {
	logger := logx.New()
	expectedElements := 1000
	targetFPR := 0.01 // 1%

	bf := NewBloomFilter(expectedElements, targetFPR, logger)

	// Add expectedElements
	for i := 0; i < expectedElements; i++ {
		bf.Add(fmt.Sprintf("https://example.com/page%d", i))
	}

	// Test false positive rate with non-added elements
	falsePositives := 0
	testSize := 10000

	for i := expectedElements; i < expectedElements+testSize; i++ {
		if bf.MayContain(fmt.Sprintf("https://example.com/page%d", i)) {
			falsePositives++
		}
	}

	actualFPR := float64(falsePositives) / float64(testSize)

	t.Logf("Expected FPR: %.4f, Actual FPR: %.4f (%d/%d false positives)",
		targetFPR, actualFPR, falsePositives, testSize)

	// Allow some tolerance (3x target rate for realistic variance)
	if actualFPR > targetFPR*3 {
		t.Errorf("false positive rate too high: %.4f > %.4f", actualFPR, targetFPR*3)
	}
}

func TestBloomFilter_NoFalseNegatives(t *testing.T) {
	logger := logx.New()
	bf := NewBloomFilter(1000, 0.01, logger)

	// Add 500 URLs
	urls := make([]string, 500)
	for i := 0; i < 500; i++ {
		urls[i] = fmt.Sprintf("https://example.com/page%d", i)
		bf.Add(urls[i])
	}

	// Verify all added URLs are found (no false negatives allowed)
	for i, url := range urls {
		if !bf.MayContain(url) {
			t.Errorf("false negative at index %d: %s", i, url)
		}
	}
}

func TestBloomFilter_Stats(t *testing.T) {
	logger := logx.New()
	bf := NewBloomFilter(100, 0.01, logger)

	stats := bf.Stats()

	if stats.Size == 0 {
		t.Error("expected non-zero size")
	}

	if stats.Hashes == 0 {
		t.Error("expected non-zero hash count")
	}

	if stats.MemoryBytes == 0 {
		t.Error("expected non-zero memory usage")
	}

	// Add some elements and check again
	bf.Add("url1")
	bf.Add("url2")
	bf.MayContain("url1")
	bf.MayContain("url3")

	stats = bf.Stats()

	if stats.Count != 2 {
		t.Errorf("expected count=2, got %d", stats.Count)
	}

	if stats.Checks != 2 {
		t.Errorf("expected checks=2, got %d", stats.Checks)
	}

	if stats.Hits == 0 {
		t.Error("expected at least one hit")
	}

	t.Logf("Bloom Filter Stats: %+v", stats)
}

func TestBloomFilter_MemoryEfficiency(t *testing.T) {
	logger := logx.New()

	tests := []struct {
		name             string
		expectedElements int
		fpRate           float64
	}{
		{"small", 1000, 0.01},
		{"medium", 10000, 0.01},
		{"large", 100000, 0.01},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bf := NewBloomFilter(tt.expectedElements, tt.fpRate, logger)
			memoryBytes := bf.MemoryBytes()

			// Calculate expected memory (rough estimate)
			// m = -n*ln(p) / (ln(2)^2)
			// Memory should be much less than storing strings directly
			naiveMemory := int64(tt.expectedElements * 100) // Assume ~100 bytes per URL

			t.Logf("%s: %d elements, Bloom: %d bytes, Naive: %d bytes, Savings: %.1f%%",
				tt.name, tt.expectedElements, memoryBytes, naiveMemory,
				(1.0-float64(memoryBytes)/float64(naiveMemory))*100)

			if memoryBytes > naiveMemory/10 {
				t.Errorf("bloom filter not efficient enough: %d > %d",
					memoryBytes, naiveMemory/10)
			}
		})
	}
}

func TestBloomFilter_ConcurrentAccess(t *testing.T) {
	logger := logx.New()
	bf := NewBloomFilter(1000, 0.01, logger)

	// Concurrent adds
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				bf.Add(fmt.Sprintf("url-%d-%d", id, j))
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify count (approximate due to concurrency)
	count := bf.Count()
	if count == 0 {
		t.Error("expected non-zero count after concurrent adds")
	}

	t.Logf("Added ~1000 URLs concurrently, count: %d", count)
}

func BenchmarkBloomFilter_Add(b *testing.B) {
	logger := logx.New()
	bf := NewBloomFilter(b.N, 0.01, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bf.Add(fmt.Sprintf("https://example.com/page%d", i))
	}
}

func BenchmarkBloomFilter_MayContain(b *testing.B) {
	logger := logx.New()
	bf := NewBloomFilter(10000, 0.01, logger)

	// Pre-populate
	for i := 0; i < 10000; i++ {
		bf.Add(fmt.Sprintf("https://example.com/page%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bf.MayContain(fmt.Sprintf("https://example.com/page%d", i%10000))
	}
}

func BenchmarkBloomFilter_AddAndCheck(b *testing.B) {
	logger := logx.New()
	bf := NewBloomFilter(b.N, 0.01, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bf.AddAndCheck(fmt.Sprintf("https://example.com/page%d", i))
	}
}
