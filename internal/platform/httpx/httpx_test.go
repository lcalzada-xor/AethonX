package httpx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aethonx/internal/platform/errors"
	"aethonx/internal/platform/logx"
	"aethonx/internal/testutil"
)

func TestNew(t *testing.T) {
	logger := logx.New()

	t.Run("creates client with default config", func(t *testing.T) {
		config := DefaultConfig()
		client := New(config, logger)

		testutil.AssertNotNil(t, client, "client should not be nil")
		testutil.AssertEqual(t, client.config.Timeout, 30*time.Second, "timeout should match")
		testutil.AssertEqual(t, client.config.MaxRetries, 3, "max retries should match")
		testutil.AssertEqual(t, client.config.UserAgent, "AethonX/1.0", "user agent should match")
	})

	t.Run("applies defaults for zero values", func(t *testing.T) {
		config := Config{}
		client := New(config, logger)

		testutil.AssertEqual(t, client.config.Timeout, 30*time.Second, "should use default timeout")
		testutil.AssertEqual(t, client.config.RetryBackoff, 1*time.Second, "should use default backoff")
		testutil.AssertEqual(t, client.config.UserAgent, "AethonX/1.0", "should use default user agent")
	})

	t.Run("creates rate limiter when configured", func(t *testing.T) {
		config := Config{
			RateLimit:      10,
			RateLimitBurst: 5,
		}
		client := New(config, logger)

		testutil.AssertNotNil(t, client.rateLimiter, "rate limiter should be created")
	})

	t.Run("does not create rate limiter when disabled", func(t *testing.T) {
		config := Config{
			RateLimit: 0,
		}
		client := New(config, logger)

		testutil.AssertTrue(t, client.rateLimiter == nil, "rate limiter should not be created")
	})
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	testutil.AssertEqual(t, config.Timeout, 30*time.Second, "timeout should be 30s")
	testutil.AssertEqual(t, config.MaxRetries, 3, "max retries should be 3")
	testutil.AssertEqual(t, config.RetryBackoff, 1*time.Second, "backoff should be 1s")
	testutil.AssertEqual(t, config.MaxRetryBackoff, 30*time.Second, "max backoff should be 30s")
	testutil.AssertEqual(t, config.UserAgent, "AethonX/1.0", "user agent should be AethonX/1.0")
	testutil.AssertEqual(t, config.RateLimit, 0.0, "rate limit should be 0")
	testutil.AssertEqual(t, config.RateLimitBurst, 1, "rate limit burst should be 1")
}

func TestClient_Get(t *testing.T) {
	logger := logx.New()

	t.Run("successful GET request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			testutil.AssertEqual(t, r.Method, http.MethodGet, "method should be GET")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "ok"}`))
		}))
		defer server.Close()

		config := DefaultConfig()
		client := New(config, logger)

		resp, err := client.Get(context.Background(), server.URL, nil)
		testutil.AssertNoError(t, err, "request should succeed")
		testutil.AssertNotNil(t, resp, "response should not be nil")
		testutil.AssertEqual(t, resp.StatusCode, http.StatusOK, "status should be 200")

		body, err := ReadBody(resp)
		testutil.AssertNoError(t, err, "should read body")
		testutil.AssertEqual(t, string(body), `{"status": "ok"}`, "body should match")
	})

	t.Run("sets custom headers", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			testutil.AssertEqual(t, r.Header.Get("X-Custom"), "test", "custom header should be set")
			testutil.AssertEqual(t, r.Header.Get("User-Agent"), "AethonX/1.0", "user agent should be set")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := DefaultConfig()
		client := New(config, logger)

		headers := map[string]string{
			"X-Custom": "test",
		}
		resp, err := client.Get(context.Background(), server.URL, headers)
		testutil.AssertNoError(t, err, "request should succeed")
		testutil.AssertNotNil(t, resp, "response should not be nil")
		resp.Body.Close()
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := DefaultConfig()
		client := New(config, logger)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		_, err := client.Get(ctx, server.URL, nil)
		testutil.AssertTrue(t, err != nil, "should return error on timeout")
	})
}

