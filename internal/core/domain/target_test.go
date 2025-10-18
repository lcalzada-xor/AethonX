// internal/core/domain/target_test.go
package domain

import (
	"testing"

	"aethonx/internal/testutil"
)

func TestNewTarget(t *testing.T) {
	target := NewTarget("example.com", ScanModePassive)

	testutil.AssertNotNil(t, target, "target should not be nil")
	testutil.AssertEqual(t, target.Root, "example.com", "root domain")
	testutil.AssertEqual(t, target.Mode, ScanModePassive, "scan mode")
	testutil.AssertTrue(t, target.Scope.IncludeSubdomains, "should include subdomains by default")
	testutil.AssertEqual(t, target.Scope.MaxDepth, 0, "max depth default")
}

func TestTarget_Validate(t *testing.T) {
	tests := []struct {
		name        string
		root        string
		shouldError bool
	}{
		{
			name:        "valid domain",
			root:        "example.com",
			shouldError: false,
		},
		{
			name:        "valid subdomain",
			root:        "test.example.com",
			shouldError: false,
		},
		{
			name:        "valid domain with hyphen",
			root:        "my-domain.com",
			shouldError: false,
		},
		{
			name:        "empty domain",
			root:        "",
			shouldError: true,
		},
		{
			name:        "IP address should fail",
			root:        "192.168.1.1",
			shouldError: true,
		},
		{
			name:        "IPv6 address should fail",
			root:        "2001:db8::1",
			shouldError: true,
		},
		{
			name:        "invalid characters",
			root:        "invalid_domain.com",
			shouldError: true,
		},
		{
			name:        "domain starting with hyphen",
			root:        "-invalid.com",
			shouldError: true,
		},
		{
			name:        "domain ending with hyphen",
			root:        "invalid-.com",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := NewTarget(tt.root, ScanModePassive)
			err := target.Validate()

			if tt.shouldError {
				testutil.AssertError(t, err, "validation should fail")
			} else {
				testutil.AssertNoError(t, err, "validation should succeed")
			}
		})
	}
}

func TestTarget_IsInScope(t *testing.T) {
	target := NewTarget("example.com", ScanModePassive)
	target.Scope.IncludeSubdomains = true
	target.Scope.ExcludeDomains = []string{"exclude.example.com", "internal.example.com"}

	tests := []struct {
		name     string
		domain   string
		inScope  bool
	}{
		{
			name:    "root domain in scope",
			domain:  "example.com",
			inScope: true,
		},
		{
			name:    "subdomain in scope",
			domain:  "test.example.com",
			inScope: true,
		},
		{
			name:    "deep subdomain in scope",
			domain:  "api.test.example.com",
			inScope: true,
		},
		{
			name:    "excluded domain",
			domain:  "exclude.example.com",
			inScope: false,
		},
		{
			name:    "wildcard exclusion",
			domain:  "api.internal.example.com",
			inScope: false,
		},
		{
			name:    "different domain",
			domain:  "other.com",
			inScope: false,
		},
		{
			name:    "different TLD",
			domain:  "example.org",
			inScope: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := target.IsInScope(tt.domain)
			if tt.inScope {
				testutil.AssertTrue(t, result, "should be in scope")
			} else {
				testutil.AssertFalse(t, result, "should not be in scope")
			}
		})
	}
}

func TestTarget_IsInScope_NoSubdomains(t *testing.T) {
	target := NewTarget("example.com", ScanModePassive)
	target.Scope.IncludeSubdomains = false

	tests := []struct {
		name    string
		domain  string
		inScope bool
	}{
		{
			name:    "root domain in scope",
			domain:  "example.com",
			inScope: true,
		},
		{
			name:    "subdomain NOT in scope when disabled",
			domain:  "test.example.com",
			inScope: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := target.IsInScope(tt.domain)
			if tt.inScope {
				testutil.AssertTrue(t, result, "should be in scope")
			} else {
				testutil.AssertFalse(t, result, "should not be in scope")
			}
		})
	}
}

func TestTarget_MaxDepth(t *testing.T) {
	target := NewTarget("example.com", ScanModePassive)
	target.Scope.IncludeSubdomains = true
	target.Scope.MaxDepth = 2

	tests := []struct {
		name    string
		domain  string
		inScope bool
	}{
		{
			name:    "root domain (depth 0)",
			domain:  "example.com",
			inScope: true,
		},
		{
			name:    "level 1 subdomain",
			domain:  "test.example.com",
			inScope: true,
		},
		{
			name:    "level 2 subdomain (at limit)",
			domain:  "api.test.example.com",
			inScope: true,
		},
		{
			name:    "level 3 subdomain (exceeds limit)",
			domain:  "v1.api.test.example.com",
			inScope: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := target.IsInScope(tt.domain)
			if tt.inScope {
				testutil.AssertTrue(t, result, "should be in scope")
			} else {
				testutil.AssertFalse(t, result, "should not be in scope (exceeds max depth)")
			}
		})
	}
}

func TestTarget_String(t *testing.T) {
	target := NewTarget("example.com", ScanModePassive)
	str := target.String()

	testutil.AssertNotEqual(t, str, "", "string representation should not be empty")
}
