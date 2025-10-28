package urlfilter

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"aethonx/internal/platform/logx"
)

// URLNormalizer normalizes URLs to canonical forms for deduplication.
type URLNormalizer struct {
	strategy NormalizationStrategy
	logger   logx.Logger

	// Compiled regex patterns for dynamic segments
	numericIDPattern  *regexp.Regexp
	uuidPattern       *regexp.Regexp
	hashPattern       *regexp.Regexp
	slugPattern       *regexp.Regexp
	datePattern       *regexp.Regexp
	timestampPattern  *regexp.Regexp

	// Ignored query parameters (tracking, analytics, etc.)
	ignoredParams map[string]bool
}

// NewURLNormalizer creates a new URL normalizer.
func NewURLNormalizer(strategy NormalizationStrategy, logger logx.Logger) *URLNormalizer {
	return &URLNormalizer{
		strategy: strategy,
		logger:   logger.With("component", "normalizer"),

		// Dynamic segment patterns
		numericIDPattern:  regexp.MustCompile(`^\d+$`),
		uuidPattern:       regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`),
		hashPattern:       regexp.MustCompile(`^[a-f0-9]{32,64}$`),
		slugPattern:       regexp.MustCompile(`^[a-z0-9]+-[a-z0-9-]+$`),
		datePattern:       regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`),
		timestampPattern:  regexp.MustCompile(`^\d{10,13}$`),

		// Tracking/analytics parameters to ignore
		ignoredParams: map[string]bool{
			// Google Analytics
			"utm_source": true, "utm_medium": true, "utm_campaign": true,
			"utm_term": true, "utm_content": true, "gclid": true,
			"gclsrc": true, "_ga": true, "_gid": true,

			// Facebook
			"fbclid": true, "fb_action_ids": true, "fb_action_types": true,
			"fb_source": true, "fb_ref": true,

			// Session/tracking
			"sessionid": true, "session_id": true, "sid": true,
			"phpsessid": true, "jsessionid": true, "aspsessionid": true,
			"csrf_token": true, "csrf": true, "_csrf": true,

			// Timestamps/cache busters
			"_": true, "timestamp": true, "ts": true, "t": true,
			"nocache": true, "cache": true, "v": true, "ver": true,

			// Misc tracking
			"ref": true, "referer": true, "referrer": true,
			"source": true, "src": true,
		},
	}
}

// NormalizeResult contains normalized URL and its signature.
type NormalizeResult struct {
	Original  string // Original URL
	Canonical string // Normalized canonical form
	Signature string // Unique signature for deduplication
	Metadata  NormalizationMetadata
}

// NormalizationMetadata provides details about normalization.
type NormalizationMetadata struct {
	Strategy           string            // Strategy used
	ParamsRemoved      []string          // Tracking params removed
	DynamicSegments    map[string]string // Dynamic segments replaced
	ExtensionRemoved   bool              // File extension removed
	CaseNormalized     bool              // Lowercased
	TrailingSlashAdded bool              // Trailing slash added
}

// Normalize normalizes a URL according to the configured strategy.
func (n *URLNormalizer) Normalize(rawURL string) (*NormalizeResult, error) {
	// Parse URL
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	result := &NormalizeResult{
		Original: rawURL,
		Metadata: NormalizationMetadata{
			Strategy:        n.strategy.String(),
			DynamicSegments: make(map[string]string),
		},
	}

	// Apply normalization based on strategy
	switch n.strategy {
	case NormBasic:
		n.applyBasicNormalization(parsed, result)
	case NormStructural:
		n.applyBasicNormalization(parsed, result)
		n.applyStructuralNormalization(parsed, result)
	case NormParametric:
		n.applyBasicNormalization(parsed, result)
		n.applyParametricNormalization(parsed, result)
	case NormExtensionless:
		n.applyBasicNormalization(parsed, result)
		n.applyExtensionlessNormalization(parsed, result)
	case NormAggressive:
		n.applyBasicNormalization(parsed, result)
		n.applyStructuralNormalization(parsed, result)
		n.applyParametricNormalization(parsed, result)
		n.applyExtensionlessNormalization(parsed, result)
	}

	// Build canonical URL
	result.Canonical = parsed.String()

	// Generate signature (for deduplication)
	result.Signature = n.generateSignature(parsed)

	return result, nil
}

