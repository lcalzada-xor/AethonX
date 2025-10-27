package urlfilter

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"aethonx/internal/platform/logx"
)

// PatternExtractor detects recurring URL patterns and extracts templates.
type PatternExtractor struct {
	minOccurrences int // Minimum times pattern must appear to be valid
	logger         logx.Logger

	// Compiled patterns for dynamic segment detection
	patterns []dynamicPattern
}

// dynamicPattern represents a pattern for detecting dynamic URL segments.
type dynamicPattern struct {
	name    string
	regex   *regexp.Regexp
	example string
}

// NewPatternExtractor creates a new pattern extractor.
func NewPatternExtractor(minOccurrences int, logger logx.Logger) *PatternExtractor {
	if minOccurrences < 1 {
		minOccurrences = 3
	}

	return &PatternExtractor{
		minOccurrences: minOccurrences,
		logger:         logger.With("component", "pattern"),
		patterns: []dynamicPattern{
			{
				name:    "numeric_id",
				regex:   regexp.MustCompile(`^\d+$`),
				example: "12345",
			},
			{
				name:    "uuid",
				regex:   regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`),
				example: "550e8400-e29b-41d4-a716-446655440000",
			},
			{
				name:    "hash",
				regex:   regexp.MustCompile(`^[a-f0-9]{32,64}$`),
				example: "a1b2c3d4e5f6...",
			},
			{
				name:    "slug",
				regex:   regexp.MustCompile(`^[a-z0-9]+-[a-z0-9-]+$`),
				example: "my-product-name",
			},
			{
				name:    "date",
				regex:   regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`),
				example: "2024-12-31",
			},
			{
				name:    "timestamp",
				regex:   regexp.MustCompile(`^\d{10,13}$`),
				example: "1609459200",
			},
			{
				name:    "hex",
				regex:   regexp.MustCompile(`^[a-f0-9]{6,16}$`),
				example: "abc123def456",
			},
		},
	}
}

// URLPattern represents a detected URL pattern.
type URLPattern struct {
	Template   string   // Template with placeholders (e.g., "/api/{version}/{resource}/{id}")
	Examples   []string // Concrete URLs matching this pattern
	Frequency  int      // Number of URLs matching this pattern
	Confidence float64  // Confidence score (0.0-1.0)
	Metadata   map[string]interface{}
}

// ExtractPatterns finds recurring URL patterns in a dataset.
func (p *PatternExtractor) ExtractPatterns(urls []string) []URLPattern {
	if len(urls) == 0 {
		return nil
	}

	// Step 1: Convert URLs to templates
	templateMap := make(map[string]*patternInfo)

	for _, rawURL := range urls {
		template, err := p.extractTemplate(rawURL)
		if err != nil {
			p.logger.Debug("failed to extract template", "url", rawURL, "error", err.Error())
			continue
		}

		if info, exists := templateMap[template]; exists {
			info.examples = append(info.examples, rawURL)
			info.frequency++
		} else {
			templateMap[template] = &patternInfo{
				template:  template,
				examples:  []string{rawURL},
				frequency: 1,
			}
		}
	}

	// Step 2: Filter patterns by minimum occurrences
	patterns := make([]URLPattern, 0, len(templateMap))

	for _, info := range templateMap {
		if info.frequency < p.minOccurrences {
			continue
		}

		// Calculate confidence based on frequency and consistency
		confidence := p.calculateConfidence(info, len(urls))

		// Limit examples to avoid memory bloat
		examples := info.examples
		if len(examples) > 10 {
			examples = examples[:10]
		}

		patterns = append(patterns, URLPattern{
			Template:   info.template,
			Examples:   examples,
			Frequency:  info.frequency,
			Confidence: confidence,
			Metadata: map[string]interface{}{
				"total_urls": len(urls),
				"coverage":   float64(info.frequency) / float64(len(urls)),
			},
		})
	}

	// Step 3: Sort by frequency (descending)
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Frequency > patterns[j].Frequency
	})

	p.logger.Info("extracted URL patterns",
		"total_urls", len(urls),
		"patterns_found", len(patterns),
		"min_occurrences", p.minOccurrences,
	)

	return patterns
}

// patternInfo tracks information about a pattern during extraction.
type patternInfo struct {
	template  string
	examples  []string
	frequency int
}

// extractTemplate converts a URL to a template by replacing dynamic segments.
func (p *PatternExtractor) extractTemplate(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse error: %w", err)
	}

	// Extract path template
	pathTemplate := p.extractPathTemplate(parsed.Path)

	// Extract query template (parameter keys only, sorted)
	queryTemplate := p.extractQueryTemplate(parsed.Query())

	// Build template
	template := fmt.Sprintf("%s://%s%s", parsed.Scheme, parsed.Host, pathTemplate)
	if queryTemplate != "" {
		template += "?" + queryTemplate
	}

	return template, nil
}

