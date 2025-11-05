package common

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
)

func TestBaseAPISource_EmitProgress(t *testing.T) {
	logger := logx.New()
	source := NewBaseAPISource(logger, "test")
	defer source.Close()

	// Test non-blocking emit
	source.EmitProgress(10, "test message")

	select {
	case update := <-source.ProgressChannel():
		if update.ArtifactCount != 10 {
			t.Errorf("Expected artifact count 10, got %d", update.ArtifactCount)
		}
		if update.Message != "test message" {
			t.Errorf("Expected message 'test message', got %s", update.Message)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected progress update but got timeout")
	}
}

func TestBaseAPISource_EmitProgress_NonBlocking(t *testing.T) {
	logger := logx.New()
	source := NewBaseAPISource(logger, "test")
	defer source.Close()

	// Fill the buffer (capacity 10)
	for i := 0; i < 10; i++ {
		source.EmitProgress(i, "fill buffer")
	}

	// This should not block even though buffer is full
	done := make(chan bool)
	go func() {
		source.EmitProgress(100, "overflow")
		done <- true
	}()

	select {
	case <-done:
		// Success - didn't block
	case <-time.After(100 * time.Millisecond):
		t.Error("EmitProgress blocked when buffer was full")
	}
}

func TestBaseAPISource_EmitProgress_Concurrent(t *testing.T) {
	logger := logx.New()
	source := NewBaseAPISource(logger, "test")
	defer source.Close()

	// Test concurrent access
	var wg sync.WaitGroup
	numGoroutines := 10
	numEmitsPerGoroutine := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numEmitsPerGoroutine; j++ {
				source.EmitProgress(id*100+j, "concurrent")
			}
		}(i)
	}

	// Drain channel in background
	go func() {
		for range source.ProgressChannel() {
			// Just drain
		}
	}()

	wg.Wait()
}

func TestBaseAPISource_DefaultStream(t *testing.T) {
	logger := logx.New()
	source := NewBaseAPISource(logger, "test")
	defer source.Close()

	target := domain.Target{Root: "example.com"}

	// Mock Run function that returns artifacts
	runFunc := func(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
		result := domain.NewScanResult(target)
		result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "sub1.example.com", "test"))
		result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "sub2.example.com", "test"))
		return result, nil
	}

	ctx := context.Background()
	artifactCh, errorCh := source.DefaultStream(ctx, target, runFunc)

	// Collect artifacts
	artifacts := make([]*domain.Artifact, 0)
	for artifact := range artifactCh {
		artifacts = append(artifacts, artifact)
	}

	// Check for errors
	select {
	case err := <-errorCh:
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	default:
	}

	if len(artifacts) != 2 {
		t.Errorf("Expected 2 artifacts, got %d", len(artifacts))
	}
}

func TestBaseAPISource_DefaultStream_WithError(t *testing.T) {
	logger := logx.New()
	source := NewBaseAPISource(logger, "test")
	defer source.Close()

	target := domain.Target{Root: "example.com"}

	// Mock Run function that returns error
	testErr := errors.New("test error")
	runFunc := func(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
		return nil, testErr
	}

	ctx := context.Background()
	artifactCh, errorCh := source.DefaultStream(ctx, target, runFunc)

	// Should get error
	for range artifactCh {
		t.Error("Expected no artifacts due to error")
	}

	select {
	case err := <-errorCh:
		if err != testErr {
			t.Errorf("Expected test error, got %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected error but got timeout")
	}
}

func TestBaseAPISource_DefaultStream_ContextCancellation(t *testing.T) {
	logger := logx.New()
	source := NewBaseAPISource(logger, "test")
	defer source.Close()

	target := domain.Target{Root: "example.com"}

	// Mock Run function that returns many artifacts
	runFunc := func(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
		result := domain.NewScanResult(target)
		for i := 0; i < 1000; i++ {
			result.AddArtifact(domain.NewArtifact(domain.ArtifactTypeSubdomain, "sub.example.com", "test"))
		}
		return result, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure cancel is called on all paths

	artifactCh, errorCh := source.DefaultStream(ctx, target, runFunc)

	// Read a few artifacts then cancel
	count := 0
	for range artifactCh {
		count++
		if count == 5 {
			cancel()
			// Continue reading to drain channel
		}
		if count > 100 {
			break // Safety
		}
	}

	// Check for cancellation error
	select {
	case err := <-errorCh:
		if err != context.Canceled {
			t.Logf("Expected context.Canceled, got %v (acceptable)", err)
		}
	default:
		// May not always send error if context cancelled early
	}

	if count < 5 {
		t.Errorf("Expected at least 5 artifacts before cancellation, got %d", count)
	}
}

func TestBaseAPISource_Close_Idempotent(t *testing.T) {
	logger := logx.New()
	source := NewBaseAPISource(logger, "test")

	// Close multiple times should not panic
	if err := source.Close(); err != nil {
		t.Errorf("First close failed: %v", err)
	}

	if err := source.Close(); err != nil {
		t.Errorf("Second close failed: %v", err)
	}

	if err := source.Close(); err != nil {
		t.Errorf("Third close failed: %v", err)
	}
}

func TestBaseAPISource_EmitAfterClose(t *testing.T) {
	logger := logx.New()
	source := NewBaseAPISource(logger, "test")

	// Close the source
	if err := source.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// EmitProgress after close should not panic
	source.EmitProgress(999, "after close")

	// Verify channel is closed
	select {
	case _, ok := <-source.ProgressChannel():
		if ok {
			t.Error("Expected channel to be closed")
		}
	default:
		t.Error("Expected to receive from closed channel")
	}
}
