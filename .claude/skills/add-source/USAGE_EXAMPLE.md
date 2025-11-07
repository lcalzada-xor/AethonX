# Add-Source Skill - Usage Examples

Complete walkthrough examples for using the add-source skill.

## Example 1: Adding Nuclei (CLI Source, Active, Stage 3)

### Step-by-Step Process

#### 1. Invoke the Skill

```
User: "I want to add nuclei as a source for AethonX"
```

#### 2. Information Gathering (Interactive Prompts)

```
🔧 AethonX Source Generator
━━━━━━━━━━━━━━━━━━━━━━━━━━━

Let's create a new source for AethonX.

Source Name: nuclei
  ✓ Valid name format
  ✓ No conflicts with existing sources

Source Type:
  [1] CLI - External binary subprocess
  [2] API - HTTP API integration
  [3] Builtin - Native Go implementation

  Select (1-3): 1

Binary Name [nuclei]: nuclei
  ✓ Format valid

Source Mode:
  [1] Passive - No direct target contact (OSINT, APIs, archives)
  [2] Active - Direct target interaction (HTTP probes, scans)
  [3] Both - Hybrid mode (adapts to --active flag)

  Select (1-3): 2

Stage Hint:
  [0] Auto-detect based on dependencies
  [1] Discovery - No inputs, produces domains/IPs/URLs
  [2] Probing - Enriches discoveries with metadata
  [3] Crawl - Deep analysis of active endpoints

  Select (0-3): 3

Input Artifacts (comma-separated, empty for none): URL
  ✓ Valid artifact type: URL

Output Artifacts (comma-separated): Vulnerability,Endpoint
  ✓ Valid artifact types: Vulnerability, Endpoint

Default Timeout (seconds) [120]: 180
  ✓ Valid timeout for active crawl source

Default Priority (0-100) [25]: 25
  ✓ Valid priority for Stage 3 source

Install Instructions: go install -v github.com/projectdiscovery/nuclei/v3/cmd/nuclei@latest

Custom Configuration:
  Add custom config field? (y/n): y

  Field name: templates_path
  Field type:
    [1] string  [2] int  [3] bool  [4] float  [5] []string
  Select: 1
  Default value: ./nuclei-templates
  Description: Path to nuclei templates directory
  ✓ Added: templates_path (string)

  Add another field? (y/n): y

  Field name: severity
  Field type: 5
  Default value (comma-separated): critical,high,medium
  Description: Severity levels to scan for
  ✓ Added: severity ([]string)

  Add another field? (y/n): y

  Field name: rate_limit
  Field type: 2
  Default value: 150
  Description: Rate limit in requests per second
  ✓ Added: rate_limit (int)

  Add another field? (y/n): n

Requires Authentication? (y/n): n
```

#### 3. Validation Summary

```
📋 Configuration Summary
━━━━━━━━━━━━━━━━━━━━━━━━━━━

Source: nuclei
Type: CLI
Mode: Active
Stage: 3 (Crawl)
Priority: 25

Inputs: [URL]
Outputs: [Vulnerability, Endpoint]

Binary: nuclei
Timeout: 180s
Install: go install -v github.com/projectdiscovery/nuclei/v3/cmd/nuclei@latest

Custom Config:
  - templates_path: ./nuclei-templates (string)
  - severity: [critical, high, medium] ([]string)
  - rate_limit: 150 (int)

Auth Required: No

✅ All validations passed
⚠️ Binary 'nuclei' not found in PATH (will need installation)

Proceed with generation? (y/n): y
```

#### 4. Code Generation

```
🔨 Generating source files...

✓ Created internal/sources/nuclei/nuclei.go (487 lines)
✓ Created internal/sources/nuclei/parser.go (145 lines)
✓ Created internal/sources/nuclei/registry.go (54 lines)
✓ Created internal/sources/nuclei/nuclei_test.go (198 lines)
✓ Created internal/sources/nuclei/fixtures_test.go (32 lines)
✓ Created internal/sources/nuclei/README.md (256 lines)

📝 Updating configuration...

✓ Updated internal/platform/config/config.go (DefaultConfig)
✓ Updated internal/platform/config/config.go (loadFromEnv)
✓ Updated internal/platform/config/config.go (loadFromFlags)

🔗 Updating main.go...

✓ Added import to cmd/aethonx/main.go

🎨 Formatting code...

✓ Running goimports on all generated files
✓ Code formatted successfully

🧪 Running tests...

✓ Registry validation test: PASS
✓ Factory test: PASS
✓ Close() test: PASS
⚠️ Run() test: SKIPPED (requires implementation)
⚠️ Integration test: SKIPPED (requires binary)

All critical tests passed (3/3)

✅ Source 'nuclei' generated successfully!
```

#### 5. Next Steps Guide

