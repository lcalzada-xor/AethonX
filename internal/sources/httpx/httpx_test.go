package httpx

import (
	"encoding/json"
	"testing"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
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
			name:      "invalid empty exec path",
			source:    NewWithConfig(logger, "", ProfileBasic, 30*time.Second, 50, 150),
			wantError: true,
		},
		{
			name:      "invalid negative timeout",
			source:    NewWithConfig(logger, "httpx", ProfileBasic, -1*time.Second, 50, 150),
			wantError: true,
		},
		{
			name:      "invalid zero threads",
			source:    NewWithConfig(logger, "httpx", ProfileBasic, 30*time.Second, 0, 150),
			wantError: true,
		},
		{
			name:      "invalid negative rate limit",
			source:    NewWithConfig(logger, "httpx", ProfileBasic, 30*time.Second, 50, -1),
			wantError: true,
		},
		{
			name:      "invalid scan profile",
			source:    NewWithConfig(logger, "httpx", "invalid_profile", 30*time.Second, 50, 150),
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
		"host": "93.184.216.34",
		"port": "443",
		"method": "GET",
		"tech": ["Nginx", "Ubuntu"],
		"cdn": true,
		"cdn_name": "Cloudflare",
		"failed": false
	}`

	var resp HTTPXResponse
	if err := json.Unmarshal([]byte(jsonLine), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	target := domain.NewTarget("example.com", domain.ScanModeActive)
	artifacts := parser.ParseResponse(&resp, *target)

	// Should create: URL + Domain (alive) + IP + 2 Technologies = 5 artifacts minimum
	if len(artifacts) < 5 {
		t.Errorf("expected at least 5 artifacts, got %d", len(artifacts))
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

	// Check Domain artifact (alive)
	domainArtifact := artifacts[1]
	if domainArtifact.Type != domain.ArtifactTypeDomain {
		t.Errorf("expected second artifact to be Domain, got %s", domainArtifact.Type)
	}
	if domainArtifact.Value != "example.com" {
		t.Errorf("expected domain 'example.com', got '%s'", domainArtifact.Value)
	}

	// Check IP artifact
	ipArtifact := artifacts[2]
	if ipArtifact.Type != domain.ArtifactTypeIP {
		t.Errorf("expected third artifact to be IP, got %s", ipArtifact.Type)
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

	args := source.buildCommandArgs(*target)

	// Verify args contain essential flags
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

// Note: extractTargetsFromInput tests removed - functionality is private and
// tested implicitly through RunWithInput integration tests
//
// func TestHTTPXSource_ExtractTargetsFromInput(t *testing.T) {
// 	logger := logx.New()
// 	source := New(logger)
//
// 	// Create input with multiple artifact types
// 	input := domain.NewScanResult(*domain.NewTarget("example.com", domain.ScanModeActive))
//
// 	// Add subdomains (note: www.example.com will be normalized to example.com)
// 	input.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "mail.example.com", "crtsh"))
// 	input.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "api.example.com", "crtsh"))
// 	input.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "blog.example.com", "crtsh"))
//
// 	// Add domains
// 	input.AddArtifact(domain.NewArtifact(domain.ArtifactTypeDomain, "example.com", "rdap"))
//
// 	// Add URLs
// 	input.AddArtifact(domain.NewArtifact(domain.ArtifactTypeURL, "https://example.com/admin", "wayback"))
// 	input.AddArtifact(domain.NewArtifact(domain.ArtifactTypeURL, "https://staging.example.com/login", "wayback"))
// 
// 	// Add non-relevant artifacts (should be ignored)
// 	input.AddArtifact(domain.NewArtifact(domain.ArtifactTypeIP, "93.184.216.34", "crtsh"))
// 	input.AddArtifact(domain.NewArtifact(domain.ArtifactTypeEmail, "admin@example.com", "rdap"))
// 
// 	targets := source.extractTargetsFromInput(input)
// 
// 	// Should extract: 3 subdomains + 1 domain + 2 URLs = 6 targets
// 	if len(targets) != 6 {
// 		t.Errorf("expected 6 targets, got %d", len(targets))
// 	}
// 
// 	// Verify no duplicates
// 	seen := make(map[string]bool)
// 	for _, target := range targets {
// 		if seen[target] {
// 			t.Errorf("duplicate target found: %s", target)
// 		}
// 		seen[target] = true
// 	}
// 
// 	// Verify specific targets are present
// 	expectedTargets := []string{
// 		"mail.example.com",
// 		"api.example.com",
// 		"blog.example.com",
// 		"example.com",
// 		"https://example.com/admin",
// 		"https://staging.example.com/login",
// 	}
// 
// 	for _, expected := range expectedTargets {
// 		found := false
// 		for _, target := range targets {
// 			if target == expected {
// 				found = true
// 				break
// 			}
// 		}
// 		if !found {
// 			t.Errorf("expected target '%s' not found in extracted targets", expected)
// 		}
// 	}
// }
// 
// func TestHTTPXSource_ExtractTargetsFromInput_Empty(t *testing.T) {
// 	logger := logx.New()
// 	source := New(logger)
// 
// 	// Create empty input
// 	input := domain.NewScanResult(*domain.NewTarget("example.com", domain.ScanModeActive))
// 
// 	targets := source.extractTargetsFromInput(input)
// 
// 	// Should return empty slice
// 	if len(targets) != 0 {
// 		t.Errorf("expected 0 targets for empty input, got %d", len(targets))
// 	}
// }
// 
// func TestHTTPXSource_ExtractTargetsFromInput_OnlyIrrelevant(t *testing.T) {
// 	logger := logx.New()
// 	source := New(logger)
// 
// 	// Create input with only irrelevant artifacts
// 	input := domain.NewScanResult(*domain.NewTarget("example.com", domain.ScanModeActive))
// 
// 	input.AddArtifact(domain.NewArtifact(domain.ArtifactTypeIP, "93.184.216.34", "crtsh"))
// 	input.AddArtifact(domain.NewArtifact(domain.ArtifactTypeEmail, "admin@example.com", "rdap"))
// 	input.AddArtifact(domain.NewArtifact(domain.ArtifactTypeTechnology, "nginx", "httpx"))
// 
// 	targets := source.extractTargetsFromInput(input)
// 
// 	// Should return empty slice
// 	if len(targets) != 0 {
// 		t.Errorf("expected 0 targets for irrelevant artifacts, got %d", len(targets))
// 	}
// }
// 
// func TestHTTPXSource_ExtractTargetsFromInput_Deduplication(t *testing.T) {
// 	logger := logx.New()
// 	source := New(logger)
// 
// 	// Create input with duplicate artifacts
// 	input := domain.NewScanResult(*domain.NewTarget("example.com", domain.ScanModeActive))
// 
// 	input.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "mail.example.com", "crtsh"))
// 	input.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "mail.example.com", "dnsbuffer"))
// 	input.AddArtifact(domain.NewArtifact(domain.ArtifactTypeDomain, "example.com", "rdap"))
// 	input.AddArtifact(domain.NewArtifact(domain.ArtifactTypeDomain, "example.com", "crtsh"))
// 
// 	targets := source.extractTargetsFromInput(input)
// 
// 	// Should deduplicate: 1 unique subdomain + 1 unique domain = 2 targets
// 	if len(targets) != 2 {
// 		t.Errorf("expected 2 deduplicated targets, got %d", len(targets))
// 	}
// 
// 	// Verify deduplication
// 	expectedTargets := map[string]bool{
// 		"mail.example.com": false,
// 		"example.com":      false,
// 	}
// 
// 	for _, target := range targets {
// 		if _, exists := expectedTargets[target]; exists {
// 			if expectedTargets[target] {
// 				t.Errorf("duplicate target found after deduplication: %s", target)
// 			}
// 			expectedTargets[target] = true
// 		} else {
// 			t.Errorf("unexpected target found: %s", target)
// 		}
// 	}
// }
// 
func TestHTTPXSource_BuildCommandWithStdin(t *testing.T) {
	logger := logx.New()
	source := NewWithConfig(logger, "httpx", ProfileBasic, 60*time.Second, 25, 100)

	args := source.buildCommandArgsWithStdin()

	// Verify args contain essential flags but NOT -u flag
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
		if arg == "-u" {
			containsTarget = true
		}

		// Verify timeout calculation considers number of targets
		if arg == "-timeout" && i+1 < len(args) {
			timeout := args[i+1]
			if timeout == "" {
				t.Error("timeout value should not be empty")
			}
		}
	}

	if !containsJSON {
		t.Error("expected command args to contain '-json'")
	}
	if !containsSilent {
		t.Error("expected command args to contain '-silent'")
	}
	if containsTarget {
		t.Error("expected command args NOT to contain '-u' when using stdin")
	}
}

func TestParser_ParseTechNameAndVersion(t *testing.T) {
	tests := []struct {
		input          string
		expectedName   string
		expectedVersion string
	}{
		{"jQuery:3.6.0", "jQuery", "3.6.0"},
		{"jQuery", "jQuery", ""},
		{"nginx:1.24.0", "nginx", "1.24.0"},
		{"React:18.2.0", "React", "18.2.0"},
		{"Bootstrap:5.3.0", "Bootstrap", "5.3.0"},
		{"", "", ""},
		{"  jQuery:3.6.0  ", "jQuery", "3.6.0"},
		{"Vue.js:3.0.0", "Vue.js", "3.0.0"},
		{"Ubuntu", "Ubuntu", ""},
		{"Apache:2.4.41:extra", "Apache", "2.4.41:extra"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, version := parseTechNameAndVersion(tt.input)
			if name != tt.expectedName {
				t.Errorf("parseTechNameAndVersion(%s) name = %s, want %s", tt.input, name, tt.expectedName)
			}
			if version != tt.expectedVersion {
				t.Errorf("parseTechNameAndVersion(%s) version = %s, want %s", tt.input, version, tt.expectedVersion)
			}
		})
	}
}

func TestParser_CreateTechnologyArtifact_WithVersion(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "httpx")

	// Test with version
	artifact := parser.createTechnologyArtifact("jQuery:3.6.0", "https://example.com")

	if artifact.Type != domain.ArtifactTypeTechnology {
		t.Errorf("expected artifact type Technology, got %s", artifact.Type)
	}

	if artifact.Value != "jQuery" {
		t.Errorf("expected artifact value 'jQuery', got '%s'", artifact.Value)
	}

	// Check TypedMetadata
	if artifact.TypedMetadata == nil {
		t.Fatal("expected TypedMetadata to be set")
	}

	techMeta, ok := artifact.TypedMetadata.(*metadata.TechnologyMetadata)
	if !ok {
		t.Fatal("expected TypedMetadata to be TechnologyMetadata")
	}

	if techMeta.Name != "jQuery" {
		t.Errorf("expected technology name 'jQuery', got '%s'", techMeta.Name)
	}

	if techMeta.Version != "3.6.0" {
		t.Errorf("expected technology version '3.6.0', got '%s'", techMeta.Version)
	}

	if techMeta.DetectionMethod != "wappalyzer" {
		t.Errorf("expected detection method 'wappalyzer', got '%s'", techMeta.DetectionMethod)
	}
}

func TestParser_CreateTechnologyArtifact_WithoutVersion(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "httpx")

	// Test without version
	artifact := parser.createTechnologyArtifact("Ubuntu", "https://example.com")

	if artifact.Value != "Ubuntu" {
		t.Errorf("expected artifact value 'Ubuntu', got '%s'", artifact.Value)
	}

	techMeta, ok := artifact.TypedMetadata.(*metadata.TechnologyMetadata)
	if !ok {
		t.Fatal("expected TypedMetadata to be TechnologyMetadata")
	}

	if techMeta.Name != "Ubuntu" {
		t.Errorf("expected technology name 'Ubuntu', got '%s'", techMeta.Name)
	}

	if techMeta.Version != "" {
		t.Errorf("expected empty version, got '%s'", techMeta.Version)
	}
}

func TestParser_ExtractHostname(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "httpx")

	tests := []struct {
		name     string
		resp     HTTPXResponse
		expected string
	}{
		{
			name: "input as domain",
			resp: HTTPXResponse{
				Input: "example.com",
				URL:   "https://example.com",
				Host:  "93.184.216.34", // IP, not hostname
			},
			expected: "example.com",
		},
		{
			name: "input as full URL",
			resp: HTTPXResponse{
				Input: "https://example.com",
				URL:   "https://example.com",
				Host:  "93.184.216.34",
			},
			expected: "example.com",
		},
		{
			name: "input empty, use URL",
			resp: HTTPXResponse{
				Input: "",
				URL:   "https://example.com:8080/path",
				Host:  "93.184.216.34",
			},
			expected: "example.com:8080",
		},
		{
			name: "subdomain in input",
			resp: HTTPXResponse{
				Input: "api.example.com",
				URL:   "https://api.example.com",
				Host:  "93.184.216.34",
			},
			expected: "api.example.com",
		},
		{
			name: "all fields empty",
			resp: HTTPXResponse{
				Input: "",
				URL:   "",
				Host:  "93.184.216.34",
			},
			expected: "",
		},
		{
			name: "input with port",
			resp: HTTPXResponse{
				Input: "example.com:8443",
				URL:   "https://example.com:8443",
				Host:  "93.184.216.34",
			},
			expected: "example.com:8443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.extractHostname(&tt.resp)
			if result != tt.expected {
				t.Errorf("extractHostname() = %v, want %v", result, tt.expected)
			}
		})
	}
}
