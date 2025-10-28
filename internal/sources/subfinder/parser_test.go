package subfinder

import (
	"testing"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
)

func TestParser_ParseResponse(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "subfinder")
	target := domain.Target{
		Root: "example.com",
		Mode: domain.ScanModePassive,
		Scope: domain.ScopeConfig{
			OnlyInScope:       true,
			IncludeSubdomains: true,
		},
	}

	tests := []struct {
		name          string
		response      *SubfinderResponse
		target        domain.Target
		expectCount   int
		expectValue   string
		expectConfidence float64
	}{
		{
			name: "valid subdomain",
			response: &SubfinderResponse{
				Host:   "sub.example.com",
				Source: []string{"crtsh", "hackertarget"},
			},
			target:           target,
			expectCount:      1,
			expectValue:      "sub.example.com",
			expectConfidence: 0.60, // ConfidenceMedium
		},
		{
			name: "wildcard subdomain - should be skipped",
			response: &SubfinderResponse{
				Host:   "*.example.com",
				Source: []string{"crtsh"},
			},
			target:      target,
			expectCount: 0,
		},
		{
			name:        "nil response",
			response:    nil,
			target:      target,
			expectCount: 0,
		},
		{
			name: "empty host",
			response: &SubfinderResponse{
				Host:   "",
				Source: []string{"crtsh"},
			},
			target:      target,
			expectCount: 0,
		},
		{
			name: "out of scope host",
			response: &SubfinderResponse{
				Host:   "sub.other.com",
				Source: []string{"crtsh"},
			},
			target:      target,
			expectCount: 0,
		},
		{
			name: "deep subdomain with tag",
			response: &SubfinderResponse{
				Host:   "a.b.c.example.com",
				Source: []string{"virustotal"},
			},
			target:           target,
			expectCount:      1,
			expectValue:      "a.b.c.example.com",
			expectConfidence: 0.60, // ConfidenceMedium
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifacts := parser.ParseResponse(tt.response, tt.target)

			if len(artifacts) != tt.expectCount {
				t.Errorf("expected %d artifacts, got %d", tt.expectCount, len(artifacts))
				return
			}

			if tt.expectCount > 0 {
				artifact := artifacts[0]

				if artifact.Value != tt.expectValue {
					t.Errorf("expected value %s, got %s", tt.expectValue, artifact.Value)
				}

				if artifact.Confidence != tt.expectConfidence {
					t.Errorf("expected confidence %f, got %f", tt.expectConfidence, artifact.Confidence)
				}

				if artifact.Type != domain.ArtifactTypeSubdomain {
					t.Errorf("expected type %s, got %s", domain.ArtifactTypeSubdomain, artifact.Type)
				}

				// Check if deep subdomain has proper tag
				if tt.name == "deep subdomain with tag" {
					hasTag := false
					for _, tag := range artifact.Tags {
						if tag == "deep-subdomain" {
							hasTag = true
							break
						}
					}
					if !hasTag {
						t.Errorf("expected 'deep-subdomain' tag for %s", artifact.Value)
					}
				}
			}
		})
	}
}

func TestParser_ParseMultipleResponses(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "subfinder")
	target := domain.Target{
		Root: "example.com",
		Mode: domain.ScanModePassive,
		Scope: domain.ScopeConfig{
			OnlyInScope:       true,
			IncludeSubdomains: true,
		},
	}

	responses := []*SubfinderResponse{
		{
			Host:   "sub1.example.com",
			Source: []string{"crtsh"},
		},
		{
			Host:   "sub2.example.com",
			Source: []string{"hackertarget"},
		},
		{
			Host:   "sub1.example.com", // Duplicate
			Source: []string{"virustotal"},
		},
		{
			Host:   "*.example.com", // Wildcard - should be skipped
			Source: []string{"crtsh"},
		},
	}

	artifacts := parser.ParseMultipleResponses(responses, target)

	// Should have 2 unique artifacts (sub1, sub2)
	if len(artifacts) != 2 {
		t.Errorf("expected 2 artifacts, got %d", len(artifacts))
	}

	// Check values
	values := make(map[string]bool)
	for _, a := range artifacts {
		values[a.Value] = true
	}

	if !values["sub1.example.com"] {
		t.Error("expected sub1.example.com in results")
	}
	if !values["sub2.example.com"] {
		t.Error("expected sub2.example.com in results")
	}
}

func TestParser_ValidateResponse(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "subfinder")

	tests := []struct {
		name      string
		response  *SubfinderResponse
		expectErr bool
	}{
		{
			name: "valid response",
			response: &SubfinderResponse{
				Host:   "sub.example.com",
				Source: []string{"crtsh"},
			},
			expectErr: false,
		},
		{
			name:      "nil response",
			response:  nil,
			expectErr: true,
		},
		{
			name: "empty host",
			response: &SubfinderResponse{
				Host:   "",
				Source: []string{"crtsh"},
			},
			expectErr: true,
		},
		{
			name: "host with protocol",
			response: &SubfinderResponse{
				Host:   "https://sub.example.com",
				Source: []string{"crtsh"},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.ValidateResponse(tt.response)
			if (err != nil) != tt.expectErr {
				t.Errorf("expected error: %v, got error: %v", tt.expectErr, err)
			}
		})
	}
}
