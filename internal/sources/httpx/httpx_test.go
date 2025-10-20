package httpx

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
)

func TestHTTPXSource_Name(t *testing.T) {
	logger := logx.New()
	source := New(logger)

	if source.Name() != "httpx" {
		t.Errorf("expected name 'httpx', got '%s'", source.Name())
	}
}

func TestHTTPXSource_Mode(t *testing.T) {
	logger := logx.New()
	source := New(logger)

	if source.Mode() != domain.SourceModeActive {
		t.Errorf("expected mode 'active', got '%s'", source.Mode())
	}
}

func TestHTTPXSource_Type(t *testing.T) {
	logger := logx.New()
	source := New(logger)

	if source.Type() != domain.SourceTypeCLI {
		t.Errorf("expected type 'cli', got '%s'", source.Type())
	}
}

func TestHTTPXSource_Validate(t *testing.T) {
	logger := logx.New()

	tests := []struct {
		name      string
		source    *HTTPXSource
		wantError bool
	}{
		{
			name:      "valid default config",
			source:    New(logger),
			wantError: false,
		},
		{
			name: "invalid empty exec path",
			source: &HTTPXSource{
				logger:    logger,
				execPath:  "",
				profile:   ProfileBasic,
				timeout:   30 * time.Second,
				threads:   50,
				rateLimit: 150,
			},
			wantError: true,
		},
		{
			name: "invalid negative timeout",
			source: &HTTPXSource{
				logger:    logger,
				execPath:  "httpx",
				profile:   ProfileBasic,
				timeout:   -1 * time.Second,
				threads:   50,
				rateLimit: 150,
			},
			wantError: true,
		},
		{
			name: "invalid zero threads",
			source: &HTTPXSource{
				logger:    logger,
				execPath:  "httpx",
				profile:   ProfileBasic,
				timeout:   30 * time.Second,
				threads:   0,
				rateLimit: 150,
			},
			wantError: true,
		},
		{
			name: "invalid negative rate limit",
			source: &HTTPXSource{
				logger:    logger,
				execPath:  "httpx",
				profile:   ProfileBasic,
				timeout:   30 * time.Second,
				threads:   50,
				rateLimit: -1,
			},
			wantError: true,
		},
		{
			name: "invalid scan profile",
			source: &HTTPXSource{
				logger:    logger,
				execPath:  "httpx",
				profile:   "invalid_profile",
				timeout:   30 * time.Second,
				threads:   50,
				rateLimit: 150,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.source.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError = %v", err, tt.wantError)
			}
		})
	}
}

func TestParser_ParseResponse_Success(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "httpx")

	jsonLine := `{
		"timestamp": "2025-10-20T10:00:00Z",
		"url": "https://example.com",
		"input": "example.com",
		"title": "Example Domain",
		"status_code": 200,
		"content_length": 1256,
		"content_type": "text/html",
		"webserver": "nginx/1.24.0",
		"response_time": "125ms",
		"scheme": "https",
		"host": "example.com",
		"port": "443",
		"method": "GET",
		"tech": ["Nginx", "Ubuntu"],
		"ip": "93.184.216.34",
		"cdn": "cloudflare",
		"cdn_name": "Cloudflare",
		"failed": false
	}`

	var resp HTTPXResponse
	if err := json.Unmarshal([]byte(jsonLine), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	target := domain.NewTarget("example.com", domain.ScanModeActive)
	artifacts := parser.ParseResponse(&resp, *target)

	// Should create: URL + IP + 2 Technologies = 4 artifacts minimum
	if len(artifacts) < 4 {
		t.Errorf("expected at least 4 artifacts, got %d", len(artifacts))
	}

	// Check URL artifact
	urlArtifact := artifacts[0]
	if urlArtifact.Type != domain.ArtifactTypeURL {
		t.Errorf("expected first artifact to be URL, got %s", urlArtifact.Type)
	}
	if urlArtifact.Value != "https://example.com" {
		t.Errorf("expected URL 'https://example.com', got '%s'", urlArtifact.Value)
	}
	if urlArtifact.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %f", urlArtifact.Confidence)
	}

	// Check IP artifact
	ipArtifact := artifacts[1]
	if ipArtifact.Type != domain.ArtifactTypeIP {
		t.Errorf("expected second artifact to be IP, got %s", ipArtifact.Type)
	}
	if ipArtifact.Value != "93.184.216.34" {
		t.Errorf("expected IP '93.184.216.34', got '%s'", ipArtifact.Value)
	}

	// Check technology artifacts
	techCount := 0
	for _, a := range artifacts {
		if a.Type == domain.ArtifactTypeTechnology {
			techCount++
		}
	}
	if techCount != 2 {
		t.Errorf("expected 2 technology artifacts, got %d", techCount)
	}
}

