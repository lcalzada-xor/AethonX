// internal/platform/resilience/retryable_source.go
package resilience

import (
	"context"
	"fmt"
	"math"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
)

// RetryableSource envuelve un Source con lógica de retry y circuit breaker.
type RetryableSource struct {
	source          ports.Source
	maxRetries      int
	backoffBase     time.Duration
	backoffMultiplier float64
	circuitBreaker  *CircuitBreaker
	logger          logx.Logger
}

// NewRetryableSource crea un nuevo RetryableSource.
func NewRetryableSource(
	source ports.Source,
	maxRetries int,
	backoffBase time.Duration,
	backoffMultiplier float64,
	cb *CircuitBreaker,
	logger logx.Logger,
) *RetryableSource {
	if maxRetries < 0 {
		maxRetries = 0
	}
	if backoffBase <= 0 {
		backoffBase = 1 * time.Second
	}
	if backoffMultiplier < 1.0 {
		backoffMultiplier = 2.0
	}

	return &RetryableSource{
		source:            source,
		maxRetries:        maxRetries,
		backoffBase:       backoffBase,
		backoffMultiplier: backoffMultiplier,
		circuitBreaker:    cb,
		logger:            logger.With("component", "retryable-source", "source", source.Name()),
	}
}

// Name retorna el nombre del source subyacente.
func (r *RetryableSource) Name() string {
	return r.source.Name()
}

// Mode retorna el modo del source subyacente.
func (r *RetryableSource) Mode() domain.SourceMode {
	return r.source.Mode()
}

// Type retorna el tipo del source subyacente.
func (r *RetryableSource) Type() domain.SourceType {
	return r.source.Type()
}

// Run ejecuta el source con retry logic y circuit breaker.
func (r *RetryableSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	// Check circuit breaker
	if r.circuitBreaker != nil && !r.circuitBreaker.Allow() {
		r.logger.Warn("circuit breaker open, skipping source")
		return nil, fmt.Errorf("circuit breaker open for source %s: %w", r.source.Name(), ErrCircuitOpen)
	}

	var lastErr error
	attempt := 0

	for attempt <= r.maxRetries {
		// Log attempt
		if attempt > 0 {
			r.logger.Info("retrying source",
				"attempt", attempt,
				"max_retries", r.maxRetries,
			)
		}

		// Execute source
		result, err := r.source.Run(ctx, target)

		if err == nil {
			// Success
			if r.circuitBreaker != nil {
				r.circuitBreaker.RecordSuccess()
			}
			if attempt > 0 {
				r.logger.Info("source succeeded after retry",
					"attempts", attempt+1,
				)
			}
			return result, nil
		}

		// Error occurred
		lastErr = err
		r.logger.Warn("source failed",
			"attempt", attempt+1,
			"error", err.Error(),
		)

		// Check if we should retry
		if attempt >= r.maxRetries {
			break
		}

		// Check context cancellation before retry
		if ctx.Err() != nil {
			r.logger.Warn("context cancelled, aborting retries")
			if r.circuitBreaker != nil {
				r.circuitBreaker.RecordFailure()
			}
			return nil, fmt.Errorf("context cancelled after %d attempts: %w", attempt+1, ctx.Err())
		}

		// Calculate backoff delay
		backoff := r.calculateBackoff(attempt)
		r.logger.Debug("backing off before retry",
			"delay_ms", backoff.Milliseconds(),
		)

		// Wait with context cancellation support
		select {
		case <-time.After(backoff):
			// Continue to next attempt
		case <-ctx.Done():
			r.logger.Warn("context cancelled during backoff")
			if r.circuitBreaker != nil {
				r.circuitBreaker.RecordFailure()
			}
			return nil, fmt.Errorf("context cancelled during backoff: %w", ctx.Err())
		}

		attempt++
	}

	// All retries exhausted
	if r.circuitBreaker != nil {
		r.circuitBreaker.RecordFailure()
	}

	r.logger.Warn("source failed after all retries",
		"attempts", attempt+1,
		"last_error", lastErr.Error(),
	)

	return nil, fmt.Errorf("source %s failed after %d attempts: %w", r.source.Name(), attempt+1, lastErr)
}

// Close cierra el source subyacente.
func (r *RetryableSource) Close() error {
	return r.source.Close()
}

// calculateBackoff calcula el delay de backoff exponencial.
func (r *RetryableSource) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: base * multiplier^attempt
	multiplier := math.Pow(r.backoffMultiplier, float64(attempt))
	backoff := time.Duration(float64(r.backoffBase) * multiplier)

	// Cap at reasonable maximum (1 minute)
	maxBackoff := 60 * time.Second
	if backoff > maxBackoff {
		backoff = maxBackoff
	}

	return backoff
}

// GetCircuitBreaker retorna el circuit breaker (útil para testing/monitoring).
func (r *RetryableSource) GetCircuitBreaker() *CircuitBreaker {
	return r.circuitBreaker
}
