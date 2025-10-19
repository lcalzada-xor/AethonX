// internal/core/usecases/mocks_test.go
package usecases

import (
	"context"
	"sync"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
)

// mockSource es un mock de ports.Source para tests del orchestrator
type mockSource struct {
	name         string
	mode         domain.SourceMode
	sourceType   domain.SourceType
	runFunc      func(ctx context.Context, target domain.Target) (*domain.ScanResult, error)
	runCallCount int
}

func newMockSource(name string, mode domain.SourceMode, sourceType domain.SourceType) *mockSource {
	return &mockSource{
		name:         name,
		mode:         mode,
		sourceType:   sourceType,
		runCallCount: 0,
	}
}

func (m *mockSource) Name() string {
	return m.name
}

func (m *mockSource) Mode() domain.SourceMode {
	return m.mode
}

func (m *mockSource) Type() domain.SourceType {
	return m.sourceType
}

func (m *mockSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	m.runCallCount++
	if m.runFunc != nil {
		return m.runFunc(ctx, target)
	}
	// Default behavior: return empty result
	result := domain.NewScanResult(target)
	return result, nil
}

func (m *mockSource) Close() error {
	// Mock source no tiene recursos que liberar
	return nil
}

// mockSourceWithArtifacts creates a mock that returns specific artifacts
func mockSourceWithArtifacts(name string, artifacts []*domain.Artifact) *mockSource {
	mock := newMockSource(name, domain.SourceModePassive, domain.SourceTypeAPI)
	mock.runFunc = func(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
		result := domain.NewScanResult(target)
		result.Artifacts = artifacts
		return result, nil
	}
	return mock
}

// mockSourceWithError creates a mock that always fails
func mockSourceWithError(name string, err error) *mockSource {
	mock := newMockSource(name, domain.SourceModePassive, domain.SourceTypeAPI)
	mock.runFunc = func(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
		return nil, err
	}
	return mock
}

// mockNotifier es un mock de ports.Notifier para tests
type mockNotifier struct {
	mu              sync.Mutex
	notifyFunc      func(ctx context.Context, event ports.Event) error
	closeFunc       func() error
	notifyCallCount int
	events          []ports.Event
}

func newMockNotifier() *mockNotifier {
	return &mockNotifier{
		notifyCallCount: 0,
		events:          []ports.Event{},
	}
}

func (m *mockNotifier) Notify(ctx context.Context, event ports.Event) error {
	m.mu.Lock()
	m.notifyCallCount++
	m.events = append(m.events, event)
	m.mu.Unlock()

	if m.notifyFunc != nil {
		return m.notifyFunc(ctx, event)
	}
	return nil
}

func (m *mockNotifier) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

// getEventsByType returns events filtered by type
func (m *mockNotifier) getEventsByType(eventType ports.EventType) []ports.Event {
	m.mu.Lock()
	defer m.mu.Unlock()

	var filtered []ports.Event
	for _, e := range m.events {
		if e.Type == eventType {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// getNotifyCallCount returns the number of times Notify was called (thread-safe)
func (m *mockNotifier) getNotifyCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.notifyCallCount
}
