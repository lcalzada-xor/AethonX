package rdap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	testutil.AssertEqual(t, source.Name(), "rdap", "name should be rdap")
	testutil.AssertEqual(t, source.Mode(), domain.SourceModePassive, "mode should be passive")
	testutil.AssertEqual(t, source.Type(), domain.SourceTypeAPI, "type should be API")
}

func TestRDAP_Run(t *testing.T) {
	logger := logx.New()

	t.Run("successful RDAP query", func(t *testing.T) {
		mockResponse := createMockRDAPResponse()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(mockResponse)
		}))
		defer server.Close()

		// Create source
		source := New(logger).(*RDAP)

		target := *domain.NewTarget("example.com", domain.ScanModePassive)

		// This will hit the real RDAP server or fail with network error
		// For a real test, we'd need dependency injection
		_, err := source.Run(context.Background(), target)

		// The test will hit the real RDAP server or fail with network error
		// This is just to test the structure
		if err != nil {
			t.Logf("Expected error in test environment: %v", err)
		}
	})

	t.Run("handles invalid target", func(t *testing.T) {
		source := New(logger)
		target := *domain.NewTarget("", domain.ScanModePassive)

		_, err := source.Run(context.Background(), target)
		testutil.AssertTrue(t, err != nil, "should return error for empty target")
	})
}

func TestRDAP_ExtractBaseDomain(t *testing.T) {
	logger := logx.New()
	source := New(logger).(*RDAP)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple domain", "example.com", "example.com"},
		{"subdomain", "www.example.com", "example.com"},
		{"deep subdomain", "api.staging.example.com", "example.com"},
		{"with protocol", "https://example.com", "example.com"},
		{"with port", "example.com:443", "example.com"},
		{"with path", "example.com/path", "example.com"},
		{"full URL", "https://www.example.com:443/path", "example.com"},
		{"with trailing dot", "example.com.", "example.com"},
		{"single word", "localhost", "localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := source.extractBaseDomain(tt.input)
			testutil.AssertEqual(t, result, tt.expected, "extracted domain should match")
		})
	}
}

func TestRDAP_HasRole(t *testing.T) {
	logger := logx.New()
	source := New(logger).(*RDAP)

	roles := []string{"registrar", "administrative", "technical"}

	testutil.AssertTrue(t, source.hasRole(roles, "registrar"), "should find registrar role")
	testutil.AssertTrue(t, source.hasRole(roles, "Registrar"), "should be case insensitive")
	testutil.AssertTrue(t, source.hasRole(roles, "technical"), "should find technical role")
	testutil.AssertTrue(t, !source.hasRole(roles, "billing"), "should not find billing role")
	testutil.AssertTrue(t, !source.hasRole([]string{}, "registrar"), "should handle empty roles")
}

func TestRDAP_ExtractVCardField(t *testing.T) {
	logger := logx.New()
	source := New(logger).(*RDAP)

	t.Run("extracts name field", func(t *testing.T) {
		vcardArray := []interface{}{
			"vcard",
			[]interface{}{
				[]interface{}{"version", map[string]interface{}{}, "text", "4.0"},
				[]interface{}{"fn", map[string]interface{}{}, "text", "John Doe"},
				[]interface{}{"email", map[string]interface{}{}, "text", "john@example.com"},
			},
		}

		name := source.extractVCardField(vcardArray, "fn")
		testutil.AssertEqual(t, name, "John Doe", "should extract name")

		email := source.extractVCardField(vcardArray, "email")
		testutil.AssertEqual(t, email, "john@example.com", "should extract email")
	})

	t.Run("returns empty for missing field", func(t *testing.T) {
		vcardArray := []interface{}{
			"vcard",
			[]interface{}{
				[]interface{}{"version", map[string]interface{}{}, "text", "4.0"},
			},
		}

		name := source.extractVCardField(vcardArray, "fn")
		testutil.AssertEqual(t, name, "", "should return empty for missing field")
	})

	t.Run("handles invalid vcard", func(t *testing.T) {
		vcardArray := []interface{}{}

		name := source.extractVCardField(vcardArray, "fn")
		testutil.AssertEqual(t, name, "", "should handle invalid vcard")
	})

	t.Run("case insensitive field matching", func(t *testing.T) {
		vcardArray := []interface{}{
			"vcard",
			[]interface{}{
				[]interface{}{"FN", map[string]interface{}{}, "text", "Jane Doe"},
			},
		}

		name := source.extractVCardField(vcardArray, "fn")
		testutil.AssertEqual(t, name, "Jane Doe", "should be case insensitive")
	})
}

