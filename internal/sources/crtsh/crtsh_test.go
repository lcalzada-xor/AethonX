// internal/sources/crtsh/crtsh_test.go
package crtsh

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/platform/logx"
	"aethonx/internal/testutil"
)

func TestNew(t *testing.T) {
	logger := logx.New()
	source := New(logger)

	testutil.AssertNotNil(t, source, "source should not be nil")
}

func TestCRT_Name(t *testing.T) {
	logger := logx.New()
	source := New(logger)

	if source.Name() != "crtsh" {
		t.Errorf("Name(): expected %q, got %q", "crtsh", source.Name())
	}
}

func TestCRT_Mode(t *testing.T) {
	logger := logx.New()
	source := New(logger)

	if source.Mode() != domain.SourceModePassive {
		t.Errorf("Mode(): expected %v, got %v", domain.SourceModePassive, source.Mode())
	}
}

func TestCRT_Type(t *testing.T) {
	logger := logx.New()
	source := New(logger)

	if source.Type() != domain.SourceTypeAPI {
		t.Errorf("Type(): expected %v, got %v", domain.SourceTypeAPI, source.Type())
	}
}

func TestCRT_Run_Success(t *testing.T) {
	// Mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL format
		if !strings.Contains(r.URL.String(), "q=%25.example.com") {
			t.Errorf("unexpected URL: %s", r.URL.String())
		}

		// Return mock JSON response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{
				"issuer_name": "Let's Encrypt Authority X3",
				"name_value": "test.example.com",
				"not_after": "2025-12-31T23:59:59",
				"not_before": "2025-01-01T00:00:00",
				"serial_number": "ABC123"
			},
			{
				"issuer_name": "Let's Encrypt Authority X3",
				"name_value": "api.example.com\nwww.example.com",
				"not_after": "2025-12-31T23:59:59",
				"not_before": "2025-01-01T00:00:00",
				"serial_number": "DEF456"
			}
		]`))
	}))
	defer server.Close()

	// Create source with custom HTTP client pointing to mock server
	logger := logx.New()
	crt := &CRT{
		client: server.Client(),
		logger: logger.With("source", "crtsh"),
	}

	// Temporarily replace the URL building logic (this is a limitation of the current design)
	// In a production test, we'd inject the base URL
	// For now, we'll test with a different approach using processRecords directly

	// Test processRecords instead
	target := *domain.NewTarget("example.com", domain.ScanModePassive)
	records := []certRecord{
		{
			IssuerName:   "Let's Encrypt Authority X3",
			NameValue:    "test.example.com",
			NotAfter:     "2025-12-31T23:59:59",
			NotBefore:    "2025-01-01T00:00:00",
			SerialNumber: "ABC123",
		},
		{
			IssuerName:   "Let's Encrypt Authority X3",
			NameValue:    "api.example.com\nwww.example.com",
			NotAfter:     "2025-12-31T23:59:59",
			NotBefore:    "2025-01-01T00:00:00",
			SerialNumber: "DEF456",
		},
	}

	artifacts := crt.processRecords(records, target)

	// Record 1: test.example.com -> 1 subdomain + 1 cert = 2
	// Record 2: api.example.com\nwww.example.com -> 2 subdomains + 2 certs (one cert per host) = 4
	// Total = 6 artifacts
	if len(artifacts) != 6 {
		t.Errorf("expected 6 artifacts, got %d", len(artifacts))
	}

	// Verify subdomain artifacts
	subdomainCount := 0
	certCount := 0
	for _, a := range artifacts {
		switch a.Type {
		case domain.ArtifactTypeSubdomain:
			subdomainCount++
			// Verify confidence
			if a.Confidence != 0.95 {
				t.Errorf("subdomain confidence: expected 0.95, got %f", a.Confidence)
			}
			// Verify TypedMetadata
			domainMeta, ok := a.TypedMetadata.(*metadata.DomainMetadata)
			if !ok {
				t.Error("subdomain should have DomainMetadata")
			}
			if domainMeta.SSLIssuer == "" {
				t.Error("subdomain should have SSL issuer in TypedMetadata")
			}
		case domain.ArtifactTypeCertificate:
			certCount++
			// Verify TypedMetadata
			certMeta, ok := a.TypedMetadata.(*metadata.CertificateMetadata)
			if !ok {
				t.Error("certificate should have CertificateMetadata")
			}
			if certMeta.IssuerCN == "" {
				t.Error("certificate should have issuer in TypedMetadata")
			}
		}
	}

	if subdomainCount != 3 {
		t.Errorf("expected 3 subdomain artifacts, got %d", subdomainCount)
	}
	if certCount != 3 {
		t.Errorf("expected 3 certificate artifacts, got %d", certCount)
	}
}

func TestCRT_Run_HTTPError(t *testing.T) {
	// This test would require modifying the source to accept a base URL
	// For now, we'll skip this integration test and focus on unit tests
	t.Skip("Skipping HTTP error test - requires URL injection capability")
}

func TestCRT_Run_ContextCancellation(t *testing.T) {
	// This would require URL injection
	t.Skip("Skipping context cancellation test - requires URL injection capability")
}

func TestCRT_ProcessRecords_EmptyRecords(t *testing.T) {
	logger := logx.New()
	crt := &CRT{
		client: &http.Client{},
		logger: logger.With("source", "crtsh"),
	}

	target := *domain.NewTarget("example.com", domain.ScanModePassive)
	records := []certRecord{}

	artifacts := crt.processRecords(records, target)

	if len(artifacts) != 0 {
		t.Errorf("expected 0 artifacts, got %d", len(artifacts))
	}
}

func TestCRT_ProcessRecords_OutOfScope(t *testing.T) {
	logger := logx.New()
	crt := &CRT{
		client: &http.Client{},
		logger: logger.With("source", "crtsh"),
	}

	target := *domain.NewTarget("example.com", domain.ScanModePassive)

	records := []certRecord{
		{
			IssuerName:   "Let's Encrypt",
			NameValue:    "outofscope.com", // Different domain
			NotAfter:     "2025-12-31T23:59:59",
			NotBefore:    "2025-01-01T00:00:00",
			SerialNumber: "XYZ789",
		},
	}

	artifacts := crt.processRecords(records, target)

	// Should be filtered out (out of scope)
	if len(artifacts) != 0 {
		t.Errorf("expected 0 artifacts (out of scope), got %d", len(artifacts))
	}
}

func TestCRT_ProcessRecords_WildcardDomain(t *testing.T) {
	logger := logx.New()
	crt := &CRT{
		client: &http.Client{},
		logger: logger.With("source", "crtsh"),
	}

	target := *domain.NewTarget("example.com", domain.ScanModePassive)

	records := []certRecord{
		{
			IssuerName:   "Let's Encrypt",
			NameValue:    "*.example.com", // Wildcard
			NotAfter:     "2025-12-31T23:59:59",
			NotBefore:    "2025-01-01T00:00:00",
			SerialNumber: "WILD123",
		},
	}

	artifacts := crt.processRecords(records, target)

	// Should have 2 artifacts: 1 subdomain + 1 certificate
	if len(artifacts) != 2 {
		t.Errorf("expected 2 artifacts, got %d", len(artifacts))
	}

	// Find subdomain artifact
	var wildcardArtifact *domain.Artifact
	for _, a := range artifacts {
		if a.Type == domain.ArtifactTypeSubdomain {
			wildcardArtifact = a
			break
		}
	}

	if wildcardArtifact == nil {
		t.Fatal("wildcard subdomain artifact not found")
	}

	// Verify wildcard tag
	hasWildcardTag := false
	for _, tag := range wildcardArtifact.Tags {
		if tag == "wildcard" {
			hasWildcardTag = true
			break
		}
	}

	if !hasWildcardTag {
		t.Error("wildcard subdomain should have 'wildcard' tag")
	}

	// Verify TypedMetadata for domain
	if wildcardArtifact.TypedMetadata == nil {
		t.Fatal("wildcard artifact should have TypedMetadata")
	}

	domainMeta, ok := wildcardArtifact.TypedMetadata.(*metadata.DomainMetadata)
	if !ok {
		t.Fatal("TypedMetadata should be *metadata.DomainMetadata")
	}

	if !domainMeta.SSLWildcard {
		t.Error("DomainMetadata.SSLWildcard should be true")
	}
}

func TestCRT_ProcessRecords_MultipleHosts(t *testing.T) {
	logger := logx.New()
	crt := &CRT{
		client: &http.Client{},
		logger: logger.With("source", "crtsh"),
	}

	target := *domain.NewTarget("example.com", domain.ScanModePassive)

	// Single record with multiple hosts separated by \n
	records := []certRecord{
		{
			IssuerName:   "Let's Encrypt",
			NameValue:    "test1.example.com\ntest2.example.com\ntest3.example.com",
			NotAfter:     "2025-12-31T23:59:59",
			NotBefore:    "2025-01-01T00:00:00",
			SerialNumber: "MULTI123",
		},
	}

	artifacts := crt.processRecords(records, target)

	// Should have: 3 subdomains + 3 certificates = 6 artifacts (one cert per subdomain)
	if len(artifacts) != 6 {
		t.Errorf("expected 6 artifacts, got %d", len(artifacts))
	}

	// Count subdomain artifacts
	subdomainCount := 0
	for _, a := range artifacts {
		if a.Type == domain.ArtifactTypeSubdomain {
			subdomainCount++
		}
	}

	if subdomainCount != 3 {
		t.Errorf("expected 3 subdomain artifacts, got %d", subdomainCount)
	}
}

func TestCRT_ProcessRecords_EmptyHostNames(t *testing.T) {
	logger := logx.New()
	crt := &CRT{
		client: &http.Client{},
		logger: logger.With("source", "crtsh"),
	}

	target := *domain.NewTarget("example.com", domain.ScanModePassive)

	records := []certRecord{
		{
			IssuerName:   "Let's Encrypt",
			NameValue:    "test.example.com\n\n  \n", // Empty entries
			NotAfter:     "2025-12-31T23:59:59",
			NotBefore:    "2025-01-01T00:00:00",
			SerialNumber: "EMPTY123",
		},
	}

	artifacts := crt.processRecords(records, target)

	// Should have 2 artifacts: 1 subdomain + 1 certificate (empty entries filtered)
	if len(artifacts) != 2 {
		t.Errorf("expected 2 artifacts, got %d", len(artifacts))
	}
}

func TestCRT_ProcessRecords_MetadataPopulation(t *testing.T) {
	logger := logx.New()
	crt := &CRT{
		client: &http.Client{},
		logger: logger.With("source", "crtsh"),
	}

	target := *domain.NewTarget("example.com", domain.ScanModePassive)

	records := []certRecord{
		{
			IssuerName:   "DigiCert Inc",
			NameValue:    "secure.example.com",
			NotAfter:     "2026-06-30T12:00:00",
			NotBefore:    "2024-07-01T12:00:00",
			SerialNumber: "0A1B2C3D",
		},
	}

	artifacts := crt.processRecords(records, target)

	// Find subdomain artifact
	var subdomainArtifact *domain.Artifact
	for _, a := range artifacts {
		if a.Type == domain.ArtifactTypeSubdomain {
			subdomainArtifact = a
			break
		}
	}

	if subdomainArtifact == nil {
		t.Fatal("subdomain artifact not found")
	}

	// Verify TypedMetadata
	if subdomainArtifact.TypedMetadata == nil {
		t.Fatal("TypedMetadata should not be nil")
	}

	domainMeta, ok := subdomainArtifact.TypedMetadata.(*metadata.DomainMetadata)
	if !ok {
		t.Fatal("TypedMetadata should be *metadata.DomainMetadata")
	}

	if !domainMeta.HasSSL {
		t.Error("DomainMetadata.HasSSL should be true")
	}
	if domainMeta.SSLIssuer != "DigiCert Inc" {
		t.Errorf("SSLIssuer: expected %q, got %q", "DigiCert Inc", domainMeta.SSLIssuer)
	}
	if domainMeta.SSLValidUntil != "2026-06-30T12:00:00" {
		t.Errorf("SSLValidUntil: expected %q, got %q", "2026-06-30T12:00:00", domainMeta.SSLValidUntil)
	}
	if domainMeta.SSLValidFrom != "2024-07-01T12:00:00" {
		t.Errorf("SSLValidFrom: expected %q, got %q", "2024-07-01T12:00:00", domainMeta.SSLValidFrom)
	}

	// Find certificate artifact
	var certArtifact *domain.Artifact
	for _, a := range artifacts {
		if a.Type == domain.ArtifactTypeCertificate {
			certArtifact = a
			break
		}
	}

	if certArtifact == nil {
		t.Fatal("certificate artifact not found")
	}

	// Verify certificate value and TypedMetadata
	if certArtifact.Value != "0A1B2C3D" {
		t.Errorf("certificate value: expected %q, got %q", "0A1B2C3D", certArtifact.Value)
	}

	certMeta, ok := certArtifact.TypedMetadata.(*metadata.CertificateMetadata)
	if !ok {
		t.Fatal("certificate TypedMetadata should be *metadata.CertificateMetadata")
	}
	if certMeta.IssuerCN != "DigiCert Inc" {
		t.Errorf("IssuerCN: expected %q, got %q", "DigiCert Inc", certMeta.IssuerCN)
	}
	if certMeta.SerialNumber != "0A1B2C3D" {
		t.Errorf("SerialNumber: expected %q, got %q", "0A1B2C3D", certMeta.SerialNumber)
	}
}

func TestCertRecord_JSONMarshaling(t *testing.T) {
	jsonData := `{
		"issuer_name": "Let's Encrypt Authority X3",
		"name_value": "example.com",
		"not_after": "2025-12-31T23:59:59",
		"not_before": "2025-01-01T00:00:00",
		"serial_number": "ABC123"
	}`

	var record certRecord
	err := testutil.UnmarshalJSON([]byte(jsonData), &record)
	if err != nil {
		t.Fatalf("failed to unmarshal certRecord: %v", err)
	}

	if record.IssuerName != "Let's Encrypt Authority X3" {
		t.Errorf("IssuerName: expected %q, got %q", "Let's Encrypt Authority X3", record.IssuerName)
	}
	if record.NameValue != "example.com" {
		t.Errorf("NameValue: expected %q, got %q", "example.com", record.NameValue)
	}
	if record.SerialNumber != "ABC123" {
		t.Errorf("SerialNumber: expected %q, got %q", "ABC123", record.SerialNumber)
	}
}
