// Package abuseipdb provides IP reputation enrichment via AbuseIPDB.com API.
// It queries the AbuseIPDB database to find IP reputation, abuse confidence score,
// and blacklist status for IP addresses.
package abuseipdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"time"

	"aethonx/internal/platform/cache"
	"aethonx/internal/platform/httpclient"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/rate"
)

const (
	apiBaseURL = "https://api.abuseipdb.com/api/v2/check"
	// Free tier: 1000 requests/day = ~0.011 req/s
	// Using conservative 0.01 req/s for safety
	defaultRateLimit = 0.01
	defaultTimeout   = 10 * time.Second
	cacheTTL         = 24 * time.Hour // 24 hours cache for reputation data
)

// AbuseIPDBClient handles communication with the AbuseIPDB.com API.
type AbuseIPDBClient struct {
	httpClient  *httpclient.Client
	cache       cache.Cache
	rateLimiter *rate.Limiter
	logger      logx.Logger
	apiKey      string // Required for AbuseIPDB API
}

// AbuseIPDBResponse represents the API response from AbuseIPDB.
type AbuseIPDBResponse struct {
	Data struct {
		IPAddress            string    `json:"ipAddress"`
		IsPublic             bool      `json:"isPublic"`
		IPVersion            int       `json:"ipVersion"`
		IsWhitelisted        bool      `json:"isWhitelisted"`
		AbuseConfidenceScore int       `json:"abuseConfidenceScore"` // 0-100
		CountryCode          string    `json:"countryCode"`
		CountryName          string    `json:"countryName"`
		UsageType            string    `json:"usageType"` // Data Center, ISP, etc.
		ISP                  string    `json:"isp"`
		Domain               string    `json:"domain"`
		Hostnames            []string  `json:"hostnames"`
		TotalReports         int       `json:"totalReports"`
		NumDistinctUsers     int       `json:"numDistinctUsers"`
		LastReportedAt       time.Time `json:"lastReportedAt"`
	} `json:"data"`
}

// NewAbuseIPDBClient creates a new AbuseIPDB API client.
func NewAbuseIPDBClient(logger logx.Logger, apiKey string, timeout time.Duration) *AbuseIPDBClient {
	if timeout == 0 {
		timeout = defaultTimeout
	}

	cfg := httpclient.Config{
		Timeout:    timeout,
		MaxRetries: 2,
		RateLimit:  0, // Handled internally via rateLimiter
		UserAgent:  "AethonX/1.0 (IP Reputation Intelligence)",
	}

	return &AbuseIPDBClient{
		httpClient:  httpclient.New(cfg, logger),
		cache:       cache.NewMemoryCache(1000), // Cache up to 1000 IP reputation lookups
		rateLimiter: rate.New(defaultRateLimit, 1),
		logger:      logger.With("component", "abuseipdb_client"),
		apiKey:      apiKey,
	}
}

// CheckIP queries AbuseIPDB for IP reputation data.
// Results are cached for 24 hours to reduce API calls.
func (c *AbuseIPDBClient) CheckIP(ctx context.Context, ip string) (*AbuseIPDBResponse, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("abuseipdb:%s", ip)
	if cached, found := c.cache.Get(cacheKey); found {
		c.logger.Debug("abuseipdb cache hit", "ip", ip)
		return cached.(*AbuseIPDBResponse), nil
	}

	// Rate limiting
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	c.logger.Debug("querying abuseipdb.com", "ip", ip)

	// Build request URL with query parameters
	reqURL := fmt.Sprintf("%s?ipAddress=%s&maxAgeInDays=90&verbose", apiBaseURL, url.QueryEscape(ip))

	// Build headers with API key
	headers := map[string]string{
		"Key":    c.apiKey,
		"Accept": "application/json",
	}

	// Make HTTP GET request
	resp, err := c.httpClient.Get(ctx, reqURL, headers)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("abuseipdb API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse JSON response
	var result AbuseIPDBResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Cache the result
	c.cache.Set(cacheKey, &result, cacheTTL)

	c.logger.Debug("abuseipdb query completed",
		"ip", ip,
		"abuse_score", result.Data.AbuseConfidenceScore,
		"reports", result.Data.TotalReports,
		"whitelisted", result.Data.IsWhitelisted,
	)

	return &result, nil
}

// Close cleans up resources.
func (c *AbuseIPDBClient) Close() error {
	// Cache cleanup happens automatically via TTL
	return nil
}
