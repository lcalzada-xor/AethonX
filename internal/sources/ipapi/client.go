// internal/sources/ipapi/client.go
package ipapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"aethonx/internal/platform/httpclient"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/rate"
)

const (
	apiBaseURL = "http://ip-api.com/json"
	// Fields parameter optimizes response size and speed
	// 66846719 = query + status + message + country + countryCode + region + regionName +
	//            city + zip + lat + lon + timezone + isp + org + as
	fieldsParam = "66846719"
)

// IPAPIClient handles HTTP communication with ip-api.com.
type IPAPIClient struct {
	httpClient  *httpclient.Client
	rateLimiter *rate.Limiter
	logger      logx.Logger
}

// IPAPIResponse represents the JSON response from ip-api.com.
type IPAPIResponse struct {
	Status      string  `json:"status"`      // "success" or "fail"
	Country     string  `json:"country"`     // "United States"
	CountryCode string  `json:"countryCode"` // "US"
	Region      string  `json:"region"`      // "CA"
	RegionName  string  `json:"regionName"`  // "California"
	City        string  `json:"city"`        // "Mountain View"
	Zip         string  `json:"zip"`         // "94043"
	Lat         float64 `json:"lat"`         // 37.4192
	Lon         float64 `json:"lon"`         // -122.0574
	Timezone    string  `json:"timezone"`    // "America/Los_Angeles"
	ISP         string  `json:"isp"`         // "Google LLC"
	Org         string  `json:"org"`         // "Google Public DNS"
	AS          string  `json:"as"`          // "AS15169 Google LLC"
	Query       string  `json:"query"`       // "8.8.8.8" (the queried IP)
	Message     string  `json:"message"`     // Error message if status == "fail"
}

// NewIPAPIClient creates a new ip-api.com client with rate limiting.
// Rate limit: 45 requests/minute (free tier) = 0.75 req/s
// We use 0.7 req/s as a safe margin to avoid hitting the limit.
func NewIPAPIClient(logger logx.Logger, timeout time.Duration) *IPAPIClient {
	cfg := httpclient.Config{
		Timeout:    timeout,
		MaxRetries: 2,
		RateLimit:  0, // Rate limiting handled separately below
		UserAgent:  "AethonX/1.0 (IP Geolocation Enrichment)",
	}

	return &IPAPIClient{
		httpClient:  httpclient.New(cfg, logger),
		rateLimiter: rate.New(0.7, 1), // 0.7 req/s = 42 req/min (safe margin), burst=1
		logger:      logger.With("component", "ipapi_client"),
	}
}

// Lookup queries ip-api.com for geolocation and network information about an IP.
// It respects the rate limit and returns an error if the API returns a failure status.
func (c *IPAPIClient) Lookup(ctx context.Context, ip string) (*IPAPIResponse, error) {
	// Wait for rate limit token (blocking)
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait failed: %w", err)
	}

	// Build URL with fields parameter for optimized response
	url := fmt.Sprintf("%s/%s?fields=%s", apiBaseURL, ip, fieldsParam)

	c.logger.Debug("querying ip-api.com", "ip", ip, "url", url)

	// Make HTTP GET request
	resp, err := c.httpClient.Get(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse JSON response
	var result IPAPIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Check API status
	if result.Status == "fail" {
		return nil, fmt.Errorf("ip-api error: %s (ip: %s)", result.Message, ip)
	}

	c.logger.Debug("ip-api lookup successful",
		"ip", ip,
		"country", result.Country,
		"city", result.City,
		"isp", result.ISP,
	)

	return &result, nil
}

// IsPrivateIP checks if an IP address is in a private range.
// Private IP ranges: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 127.0.0.0/8
// ip-api.com rejects private IPs, so we filter them before making requests.
func IsPrivateIP(ip string) bool {
	// Simple heuristic check (not perfect but fast)
	// Full implementation would use net.ParseIP and check CIDR ranges
	if len(ip) == 0 {
		return true
	}

	// Check common private prefixes
	privateRanges := []string{
		"10.", "127.", "192.168.",
		"172.16.", "172.17.", "172.18.", "172.19.",
		"172.20.", "172.21.", "172.22.", "172.23.",
		"172.24.", "172.25.", "172.26.", "172.27.",
		"172.28.", "172.29.", "172.30.", "172.31.",
		"::1",        // IPv6 loopback
		"fe80::",     // IPv6 link-local
		"fc00::",     // IPv6 unique local
		"fd00::",     // IPv6 unique local
		"localhost",
	}

	for _, prefix := range privateRanges {
		if len(ip) >= len(prefix) && ip[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}
