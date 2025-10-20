# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**AethonX** is a modular reconnaissance engine for passive and active web enumeration, written in Go. It implements **Clean Architecture** (Hexagonal/Ports & Adapters) with a concurrent orchestrator that executes multiple reconnaissance sources in parallel.

The project is inspired by the Greek mythology horse Aethon (one of Helios' horses) - just as Aethon illuminated the world, AethonX illuminates exposed digital assets.

## Core Architecture

### Clean Architecture Layers

The codebase follows strict dependency rules:

```
┌─────────────────────────────────────────┐
│  cmd/aethonx (main.go)                  │  ← CLI entry point
│  - Dependency injection                 │
│  - Config loading                       │
│  - Registry-based source building       │
└────────────┬────────────────────────────┘
             │
┌────────────▼────────────────────────────┐
│  internal/adapters/                     │  ← Adapters (outer layer)
│  ├─ output/     (JSON, Table, Streaming)│
│  ├─ report/     (Graph analysis - pending)│
│  ├─ storage/    (SQLite - pending)      │
│  └─ notifiers/  (Webhook, Slack - pending)
└────────────┬────────────────────────────┘
             │
┌────────────▼────────────────────────────┐
│  internal/core/                         │  ← Business logic (inner layer)
│  ├─ domain/     (Entities, Value Objects, Metadata)
│  ├─ usecases/   (Orchestrator, DedupeService, GraphService, MergeService)
│  └─ ports/      (Interfaces: Source, Notifier, Repository, Exporter)
└────────────┬────────────────────────────┘
             │
┌────────────▼────────────────────────────┐
│  internal/sources/                      │  ← Source implementations
│  ├─ crtsh/      (crt.sh certificate transparency)
│  └─ rdap/       (RDAP WHOIS queries)
└─────────────────────────────────────────┘
             │
┌────────────▼────────────────────────────┐
│  internal/platform/                     │  ← Infrastructure
│  ├─ config/     (ENV + flags)           │
│  ├─ logx/       (Structured logging)    │
│  ├─ httpx/      (HTTP client w/ retry)  │
│  ├─ cache/      (In-memory TTL cache)   │
│  ├─ rate/       (Token bucket limiter)  │
│  ├─ errors/     (Error handling)        │
│  ├─ workerpool/ (Priority-based scheduler) │
│  ├─ resilience/ (Circuit breaker, retry)│
│  ├─ registry/   (Source registry + factory)│
│  ├─ adaptive/   (Dynamic streaming config)│
│  └─ validator/  (Validation utilities)  │
└─────────────────────────────────────────┘
```

**Dependency Rule**: Inner layers NEVER depend on outer layers. All dependencies point inward.

### Key Architectural Patterns

**1. Ports & Adapters (Hexagonal Architecture)**
- `internal/core/ports/` defines interfaces (ports)
- `internal/sources/`, `internal/adapters/` implement these ports
- This allows swapping implementations without changing business logic

**2. Orchestrator Pattern**
The `Orchestrator` (`internal/core/usecases/orchestrator.go`) is the heart of the system:
- Filters sources by compatibility (passive vs active modes)
- Executes sources **concurrently** with **advanced worker pool** (priority-based scheduling)
- **Streams large results to disk** to prevent OOM with massive datasets
- Consolidates results from all sources (memory + disk)
- Deduplicates artifacts using `DedupeService`
- Builds relationship graph with `GraphService`
- Emits events to `Notifier` observers (async, non-blocking)

**Advanced Scheduling** (`internal/core/usecases/source_task.go`):
- Sources wrapped in `SourceTask` adapter implementing `workerpool.Task`
- Auto-estimated task weights based on source type and mode:
  - API sources: weight=30 (fast)
  - CLI sources: weight=70 (slow)
  - Builtin sources: weight=20 (very fast)
  - Active mode: +20 weight penalty
- Priority from source config or registry metadata
- Worker pool executes tasks in optimal order (high priority, low weight first)

**3. Source Interface** (`internal/core/ports/source.go`)
All reconnaissance sources must implement:
```go
type Source interface {
    Name() string
    Mode() domain.SourceMode  // passive, active, or hybrid
    Type() domain.SourceType  // API, CLI, or builtin
    Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error)
    Close() error             // MANDATORY: cleanup resources
}
```

**Optional Extended Interfaces**:
- `AdvancedSource`: Adds `Initialize()`, `Validate()`, `HealthCheck()`
- `StreamingSource`: Emits artifacts in real-time via channels
- `RateLimitedSource`: Configurable rate limiting per source

**4. Metadata System**
Two parallel metadata systems exist (migration pending):
- **Typed Metadata**: Type-safe structs in `internal/core/domain/metadata/` (DomainMetadata, CertificateMetadata, etc.)
- **Generic Metadata**: `map[string]string` for backward compatibility

The goal is to migrate fully to TypedMetadata for type safety.

## Common Development Commands

### Building
```bash
# Development build
make build

# Build with version info
make build VERSION=1.0.0

# Multi-platform builds
make build-all
```

### Testing
```bash
# Run all tests with coverage
make test

# Fast tests (no race detector)
make test-short

# Coverage report in browser
make test-coverage

# Coverage summary
make coverage

# Run tests for specific package
go test -v ./internal/core/domain/...

# Run single test
go test -v -run TestArtifact_Normalize ./internal/core/domain/
```

### CI Pipeline
```bash
# Full CI (used in automation)
make ci

# CI with linting
make ci-lint

# Individual checks
make fmt      # Format code
make vet      # Run go vet
make lint     # Run golangci-lint
```

### Running
```bash
# Basic scan
./aethonx -target example.com

# With JSON output
./aethonx -target example.com -out.json -out results/

# Custom workers and timeout
./aethonx -target example.com -workers 8 -timeout 60

# Via make
make run               # Runs with example.com
make scan-json         # Runs with JSON output
```

## Critical Import Cycle Prevention

**RULE**: `internal/testutil/` must NEVER import `internal/core/domain/` or `internal/core/ports/`.

**Why**: Test utilities are imported by domain tests, creating circular dependencies.

**Solution**:
- `testutil/` contains only generic helpers (`AssertEqual`, `AssertNotNil`, etc.)
- Domain-specific fixtures live in `*_test.go` files within their own packages
- Example: `internal/core/domain/fixtures_test.go` contains domain test fixtures
- Example: `internal/core/usecases/mocks_test.go` contains Source/Notifier mocks

## Implemented Sources

**crt.sh** (`internal/sources/crtsh/`)
- Queries Certificate Transparency logs via crt.sh API
- Discovers subdomains from SSL/TLS certificates
- Passive reconnaissance (no direct target contact)
- Returns artifacts: `ArtifactTypeSubdomain`, `ArtifactTypeCertificate`

**RDAP** (`internal/sources/rdap/`)
- Queries RDAP (Registration Data Access Protocol) for domain info
- Uses rdap.org bootstrap service for automatic server discovery
- In-memory caching (24h TTL) to reduce API calls
- Automatic cache cleanup worker (runs every 1 hour)
- Returns artifacts: `ArtifactTypeDomain`, `ArtifactTypeEmail`, `ArtifactTypeNameserver`
- Includes rich metadata: registrar, registration dates, nameservers, contacts
- Properly implements `Close()` to stop cleanup goroutine

## Adding New Sources

To add a new reconnaissance source (e.g., Subfinder, Amass):

**1. Create source package**
```bash
internal/sources/mytool/
├── mytool.go          # Implements ports.Source
└── mytool_test.go     # Unit tests
```

**2. Implement the Source interface**
```go
// internal/sources/mytool/mytool.go
package mytool

type MyTool struct {
    client httpx.Client
    cache  cache.Cache
    logger logx.Logger
}

func New(logger logx.Logger) ports.Source {
    return &MyTool{
        client: httpx.NewClient(httpx.DefaultConfig()),
        cache:  cache.New(),
        logger: logger.With("source", "mytool"),
    }
}

func (m *MyTool) Name() string { return "mytool" }
func (m *MyTool) Mode() domain.SourceMode { return domain.SourceModePassive }
func (m *MyTool) Type() domain.SourceType { return domain.SourceTypeAPI }

func (m *MyTool) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
    result := domain.NewScanResult(target)

    // 1. Query API (use m.client for HTTP requests)
    // 2. Parse response
    // 3. Create artifacts with metadata
    // 4. Add artifacts to result

    return result, nil
}

func (m *MyTool) Close() error {
    // CRITICAL: Implementar Close() para liberar recursos
    // Ejemplos:
    // - Detener goroutines de background workers
    // - Cerrar conexiones HTTP/DB
    // - Liberar recursos del cache
    m.logger.Debug("closing mytool source")
    return nil
}
```

**3. Register in buildSources()** (`cmd/aethonx/main.go:144`)
```go
if cfg.Sources.MyToolEnabled {
    sources = append(sources, mytool.New(logger))
}
```

**4. Add config flag** (`internal/platform/config/config.go`)
```go
type Sources struct {
    CRTSHEnabled   bool
    RDAPEnabled    bool
    MyToolEnabled  bool  // Add this
}
```

**5. Write tests** (aim for 50%+ coverage)
```go
// internal/sources/mytool/mytool_test.go
func TestMyTool_Run(t *testing.T) {
    logger := logx.New()
    source := New(logger)
    target := *domain.NewTarget("example.com", domain.ScanModePassive)

    result, err := source.Run(context.Background(), target)

    testutil.AssertNoError(t, err, "run should succeed")
    testutil.AssertNotNil(t, result, "result should not be nil")
    testutil.AssertTrue(t, len(result.Artifacts) > 0, "should discover artifacts")
}
```

**6. (Optional) Register with Source Registry**
For auto-discovery, add init() function:
```go
// internal/sources/mytool/registry.go
func init() {
    registry.Global().Register("mytool", factory, ports.SourceMetadata{
        Name:        "mytool",
        Description: "MyTool reconnaissance source",
        Mode:        domain.SourceModePassive,
        Type:        domain.SourceTypeAPI,
    })
}

func factory(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
    return New(logger), nil
}
```

## Source Registry Workflow

The Source Registry implements the **Registry + Factory pattern** to decouple source creation from application code.

### Auto-Registration Pattern

**How it Works**:
1. Each source package has an `init()` function that runs at import time
2. `init()` calls `registry.Global().Register(name, factory, metadata)`
3. Main imports source packages (even with blank import `_`)
4. Sources automatically register themselves before `main()` runs

**Example Source Package Structure**:
```
internal/sources/mytool/
├── mytool.go       # Source implementation
├── mytool_test.go  # Unit tests
└── registry.go     # Auto-registration (init function)
```

**Complete Registration Example**:
```go
// internal/sources/mytool/registry.go
package mytool

import (
    "aethonx/internal/core/ports"
    "aethonx/internal/core/domain"
    "aethonx/internal/platform/registry"
    "aethonx/internal/platform/logx"
)

func init() {
    err := registry.Global().Register("mytool", factory, ports.SourceMetadata{
        Name:        "mytool",
        Description: "MyTool reconnaissance source",
        Author:      "Your Name",
        Version:     "1.0.0",
        Mode:        domain.SourceModePassive,
        Type:        domain.SourceTypeAPI,
    })
    if err != nil {
        panic(err) // Init-time panic is acceptable for registration
    }
}

func factory(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
    // Extract custom config if needed
    apiKey, _ := cfg.Custom["api_key"].(string)

    // Validate required config
    if apiKey == "" && cfg.Enabled {
        return nil, fmt.Errorf("mytool requires api_key in config")
    }

    return New(logger, apiKey), nil
}
```

### Building Sources from Registry

**In main.go**:
```go
import (
    _ "aethonx/internal/sources/crtsh"  // Blank import triggers init()
    _ "aethonx/internal/sources/rdap"
    _ "aethonx/internal/sources/mytool" // Your new source
)

func main() {
    // Load configuration
    cfg := config.Load()

    // Prepare source configs (from ENV, flags, or config file)
    sourceConfigs := map[string]ports.SourceConfig{
        "crtsh": {
            Enabled:  cfg.Sources.CRTSHEnabled,
            Priority: 10,
            Timeout:  30 * time.Second,
        },
        "mytool": {
            Enabled:  cfg.Sources.MyToolEnabled,
            Priority: 8,
            Timeout:  45 * time.Second,
            Custom: map[string]interface{}{
                "api_key": cfg.Sources.MyToolAPIKey,
            },
        },
    }

    // Build sources from registry (automatic!)
    sources, err := registry.Global().Build(sourceConfigs, logger)
    if err != nil {
        logger.Error("failed to build sources", "error", err)
        // Partial success is OK, continue with available sources
    }

    // Sources are ready, sorted by priority
    logger.Info("sources built", "count", len(sources))
}
```

### Registry Benefits

**1. Decoupling**:
- Main doesn't need to know source constructors
- Add/remove sources by importing/unimporting packages
- No manual `buildSources()` function needed

**2. Priority-Based Building**:
- Sources built in priority order (high to low)
- Important sources initialized first
- Priority from config overrides metadata default

**3. Graceful Degradation**:
- Failed source registrations logged but don't stop startup
- Invalid configs skip source but continue with others
- Partial builds are acceptable (some sources OK, others fail)

**4. Advanced Initialization**:
- Sources implementing `AdvancedSource` get `Initialize()` called automatically
- Registry validates and health-checks sources if interfaces present
- Cleanup on failed initialization

**5. Discoverability**:
```bash
# List all registered sources
sources := registry.Global().List()
// ["crtsh", "mytool", "rdap"]

# Get source metadata
meta, exists := registry.Global().GetMetadata("mytool")
if exists {
    fmt.Printf("%s: %s\n", meta.Name, meta.Description)
}
```

### Testing with Registry

**Important**: Clear registry between tests to avoid cross-test pollution:
```go
func TestMyFeature(t *testing.T) {
    // Clear global registry
    registry.Global().Clear()

    // Register test sources
    registry.Global().Register("fake", fakeFactory, fakeMetadata)

    // Test code here
    sources, err := registry.Global().Build(configs, logger)
    testutil.AssertEqual(t, len(sources), 1, "expected 1 source")
}
```

## Artifact Types and Metadata

**34 Artifact Types** are defined in `internal/core/domain/enums.go`:

**Critical artifacts**:
- `ArtifactTypeSubdomain`
- `ArtifactTypeIP`
- `ArtifactTypeEmail`
- `ArtifactTypeURL`
- `ArtifactTypeCertificate`

**Metadata types** (`internal/core/domain/metadata/`):
- `DomainMetadata` - SSL info, DNS records, technologies
- `CertificateMetadata` - Issuer, serial number, validity dates
- `IPMetadata` - Geolocation, ASN, cloud provider
- `ServiceMetadata` - Port, protocol, version, banner
- ... and 8 more specialized types

**Creating artifacts with metadata**:
```go
// With typed metadata
domainMeta := metadata.NewDomainMetadata()
domainMeta.HasSSL = true
domainMeta.SSLIssuer = "Let's Encrypt"

artifact := domain.NewArtifactWithMetadata(
    domain.ArtifactTypeSubdomain,
    "test.example.com",
    "crtsh",
    domainMeta,
)

// Legacy: with string map
artifact.Metadata["cert_issuer"] = "Let's Encrypt"
```

## Testing Conventions

**Test File Naming**:
- `*_test.go` - Unit tests in same package
- `fixtures_test.go` - Test fixtures (domain-specific)
- `mocks_test.go` - Mock implementations

**Assertion Helpers** (`internal/testutil/helpers.go`):
```go
testutil.AssertEqual(t, got, want, "description")
testutil.AssertNotNil(t, value, "description")
testutil.AssertNoError(t, err, "description")
testutil.AssertTrue(t, condition, "description")
testutil.AssertContains(t, slice, element, "description")
```

**Table-Driven Tests** (preferred pattern):
```go
func TestArtifact_Normalize(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"lowercase domain", "EXAMPLE.COM", "example.com"},
        {"remove trailing dot", "example.com.", "example.com"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

## Configuration System

Configuration is loaded from **ENV variables first**, then **CLI flags** override them.

**Priority**: CLI flags > ENV vars > defaults

**Example**:
```bash
# Via environment
export AETHONX_TARGET=example.com
export AETHONX_WORKERS=8
./aethonx

# Via flags (overrides ENV)
./aethonx -target example.com -workers 8

# Mixed
export AETHONX_TIMEOUT=60
./aethonx -target example.com -workers 8  # timeout from ENV, others from flags
```

**Config structure** (`internal/platform/config/config.go`):
```go
type Config struct {
    Target       string
    Active       bool
    Workers      int
    TimeoutS     int
    OutputDir    string
    Sources      Sources
    Outputs      Outputs
}
```

## Deduplication Logic

`DedupeService` (`internal/core/usecases/dedupe_service.go`) handles artifact deduplication:

**Key**: `fmt.Sprintf("%s:%s", artifact.Type, normalizedValue)`

**Normalization rules**:
- Domains: lowercase, remove trailing dot, remove `www.` prefix
- Emails: lowercase
- URLs: lowercase
- IPs: trim spaces

**Source merging**: When duplicates are found, sources are merged:
```go
// artifact1: test.example.com from "crtsh"
// artifact2: test.example.com from "rdap"
// Result: test.example.com from ["crtsh", "rdap"]
```

## Event System (Notifiers)

The orchestrator emits events to `Notifier` observers:

**Event Types** (`internal/core/ports/event.go`):
- `EventTypeScanStarted`
- `EventTypeScanCompleted`
- `EventTypeSourceStarted`
- `EventTypeSourceCompleted`
- `EventTypeSourceFailed`
- `EventTypeArtifactDiscovered` (future)

**Pattern**: Events are emitted **asynchronously** (goroutines) to avoid blocking the scan pipeline.

**Goroutine Management**:
- All notifier goroutines are tracked with `sync.WaitGroup`
- Each notification has a 5-second timeout to prevent hanging
- The orchestrator waits for all notifications to complete before returning
- This prevents goroutine leaks and ensures clean shutdown

## Output Formats

**Table Output** (`internal/adapters/output/table.go`)
- Human-readable terminal table using `tabwriter`
- Shows: target, mode, duration, artifact count, sources used
- Displays artifacts with: type, value, sources, confidence
- Includes warnings and errors sections if present
- Statistics section with artifact counts by type

**JSON Output** (`internal/adapters/output/json.go`)
- **ALWAYS generated** (required for streaming consolidation)
- Structured JSON output to file
- Filename format: `aethonx_{domain}_{timestamp}.json`
- Pretty-printed with 2-space indentation
- Contains full scan result with all metadata, deduplication, and graph
- Two functions: `OutputJSON(dir, result)` for files, `OutputJSONStdout(result, pretty)` for console

## Streaming System (Memory Management)

AethonX implements an **incremental streaming architecture** to prevent Out-of-Memory (OOM) errors when sources return massive datasets (e.g., Wayback with 100k+ URLs, Amass with thousands of subdomains).

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ Source Execution (executeSource)                            │
│  ├─ Run source.Run(ctx, target)                             │
│  ├─ Check: len(artifacts) >= threshold?                     │
│  │   ├─ YES → StreamingWriter.WritePartial(source, result)  │
│  │   │        └─ Free memory: result.Artifacts = nil        │
│  │   └─ NO  → Keep in memory                                │
│  └─ Return sourceResult (with or without artifacts)         │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│ Consolidation (Run)                                         │
│  ├─ consolidateResults() - merge in-memory results          │
│  ├─ MergeService.LoadPartialResults() - load from disk      │
│  ├─ DedupeService.Deduplicate() - deduplicate all artifacts │
│  ├─ GraphService.Build() - build relationship graph         │
│  └─ MergeService.ClearPartialFiles() - cleanup disk         │
└─────────────────────────────────────────────────────────────┘
```

### Key Components

**1. StreamingWriter** (`internal/adapters/output/streaming.go`)
- Writes partial results per source to disk
- Filename format: `aethonx_{target}_{timestamp}_partial_{source}.json`
- Includes artifacts, warnings, errors, and metadata
- **Note**: `TypedMetadata` is NOT serialized (marked with `json:"-"` tag) because it's an interface and cannot be deserialized generically

**2. MergeService** (`internal/core/usecases/merge_service.go`)
- Loads partial results from disk using glob patterns
- Consolidates artifacts, warnings, and errors into the main ScanResult
- Cleans up partial files after successful consolidation

**3. Orchestrator Integration** (`internal/core/usecases/orchestrator.go`)
- `executeSource()` checks threshold and streams if exceeded (line ~265-288)
- `Run()` loads partial results before deduplication (line ~116-132)
- `Run()` clears partial files after finalization (line ~157-164)

### Configuration

**Via environment variable**:
```bash
export AETHONX_STREAMING_THRESHOLD=500
./aethonx -target example.com
```

**Via CLI flag**:
```bash
./aethonx -target example.com -streaming.threshold 5000
```

**Default**: 1000 artifacts per source

### Workflow Example

1. **Small source (< threshold)**: crtsh returns 128 artifacts
   - Artifacts stay in memory
   - No disk write
   - Included directly in deduplication

2. **Large source (≥ threshold)**: wayback returns 50,000 URLs
   - Artifacts written to `aethonx_example.com_20251019_144547_partial_wayback.json`
   - Memory freed: `result.Artifacts = nil`
   - Warning added: "artifacts written to disk (50000 artifacts)"
   - At consolidation: loaded from disk, deduped, then partial file deleted

### Benefits

- ✅ **Scalable**: Handles sources with millions of artifacts
- ✅ **Memory-efficient**: Only holds small results in memory
- ✅ **Reliable**: Partial files preserved if process crashes
- ✅ **Transparent**: Works automatically based on threshold
- ✅ **Complete**: Final result includes full graph and deduplication

### Testing Streaming

```bash
# Low threshold to force streaming
./aethonx -target example.com -streaming.threshold 2

# Check for partial files during execution
ls -la aethonx_out/*_partial_*.json

# Verify streaming was triggered
# Look for log messages: "writing partial result to disk"
```

## Platform Infrastructure

### Core Platform Modules

**httpx Client** (`internal/platform/httpx/`)
- HTTP client with automatic retry logic (exponential backoff)
- Configurable timeouts, max retries, and backoff delays
- Context-aware requests for cancellation support
- Used by sources like RDAP and future HTTP-based tools

**cache Module** (`internal/platform/cache/`)
- In-memory TTL-based cache for API responses
- Thread-safe with mutex-protected operations
- Automatic expiration of stale entries
- Used by RDAP to cache domain lookups (24h TTL)

**rate Limiter** (`internal/platform/rate/`)
- Token bucket algorithm for rate limiting
- Prevents API throttling and ensures respectful querying
- Configurable tokens per second
- Ready for integration with future API-based sources

**errors Module** (`internal/platform/errors/`)
- Centralized error handling and wrapping
- Consistent error types across the application
- Helps with error categorization (network, API, parsing errors)

### Advanced Platform Modules

**Worker Pool** (`internal/platform/workerpool/`)
- Priority-based task scheduling system
- Multiple scheduler strategies: Priority, FIFO, Weighted
- Buffered channels for task queue and results (2x workers)
- Graceful shutdown with WaitGroup and context cancellation
- Task interface: `Execute(ctx)`, `Priority()`, `Weight()`, `Name()`
- Used by orchestrator for intelligent source execution ordering

**Circuit Breaker** (`internal/platform/resilience/circuit_breaker.go`)
- Prevents cascading failures when sources are down
- Three states: Closed (normal), Open (failing), HalfOpen (testing recovery)
- Configurable failure threshold (default: 5), timeout (default: 60s), half-open max (default: 3)
- Automatic state transitions based on success/failure patterns
- `RetryableSource` wrapper combines circuit breaker with retry logic

**Source Registry** (`internal/platform/registry/source_registry.go`)
- Registry + Factory pattern for dynamic source management
- Global singleton: `registry.Global()`
- Auto-registration via `init()` functions in source packages
- Priority-based source building and initialization
- Thread-safe with RWMutex protection
- Validates source configs and handles initialization failures gracefully

**Adaptive Streaming** (`internal/platform/adaptive/streaming_config.go`)
- Dynamic streaming threshold based on real-time memory usage
- Monitors Go runtime memory stats (`runtime.MemStats`)
- Calculates threshold: `(availableMB * 1024 / avgArtifactSizeKB) / 2`
- High memory pressure (>80%) triggers minimum threshold
- Auto-recalculation every update interval (default: 10s)
- Prevents OOM by adapting to system conditions

**Validator** (`internal/platform/validator/validator.go`)
- Comprehensive validation utilities for artifacts
- Domain validators: `IsDomain()`, `IsSubdomain()`, `NormalizeDomain()`
- Network validators: `IsIP()`, `IsIPv4()`, `IsIPv6()`, `IsPort()`, `NormalizeIP()`
- URL validators: `IsURL()`, `NormalizeURL()` (handles scheme, host, default ports)
- Other validators: `IsEmail()`, `IsCertSerial()`, `IsHash()`
- Used by DedupeService for consistent normalization across all artifact types

## Version Information

Versions are embedded at build time via ldflags:

```bash
go build -ldflags "-X main.version=1.0.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" ./cmd/aethonx
```

Or via Makefile (automatic):
```bash
make build
```

Check version:
```bash
./aethonx -version
```

## Code Quality Standards

- **Coverage goal**: 80%+ for new platform modules, 50%+ for sources
- **No import cycles**: Test helpers must be generic
- **Thread safety**: All concurrent code uses sync primitives (Mutex, WaitGroup, channels)
- **Context propagation**: All long-running operations accept `context.Context`
- **Error handling**: Errors are logged but don't stop the entire scan (fail-soft)
- **Logging**: Use structured logging with key-value pairs (`logger.Info("msg", "key", value)`)

## Goroutine Lifecycle Management

AethonX implements strict goroutine lifecycle control to prevent leaks:

**Notifier Goroutines** (orchestrator.go:339-375):
- Tracked with `sync.WaitGroup` to ensure all complete before shutdown
- Each notification has a 5-second timeout to prevent hanging
- orchestrator.Run() waits for all notifications via `notifyWg.Wait()` (line 185)
- Pattern: `Add(1)` before spawn, `defer Done()` inside goroutine

**Source Cleanup** (main.go:88-97):
- All sources implement `Close()` method (mandatory in `ports.Source`)
- Main calls `defer src.Close()` for all sources
- RDAP stops cache cleanup worker in `Close()` (rdap.go:509-518)
- CRT.sh has no background workers, but implements `Close()` for interface compliance

**Signal Handler** (main.go:203-240):
- Goroutine waits for SIGINT/SIGTERM or context cancellation
- Cleanup function calls `signal.Stop()` to remove handler
- Closes signal channel to allow goroutine to exit
- Pattern: select on signal OR `ctx.Done()` for dual termination

**Cache Cleanup Worker** (rdap.go:129):
- Started automatically in RDAP.New() with 1-hour interval
- Returns stop function stored in `r.stopCleanup`
- Called in `Close()` to terminate background goroutine
- Prevents memory leak from never-cleaned cache entries

**Best Practices**:
1. All background goroutines MUST be tracked (WaitGroup, channels, or stop functions)
2. All sources MUST implement `Close()` even if no cleanup needed
3. Use timeouts for all blocking operations (5s for notifications)
4. Always defer cleanup in main (signals, sources)
5. Test with `-race` flag to detect goroutine issues

## Performance Characteristics

- **Worker pool**: Advanced priority-based scheduler (default: 4 workers)
- **Scheduling**: Priority + weight-based task ordering for optimal execution
- **Task weights**: API=30, CLI=70, Builtin=20 (auto-estimated per source)
- **Goroutines per scan**: 1 per worker + N per notifier (N = number of observers)
- **Memory management**: Adaptive streaming with dynamic thresholds
  - Static mode: configurable threshold (default: 1000 artifacts)
  - Adaptive mode: calculates threshold based on runtime memory stats
  - Sources with artifacts < threshold: held in memory
  - Sources with artifacts ≥ threshold: written to disk, memory freed
  - Prevents OOM on massive datasets (Wayback, Amass, etc.)
- **Disk streaming**: Partial JSON files written per source, consolidated at end
- **Resilience**: Circuit breaker pattern prevents cascading failures
- **Typical scan**: <5s for passive scan with 2-3 sources

## Key Files to Understand First

To understand the architecture, read in this order:

### Core Architecture
1. `internal/core/ports/source.go` - Source interface definition
2. `internal/core/domain/artifact.go` - Core entity (Artifact, Target, ScanResult)
3. `internal/core/usecases/orchestrator.go` - Orchestration logic (worker pool, concurrency, streaming)
4. `cmd/aethonx/main.go` - Dependency injection and source registration

### Source Examples
5. `internal/sources/crtsh/crtsh.go` - Simple passive source example
6. `internal/sources/rdap/rdap.go` - Advanced source with caching and httpx client

### Data Processing
7. `internal/core/usecases/dedupe_service.go` - Deduplication and normalization
8. `internal/adapters/output/streaming.go` - Streaming writer for memory management
9. `internal/core/usecases/merge_service.go` - Merge service for consolidating partial results
10. `internal/core/usecases/graph_service.go` - Relationship graph builder

### Advanced Platform
11. `internal/platform/workerpool/worker_pool.go` - Priority-based task scheduler
12. `internal/platform/resilience/circuit_breaker.go` - Circuit breaker for fault tolerance
13. `internal/platform/registry/source_registry.go` - Source registry and factory
14. `internal/platform/adaptive/streaming_config.go` - Adaptive memory management
15. `internal/platform/validator/validator.go` - Validation and normalization utilities
16. `internal/platform/httpx/httpx.go` - HTTP client with retry logic

## Resilience and Fault Tolerance

AethonX implements multiple resilience patterns to handle failures gracefully:

### Circuit Breaker Pattern

**Purpose**: Prevent cascading failures when sources are unreachable or consistently failing.

**States and Transitions**:
```
Closed (Normal) --[5 failures]--> Open (Failing)
       ^                              |
       |                              |
[3 successes]                   [60s timeout]
       |                              |
       |                              v
Half-Open (Testing) <-----------------
```

**Configuration Example**:
```go
breaker := resilience.NewCircuitBreaker(
    5,              // failureThreshold
    60*time.Second, // timeout before half-open
    3,              // halfOpenMax test requests
)

// Check before executing
if !breaker.Allow() {
    return nil, resilience.ErrCircuitOpen
}

// Record result
if err != nil {
    breaker.RecordFailure()
} else {
    breaker.RecordSuccess()
}
```

### RetryableSource Wrapper

**Purpose**: Combine retry logic with circuit breaker for automatic recovery.

**Features**:
- Exponential backoff between retries (configurable multiplier)
- Maximum backoff cap to prevent excessive delays
- Circuit breaker integration (stops retrying when circuit opens)
- Context-aware cancellation

**Usage Example**:
```go
source := crtsh.New(logger)
retryable := resilience.NewRetryableSource(source, resilience.RetryConfig{
    MaxRetries:        3,
    InitialBackoff:    1 * time.Second,
    MaxBackoff:        10 * time.Second,
    BackoffMultiplier: 2.0,
})

// Now retryable can replace source in orchestrator
// Automatic retries + circuit breaker protection
```

### Worker Pool Fault Isolation

**Purpose**: Isolate source failures to prevent blocking the entire scan pipeline.

**Mechanism**:
- Each source executes in separate goroutine via worker pool
- Failed sources return error in `TaskResult` but don't panic
- Orchestrator continues with remaining sources
- Warnings logged for failed sources, included in final ScanResult

**Example Flow**:
```
Source A: Success (128 artifacts)
Source B: Circuit Open (skipped)
Source C: Network Timeout (retry → fail)
Source D: Success (45 artifacts)

Final Result:
- 173 artifacts (A + D)
- Warnings: ["Source B: circuit breaker open", "Source C: network timeout after 3 retries"]
```

### Graceful Degradation

**Philosophy**: Scans should succeed even if some sources fail.

**Implementation**:
- Fail-soft approach: log errors but continue execution
- Partial results better than no results
- Warnings and errors included in ScanResult metadata
- Statistics show which sources succeeded/failed
- JSON output preserves error context for debugging

## Common Pitfalls

1. **Import cycles**: Don't import domain from testutil
2. **Goroutine leaks**: CRITICAL - All sources MUST implement `Close()` to cleanup resources
   - Notifier goroutines are tracked with WaitGroup (orchestrator.go:28)
   - Sources must stop background workers in `Close()` (e.g., RDAP cache cleanup)
   - Signal handler goroutine is properly cleaned up (main.go:233-237)
3. **nil pointer**: Check `result != nil` before accessing fields
4. **Context ignored**: Pass `ctx` to all HTTP requests and long operations
5. **Race conditions**: Run `make test` (uses `-race` flag) before committing
6. **Metadata confusion**: Use TypedMetadata when possible, avoid string map
7. **Missing Close()**: ALL sources MUST implement `Close()` - this is now mandatory in `ports.Source`
8. **Registry pollution in tests**: Always call `registry.Global().Clear()` in test setup to avoid cross-test interference
9. **Forgetting blank imports**: New sources must be imported in main.go (even with `_`) to trigger `init()` registration
10. **Invalid task weights**: Worker pool weight must be 0-100; values outside range are clamped automatically

## File References

Use line number references when discussing specific code:
- Example: "The orchestrator worker pool is at `orchestrator.go:139-164`"
- Example: "Source registration happens in `main.go:143-164`"
