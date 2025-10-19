package rate

import (
	"context"
	"sync"
	"testing"
	"time"

	"aethonx/internal/testutil"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name       string
		rate       float64
		burst      int
		wantRate   float64
		wantBurst  int
		wantTokens float64
	}{
		{
			name:       "valid rate and burst",
			rate:       10.0,
			burst:      5,
			wantRate:   10.0,
			wantBurst:  5,
			wantTokens: 5.0,
		},
		{
			name:       "zero rate defaults to 1",
			rate:       0,
			burst:      5,
			wantRate:   1.0,
			wantBurst:  5,
			wantTokens: 5.0,
		},
		{
			name:       "negative rate defaults to 1",
			rate:       -5.0,
			burst:      5,
			wantRate:   1.0,
			wantBurst:  5,
			wantTokens: 5.0,
		},
		{
			name:       "zero burst defaults to 1",
			rate:       10.0,
			burst:      0,
			wantRate:   10.0,
			wantBurst:  1,
			wantTokens: 1.0,
		},
		{
			name:       "negative burst defaults to 1",
			rate:       10.0,
			burst:      -5,
			wantRate:   10.0,
			wantBurst:  1,
			wantTokens: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := New(tt.rate, tt.burst)

			testutil.AssertEqual(t, limiter.Rate(), tt.wantRate, "rate should match")
			testutil.AssertEqual(t, limiter.Burst(), tt.wantBurst, "burst should match")
			testutil.AssertEqual(t, limiter.Tokens(), tt.wantTokens, "tokens should start at burst capacity")
		})
	}
}

func TestLimiter_Allow(t *testing.T) {
	t.Run("allows operations within burst", func(t *testing.T) {
		limiter := New(10, 5)

		// Should allow burst number of operations immediately
		for i := 0; i < 5; i++ {
			allowed := limiter.Allow()
			testutil.AssertTrue(t, allowed, "should allow operation within burst")
		}

		// Next operation should be denied (bucket empty)
		allowed := limiter.Allow()
		testutil.AssertTrue(t, !allowed, "should deny operation when bucket empty")
	})

	t.Run("refills tokens over time", func(t *testing.T) {
		limiter := New(10, 1) // 10 tokens/second, burst of 1

		// Consume the initial token
		allowed := limiter.Allow()
		testutil.AssertTrue(t, allowed, "should allow first operation")

		// Should be denied immediately
		allowed = limiter.Allow()
		testutil.AssertTrue(t, !allowed, "should deny when bucket empty")

		// Wait for token refill (100ms = 1 token at 10/s)
		time.Sleep(100 * time.Millisecond)

		// Should be allowed now
		allowed = limiter.Allow()
		testutil.AssertTrue(t, allowed, "should allow after token refill")
	})
}

func TestLimiter_AllowN(t *testing.T) {
	t.Run("allows batch operations within capacity", func(t *testing.T) {
		limiter := New(10, 10)

		// Should allow batch of 5
		allowed := limiter.AllowN(5)
		testutil.AssertTrue(t, allowed, "should allow batch within capacity")
		tokens := limiter.Tokens()
		testutil.AssertTrue(t, tokens >= 4.9 && tokens <= 5.1, "should have ~5 tokens remaining")

		// Should allow another batch of 5
		allowed = limiter.AllowN(5)
		testutil.AssertTrue(t, allowed, "should allow second batch")
		tokens = limiter.Tokens()
		testutil.AssertTrue(t, tokens >= 0.0 && tokens <= 0.1, "should have ~0 tokens remaining")
	})

	t.Run("denies batch exceeding capacity", func(t *testing.T) {
		limiter := New(10, 10)

		// Should deny batch exceeding capacity
		allowed := limiter.AllowN(15)
		testutil.AssertTrue(t, !allowed, "should deny batch exceeding capacity")
		testutil.AssertEqual(t, limiter.Tokens(), 10.0, "tokens should be unchanged")
	})
}

func TestLimiter_Wait(t *testing.T) {
	t.Run("waits for available token", func(t *testing.T) {
		limiter := New(10, 1) // 10 tokens/second

		// Consume initial token
		allowed := limiter.Allow()
		testutil.AssertTrue(t, allowed, "should allow first operation")

		// Wait should block until token is available
		ctx := context.Background()
		start := time.Now()
		err := limiter.Wait(ctx)
		elapsed := time.Since(start)

		testutil.AssertNoError(t, err, "wait should succeed")
		// Should wait approximately 100ms (1/10 second)
		testutil.AssertTrue(t, elapsed >= 90*time.Millisecond, "should wait for token refill")
		testutil.AssertTrue(t, elapsed < 200*time.Millisecond, "should not wait too long")
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		limiter := New(1, 1) // 1 token/second

		// Consume initial token
		limiter.Allow()

		// Create context with short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Wait should return error when context is canceled
		err := limiter.Wait(ctx)
		testutil.AssertTrue(t, err != nil, "wait should return error on context cancellation")
		testutil.AssertEqual(t, err, context.DeadlineExceeded, "error should be DeadlineExceeded")
	})

	t.Run("immediate success when token available", func(t *testing.T) {
		limiter := New(10, 5)

		ctx := context.Background()
		start := time.Now()
		err := limiter.Wait(ctx)
		elapsed := time.Since(start)

		testutil.AssertNoError(t, err, "wait should succeed immediately")
		testutil.AssertTrue(t, elapsed < 10*time.Millisecond, "should not wait when token available")
	})
}

