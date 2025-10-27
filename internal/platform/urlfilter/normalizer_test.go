package urlfilter

import (
	"testing"

	"aethonx/internal/platform/logx"
)

func TestURLNormalizer_BasicNormalization(t *testing.T) {
	logger := logx.New()
	normalizer := NewURLNormalizer(NormBasic, logger)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "lowercase scheme and host",
			input:    "HTTPS://EXAMPLE.COM/path",
			expected: "https://example.com/path/",
		},
		{
			name:     "remove default port 80",
			input:    "http://example.com:80/path",
			expected: "http://example.com/path/",
		},
		{
			name:     "remove default port 443",
			input:    "https://example.com:443/path",
			expected: "https://example.com/path/",
		},
		{
			name:     "remove fragment",
			input:    "https://example.com/page#section",
			expected: "https://example.com/page/",
		},
		{
			name:     "sort query parameters",
			input:    "https://example.com/path?z=3&a=1&m=2",
			expected: "https://example.com/path/?a=1&m=2&z=3",
		},
		{
			name:     "add trailing slash for directories",
			input:    "https://example.com/api",
			expected: "https://example.com/api/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := normalizer.Normalize(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Canonical != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result.Canonical)
			}
		})
	}
}

func TestURLNormalizer_StructuralNormalization(t *testing.T) {
	logger := logx.New()
	normalizer := NewURLNormalizer(NormStructural, logger)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "replace numeric ID",
			input:    "https://example.com/api/users/12345",
			expected: "https://example.com/api/users/%7Bid%7D/", // {id} URL-encoded
		},
		{
			name:     "replace UUID",
			input:    "https://example.com/api/items/550e8400-e29b-41d4-a716-446655440000",
			expected: "https://example.com/api/items/%7Buuid%7D/",
		},
		{
			name:     "replace hash",
			input:    "https://example.com/files/a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4",
			expected: "https://example.com/files/%7Bhash%7D/",
		},
		{
			name:     "replace date",
			input:    "https://example.com/archive/2024-12-31",
			expected: "https://example.com/archive/%7Bdate%7D/",
		},
		{
			name:     "replace timestamp",
			input:    "https://example.com/posts/1609459200",
			expected: "https://example.com/posts/%7Btimestamp%7D/",
		},
		{
			name:     "replace slug",
			input:    "https://example.com/blog/my-awesome-blog-post-title",
			expected: "https://example.com/blog/%7Bslug%7D/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := normalizer.Normalize(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Canonical != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result.Canonical)
			}
		})
	}
}

func TestURLNormalizer_ParametricNormalization(t *testing.T) {
	logger := logx.New()
	normalizer := NewURLNormalizer(NormParametric, logger)

	tests := []struct {
		name           string
		input          string
		expectedParams string
		removedParams  []string
	}{
		{
			name:           "remove tracking parameters",
			input:          "https://example.com/page?id=123&utm_source=google&utm_medium=email",
			expectedParams: "id=",
			removedParams:  []string{"utm_source", "utm_medium"},
		},
		{
			name:           "remove session parameters",
			input:          "https://example.com/page?page=2&sessionid=abc123",
			expectedParams: "page=",
			removedParams:  []string{"sessionid"},
		},
		{
			name:           "remove facebook tracking",
			input:          "https://example.com/page?id=1&fbclid=IwAR123",
			expectedParams: "id=",
			removedParams:  []string{"fbclid"},
		},
		{
			name:           "keep functional parameters",
			input:          "https://example.com/search?q=test&page=1&limit=10",
			expectedParams: "limit=&page=&q=",
			removedParams:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := normalizer.Normalize(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check if expected params are in canonical URL
			if !contains(result.Canonical, tt.expectedParams) {
				t.Errorf("expected params %q not found in %q", tt.expectedParams, result.Canonical)
			}

			// Check removed params
			if len(tt.removedParams) > 0 {
				for _, removed := range tt.removedParams {
					found := false
					for _, p := range result.Metadata.ParamsRemoved {
						if p == removed {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected param %q to be marked as removed", removed)
					}
				}
			}
		})
	}
}

func TestURLNormalizer_ExtensionlessNormalization(t *testing.T) {
	logger := logx.New()
	normalizer := NewURLNormalizer(NormExtensionless, logger)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normalize jpg extension",
			input:    "https://example.com/images/photo.jpg",
			expected: "https://example.com/images/photo.%7Bext%7D", // {ext} URL-encoded
		},
		{
			name:     "normalize png extension",
			input:    "https://example.com/images/logo.png",
			expected: "https://example.com/images/logo.%7Bext%7D",
		},
		{
			name:     "normalize css extension",
			input:    "https://example.com/static/style.css",
			expected: "https://example.com/static/style.%7Bext%7D",
		},
		{
			name:     "normalize js extension",
			input:    "https://example.com/js/app.js",
			expected: "https://example.com/js/app.%7Bext%7D",
		},
		{
			name:     "don't normalize html",
			input:    "https://example.com/page.html",
			expected: "https://example.com/page.html",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := normalizer.Normalize(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Canonical != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result.Canonical)
			}

			if result.Metadata.ExtensionRemoved && tt.expected == tt.input {
				t.Error("metadata indicates extension removed but URLs are identical")
			}
		})
	}
}