```
📚 Next Steps
━━━━━━━━━━━━━━━━━━━━━━━━━━━

1. Install the binary:
   $ go install -v github.com/projectdiscovery/nuclei/v3/cmd/nuclei@latest

2. Implement custom parsing logic:
   Edit: internal/sources/nuclei/parser.go

   The parser currently has placeholder logic. Implement:
   - ProcessLine() to parse nuclei JSON output
   - createMetadata() to extract vulnerability details
   - Map nuclei severity to AethonX confidence levels

3. Implement Run() logic:
   Edit: internal/sources/nuclei/nuclei.go:Run()

   Customize:
   - buildCommandArgs() for nuclei-specific flags
   - Response type structure for nuclei JSON format
   - Artifact creation from nuclei findings

4. Add test fixtures:
   Edit: internal/sources/nuclei/fixtures_test.go

   Add sample nuclei output for testing:
   - Valid JSON responses
   - Edge cases (no findings, errors)
   - Different severity levels

5. Build and test:
   $ make build
   $ ./aethonx -t example.com --src.nuclei

6. Run tests with race detection:
   $ go test -race ./internal/sources/nuclei/...

Configuration:
  ENV: AETHONX_SOURCES_NUCLEI_ENABLED=true
  CLI: ./aethonx -t example.com --src.nuclei

Documentation: internal/sources/nuclei/README.md

Happy hacking! 🚀
```

## Example 2: Adding VirusTotal (API Source, Passive, Stage 0)

### Quick Summary (No Full Interactive Display)

**Input:**
- Name: virustotal
- Type: API
- Mode: Passive
- Stage: 0 (Discovery)
- Inputs: (empty)
- Outputs: Subdomain, IP
- Base URL: https://www.virustotal.com/api/v3
- Requires Auth: Yes
- Custom Config:
  - api_key: string (required)
  - rate_limit: float (4.0 req/s)

**Generated Files:**
```
internal/sources/virustotal/
├── virustotal.go         (API implementation)
├── registry.go           (auto-registration)
├── virustotal_test.go    (tests)
├── fixtures_test.go      (test data)
└── README.md             (documentation)
```

**Configuration Added:**
- DefaultConfig with API settings
- ENV variables: `AETHONX_SOURCES_VIRUSTOTAL_*`
- CLI flags: `--src.virustotal.*`

**Usage:**
```bash
export AETHONX_SOURCES_VIRUSTOTAL_API_KEY="your-key-here"
./aethonx -t example.com --src.virustotal
```

## Example 3: Adding Custom Scanner (CLI Source, Both Modes, Stage 2)

**Input:**
- Name: custom_scanner
- Type: CLI
- Binary: custom-scanner
- Mode: Both (adapts to --active)
- Stage: 2 (Probing)
- Inputs: Subdomain
- Outputs: Technology, Service
- Custom Config:
  - profile: string ("fast", "normal", "thorough")
  - threads: int (50)
  - timeout_per_host: int (10)

**Key Implementation Details:**
- Implements InputConsumer for Stage 2
- Uses ExecuteCLIWithStdin to pass subdomains via stdin
- Adapts behavior based on cfg.Custom["active_mode"]
- Extracts technologies and services from scan results

**Generated Code Includes:**
- RunWithInput() method (InputConsumer interface)
- extractInputItems() helper
- Conditional args based on active mode

## Example 4: Validation Failures and Recovery

### Scenario: Invalid Stage Configuration

```
❌ Validation Failed

Issue: Stage 3 source with no InputArtifacts

Stage 3 sources (Crawl) typically consume results from earlier stages.
Your source is configured to run in Stage 3 but has no input artifacts.

Suggestions:
  1. Add InputArtifacts (common: URL, Subdomain)
  2. Change Stage to 0 or 1 if source doesn't need inputs
  3. Review similar sources: golinkfinderevo (Stage 3)

What would you like to do?
  [1] Add InputArtifacts
  [2] Change Stage
  [3] Review configuration
  [4] Cancel

Select: 1

Enter InputArtifacts (comma-separated): URL
✓ Updated: InputArtifacts = [URL]
✓ Validation passed

Proceed with generation? (y/n): y
```

### Scenario: Binary Not Found

```
⚠️ Binary Check Warning

Binary 'my-scanner' not found in PATH

This won't prevent generation, but the source won't work until installed.

Options:
  [1] Continue anyway (I'll install later)
  [2] Specify custom path
  [3] Cancel

Select: 2

Custom binary path: /opt/tools/my-scanner
✓ Binary found at /opt/tools/my-scanner
✓ Binary is executable

Proceed with generation? (y/n): y
```

## Example 5: Complete Workflow with Testing

