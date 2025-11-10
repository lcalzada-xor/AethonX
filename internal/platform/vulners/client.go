// Package vulners provides vulnerability intelligence enrichment via Vulners.com API.
// It queries the Vulners database to find known CVEs for software products/versions.
package vulners

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"aethonx/internal/platform/cache"
	"aethonx/internal/platform/httpclient"
	"aethonx/internal/platform/logx"
)

const (
	apiBaseURL = "https://vulners.com/api/v3/burp/software/"
	// Free tier: 1000 requests/day
	defaultTimeout = 10 * time.Second
	cacheTTL       = 7 * 24 * time.Hour // 7 days cache for CVE results
)

// VulnersClient handles communication with the Vulners.com API.
type VulnersClient struct {
	httpClient *httpclient.Client
	cache      cache.Cache
	logger     logx.Logger
	apiKey     string // Optional, increases rate limit if provided
}

// VulnersRequest represents the API request payload.
type VulnersRequest struct {
	Software string `json:"software"` // e.g., "nginx"
	Version  string `json:"version"`  // e.g., "1.18.0"
	Type     string `json:"type"`     // Always "software"
}

// VulnersResponse represents the API response.
type VulnersResponse struct {
	Result string `json:"result"` // "OK" or "ERROR"
	Data   struct {
		Search []VulnerabilityItem `json:"search"`
		Total  int                 `json:"total"`
	} `json:"data"`
	Error string `json:"error"` // Error message if result == "ERROR"
}

// VulnerabilityItem represents a single vulnerability from the API.
type VulnerabilityItem struct {
	ID          string   `json:"_id"`          // "CVE-2021-23017"
	Type        string   `json:"type"`         // "cve"
	Title       string   `json:"title"`        // Vulnerability title
	Description string   `json:"description"`  // Full description
	CVSS        CVSS     `json:"cvss"`         // CVSS scoring
	Published   string   `json:"published"`    // ISO 8601 timestamp
	Modified    string   `json:"modified"`     // ISO 8601 timestamp
	CWE         []string `json:"cwe"`          // CWE identifiers
	References  []string `json:"references"`   // External references
	Exploit     bool     `json:"exploit"`      // Has public exploit
	Lastseen    string   `json:"lastseen"`     // Last seen timestamp
}

// CVSS represents CVSS scoring information.
type CVSS struct {
	Score   float64 `json:"score"`   // CVSS score (0.0-10.0)
	Vector  string  `json:"vector"`  // CVSS vector string
	Version string  `json:"version"` // CVSS version ("2.0" or "3.x")
}

// NewVulnersClient creates a new Vulners.com API client.
func NewVulnersClient(logger logx.Logger, apiKey string) *VulnersClient {
	cfg := httpclient.Config{
		Timeout:    defaultTimeout,
		MaxRetries: 2,
		UserAgent:  "AethonX/1.0 (Vulnerability Intelligence)",
	}

	return &VulnersClient{
		httpClient: httpclient.New(cfg, logger),
		cache:      cache.NewMemoryCache(1000), // Cache up to 1000 product:version combinations
		logger:     logger.With("component", "vulners_client"),
		apiKey:     apiKey,
	}
}

// SearchVulnerabilities queries Vulners.com for known CVEs affecting a software product/version.
// Results are cached for 7 days to reduce API calls.
func (c *VulnersClient) SearchVulnerabilities(ctx context.Context, product, version string) ([]CVEInfo, error) {
	// Normalize inputs
	product = normalizeProduct(product)
	version = normalizeVersion(version)

	if product == "" || version == "" {
		return nil, fmt.Errorf("product and version are required")
	}

	// Check cache first
	cacheKey := fmt.Sprintf("vulners:%s:%s", product, version)
	if cached, found := c.cache.Get(cacheKey); found {
		c.logger.Debug("vulners cache hit", "product", product, "version", version)
		return cached.([]CVEInfo), nil
	}

	c.logger.Debug("querying vulners.com", "product", product, "version", version)

	// Build request payload
	reqPayload := VulnersRequest{
		Software: product,
		Version:  version,
		Type:     "software",
	}

	reqBody, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make HTTP POST request
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Add API key if provided (increases rate limit)
	if c.apiKey != "" {
		headers["Authorization"] = fmt.Sprintf("api-key %s", c.apiKey)
	}

	resp, err := c.httpClient.Request(ctx, "POST", apiBaseURL, bytes.NewReader(reqBody), headers)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse JSON response
	var result VulnersResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API errors
	if result.Result != "OK" {
		return nil, fmt.Errorf("vulners API error: %s", result.Error)
	}

	// Parse vulnerabilities to CVEInfo
	cves := make([]CVEInfo, 0, len(result.Data.Search))
	for _, item := range result.Data.Search {
		cve := CVEInfo{
			ID:          item.ID,
			CVSS:        item.CVSS.Score,
			CVSSVector:  item.CVSS.Vector,
			CVSSVersion: item.CVSS.Version,
			Severity:    calculateSeverity(item.CVSS.Score),
			Description: item.Description,
			Published:   item.Published,
			Modified:    item.Modified,
			CWE:         item.CWE,
			References:  item.References,
			HasExploit:  item.Exploit,
		}
		cves = append(cves, cve)
	}

	// Cache results
	c.cache.Set(cacheKey, cves, cacheTTL)

	c.logger.Info("vulners query completed",
		"product", product,
		"version", version,
		"cves_found", len(cves),
	)

	return cves, nil
}

// Close cleans up resources (cache cleanup).
func (c *VulnersClient) Close() error {
	// Cache cleanup happens automatically via TTL
	return nil
}
