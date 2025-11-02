// internal/platform/cveapi/providers/nvd.go
package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aethonx/internal/platform/cveapi"
	"aethonx/internal/platform/httpclient"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/rate"
)

// NVDProvider implements CVE enrichment using NIST NVD API 2.0.
type NVDProvider struct {
	baseURL     string
	apiKey      string
	client      *httpclient.Client
	rateLimiter *rate.Limiter
	logger      logx.Logger
}

// NVDConfig contains NVD provider configuration.
type NVDConfig struct {
	APIKey  string        // Optional - increases rate limit from 0.6 to 50 req/s
	BaseURL string        // Default: https://services.nvd.nist.gov/rest/json/cves/2.0
	Timeout time.Duration // Default: 10s
}

// NewNVDProvider creates a new NVD API provider.
func NewNVDProvider(config NVDConfig, logger logx.Logger) *NVDProvider {
	if config.BaseURL == "" {
		config.BaseURL = "https://services.nvd.nist.gov/rest/json/cves/2.0"
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	// Rate limit: 0.6 req/s without API key, 50 req/s with key
	rateLimit := 0.6
	if config.APIKey != "" {
		rateLimit = 50.0
	}

	httpConfig := httpclient.Config{
		Timeout:         config.Timeout,
		MaxRetries:      3,
		RetryBackoff:    2 * time.Second,
		MaxRetryBackoff: 10 * time.Second,
		UserAgent:       "AethonX CVE Enrichment (Educational)",
	}

	return &NVDProvider{
		baseURL:     config.BaseURL,
		apiKey:      config.APIKey,
		client:      httpclient.New(httpConfig, logger),
		rateLimiter: rate.New(rateLimit, 1),
		logger:      logger.With("provider", "nvd"),
	}
}

// Name returns the provider identifier.
func (n *NVDProvider) Name() string {
	return "nvd"
}

// RateLimit returns the provider's rate limit.
func (n *NVDProvider) RateLimit() float64 {
	if n.apiKey != "" {
		return 50.0
	}
	return 0.6
}

// HealthCheck verifies NVD API accessibility.
func (n *NVDProvider) HealthCheck() error {
	// Simple health check - try to fetch a known CVE
	_, err := n.Enrich("CVE-2021-44228") // Log4Shell
	return err
}

// Enrich fetches CVE data from NVD API 2.0.
func (n *NVDProvider) Enrich(cveID string) (*cveapi.EnrichedCVE, error) {
	// Build request URL
	url := fmt.Sprintf("%s?cveId=%s", n.baseURL, cveID)

	// Build headers
	headers := map[string]string{}
	if n.apiKey != "" {
		headers["apiKey"] = n.apiKey
	}

	n.logger.Debug("fetching CVE from NVD", "cve", cveID)

	// Execute request with rate limiter (context.Background for enrichment)
	ctx := context.Background()

	// Wait for rate limiter
	if err := n.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait failed: %w", err)
	}

	resp, err := n.client.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("nvd api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("nvd api returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var nvdResp nvdResponse
	if err := json.NewDecoder(resp.Body).Decode(&nvdResp); err != nil {
		return nil, fmt.Errorf("failed to parse nvd response: %w", err)
	}

	// Validate response
	if len(nvdResp.Vulnerabilities) == 0 {
		return nil, fmt.Errorf("cve not found in nvd: %s", cveID)
	}

	// Convert NVD format to EnrichedCVE
	return n.parseNVDResponse(&nvdResp.Vulnerabilities[0])
}

// parseNVDResponse converts NVD JSON response to EnrichedCVE.
func (n *NVDProvider) parseNVDResponse(vuln *nvdVulnerability) (*cveapi.EnrichedCVE, error) {
	cve := vuln.CVE

	enriched := &cveapi.EnrichedCVE{
		CVEID:            cve.ID,
		SourceIdentifier: cve.SourceIdentifier,
		VulnStatus:       cve.VulnStatus,
	}

	// Parse dates
	if published, err := time.Parse(time.RFC3339, cve.Published); err == nil {
		enriched.PublishedDate = published
	}
	if modified, err := time.Parse(time.RFC3339, cve.LastModified); err == nil {
		enriched.LastModifiedDate = modified
	}

	// Parse descriptions
	for _, desc := range cve.Descriptions {
		enriched.Descriptions = append(enriched.Descriptions, cveapi.Description{
			Lang:  desc.Lang,
			Value: desc.Value,
		})
	}

	// Parse CWEs
	for _, weakness := range cve.Weaknesses {
		for _, desc := range weakness.Description {
			if desc.Value != "NVD-CWE-noinfo" && desc.Value != "NVD-CWE-Other" {
				enriched.CWEs = append(enriched.CWEs, desc.Value)
			}
		}
	}

	// Parse CPEs
	for _, config := range cve.Configurations {
		for _, node := range config.Nodes {
			for _, match := range node.CPEMatch {
				if match.Criteria != "" {
					enriched.CPEs = append(enriched.CPEs, match.Criteria)
				}
			}
		}
	}

	// Parse CVSS metrics
	if cve.Metrics.CVSSMetricV2 != nil && len(cve.Metrics.CVSSMetricV2) > 0 {
		enriched.CVSSv2 = n.parseCVSSv2(&cve.Metrics.CVSSMetricV2[0])
	}
	if cve.Metrics.CVSSMetricV31 != nil && len(cve.Metrics.CVSSMetricV31) > 0 {
		enriched.CVSSv3 = n.parseCVSSv3(&cve.Metrics.CVSSMetricV31[0])
	} else if cve.Metrics.CVSSMetricV30 != nil && len(cve.Metrics.CVSSMetricV30) > 0 {
		enriched.CVSSv3 = n.parseCVSSv3(&cve.Metrics.CVSSMetricV30[0])
	}

	// Parse references
	for _, ref := range cve.References {
		enriched.References = append(enriched.References, cveapi.Reference{
			URL:    ref.URL,
			Source: ref.Source,
			Tags:   ref.Tags,
		})
	}

	return enriched, nil
}

// parseCVSSv2 converts NVD CVSS v2 to our format.
func (n *NVDProvider) parseCVSSv2(metric *nvdCVSSMetricV2) *cveapi.CVSSMetrics {
	data := metric.CVSSData

	return &cveapi.CVSSMetrics{
		Version:               "2.0",
		Vector:                data.VectorString,
		Score:                 data.BaseScore,
		AttackVector:          strings.ToUpper(data.AccessVector),
		AttackComplexity:      strings.ToUpper(data.AccessComplexity),
		ConfidentialityImpact: strings.ToUpper(data.ConfidentialityImpact),
		IntegrityImpact:       strings.ToUpper(data.IntegrityImpact),
		AvailabilityImpact:    strings.ToUpper(data.AvailabilityImpact),
		ExploitabilityScore:   metric.ExploitabilityScore,
		ImpactScore:           metric.ImpactScore,
		Severity:              deriveSeverityFromScore(data.BaseScore),
	}
}

// parseCVSSv3 converts NVD CVSS v3.x to our format.
func (n *NVDProvider) parseCVSSv3(metric *nvdCVSSMetricV3) *cveapi.CVSSMetrics {
	data := metric.CVSSData

	return &cveapi.CVSSMetrics{
		Version:               data.Version,
		Vector:                data.VectorString,
		Score:                 data.BaseScore,
		AttackVector:          data.AttackVector,
		AttackComplexity:      data.AttackComplexity,
		PrivilegesRequired:    data.PrivilegesRequired,
		UserInteraction:       data.UserInteraction,
		Scope:                 data.Scope,
		ConfidentialityImpact: data.ConfidentialityImpact,
		IntegrityImpact:       data.IntegrityImpact,
		AvailabilityImpact:    data.AvailabilityImpact,
		ExploitabilityScore:   metric.ExploitabilityScore,
		ImpactScore:           metric.ImpactScore,
		Severity:              data.BaseSeverity,
	}
}

// deriveSeverityFromScore calculates severity from CVSS score (for v2).
func deriveSeverityFromScore(score float64) string {
	switch {
	case score >= 9.0:
		return "CRITICAL"
	case score >= 7.0:
		return "HIGH"
	case score >= 4.0:
		return "MEDIUM"
	case score > 0.0:
		return "LOW"
	default:
		return "NONE"
	}
}

// NVD API 2.0 Response Structures

type nvdResponse struct {
	ResultsPerPage  int                 `json:"resultsPerPage"`
	StartIndex      int                 `json:"startIndex"`
	TotalResults    int                 `json:"totalResults"`
	Format          string              `json:"format"`
	Version         string              `json:"version"`
	Timestamp       string              `json:"timestamp"`
	Vulnerabilities []nvdVulnerability  `json:"vulnerabilities"`
}

type nvdVulnerability struct {
	CVE nvdCVE `json:"cve"`
}

type nvdCVE struct {
	ID               string            `json:"id"`
	SourceIdentifier string            `json:"sourceIdentifier"`
	Published        string            `json:"published"`
	LastModified     string            `json:"lastModified"`
	VulnStatus       string            `json:"vulnStatus"`
	Descriptions     []nvdDescription  `json:"descriptions"`
	Metrics          nvdMetrics        `json:"metrics"`
	Weaknesses       []nvdWeakness     `json:"weaknesses"`
	Configurations   []nvdConfiguration `json:"configurations"`
	References       []nvdReference    `json:"references"`
}

type nvdDescription struct {
	Lang  string `json:"lang"`
	Value string `json:"value"`
}

type nvdMetrics struct {
	CVSSMetricV2  []nvdCVSSMetricV2 `json:"cvssMetricV2"`
	CVSSMetricV30 []nvdCVSSMetricV3 `json:"cvssMetricV30"`
	CVSSMetricV31 []nvdCVSSMetricV3 `json:"cvssMetricV31"`
}

type nvdCVSSMetricV2 struct {
	Source                  string        `json:"source"`
	Type                    string        `json:"type"`
	CVSSData                nvdCVSSDataV2 `json:"cvssData"`
	BaseSeverity            string        `json:"baseSeverity"`
	ExploitabilityScore     float64       `json:"exploitabilityScore"`
	ImpactScore             float64       `json:"impactScore"`
	ACInsufInfo             bool          `json:"acInsufInfo"`
	ObtainAllPrivilege      bool          `json:"obtainAllPrivilege"`
	ObtainUserPrivilege     bool          `json:"obtainUserPrivilege"`
	ObtainOtherPrivilege    bool          `json:"obtainOtherPrivilege"`
	UserInteractionRequired bool          `json:"userInteractionRequired"`
}

type nvdCVSSDataV2 struct {
	Version               string  `json:"version"`
	VectorString          string  `json:"vectorString"`
	AccessVector          string  `json:"accessVector"`
	AccessComplexity      string  `json:"accessComplexity"`
	Authentication        string  `json:"authentication"`
	ConfidentialityImpact string  `json:"confidentialityImpact"`
	IntegrityImpact       string  `json:"integrityImpact"`
	AvailabilityImpact    string  `json:"availabilityImpact"`
	BaseScore             float64 `json:"baseScore"`
}

type nvdCVSSMetricV3 struct {
	Source              string        `json:"source"`
	Type                string        `json:"type"`
	CVSSData            nvdCVSSDataV3 `json:"cvssData"`
	ExploitabilityScore float64       `json:"exploitabilityScore"`
	ImpactScore         float64       `json:"impactScore"`
}

type nvdCVSSDataV3 struct {
	Version               string  `json:"version"`
	VectorString          string  `json:"vectorString"`
	AttackVector          string  `json:"attackVector"`
	AttackComplexity      string  `json:"attackComplexity"`
	PrivilegesRequired    string  `json:"privilegesRequired"`
	UserInteraction       string  `json:"userInteraction"`
	Scope                 string  `json:"scope"`
	ConfidentialityImpact string  `json:"confidentialityImpact"`
	IntegrityImpact       string  `json:"integrityImpact"`
	AvailabilityImpact    string  `json:"availabilityImpact"`
	BaseScore             float64 `json:"baseScore"`
	BaseSeverity          string  `json:"baseSeverity"`
}

type nvdWeakness struct {
	Source      string           `json:"source"`
	Type        string           `json:"type"`
	Description []nvdDescription `json:"description"`
}

type nvdConfiguration struct {
	Nodes []nvdNode `json:"nodes"`
}

type nvdNode struct {
	Operator string        `json:"operator"`
	Negate   bool          `json:"negate"`
	CPEMatch []nvdCPEMatch `json:"cpeMatch"`
}

type nvdCPEMatch struct {
	Vulnerable            bool   `json:"vulnerable"`
	Criteria              string `json:"criteria"`
	MatchCriteriaID       string `json:"matchCriteriaId"`
	VersionStartIncluding string `json:"versionStartIncluding,omitempty"`
	VersionEndIncluding   string `json:"versionEndIncluding,omitempty"`
	VersionStartExcluding string `json:"versionStartExcluding,omitempty"`
	VersionEndExcluding   string `json:"versionEndExcluding,omitempty"`
}

type nvdReference struct {
	URL    string   `json:"url"`
	Source string   `json:"source"`
	Tags   []string `json:"tags,omitempty"`
}
