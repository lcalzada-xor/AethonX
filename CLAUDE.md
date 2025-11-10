# CLAUDE.md - AethonX Context

## Project
AethonX: Modular recon engine for passive/active web enum. Go. Clean Architecture (Hexagonal/Ports&Adapters). Concurrent orchestrator executes multiple recon sources in parallel.

## Architecture
```
cmd/aethonx (main.go) → CLI entry, DI, config, registry-based source building
internal/adapters/ → output (JSON,Table,Streaming)
internal/core/ → domain (Entities,Metadata), usecases (Orchestrator,Services), ports (Interfaces)
internal/sources/ → common(BaseCLISource), crtsh, rdap, httpx, subfinder, waybackurls, shodan
internal/platform/ → config(ENV+pflag), logx, ui, httpclient, cache, rate, errors, workerpool, resilience, registry, adaptive, validator, cveapi
```
**Dependency Rule**: Inner layers NEVER depend on outer layers.

## Key Patterns
**Ports&Adapters**: `internal/core/ports/` defines interfaces, `internal/sources/` implements
**Orchestrator**: `PipelineOrchestrator` at `internal/core/usecases/pipeline_orchestrator.go` - filters sources by mode, executes concurrently with priority scheduling, streams large results to disk (OOM prevention), consolidates, dedupes, builds graph, emits events
**Source Interface**: `internal/core/ports/source.go`
```go
type Source interface {
    Name() string
    Mode() domain.SourceMode  // passive|active|both(hybrid)
    Type() domain.SourceType  // API|CLI|builtin
    Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error)
    Close() error             // MANDATORY
}
```
**Source Modes**: Passive (OSINT,APIs,no target contact), Active (HTTP,DNS,port scan), Both (hybrid adapts to `--active` flag via `Custom["active_mode"]`)
**Optional Interfaces**: AdvancedSource (Initialize,Validate,HealthCheck), StreamingSource (channels), RateLimitedSource

## Commands
Build: `make build` | Test: `make test` (with `-race`) | Run: `./aethonx -t example.com`
**Flags**: pflag library. MUST use `--` for long (`--target`), `-` for short (`-t`). Priority: CLI>ENV>.env>defaults

**Core Flags**: `-t/--target`, `-a/--active`, `-w/--workers(16)`, `-T/--timeout(30)`, `-o/--out(aethonx_out)`, `-q/--quiet`, `-s/--streaming(1000)`, `-r/--retries(3)`, `--circuit-breaker(true)`, `-p/--proxy`
**Source Flags**: `--src.{crtsh|rdap|waybackurls|subfinder|httpx|shodan|golinkfinderevo|retirejs}`, `--src.shodan.api_key`, `--src.shodan.use_cli`, `--src.shodan.rate_limit(1.0)`, `--src.golinkfinderevo.profile(standard)`, `--src.golinkfinderevo.workers(20)`, `--src.golinkfinderevo.max-script-files(50)`, `--src.golinkfinderevo.max-html-files(50)`, `--src.golinkfinderevo.gf-patterns(all)`, `--src.retirejs.severity(medium)`, `--src.retirejs.max-files(100)`, `--src.retirejs.prefer-local(true)`, `--src.retirejs.deep`, `--src.retirejs.include-osv`
**Enrichment Flags**: `--enrich(true)`, `--enrich-nvd-api-key`, `--enrich-provider(nvd)`, `--enrich-cache-ttl(168h)`

**.env file**: Auto-loads from CWD. `cp .env.example .env`. Supports: `AETHONX_SOURCES_SHODAN_API_KEY`, `AETHONX_SRC_SHODAN_API_KEY` (both formats work), `AETHONX_ENRICHMENT_NVD_API_KEY`, etc.