// applyBasicNormalization applies basic normalization: lowercase, sort params, trailing slash.
func (n *URLNormalizer) applyBasicNormalization(parsed *url.URL, result *NormalizeResult) {
	// Lowercase scheme and host
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	result.Metadata.CaseNormalized = true

	// Remove default ports
	parsed.Host = strings.TrimSuffix(parsed.Host, ":80")
	parsed.Host = strings.TrimSuffix(parsed.Host, ":443")

	// Remove fragment
	parsed.Fragment = ""

	// Normalize path (remove double slashes, clean)
	// IMPORTANT: Use path.Clean() not filepath.Clean() to avoid OS-specific separators
	if parsed.Path != "" {
		parsed.Path = path.Clean(parsed.Path)
		// Add trailing slash for directories (paths without extension)
		if !strings.Contains(filepath.Base(parsed.Path), ".") && !strings.HasSuffix(parsed.Path, "/") {
			parsed.Path += "/"
			result.Metadata.TrailingSlashAdded = true
		}
	}

	// Sort query parameters for consistency
	if len(parsed.Query()) > 0 {
		query := parsed.Query()
		keys := make([]string, 0, len(query))
		for k := range query {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		newQuery := url.Values{}
		for _, k := range keys {
			newQuery[k] = query[k]
		}
		parsed.RawQuery = newQuery.Encode()
	}
}

// applyStructuralNormalization replaces dynamic segments with placeholders.
func (n *URLNormalizer) applyStructuralNormalization(parsed *url.URL, result *NormalizeResult) {
	if parsed.Path == "" || parsed.Path == "/" {
		return
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	modified := false

	for i, segment := range segments {
		if segment == "" {
			continue
		}

		placeholder := n.detectDynamicSegment(segment)
		if placeholder != "" {
			result.Metadata.DynamicSegments[segment] = placeholder
			segments[i] = placeholder
			modified = true
		}
	}

	if modified {
		parsed.Path = "/" + strings.Join(segments, "/")
		if result.Metadata.TrailingSlashAdded {
			parsed.Path += "/"
		}
	}
}

// detectDynamicSegment detects if segment is dynamic and returns placeholder.
func (n *URLNormalizer) detectDynamicSegment(segment string) string {
	// Numeric ID: 12345
	if n.numericIDPattern.MatchString(segment) {
		return "{id}"
	}

	// UUID: 550e8400-e29b-41d4-a716-446655440000
	if n.uuidPattern.MatchString(strings.ToLower(segment)) {
		return "{uuid}"
	}

	// Hash: a1b2c3d4e5f6...
	if n.hashPattern.MatchString(strings.ToLower(segment)) {
		return "{hash}"
	}

	// Date: 2024-12-31
	if n.datePattern.MatchString(segment) {
		return "{date}"
	}

	// Timestamp: 1609459200
	if n.timestampPattern.MatchString(segment) {
		return "{timestamp}"
	}

	// Slug: my-product-name
	if n.slugPattern.MatchString(strings.ToLower(segment)) && len(segment) > 10 {
		return "{slug}"
	}

	return ""
}

// applyParametricNormalization removes parameter values and tracking params.
func (n *URLNormalizer) applyParametricNormalization(parsed *url.URL, result *NormalizeResult) {
	query := parsed.Query()
	if len(query) == 0 {
		return
	}

	newQuery := url.Values{}
	for key := range query {
		// Remove tracking parameters
		if n.ignoredParams[strings.ToLower(key)] {
			result.Metadata.ParamsRemoved = append(result.Metadata.ParamsRemoved, key)
			continue
		}

		// Keep parameter key but remove value
		newQuery[key] = []string{""}
	}

	if len(newQuery) > 0 {
		// Sort keys for consistency
		keys := make([]string, 0, len(newQuery))
		for k := range newQuery {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// Build query string manually (with empty values)
		queryParts := make([]string, 0, len(keys))
		for _, k := range keys {
			queryParts = append(queryParts, url.QueryEscape(k)+"=")
		}
		parsed.RawQuery = strings.Join(queryParts, "&")
	} else {
		parsed.RawQuery = ""
	}
}

// applyExtensionlessNormalization replaces file extensions with placeholder.
func (n *URLNormalizer) applyExtensionlessNormalization(parsed *url.URL, result *NormalizeResult) {
	if parsed.Path == "" || parsed.Path == "/" {
		return
	}

	base := filepath.Base(parsed.Path)
	ext := filepath.Ext(base)

	// Only normalize common asset extensions
	if ext != "" && n.isAssetExtension(ext) {
		nameWithoutExt := strings.TrimSuffix(base, ext)
		dir := filepath.Dir(parsed.Path)

		// Replace extension with placeholder
		if dir == "." {
			parsed.Path = nameWithoutExt + ".{ext}"
		} else {
			parsed.Path = filepath.Join(dir, nameWithoutExt+".{ext}")
		}

		result.Metadata.ExtensionRemoved = true
	}
}

// isAssetExtension checks if extension is a common asset type.
func (n *URLNormalizer) isAssetExtension(ext string) bool {
	ext = strings.ToLower(ext)
	assetExtensions := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".svg": true,
		".webp": true, ".ico": true, ".bmp": true,
		".css": true, ".js": true, ".json": true, ".xml": true,
		".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
		".mp4": true, ".webm": true, ".ogg": true, ".mp3": true,
		".pdf": true, ".zip": true, ".tar": true, ".gz": true,
	}
	return assetExtensions[ext]
}

// generateSignature generates a unique signature for deduplication.
func (n *URLNormalizer) generateSignature(parsed *url.URL) string {
	// Signature format: scheme://host/path?params
	// This allows exact duplicate detection after normalization
	return parsed.String()
}

// NormalizeBatch normalizes multiple URLs concurrently.
func (n *URLNormalizer) NormalizeBatch(urls []string) []*NormalizeResult {
	results := make([]*NormalizeResult, 0, len(urls))

	for _, rawURL := range urls {
		result, err := n.Normalize(rawURL)
		if err != nil {
			n.logger.Debug("failed to normalize URL", "url", rawURL, "error", err.Error())
			continue
		}
		results = append(results, result)
	}

	return results
}
