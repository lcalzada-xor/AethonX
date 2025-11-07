# Add-Source Skill for AethonX

Automated source generation skill that creates complete, production-ready reconnaissance sources for AethonX with full scaffolding, tests, and documentation.

## What This Skill Does

This skill automates the **entire process** of adding a new source to AethonX, including:

✅ **Complete source implementation** (CLI/API/Builtin)
✅ **Parser with typed metadata** support
✅ **Registry auto-registration** with factory pattern
✅ **Comprehensive test suite** (registry, factory, integration)
✅ **Configuration integration** (ENV + CLI flags)
✅ **Main.go import** (automatic)
✅ **Full documentation** (README with usage examples)
✅ **Validation** (name conflicts, semantic checks, binary detection)

## Time Savings

**Without skill**: 4-6 hours per source
**With skill**: 10-15 minutes

Eliminates common errors:
- Goroutine leaks (missing Close())
- Import cycles
- Configuration mismatches
- Registry conflicts
- Test coverage gaps

## Quick Start

### Invoke the Skill

Simply tell Claude Code you want to add a source:

```
"I want to add [tool_name] as a source for AethonX"
```

or

```
"Create a new source called [name] that does [description]"
```

### Interactive Process

The skill will guide you through:

1. **Source Information**
   - Name, type (CLI/API/Builtin), mode (Passive/Active)
   - Input/output artifacts
   - Priority and stage

2. **Configuration**
   - Binary name (CLI) or API details
   - Custom config fields (API keys, threads, etc.)
   - Timeouts and rate limits

3. **Validation**
   - Name conflicts
   - Semantic consistency
   - Binary availability

4. **Generation**
   - All source files
   - Configuration updates
   - Tests and documentation

5. **Next Steps**
   - Implementation guidance
   - Testing instructions
   - Usage examples

## Examples

### CLI Source (nuclei)

```
User: "Add nuclei for vulnerability scanning"

Skill generates:
  - internal/sources/nuclei/nuclei.go (CLI implementation)
  - Parser for nuclei JSON output
  - Stage 3 (Crawl) configuration
  - InputConsumer interface (consumes URLs)
  - Tests and docs
```

### API Source (VirusTotal)

```
User: "Add VirusTotal API for subdomain discovery"

Skill generates:
  - internal/sources/virustotal/virustotal.go (API implementation)
  - HTTP client with rate limiting
  - Stage 0 (Discovery) configuration
  - ENV/flag support for API key
  - Tests and docs
```

## Architecture Adherence

The skill enforces AethonX best practices:

### ✅ Registry + Factory Pattern
- Auto-registration via `init()`
- Type-safe factory functions
- Proper metadata declaration

### ✅ BaseCLISource for CLI Tools
- Subprocess management
- OutputHandler pattern
- Context cancellation
- Thread-safe operations

### ✅ BaseAPISource for APIs
- HTTP client with retry
- Rate limiting
- Progress tracking

### ✅ Registry Helpers
- Type-safe config extraction
- ~75% less boilerplate
- GetStringConfig, GetIntConfig, etc.

### ✅ Clean Architecture
- Respects dependency rules
- Inner layers never depend on outer
- Proper port/adapter separation

### ✅ Comprehensive Testing
- Registry validation
- Factory creation
- Race condition detection
- Integration tests

## Generated Files

For a source named `example`:

```
internal/sources/example/
├── example.go              # Main implementation (CLI/API/Builtin)
├── parser.go              # Output parsing logic
├── registry.go            # Auto-registration + factory
├── example_test.go        # Comprehensive test suite
├── fixtures_test.go       # Test fixtures and mocks
└── README.md              # Usage documentation

Configuration updates:
├── internal/platform/config/config.go   # DefaultConfig, ENV, flags
└── cmd/aethonx/main.go                  # Import added
```

## Templates

The skill includes production-ready templates:

### CLI Source Template
- BaseCLISource integration
- OutputHandler implementation
- ExecuteCLI/ExecuteCLIWithStdin support
- Default* methods (Initialize, Validate, HealthCheck)
- Proper error handling

### API Source Template
- BaseAPISource integration
- httpclient with retry + rate limiting
- JSON/XML parsing
- Fail-soft error handling

