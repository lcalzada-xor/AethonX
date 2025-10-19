// Package rdap implements a RDAP (Registration Data Access Protocol) source for domain reconnaissance.
// It queries RDAP servers to retrieve domain registration information including registrar details,
// contact information, nameservers, and important dates.
package rdap

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/cache"
	"aethonx/internal/platform/errors"
	"aethonx/internal/platform/httpx"
	"aethonx/internal/platform/logx"
)

const (
	// RDAP bootstrap service for automatic server discovery
	rdapBootstrapURL = "https://rdap.org/domain/%s"

	// Cache TTL for RDAP responses (24 hours)
	cacheTTL = 24 * time.Hour

	// Source name
	sourceName = "rdap"
)

// RDAP implements the ports.Source interface for RDAP queries
type RDAP struct {
	client httpx.Client
	cache  cache.Cache
	logger logx.Logger
}

// rdapResponse representa la respuesta de RDAP (simplificada)
type rdapResponse struct {
	ObjectClassName string   `json:"objectClassName"`
	Handle          string   `json:"handle"`
	LDHName         string   `json:"ldhName"` // Domain name
	Status          []string `json:"status"`

	// Entities (contacts, registrar)
	Entities []rdapEntity `json:"entities"`

	// Nameservers
	Nameservers []rdapNameserver `json:"nameservers"`

	// Events (created, updated, expiry)
	Events []rdapEvent `json:"events"`

	// DNSSEC
	SecureDNS struct {
		DelegationSigned bool `json:"delegationSigned"`
	} `json:"secureDNS"`

	// Links
	Links []rdapLink `json:"links"`
}

// rdapEntity representa una entidad (registrar, contacto)
type rdapEntity struct {
	ObjectClassName string   `json:"objectClassName"`
	Handle          string   `json:"handle"`
	Roles           []string `json:"roles"` // registrar, registrant, admin, tech, billing

	// Contact info (VCard)
	VCardArray []interface{} `json:"vcardArray"`

	// Public IDs
	PublicIDs []struct {
		Type       string `json:"type"`
		Identifier string `json:"identifier"`
	} `json:"publicIds"`

	// Nested entities
	Entities []rdapEntity `json:"entities"`
}

// rdapNameserver representa un nameserver
type rdapNameserver struct {
	ObjectClassName string `json:"objectClassName"`
	LDHName         string `json:"ldhName"`
}

// rdapEvent representa un evento (created, updated, expiration)
type rdapEvent struct {
	EventAction string `json:"eventAction"` // registration, last changed, expiration
	EventDate   string `json:"eventDate"`
}

// rdapLink representa un link relacionado
type rdapLink struct {
	Value string `json:"value"`
	Rel   string `json:"rel"`
	Href  string `json:"href"`
	Type  string `json:"type"`
}

// New creates a new RDAP source
func New(logger logx.Logger) ports.Source {
	// Create HTTP client with retry and rate limiting
	httpConfig := httpx.Config{
		Timeout:         30 * time.Second,
		MaxRetries:      3,
		RetryBackoff:    1 * time.Second,
		MaxRetryBackoff: 10 * time.Second,
		UserAgent:       "AethonX/1.0 RDAP Client",
		RateLimit:       5,  // 5 requests per second
		RateLimitBurst:  2,
	}

	return &RDAP{
		client: *httpx.New(httpConfig, logger),
		cache:  cache.NewMemoryCache(1000), // Cache up to 1000 domains
		logger: logger.With("source", sourceName),
	}
}

// Name implements ports.Source
func (r *RDAP) Name() string {
	return sourceName
}

// Mode implements ports.Source
func (r *RDAP) Mode() domain.SourceMode {
	return domain.SourceModePassive
}

// Type implements ports.Source
func (r *RDAP) Type() domain.SourceType {
	return domain.SourceTypeAPI
}

// Run implements ports.Source
func (r *RDAP) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	result := domain.NewScanResult(target)

	r.logger.Info("Starting RDAP query",
		"target", target.Root,
		"mode", target.Mode,
	)

	// Extract base domain from target
	domainName := r.extractBaseDomain(target.Root)
	if domainName == "" {
		return result, errors.New("invalid target: could not extract domain name")
	}

	// Check cache first
	cacheKey := fmt.Sprintf("rdap:%s", domainName)
	if cached, found := r.cache.Get(cacheKey); found {
		r.logger.Debug("RDAP response found in cache", "domain", domainName)
		cachedResult, ok := cached.(*domain.ScanResult)
		if ok {
			return cachedResult, nil
		}
	}

	// Query RDAP server
	rdapData, err := r.queryRDAP(ctx, domainName)
	if err != nil {
		r.logger.Warn("RDAP query failed",
			"domain", domainName,
			"error", err.Error(),
		)
		return result, errors.Wrapf(err, "RDAP query failed for %s", domainName)
	}

	// Extract artifacts from RDAP response
	r.extractArtifacts(result, rdapData, domainName)

	// Cache result
	r.cache.Set(cacheKey, result, cacheTTL)

	r.logger.Info("RDAP query completed",
		"domain", domainName,
		"artifacts", len(result.Artifacts),
	)

	return result, nil
}