func TestParser_ParseResponse_Failed(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "httpx")

	jsonLine := `{
		"url": "https://notfound.example.com",
		"input": "notfound.example.com",
		"failed": true,
		"status_code": 0
	}`

	var resp HTTPXResponse
	if err := json.Unmarshal([]byte(jsonLine), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	target := domain.NewTarget("notfound.example.com", domain.ScanModeActive)
	artifacts := parser.ParseResponse(&resp, *target)

	// Failed probe should return empty artifacts
	if len(artifacts) != 0 {
		t.Errorf("expected 0 artifacts for failed probe, got %d", len(artifacts))
	}
}

func TestParser_ParseResponse_WithTLS(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "httpx")

	jsonLine := `{
		"url": "https://example.com",
		"status_code": 200,
		"scheme": "https",
		"host": "example.com",
		"port": "443",
		"failed": false,
		"tls": {
			"host": "example.com",
			"port": "443",
			"probe_status": true,
			"version": "TLSv1.3",
			"cipher": "TLS_AES_128_GCM_SHA256",
			"subject_dn": "CN=example.com",
			"issuer_dn": "CN=Let's Encrypt Authority X3",
			"subject_cn": "example.com",
			"issuer_cn": "Let's Encrypt Authority X3",
			"subject_an": ["example.com", "www.example.com"],
			"not_before": "2025-01-01T00:00:00Z",
			"not_after": "2025-04-01T00:00:00Z"
		}
	}`

	var resp HTTPXResponse
	if err := json.Unmarshal([]byte(jsonLine), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	target := domain.NewTarget("example.com", domain.ScanModeActive)
	artifacts := parser.ParseResponse(&resp, *target)

	// Should create: URL + Certificate + 2 SANs as subdomains
	certCount := 0
	subdomainCount := 0
	for _, a := range artifacts {
		if a.Type == domain.ArtifactTypeCertificate {
			certCount++
		}
		if a.Type == domain.ArtifactTypeSubdomain {
			subdomainCount++
		}
	}

	if certCount != 1 {
		t.Errorf("expected 1 certificate artifact, got %d", certCount)
	}

	if subdomainCount != 2 {
		t.Errorf("expected 2 subdomain artifacts from SANs, got %d", subdomainCount)
	}
}

func TestParser_ExtractProduct(t *testing.T) {
	tests := []struct {
		banner   string
		expected string
	}{
		{"nginx/1.24.0", "nginx"},
		{"Apache/2.4.41 (Ubuntu)", "Apache"},
		{"Microsoft-IIS/10.0", "Microsoft-IIS"},
		{"", ""},
		{"nginx", "nginx"},
	}

	for _, tt := range tests {
		t.Run(tt.banner, func(t *testing.T) {
			result := extractProduct(tt.banner)
			if result != tt.expected {
				t.Errorf("extractProduct(%s) = %s, want %s", tt.banner, result, tt.expected)
			}
		})
	}
}

func TestParser_ExtractVersion(t *testing.T) {
	tests := []struct {
		banner   string
		expected string
	}{
		{"nginx/1.24.0", "1.24.0"},
		{"Apache/2.4.41 (Ubuntu)", "2.4.41"},
		{"Microsoft-IIS/10.0", "10.0"},
		{"", ""},
		{"nginx", ""},
	}

	for _, tt := range tests {
		t.Run(tt.banner, func(t *testing.T) {
			result := extractVersion(tt.banner)
			if result != tt.expected {
				t.Errorf("extractVersion(%s) = %s, want %s", tt.banner, result, tt.expected)
			}
		})
	}
}

func TestParser_ParsePort(t *testing.T) {
	tests := []struct {
		portStr  string
		expected int
	}{
		{"80", 80},
		{"443", 443},
		{"8080", 8080},
		{"", 0},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.portStr, func(t *testing.T) {
			result := parsePort(tt.portStr)
			if result != tt.expected {
				t.Errorf("parsePort(%s) = %d, want %d", tt.portStr, result, tt.expected)
			}
		})
	}
}

func TestParser_IsValidDomain(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "httpx")

	tests := []struct {
		domain   string
		expected bool
	}{
		{"example.com", true},
		{"sub.example.com", true},
		{"*.example.com", true},
		{"example", false},
		{"", false},
		{"example@com", false},
		{"example..com", true}, // Basic validation doesn't catch consecutive dots
		{"very.long.subdomain.example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			result := parser.isValidDomain(tt.domain)
			if result != tt.expected {
				t.Errorf("isValidDomain(%s) = %v, want %v", tt.domain, result, tt.expected)
			}
		})
	}
}

