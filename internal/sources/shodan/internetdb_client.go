// internal/sources/shodan/internetdb_client.go
package shodan

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"aethonx/internal/platform/errors"
	"aethonx/internal/platform/httpclient"
	"aethonx/internal/platform/logx"
)

const (
	internetDBBaseURL = "https://internetdb.shodan.io"
)

// InternetDBClient handles requests to Shodan's free InternetDB API.
// NO API KEY REQUIRED - Completely free for non-commercial use.
// Rate limit: 1 req/s (enforced by Shodan, temp ban if exceeded).
type InternetDBClient struct {
	client  httpclient.Client
	logger  logx.Logger
	baseURL string
}

// InternetDBResponse represents the response from InternetDB API.
type InternetDBResponse struct {
	IP        string   `json:"ip"`
	Ports     []int    `json:"ports"`
	CPEs      []string `json:"cpes"`
	Hostnames []string `json:"hostnames"`
	Tags      []string `json:"tags"`
	Vulns     []string `json:"vulns"`
}

// NewInternetDBClient creates a new InternetDB client.
// Rate limiting is critical: 1 req/s max to avoid temporary IP bans.
func NewInternetDBClient(logger logx.Logger) *InternetDBClient {
	// Configure with strict rate limiting (1 req/s to avoid temp bans)
	httpConfig := httpclient.Config{
		Timeout:         30 * time.Second,
		MaxRetries:      2,
		RetryBackoff:    1 * time.Second,
		MaxRetryBackoff: 5 * time.Second,
		UserAgent:       "AethonX/1.0",
		RateLimit:       1.0, // CRITICAL: 1 req/s max
		RateLimitBurst:  1,
	}

	return &InternetDBClient{
		client:  *httpclient.New(httpConfig, logger),
		logger:  logger.With("component", "internetdb"),
		baseURL: internetDBBaseURL,
	}
}

// LookupIP queries InternetDB for information about a specific IP.
// Returns nil response if IP has no data (not an error).
func (c *InternetDBClient) LookupIP(ctx context.Context, ip string) (*InternetDBResponse, error) {
	url := fmt.Sprintf("%s/%s", c.baseURL, ip)

	c.logger.Debug("querying internetdb", "ip", ip)

	body, err := c.client.FetchJSON(ctx, url)
	if err != nil {
		// 404 means no data for this IP (not an error)
		if errors.IsNotFound(err) {
			c.logger.Debug("no internetdb data for IP", "ip", ip)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query internetdb: %w", err)
	}

	var resp InternetDBResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse internetdb response: %w", err)
	}

	c.logger.Debug("internetdb lookup successful",
		"ip", resp.IP,
		"ports", len(resp.Ports),
		"cpes", len(resp.CPEs),
		"vulns", len(resp.Vulns),
	)

	return &resp, nil
}

// LookupBatch performs batch lookups for multiple IPs.
// Rate limiting is handled automatically by httpclient.
func (c *InternetDBClient) LookupBatch(ctx context.Context, ips []string) ([]*InternetDBResponse, error) {
	results := make([]*InternetDBResponse, 0, len(ips))

	for _, ip := range ips {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		resp, err := c.LookupIP(ctx, ip)
		if err != nil {
			c.logger.Warn("batch lookup failed for IP", "ip", ip, "error", err)
			continue
		}

		if resp != nil {
			results = append(results, resp)
		}
	}

	return results, nil
}
