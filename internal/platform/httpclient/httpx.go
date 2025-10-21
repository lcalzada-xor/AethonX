// Package httpclient provides an enhanced HTTP client with retry, rate limiting, and timeout support.
package httpclient

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"aethonx/internal/platform/errors"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/rate"
)

// Client is an enhanced HTTP client with retry logic, rate limiting, and timeout support.
type Client struct {
	httpClient  *http.Client
	rateLimiter *rate.Limiter
	logger      logx.Logger
	config      Config
}

// Config holds the configuration for the HTTP client.
type Config struct {
	// Timeout is the request timeout duration.
	// Default: 30 seconds
	Timeout time.Duration

	// MaxRetries is the maximum number of retry attempts.
	// Default: 3
	MaxRetries int

	// RetryBackoff is the initial backoff duration for retries.
	// Backoff increases exponentially with each retry.
	// Default: 1 second
	RetryBackoff time.Duration

	// MaxRetryBackoff is the maximum backoff duration between retries.
	// Default: 30 seconds
	MaxRetryBackoff time.Duration

	// UserAgent is the User-Agent header value.
	// Default: "AethonX/1.0"
	UserAgent string

	// RateLimit is the maximum requests per second.
	// 0 means no rate limiting.
	// Default: 0 (no limit)
	RateLimit float64

	// RateLimitBurst is the burst size for rate limiting.
	// Default: 1
	RateLimitBurst int
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Timeout:          30 * time.Second,
		MaxRetries:       3,
		RetryBackoff:     1 * time.Second,
		MaxRetryBackoff:  30 * time.Second,
		UserAgent:        "AethonX/1.0",
		RateLimit:        0,
		RateLimitBurst:   1,
	}
}

// New creates a new HTTP client with the given configuration.
func New(config Config, logger logx.Logger) *Client {
	// Apply defaults for zero values
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.RetryBackoff == 0 {
		config.RetryBackoff = 1 * time.Second
	}
	if config.MaxRetryBackoff == 0 {
		config.MaxRetryBackoff = 30 * time.Second
	}
	if config.UserAgent == "" {
		config.UserAgent = "AethonX/1.0"
	}
	if config.RateLimitBurst == 0 {
		config.RateLimitBurst = 1
	}

	httpClient := &http.Client{
		Timeout: config.Timeout,
	}

	var rateLimiter *rate.Limiter
	if config.RateLimit > 0 {
		rateLimiter = rate.New(config.RateLimit, config.RateLimitBurst)
	}

	return &Client{
		httpClient:  httpClient,
		rateLimiter: rateLimiter,
		logger:      logger.With("component", "httpx"),
		config:      config,
	}
}

// Request performs an HTTP request with retry logic and rate limiting.
func (c *Client) Request(ctx context.Context, method, url string, body io.Reader, headers map[string]string) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		// Rate limiting
		if c.rateLimiter != nil {
			if err := c.rateLimiter.Wait(ctx); err != nil {
				return nil, errors.Wrap(err, "rate limit wait failed")
			}
		}

		// Create request
		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create request for %s %s", method, url)
		}

		// Set headers
		req.Header.Set("User-Agent", c.config.UserAgent)
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		// Log request
		c.logger.Debug("HTTP request",
			"method", method,
			"url", url,
			"attempt", attempt+1,
			"max_retries", c.config.MaxRetries+1,
		)

		// Perform request
		start := time.Now()
		resp, err := c.httpClient.Do(req)
		duration := time.Since(start)

		// Log response
		if err != nil {
			c.logger.Warn("HTTP request failed",
				"method", method,
				"url", url,
				"attempt", attempt+1,
				"error", err.Error(),
				"duration_ms", duration.Milliseconds(),
			)
			lastErr = err

			// Check if we should retry
			if !c.shouldRetry(attempt, err, nil) {
				return nil, errors.Wrapf(err, "request failed after %d attempts", attempt+1)
			}

			// Backoff before retry
			if err := c.backoff(ctx, attempt); err != nil {
				return nil, errors.Wrap(err, "backoff interrupted")
			}
			continue
		}

		// Log successful response
		c.logger.Debug("HTTP response received",
			"method", method,
			"url", url,
			"status", resp.StatusCode,
			"duration_ms", duration.Milliseconds(),
		)

		// Check if this is a retryable status code
		isRetryableStatus := c.isRetryableStatus(resp)

		// If not a retryable status, return the response
		if !isRetryableStatus {
			return resp, nil
		}

		// Check if we can retry
		if !c.shouldRetry(attempt, nil, resp) {
			// Max retries exhausted, close body and fall through to return error
			resp.Body.Close()
			lastErr = errors.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
			break
		}

		// Close response body before retry
		resp.Body.Close()

		lastErr = errors.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		c.logger.Warn("HTTP request returned retryable status",
			"method", method,
			"url", url,
			"status", resp.StatusCode,
			"attempt", attempt+1,
		)

		// Backoff before retry
		if err := c.backoff(ctx, attempt); err != nil {
			return nil, errors.Wrap(err, "backoff interrupted")
		}
	}

	return nil, errors.Wrapf(lastErr, "request failed after %d attempts", c.config.MaxRetries+1)
}