func TestClient_Post(t *testing.T) {
	logger := logx.New()

	t.Run("successful POST request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			testutil.AssertEqual(t, r.Method, http.MethodPost, "method should be POST")

			body, _ := io.ReadAll(r.Body)
			testutil.AssertEqual(t, string(body), `{"key": "value"}`, "body should match")

			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"created": true}`))
		}))
		defer server.Close()

		config := DefaultConfig()
		client := New(config, logger)

		body := strings.NewReader(`{"key": "value"}`)
		resp, err := client.Post(context.Background(), server.URL, body, nil)
		testutil.AssertNoError(t, err, "request should succeed")
		testutil.AssertNotNil(t, resp, "response should not be nil")
		testutil.AssertEqual(t, resp.StatusCode, http.StatusCreated, "status should be 201")

		respBody, err := ReadBody(resp)
		testutil.AssertNoError(t, err, "should read body")
		testutil.AssertEqual(t, string(respBody), `{"created": true}`, "body should match")
	})
}

func TestClient_Retry(t *testing.T) {
	logger := logx.New()

	t.Run("retries on 503 status", func(t *testing.T) {
		attempts := int32(0)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&attempts, 1)
			if count < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		config := Config{
			MaxRetries:   3,
			RetryBackoff: 10 * time.Millisecond,
		}
		client := New(config, logger)

		resp, err := client.Get(context.Background(), server.URL, nil)
		testutil.AssertNoError(t, err, "should succeed after retries")
		testutil.AssertNotNil(t, resp, "response should not be nil")
		testutil.AssertEqual(t, resp.StatusCode, http.StatusOK, "final status should be 200")
		testutil.AssertTrue(t, atomic.LoadInt32(&attempts) >= 3, "should have retried")
		resp.Body.Close()
	})

	t.Run("retries on 429 rate limit", func(t *testing.T) {
		attempts := int32(0)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&attempts, 1)
			if count < 2 {
				w.WriteHeader(http.StatusTooManyRequests)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		config := Config{
			MaxRetries:   3,
			RetryBackoff: 10 * time.Millisecond,
		}
		client := New(config, logger)

		resp, err := client.Get(context.Background(), server.URL, nil)
		testutil.AssertNoError(t, err, "should succeed after retries")
		testutil.AssertEqual(t, resp.StatusCode, http.StatusOK, "final status should be 200")
		testutil.AssertTrue(t, atomic.LoadInt32(&attempts) >= 2, "should have retried")
		resp.Body.Close()
	})

	t.Run("does not retry on 404", func(t *testing.T) {
		attempts := int32(0)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		config := Config{
			MaxRetries:   3,
			RetryBackoff: 10 * time.Millisecond,
		}
		client := New(config, logger)

		resp, err := client.Get(context.Background(), server.URL, nil)
		testutil.AssertNoError(t, err, "request should complete")
		testutil.AssertEqual(t, resp.StatusCode, http.StatusNotFound, "status should be 404")
		testutil.AssertEqual(t, atomic.LoadInt32(&attempts), int32(1), "should not retry on 404")
		resp.Body.Close()
	})

	t.Run("exhausts retries and returns error", func(t *testing.T) {
		attempts := int32(0)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		config := Config{
			MaxRetries:   2,
			RetryBackoff: 10 * time.Millisecond,
		}
		client := New(config, logger)

		_, err := client.Get(context.Background(), server.URL, nil)
		testutil.AssertTrue(t, err != nil, "should return error after exhausting retries")
		testutil.AssertEqual(t, atomic.LoadInt32(&attempts), int32(3), "should attempt 3 times (1 + 2 retries)")
	})
}

func TestClient_RateLimit(t *testing.T) {
	logger := logx.New()

	t.Run("enforces rate limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := Config{
			RateLimit:      10, // 10 req/s
			RateLimitBurst: 2,
		}
		client := New(config, logger)

		start := time.Now()

		// Make 5 requests (burst of 2, then 3 more at 10/s = ~300ms)
		for i := 0; i < 5; i++ {
			resp, err := client.Get(context.Background(), server.URL, nil)
			testutil.AssertNoError(t, err, "request should succeed")
			resp.Body.Close()
		}

		elapsed := time.Since(start)

		// Should take at least 300ms (3 requests at 10/s after burst)
		testutil.AssertTrue(t, elapsed >= 250*time.Millisecond, "should enforce rate limit")
	})
}

func TestClient_GetJSON(t *testing.T) {
	logger := logx.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testutil.AssertEqual(t, r.Header.Get("Accept"), "application/json", "should set Accept header")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"key": "value"}`))
	}))
	defer server.Close()

	config := DefaultConfig()
	client := New(config, logger)

	resp, err := client.GetJSON(context.Background(), server.URL)
	testutil.AssertNoError(t, err, "request should succeed")
	testutil.AssertNotNil(t, resp, "response should not be nil")
	resp.Body.Close()
}