// queryRDAP performs the RDAP query
func (r *RDAP) queryRDAP(ctx context.Context, domain string) (*rdapResponse, error) {
	// Use rdap.org bootstrap service for automatic server discovery
	url := fmt.Sprintf(rdapBootstrapURL, domain)

	r.logger.Debug("Querying RDAP server",
		"domain", domain,
		"url", url,
	)

	// Fetch JSON response
	body, err := r.client.FetchJSON(ctx, url)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.Wrapf(err, "domain not found in RDAP: %s", domain)
		}
		if errors.IsRateLimit(err) {
			return nil, errors.Wrap(err, "RDAP rate limit exceeded")
		}
		return nil, errors.Wrap(err, "failed to fetch RDAP data")
	}

	// Parse response
	var rdapData rdapResponse
	if err := json.Unmarshal(body, &rdapData); err != nil {
		return nil, errors.Wrapf(err, "failed to parse RDAP response for %s", domain)
	}

	return &rdapData, nil
}

// extractArtifacts extracts artifacts from RDAP response
func (r *RDAP) extractArtifacts(result *domain.ScanResult, rdapData *rdapResponse, domainName string) {
	// Create registrar metadata
	regMeta := r.extractRegistrarMetadata(rdapData)

	// Add domain artifact with registrar metadata
	if regMeta.IsValid() {
		domainArtifact := domain.NewArtifactWithMetadata(
			domain.ArtifactTypeDomain,
			domainName,
			sourceName,
			regMeta,
		)
		domainArtifact.Confidence = 1.0
		result.AddArtifact(domainArtifact)
	}

	// Extract nameservers
	for _, ns := range rdapData.Nameservers {
		if ns.LDHName != "" {
			nsArtifact := domain.NewArtifact(
				domain.ArtifactTypeNameserver,
				ns.LDHName,
				sourceName,
			)
			nsArtifact.Confidence = 1.0
			result.AddArtifact(nsArtifact)
		}
	}

	// Extract emails and contacts
	r.extractContacts(result, rdapData.Entities)
}

// extractRegistrarMetadata creates RegistrarMetadata from RDAP response
func (r *RDAP) extractRegistrarMetadata(rdapData *rdapResponse) *metadata.RegistrarMetadata {
	regMeta := metadata.NewRegistrarMetadata()

	// Status
	regMeta.Status = rdapData.Status

	// DNSSEC
	regMeta.DNSSECEnabled = rdapData.SecureDNS.DelegationSigned

	// Events (created, updated, expiry dates)
	for _, event := range rdapData.Events {
		switch strings.ToLower(event.EventAction) {
		case "registration":
			regMeta.CreatedDate = event.EventDate
		case "last changed", "last update of rdap database":
			regMeta.UpdatedDate = event.EventDate
		case "expiration":
			regMeta.ExpiryDate = event.EventDate
		}
	}

	// Nameservers
	for _, ns := range rdapData.Nameservers {
		if ns.LDHName != "" {
			regMeta.Nameservers = append(regMeta.Nameservers, ns.LDHName)
		}
	}

	// Extract registrar info from entities
	for _, entity := range rdapData.Entities {
		if r.hasRole(entity.Roles, "registrar") {
			// Extract registrar name from VCard
			if name := r.extractVCardField(entity.VCardArray, "fn"); name != "" {
				regMeta.RegistrarName = name
			}

			// Extract IANA ID
			for _, pubID := range entity.PublicIDs {
				if pubID.Type == "IANA Registrar ID" {
					regMeta.RegistrarIANA = pubID.Identifier
				}
			}
		}

		// Extract organization from registrant
		if r.hasRole(entity.Roles, "registrant") {
			if org := r.extractVCardField(entity.VCardArray, "org"); org != "" {
				regMeta.Organization = org
			}
		}
	}

	return regMeta
}

// extractContacts extracts contact information from entities
func (r *RDAP) extractContacts(result *domain.ScanResult, entities []rdapEntity) {
	for _, entity := range entities {
		// Extract emails
		if email := r.extractVCardField(entity.VCardArray, "email"); email != "" {
			emailArtifact := domain.NewArtifact(
				domain.ArtifactTypeEmail,
				email,
				sourceName,
			)
			emailArtifact.Confidence = 0.95 // High confidence for RDAP emails

			// Add contact metadata
			contactMeta := r.extractContactMetadata(entity)
			if contactMeta.IsValid() {
				emailArtifact.TypedMetadata = contactMeta
			}

			result.AddArtifact(emailArtifact)
		}

		// Recursively process nested entities
		if len(entity.Entities) > 0 {
			r.extractContacts(result, entity.Entities)
		}
	}
}

