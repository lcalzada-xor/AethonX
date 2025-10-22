# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

**AethonX** is a modular reconnaissance engine for passive and active web enumeration, written in Go. It implements **Clean Architecture** (Hexagonal/Ports & Adapters) with a concurrent orchestrator that executes multiple reconnaissance sources in parallel.

Named after the Greek mythology horse Aethon (one of Helios' horses) - just as Aethon illuminated the world, AethonX illuminates exposed digital assets.

## Core Architecture

### Clean Architecture Layers

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
│  └─ output/     (JSON, Table, Streaming)│
└────────────┬────────────────────────────┘
             │
┌────────────▼────────────────────────────┐
│  internal/core/                         │  ← Business logic (inner layer)
│  ├─ domain/     (Entities, Metadata)    │
│  ├─ usecases/   (Orchestrator, Services)│
│  └─ ports/      (Interfaces)            │
└────────────┬────────────────────────────┘
             │
┌────────────▼────────────────────────────┐
│  internal/sources/                      │  ← Source implementations
│  ├─ crtsh/      (Certificate logs)      │
│  ├─ rdap/       (WHOIS queries)         │
│  └─ httpx/      (HTTP probing)          │
└────────────┬────────────────────────────┘
             │
┌────────────▼────────────────────────────┐
│  internal/platform/                     │  ← Infrastructure
│  ├─ config/     (ENV + pflag)           │
│  ├─ logx/       (Structured logging)    │
│  ├─ ui/         (Visual presentation)   │
│  ├─ httpclient/ (HTTP with retry)       │
│  ├─ cache/      (In-memory TTL)         │
│  ├─ rate/       (Token bucket limiter)  │
│  ├─ errors/     (Error handling)        │
│  ├─ workerpool/ (Priority scheduler)    │
│  ├─ resilience/ (Circuit breaker)       │
│  ├─ registry/   (Source registry)       │
│  ├─ adaptive/   (Dynamic streaming)     │
│  └─ validator/  (Validation utilities)  │
└─────────────────────────────────────────┘
```

**Dependency Rule**: Inner layers NEVER depend on outer layers.

### Key Patterns

**1. Ports & Adapters (Hexagonal Architecture)**
- `internal/core/ports/` defines interfaces (ports)
- `internal/sources/`, `internal/adapters/` implement these ports
- Swap implementations without changing business logic

**2. Orchestrator Pattern**
The `PipelineOrchestrator` (`internal/core/usecases/pipeline_orchestrator.go`) executes sources:
- Filters sources by compatibility (passive vs active modes)
- Executes sources **concurrently** with **priority-based scheduling**
- **Streams large results to disk** to prevent OOM
- Consolidates results from all sources (memory + disk)
- Deduplicates artifacts using `DedupeService`
- Builds relationship graph with `GraphService`
- Emits events to `Notifier` observers (async, non-blocking)

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

## Common Commands

### Building
```bash
make build              # Development build
make build VERSION=1.0.0  # With version info
make build-all          # Multi-platform
```

### Testing
```bash
make test              # All tests with coverage
make test-short        # Fast tests (no race detector)
make test-coverage     # Coverage report in browser
make coverage          # Coverage summary
```

### Running

**IMPORTANT**: AethonX uses `pflag` library. You MUST use:
- **Double dash (`--`)** for long flag names: `--target`, `--workers`
- **Single dash (`-`)** for short flags: `-t`, `-w`

```bash
# ✓ CORRECT: Basic passive scan
./aethonx -t example.com
./aethonx --target example.com

# ❌ WRONG: This will fail with a clear error
./aethonx -target example.com

# Active scan with custom workers
./aethonx -t example.com -a -w 8

# Quiet mode (JSON only, no table)
./aethonx -t example.com -q

# Custom timeout and output directory
./aethonx -t example.com -T 60 -o results/

# Streaming tuning for high-volume targets
./aethonx -t example.com -s 500 -w 8

# Disable specific sources
./aethonx -t example.com --src.crtsh=false

# Help and version
./aethonx -h           # Show help
./aethonx -v           # Show version
```

**Available Flags**:

**Core Options:**
- `-t, --target` - Target domain (required)
- `-a, --active` - Enable active reconnaissance
- `-w, --workers` - Concurrent workers (default: 4)
- `-T, --timeout` - Global timeout in seconds (default: 30)
- `-o, --out` - Output directory (default: "aethonx_out")

**Source Options:**
- `--src.crtsh` - Enable/disable crt.sh (default: true)
- `--src.rdap` - Enable/disable RDAP (default: true)
- `--src.httpx` - Enable/disable httpx (default: true)

**Output Options:**
- `-q, --quiet` - Disable table output, JSON only

**Streaming Options:**
- `-s, --streaming` - Artifact threshold for partial writes (default: 1000)

**Resilience Options:**
- `-r, --retries` - Max retries per source (default: 3)
- `--circuit-breaker` - Enable circuit breaker (default: true)

**Network Options:**
- `-p, --proxy` - HTTP(S) proxy URL

## Implemented Sources

**crt.sh** (`internal/sources/crtsh/`)
- Queries Certificate Transparency logs
- Discovers subdomains from SSL/TLS certificates
- Passive reconnaissance (no direct target contact)
- Returns: `ArtifactTypeSubdomain`, `ArtifactTypeCertificate`

**RDAP** (`internal/sources/rdap/`)
- Queries RDAP (Registration Data Access Protocol) for domain info
- In-memory caching (24h TTL) to reduce API calls
- Returns: `ArtifactTypeDomain`, `ArtifactTypeEmail`, `ArtifactTypeNameserver`
- Includes metadata: registrar, registration dates, nameservers, contacts

**httpx** (`internal/sources/httpx/`)
- Executes Project Discovery's httpx CLI tool as subprocess
- HTTP probing and fingerprinting
- Active reconnaissance (requires httpx binary in PATH)
- Returns: `ArtifactTypeURL`, `ArtifactTypeDomain`, `ArtifactTypeTechnology`
- Scan profiles: Fast, Standard, Full
- Flexible JSON parsing with type normalization

## Adding New Sources

To add a new reconnaissance source:

**1. Create source package**
```bash
internal/sources/mytool/
├── mytool.go          # Implements ports.Source
├── mytool_test.go     # Unit tests
└── registry.go        # Auto-registration
```

**2. Implement the Source interface**
```go
// internal/sources/mytool/mytool.go
package mytool

type MyTool struct {
    client httpclient.Client
    cache  cache.Cache
    logger logx.Logger
}

func New(logger logx.Logger) *MyTool {
    return &MyTool{
        client: httpclient.NewClient(httpclient.DefaultConfig()),
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
    // CRITICAL: Implement Close() to free resources
    m.logger.Debug("closing mytool source")
    return nil
}
```

**3. Register with Source Registry**
```go
// internal/sources/mytool/registry.go
package mytool

import (
    "aethonx/internal/core/ports"
    "aethonx/internal/platform/registry"
)

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

**4. Import in main.go**
```go
import (
    _ "aethonx/internal/sources/mytool" // Blank import triggers init()
)
```

## Source Registry Workflow

The Source Registry implements the **Registry + Factory pattern**.

### Auto-Registration Pattern

1. Each source has an `init()` function
2. `init()` calls `registry.Global().Register(name, factory, metadata)`
3. Main imports source packages (even with blank import `_`)
4. Sources auto-register before `main()` runs

### Building Sources from Registry

```go
// In main.go
import (
    _ "aethonx/internal/sources/crtsh"  // Blank import triggers init()
    _ "aethonx/internal/sources/rdap"
)

func main() {
    cfg := config.Load()

    // Prepare source configs
    sourceConfigs := map[string]ports.SourceConfig{
        "crtsh": {
            Enabled:  cfg.Sources.CRTSHEnabled,
            Priority: 10,
        },
    }

    // Build sources from registry (automatic!)
    sources, err := registry.Global().Build(sourceConfigs, logger)

    // Sources are ready, sorted by priority
}
```

## Artifact Types and Metadata

**42 Artifact Types** defined in `internal/core/domain/artifact_types.go`:

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
- And 9 more specialized types

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
```

## Testing Conventions

**Test File Naming**:
- `*_test.go` - Unit tests in same package
- `fixtures_test.go` - Test fixtures (domain-specific)
- `mocks_test.go` - Mock implementations

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

Configuration loaded from **ENV variables first**, then **CLI flags** override.

**Priority**: CLI flags > ENV vars > defaults

**Config structure** (`internal/platform/config/config.go`):

```go
type Config struct {
    Core       CoreConfig       // Target, Active, Workers, TimeoutS
    Source     SourceConfig     // Source-specific configs
    Output     OutputConfig     // Dir, TableDisabled
    Streaming  StreamingConfig  // ArtifactThreshold
    Resilience ResilienceConfig // MaxRetries, CircuitBreaker, etc.
    Network    NetworkConfig    // ProxyURL
}
```

## Deduplication Logic

`DedupeService` (`internal/core/usecases/dedupe_service.go`):

**Key**: `fmt.Sprintf("%s:%s", artifact.Type, normalizedValue)`

**Normalization rules**:
- Domains: lowercase, remove trailing dot, remove `www.`
- Emails: lowercase
- URLs: lowercase
- IPs: trim spaces

**Source merging**: When duplicates found, sources are merged:
```go
// artifact1: test.example.com from "crtsh"
// artifact2: test.example.com from "rdap"
// Result: test.example.com from ["crtsh", "rdap"]
```

## Streaming System (Memory Management)

AethonX implements **incremental streaming** to prevent OOM with massive datasets.

### Architecture

```
Source → Check threshold → Exceed? → WritePartial() → Free memory
                        ↓ No
                        Keep in memory
```

### Key Components

**1. StreamingWriter** (`internal/adapters/output/streaming.go`)
- Writes partial results per source to disk
- Filename: `aethonx_{target}_{timestamp}_partial_{source}.json`

**2. MergeService** (`internal/core/usecases/merge_service.go`)
- Loads partial results from disk
- Consolidates artifacts into main ScanResult
- Cleans up partial files after consolidation

**3. Orchestrator Integration**
- `executeSource()` checks threshold and streams if exceeded
- `Run()` loads partial results before deduplication
- `Run()` clears partial files after finalization

### Configuration

```bash
# Via environment
export AETHONX_STREAMING_THRESHOLD=500
./aethonx -t example.com

# Via CLI flag
./aethonx -t example.com -s 5000

# Default: 1000 artifacts per source
```

## Platform Infrastructure

### Core Modules

**httpclient** (`internal/platform/httpclient/`)
- HTTP client with automatic retry (exponential backoff)
- Configurable timeouts, max retries, backoff delays
- Context-aware for cancellation

**cache** (`internal/platform/cache/`)
- In-memory TTL-based cache
- Thread-safe with mutex
- Auto-expiration of stale entries

**rate** (`internal/platform/rate/`)
- Token bucket algorithm
- Prevents API throttling
- Configurable tokens per second

**workerpool** (`internal/platform/workerpool/`)
- Priority-based task scheduling
- Multiple strategies: Priority, FIFO, Weighted
- Graceful shutdown with context cancellation

**resilience** (`internal/platform/resilience/`)
- Circuit breaker pattern
- Three states: Closed, Open, HalfOpen
- `RetryableSource` wrapper combines breaker + retry

**registry** (`internal/platform/registry/`)
- Registry + Factory pattern
- Global singleton: `registry.Global()`
- Auto-registration via `init()` functions
- Priority-based source building

**validator** (`internal/platform/validator/`)
- Comprehensive validation utilities
- Domain, IP, URL, email validators
- Normalization functions

## Resilience and Fault Tolerance

### Circuit Breaker Pattern

**States**:
```
Closed (Normal) --[5 failures]--> Open (Failing)
       ^                              |
       |                         [60s timeout]
       |                              v
Half-Open (Testing) <-----------------
```

### RetryableSource Wrapper

Combines retry logic with circuit breaker for automatic recovery.

```go
retryable := resilience.NewRetryableSource(source, resilience.RetryConfig{
    MaxRetries:        3,
    InitialBackoff:    1 * time.Second,
    MaxBackoff:        10 * time.Second,
    BackoffMultiplier: 2.0,
})
```

### Graceful Degradation

**Philosophy**: Scans should succeed even if some sources fail.

- Fail-soft approach: log errors but continue
- Partial results better than no results
- Warnings included in ScanResult metadata

## Goroutine Lifecycle Management

**Notifier Goroutines**:
- Tracked with `sync.WaitGroup`
- 5-second timeout per notification
- orchestrator waits for all via `notifyWg.Wait()`

**Source Cleanup**:
- All sources implement `Close()` (mandatory)
- Main calls `defer src.Close()` for all sources

**Signal Handler**:
- Goroutine waits for SIGINT/SIGTERM
- Cleanup calls `signal.Stop()`

**Best Practices**:
1. All background goroutines MUST be tracked
2. All sources MUST implement `Close()`
3. Use timeouts for blocking operations
4. Always defer cleanup in main
5. Test with `-race` flag

## Visual UI System

AethonX implements a **user-friendly visual interface** with spinners, colors, and real-time progress tracking using the **Presenter Pattern**.

### Architecture

```
internal/platform/ui/
├── presenter.go         # Presenter interface
├── pterm_presenter.go   # Visual implementation (PTerm)
├── noop_presenter.go    # Silent implementation (quiet mode)
├── symbols.go           # Status symbols and colors
```

### Presenter Interface

The `Presenter` interface decouples visualization from business logic:

```go
type Presenter interface {
    Start(info ScanInfo)
    StartStage(stage StageInfo)
    FinishStage(stageNum int, duration time.Duration)
    StartSource(stageNum int, sourceName string)
    UpdateSource(sourceName string, artifactCount int)
    FinishSource(sourceName string, status Status, duration time.Duration, artifactCount int)
    Info(msg string)
    Warning(msg string)
    Error(msg string)
    Finish(stats ScanStats)
    Close() error
}
```

### Implementations

**1. PTermPresenter** (Default - Visual Mode)
- Uses [pterm](https://github.com/pterm/pterm) for terminal UI
- Animated spinners for each source
- Color-coded status symbols (⏸ ⣾ ✓ ⚠ ✗)
- Progress tracking with artifact counts
- Beautiful headers and summary tables

**2. NoopPresenter** (Quiet Mode)
- No-op implementation for headless/CI environments
- Used when `-q/--quiet` or `--no-ui` flags are set

### Status Symbols

| Status       | Symbol | Color  | Description           |
|--------------|--------|--------|-----------------------|
| `Pending`    | ⏸      | Gray   | Waiting to execute    |
| `Running`    | ⣾ (spinner) | Cyan | Executing now    |
| `Success`    | ✓      | Green  | Completed OK          |
| `Warning`    | ⚠      | Yellow | Completed with issues |
| `Error`      | ✗      | Red    | Failed                |
| `Skipped`    | ⊘      | Gray   | Skipped by dependency |

### Usage Modes

```bash
# Default: Visual UI with spinners and colors
./aethonx -t example.com

# Quiet mode: No visual UI, JSON only
./aethonx -t example.com -q

# Disable UI: Simple text logs
./aethonx -t example.com --no-ui
```

### Integration

The Presenter is injected into `PipelineOrchestrator`:

```go
// main.go
var presenter ui.Presenter
if cfg.Output.QuietMode {
    presenter = ui.NewNoopPresenter()
} else {
    presenter = ui.NewPTermPresenter()
}

orch := usecases.NewPipelineOrchestrator(usecases.PipelineOrchestratorOptions{
    // ... other options
    Presenter: presenter,
})
```

The orchestrator notifies the presenter at key lifecycle events:
- Scan start/finish
- Stage start/finish (scalable for future multi-stage pipelines)
- Source start/update/finish
- Info/warning/error messages

### Scalability

The Presenter system is **designed for future expansion**:
- **Multi-stage support**: Track multiple stages with different sources
- **Real-time updates**: Update artifact counts as sources discover data
- **Extensible**: Add new Presenter implementations (e.g., web UI, TUI, etc.)
- **Thread-safe**: Safe for concurrent updates from multiple sources

## Common Pitfalls

1. **Import cycles**: Don't import domain from testutil
2. **Goroutine leaks**: ALL sources MUST implement `Close()`
3. **nil pointer**: Check `result != nil` before accessing
4. **Context ignored**: Pass `ctx` to all operations
5. **Race conditions**: Run `make test` (uses `-race`) before committing
6. **Missing Close()**: ALL sources MUST implement `Close()`
7. **Registry pollution**: Call `registry.Global().Clear()` in test setup
8. **Forgetting imports**: New sources must be imported in main.go
9. **Wrong flag syntax**: Use `--target` not `-target`
10. **Presenter lifecycle**: Always call `presenter.Close()` after scan completion

## Key Files

**Core Architecture**:
1. `internal/core/ports/source.go` - Source interface
2. `internal/core/domain/artifact.go` - Core entity
3. `internal/core/usecases/pipeline_orchestrator.go` - Orchestration
4. `cmd/aethonx/main.go` - Dependency injection

**Source Examples**:
5. `internal/sources/crtsh/crtsh.go` - Simple passive source
6. `internal/sources/rdap/rdap.go` - Advanced source with caching
7. `internal/sources/httpx/httpx.go` - CLI wrapper source

**Data Processing**:
8. `internal/core/usecases/dedupe_service.go` - Deduplication
9. `internal/adapters/output/streaming.go` - Streaming writer
10. `internal/core/usecases/merge_service.go` - Merge service

**Platform**:
11. `internal/platform/workerpool/worker_pool.go` - Task scheduler
12. `internal/platform/resilience/circuit_breaker.go` - Circuit breaker
13. `internal/platform/registry/source_registry.go` - Source registry
14. `internal/platform/validator/validator.go` - Validation utilities
15. `internal/platform/config/config.go` - Configuration management

**Visual UI**:
16. `internal/platform/ui/presenter.go` - Presenter interface
17. `internal/platform/ui/pterm_presenter.go` - Visual implementation
18. `internal/platform/ui/noop_presenter.go` - Silent implementation
19. `internal/platform/ui/symbols.go` - Status symbols and colors

## Code References

Use line number references when discussing code:
- Example: "Source registration at `main.go:22-24`"
- Example: "Orchestrator worker pool at `pipeline_orchestrator.go:139-164`"
