package golinkfinderevo

import (
	"testing"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/platform/logx"
)

func TestGoLinkfinderEvo_FilterURLs(t *testing.T) {
	tests := []struct {
		name         string
		inputURLs    []*domain.Artifact
		expectedJS   int
		expectedHTML int
	}{
		{
			name: "mixed content types with alive URLs",
			inputURLs: []*domain.Artifact{
				createTestURL("https://example.com/app.js", 200, "application/javascript"),
				createTestURL("https://example.com/bundle.js", 200, "text/javascript"),
				createTestURL("https://example.com/index.html", 200, "text/html"),
				createTestURL("https://example.com/about.html", 200, "text/html; charset=utf-8"),
				createTestURL("https://example.com/api/data", 200, "application/json"), // Should be ignored
				createTestURL("https://example.com/error.html", 404, "text/html"),      // Should be ignored (404)
				createTestURL("https://example.com/broken.js", 500, "text/javascript"), // Should be ignored (500)
			},
			expectedJS:   2,
			expectedHTML: 2,
		},
		{
			name: "only JavaScript files",
			inputURLs: []*domain.Artifact{
				createTestURL("https://example.com/main.js", 200, "application/javascript"),
				createTestURL("https://example.com/vendor.js", 200, "text/javascript"),
				createTestURL("https://example.com/app.bundle.js", 200, "application/ecmascript"),
			},
			expectedJS:   3,
			expectedHTML: 0,
		},
		{
			name: "only HTML files",
			inputURLs: []*domain.Artifact{
				createTestURL("https://example.com/index.html", 200, "text/html"),
				createTestURL("https://example.com/about.htm", 200, "text/html"),
				createTestURL("https://example.com/contact.html", 200, "text/html; charset=utf-8"),
			},
			expectedJS:   0,
			expectedHTML: 3,
		},
		{
			name:         "empty input",
			inputURLs:    []*domain.Artifact{},
			expectedJS:   0,
			expectedHTML: 0,
		},
		{
			name: "non-URL artifacts",
			inputURLs: []*domain.Artifact{
				{Type: domain.ArtifactTypeSubdomain, Value: "sub.example.com"},
				{Type: domain.ArtifactTypeDomain, Value: "example.com"},
				{Type: domain.ArtifactTypeIP, Value: "1.2.3.4"},
			},
			expectedJS:   0,
			expectedHTML: 0,
		},
		{
			name: "URLs without metadata",
			inputURLs: []*domain.Artifact{
				{Type: domain.ArtifactTypeURL, Value: "https://example.com/app.js"},
				{Type: domain.ArtifactTypeURL, Value: "https://example.com/index.html"},
			},
			expectedJS:   0,
			expectedHTML: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logx.New()
			source := New(logger)
			candidates := source.filterURLsForCrawling(tt.inputURLs)

			jsCount := 0
			htmlCount := 0
			for _, c := range candidates {
				if c.Category == URLCategoryJavaScript {
					jsCount++
				} else if c.Category == URLCategoryHTML {
					htmlCount++
				}
			}

			if jsCount != tt.expectedJS {
				t.Errorf("expected %d JavaScript files, got %d", tt.expectedJS, jsCount)
			}

			if htmlCount != tt.expectedHTML {
				t.Errorf("expected %d HTML files, got %d", tt.expectedHTML, htmlCount)
			}
		})
	}
}