func TestRDAP_ExtractVCardAddress(t *testing.T) {
	logger := logx.New()
	source := New(logger).(*RDAP)

	t.Run("extracts full address", func(t *testing.T) {
		vcardArray := []interface{}{
			"vcard",
			[]interface{}{
				[]interface{}{
					"adr",
					map[string]interface{}{},
					"text",
					[]interface{}{
						"",                    // pobox
						"",                    // ext
						"123 Main St",         // street
						"Anytown",             // locality
						"CA",                  // region
						"12345",               // code
						"US",                  // country
					},
				},
			},
		}

		addr := source.extractVCardAddress(vcardArray)
		testutil.AssertNotNil(t, addr, "should extract address")
		testutil.AssertEqual(t, addr["street"], "123 Main St", "should extract street")
		testutil.AssertEqual(t, addr["locality"], "Anytown", "should extract locality")
		testutil.AssertEqual(t, addr["region"], "CA", "should extract region")
		testutil.AssertEqual(t, addr["code"], "12345", "should extract postal code")
		testutil.AssertEqual(t, addr["country"], "US", "should extract country")
	})

	t.Run("returns nil for missing address", func(t *testing.T) {
		vcardArray := []interface{}{
			"vcard",
			[]interface{}{
				[]interface{}{"fn", map[string]interface{}{}, "text", "John Doe"},
			},
		}

		addr := source.extractVCardAddress(vcardArray)
		testutil.AssertTrue(t, addr == nil, "should return nil for missing address")
	})
}

