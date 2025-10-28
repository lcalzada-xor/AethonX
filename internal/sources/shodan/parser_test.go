// internal/sources/shodan/parser_test.go
package shodan

import (
	"fmt"
	"testing"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
)

func TestParser_ParseHostResponse(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "shodan")

	target := domain.Target{Root: "example.com"}

	tests := []struct {
		name           string
		response       *ShodanHostResponse
		expectedCount  int
		expectedTypes  []domain.ArtifactType
	}{
		{
			name: "basic host with IP and port",
			response: &ShodanHostResponse{
				IPStr:     "93.184.216.34",
				Port:      443,
				Transport: "tcp",
				Hostnames: []string{"example.com"},
				Location:  LocationData{CountryCode: "US", CountryName: "United States"},
			},
			expectedCount: 4, // IP, subdomain, port, service
			expectedTypes: []domain.ArtifactType{
				domain.ArtifactTypeIP,
				domain.ArtifactTypeSubdomain,
				domain.ArtifactTypePort,
				domain.ArtifactTypeService,
			},
		},
		{
			name: "host with vulnerabilities",
			response: &ShodanHostResponse{
				IPStr:     "93.184.216.34",
				Port:      443,
				Hostnames: []string{"example.com"},
				Vulns:     []string{"CVE-2021-44228", "CVE-2022-0778"},
				Product:   "nginx",
				Version:   "1.18.0",
				Location:  LocationData{CountryCode: "US"},
			},
			expectedCount: 7, // IP, subdomain, port, service, technology, 2 vulns
			expectedTypes: []domain.ArtifactType{
				domain.ArtifactTypeIP,
				domain.ArtifactTypeSubdomain,
				domain.ArtifactTypePort,
				domain.ArtifactTypeService,
				domain.ArtifactTypeVulnerability,
				domain.ArtifactTypeTechnology,
			},
		},
		{
			name: "host with SSL certificate",
			response: &ShodanHostResponse{
				IPStr:     "93.184.216.34",
				Port:      443,
				Hostnames: []string{"example.com"},
				SSL: &SSLData{
					Cert: CertData{
						Subject: CertName{CN: "example.com"},
						Issuer:  CertName{CN: "DigiCert"},
						Serial:  "123456",
						Expires: "2025-12-31T23:59:59Z",
					},
				},
				Location: LocationData{CountryCode: "US"},
			},
			expectedCount: 5, // IP, subdomain, port, service, certificate
			expectedTypes: []domain.ArtifactType{
				domain.ArtifactTypeIP,
				domain.ArtifactTypeSubdomain,
				domain.ArtifactTypePort,
				domain.ArtifactTypeService,
				domain.ArtifactTypeCertificate,
			},
		},
		{
			name: "host with ASN and cloud provider",
			response: &ShodanHostResponse{
				IPStr:     "93.184.216.34",
				Port:      443,
				Hostnames: []string{"example.com"},
				ASN:       "AS15133",
				Cloud: &CloudData{
					Provider: "aws",
					Region:   "us-east-1",
				},
				Location: LocationData{CountryCode: "US"},
			},
			expectedCount: 6, // IP, subdomain, port, service, ASN, cloud
			expectedTypes: []domain.ArtifactType{
				domain.ArtifactTypeIP,
				domain.ArtifactTypeSubdomain,
				domain.ArtifactTypePort,
				domain.ArtifactTypeService,
				domain.ArtifactTypeASN,
				domain.ArtifactTypeCloudResource,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifacts := parser.ParseHostResponse(tt.response, target)

			if len(artifacts) != tt.expectedCount {
				t.Errorf("expected %d artifacts, got %d", tt.expectedCount, len(artifacts))
			}

			// Verify expected artifact types are present
			foundTypes := make(map[domain.ArtifactType]bool)
			for _, artifact := range artifacts {
				foundTypes[artifact.Type] = true
			}

			for _, expectedType := range tt.expectedTypes {
				if !foundTypes[expectedType] {
					t.Errorf("expected artifact type %s not found", expectedType)
				}
			}
		})
	}
}