func TestGoLinkfinderEvo_ApplyLimits(t *testing.T) {
	tests := []struct {
		name           string
		maxScript      int
		maxHTML        int
		candidates     []URLCandidate
		expectedTotal  int
		expectedJS     int
		expectedHTML   int
	}{
		{
			name:      "within limits",
			maxScript: 50,
			maxHTML:   50,
			candidates: []URLCandidate{
				{URL: "https://example.com/1.js", Category: URLCategoryJavaScript},
				{URL: "https://example.com/2.js", Category: URLCategoryJavaScript},
				{URL: "https://example.com/1.html", Category: URLCategoryHTML},
				{URL: "https://example.com/2.html", Category: URLCategoryHTML},
			},
			expectedTotal: 4,
			expectedJS:    2,
			expectedHTML:  2,
		},
		{
			name:      "exceeding JS limit",
			maxScript: 2,
			maxHTML:   50,
			candidates: []URLCandidate{
				{URL: "https://example.com/1.js", Category: URLCategoryJavaScript},
				{URL: "https://example.com/2.js", Category: URLCategoryJavaScript},
				{URL: "https://example.com/3.js", Category: URLCategoryJavaScript},
				{URL: "https://example.com/4.js", Category: URLCategoryJavaScript},
				{URL: "https://example.com/1.html", Category: URLCategoryHTML},
			},
			expectedTotal: 3,
			expectedJS:    2,
			expectedHTML:  1,
		},
		{
			name:      "exceeding HTML limit",
			maxScript: 50,
			maxHTML:   2,
			candidates: []URLCandidate{
				{URL: "https://example.com/1.js", Category: URLCategoryJavaScript},
				{URL: "https://example.com/1.html", Category: URLCategoryHTML},
				{URL: "https://example.com/2.html", Category: URLCategoryHTML},
				{URL: "https://example.com/3.html", Category: URLCategoryHTML},
				{URL: "https://example.com/4.html", Category: URLCategoryHTML},
			},
			expectedTotal: 3,
			expectedJS:    1,
			expectedHTML:  2,
		},
		{
			name:      "exceeding both limits",
			maxScript: 1,
			maxHTML:   1,
			candidates: []URLCandidate{
				{URL: "https://example.com/1.js", Category: URLCategoryJavaScript},
				{URL: "https://example.com/2.js", Category: URLCategoryJavaScript},
				{URL: "https://example.com/1.html", Category: URLCategoryHTML},
				{URL: "https://example.com/2.html", Category: URLCategoryHTML},
			},
			expectedTotal: 2,
			expectedJS:    1,
			expectedHTML:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logx.New()
			source := New(logger)
			source.maxScriptFiles = tt.maxScript
			source.maxHTMLFiles = tt.maxHTML

			urls := source.applyURLLimits(tt.candidates)

			if len(urls) != tt.expectedTotal {
				t.Errorf("expected %d total URLs, got %d", tt.expectedTotal, len(urls))
			}

			// Verify counts
			jsCount := 0
			htmlCount := 0
			for _, url := range urls {
				for _, c := range tt.candidates {
					if c.URL == url {
						if c.Category == URLCategoryJavaScript {
							jsCount++
						} else if c.Category == URLCategoryHTML {
							htmlCount++
						}
						break
					}
				}
			}

			if jsCount != tt.expectedJS {
				t.Errorf("expected %d JS URLs, got %d", tt.expectedJS, jsCount)
			}

			if htmlCount != tt.expectedHTML {
				t.Errorf("expected %d HTML URLs, got %d", tt.expectedHTML, htmlCount)
			}
		})
	}
}

func TestProfiles_Validation(t *testing.T) {
	tests := []struct {
		name     string
		profile  ScanProfile
		expected bool
	}{
		{"quick profile valid", ProfileQuick, true},
		{"standard profile valid", ProfileStandard, true},
		{"deep profile valid", ProfileDeep, true},
		{"invalid profile", ScanProfile("invalid"), false},
		{"empty profile", ScanProfile(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateProfile(tt.profile)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestProfiles_GetProfile(t *testing.T) {
	// Test valid profiles
	quickCfg := GetProfile(ProfileQuick)
	if quickCfg.Workers != 10 {
		t.Errorf("expected quick profile workers=10, got %d", quickCfg.Workers)
	}

	standardCfg := GetProfile(ProfileStandard)
	if standardCfg.Workers != 20 {
		t.Errorf("expected standard profile workers=20, got %d", standardCfg.Workers)
	}

	deepCfg := GetProfile(ProfileDeep)
	if deepCfg.Workers != 30 {
		t.Errorf("expected deep profile workers=30, got %d", deepCfg.Workers)
	}

	// Test invalid profile (should return standard)
	invalidCfg := GetProfile(ScanProfile("invalid"))
	if invalidCfg.Workers != standardCfg.Workers {
		t.Errorf("expected fallback to standard profile, got workers=%d", invalidCfg.Workers)
	}
}

// Helper function to create test URL artifacts with metadata
func createTestURL(url string, statusCode int, contentType string) *domain.Artifact {
	artifact := domain.NewArtifact(domain.ArtifactTypeURL, url, "test")

	domainMeta := &metadata.DomainMetadata{
		HTTPStatus: statusCode,
	}

	artifact.TypedMetadata = domainMeta

	// Add content-type as tag for categorization
	if contentType != "" {
		artifact.AddTag("content_type:" + contentType)
	}

	return artifact
}
