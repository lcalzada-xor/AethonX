package urlfilter

import (
	"math"
	"net/url"
	"sort"
	"strings"

	"aethonx/internal/platform/logx"
)

// SimilarityDetector groups similar URLs using clustering algorithms.
type SimilarityDetector struct {
	threshold float64            // Similarity threshold (0.0-1.0)
	algorithm SimilarityAlgorithm
	logger    logx.Logger
}

// NewSimilarityDetector creates a new similarity detector.
func NewSimilarityDetector(threshold float64, algorithm SimilarityAlgorithm, logger logx.Logger) *SimilarityDetector {
	if threshold < 0.0 || threshold > 1.0 {
		threshold = 0.85
	}

	return &SimilarityDetector{
		threshold: threshold,
		algorithm: algorithm,
		logger:    logger.With("component", "similarity"),
	}
}

// URLCluster represents a group of similar URLs.
type URLCluster struct {
	Representative string   // Most complete/informative URL
	Members        []string // All similar URLs in cluster
	Count          int      // Cluster size
	Confidence     float64  // Similarity confidence (0.0-1.0)
	Signature      string   // Cluster signature
}

// Cluster groups similar URLs and returns representative clusters.
func (s *SimilarityDetector) Cluster(urls []string) []URLCluster {
	if len(urls) == 0 {
		return nil
	}

	s.logger.Debug("clustering URLs",
		"count", len(urls),
		"threshold", s.threshold,
		"algorithm", s.algorithm.String(),
	)

	// Select clustering algorithm
	var clusters []URLCluster
	switch s.algorithm {
	case SimLevenshtein:
		clusters = s.clusterByLevenshtein(urls)
	case SimJaccard:
		clusters = s.clusterByJaccard(urls)
	case SimTemplate:
		clusters = s.clusterByTemplate(urls)
	default:
		clusters = s.clusterByJaccard(urls) // Default to Jaccard
	}

	s.logger.Info("clustering complete",
		"input_urls", len(urls),
		"clusters", len(clusters),
		"reduction", len(urls)-len(clusters),
	)

	return clusters
}

// clusterByJaccard uses Jaccard similarity on path segments.
// Fast and effective for URL clustering.
func (s *SimilarityDetector) clusterByJaccard(urls []string) []URLCluster {
	// Parse and tokenize URLs
	type urlInfo struct {
		original string
		tokens   []string
		parsed   *url.URL
	}

	urlInfos := make([]urlInfo, 0, len(urls))
	for _, rawURL := range urls {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			s.logger.Debug("failed to parse URL", "url", rawURL, "error", err.Error())
			continue
		}

		tokens := s.tokenizeURL(parsed)
		urlInfos = append(urlInfos, urlInfo{
			original: rawURL,
			tokens:   tokens,
			parsed:   parsed,
		})
	}

	// Simple greedy clustering
	clusters := make([]URLCluster, 0)
	used := make(map[int]bool)

	for i, info1 := range urlInfos {
		if used[i] {
			continue
		}

		// Start new cluster
		cluster := URLCluster{
			Representative: info1.original,
			Members:        []string{info1.original},
			Count:          1,
			Signature:      s.generateSignature(info1.parsed),
		}

		// Find similar URLs
		maxSimilarity := 0.0
		for j := i + 1; j < len(urlInfos); j++ {
			if used[j] {
				continue
			}

			info2 := urlInfos[j]
			similarity := s.jaccardSimilarity(info1.tokens, info2.tokens)

			if similarity >= s.threshold {
				cluster.Members = append(cluster.Members, info2.original)
				cluster.Count++
				used[j] = true

				if similarity > maxSimilarity {
					maxSimilarity = similarity
				}
			}
		}

		cluster.Confidence = maxSimilarity
		if cluster.Count == 1 {
			cluster.Confidence = 1.0 // Single-member cluster has perfect confidence
		}

		// Select best representative (prefer shorter, more general URLs)
		cluster.Representative = s.selectRepresentative(cluster.Members)

		clusters = append(clusters, cluster)
		used[i] = true
	}

	return clusters
}

