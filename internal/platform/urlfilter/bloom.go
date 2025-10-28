package urlfilter

import (
	"hash/fnv"
	"math"
	"sync"

	"aethonx/internal/platform/logx"
)

// BloomFilter is a space-efficient probabilistic data structure for deduplication.
// It provides O(1) membership testing with controlled false positive rate.
// False negatives are impossible: if MayContain returns false, element is definitely not present.
type BloomFilter struct {
	bits   []uint64      // Bit array
	size   uint64        // Number of bits
	hashes int           // Number of hash functions
	count  uint64        // Approximate element count
	checks uint64        // Total checks performed
	hits   uint64        // Cache hits (already seen)
	mu     sync.RWMutex  // Thread-safe operations
	logger logx.Logger
}

// NewBloomFilter creates a Bloom filter optimized for expected element count.
// falsePositiveRate: desired false positive probability (e.g., 0.01 = 1%)
// expectedElements: estimated number of unique elements
func NewBloomFilter(expectedElements int, falsePositiveRate float64, logger logx.Logger) *BloomFilter {
	if expectedElements <= 0 {
		expectedElements = 10000
	}
	if falsePositiveRate <= 0 || falsePositiveRate >= 1 {
		falsePositiveRate = 0.01
	}

	// Calculate optimal bit array size: m = -n*ln(p) / (ln(2)^2)
	m := -float64(expectedElements) * math.Log(falsePositiveRate) / (math.Ln2 * math.Ln2)
	size := uint64(math.Ceil(m))

	// Calculate optimal number of hash functions: k = (m/n) * ln(2)
	k := int(math.Ceil((m / float64(expectedElements)) * math.Ln2))
	if k < 1 {
		k = 1
	}

	// Allocate bit array (using uint64 chunks)
	chunks := (size + 63) / 64 // Round up to nearest 64
	bits := make([]uint64, chunks)

	logger.Debug("created bloom filter",
		"expected_elements", expectedElements,
		"false_positive_rate", falsePositiveRate,
		"size_bits", size,
		"hash_functions", k,
		"memory_bytes", chunks*8,
	)

	return &BloomFilter{
		bits:   bits,
		size:   size,
		hashes: k,
		logger: logger.With("component", "bloom"),
	}
}

// Add adds an element to the Bloom filter.
func (b *BloomFilter) Add(element string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	hashes := b.computeHashes(element)
	for _, h := range hashes {
		idx := h % b.size
		chunkIdx := idx / 64
		bitIdx := idx % 64
		b.bits[chunkIdx] |= (1 << bitIdx)
	}

	b.count++
}

// MayContain checks if element might be in the set.
// Returns true if element might exist (can be false positive).
// Returns false if element definitely doesn't exist (no false negatives).
func (b *BloomFilter) MayContain(element string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	b.checks++

	hashes := b.computeHashes(element)
	for _, h := range hashes {
		idx := h % b.size
		chunkIdx := idx / 64
		bitIdx := idx % 64

		if (b.bits[chunkIdx] & (1 << bitIdx)) == 0 {
			// Bit not set, element definitely not present
			return false
		}
	}

	// All bits set, element might be present
	b.hits++
	return true
}

// AddAndCheck atomically adds element and returns whether it was already present.
// Returns true if element was likely already present (potential false positive).
func (b *BloomFilter) AddAndCheck(element string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.checks++

	hashes := b.computeHashes(element)
	allSet := true

	for _, h := range hashes {
		idx := h % b.size
		chunkIdx := idx / 64
		bitIdx := idx % 64
		mask := uint64(1 << bitIdx)

		// Check if bit already set
		if (b.bits[chunkIdx] & mask) == 0 {
			allSet = false
		}

		// Set bit
		b.bits[chunkIdx] |= mask
	}

	if allSet {
		b.hits++
	} else {
		b.count++
	}

	return allSet
}