### Registry Template
- init() auto-registration
- Factory with registry helpers
- SourceMetadata with stage hints
- Input/output artifact declarations

### Parser Template
- Type-safe artifact creation
- Metadata attachment
- Confidence scoring
- Scope validation

### Test Template
- Registry validation
- Factory creation tests
- Close() idempotency
- Context cancellation
- Race condition tests (with -race)

## Validation Rules

### Name Validation
- Lowercase, alphanumeric + underscores
- 3-30 characters
- No conflicts with existing sources
- Not a Go reserved keyword

### Semantic Validation
- Stage consistency (Stage 3 → needs InputArtifacts)
- Mode compatibility (Active tools in passive mode?)
- Artifact type validity
- Timeout reasonableness

### Dependency Validation
- Binary exists (CLI)
- API URL valid (API)
- No import cycles
- Config key uniqueness

## Configuration Integration

### Environment Variables
```bash
AETHONX_SOURCES_{NAME}_ENABLED=true
AETHONX_SOURCES_{NAME}_PRIORITY=20
AETHONX_SOURCES_{NAME}_{CUSTOM_KEY}=value
```

### CLI Flags
```bash
--src.{name}
--src.{name}.priority=20
--src.{name}.{custom_key}=value
```

### Priority
FLAGS > ENV > .env > defaults

## Usage Guide

See [USAGE_EXAMPLE.md](./USAGE_EXAMPLE.md) for:
- Complete walkthroughs
- Multiple source types
- Validation scenarios
- Testing workflows
- Troubleshooting tips

## Helper Guides

Detailed implementation guides:

- **[validation_rules.md](./helpers/validation_rules.md)** - All validation rules and examples
- **[config_injection_guide.md](./helpers/config_injection_guide.md)** - How config updates work
- **[main_updater_guide.md](./helpers/main_updater_guide.md)** - Import management

## Customization After Generation

Generated code is fully customizable:

### 1. Implement Parsing Logic
Edit `parser.go` to handle tool-specific output format.

### 2. Customize Arguments
Update `buildCommandArgs()` for tool-specific flags.

### 3. Add Metadata
Enhance metadata creation with tool-specific fields.

### 4. Add Tests
Implement skipped tests with real fixtures.

### 5. Optimize Performance
Add caching, batching, or streaming as needed.

## Supported Source Types

### CLI Sources
- **Use Case**: External binary tools (subfinder, httpx, nuclei)
- **Base**: BaseCLISource
- **Features**: Subprocess management, OutputHandler, stdin support
- **Examples**: subfinder, httpx, waybackurls, golinkfinderevo

### API Sources
- **Use Case**: HTTP API integrations (crt.sh, Shodan, VirusTotal)
- **Base**: BaseAPISource + httpclient
- **Features**: Retry, rate limiting, JSON parsing
- **Examples**: crtsh, shodan (API mode), rdap

### Builtin Sources
- **Use Case**: Native Go implementations
- **Base**: None (implement from scratch)
- **Features**: No external dependencies
- **Examples**: rdap, custom parsers

## Source Modes

### Passive
- No direct target contact
- OSINT, APIs, archives
- Examples: Certificate Transparency, WHOIS, archive.org

### Active
- Direct target interaction
- HTTP probes, port scans
- Requires `--active` flag

### Both (Hybrid)
- Adapts based on `--active` flag
- Uses `Custom["active_mode"]` to detect
- Example: Shodan (InternetDB passive, API active)

## Stage System

### Stage 0 - Discovery (Auto-detect or no dependencies)
- No InputArtifacts
- Produces: Subdomain, Domain, IP, Email
- Priority: 5-10

### Stage 1 - Probing (Enrichment)
- Inputs: Subdomain, IP
- Produces: Technology, Service, Port
- Priority: 15-20

### Stage 2 - Active Scanning
- Inputs: Subdomain, IP
- Produces: HTTP metadata, technologies
- Priority: 15-20

### Stage 3 - Crawl (Deep Analysis)
- Inputs: URL, Subdomain
- Produces: Vulnerability, Endpoint, Parameter
- Priority: 20-25

