// Package rate provides a token bucket rate limiter for controlling request rates.
package rate

import (
	"context"
	"sync"
	"time"
)

// Limiter implements a token bucket rate limiter that controls the rate of operations.
// It supports both blocking (Wait) and non-blocking (Allow) modes.
type Limiter struct {
	rate  float64       // tokens per second
	burst int           // maximum burst size (bucket capacity)
	mu    sync.Mutex    // protects the following fields
	tokens float64      // current number of tokens
	last   time.Time    // last time tokens were updated
}

// New creates a new rate limiter with the specified rate (requests per second)
// and burst size (maximum concurrent requests).
//
// The rate parameter specifies how many tokens are added to the bucket per second.
// The burst parameter specifies the maximum number of tokens the bucket can hold.
//
// Example:
//   limiter := rate.New(10, 5) // 10 req/s, burst of 5
func New(rate float64, burst int) *Limiter {
	if rate <= 0 {
		rate = 1
	}
	if burst <= 0 {
		burst = 1
	}

	return &Limiter{
		rate:   rate,
		burst:  burst,
		tokens: float64(burst), // start with full bucket
		last:   time.Now(),
	}
}

// Wait blocks until the limiter allows an operation to proceed or the context is canceled.
// It returns an error if the context is canceled before the operation can proceed.
//
// This method uses a blocking approach and is suitable when you want to enforce
// rate limiting by making callers wait.
func (l *Limiter) Wait(ctx context.Context) error {
	for {
		if l.Allow() {
			return nil
		}

		// Calculate how long to wait for the next token
		waitTime := l.waitDuration()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Continue to next iteration to check if we can proceed
		}
	}
}

// Allow reports whether an operation can proceed immediately.
// It consumes one token from the bucket if available.
//
// This method is non-blocking and is suitable when you want to check
// if an operation is allowed without waiting.
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.advance(time.Now())

	if l.tokens >= 1 {
		l.tokens--
		return true
	}

	return false
}

// AllowN reports whether n operations can proceed immediately.
// It consumes n tokens from the bucket if available.
func (l *Limiter) AllowN(n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.advance(time.Now())

	tokensNeeded := float64(n)
	if l.tokens >= tokensNeeded {
		l.tokens -= tokensNeeded
		return true
	}

	return false
}

// SetRate changes the rate limit dynamically.
// This is useful for adjusting rate limits based on runtime conditions.
func (l *Limiter) SetRate(rate float64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if rate <= 0 {
		rate = 1
	}

	l.advance(time.Now())
	l.rate = rate
}

// SetBurst changes the burst size dynamically.
func (l *Limiter) SetBurst(burst int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if burst <= 0 {
		burst = 1
	}

	l.advance(time.Now())
	l.burst = burst

	// Cap current tokens to new burst size
	if l.tokens > float64(burst) {
		l.tokens = float64(burst)
	}
}

// Tokens returns the current number of available tokens.
// This is useful for monitoring and debugging.
func (l *Limiter) Tokens() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.advance(time.Now())
	return l.tokens
}

// Rate returns the current rate limit (tokens per second).
func (l *Limiter) Rate() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rate
}

// Burst returns the current burst size.
func (l *Limiter) Burst() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.burst
}

// Reset resets the limiter to full capacity.
// This is useful for testing or manual rate limit resets.
func (l *Limiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.tokens = float64(l.burst)
	l.last = time.Now()
}

// advance updates the number of tokens based on elapsed time.
// Must be called with l.mu held.
func (l *Limiter) advance(now time.Time) {
	elapsed := now.Sub(l.last).Seconds()

	// Add tokens based on elapsed time and rate
	l.tokens += elapsed * l.rate

	// Cap tokens at burst size
	if l.tokens > float64(l.burst) {
		l.tokens = float64(l.burst)
	}

	l.last = now
}

// waitDuration calculates how long to wait for the next token.
// Must be called with l.mu held.
func (l *Limiter) waitDuration() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.advance(time.Now())

	if l.tokens >= 1 {
		return 0
	}

	// Calculate time needed to accumulate one token
	tokensNeeded := 1.0 - l.tokens
	secondsNeeded := tokensNeeded / l.rate

	return time.Duration(secondsNeeded * float64(time.Second))
}