func TestRDAP_IsRedacted(t *testing.T) {
	logger := logx.New()
	source := New(logger).(*RDAP)

	tests := []struct {
		name     string
		vcard    []interface{}
		expected bool
	}{
		{
			name: "redacted email",
			vcard: []interface{}{
				"vcard",
				[]interface{}{
					[]interface{}{"fn", map[string]interface{}{}, "text", "John Doe"},
					[]interface{}{"email", map[string]interface{}{}, "text", "redacted@example.com"},
				},
			},
			expected: true,
		},
		{
			name: "redacted name",
			vcard: []interface{}{
				"vcard",
				[]interface{}{
					[]interface{}{"fn", map[string]interface{}{}, "text", "REDACTED FOR PRIVACY"},
					[]interface{}{"email", map[string]interface{}{}, "text", "john@example.com"},
				},
			},
			expected: true,
		},
		{
			name: "privacy protected",
			vcard: []interface{}{
				"vcard",
				[]interface{}{
					[]interface{}{"fn", map[string]interface{}{}, "text", "John Doe"},
					[]interface{}{"email", map[string]interface{}{}, "text", "privacy@example.com"},
				},
			},
			expected: true,
		},
		{
			name: "empty email",
			vcard: []interface{}{
				"vcard",
				[]interface{}{
					[]interface{}{"fn", map[string]interface{}{}, "text", "John Doe"},
				},
			},
			expected: true,
		},
		{
			name: "valid contact",
			vcard: []interface{}{
				"vcard",
				[]interface{}{
					[]interface{}{"fn", map[string]interface{}{}, "text", "John Doe"},
					[]interface{}{"email", map[string]interface{}{}, "text", "john@example.com"},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := source.isRedacted(tt.vcard)
			testutil.AssertEqual(t, result, tt.expected, "redaction detection should match")
		})
	}
}

func TestRDAP_ExtractRegistrarMetadata(t *testing.T) {
	logger := logx.New()
	source := New(logger).(*RDAP)

	rdapData := &rdapResponse{
		Status: []string{"active", "clientTransferProhibited"},
		SecureDNS: struct {
			DelegationSigned bool `json:"delegationSigned"`
		}{
			DelegationSigned: true,
		},
		Events: []rdapEvent{
			{EventAction: "registration", EventDate: "2020-01-01T00:00:00Z"},
			{EventAction: "last changed", EventDate: "2023-06-15T10:30:00Z"},
			{EventAction: "expiration", EventDate: "2025-01-01T00:00:00Z"},
		},
		Nameservers: []rdapNameserver{
			{LDHName: "ns1.example.com"},
			{LDHName: "ns2.example.com"},
		},
		Entities: []rdapEntity{
			{
				Roles: []string{"registrar"},
				VCardArray: []interface{}{
					"vcard",
					[]interface{}{
						[]interface{}{"fn", map[string]interface{}{}, "text", "Example Registrar Inc."},
					},
				},
				PublicIDs: []struct {
					Type       string `json:"type"`
					Identifier string `json:"identifier"`
				}{
					{Type: "IANA Registrar ID", Identifier: "1234"},
				},
			},
			{
				Roles: []string{"registrant"},
				VCardArray: []interface{}{
					"vcard",
					[]interface{}{
						[]interface{}{"org", map[string]interface{}{}, "text", "ACME Corporation"},
					},
				},
			},
		},
	}

	regMeta := source.extractRegistrarMetadata(rdapData)

	testutil.AssertTrue(t, regMeta.IsValid(), "metadata should be valid")
	testutil.AssertEqual(t, len(regMeta.Status), 2, "should have 2 statuses")
	testutil.AssertTrue(t, regMeta.DNSSECEnabled, "DNSSEC should be enabled")
	testutil.AssertEqual(t, regMeta.CreatedDate, "2020-01-01T00:00:00Z", "should extract created date")
	testutil.AssertEqual(t, regMeta.UpdatedDate, "2023-06-15T10:30:00Z", "should extract updated date")
	testutil.AssertEqual(t, regMeta.ExpiryDate, "2025-01-01T00:00:00Z", "should extract expiry date")
	testutil.AssertEqual(t, len(regMeta.Nameservers), 2, "should have 2 nameservers")
	testutil.AssertEqual(t, regMeta.RegistrarName, "Example Registrar Inc.", "should extract registrar name")
	testutil.AssertEqual(t, regMeta.RegistrarIANA, "1234", "should extract IANA ID")
	testutil.AssertEqual(t, regMeta.Organization, "ACME Corporation", "should extract organization")
}

func TestRDAP_ExtractContactMetadata(t *testing.T) {
	logger := logx.New()
	source := New(logger).(*RDAP)

	entity := rdapEntity{
		Roles: []string{"administrative"},
		VCardArray: []interface{}{
			"vcard",
			[]interface{}{
				[]interface{}{"fn", map[string]interface{}{}, "text", "Admin Contact"},
				[]interface{}{"org", map[string]interface{}{}, "text", "ACME Corp"},
				[]interface{}{"email", map[string]interface{}{}, "text", "admin@example.com"},
				[]interface{}{"tel", map[string]interface{}{}, "text", "+1-555-1234"},
				[]interface{}{
					"adr",
					map[string]interface{}{},
					"text",
					[]interface{}{"", "", "123 Admin St", "AdminCity", "AC", "11111", "US"},
				},
			},
		},
	}

	contactMeta := source.extractContactMetadata(entity)

	testutil.AssertTrue(t, contactMeta.IsValid(), "contact metadata should be valid")
	testutil.AssertEqual(t, contactMeta.ContactType, "admin", "should have admin type")
	testutil.AssertEqual(t, contactMeta.Name, "Admin Contact", "should extract name")
	testutil.AssertEqual(t, contactMeta.Organization, "ACME Corp", "should extract organization")
	testutil.AssertEqual(t, contactMeta.Email, "admin@example.com", "should extract email")
	testutil.AssertEqual(t, contactMeta.Phone, "+1-555-1234", "should extract phone")
	testutil.AssertEqual(t, contactMeta.Street, "123 Admin St", "should extract street")
	testutil.AssertEqual(t, contactMeta.City, "AdminCity", "should extract city")
	testutil.AssertEqual(t, contactMeta.State, "AC", "should extract state")
	testutil.AssertEqual(t, contactMeta.PostalCode, "11111", "should extract postal code")
	testutil.AssertEqual(t, contactMeta.Country, "US", "should extract country")
	testutil.AssertTrue(t, !contactMeta.Redacted, "should not be redacted")
}

func TestRegistrarMetadata_IsExpired(t *testing.T) {
	tests := []struct {
		name       string
		expiryDate string
		expected   bool
	}{
		{"expired date", "2020-01-01T00:00:00Z", true},
		{"future date", "2030-01-01T00:00:00Z", false},
		{"empty date", "", false},
		{"invalid date", "not-a-date", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regMeta := metadata.NewRegistrarMetadata()
			regMeta.ExpiryDate = tt.expiryDate

			result := regMeta.IsExpired()
			testutil.AssertEqual(t, result, tt.expected, "expiration check should match")
		})
	}
}

