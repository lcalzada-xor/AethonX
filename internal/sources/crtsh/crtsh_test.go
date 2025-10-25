// internal/sources/crtsh/crtsh_test.go
package crtsh

import (
	"context"
	"testing"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
	"aethonx/internal/testutil"
)

func TestNew(t *testing.T) {
	logger := logx.New()
	source := New(logger)

	testutil.AssertNotNil(t, source, "source should not be nil")
	testutil.AssertEqual(t, source.Name(), "crtsh", "name should be crtsh")
	testutil.AssertEqual(t, source.Mode(), domain.SourceModePassive, "mode should be passive")
	testutil.AssertEqual(t, source.Type(), domain.SourceTypeAPI, "type should be API")
}

func TestCRT_Name(t *testing.T) {
	logger := logx.New()
	crt := New(logger)

	testutil.AssertEqual(t, crt.Name(), "crtsh", "name should be crtsh")
}

func TestCRT_Mode(t *testing.T) {
	logger := logx.New()
	crt := New(logger)

	testutil.AssertEqual(t, crt.Mode(), domain.SourceModePassive, "mode should be passive")
}

func TestCRT_Type(t *testing.T) {
	logger := logx.New()
	crt := New(logger)

	testutil.AssertEqual(t, crt.Type(), domain.SourceTypeAPI, "type should be API")
}

func TestCRT_Close(t *testing.T) {
	logger := logx.New()
	crt := New(logger)

	err := crt.Close()
	testutil.AssertNoError(t, err, "close should not return error")
}

func TestProcessRecords(t *testing.T) {
	logger := logx.New()
	crt := New(logger).(*CRT)
	target := *domain.NewTarget("example.com", domain.ScanModePassive)

	tests := []struct {
		name           string
		records        []certRecord
		expectedCount  int
		expectedValues []string
	}{
		{
			name: "single subdomain",
			records: []certRecord{
				{
					IssuerName:   "Let's Encrypt Authority X3",
					NameValue:    "test.example.com",
					NotAfter:     "2025-12-31T23:59:59",
					NotBefore:    "2025-01-01T00:00:00",
					SerialNumber: "ABC123",
				},
			},
			expectedCount:  2, // 1 subdomain + 1 certificate
			expectedValues: []string{"test.example.com"},
		},
		{
			name: "multiple subdomains in one certificate",
			records: []certRecord{
				{
					IssuerName:   "Let's Encrypt",
					NameValue:    "api.example.com\nwww.example.com",
					NotAfter:     "2025-12-31T23:59:59",
					NotBefore:    "2025-01-01T00:00:00",
					SerialNumber: "DEF456",
				},
			},
			expectedCount:  4, // 2 subdomains + 2 certificates (uno por subdomain)
			expectedValues: []string{"api.example.com", "example.com"}, // www. se normaliza a example.com
		},
		{
			name: "wildcard certificate",
			records: []certRecord{
				{
					IssuerName:   "DigiCert",
					NameValue:    "*.example.com",
					NotAfter:     "2026-01-01T00:00:00",
					NotBefore:    "2025-01-01T00:00:00",
					SerialNumber: "GHI789",
				},
			},
			expectedCount:  2, // 1 subdomain (normalizado sin *.) + 1 certificate
			expectedValues: []string{"example.com"}, // *. se normaliza a example.com
		},
		{
			name:           "empty records",
			records:        []certRecord{},
			expectedCount:  0,
			expectedValues: []string{},
		},
		{
			name: "out of scope subdomain",
			records: []certRecord{
				{
					IssuerName:   "Let's Encrypt",
					NameValue:    "test.other-domain.com",
					NotAfter:     "2025-12-31T23:59:59",
					NotBefore:    "2025-01-01T00:00:00",
					SerialNumber: "JKL012",
				},
			},
			expectedCount:  0, // Fuera de scope - no debería retornar artifacts
			expectedValues: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifacts := crt.processRecordsWithProgress(context.Background(), tt.records, target)

			testutil.AssertEqual(t, len(artifacts), tt.expectedCount,
				"should have expected number of artifacts")

			// Verificar valores de subdomains
			subdomainCount := 0
			for _, artifact := range artifacts {
				if artifact.Type == domain.ArtifactTypeSubdomain {
					subdomainCount++
					// Verificar que está en la lista esperada
					found := false
					for _, expected := range tt.expectedValues {
						if artifact.Value == expected {
							found = true
							break
						}
					}
					testutil.AssertTrue(t, found || len(tt.expectedValues) == 0,
						"subdomain should be in expected values")
				}
			}

			testutil.AssertEqual(t, subdomainCount, len(tt.expectedValues),
				"should have expected number of subdomains")
		})
	}
}

func TestProcessRecords_MetadataAndRelations(t *testing.T) {
	logger := logx.New()
	crt := New(logger).(*CRT)
	target := *domain.NewTarget("example.com", domain.ScanModePassive)

	records := []certRecord{
		{
			IssuerName:   "Let's Encrypt Authority X3",
			NameValue:    "test.example.com",
			NotAfter:     "2025-12-31T23:59:59",
			NotBefore:    "2025-01-01T00:00:00",
			SerialNumber: "ABC123",
		},
	}

	artifacts := crt.processRecordsWithProgress(context.Background(), records, target)

	// Debería tener 2 artifacts: 1 subdomain + 1 certificate
	testutil.AssertEqual(t, len(artifacts), 2, "should have 2 artifacts")

	// Encontrar el subdomain artifact
	var subdomainArtifact *domain.Artifact
	var certArtifact *domain.Artifact

	for _, a := range artifacts {
		if a.Type == domain.ArtifactTypeSubdomain {
			subdomainArtifact = a
		} else if a.Type == domain.ArtifactTypeCertificate {
			certArtifact = a
		}
	}

	testutil.AssertNotNil(t, subdomainArtifact, "should have subdomain artifact")
	testutil.AssertNotNil(t, certArtifact, "should have certificate artifact")

	// Verificar metadata del subdomain
	testutil.AssertEqual(t, subdomainArtifact.Value, "test.example.com", "subdomain value")
	testutil.AssertEqual(t, subdomainArtifact.Confidence, 0.95, "subdomain confidence")
	testutil.AssertNotNil(t, subdomainArtifact.TypedMetadata, "subdomain should have typed metadata")

	// Verificar que el subdomain tiene relación con el certificate
	testutil.AssertTrue(t, len(subdomainArtifact.Relations) == 1, "subdomain should have 1 relation")
	testutil.AssertEqual(t, subdomainArtifact.Relations[0].Type, domain.RelationUsesCert, "relation type")
	testutil.AssertEqual(t, subdomainArtifact.Relations[0].TargetID, certArtifact.ID, "relation target")
}