func TestGetProfile(t *testing.T) {
	tests := []struct {
		profile  ScanProfile
		expected string
	}{
		{ProfileBasic, "Basic host verification with essential metadata"},
		{ProfileTech, "Technology detection and advanced fingerprinting"},
		{ProfileTLS, "TLS/SSL certificate analysis"},
		{ProfileFull, "Comprehensive scan with all probes enabled"},
		{ProfileHeadless, "Visual reconnaissance with headless browser (requires Chrome)"},
		{"invalid", "Basic host verification with essential metadata"}, // Falls back to basic
	}

	for _, tt := range tests {
		t.Run(string(tt.profile), func(t *testing.T) {
			cfg := GetProfile(tt.profile)
			if cfg.Description != tt.expected {
				t.Errorf("GetProfile(%s).Description = %s, want %s", tt.profile, cfg.Description, tt.expected)
			}
		})
	}
}

func TestHTTPXSource_SetProfile(t *testing.T) {
	logger := logx.New()
	source := New(logger)

	// Default should be ProfileFull
	if source.profile != ProfileFull {
		t.Errorf("expected default profile 'full', got '%s'", source.profile)
	}

	// Change to ProfileBasic
	source.SetProfile(ProfileBasic)
	if source.profile != ProfileBasic {
		t.Errorf("expected profile 'basic' after SetProfile, got '%s'", source.profile)
	}
}

func TestHTTPXSource_SetCustomFlags(t *testing.T) {
	logger := logx.New()
	source := New(logger)

	customFlags := []string{"-proxy", "http://proxy:8080", "-header", "X-Custom: value"}
	source.SetCustomFlags(customFlags)

	if len(source.customFlags) != len(customFlags) {
		t.Errorf("expected %d custom flags, got %d", len(customFlags), len(source.customFlags))
	}

	for i, flag := range customFlags {
		if source.customFlags[i] != flag {
			t.Errorf("expected custom flag[%d] = '%s', got '%s'", i, flag, source.customFlags[i])
		}
	}
}

func TestHTTPXSource_Close(t *testing.T) {
	logger := logx.New()
	source := New(logger)

	// Close should not error even if no process is running
	if err := source.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Close should be idempotent
	if err := source.Close(); err != nil {
		t.Errorf("second Close() returned error: %v", err)
	}
}

func TestHTTPXSource_BuildCommand(t *testing.T) {
	logger := logx.New()
	source := NewWithConfig(logger, "httpx", ProfileBasic, 60*time.Second, 25, 100)

	target := domain.NewTarget("example.com", domain.ScanModeActive)
	ctx := context.Background()

	cmd := source.buildCommand(ctx, *target)

	// Verify command path
	if cmd.Path == "" {
		t.Error("expected command path to be set")
	}

	// Verify args contain essential flags
	args := cmd.Args
	containsJSON := false
	containsSilent := false
	containsTarget := false

	for i, arg := range args {
		if arg == "-json" {
			containsJSON = true
		}
		if arg == "-silent" {
			containsSilent = true
		}
		if arg == "-u" && i+1 < len(args) && args[i+1] == "example.com" {
			containsTarget = true
		}
	}

	if !containsJSON {
		t.Error("expected command args to contain '-json'")
	}
	if !containsSilent {
		t.Error("expected command args to contain '-silent'")
	}
	if !containsTarget {
		t.Error("expected command args to contain target 'example.com'")
	}
}

func TestParser_ParseMultipleResponses(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "httpx")

	responses := []*HTTPXResponse{
		{
			URL:        "https://example.com",
			StatusCode: 200,
			Host:       "example.com",
			Port:       "443",
			Scheme:     "https",
			Failed:     false,
		},
		{
			URL:        "https://www.example.com",
			StatusCode: 200,
			Host:       "www.example.com",
			Port:       "443",
			Scheme:     "https",
			Failed:     false,
		},
	}

	target := domain.NewTarget("example.com", domain.ScanModeActive)
	artifacts := parser.ParseMultipleResponses(responses, *target)

	// Should create at least 2 URL artifacts (one per response)
	urlCount := 0
	for _, a := range artifacts {
		if a.Type == domain.ArtifactTypeURL {
			urlCount++
		}
	}

	if urlCount < 2 {
		t.Errorf("expected at least 2 URL artifacts, got %d", urlCount)
	}
}