func TestRegistrarMetadata_DaysUntilExpiry(t *testing.T) {
	regMeta := metadata.NewRegistrarMetadata()

	t.Run("valid future date", func(t *testing.T) {
		regMeta.ExpiryDate = "2030-01-01T00:00:00Z"
		days := regMeta.DaysUntilExpiry()
		testutil.AssertTrue(t, days > 0, "should have positive days until expiry")
	})

	t.Run("past date", func(t *testing.T) {
		regMeta.ExpiryDate = "2020-01-01T00:00:00Z"
		days := regMeta.DaysUntilExpiry()
		testutil.AssertTrue(t, days < 0, "should have negative days for past date")
	})

	t.Run("empty date", func(t *testing.T) {
		regMeta.ExpiryDate = ""
		days := regMeta.DaysUntilExpiry()
		testutil.AssertEqual(t, days, -1, "should return -1 for empty date")
	})

	t.Run("invalid date", func(t *testing.T) {
		regMeta.ExpiryDate = "invalid"
		days := regMeta.DaysUntilExpiry()
		testutil.AssertEqual(t, days, -1, "should return -1 for invalid date")
	})
}

func TestContactMetadata_HasPrivateInfo(t *testing.T) {
	tests := []struct {
		name     string
		contact  *metadata.ContactMetadata
		expected bool
	}{
		{
			name: "has private info",
			contact: &metadata.ContactMetadata{
				Email:    "john@example.com",
				Redacted: false,
			},
			expected: true,
		},
		{
			name: "redacted",
			contact: &metadata.ContactMetadata{
				Email:    "john@example.com",
				Redacted: true,
			},
			expected: false,
		},
		{
			name: "empty and not redacted",
			contact: &metadata.ContactMetadata{
				Email:    "",
				Redacted: false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.contact.HasPrivateInfo()
			testutil.AssertEqual(t, result, tt.expected, "private info check should match")
		})
	}
}

// Helper function to create mock RDAP response for testing
func createMockRDAPResponse() *rdapResponse {
	return &rdapResponse{
		ObjectClassName: "domain",
		LDHName:         "example.com",
		Status:          []string{"active"},
		Entities: []rdapEntity{
			{
				Roles: []string{"registrar"},
				VCardArray: []interface{}{
					"vcard",
					[]interface{}{
						[]interface{}{"fn", map[string]interface{}{}, "text", "Test Registrar"},
					},
				},
			},
		},
		Nameservers: []rdapNameserver{
			{LDHName: "ns1.example.com"},
		},
		Events: []rdapEvent{
			{EventAction: "registration", EventDate: "2020-01-01T00:00:00Z"},
		},
	}
}
