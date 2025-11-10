package golinkfinderevo

import (
	"testing"

	"aethonx/internal/platform/logx"
)

func TestParser_IsValidURL(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "test")

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		// Valid URLs
		{"valid https URL", "https://real-site.com/path", true},
		{"valid http URL", "http://api.real-site.com/api", true},
		{"valid subdomain", "https://sub.real-site.com/endpoint", true},
		{"valid with port", "https://real-site.com:8080/api", true},
		{"valid with query", "https://real-site.com/api?key=value", true},
		{"valid with fragment", "https://real-site.com/page#section", true},

		// Invalid URLs - single-char domains
		{"single char domain a", "https://a", false},
		{"single char domain x", "https://x", false},
		{"single char with path", "https://a/c%20d?a=1&c=3", false},
		{"single char with fragment", "https://a#б", false},
		{"two char domain", "https://ab", false},

		// Invalid URLs - no TLD (except localhost which is allowed)
		{"username in URL", "https://a@b", false},

		// Invalid URLs - special schemes
		{"about:blank", "https://about:blank", false},
		{"data scheme", "data:text/html,<html></html>", false},
		{"javascript scheme", "javascript:alert(1)", false},
		{"file scheme", "file:///etc/passwd", false},

		// Invalid URLs - test/placeholder domains
		{"example.com", "http://example.com", false},
		{"example.org", "https://example.org/path", false},
		{"test.com", "http://test.com", false},
		{"dummy.com", "https://dummy.com", false},
		{"placeholder.com", "http://placeholder.com/api", false},

		// Valid URLs - localhost and local IPs (important for local testing)
		{"localhost", "http://localhost:8080", true},
		{"localhost with path", "http://localhost/api", true},
		{"127.0.0.1", "http://127.0.0.1", true},
		{"127.0.0.1 with port", "http://127.0.0.1:3000", true},
		{"0.0.0.0", "http://0.0.0.0:8080", true},

		// Edge cases
		{"empty string", "", false},
		{"invalid URL", "not-a-url", false},
		{"missing scheme", "example.com/path", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.isValidURL(tt.url)
			if result != tt.expected {
				t.Errorf("isValidURL(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestParser_NormalizeEndpoint_WithValidation(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "test")

	tests := []struct {
		name     string
		baseURL  string
		endpoint string
		expected string // empty string means should be filtered out
	}{
		// Valid absolute URLs
		{
			name:     "valid absolute URL",
			baseURL:  "https://real-site.com/app.js",
			endpoint: "https://api.real-site.com/users",
			expected: "https://api.real-site.com/users",
		},
		{
			name:     "valid absolute URL with path",
			baseURL:  "https://real-site.com/",
			endpoint: "https://sub.real-site.com/api/v1",
			expected: "https://sub.real-site.com/api/v1",
		},

		// Invalid absolute URLs (should be filtered)
		{
			name:     "single-char domain should be filtered",
			baseURL:  "https://real-site.com/polyfill.js",
			endpoint: "https://a",
			expected: "",
		},
		{
			name:     "single-char with path should be filtered",
			baseURL:  "https://real-site.com/polyfill.js",
			endpoint: "https://a/c%20d?a=1&c=3",
			expected: "",
		},
		{
			name:     "about:blank should be filtered",
			baseURL:  "https://real-site.com/popup.js",
			endpoint: "https://about:blank",
			expected: "",
		},
		{
			name:     "example.com should be filtered",
			baseURL:  "https://real-site.com/app.js",
			endpoint: "http://example.com",
			expected: "",
		},
		{
			name:     "test.com should be filtered",
			baseURL:  "https://real-site.com/app.js",
			endpoint: "https://test.com/api",
			expected: "",
		},

		// Relative URLs (should work normally)
		{
			name:     "absolute path",
			baseURL:  "https://real-site.com/js/app.js",
			endpoint: "/api/users",
			expected: "https://real-site.com/api/users",
		},
		{
			name:     "relative path",
			baseURL:  "https://real-site.com/js/app.js",
			endpoint: "config.json",
			expected: "https://real-site.com/js/config.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.normalizeEndpoint(tt.baseURL, tt.endpoint)
			if result != tt.expected {
				t.Errorf("normalizeEndpoint(%q, %q) = %q, expected %q",
					tt.baseURL, tt.endpoint, result, tt.expected)
			}
		})
	}
}

func TestGFParser_LooksLikeEmail(t *testing.T) {
	logger := logx.New()
	gfParser := &GFParser{logger: logger}

	tests := []struct {
		input    string
		expected bool
		reason   string
	}{
		// Valid emails
		{"user@example.com", true, "valid basic email"},
		{"test.user@domain.co.uk", true, "valid email with subdomain TLD"},
		{"abuse@nominalia.com", true, "real email from RDAP"},
		{"abuse@interdominios.com", true, "real email from RDAP"},
		{"contact@site.org", true, "valid email with .org"},
		{"admin@localhost.local", true, "valid local domain email"},
		{"name+tag@example.com", true, "email with plus addressing"},
		{"user_name@example.com", true, "email with underscore"},
		{"first.last@example.com", true, "email with dot in local part"},

		// Invalid: retina display images (@2x, @3x, @4x)
		{"logo@2x.png", false, "retina display @2x image"},
		{"icon@3x.jpg", false, "retina display @3x image"},
		{"banner@4x.webp", false, "retina display @4x image"},
		{"ayuntamiento_valladolid_1_2_0@2x.png", false, "complex filename @2x.png"},
		{"embajada-india_1_1@2x.png", false, "real false positive from scan"},
		{"icc_small_0@2x.png", false, "real false positive from scan"},
		{"universidad-de-valladolid_1_0@2x.png", false, "real false positive from scan"},
		{"header@2x.jpeg", false, "retina @2x JPEG"},
		{"footer@3x.svg", false, "retina @3x SVG"},

		// Invalid: other static file patterns
		{"file@name.svg", false, "SVG file"},
		{"image@path.gif", false, "GIF file"},
		{"photo@album.bmp", false, "BMP file"},
		{"graphic@design.tiff", false, "TIFF file"},
		{"avatar@user.webp", false, "WebP file"},
		{"icon@app.ico", false, "ICO file"},

		// Invalid: malformed email addresses
		{"notanemail", false, "no @ symbol"},
		{"@nodomain.com", false, "no local part"},
		{"noat.com", false, "missing @ symbol"},
		{"double@@at.com", false, "double @ symbol"},
		{"user@", false, "no domain part"},
		{"@domain.com", false, "empty local part"},
		{"user@d", false, "domain too short"},
		{"user@domain", false, "domain without TLD"},
		{"a@bc", false, "domain part too short"},
		{"user@.com", false, "domain starts with dot"},
		{"user@domain.", false, "domain ends with dot"},

		// Edge cases
		{"", false, "empty string"},
		{"@", false, "only @ symbol"},
		{"a@b.c", true, "minimal valid email"},
		{"user@sub.domain.com", true, "subdomain email"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := gfParser.looksLikeEmail(tt.input)
			if result != tt.expected {
				t.Errorf("looksLikeEmail(%q) = %v, want %v (%s)",
					tt.input, result, tt.expected, tt.reason)
			}
		})
	}
}