// Get performs a GET request.
func (c *Client) Get(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	return c.Request(ctx, http.MethodGet, url, nil, headers)
}

// Post performs a POST request.
func (c *Client) Post(ctx context.Context, url string, body io.Reader, headers map[string]string) (*http.Response, error) {
	return c.Request(ctx, http.MethodPost, url, body, headers)
}

// isRetryableStatus checks if an HTTP status code should trigger a retry.
func (c *Client) isRetryableStatus(resp *http.Response) bool {
	if resp == nil {
		return false
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests: // 429
		return true
	case http.StatusServiceUnavailable: // 503
		return true
	case http.StatusGatewayTimeout: // 504
		return true
	case http.StatusBadGateway: // 502
		return true
	default:
		return false
	}
}

// shouldRetry determines if a request should be retried based on the attempt number,
// error, and response status code.
func (c *Client) shouldRetry(attempt int, err error, resp *http.Response) bool {
	// No more retries if max attempts reached
	if attempt >= c.config.MaxRetries {
		return false
	}

	// Retry on network errors
	if err != nil {
		return true
	}

	// Retry on specific HTTP status codes
	return c.isRetryableStatus(resp)
}

// backoff implements exponential backoff with jitter.
func (c *Client) backoff(ctx context.Context, attempt int) error {
	// Calculate backoff duration with exponential increase
	backoff := c.config.RetryBackoff * time.Duration(math.Pow(2, float64(attempt)))

	// Cap at max backoff
	if backoff > c.config.MaxRetryBackoff {
		backoff = c.config.MaxRetryBackoff
	}

	c.logger.Debug("Backing off before retry",
		"attempt", attempt+1,
		"backoff_ms", backoff.Milliseconds(),
	)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(backoff):
		return nil
	}
}

// SetRateLimit updates the rate limit dynamically.
func (c *Client) SetRateLimit(rps float64, burst int) {
	if rps <= 0 {
		c.rateLimiter = nil
		return
	}

	if c.rateLimiter == nil {
		c.rateLimiter = rate.New(rps, burst)
	} else {
		c.rateLimiter.SetRate(rps)
		c.rateLimiter.SetBurst(burst)
	}

	c.logger.Info("Rate limit updated",
		"rps", rps,
		"burst", burst,
	)
}

// GetJSON is a convenience method for GET requests that expect JSON responses.
func (c *Client) GetJSON(ctx context.Context, url string) (*http.Response, error) {
	headers := map[string]string{
		"Accept": "application/json",
	}
	return c.Get(ctx, url, headers)
}

// PostJSON is a convenience method for POST requests with JSON body.
func (c *Client) PostJSON(ctx context.Context, url string, body io.Reader) (*http.Response, error) {
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
	}
	return c.Post(ctx, url, body, headers)
}

// ReadBody reads the response body and closes it.
// This is a convenience method to ensure the body is always closed.
func ReadBody(resp *http.Response) ([]byte, error) {
	if resp == nil {
		return nil, errors.New("response is nil")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	return body, nil
}

// CheckStatus validates the HTTP status code and returns an error if it's not successful.
func CheckStatus(resp *http.Response) error {
	if resp == nil {
		return errors.New("response is nil")
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return errors.ErrRateLimit
	case http.StatusNotFound:
		return errors.ErrNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return errors.ErrUnauthorized
	case http.StatusServiceUnavailable, http.StatusGatewayTimeout, http.StatusBadGateway:
		return errors.ErrServiceUnavailable
	default:
		return errors.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
}

// FetchJSON performs a GET request and returns the response body as bytes.
// The response is validated for 2xx status codes.
func (c *Client) FetchJSON(ctx context.Context, url string) ([]byte, error) {
	resp, err := c.GetJSON(ctx, url)
	if err != nil {
		return nil, err
	}

	if err := CheckStatus(resp); err != nil {
		resp.Body.Close()
		return nil, errors.Wrapf(err, "request to %s failed", url)
	}

	return ReadBody(resp)
}

// String returns a human-readable representation of the client configuration.
func (c *Client) String() string {
	return fmt.Sprintf("HTTPClient{timeout=%s, max_retries=%d, rate_limit=%.1f/s}",
		c.config.Timeout,
		c.config.MaxRetries,
		c.config.RateLimit,
	)
}