func TestURLNormalizer_AggressiveNormalization(t *testing.T) {
	logger := logx.New()
	normalizer := NewURLNormalizer(NormAggressive, logger)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "full normalization",
			input:    "HTTPS://EXAMPLE.COM/api/users/12345?utm_source=google&page=2",
			expected: "https://example.com/api/users/%7Bid%7D/?page=",
		},
		{
			name:     "complex URL with everything",
			input:    "HTTPS://EXAMPLE.COM:443/images/photo123.jpg?v=2&utm_campaign=test#section",
			expected: "https://example.com/images/photo123.%7Bext%7D?v=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := normalizer.Normalize(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Canonical != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result.Canonical)
			}

			// Verify metadata is populated
			if result.Metadata.Strategy != "aggressive" {
				t.Errorf("expected strategy 'aggressive', got %q", result.Metadata.Strategy)
			}
		})
	}
}

func TestURLNormalizer_InvalidURLs(t *testing.T) {
	logger := logx.New()
	normalizer := NewURLNormalizer(NormBasic, logger)

	invalidURLs := []string{
		"://missing-scheme",
	}

	for _, input := range invalidURLs {
		t.Run(input, func(t *testing.T) {
			_, err := normalizer.Normalize(input)
			if err == nil {
				t.Error("expected error for invalid URL")
			}
		})
	}
}

func TestURLNormalizer_NormalizeBatch(t *testing.T) {
	logger := logx.New()
	normalizer := NewURLNormalizer(NormBasic, logger)

	urls := []string{
		"https://example.com/page1",
		"HTTPS://EXAMPLE.COM/page2",
		"https://example.com/page3?z=1&a=2",
		"://invalid",
	}

	results := normalizer.NormalizeBatch(urls)

	// Should get 3 valid results (1 invalid skipped)
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// Verify results
	for _, result := range results {
		if result.Canonical == "" {
			t.Error("got empty canonical URL")
		}
		if result.Signature == "" {
			t.Error("got empty signature")
		}
	}
}

func TestURLNormalizer_SignatureGeneration(t *testing.T) {
	logger := logx.New()
	normalizer := NewURLNormalizer(NormBasic, logger)

	// Same URLs should have same signature
	url1 := "https://example.com/page?b=2&a=1"
	url2 := "https://example.com/page?a=1&b=2"

	result1, _ := normalizer.Normalize(url1)
	result2, _ := normalizer.Normalize(url2)

	if result1.Signature != result2.Signature {
		t.Errorf("expected same signature for equivalent URLs:\n  %q\n  %q",
			result1.Signature, result2.Signature)
	}

	// Different URLs should have different signatures
	url3 := "https://example.com/different"
	result3, _ := normalizer.Normalize(url3)

	if result1.Signature == result3.Signature {
		t.Error("expected different signatures for different URLs")
	}
}

func BenchmarkURLNormalizer_Basic(b *testing.B) {
	logger := logx.New()
	normalizer := NewURLNormalizer(NormBasic, logger)
	url := "HTTPS://EXAMPLE.COM/page?z=3&a=1&m=2#section"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = normalizer.Normalize(url)
	}
}

func BenchmarkURLNormalizer_Aggressive(b *testing.B) {
	logger := logx.New()
	normalizer := NewURLNormalizer(NormAggressive, logger)
	url := "HTTPS://EXAMPLE.COM/api/users/12345?utm_source=google&page=2#section"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = normalizer.Normalize(url)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
