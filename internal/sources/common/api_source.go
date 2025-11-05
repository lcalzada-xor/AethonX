// Package common provides shared abstractions for source implementations.
package common

import (
	"context"
	"sync"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
)

// BaseAPISource provides common functionality for API-based reconnaissance sources.
// It handles progress channel management, streaming, and resource cleanup.
//
// Usage:
//   1. Embed BaseAPISource in your API source struct
//   2. Call NewBaseAPISource() in your constructor
//   3. Use EmitProgress() for progress updates
//   4. Use DefaultStream() for streaming implementation
//   5. Call Close() via embedded method (automatic)
type BaseAPISource struct {
	logger     logx.Logger
	progressCh chan ports.ProgressUpdate
	chClosed   bool // Track if progressCh is closed
	mu         sync.Mutex
}

// NewBaseAPISource creates a new BaseAPISource with the given logger and source name.
func NewBaseAPISource(logger logx.Logger, sourceName string) *BaseAPISource {
	return &BaseAPISource{
		logger:     logger.With("source", sourceName),
		progressCh: make(chan ports.ProgressUpdate, 10),
	}
}

// EmitProgress sends a progress update in a non-blocking, thread-safe manner.
// If the channel buffer is full, the update is silently dropped to avoid blocking.
//
// This method is safe to call from multiple goroutines.
func (b *BaseAPISource) EmitProgress(artifactCount int, message string) {
	b.mu.Lock()
	if b.chClosed {
		b.mu.Unlock()
		return // Channel already closed, skip emit
	}
	ch := b.progressCh
	b.mu.Unlock()

	select {
	case ch <- ports.ProgressUpdate{
		ArtifactCount: artifactCount,
		Message:       message,
	}:
	default:
		// Channel full, skip update to avoid blocking
	}
}

// ProgressChannel returns the progress channel for streaming updates.
// Implements ports.StreamingSource interface.
func (b *BaseAPISource) ProgressChannel() <-chan ports.ProgressUpdate {
	return b.progressCh
}

// DefaultStream provides a default Stream implementation that wraps Run().
// Implements ports.StreamingSource by delegating to Run() and emitting artifacts.
//
// This is the standard implementation for API sources that don't need custom streaming.
func (b *BaseAPISource) DefaultStream(
	ctx context.Context,
	target domain.Target,
	runFunc func(ctx context.Context, target domain.Target) (*domain.ScanResult, error),
) (<-chan *domain.Artifact, <-chan error) {
	artifactCh := make(chan *domain.Artifact, 100)
	errorCh := make(chan error, 1)

	go func() {
		defer close(artifactCh)
		defer close(errorCh)

		result, err := runFunc(ctx, target)
		if err != nil {
			errorCh <- err
			return
		}

		// Stream artifacts
		for _, artifact := range result.Artifacts {
			select {
			case artifactCh <- artifact:
			case <-ctx.Done():
				errorCh <- ctx.Err()
				return
			}
		}
	}()

	return artifactCh, errorCh
}

// Close terminates the progress channel and cleans up resources.
// Safe to call multiple times (idempotent).
//
// This method is automatically available to all sources that embed BaseAPISource.
func (b *BaseAPISource) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.logger.Debug("closing API source")

	// Close progress channel to prevent goroutine leaks (only once)
	if !b.chClosed {
		close(b.progressCh)
		b.chClosed = true
	}

	b.logger.Debug("API source closed")
	return nil
}

// GetLogger returns the logger instance.
// Useful for sources that need to access the configured logger.
func (b *BaseAPISource) GetLogger() logx.Logger {
	return b.logger
}