// extractPathTemplate converts path to template by replacing dynamic segments.
func (p *PatternExtractor) extractPathTemplate(path string) string {
	if path == "" || path == "/" {
		return "/"
	}

	segments := strings.Split(strings.Trim(path, "/"), "/")
	templateSegments := make([]string, len(segments))

	for i, segment := range segments {
		if segment == "" {
			templateSegments[i] = ""
			continue
		}

		// Check if segment matches any dynamic pattern
		placeholder := p.detectDynamicSegment(segment)
		if placeholder != "" {
			templateSegments[i] = placeholder
		} else {
			templateSegments[i] = segment
		}
	}

	return "/" + strings.Join(templateSegments, "/")
}

// detectDynamicSegment checks if a path segment is dynamic and returns placeholder.
func (p *PatternExtractor) detectDynamicSegment(segment string) string {
	segmentLower := strings.ToLower(segment)

	for _, pattern := range p.patterns {
		if pattern.regex.MatchString(segmentLower) {
			return "{" + pattern.name + "}"
		}
	}

	// Check for mixed alphanumeric that's likely dynamic
	if len(segment) > 15 && p.isMixedAlphanumeric(segment) {
		return "{dynamic}"
	}

	return ""
}

// isMixedAlphanumeric checks if string has both letters and numbers.
func (p *PatternExtractor) isMixedAlphanumeric(s string) bool {
	hasLetter := false
	hasDigit := false

	for _, r := range s {
		if r >= '0' && r <= '9' {
			hasDigit = true
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			hasLetter = true
		}

		if hasLetter && hasDigit {
			return true
		}
	}

	return false
}

// extractQueryTemplate extracts query parameter template (keys only, sorted).
func (p *PatternExtractor) extractQueryTemplate(query url.Values) string {
	if len(query) == 0 {
		return ""
	}

	// Extract and sort parameter keys
	keys := make([]string, 0, len(query))
	for key := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Build template with empty values
	parts := make([]string, len(keys))
	for i, key := range keys {
		parts[i] = key + "="
	}

	return strings.Join(parts, "&")
}

// calculateConfidence calculates confidence score for a pattern.
func (p *PatternExtractor) calculateConfidence(info *patternInfo, totalURLs int) float64 {
	// Base confidence from frequency
	frequencyScore := float64(info.frequency) / float64(totalURLs)

	// Bonus for high frequency
	if info.frequency >= 10 {
		frequencyScore *= 1.2
	} else if info.frequency >= 5 {
		frequencyScore *= 1.1
	}

	// Cap at 1.0
	if frequencyScore > 1.0 {
		frequencyScore = 1.0
	}

	return frequencyScore
}

// GroupByPattern groups URLs by their patterns.
func (p *PatternExtractor) GroupByPattern(urls []string) map[string][]string {
	groups := make(map[string][]string)

	for _, rawURL := range urls {
		template, err := p.extractTemplate(rawURL)
		if err != nil {
			p.logger.Debug("failed to extract template", "url", rawURL, "error", err.Error())
			continue
		}

		groups[template] = append(groups[template], rawURL)
	}

	return groups
}

// SelectRepresentatives selects representative URLs from each pattern group.
// Returns at most maxPerPattern URLs for each pattern.
func (p *PatternExtractor) SelectRepresentatives(patterns []URLPattern, maxPerPattern int) []string {
	if maxPerPattern <= 0 {
		maxPerPattern = 1
	}

	representatives := make([]string, 0, len(patterns)*maxPerPattern)

	for _, pattern := range patterns {
		count := maxPerPattern
		if count > len(pattern.Examples) {
			count = len(pattern.Examples)
		}

		representatives = append(representatives, pattern.Examples[:count]...)
	}

	return representatives
}

// FilterByPattern filters URLs keeping only maxPerPattern URLs per pattern.
func (p *PatternExtractor) FilterByPattern(urls []string, maxPerPattern int) []string {
	groups := p.GroupByPattern(urls)

	filtered := make([]string, 0, len(urls))
	for _, groupURLs := range groups {
		count := maxPerPattern
		if count > len(groupURLs) {
			count = len(groupURLs)
		}
		filtered = append(filtered, groupURLs[:count]...)
	}

	p.logger.Info("filtered URLs by pattern",
		"input", len(urls),
		"output", len(filtered),
		"patterns", len(groups),
		"max_per_pattern", maxPerPattern,
	)

	return filtered
}
