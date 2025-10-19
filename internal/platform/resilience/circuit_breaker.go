// internal/platform/resilience/circuit_breaker.go
package resilience

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrCircuitOpen     = errors.New("circuit breaker is open")
	ErrTooManyRequests = errors.New("too many requests in half-open state")
)

// State representa el estado del circuit breaker.
type State int

const (
	StateClosed   State = iota // Normal operation
	StateOpen                  // Failing, rejecting requests
	StateHalfOpen              // Testing if service recovered
)

// CircuitBreaker implementa el patrón Circuit Breaker para prevenir
// cascadas de fallos en sources que están caídas.
type CircuitBreaker struct {
	mu               sync.RWMutex
	state            State
	failureCount     int
	successCount     int
	lastFailureTime  time.Time
	lastSuccessTime  time.Time

	// Config
	failureThreshold int           // Failures to open circuit
	timeout          time.Duration // Time to wait before half-open
	halfOpenMax      int           // Max requests in half-open state
}

// NewCircuitBreaker crea un nuevo circuit breaker.
func NewCircuitBreaker(failureThreshold int, timeout time.Duration, halfOpenMax int) *CircuitBreaker {
	if failureThreshold <= 0 {
		failureThreshold = 5
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	if halfOpenMax <= 0 {
		halfOpenMax = 3
	}

	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: failureThreshold,
		timeout:          timeout,
		halfOpenMax:      halfOpenMax,
	}
}

// Allow verifica si una request puede pasar.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	switch cb.state {
	case StateClosed:
		// Normal operation, allow
		return true

	case StateOpen:
		// Check if timeout elapsed
		if now.Sub(cb.lastFailureTime) > cb.timeout {
			// Transition to half-open
			cb.state = StateHalfOpen
			cb.successCount = 0
			cb.failureCount = 0
			return true
		}
		// Still open, reject
		return false

	case StateHalfOpen:
		// Allow limited requests to test recovery
		if cb.successCount+cb.failureCount < cb.halfOpenMax {
			return true
		}
		// Too many requests in half-open
		return false

	default:
		return false
	}
}

// RecordSuccess registra una operación exitosa.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastSuccessTime = time.Now()

	switch cb.state {
	case StateClosed:
		// Normal operation, reset failure count
		cb.failureCount = 0

	case StateHalfOpen:
		// Success in half-open, increment counter
		cb.successCount++

		// If enough successes, close circuit
		if cb.successCount >= cb.halfOpenMax {
			cb.state = StateClosed
			cb.failureCount = 0
			cb.successCount = 0
		}
	}
}

// RecordFailure registra una operación fallida.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailureTime = time.Now()
	cb.failureCount++

	switch cb.state {
	case StateClosed:
		// Check if threshold reached
		if cb.failureCount >= cb.failureThreshold {
			cb.state = StateOpen
		}

	case StateHalfOpen:
		// Failure in half-open, re-open circuit immediately
		cb.state = StateOpen
		cb.successCount = 0
		cb.failureCount = 0
	}
}

// State retorna el estado actual del circuit breaker.
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset resetea el circuit breaker al estado cerrado.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
}

// Stats retorna estadísticas del circuit breaker.
func (cb *CircuitBreaker) Stats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		State:           cb.state,
		FailureCount:    cb.failureCount,
		SuccessCount:    cb.successCount,
		LastFailureTime: cb.lastFailureTime,
		LastSuccessTime: cb.lastSuccessTime,
	}
}

// CircuitBreakerStats contiene estadísticas del circuit breaker.
type CircuitBreakerStats struct {
	State           State
	FailureCount    int
	SuccessCount    int
	LastFailureTime time.Time
	LastSuccessTime time.Time
}

// String retorna una representación legible del estado.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}