func TestParser_ParseDomainResponse(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "shodan")

	target := domain.Target{Root: "example.com"}

	tests := []struct {
		name      string
		response  *ShodanDomainResponse
		shouldNil bool
	}{
		{
			name: "valid subdomain",
			response: &ShodanDomainResponse{
				Domain:    "example.com",
				Subdomain: "www",
				Type:      "A",
				Value:     "93.184.216.34",
			},
			shouldNil: false,
		},
		{
			name: "full subdomain format",
			response: &ShodanDomainResponse{
				Domain:    "example.com",
				Subdomain: "api.example.com",
				Type:      "A",
				Value:     "93.184.216.35",
			},
			shouldNil: false,
		},
		{
			name: "empty subdomain",
			response: &ShodanDomainResponse{
				Domain:    "example.com",
				Subdomain: "",
				Type:      "A",
				Value:     "93.184.216.34",
			},
			shouldNil: true,
		},
		{
			name: "irrelevant domain",
			response: &ShodanDomainResponse{
				Domain:    "other.com",
				Subdomain: "www",
				Type:      "A",
				Value:     "93.184.216.34",
			},
			shouldNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact := parser.ParseDomainResponse(tt.response, target)

			if tt.shouldNil && artifact != nil {
				t.Errorf("expected nil artifact, got %v", artifact)
			}

			if !tt.shouldNil && artifact == nil {
				t.Errorf("expected non-nil artifact, got nil")
			}

			if artifact != nil && artifact.Type != domain.ArtifactTypeSubdomain {
				t.Errorf("expected artifact type subdomain, got %s", artifact.Type)
			}
		})
	}
}

func TestParser_IsRelevantHostname(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "shodan")

	tests := []struct {
		hostname   string
		targetRoot string
		expected   bool
	}{
		{"example.com", "example.com", true},
		{"www.example.com", "example.com", true},
		{"api.example.com", "example.com", true},
		{"sub.api.example.com", "example.com", true},
		{"example.org", "example.com", false},
		{"examplecom.org", "example.com", false},
		{"", "example.com", false},
		{"example.com", "", false},
		{"EXAMPLE.COM", "example.com", true}, // Case insensitive
		{"WWW.EXAMPLE.COM", "example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			result := parser.isRelevantHostname(tt.hostname, tt.targetRoot)
			if result != tt.expected {
				t.Errorf("isRelevantHostname(%s, %s) = %v, expected %v",
					tt.hostname, tt.targetRoot, result, tt.expected)
			}
		})
	}
}

func TestInferSeverityFromCVE(t *testing.T) {
	tests := []struct {
		cve      string
		expected string
	}{
		{"CVE-2021-44228", "critical"}, // Log4Shell
		{"CVE-2014-0160", "critical"},  // Heartbleed
		{"CVE-2022-12345", "unknown"},  // Unknown CVE
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.cve, func(t *testing.T) {
			result := InferSeverityFromCVE(tt.cve)
			if result != tt.expected {
				t.Errorf("InferSeverityFromCVE(%s) = %s, expected %s",
					tt.cve, result, tt.expected)
			}
		})
	}
}

func TestNormalizeSeverity(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"CRITICAL", "critical"},
		{"High", "high"},
		{"medium", "medium"},
		{"LOW", "low"},
		{"info", "info"},
		{"moderate", "medium"},
		{"unknown", "unknown"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeSeverity(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeSeverity(%s) = %s, expected %s",
					tt.input, result, tt.expected)
			}
		})
	}
}

func TestCVSSScoreToSeverity(t *testing.T) {
	tests := []struct {
		score    float64
		expected string
	}{
		{10.0, "critical"},
		{9.5, "critical"},
		{9.0, "critical"},
		{8.5, "high"},
		{7.0, "high"},
		{6.5, "medium"},
		{4.0, "medium"},
		{3.5, "low"},
		{0.1, "low"},
		{0.0, "info"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%.1f", tt.score), func(t *testing.T) {
			result := CVSSScoreToSeverity(tt.score)
			if result != tt.expected {
				t.Errorf("CVSSScoreToSeverity(%.1f) = %s, expected %s",
					tt.score, result, tt.expected)
			}
		})
	}
}