// clusterByLevenshtein uses Levenshtein distance for path similarity.
// More precise but slower than Jaccard.
func (s *SimilarityDetector) clusterByLevenshtein(urls []string) []URLCluster {
	// Parse URLs
	parsed := make([]*url.URL, 0, len(urls))
	for _, rawURL := range urls {
		p, err := url.Parse(rawURL)
		if err != nil {
			s.logger.Debug("failed to parse URL", "url", rawURL, "error", err.Error())
			continue
		}
		parsed = append(parsed, p)
	}

	// Simple greedy clustering by path similarity
	clusters := make([]URLCluster, 0)
	used := make(map[int]bool)

	for i, url1 := range parsed {
		if used[i] {
			continue
		}

		cluster := URLCluster{
			Representative: url1.String(),
			Members:        []string{url1.String()},
			Count:          1,
			Signature:      s.generateSignature(url1),
		}

		maxSimilarity := 0.0
		for j := i + 1; j < len(parsed); j++ {
			if used[j] {
				continue
			}

			url2 := parsed[j]

			// Only cluster URLs from same host
			if url1.Host != url2.Host {
				continue
			}

			similarity := s.levenshteinSimilarity(url1.Path, url2.Path)

			if similarity >= s.threshold {
				cluster.Members = append(cluster.Members, url2.String())
				cluster.Count++
				used[j] = true

				if similarity > maxSimilarity {
					maxSimilarity = similarity
				}
			}
		}

		cluster.Confidence = maxSimilarity
		if cluster.Count == 1 {
			cluster.Confidence = 1.0
		}

		cluster.Representative = s.selectRepresentative(cluster.Members)
		clusters = append(clusters, cluster)
		used[i] = true
	}

	return clusters
}

// clusterByTemplate groups URLs by structural templates.
func (s *SimilarityDetector) clusterByTemplate(urls []string) []URLCluster {
	// Use pattern extractor to identify templates
	extractor := NewPatternExtractor(1, s.logger) // minOccurrences=1 to find all patterns
	groups := extractor.GroupByPattern(urls)

	clusters := make([]URLCluster, 0, len(groups))
	for template, members := range groups {
		clusters = append(clusters, URLCluster{
			Representative: s.selectRepresentative(members),
			Members:        members,
			Count:          len(members),
			Confidence:     1.0, // Template matching has high confidence
			Signature:      template,
		})
	}

	// Sort by count (descending)
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Count > clusters[j].Count
	})

	return clusters
}

// tokenizeURL converts URL to tokens for Jaccard similarity.
func (s *SimilarityDetector) tokenizeURL(parsed *url.URL) []string {
	tokens := make([]string, 0, 10)

	// Add host as token
	tokens = append(tokens, "host:"+parsed.Host)

	// Add path segments
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for _, seg := range segments {
		if seg != "" {
			tokens = append(tokens, "path:"+seg)
		}
	}

	// Add query parameter keys (not values)
	for key := range parsed.Query() {
		tokens = append(tokens, "param:"+key)
	}

	return tokens
}

// jaccardSimilarity calculates Jaccard similarity coefficient between two token sets.
// Returns value in [0.0, 1.0] where 1.0 = identical.
func (s *SimilarityDetector) jaccardSimilarity(tokens1, tokens2 []string) float64 {
	if len(tokens1) == 0 && len(tokens2) == 0 {
		return 1.0
	}
	if len(tokens1) == 0 || len(tokens2) == 0 {
		return 0.0
	}

	// Build sets
	set1 := make(map[string]bool, len(tokens1))
	for _, t := range tokens1 {
		set1[t] = true
	}

	set2 := make(map[string]bool, len(tokens2))
	for _, t := range tokens2 {
		set2[t] = true
	}

	// Calculate intersection
	intersection := 0
	for t := range set1 {
		if set2[t] {
			intersection++
		}
	}

	// Calculate union
	union := len(set1) + len(set2) - intersection

	if union == 0 {
		return 1.0
	}

	return float64(intersection) / float64(union)
}