## Sources
**crtsh** (`internal/sources/crtsh/`): Certificate Transparency logs, passive, returns Subdomain+Certificate
**rdap** (`internal/sources/rdap/`): RDAP protocol, 24h cache, returns Domain+Email+Nameserver, metadata: registrar,dates,contacts
**subfinder** (`internal/sources/subfinder/`): CLI subprocess, multi-source subdomain discovery (30+ sources), passive, requires binary in PATH
**httpx** (`internal/sources/httpx/`): CLI subprocess, HTTP probing/fingerprinting, active, profiles: Fast|Standard|Full
**shodan** (`internal/sources/shodan/`): Hybrid: InternetDB(FREE,no key)+DNS fallback(Cloudflare→Google→System)+REST API(free+paid endpoints)+CLI. Passive. Max endpoint execution philosophy, graceful degradation. InternetDB: `https://internetdb.shodan.io/{ip}`, 1req/s, returns IP+Port+Subdomain+Vulnerability+Technology. DNS: Cloudflare DoH primary, Google fallback, system final. API: FREE endpoints (/shodan/host/count, /api-info, /tools/myip), PAID endpoints tolerate failures (/dns/domain, /shodan/host/search, /shodan/host/{ip}). Priority:12. Modes: No key(InternetDB+DNS), Free key(+free API), Paid(+paid API)
**waybackurls** (`internal/sources/waybackurls/`): Internet Archive, historical URLs, passive, returns URL+Subdomain, priority:5
**golinkfinderevo** (`internal/sources/golinkfinderevo/`): CLI subprocess, endpoint/secret discovery in JS/HTML, active, Stage 3 (Crawl). Consumes alive HTTP URLs from httpx (max 50 JS + 50 HTML files). GF integration with 10 modern templates (api-keys, jwt, credentials, sqli, xss, sensitive-files, endpoints, cloud-resources, crypto, custom-params). Profiles: Quick(10 workers,30s,no recursion,25 files)|Standard(20 workers,60s,1 level,50 files)|Deep(30 workers,120s,2 levels,100 files). Priority:20, StageHint:3. Requires `golinkfinder` binary: https://github.com/lcalzada-xor/GoLinkfinderEVO. InputArtifacts: ArtifactTypeURL. OutputArtifacts: Endpoint+Parameter+Credential+SensitiveFile+API+StorageBucket+JavaScript(for Stage 4). Smart filtering by file extension (.js,.html) and HTTP status (200-399), requires DomainMetadata from httpx. Emits ArtifactTypeJavaScript with SourceURL for Stage 4 consumption
**retirejs** (`internal/sources/retirejs/`): CLI subprocess, JavaScript library vulnerability detection, active, Stage 4 (CVE Assessment). Consumes ArtifactTypeJavaScript from golinkfinderevo, reutilizes downloaded JS files (NO re-download). Detects 200+ libraries (jQuery, Angular, React, lodash, moment, etc.) with known CVEs. Returns: Vulnerability(CVE IDs,CWE,severity,GHSA), Technology(lib detected), JavaScript(files analyzed). Metadata: JavaScriptMetadata(component,version,detection_method,vuln_count,highest_severity). Priority:25, StageHint:4. Severity filtering: none|low|medium|high|critical. Modes: prefer_local(true, reuse golinkfinder files)|fallback_download(true, download if no locals). Output: jsonsimple format. Options: --deep(deep scan), --include-osv(OSV advisories), --no-cache(disable cache). Requires `retire` binary: npm install -g retire (https://github.com/RetireJS/retire.js). ENV: AETHONX_SOURCES_RETIREJS_*. CLI: --src.retirejs, --src.retirejs.severity, --src.retirejs.max-files(100), --src.retirejs.prefer-local, --src.retirejs.fallback-download. Auto-enriched with NVD data via CVE Enrichment service

## Adding Source
1. Create `internal/sources/mytool/mytool.go` (implement Source interface with Close()), `mytool_test.go`, `registry.go`
2. Registry: `func init() { registry.Global().Register("mytool", factory, metadata) }`
3. Factory: `func factory(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) { return New(logger), nil }`
4. Import in main.go: `_ "aethonx/internal/sources/mytool"`

## BaseCLISource (`internal/sources/common/cli_source.go`)
Reusable abstraction for CLI sources. Handles: subprocess lifecycle, context cancellation, stdout/stderr pipes, thread-safe process tracking, idempotent Close(), progress channels, Default* methods.
**OutputHandler Interface**:
```go
type OutputHandler interface {
    ProcessLine(line []byte) error  // real-time stdout processing
    Finalize() error                // cleanup after all lines
}
```
**Thread-safe**: mutex-protected cmd, chClosed flag prevents double-close panics, passes `-race`
**Default Methods**: DefaultInitialize (verify binary in PATH), DefaultValidate, DefaultHealthCheck (-version/-h), DefaultStream, ProcessOutput (stdin mode)
**Usage**: 1.Embed BaseCLISource 2.Implement OutputHandler 3.Use ExecuteCLI in Run() 4.Use Default*
**Special Cases**: HTTPx (stdin support, ProcessOutput method)

## Registry Helpers (`internal/platform/registry/helpers.go`)
Type-safe config extraction. Functions: `GetStringConfig`, `GetIntConfig`, `GetBoolConfig`, `GetDurationConfig`, `GetSliceConfig`, `GetFloat64Config`. Validation: `ValidateRequiredString`, `ValidatePositiveInt`, `ValidateIntRange`, `ValidateNonNegativeInt`, `ValidatePositiveDuration`, `ValidateNonEmptySlice`. Benefits: type/nil safety, defaults, ~75% code reduction.

## Registry Workflow
Registry+Factory pattern. Auto-registration: source `init()` calls `registry.Global().Register(name,factory,metadata)`, main imports (blank `_`), sources register before `main()` runs. Building: `registry.Global().Build(sourceConfigs, logger)` returns sources sorted by priority.

## CVE Enrichment
Auto-enriches vulnerability artifacts with NVD/circl.lu data. Flow: Scan→Vulns→EnrichmentService→[NVD→circl]→Enhanced. Cache: 7d TTL. Integration: runs after dedup in orchestrator (`pipeline_orchestrator.go:331-337`).
**Providers**: NVD (primary, API 2.0, `https://services.nvd.nist.gov/rest/json/cves/2.0`, 0.6req/s no key / 50req/s with key, full CVSS v2/v3, CWE, CPE, refs. Get key: https://nvd.nist.gov/developers/request-an-api-key). circl.lu (fallback, `https://cve.circl.lu/api`, no limit, basic data). Multi-provider: NVD first, circl fallback.
**Config**: ENV (`AETHONX_ENRICHMENT_ENABLED(true)`, `AETHONX_ENRICHMENT_NVD_API_KEY`, `AETHONX_ENRICHMENT_PROVIDER(nvd)`, `AETHONX_ENRICHMENT_CACHE_TTL(168h)`, `AETHONX_ENRICHMENT_TIMEOUT(10s)`, `AETHONX_ENRICHMENT_MAX_CONCURRENT(5)`). CLI (`--enrich`, `--no-enrich`, `--enrich-nvd-api-key`, `--enrich-provider`).
**Enriched Data (40+ fields)**: CVE ID, source_identifier, vuln_status, dates, CVSS v2/v3 (vector,score,severity,metrics), CWE[], CPE[], refs[]. Example: CVE-2021-44228 CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H, score:10.0, CRITICAL.
**Files**: `internal/platform/cveapi/{types,enrichment_service,converter,providers/{nvd,circl}}.go`, `internal/core/usecases/vulnerability_enrichment_service.go`, `internal/core/domain/metadata/vulnerability.go`
**Features**: provider pattern, auto fallback, in-mem cache, rate limiting, concurrent enrichment, fail-soft, skip dupes, error tracking.

## Stage 3 (Crawl) & GF Integration
**Stage 3** executes after httpx (Stage 1) completes. Sources with `StageHint:3` consume alive HTTP URLs to perform deep content analysis.
**golinkfinderevo** runs in Stage 3, analyzing JS/HTML files for endpoints, API routes, secrets, and sensitive data. Implements smart filtering: selects only alive URLs (status 200-399) with `.js` or `.html` extensions, max 50 of each type. Requires `DomainMetadata` from httpx.
**GF Templates** (`internal/platform/gf_templates/`): 10 modern pattern templates for detecting embedded artifacts:
1. **api-keys.json**: AWS (AKIA*), GCP, Azure, GitHub (ghp_*), Slack (xoxb-*), Stripe, SendGrid, Twilio, DigitalOcean, Mailgun
2. **jwt.json**: JWT tokens (eyJ*.eyJ*.*)
3. **credentials.json**: passwords, secrets, tokens, Basic/Bearer auth
4. **sqli.json**: SQL injection-prone parameters (id, user, search, filter, query, etc.)
5. **xss.json**: XSS-vulnerable parameters (q, query, callback, redirect, url, etc.)
6. **sensitive-files.json**: .env, config files, backups, .git/, composer.json, package.json
7. **endpoints.json**: API routes (/api/v*, /graphql, REST patterns)
8. **cloud-resources.json**: S3 buckets, Azure blobs, GCP storage (s3.amazonaws.com, blob.core.windows.net, storage.googleapis.com)
9. **crypto.json**: Private keys, certificates, SSH keys (BEGIN RSA/EC/OPENSSH PRIVATE KEY)
10. **custom-params.json**: admin, debug, dev, test, command injection (cmd, exec, command, etc.)
**Usage**: Set `AETHONX_SOURCES_GOLINKFINDEREVO_GF_PATTERNS=all` or comma-separated list. Templates in `./internal/platform/gf_templates/` directory. See `internal/platform/gf_templates/README.md` for pattern documentation.

## Stage 4 (CVE Assessment) & retire.js Integration
**Stage 4** executes after Stage 3 (Crawl) completes. Sources with `StageHint:4` perform vulnerability analysis on JavaScript libraries and components identified in previous stages.
**Pipeline Flow**:
```
Stage 0 (Discovery) → Stage 1 (Probing - httpx) → Stage 3 (Crawl - golinkfinderevo) → Stage 4 (CVE Assessment - retirejs) → Output
                                                               ↓ (JS files + metadata)
                                                               ↓ (ArtifactTypeJavaScript with SourceURL)
                                                               ↓ (File reuse - NO re-download)
```
**retirejs** (`internal/sources/retirejs/`): Active mode, Priority:25, StageHint:4. Implements InputConsumer interface to receive ArtifactTypeJavaScript from golinkfinderevo. Two execution modes:
1. **Prefer Local Mode** (default): Extracts FilePath from JavaScriptMetadata → scans local files → NO bandwidth usage
2. **Fallback Download Mode**: If no local files, extracts SourceURL → downloads JS files → scans downloaded files

**Key Components**:
- **FileManager** (`file_manager.go`): Extracts local JS file paths from artifacts, deduplicates, groups by directory to minimize retire CLI calls
- **Parser** (`parser.go`): Parses retire.js JSON output (jsonsimple format), creates 3 artifact types: Vulnerability (CVE data), Technology (library info), JavaScript (analyzed files)
- **Downloader** (`downloader.go`): Fallback mode, downloads JS files concurrently (max 5 parallel), respects maxFiles limit, generates valid filenames

**JavaScriptMetadata** (`internal/core/domain/metadata/javascript.go`): Metadata for JavaScript artifacts. Fields: FilePath (local path for reuse), Component (library name), Version, DetectionMethod (filecontent|filename|hash), License, VulnCount (number of CVEs), HighestSeverity (critical|high|medium|low), IsMinified, SourceURL (original URL for fallback), NPMName

**Configuration**:
ENV: `AETHONX_SOURCES_RETIREJS_ENABLED(true)`, `AETHONX_SOURCES_RETIREJS_SEVERITY(medium)`, `AETHONX_SOURCES_RETIREJS_DEEP(false)`, `AETHONX_SOURCES_RETIREJS_MAX_FILES(100)`, `AETHONX_SOURCES_RETIREJS_PREFER_LOCAL(true)`, `AETHONX_SOURCES_RETIREJS_FALLBACK_DOWNLOAD(true)`
CLI: `--src.retirejs`, `--src.retirejs.severity=(none|low|medium|high|critical)`, `--src.retirejs.deep`, `--src.retirejs.include-osv`, `--src.retirejs.max-files`, `--src.retirejs.prefer-local`, `--src.retirejs.fallback-download`

**Integration with CVE Enrichment**: Vulnerability artifacts emitted by retirejs are automatically enriched with full NVD data (CVSS v2/v3, CWE mappings, CPE, references) via the CVE Enrichment service (orchestrator line 331-337). Cache TTL: 7 days.

**Example Usage**:
```bash
# Stage 3 + Stage 4 pipeline (golinkfinderevo + retirejs)
./aethonx -t example.com --active --src.httpx --src.golinkfinderevo --src.retirejs

# Only high/critical vulnerabilities
./aethonx -t example.com --active --src.retirejs --src.retirejs.severity=high

# Force download mode (disable local file reuse)
./aethonx -t example.com --active --src.retirejs --src.retirejs.prefer-local=false

# Deep scan with OSV advisories
./aethonx -t example.com --active --src.retirejs --src.retirejs.deep --src.retirejs.include-osv
```

**Performance Benefits**: Reusing local JS files from golinkfinderevo saves 80-90% bandwidth and 2-3x faster execution vs. re-downloading. Example: 87 JS files analyzed with 0 downloads needed.

## Artifact Types (42 total)
Critical: Subdomain, IP, Email, URL, Certificate. Metadata types (13 total, `internal/core/domain/metadata/`): DomainMetadata (SSL,DNS,techs), CertificateMetadata (issuer,serial,dates), IPMetadata (geo,ASN,cloud), ServiceMetadata (port,protocol,version,banner), VulnerabilityMetadata (40+ CVE fields), etc.
Creating: `artifact := domain.NewArtifactWithMetadata(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh", domainMeta)`

## Artifact Structure (Enhanced Context Fields)
**Structured Context Fields** (3 new domain types at `internal/core/domain/`):

1. **DiscoveryContext** - Traceability metadata (where/how artifact was found):
   - `SourceURL`: URL/file where discovered
   - `SourceResource`: Specific resource containing the finding
   - `LineNumber`: Line number in source file
   - `Context`: Surrounding text/code context
   - `MatchPattern`: Pattern/rule that matched (e.g., GF pattern name)
   - Usage: `artifact.SetDiscoveryContext(&domain.DiscoveryContext{SourceURL: url, LineNumber: 42})`

2. **SecurityContext** - Security classification and risk assessment:
   - `Severity`: Risk level (Critical|High|Medium|Low|Info)
   - `VulnerabilityTypes`: []VulnerabilityType (SQLi, XSS, Credential, APIKey, Token, DBConnection, InternalIP)
   - `TokenType`: Authentication token classification (JWT, GitHub, Slack, OAuth2, AWS, GCP, Azure, Stripe, Twilio)
   - `CloudProvider`: Cloud service provider (AWS, GCP, Azure)
   - `ExposureType`: How artifact is exposed (e.g., "internal_network", "connection_string")
   - `IsSensitive`: Boolean flag for sensitive data
   - Usage: `artifact.SetSecurityContext(domain.NewSecurityContext().WithSeverity(domain.SeverityHigh).WithVulnerabilityType(domain.VulnTypeSQLi))`

3. **Classification** - Categorization metadata:
   - `ResourceType`: Web resource type (Static|Dynamic|API|Endpoint)
   - `ParameterType`: Parameter classification (Query|Path|Body|Header)
   - `DataType`: Data classification (Credential, Email, InternalIP, DBConnection, OAuthToken, DeveloperNote, APIKey, JWT, Password, Secret)
   - `IsExternal`: Boolean for external domain flag
   - `Category`: Free-form additional category
   - Usage: `artifact.SetClassification(domain.NewClassification().WithDataType(domain.DataTypeAPIKey).WithExternal(true))`

**Tags Usage Rules**:
- ✅ **Good (Simple categories)**: `alive`, `dead`, `http-success`, `http-redirect` - single-word status/category tags
- ❌ **Bad (Structured data)**: `severity:high`, `line:123`, `discovered_from:URL`, `provider:aws` - use structured fields instead
- **Migration**: All golinkfinderevo structured tags moved to proper context fields
- **Benefits**: Type-safe enums, structured queries (`severity == Critical`), intelligent merge logic, clean JSON schema

## Deduplication (`internal/core/usecases/dedupe_service.go`)
Key: `fmt.Sprintf("%s:%s", artifact.Type, normalizedValue)`. Normalization: domains (lowercase, remove trailing dot, remove www.), emails (lowercase), URLs (lowercase), IPs (trim). Source merging: duplicates merge sources arrays.

## Streaming (OOM prevention)
Incremental streaming to disk. Flow: Source→Check threshold→Exceed?→WritePartial()→Free memory. Components: StreamingWriter (`internal/adapters/output/streaming.go`, writes partial per source `aethonx_{target}_{timestamp}_partial_{source}.json`), MergeService (`internal/core/usecases/merge_service.go`, loads/consolidates/cleans partials), Orchestrator (executeSource checks threshold, Run loads partials before dedup, clears after). Config: `-s 5000` or `AETHONX_STREAMING_THRESHOLD=500`. Default: 1000 artifacts/source.

## Platform Modules
**httpclient**: HTTP client, auto retry, exp backoff, timeouts, context-aware
**cache**: In-mem TTL cache, thread-safe mutex, auto-expiration
**rate**: Token bucket, configurable tokens/sec
**workerpool**: Priority-based scheduling, strategies (Priority|FIFO|Weighted), graceful shutdown
**resilience**: Circuit breaker (Closed→Open→HalfOpen), RetryableSource wrapper
**registry**: Registry+Factory, Global singleton, auto-registration via init(), priority building
**validator**: Domain/IP/URL/email validators, normalization

## Resilience
Circuit Breaker: Closed(normal)→[5 failures]→Open(failing)→[60s timeout]→HalfOpen(testing)→Closed. RetryableSource: combines retry+breaker. Graceful degradation: fail-soft, log errors, continue, partial results OK, warnings in metadata.

## Goroutine Lifecycle
Notifier: tracked with WaitGroup, 5s timeout/notification, orchestrator waits. Source Cleanup: all sources implement Close(), main defers src.Close(). Signal Handler: goroutine waits SIGINT/SIGTERM, cleanup calls signal.Stop(). Best Practices: track all goroutines, all sources MUST Close(), use timeouts, defer cleanup, test with `-race`.

## Visual UI (`internal/platform/ui/`)
Presenter Pattern. Files: presenter.go (interface), custom_presenter.go (visual/pretty), raw_presenter.go (log/text/JSON), global_progress.go (global bar+spinner), symbols.go (status symbols/colors), metrics.go, terminal/ansi.go.
**Design**: single global progress bar (not per-source), no goroutines for UI (sync updates by orchestrator), accumulate results (display at stage completion), in-place updates (ANSI cursor control), integrated spinner (advances on Render()).
**Presenter Interface**: Start, StartStage, FinishStage, StartSource, UpdateSource, FinishSource, Info, Warning, Error, Finish, Close.
**CustomPresenter** (default/pretty): ASCII header, global bar with source dashboard format `⠋ [██████████▓░░░░░] 50% | (1/3) | 2.4s | [httpx ⠋] [rdap ✓] [crtsh ○]`, spinner (⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏ rotates 200ms), color-coded progress (cyan<50%→yellow<100%→green), accumulates results, clean summary, thread-safe mutex.
**RawPresenter**: log-based (headless/CI), logfmt/JSON, structured with timestamps.
**Status Symbols**: Pending(⏸ gray), Running(spinner cyan), Success(✓ green), Warning(⚠ yellow), Error(✗ red), Skipped(⊘ gray).
**Usage**: Default (visual), `-q` (quiet/JSON only), `--no-ui` (simple logs).
**GlobalProgress**: thread-safe RWMutex, in-place ANSI, independent spinner goroutine (200ms ticker), ETA calc, slow source detect (⏱ if >5s), artifact tracking with velocity, smart coloring (50%/75%/100%), per-source status tracking, individual source spinners, stateful (totalSources,completedSources,currentSource,artifacts,timings,statuses). API: InitializeSources, Start, UpdateCurrent, UpdateSourceStatus, IncrementCompleted, UpdateArtifactCount, Render, Stop, Clear.
**Spinner**: Unicode Braille patterns ⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏, 200ms, smooth, changes to ✓ when complete.
**Visual**: Progress colors (cyan→yellow→bright yellow→green), animated bar (growing edge `▓▒░` rotates 200ms), ETA display, artifact counter+velocity, slow indicator, source dashboard `| [httpx ⠋] [rdap ✓] [crtsh ✖]` with per-source status+spinners.
**Integration**: Injected into PipelineOrchestrator, notifies at scan start/finish, stage start/finish, source start/update/finish, messages.
**Scalability**: multi-stage support (tracks stage nums), real-time UpdateSource, extensible (easy new implementations), thread-safe (mutexes).

## Common Pitfalls
1.Import cycles (no domain from testutil) 2.Goroutine leaks (ALL sources MUST Close()) 3.nil pointer checks 4.Context ignored (pass ctx) 5.Race conditions (`make test` uses `-race`) 6.Missing Close() 7.Registry pollution (Clear() in test setup) 8.Forgetting imports (new sources in main.go) 9.Wrong flag syntax (`--target` not `-target`) 10.Presenter lifecycle (call Close() after scan)

## Key Files
Core: 1.`internal/core/ports/source.go` 2.`internal/core/domain/artifact.go` 3.`internal/core/usecases/pipeline_orchestrator.go` 4.`cmd/aethonx/main.go`
Domain Context: 5.`internal/core/domain/discovery_context.go` 6.`internal/core/domain/security_context.go` 7.`internal/core/domain/classification.go`
CLI Abstractions: 8.`internal/sources/common/cli_source.go` 9.`internal/platform/registry/helpers.go`
Sources: 10.`internal/sources/{crtsh,rdap,subfinder,httpx,waybackurls,shodan,golinkfinderevo}/`
Data: 11.`internal/core/usecases/dedupe_service.go` 12.`internal/adapters/output/streaming.go` 13.`internal/core/usecases/merge_service.go`
Platform: 14.`internal/platform/{workerpool,resilience,registry,validator,config,cveapi}/`
UI: 15.`internal/platform/ui/{presenter,custom_presenter,raw_presenter,global_progress,symbols}.go`
GF Templates: 16.`internal/platform/gf_templates/` (10 modern templates for golinkfinderevo pattern matching)

## Testing
Files: `*_test.go` (unit tests same package), `fixtures_test.go` (test fixtures), `mocks_test.go` (mocks). Preferred: table-driven tests. Always test with `-race`. Use `registry.Global().Clear()` in test setup.

## Config (`internal/platform/config/config.go`)
```go
type Config struct {
    Core       CoreConfig       // Target,Active,Workers,TimeoutS
    Source     SourceConfig     // Source-specific configs
    Output     OutputConfig     // Dir,TableDisabled
    Streaming  StreamingConfig  // ArtifactThreshold
    Resilience ResilienceConfig // MaxRetries,CircuitBreaker
    Network    NetworkConfig    // ProxyURL
    Enrichment EnrichmentConfig // Enabled,Provider,NVDAPIKey,CacheTTL,Timeout,MaxConcurrent
}
```