## Best Practices Enforced

1. **Always Close()** - All sources implement Close() for cleanup
2. **Context-aware** - Respect context cancellation
3. **Fail-soft** - Partial results > no results
4. **Thread-safe** - Use sync.Mutex for shared state
5. **Progress tracking** - Emit progress updates
6. **Scope validation** - Check target.IsInScope()
7. **Type-safe config** - Use registry helpers
8. **Comprehensive logging** - Structured logging with logx
9. **Error handling** - Warn but don't fail on non-fatal errors
10. **Test coverage** - Race detection with `-race` flag

## Integration with AethonX

Generated sources integrate seamlessly:

### Registry Auto-Registration
```go
func init() {
    registry.Global().Register("newsource", factory, metadata)
}
```

### Factory Pattern
```go
func factory(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
    execPath := registry.GetStringConfig(cfg.Custom, "exec_path", "tool")
    return NewWithConfig(logger, execPath, ...), nil
}
```

### Orchestrator Integration
- Stage-based scheduling (PipelineOrchestrator)
- Concurrent execution with priority
- Streaming for large results
- Deduplication service
- CVE enrichment (if applicable)

## Troubleshooting

### Source Not Registered
**Issue**: `WARN source not registered, skipping source=newsource`

**Fix**:
1. Check main.go has blank import
2. Verify registry.go init() function
3. Rebuild with `-a` flag

### Config Not Applied
**Issue**: Default values used despite ENV/flags

**Fix**:
1. Check loadFromEnv() source block
2. Verify flag apply-back after Parse()
3. Add debug logging in factory

### Tests Failing
**Issue**: Race detector warnings

**Fix**:
1. Use sync.Mutex for shared state
2. Check progress channel closed once
3. Verify BaseCLISource thread-safety

### Binary Not Found
**Issue**: `Error: tool not found in PATH`

**Fix**:
1. Install binary: `go install ...`
2. Or set path: `export AETHONX_SOURCES_TOOL_EXEC_PATH=/path/to/binary`

## Contributing

To improve this skill:

1. **Add new templates** - For specialized source types
2. **Enhance validation** - More semantic checks
3. **Improve error messages** - Clearer guidance
4. **Add examples** - More use cases
5. **Update documentation** - Keep guides current

## Files in This Skill

```
.claude/skills/add-source/
├── README.md                           # This file
├── skill.md                            # Main skill orchestrator
├── USAGE_EXAMPLE.md                    # Complete usage examples
│
├── templates/                          # Code templates
│   ├── cli_source.go.tmpl             # CLI source template
│   ├── api_source.go.tmpl             # API source template
│   ├── builtin_source.go.tmpl         # Builtin source template
│   ├── registry.go.tmpl               # Registry auto-registration
│   ├── parser_cli.go.tmpl             # CLI parser template
│   ├── parser_api.go.tmpl             # API parser template
│   ├── test.go.tmpl                   # Test template
│   ├── fixtures.go.tmpl               # Fixtures template
│   └── readme.md.tmpl                 # README template
│
├── helpers/                            # Implementation guides
│   ├── validation_rules.md            # Validation rules and examples
│   ├── config_injection_guide.md      # Config update guide
│   └── main_updater_guide.md          # Import management guide
│
├── generators/                         # Code generators
│   ├── config_generator.go            # (Future) Config injection
│   ├── main_updater.go                # (Future) main.go updater
│   └── doc_generator.go               # (Future) Doc generation
│
└── validators/                         # Validators
    ├── name_validator.go              # (Future) Name validation
    ├── semantic_validator.go          # (Future) Semantic validation
    └── dependency_validator.go        # (Future) Dependency check
```

## Version History

### v1.0.0 - Initial Release
- Complete template system
- CLI/API source support
- Configuration integration
- Validation framework
- Comprehensive documentation

## License

Part of AethonX project. See main repository for license.

## Support

For questions or issues:
1. Check USAGE_EXAMPLE.md for examples
2. Review existing sources in internal/sources/
3. Read CLAUDE.md for architecture details
4. Open an issue in the repository

---

**Ready to add sources 10x faster? Let's go! 🚀**