// levenshteinSimilarity calculates normalized Levenshtein similarity.
// Returns value in [0.0, 1.0] where 1.0 = identical.
func (s *SimilarityDetector) levenshteinSimilarity(s1, s2 string) float64 {
	distance := s.levenshteinDistance(s1, s2)
	maxLen := max(len(s1), len(s2))

	if maxLen == 0 {
		return 1.0
	}

	return 1.0 - (float64(distance) / float64(maxLen))
}

// levenshteinDistance calculates edit distance between two strings.
func (s *SimilarityDetector) levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Use Wagner-Fischer algorithm with single row optimization
	row := make([]int, len(s2)+1)
	for i := range row {
		row[i] = i
	}

	for i := 1; i <= len(s1); i++ {
		prev := i
		for j := 1; j <= len(s2); j++ {
			current := row[j]
			if s1[i-1] == s2[j-1] {
				row[j] = row[j-1]
			} else {
				row[j] = min(row[j-1], min(row[j], prev)) + 1
			}
			prev = current
		}
	}

	return row[len(s2)]
}

// selectRepresentative selects the best representative URL from a group.
// Prefers shorter, more general URLs.
func (s *SimilarityDetector) selectRepresentative(urls []string) string {
	if len(urls) == 0 {
		return ""
	}
	if len(urls) == 1 {
		return urls[0]
	}

	// Score each URL (lower is better for representative)
	type scoredURL struct {
		url   string
		score int
	}

	scoredURLs := make([]scoredURL, len(urls))
	for i, u := range urls {
		score := len(u) // Prefer shorter URLs

		// Penalize query parameters (representatives should be simple)
		if strings.Contains(u, "?") {
			paramCount := strings.Count(u, "&") + 1
			score += paramCount * 10
		}

		// Prefer paths without numeric IDs
		parsed, err := url.Parse(u)
		if err == nil {
			segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
			for _, seg := range segments {
				// Penalize numeric segments
				if isNumeric(seg) {
					score += 5
				}
			}
		}

		scoredURLs[i] = scoredURL{url: u, score: score}
	}

	// Sort by score (ascending)
	sort.Slice(scoredURLs, func(i, j int) bool {
		return scoredURLs[i].score < scoredURLs[j].score
	})

	return scoredURLs[0].url
}

// generateSignature generates a signature for a cluster based on URL structure.
func (s *SimilarityDetector) generateSignature(parsed *url.URL) string {
	// Signature: host + path structure + param keys
	sig := parsed.Host + ":" + parsed.Path

	if len(parsed.Query()) > 0 {
		keys := make([]string, 0, len(parsed.Query()))
		for k := range parsed.Query() {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		sig += "?" + strings.Join(keys, ",")
	}

	return sig
}

// SelectTopK selects top K URLs from each cluster.
func (s *SimilarityDetector) SelectTopK(clusters []URLCluster, topK int) []string {
	if topK <= 0 {
		topK = 1
	}

	selected := make([]string, 0, len(clusters)*topK)

	for _, cluster := range clusters {
		count := min(topK, len(cluster.Members))
		selected = append(selected, cluster.Members[:count]...)
	}

	s.logger.Info("selected top-K from clusters",
		"clusters", len(clusters),
		"top_k", topK,
		"selected", len(selected),
	)

	return selected
}

// Helper functions

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// CalculateCoverage returns percentage of URLs covered by clusters.
func CalculateCoverage(clusters []URLCluster, totalURLs int) float64 {
	if totalURLs == 0 {
		return 0.0
	}

	coveredURLs := 0
	for _, cluster := range clusters {
		coveredURLs += cluster.Count
	}

	return math.Min(1.0, float64(coveredURLs)/float64(totalURLs))
}
