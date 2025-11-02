// internal/platform/cveapi/providers/circl.go
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
)

// CirclProvider implements CVE enrichment using CVE.circl.lu API.
// This is a free, community-maintained alternative to NVD with no rate limits.
type CirclProvider struct {
	baseURL string
	client  *httpclient.Client
	logger  logx.Logger
}

// CirclConfig contains CVE.circl.lu provider configuration.
type CirclConfig struct {
	BaseURL string        // Default: https://cve.circl.lu/api
	Timeout time.Duration // Default: 5s
}

// NewCirclProvider creates a new CVE.circl.lu provider.
func NewCirclProvider(config CirclConfig, logger logx.Logger) *CirclProvider {
	if config.BaseURL == "" {
		config.BaseURL = "https://cve.circl.lu/api"
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}

	httpConfig := httpclient.Config{
		Timeout:         config.Timeout,
		MaxRetries:      2,
		RetryBackoff:    1 * time.Second,
		MaxRetryBackoff: 5 * time.Second,
		UserAgent:       "AethonX CVE Enrichment (Educational)",
	}

	return &CirclProvider{
		baseURL: config.BaseURL,
		client:  httpclient.New(httpConfig, logger),
		logger:  logger.With("provider", "circl"),
	}
}

// Name returns the provider identifier.
func (c *CirclProvider) Name() string {
	return "circl"
}

// RateLimit returns the provider's rate limit (no limit for circl).
func (c *CirclProvider) RateLimit() float64 {
	return 1000.0 // Effectively unlimited
}

// HealthCheck verifies circl API accessibility.
func (c *CirclProvider) HealthCheck() error {
	url := fmt.Sprintf("%s/cve/CVE-2021-44228", c.baseURL)
	ctx := context.Background()
	resp, err := c.client.Get(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("circl health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("circl returned status %d", resp.StatusCode)
	}

	return nil
}

// Enrich fetches CVE data from circl API.
func (c *CirclProvider) Enrich(cveID string) (*cveapi.EnrichedCVE, error) {
	url := fmt.Sprintf("%s/cve/%s", c.baseURL, cveID)

	c.logger.Debug("fetching CVE from circl", "cve", cveID)

	ctx := context.Background()
	resp, err := c.client.Get(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("circl api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("cve not found in circl: %s", cveID)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("circl api returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var circlResp circlResponse
	if err := json.NewDecoder(resp.Body).Decode(&circlResp); err != nil {
		return nil, fmt.Errorf("failed to parse circl response: %w", err)
	}

	// Convert circl format to EnrichedCVE
	return c.parseCirclResponse(&circlResp)
}

// parseCirclResponse converts circl JSON response to EnrichedCVE.
func (c *CirclProvider) parseCirclResponse(resp *circlResponse) (*cveapi.EnrichedCVE, error) {
	enriched := &cveapi.EnrichedCVE{
		CVEID: resp.ID,
	}

	// Parse published date
	if resp.Published != "" {
		if published, err := time.Parse("2006-01-02T15:04:05", resp.Published); err == nil {
			enriched.PublishedDate = published
		}
	}

	// Parse last modified date
	if resp.LastModified != "" {
		if modified, err := time.Parse("2006-01-02T15:04:05", resp.LastModified); err == nil {
			enriched.LastModifiedDate = modified
		}
	}

	// Parse summary as description
	if resp.Summary != "" {
		enriched.Descriptions = []cveapi.Description{
			{
				Lang:  "en",
				Value: resp.Summary,
			},
		}
	}

	// Parse CWE
	if resp.CWE != "" {
		enriched.CWEs = []string{resp.CWE}
	}

	// Parse CVSS v2
	if resp.CVSS != 0 {
		enriched.CVSSv2 = &cveapi.CVSSMetrics{
			Version:  "2.0",
			Score:    resp.CVSS,
			Severity: deriveSeverityFromScore(resp.CVSS),
		}

		// Parse access vector, complexity from circl if available
		if resp.Access != nil {
			if resp.Access.Vector != "" {
				enriched.CVSSv2.AttackVector = strings.ToUpper(resp.Access.Vector)
			}
			if resp.Access.Complexity != "" {
				enriched.CVSSv2.AttackComplexity = strings.ToUpper(resp.Access.Complexity)
			}
		}

		// Parse impact
		if resp.Impact != nil {
			if resp.Impact.Confidentiality != "" {
				enriched.CVSSv2.ConfidentialityImpact = strings.ToUpper(resp.Impact.Confidentiality)
			}
			if resp.Impact.Integrity != "" {
				enriched.CVSSv2.IntegrityImpact = strings.ToUpper(resp.Impact.Integrity)
			}
			if resp.Impact.Availability != "" {
				enriched.CVSSv2.AvailabilityImpact = strings.ToUpper(resp.Impact.Availability)
			}
		}
	}

	// Parse CVSS v3
	if resp.CVSSv3 != 0 {
		enriched.CVSSv3 = &cveapi.CVSSMetrics{
			Version:  "3.1",
			Score:    resp.CVSSv3,
			Severity: deriveSeverityFromScore(resp.CVSSv3),
		}
	}

	// Parse references
	if len(resp.References) > 0 {
		for _, ref := range resp.References {
			enriched.References = append(enriched.References, cveapi.Reference{
				URL: ref,
			})
		}
	}

	// Parse vulnerable configurations (CPE)
	if len(resp.VulnerableConfiguration) > 0 {
		enriched.CPEs = resp.VulnerableConfiguration
	}

	return enriched, nil
}

// Circl API Response Structures

type circlResponse struct {
	ID                       string         `json:"id"`
	Published                string         `json:"Published"`
	LastModified             string         `json:"last-modified"`
	CVSS                     float64        `json:"cvss"`
	CVSSv3                   float64        `json:"cvss-vector-v3,omitempty"`
	CVSSTime                 string         `json:"cvss-time,omitempty"`
	CWE                      string         `json:"cwe,omitempty"`
	Summary                  string         `json:"summary"`
	VulnerableConfiguration  []string       `json:"vulnerable_configuration,omitempty"`
	VulnerableProduct        []string       `json:"vulnerable_product,omitempty"`
	References               []string       `json:"references,omitempty"`
	Access                   *circlAccess   `json:"access,omitempty"`
	Impact                   *circlImpact   `json:"impact,omitempty"`
	VectorString             string         `json:"vector_string,omitempty"`
	Assigner                 string         `json:"assigner,omitempty"`
}

type circlAccess struct {
	Vector        string `json:"vector,omitempty"`
	Complexity    string `json:"complexity,omitempty"`
	Authentication string `json:"authentication,omitempty"`
}

type circlImpact struct {
	Confidentiality string `json:"confidentiality,omitempty"`
	Integrity       string `json:"integrity,omitempty"`
	Availability    string `json:"availability,omitempty"`
}
