// internal/platform/registry/source_registry_test.go
package registry

import (
	"context"
	"testing"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/testutil"
)

// mockSource es un mock local de ports.Source para testing
type mockSource struct {
	name string
	mode domain.SourceMode
	typ  domain.SourceType
}

func (m *mockSource) Name() string                                                        { return m.name }
func (m *mockSource) Mode() domain.SourceMode                                             { return m.mode }
func (m *mockSource) Type() domain.SourceType                                             { return m.typ }
func (m *mockSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	return domain.NewScanResult(target), nil
}
func (m *mockSource) Close() error { return nil }

func TestSourceRegistry_Register(t *testing.T) {
	registry := NewSourceRegistry(logx.New())

	factory := func(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
		return &mockSource{name: "test"}, nil
	}

	meta := ports.SourceMetadata{
		Name: "test",
		Mode: domain.SourceModePassive,
		Type: domain.SourceTypeAPI,
	}

	err := registry.Register("test", factory, meta)
	testutil.AssertNoError(t, err, "register should succeed")

	testutil.AssertTrue(t, registry.IsRegistered("test"), "source should be registered")
}

func TestSourceRegistry_Register_Duplicate(t *testing.T) {
	registry := NewSourceRegistry(logx.New())

	factory := func(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
		return &mockSource{name: "test"}, nil
	}

	meta := ports.SourceMetadata{Name: "test"}

	registry.Register("test", factory, meta)
	err := registry.Register("test", factory, meta)

	testutil.AssertTrue(t, err != nil, "duplicate registration should fail")
}

func TestSourceRegistry_Build(t *testing.T) {
	registry := NewSourceRegistry(logx.New())

	factory := func(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
		return &mockSource{name: "test"}, nil
	}

	meta := ports.SourceMetadata{
		Name: "test",
		Mode: domain.SourceModePassive,
	}

	registry.Register("test", factory, meta)

	configs := map[string]ports.SourceConfig{
		"test": {
			Enabled:  true,
			Priority: 5,
		},
	}

	sources, err := registry.Build(configs, logx.New())

	testutil.AssertNoError(t, err, "build should succeed")
	testutil.AssertEqual(t, len(sources), 1, "should build one source")
}

func TestSourceRegistry_Build_DisabledSource(t *testing.T) {
	registry := NewSourceRegistry(logx.New())

	factory := func(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
		return &mockSource{name: "test"}, nil
	}

	meta := ports.SourceMetadata{Name: "test"}
	registry.Register("test", factory, meta)

	configs := map[string]ports.SourceConfig{
		"test": {
			Enabled: false,
		},
	}

	sources, err := registry.Build(configs, logx.New())

	// Cuando todas las sources están disabled, debería retornar error
	testutil.AssertTrue(t, err != nil, "build should fail when all sources disabled")
	testutil.AssertEqual(t, len(sources), 0, "should build zero sources")
}

func TestSourceRegistry_Build_Priority(t *testing.T) {
	registry := NewSourceRegistry(logx.New())

	factoryA := func(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
		return &mockSource{name: "source_a"}, nil
	}
	factoryB := func(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
		return &mockSource{name: "source_b"}, nil
	}

	registry.Register("source_a", factoryA, ports.SourceMetadata{Name: "source_a"})
	registry.Register("source_b", factoryB, ports.SourceMetadata{Name: "source_b"})

	configs := map[string]ports.SourceConfig{
		"source_a": {Enabled: true, Priority: 10},
		"source_b": {Enabled: true, Priority: 5},
	}

	sources, err := registry.Build(configs, logx.New())

	testutil.AssertNoError(t, err, "build should succeed")
	testutil.AssertEqual(t, len(sources), 2, "should build two sources")

	// source_a (priority 10) debe venir antes que source_b (priority 5)
	testutil.AssertEqual(t, sources[0].Name(), "source_a", "higher priority first")
	testutil.AssertEqual(t, sources[1].Name(), "source_b", "lower priority second")
}

func TestSourceRegistry_List(t *testing.T) {
	registry := NewSourceRegistry(logx.New())

	factory := func(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
		return &mockSource{}, nil
	}

	registry.Register("alpha", factory, ports.SourceMetadata{Name: "alpha"})
	registry.Register("beta", factory, ports.SourceMetadata{Name: "beta"})

	names := registry.List()

	testutil.AssertEqual(t, len(names), 2, "should list two sources")
	testutil.AssertEqual(t, names[0], "alpha", "should be sorted alphabetically")
	testutil.AssertEqual(t, names[1], "beta", "should be sorted alphabetically")
}

func TestSourceRegistry_GetMetadata(t *testing.T) {
	registry := NewSourceRegistry(logx.New())

	factory := func(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
		return &mockSource{}, nil
	}

	meta := ports.SourceMetadata{
		Name:        "test",
		Description: "Test source",
		Version:     "1.0.0",
		Mode:        domain.SourceModePassive,
	}

	registry.Register("test", factory, meta)

	retrieved, exists := registry.GetMetadata("test")

	testutil.AssertTrue(t, exists, "metadata should exist")
	testutil.AssertEqual(t, retrieved.Name, "test", "name should match")
	testutil.AssertEqual(t, retrieved.Description, "Test source", "description should match")
	testutil.AssertEqual(t, retrieved.Version, "1.0.0", "version should match")
}

func TestSourceRegistry_Clear(t *testing.T) {
	registry := NewSourceRegistry(logx.New())

	factory := func(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
		return &mockSource{}, nil
	}

	registry.Register("test", factory, ports.SourceMetadata{Name: "test"})
	testutil.AssertTrue(t, registry.IsRegistered("test"), "source should be registered")

	registry.Clear()
	testutil.AssertTrue(t, !registry.IsRegistered("test"), "source should not be registered after clear")
}

func TestSourceRegistry_Build_ValidationNilConfigs(t *testing.T) {
	registry := NewSourceRegistry(logx.New())

	sources, err := registry.Build(nil, logx.New())

	testutil.AssertTrue(t, err != nil, "should fail with nil configs")
	testutil.AssertTrue(t, sources == nil, "sources should be nil")
}

func TestSourceRegistry_Build_ValidationNilLogger(t *testing.T) {
	registry := NewSourceRegistry(logx.New())

	configs := map[string]ports.SourceConfig{
		"test": {Enabled: true},
	}

	sources, err := registry.Build(configs, nil)

	testutil.AssertTrue(t, err != nil, "should fail with nil logger")
	testutil.AssertTrue(t, sources == nil, "sources should be nil")
}

func TestSourceRegistry_Build_UnregisteredSource(t *testing.T) {
	registry := NewSourceRegistry(logx.New())

	configs := map[string]ports.SourceConfig{
		"nonexistent": {Enabled: true},
	}

	sources, err := registry.Build(configs, logx.New())

	// Debería retornar error porque la source no está registrada
	testutil.AssertTrue(t, err != nil, "should fail when source not registered")
	testutil.AssertEqual(t, len(sources), 0, "should build zero sources")
}