// computeHashes generates k hash values for element using double hashing.
// Uses FNV-1a as base hash, then derives k hashes using Kirsch-Mitzenmacher technique.
func (b *BloomFilter) computeHashes(element string) []uint64 {
	// Use FNV-1a hash (fast, good distribution)
	h1 := fnv.New64a()
	h1.Write([]byte(element))
	hash1 := h1.Sum64()

	// Second hash using modified input
	h2 := fnv.New64a()
	h2.Write([]byte(element + "\x00"))
	hash2 := h2.Sum64()

	// Generate k hashes using double hashing: h_i = h1 + i*h2
	hashes := make([]uint64, b.hashes)
	for i := 0; i < b.hashes; i++ {
		hashes[i] = hash1 + uint64(i)*hash2
	}

	return hashes
}

// Count returns approximate number of elements added (not accounting for duplicates).
func (b *BloomFilter) Count() uint64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count
}

// Size returns total bit array size.
func (b *BloomFilter) Size() uint64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.size
}

// MemoryBytes returns approximate memory usage in bytes.
func (b *BloomFilter) MemoryBytes() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return int64(len(b.bits) * 8)
}

// Stats returns Bloom filter statistics.
func (b *BloomFilter) Stats() BloomStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Estimate actual false positive rate based on fill ratio
	fillRatio := b.estimateFillRatio()
	actualFP := math.Pow(fillRatio, float64(b.hashes))

	// Calculate hit rate, avoiding division by zero
	hitRate := 0.0
	if b.checks > 0 {
		hitRate = float64(b.hits) / float64(b.checks)
	}

	return BloomStats{
		Size:              b.size,
		Hashes:            b.hashes,
		Count:             b.count,
		Checks:            b.checks,
		Hits:              b.hits,
		MemoryBytes:       int64(len(b.bits) * 8),
		FillRatio:         fillRatio,
		EstimatedFPRate:   actualFP,
		HitRate:           hitRate,
	}
}

// BloomStats contains Bloom filter statistics.
type BloomStats struct {
	Size            uint64  // Total bit array size
	Hashes          int     // Number of hash functions
	Count           uint64  // Approximate elements added
	Checks          uint64  // Total membership checks
	Hits            uint64  // Checks that returned true
	MemoryBytes     int64   // Memory usage
	FillRatio       float64 // Percentage of bits set
	EstimatedFPRate float64 // Estimated false positive rate
	HitRate         float64 // Percentage of checks that were hits
}

// estimateFillRatio estimates percentage of bits set in the filter.
// Uses sampling for large bit arrays to avoid full scan.
func (b *BloomFilter) estimateFillRatio() float64 {
	const sampleSize = 1000
	chunks := len(b.bits)

	if chunks == 0 {
		return 0.0
	}

	// For small arrays, count all bits
	if chunks <= sampleSize {
		setBits := uint64(0)
		for _, chunk := range b.bits {
			setBits += uint64(popcount(chunk))
		}
		return float64(setBits) / float64(b.size)
	}

	// For large arrays, sample
	step := chunks / sampleSize
	if step < 1 {
		step = 1
	}

	setBits := uint64(0)
	sampledChunks := 0
	for i := 0; i < chunks; i += step {
		setBits += uint64(popcount(b.bits[i]))
		sampledChunks++
	}

	avgBitsPerChunk := float64(setBits) / float64(sampledChunks)
	estimatedTotalBits := avgBitsPerChunk * float64(chunks)

	return estimatedTotalBits / float64(b.size)
}

// popcount counts number of set bits in uint64.
func popcount(x uint64) int {
	// Brian Kernighan's algorithm
	count := 0
	for x != 0 {
		x &= x - 1
		count++
	}
	return count
}

// Clear resets the Bloom filter.
func (b *BloomFilter) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i := range b.bits {
		b.bits[i] = 0
	}
	b.count = 0
	b.checks = 0
	b.hits = 0
}
