// Package common provides shared abstractions for source implementations.
package common

import (
	"encoding/json"
	"sync"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
)

// JSONLineHandler is a generic OutputHandler that parses JSON lines.
// It uses Go generics to provide type-safe JSON parsing with zero boilerplate.
//
// Usage:
//   handler := common.NewJSONLineHandler[SubfinderResponse](logger, target)
//   responses := handler.GetResponses()
//   // Parse responses using your source-specific parser
//
// Type parameter T must be a JSON-unmarshable struct (e.g., HTTPXResponse, SubfinderResponse).
type JSONLineHandler[T any] struct {
	target    domain.Target
	logger    logx.Logger
	responses []T
	mu        sync.Mutex
}

// NewJSONLineHandler creates a new JSONLineHandler for the given response type.
func NewJSONLineHandler[T any](logger logx.Logger, target domain.Target) *JSONLineHandler[T] {
	return &JSONLineHandler[T]{
		target:    target,
		logger:    logger,
		responses: make([]T, 0, 100),
	}
}

// ProcessLine handles each line of stdout by parsing it as JSON.
// Implements common.OutputHandler interface.
//
// This method is thread-safe and can be called concurrently.
// JSON parsing errors are logged as warnings but don't stop processing.
func (h *JSONLineHandler[T]) ProcessLine(line []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var resp T
	if err := json.Unmarshal(line, &resp); err != nil {
		h.logger.Warn("failed to parse JSON line",
			"line", string(line),
			"error", err.Error(),
		)
		return nil // Non-fatal, continue processing
	}

	h.responses = append(h.responses, resp)

	h.logger.Debug("parsed JSON response",
		"total_responses", len(h.responses),
	)

	return nil
}

// Finalize is called after all lines are processed.
// Implements common.OutputHandler interface.
func (h *JSONLineHandler[T]) Finalize() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.logger.Info("finalized JSON handler",
		"responses", len(h.responses),
	)

	return nil
}

// GetResponses returns all successfully parsed responses.
// This method is thread-safe and returns a copy of the internal slice.
func (h *JSONLineHandler[T]) GetResponses() []T {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Return a copy to prevent external modification
	result := make([]T, len(h.responses))
	copy(result, h.responses)

	return result
}

// GetResponseCount returns the number of successfully parsed responses.
// This method is thread-safe.
func (h *JSONLineHandler[T]) GetResponseCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	return len(h.responses)
}