func TestLimiter_SetRate(t *testing.T) {
	limiter := New(10, 5)

	// Change rate
	limiter.SetRate(20)
	testutil.AssertEqual(t, limiter.Rate(), 20.0, "rate should be updated")

	// Zero rate should default to 1
	limiter.SetRate(0)
	testutil.AssertEqual(t, limiter.Rate(), 1.0, "zero rate should default to 1")

	// Negative rate should default to 1
	limiter.SetRate(-10)
	testutil.AssertEqual(t, limiter.Rate(), 1.0, "negative rate should default to 1")
}

func TestLimiter_SetBurst(t *testing.T) {
	limiter := New(10, 5)

	// Change burst
	limiter.SetBurst(10)
	testutil.AssertEqual(t, limiter.Burst(), 10, "burst should be updated")

	// Zero burst should default to 1
	limiter.SetBurst(0)
	testutil.AssertEqual(t, limiter.Burst(), 1, "zero burst should default to 1")

	// Negative burst should default to 1
	limiter.SetBurst(-5)
	testutil.AssertEqual(t, limiter.Burst(), 1, "negative burst should default to 1")
}

func TestLimiter_SetBurst_CapsTokens(t *testing.T) {
	limiter := New(10, 10)
	testutil.AssertEqual(t, limiter.Tokens(), 10.0, "should start with 10 tokens")

	// Reduce burst size
	limiter.SetBurst(5)
	testutil.AssertEqual(t, limiter.Tokens(), 5.0, "tokens should be capped at new burst size")
}

func TestLimiter_Reset(t *testing.T) {
	limiter := New(10, 5)

	// Consume some tokens
	limiter.Allow()
	limiter.Allow()
	tokens := limiter.Tokens()
	testutil.AssertTrue(t, tokens >= 2.9 && tokens <= 3.1, "should have ~3 tokens after consuming 2")

	// Reset
	limiter.Reset()
	testutil.AssertEqual(t, limiter.Tokens(), 5.0, "should reset to full capacity")
}

func TestLimiter_ConcurrentAccess(t *testing.T) {
	limiter := New(100, 50)
	var wg sync.WaitGroup
	allowed := 0
	denied := 0
	var mu sync.Mutex

	// Spawn 100 goroutines trying to acquire tokens
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if limiter.Allow() {
				mu.Lock()
				allowed++
				mu.Unlock()
			} else {
				mu.Lock()
				denied++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	testutil.AssertEqual(t, allowed, 50, "should allow burst number of operations")
	testutil.AssertEqual(t, denied, 50, "should deny operations beyond burst")
}

func TestLimiter_RateAccuracy(t *testing.T) {
	limiter := New(10, 1) // 10 operations per second

	// Consume initial token
	limiter.Allow()

	// Measure how many operations are allowed in 1 second
	ctx := context.Background()
	start := time.Now()
	count := 0

	for time.Since(start) < time.Second {
		if err := limiter.Wait(ctx); err == nil {
			count++
		}
	}

	// Should allow approximately 10 operations per second
	// Allow some margin for timing inaccuracies
	testutil.AssertTrue(t, count >= 8 && count <= 12,
		"should allow approximately 10 operations per second")
}

func TestLimiter_BurstHandling(t *testing.T) {
	limiter := New(5, 10) // 5/sec, burst of 10

	// Should allow full burst immediately
	for i := 0; i < 10; i++ {
		allowed := limiter.Allow()
		testutil.AssertTrue(t, allowed, "should allow operation within burst")
	}

	// 11th operation should be denied
	allowed := limiter.Allow()
	testutil.AssertTrue(t, !allowed, "should deny operation beyond burst")

	// Wait for 1 second (5 tokens should refill)
	time.Sleep(time.Second)

	// Should allow 5 more operations
	for i := 0; i < 5; i++ {
		allowed := limiter.Allow()
		testutil.AssertTrue(t, allowed, "should allow operation after refill")
	}

	// 6th operation should be denied
	allowed = limiter.Allow()
	testutil.AssertTrue(t, !allowed, "should deny operation after refilled tokens consumed")
}

func BenchmarkLimiter_Allow(b *testing.B) {
	limiter := New(1000000, 1000000) // High limits to avoid blocking

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow()
	}
}

func BenchmarkLimiter_AllowN(b *testing.B) {
	limiter := New(1000000, 1000000) // High limits to avoid blocking

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.AllowN(10)
	}
}

func BenchmarkLimiter_ConcurrentAllow(b *testing.B) {
	limiter := New(1000000, 1000000) // High limits to avoid blocking

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}