```bash
# 1. Generate source using skill
# (Interactive prompts completed)

# 2. Install binary
$ go install github.com/tool/cmd@latest

# 3. Implement parser logic
$ $EDITOR internal/sources/mytool/parser.go

# 4. Run tests
$ go test -v ./internal/sources/mytool/
=== RUN   TestRegistry_MyTool
--- PASS: TestRegistry_MyTool (0.00s)
=== RUN   TestFactory_MyTool
--- PASS: TestFactory_MyTool (0.01s)
=== RUN   TestMyTool_Close
--- PASS: TestMyTool_Close (0.00s)
PASS
ok      aethonx/internal/sources/mytool 0.015s

# 5. Build AethonX
$ make build
go build -o aethonx cmd/aethonx/main.go

# 6. Test with target
$ ./aethonx -t example.com --src.mytool
[INFO] Starting scan target=example.com sources=[mytool]
[INFO] Starting mytool scan target=example.com
[INFO] mytool scan completed artifacts=42 duration=15.2s
[INFO] Scan completed total_artifacts=42 duration=15.3s

# 7. Verify output
$ cat aethonx_out/example.com_*.json | jq '.artifacts[] | select(.sources[] == "mytool") | .type' | sort | uniq -c
   25 "Subdomain"
   17 "IP"

# 8. Run integration test with multiple sources
$ ./aethonx -t hackerone.com --src.crtsh --src.mytool --src.httpx -o test_scan/

# Success! ✅
```

## Tips for Successful Source Creation

### 1. Start Simple
Begin with minimal custom config, add complexity later.

### 2. Study Similar Sources
- CLI tool? Look at subfinder, httpx
- API? Look at crtsh, shodan
- Stage 3? Look at golinkfinderevo

### 3. Test Early and Often
Don't wait until everything is implemented to test.

### 4. Use Fixtures
Create test fixtures with real tool output for accurate parsing.

### 5. Handle Errors Gracefully
Use fail-soft pattern - partial results are better than none.

### 6. Document as You Go
Update README.md with actual usage patterns.

### 7. Consider Rate Limits
Be respectful of API rate limits and add appropriate delays.

### 8. Validate Scope
Always check `target.IsInScope()` before creating artifacts.

## Common Patterns

### Pattern 1: Stdin-Based CLI Tool
```go
// Use ExecuteCLIWithStdin for tools that accept input via stdin
items := []string{"url1", "url2", "url3"}
result, stderr, err := s.ExecuteCLIWithStdin(ctx, target, args, items, handler)
```

### Pattern 2: JSON Lines Output
```go
// Many tools output JSON lines (one JSON object per line)
func (h *handler) ProcessLine(line []byte) error {
    var resp Response
    if err := json.Unmarshal(line, &resp); err != nil {
        return nil // Non-fatal, continue
    }
    h.responses = append(h.responses, resp)
    return nil
}
```

### Pattern 3: Progressive Confidence
```go
// Adjust confidence based on verification
artifact.Confidence = domain.ConfidenceLow  // Initial discovery
if resp.Verified {
    artifact.Confidence = domain.ConfidenceHigh  // Confirmed
}
```

### Pattern 4: Metadata Enrichment
```go
// Use typed metadata for rich artifact context
meta := metadata.NewVulnerabilityMetadata()
meta.CVEID = resp.CVE
meta.Severity = resp.Severity
meta.CVSS = resp.CVSS
artifact := domain.NewArtifactWithMetadata(
    domain.ArtifactTypeVulnerability,
    resp.ID,
    sourceName,
    meta,
)
```

## Troubleshooting Generated Code

### Issue: Registry Not Finding Source

**Symptom:**
```
WARN source not registered, skipping source=newsource
```

**Solutions:**
1. Check blank import in main.go
2. Verify init() function in registry.go
3. Rebuild: `go build -a cmd/aethonx/main.go`

### Issue: Config Not Being Applied

**Symptom:**
Default values used despite setting ENV/flags.

**Solutions:**
1. Check loadFromEnv() has source-specific block
2. Verify flag apply-back after Parse()
3. Debug: Add fmt.Printf in factory function

### Issue: Tests Failing with Race Detector

**Symptom:**
```
WARNING: DATA RACE
```

**Solutions:**
1. Use sync.Mutex for shared state
2. Ensure progress channel closed once
3. Check BaseCLISource handles concurrency

## Advanced Customization

After generation, you can customize:

1. **Add custom interfaces**: RateLimitedSource, HealthCheckable
2. **Implement streaming**: Real-time artifact emission
3. **Add caching**: Cache API responses
4. **Custom validation**: Override Validate() method
5. **Metrics**: Add performance tracking
6. **Retry logic**: Custom retry strategies

## Getting Help

If you encounter issues:

1. Check existing sources for patterns
2. Review CLAUDE.md for architecture details
3. Read tool-specific documentation
4. Test individual components in isolation
5. Ask for guidance with specific error messages

Happy source development! 🎉
