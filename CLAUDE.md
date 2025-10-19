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
│  - Source registration                  │
└────────────┬────────────────────────────┘
             │
┌────────────▼────────────────────────────┐
│  internal/adapters/                     │  ← Adapters (outer layer)
│  ├─ output/     (JSON, Table, etc.)     │
│  ├─ storage/    (SQLite - pending)      │
│  └─ notifiers/  (Webhook, Slack - pending)
└────────────┬────────────────────────────┘
             │
┌────────────▼────────────────────────────┐
│  internal/core/                         │  ← Business logic (inner layer)
│  ├─ domain/     (Entities, Value Objects)
│  ├─ usecases/   (Orchestrator, DedupeService)
│  └─ ports/      (Interfaces: Source, Notifier, Repository)
└────────────┬────────────────────────────┘
             │
┌────────────▼────────────────────────────┐
│  internal/sources/                      │  ← Source implementations
│  ├─ crtsh/      (crt.sh certificate transparency)
│  └─ rdap/       (RDAP WHOIS - pending)
└─────────────────────────────────────────┘
             │
┌────────────▼────────────────────────────┐
│  internal/platform/                     │  ← Infrastructure
│  ├─ config/     (ENV + flags)           │
│  ├─ logx/       (Structured logging)    │
│  ├─ httpx/      (HTTP client - pending) │
│  ├─ cache/      (In-memory - pending)   │
│  └─ rate/       (Rate limiter - pending)│
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
- Executes sources **concurrently** with a **worker pool** (semaphore pattern)
- Consolidates results from all sources
- Deduplicates artifacts using `DedupeService`
- Emits events to `Notifier` observers (async, non-blocking)

**3. Source Interface** (`internal/core/ports/source.go`)
All reconnaissance sources must implement:
```go
type Source interface {
    Name() string
    Mode() domain.SourceMode  // passive, active, or hybrid
    Type() domain.SourceType  // API, CLI, or builtin
    Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error)
}
```

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

## Adding New Sources

To add a new reconnaissance source (e.g., RDAP, Subfinder):

**1. Create source package**
```bash
internal/sources/rdap/
├── rdap.go          # Implements ports.Source
└── rdap_test.go     # Unit tests
```

**2. Implement the Source interface**
```go
// internal/sources/rdap/rdap.go
package rdap

type RDAP struct {
    client httpx.Client
    logger logx.Logger
}

func New(logger logx.Logger) ports.Source {
    return &RDAP{
        logger: logger.With("source", "rdap"),
    }
}

func (r *RDAP) Name() string { return "rdap" }
func (r *RDAP) Mode() domain.SourceMode { return domain.SourceModePassive }
func (r *RDAP) Type() domain.SourceType { return domain.SourceTypeAPI }

func (r *RDAP) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
    result := domain.NewScanResult(target)

    // 1. Query RDAP API
    // 2. Parse response
    // 3. Create artifacts
    // 4. Populate metadata

    return result, nil
}
```

**3. Register in buildSources()** (`cmd/aethonx/main.go:143`)
```go
if cfg.Sources.RDAPEnabled {
    sources = append(sources, rdap.New(logger))
}
```

**4. Add config flag** (`internal/platform/config/config.go`)
```go
type Sources struct {
    CRTSHEnabled bool
    RDAPEnabled  bool  // Add this
}
```

**5. Write tests** (aim for 50%+ coverage)
```go
// internal/sources/rdap/rdap_test.go
func TestRDAP_Run(t *testing.T) {
    logger := logx.New()
    source := New(logger)
    target := *domain.NewTarget("example.com", domain.ScanModePassive)

    result, err := source.Run(context.Background(), target)

    testutil.AssertNoError(t, err, "run should succeed")
    testutil.AssertNotNil(t, result, "result should not be nil")
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

## Performance Characteristics

- **Worker pool**: Limits concurrent source execution (default: 4 workers)
- **Semaphore pattern**: `sem := make(chan struct{}, maxWorkers)` in orchestrator
- **Goroutines per scan**: 1 per source + 1 per notifier event
- **Memory**: Artifacts are held in memory (no streaming yet)
- **Typical scan**: <5s for passive scan with 2-3 sources

## Key Files to Understand First

To understand the architecture, read in this order:

1. `internal/core/ports/source.go` - Source interface
2. `internal/core/domain/artifact.go` - Core entity
3. `internal/core/usecases/orchestrator.go` - Orchestration logic
4. `cmd/aethonx/main.go` - Dependency injection
5. `internal/sources/crtsh/crtsh.go` - Source example
6. `internal/core/usecases/dedupe_service.go` - Deduplication

## Common Pitfalls

1. **Import cycles**: Don't import domain from testutil
2. **Goroutine leaks**: Always use WaitGroup or context cancellation
3. **nil pointer**: Check `result != nil` before accessing fields
4. **Context ignored**: Pass `ctx` to all HTTP requests and long operations
5. **Race conditions**: Run `make test` (uses `-race` flag) before committing
6. **Metadata confusion**: Use TypedMetadata when possible, avoid string map

## File References

Use line number references when discussing specific code:
- Example: "The orchestrator worker pool is at `orchestrator.go:139-164`"
- Example: "Source registration happens in `main.go:143-164`"
