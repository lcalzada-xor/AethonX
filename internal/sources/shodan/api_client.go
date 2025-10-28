// internal/sources/shodan/api_client.go
package shodan

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"aethonx/internal/platform/httpclient"
	"aethonx/internal/platform/logx"
)

const (
	// Shodan API base URL
	defaultBaseURL = "https://api.shodan.io"

	// API endpoints
	endpointHostInfo    = "/shodan/host/%s"           // /shodan/host/{ip}
	endpointHostSearch  = "/shodan/host/search"       // /shodan/host/search
	endpointDomainInfo  = "/dns/domain/%s"            // /dns/domain/{domain}
	endpointAPIInfo     = "/api-info"                 // /api-info (account info)
)

// ShodanAPIClient wraps the Shodan REST API.
type ShodanAPIClient struct {
	apiKey  string
	client  httpclient.Client
	logger  logx.Logger
	baseURL string
}

// NewAPIClient creates a new Shodan API client.
func NewAPIClient(apiKey string, logger logx.Logger) *ShodanAPIClient {
	// Configure HTTP client with Shodan-specific settings
	httpConfig := httpclient.Config{
		Timeout:         60 * time.Second,
		MaxRetries:      3,
		RetryBackoff:    2 * time.Second,
		MaxRetryBackoff: 30 * time.Second,
		UserAgent:       "AethonX/1.0 (Reconnaissance tool; +https://github.com/yourusername/aethonx)",
		RateLimit:       1.0, // 1 req/s (free tier default)
		RateLimitBurst:  1,
	}

	return &ShodanAPIClient{
		apiKey:  apiKey,
		client:  *httpclient.New(httpConfig, logger),
		logger:  logger.With("component", "shodan-api"),
		baseURL: defaultBaseURL,
	}
}

// NewAPIClientWithConfig creates a client with custom configuration.
func NewAPIClientWithConfig(apiKey string, logger logx.Logger, rateLimit float64, timeout time.Duration) *ShodanAPIClient {
	httpConfig := httpclient.Config{
		Timeout:         timeout,
		MaxRetries:      3,
		RetryBackoff:    2 * time.Second,
		MaxRetryBackoff: 30 * time.Second,
		UserAgent:       "AethonX/1.0",
		RateLimit:       rateLimit,
		RateLimitBurst:  1,
	}

	return &ShodanAPIClient{
		apiKey:  apiKey,
		client:  *httpclient.New(httpConfig, logger),
		logger:  logger.With("component", "shodan-api"),
		baseURL: defaultBaseURL,
	}
}

// GetDomainInfo fetches subdomains and DNS records via /dns/domain/{domain}.
// Requires at least Shodan Membership plan.
func (c *ShodanAPIClient) GetDomainInfo(ctx context.Context, domain string) ([]ShodanDomainResponse, error) {
	endpoint := fmt.Sprintf(endpointDomainInfo, domain)
	apiURL := c.buildURL(endpoint, nil)

	c.logger.Debug("fetching domain info",
		"domain", domain,
		"endpoint", endpoint,
	)

	body, err := c.client.FetchJSON(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch domain info: %w", err)
	}

	// Response can be either an object with "data" field or direct array
	// Try to parse as object first
	var wrapper struct {
		Data []ShodanDomainResponse `json:"data"`
	}

	if err := json.Unmarshal(body, &wrapper); err == nil && len(wrapper.Data) > 0 {
		c.logger.Debug("parsed domain info", "count", len(wrapper.Data))
		return wrapper.Data, nil
	}

	// Try direct array parsing
	var records []ShodanDomainResponse
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, fmt.Errorf("failed to parse domain response: %w", err)
	}

	c.logger.Debug("parsed domain info", "count", len(records))
	return records, nil
}