func TestClient_PostJSON(t *testing.T) {
	logger := logx.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testutil.AssertEqual(t, r.Header.Get("Content-Type"), "application/json", "should set Content-Type")
		testutil.AssertEqual(t, r.Header.Get("Accept"), "application/json", "should set Accept header")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := DefaultConfig()
	client := New(config, logger)

	body := strings.NewReader(`{"key": "value"}`)
	resp, err := client.PostJSON(context.Background(), server.URL, body)
	testutil.AssertNoError(t, err, "request should succeed")
	testutil.AssertNotNil(t, resp, "response should not be nil")
	resp.Body.Close()
}

func TestClient_FetchJSON(t *testing.T) {
	logger := logx.New()

	t.Run("successful fetch", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "ok"}`))
		}))
		defer server.Close()

		config := DefaultConfig()
		client := New(config, logger)

		body, err := client.FetchJSON(context.Background(), server.URL)
		testutil.AssertNoError(t, err, "fetch should succeed")
		testutil.AssertEqual(t, string(body), `{"status": "ok"}`, "body should match")
	})

	t.Run("returns error on non-2xx status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		config := DefaultConfig()
		client := New(config, logger)

		_, err := client.FetchJSON(context.Background(), server.URL)
		testutil.AssertTrue(t, err != nil, "should return error on 404")
		testutil.AssertTrue(t, errors.IsNotFound(err), "should be not found error")
	})
}

func TestCheckStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    error
	}{
		{"200 OK", http.StatusOK, nil},
		{"201 Created", http.StatusCreated, nil},
		{"204 No Content", http.StatusNoContent, nil},
		{"400 Bad Request", http.StatusBadRequest, nil},
		{"404 Not Found", http.StatusNotFound, errors.ErrNotFound},
		{"429 Too Many Requests", http.StatusTooManyRequests, errors.ErrRateLimit},
		{"401 Unauthorized", http.StatusUnauthorized, errors.ErrUnauthorized},
		{"403 Forbidden", http.StatusForbidden, errors.ErrUnauthorized},
		{"503 Service Unavailable", http.StatusServiceUnavailable, errors.ErrServiceUnavailable},
		{"504 Gateway Timeout", http.StatusGatewayTimeout, errors.ErrServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Status:     http.StatusText(tt.statusCode),
			}

			err := CheckStatus(resp)

			if tt.wantErr == nil {
				if tt.statusCode >= 200 && tt.statusCode < 300 {
					testutil.AssertNoError(t, err, "should not return error for 2xx")
				}
			} else {
				testutil.AssertTrue(t, err != nil, "should return error")
				testutil.AssertTrue(t, errors.Is(err, tt.wantErr), "error should match expected type")
			}
		})
	}

	t.Run("nil response", func(t *testing.T) {
		err := CheckStatus(nil)
		testutil.AssertTrue(t, err != nil, "should return error for nil response")
	})
}

func TestReadBody(t *testing.T) {
	t.Run("reads response body", func(t *testing.T) {
		resp := &http.Response{
			Body: io.NopCloser(strings.NewReader("test body")),
		}

		body, err := ReadBody(resp)
		testutil.AssertNoError(t, err, "should read body")
		testutil.AssertEqual(t, string(body), "test body", "body should match")
	})

	t.Run("returns error for nil response", func(t *testing.T) {
		_, err := ReadBody(nil)
		testutil.AssertTrue(t, err != nil, "should return error for nil response")
	})
}

func TestClient_SetRateLimit(t *testing.T) {
	logger := logx.New()
	config := DefaultConfig()
	client := New(config, logger)

	t.Run("enables rate limiting", func(t *testing.T) {
		client.SetRateLimit(10, 5)
		testutil.AssertNotNil(t, client.rateLimiter, "rate limiter should be created")
	})

	t.Run("disables rate limiting with zero rate", func(t *testing.T) {
		client.SetRateLimit(0, 0)
		testutil.AssertTrue(t, client.rateLimiter == nil, "rate limiter should be removed")
	})

	t.Run("updates existing rate limiter", func(t *testing.T) {
		client.SetRateLimit(10, 5)
		initialLimiter := client.rateLimiter

		client.SetRateLimit(20, 10)
		testutil.AssertEqual(t, client.rateLimiter, initialLimiter, "should reuse existing limiter")
	})
}

func TestClient_String(t *testing.T) {
	logger := logx.New()
	config := Config{
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RateLimit:  10.5,
	}
	client := New(config, logger)

	str := client.String()
	testutil.AssertTrue(t, strings.Contains(str, "30s"), "should contain timeout")
	testutil.AssertTrue(t, strings.Contains(str, "3"), "should contain max retries")
	testutil.AssertTrue(t, strings.Contains(str, "10.5"), "should contain rate limit")
}

func TestClient_Backoff(t *testing.T) {
	logger := logx.New()

	t.Run("exponential backoff", func(t *testing.T) {
		config := Config{
			RetryBackoff:    10 * time.Millisecond,
			MaxRetryBackoff: 100 * time.Millisecond,
		}
		client := New(config, logger)

		// First attempt (2^0 = 1x)
		start := time.Now()
		err := client.backoff(context.Background(), 0)
		elapsed := time.Since(start)
		testutil.AssertNoError(t, err, "backoff should succeed")
		testutil.AssertTrue(t, elapsed >= 10*time.Millisecond, "should backoff 10ms")

		// Second attempt (2^1 = 2x)
		start = time.Now()
		err = client.backoff(context.Background(), 1)
		elapsed = time.Since(start)
		testutil.AssertNoError(t, err, "backoff should succeed")
		testutil.AssertTrue(t, elapsed >= 20*time.Millisecond, "should backoff 20ms")
	})

	t.Run("caps at max backoff", func(t *testing.T) {
		config := Config{
			RetryBackoff:    10 * time.Millisecond,
			MaxRetryBackoff: 30 * time.Millisecond,
		}
		client := New(config, logger)

		// Large attempt number should cap at max
		start := time.Now()
		err := client.backoff(context.Background(), 10)
		elapsed := time.Since(start)
		testutil.AssertNoError(t, err, "backoff should succeed")
		testutil.AssertTrue(t, elapsed >= 30*time.Millisecond && elapsed < 50*time.Millisecond,
			"should cap at max backoff")
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		config := Config{
			RetryBackoff: 1 * time.Second,
		}
		client := New(config, logger)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		err := client.backoff(ctx, 0)
		testutil.AssertTrue(t, err != nil, "should return error on context cancellation")
	})
}

func BenchmarkClient_Get(b *testing.B) {
	logger := logx.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	config := DefaultConfig()
	client := New(config, logger)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, _ := client.Get(ctx, server.URL, nil)
		if resp != nil {
			resp.Body.Close()
		}
	}
}

func ExampleClient_Get() {
	logger := logx.New()
	config := DefaultConfig()
	client := New(config, logger)

	resp, err := client.Get(context.Background(), "https://api.example.com/data", nil)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()

	fmt.Println("Status:", resp.StatusCode)
}

func ExampleClient_FetchJSON() {
	logger := logx.New()
	config := DefaultConfig()
	client := New(config, logger)

	body, err := client.FetchJSON(context.Background(), "https://api.example.com/data")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Body:", string(body))
}