// extractContactMetadata creates ContactMetadata from entity
func (r *RDAP) extractContactMetadata(entity rdapEntity) *metadata.ContactMetadata {
	// Determine contact type from roles
	contactType := "unknown"
	for _, role := range entity.Roles {
		switch strings.ToLower(role) {
		case "registrant":
			contactType = "registrant"
		case "administrative":
			contactType = "admin"
		case "technical":
			contactType = "tech"
		case "billing":
			contactType = "billing"
		}
	}

	contactMeta := metadata.NewContactMetadata(contactType)

	// Extract VCard fields
	contactMeta.Name = r.extractVCardField(entity.VCardArray, "fn")
	contactMeta.Organization = r.extractVCardField(entity.VCardArray, "org")
	contactMeta.Email = r.extractVCardField(entity.VCardArray, "email")
	contactMeta.Phone = r.extractVCardField(entity.VCardArray, "tel")

	// Extract address
	if addr := r.extractVCardAddress(entity.VCardArray); addr != nil {
		contactMeta.Street = addr["street"]
		contactMeta.City = addr["locality"]
		contactMeta.State = addr["region"]
		contactMeta.PostalCode = addr["code"]
		contactMeta.Country = addr["country"]
	}

	// Check if redacted
	contactMeta.Redacted = r.isRedacted(entity.VCardArray)

	return contactMeta
}

// extractVCardField extracts a specific field from VCard array
func (r *RDAP) extractVCardField(vcardArray []interface{}, fieldName string) string {
	if len(vcardArray) < 2 {
		return ""
	}

	// VCard format: ["vcard", [["version", {}, "text", "4.0"], ["fn", {}, "text", "John Doe"], ...]]
	vcard, ok := vcardArray[1].([]interface{})
	if !ok {
		return ""
	}

	for _, item := range vcard {
		field, ok := item.([]interface{})
		if !ok || len(field) < 4 {
			continue
		}

		name, ok := field[0].(string)
		if !ok || !strings.EqualFold(name, fieldName) {
			continue
		}

		// Value is at index 3
		value, ok := field[3].(string)
		if ok {
			return value
		}
	}

	return ""
}

// extractVCardAddress extracts address from VCard array
func (r *RDAP) extractVCardAddress(vcardArray []interface{}) map[string]string {
	if len(vcardArray) < 2 {
		return nil
	}

	vcard, ok := vcardArray[1].([]interface{})
	if !ok {
		return nil
	}

	for _, item := range vcard {
		field, ok := item.([]interface{})
		if !ok || len(field) < 4 {
			continue
		}

		name, ok := field[0].(string)
		if !ok || !strings.EqualFold(name, "adr") {
			continue
		}

		// Address value is at index 3 and is an array
		addrValue, ok := field[3].([]interface{})
		if !ok || len(addrValue) < 7 {
			continue
		}

		addr := make(map[string]string)

		// Address format: [pobox, ext, street, locality, region, code, country]
		if street, ok := addrValue[2].(string); ok {
			addr["street"] = street
		}
		if locality, ok := addrValue[3].(string); ok {
			addr["locality"] = locality
		}
		if region, ok := addrValue[4].(string); ok {
			addr["region"] = region
		}
		if code, ok := addrValue[5].(string); ok {
			addr["code"] = code
		}
		if country, ok := addrValue[6].(string); ok {
			addr["country"] = country
		}

		return addr
	}

	return nil
}

// isRedacted checks if VCard data is redacted
func (r *RDAP) isRedacted(vcardArray []interface{}) bool {
	// Check for common redaction markers
	email := r.extractVCardField(vcardArray, "email")
	name := r.extractVCardField(vcardArray, "fn")

	return strings.Contains(strings.ToLower(email), "redacted") ||
		strings.Contains(strings.ToLower(name), "redacted") ||
		strings.Contains(strings.ToLower(email), "privacy") ||
		email == ""
}

// hasRole checks if entity has a specific role
func (r *RDAP) hasRole(roles []string, role string) bool {
	for _, r := range roles {
		if strings.EqualFold(r, role) {
			return true
		}
	}
	return false
}

// extractBaseDomain extracts the base domain from a target value
// Example: subdomain.example.com -> example.com
func (r *RDAP) extractBaseDomain(target string) string {
	// Remove protocol if present
	target = strings.TrimPrefix(target, "http://")
	target = strings.TrimPrefix(target, "https://")

	// Remove port if present
	if idx := strings.Index(target, ":"); idx != -1 {
		target = target[:idx]
	}

	// Remove path if present
	if idx := strings.Index(target, "/"); idx != -1 {
		target = target[:idx]
	}

	// Remove trailing dot
	target = strings.TrimSuffix(target, ".")

	// Split by dots
	parts := strings.Split(target, ".")
	if len(parts) < 2 {
		return target
	}

	// Return last two parts (domain.tld)
	// This is a simplification - doesn't handle .co.uk, etc.
	return strings.Join(parts[len(parts)-2:], ".")
}