// GetHostInfo fetches detailed information about a specific IP address via /shodan/host/{ip}.
func (c *ShodanAPIClient) GetHostInfo(ctx context.Context, ip string) (*ShodanHostResponse, error) {
	endpoint := fmt.Sprintf(endpointHostInfo, ip)
	apiURL := c.buildURL(endpoint, nil)

	c.logger.Debug("fetching host info", "ip", ip)

	body, err := c.client.FetchJSON(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch host info: %w", err)
	}

	var host ShodanHostResponse
	if err := json.Unmarshal(body, &host); err != nil {
		return nil, fmt.Errorf("failed to parse host response: %w", err)
	}

	c.logger.Debug("parsed host info",
		"ip", host.IPStr,
		"ports", len(host.Hostnames),
	)

	return &host, nil
}

// SearchHosts searches Shodan database via /shodan/host/search.
// Query examples:
//   - "hostname:example.com"
//   - "org:\"Example Corp\""
//   - "net:93.184.216.0/24"
func (c *ShodanAPIClient) SearchHosts(ctx context.Context, query string) ([]ShodanHostResponse, error) {
	params := map[string]string{
		"query": query,
	}

	apiURL := c.buildURL(endpointHostSearch, params)

	c.logger.Debug("searching hosts", "query", query)

	body, err := c.client.FetchJSON(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search hosts: %w", err)
	}

	var searchResp ShodanSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	c.logger.Debug("parsed search results",
		"query", query,
		"total", searchResp.Total,
		"matches", len(searchResp.Matches),
	)

	return searchResp.Matches, nil
}

// SearchHostsPaginated searches with pagination support.
// Returns up to 'maxResults' results by paginating through pages.
func (c *ShodanAPIClient) SearchHostsPaginated(ctx context.Context, query string, maxResults int) ([]ShodanHostResponse, error) {
	const resultsPerPage = 100 // Shodan default page size
	var allMatches []ShodanHostResponse

	page := 1
	for len(allMatches) < maxResults {
		params := map[string]string{
			"query": query,
			"page":  fmt.Sprintf("%d", page),
		}

		apiURL := c.buildURL(endpointHostSearch, params)

		c.logger.Debug("searching hosts (paginated)",
			"query", query,
			"page", page,
			"collected", len(allMatches),
		)

		body, err := c.client.FetchJSON(ctx, apiURL)
		if err != nil {
			// Return partial results on error
			c.logger.Warn("pagination failed, returning partial results",
				"error", err.Error(),
				"collected", len(allMatches),
			)
			return allMatches, nil
		}

		var searchResp ShodanSearchResponse
		if err := json.Unmarshal(body, &searchResp); err != nil {
			return allMatches, fmt.Errorf("failed to parse search response: %w", err)
		}

		if len(searchResp.Matches) == 0 {
			break // No more results
		}

		allMatches = append(allMatches, searchResp.Matches...)

		// Check if we've reached the end
		if len(allMatches) >= searchResp.Total || len(searchResp.Matches) < resultsPerPage {
			break
		}

		page++
	}

	// Trim to maxResults
	if len(allMatches) > maxResults {
		allMatches = allMatches[:maxResults]
	}

	c.logger.Info("search completed",
		"query", query,
		"total_matches", len(allMatches),
	)

	return allMatches, nil
}

// GetAPIInfo fetches account information (credits, plan, etc.).
func (c *ShodanAPIClient) GetAPIInfo(ctx context.Context) (map[string]interface{}, error) {
	apiURL := c.buildURL(endpointAPIInfo, nil)

	c.logger.Debug("fetching API info")

	body, err := c.client.FetchJSON(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API info: %w", err)
	}

	var info map[string]interface{}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse API info: %w", err)
	}

	return info, nil
}

// buildURL constructs the full API URL with authentication and parameters.
func (c *ShodanAPIClient) buildURL(endpoint string, params map[string]string) string {
	u, _ := url.Parse(c.baseURL + endpoint)
	q := u.Query()

	// Add API key
	q.Set("key", c.apiKey)

	// Add additional parameters
	for key, value := range params {
		q.Set(key, value)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

// ValidateAPIKey checks if the API key is valid by making a test request.
func (c *ShodanAPIClient) ValidateAPIKey(ctx context.Context) error {
	c.logger.Debug("validating API key")

	_, err := c.GetAPIInfo(ctx)
	if err != nil {
		return fmt.Errorf("invalid API key: %w", err)
	}

	c.logger.Info("API key validated successfully")
	return nil
}
