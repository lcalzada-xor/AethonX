// internal/platform/validator/validator_test.go
package validator

import (
	"testing"

	"aethonx/internal/testutil"
)

func TestIsDomain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid domain", "example.com", true},
		{"valid subdomain", "test.example.com", true},
		{"valid multi-level", "api.test.example.com", true},
		{"empty string", "", false},
		{"too long", string(make([]byte, 300)), false},
		{"ip address", "192.168.1.1", false},
		{"invalid chars", "exam ple.com", false},
		{"starts with hyphen", "-example.com", false},
		{"ends with hyphen", "example-.com", false},
		{"single label", "localhost", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDomain(tt.input)
			testutil.AssertEqual(t, result, tt.expected, "domain validation")
		})
	}
}

func TestIsSubdomain(t *testing.T) {
	tests := []struct {
		name       string
		subdomain  string
		baseDomain string
		expected   bool
	}{
		{"valid subdomain", "test.example.com", "example.com", true},
		{"multi-level subdomain", "api.test.example.com", "example.com", true},
		{"same domain", "example.com", "example.com", false},
		{"not a subdomain", "other.com", "example.com", false},
		{"partial match", "example.com.test", "example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSubdomain(tt.subdomain, tt.baseDomain)
			testutil.AssertEqual(t, result, tt.expected, "subdomain check")
		})
	}
}

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"lowercase", "EXAMPLE.COM", "example.com"},
		{"remove trailing dot", "example.com.", "example.com"},
		{"remove www prefix", "www.example.com", "example.com"},
		{"all together", "WWW.EXAMPLE.COM.", "example.com"},
		{"trim spaces", "  example.com  ", "example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeDomain(tt.input)
			testutil.AssertEqual(t, result, tt.expected, "normalized domain")
		})
	}
}

func TestIsEmail(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid email", "test@example.com", true},
		{"with plus", "test+tag@example.com", true},
		{"with hyphen", "test-user@example.com", true},
		{"empty string", "", false},
		{"no at sign", "testexample.com", false},
		{"no domain", "test@", false},
		{"no user", "@example.com", false},
		{"multiple at", "test@@example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEmail(tt.input)
			testutil.AssertEqual(t, result, tt.expected, "email validation")
		})
	}
}

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"lowercase", "TEST@EXAMPLE.COM", "test@example.com"},
		{"trim spaces", "  test@example.com  ", "test@example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeEmail(tt.input)
			testutil.AssertEqual(t, result, tt.expected, "normalized email")
		})
	}
}

func TestIsIP(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid ipv4", "192.168.1.1", true},
		{"valid ipv6", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", true},
		{"invalid ip", "256.1.1.1", false},
		{"domain", "example.com", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsIP(tt.input)
			testutil.AssertEqual(t, result, tt.expected, "ip validation")
		})
	}
}

func TestIsIPv4(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid ipv4", "192.168.1.1", true},
		{"ipv6", "2001:0db8:85a3::8a2e:0370:7334", false},
		{"invalid", "256.1.1.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsIPv4(tt.input)
			testutil.AssertEqual(t, result, tt.expected, "ipv4 validation")
		})
	}
}

func TestIsPort(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid port", "80", true},
		{"max port", "65535", true},
		{"min port", "1", true},
		{"zero", "0", false},
		{"too high", "65536", false},
		{"negative", "-1", false},
		{"not a number", "abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPort(tt.input)
			testutil.AssertEqual(t, result, tt.expected, "port validation")
		})
	}
}

func TestIsURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid http", "http://example.com", true},
		{"valid https", "https://example.com", true},
		{"with path", "https://example.com/path", true},
		{"with query", "https://example.com?query=1", true},
		{"no scheme", "example.com", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsURL(tt.input)
			testutil.AssertEqual(t, result, tt.expected, "url validation")
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"lowercase scheme", "HTTP://EXAMPLE.COM", "http://example.com"},
		{"lowercase host", "http://EXAMPLE.COM", "http://example.com"},
		{"remove trailing slash", "http://example.com/", "http://example.com"},
		{"keep path", "http://example.com/path", "http://example.com/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeURL(tt.input)
			testutil.AssertEqual(t, result, tt.expected, "normalized url")
		})
	}
}

func TestIsCertSerial(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid hex", "1a2b3c4d", true},
		{"uppercase hex", "1A2B3C4D", true},
		{"with colons", "1a:2b:3c:4d", true},
		{"with spaces", "1a 2b 3c", true}, // Spaces are allowed as separators
		{"empty", "", false},
		{"invalid chars", "xyz123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCertSerial(tt.input)
			testutil.AssertEqual(t, result, tt.expected, "cert serial validation")
		})
	}
}

func TestNormalizeCertSerial(t *testing.T) {
	result := NormalizeCertSerial("  1A2B3C4D  ")
	testutil.AssertEqual(t, result, "1a2b3c4d", "normalized cert serial")
}

func TestIsHash(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"md5", "d41d8cd98f00b204e9800998ecf8427e", true},
		{"sha1", "da39a3ee5e6b4b0d3255bfef95601890afd80709", true},
		{"sha256", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", true},
		{"invalid length", "abc123", false},
		{"non-hex", "zzz39a3ee5e6b4b0d3255bfef95601890afd80709", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsHash(tt.input)
			testutil.AssertEqual(t, result, tt.expected, "hash validation")
		})
	}
}

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty string", "", true},
		{"spaces only", "   ", true},
		{"has content", "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEmpty(tt.input)
			testutil.AssertEqual(t, result, tt.expected, "empty check")
		})
	}
}

func TestMaxLength(t *testing.T) {
	testutil.AssertTrue(t, MaxLength("test", 10), "should pass max length")
	testutil.AssertTrue(t, MaxLength("test", 4), "should pass equal length")
	testutil.AssertTrue(t, !MaxLength("test", 3), "should fail max length")
}

func TestMinLength(t *testing.T) {
	testutil.AssertTrue(t, MinLength("test", 2), "should pass min length")
	testutil.AssertTrue(t, MinLength("test", 4), "should pass equal length")
	testutil.AssertTrue(t, !MinLength("test", 5), "should fail min length")
}
