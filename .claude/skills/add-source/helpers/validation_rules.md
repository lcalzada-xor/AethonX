# Validation Rules for Add-Source Skill

## Name Validation

### Rules:
1. Must be lowercase
2. Only alphanumeric characters and underscores allowed
3. Must start with a letter
4. Length between 3 and 30 characters
5. Cannot conflict with existing sources in `internal/sources/`
6. Cannot be a Go reserved keyword
7. Cannot match common package names (http, fmt, json, etc.)

### Examples:
- ✅ Valid: `nuclei`, `custom_scanner`, `my_tool`, `amass2`
- ❌ Invalid: `Nuclei`, `my-tool`, `2tool`, `http`, `x`

## Source Type Validation

### Valid Types:
1. **CLI** - External binary subprocess
   - Requires: binary_name, install_instructions
   - Uses: BaseCLISource
   - Example: subfinder, httpx, nuclei

2. **API** - HTTP API integration
   - Requires: base_url, service_name
   - Uses: BaseAPISource + httpclient
   - Example: crtsh, shodan API

3. **Builtin** - Native Go implementation
   - Requires: No external dependencies
   - Example: rdap

## Mode Validation

### Valid Modes:
1. **Passive** - No direct target contact
   - Examples: Certificate Transparency logs, WHOIS, archive APIs
   - No network probes to target

2. **Active** - Direct target interaction
   - Examples: HTTP probes, port scans, vulnerability scans
   - Requires user consent (--active flag)

3. **Both** - Hybrid mode
   - Adapts behavior based on --active flag
   - Examples: Shodan (InternetDB passive, API active)

## Stage Validation

### Stage Hints:
- **0**: Auto-detect based on dependencies
- **1**: Discovery stage (no inputs, produces domains/IPs/URLs)
- **2**: Probing stage (enriches with metadata, tech detection)
- **3**: Crawl stage (deep analysis, requires active endpoints)

### Semantic Checks:
1. Stage 0 sources should have empty InputArtifacts
2. Stage 1 sources typically have empty InputArtifacts
3. Stage 2+ sources should declare InputArtifacts
4. Stage 3 sources should consume URL or Subdomain artifacts

## Artifact Type Validation

### Known Input Artifacts:
- Domain, Subdomain, IP, URL, Email, Port, Nameserver

### Known Output Artifacts:
- Subdomain, IP, URL, Port, Vulnerability, Technology, Certificate, Email, Nameserver, Service, APIEndpoint, Parameter, Credential, SensitiveFile, StorageBucket

### Validation:
1. OutputArtifacts cannot be empty
2. InputArtifacts should match stage expectations
3. Warn on unknown artifact types (typos)
4. Check logical consistency (e.g., DNS source shouldn't produce Vulnerability)

## Custom Configuration Validation

### Valid Configuration Keys:
- API sources: api_key, base_url, rate_limit, timeout, endpoints
- CLI sources: exec_path, threads, workers, max_files, profile, custom_flags
- General: retries, circuit_breaker, cache_ttl

### Type Validation:
- Strings: api_key, base_url, exec_path, profile
- Integers: threads, workers, max_files, port, timeout
- Floats: rate_limit
- Booleans: enabled, use_cli, verify_ssl
- Slices: sources, patterns, query_types

## Timeout Validation

### Recommendations:
- API sources: 30-60 seconds
- Fast CLI sources: 60-120 seconds
- Slow CLI sources: 120-300 seconds (subfinder, amass)
- Crawling sources: 60-180 seconds

### Warnings:
- < 10s: Likely too short, will timeout
- > 600s: Excessive, consider reducing
- Suggest defaults based on source type

## Priority Validation

### Typical Ranges:
- Stage 0 (passive discovery): 5-10
- Stage 1 (active probing): 15-20
- Stage 2 (enrichment): 12-18
- Stage 3 (deep crawl): 20-25

### Rules:
- Must be between 0 and 100
- Higher priority = executes earlier
- Suggest priority based on stage and mode

## Conflict Detection

### Check for:
1. Source name already exists in `internal/sources/`
2. Binary name conflicts with system commands
3. Import conflicts (name matches stdlib package)
4. Configuration key conflicts with existing sources

## Semantic Validation Matrix

| Stage | InputArtifacts | OutputArtifacts | Mode | Valid? | Reason |
|-------|---------------|----------------|------|--------|---------|
| 0 | Empty | Subdomain | Passive | ✅ | Discovery source |
| 0 | [URL] | Subdomain | Passive | ⚠️ | Stage 0 shouldn't need inputs |
| 1 | Empty | Subdomain, IP | Passive | ✅ | Discovery source |
| 2 | [Subdomain] | IP, Technology | Active | ✅ | Probing/enrichment |
| 2 | Empty | Technology | Active | ⚠️ | Stage 2 should consume inputs |
| 3 | [URL] | Vulnerability | Active | ✅ | Deep crawl/scan |
| 3 | Empty | Vulnerability | Active | ❌ | Stage 3 requires inputs |
| 0 | Empty | Empty | Passive | ❌ | Must produce artifacts |

## Binary Validation (CLI only)

### Checks:
1. Binary exists in PATH
2. Binary is executable
3. Binary responds to version/help flags
4. Binary permissions correct

### Commands to test:
```bash
which <binary_name>
<binary_name> -version
<binary_name> --version
<binary_name> -h
<binary_name> --help
```

## API Validation (API only)

### Checks:
1. Base URL format valid (http/https)
2. Base URL accessible (optional HEAD request)
3. API key format valid (if required)
4. Rate limit reasonable (0.1-100 req/s)

## Examples of Good Validation

### Example 1: CLI Source (Valid)
```
Name: nuclei
Type: CLI
Mode: Active
Stage: 3
InputArtifacts: [URL]
OutputArtifacts: [Vulnerability, Endpoint]
BinaryName: nuclei
Timeout: 120s
Priority: 25
```
✅ All checks pass

### Example 2: CLI Source (Invalid)
```
Name: Nuclei
Type: CLI
Mode: Active
Stage: 3
InputArtifacts: []
OutputArtifacts: [Vulnerability]
```
❌ Issues:
- Name must be lowercase
- Stage 3 requires InputArtifacts
- Missing binary_name

### Example 3: API Source (Valid)
```
Name: virustotal
Type: API
Mode: Passive
Stage: 0
InputArtifacts: []
OutputArtifacts: [Subdomain, IP]
BaseURL: https://www.virustotal.com/api/v3
Timeout: 30s
Priority: 8
RequiresAuth: true
```
✅ All checks pass

### Example 4: Semantic Issue
```
Name: custom_crawler
Type: CLI
Mode: Passive
Stage: 3
InputArtifacts: [URL]
OutputArtifacts: [Endpoint]
```
⚠️ Warning: Stage 3 with Passive mode is unusual (Stage 3 typically active)
⚠️ Warning: Crawling (Stage 3) is typically Active mode

## Recovery Suggestions

When validation fails, provide:
1. Clear explanation of the issue
2. Example of correct format
3. Suggestion for fix
4. Option to retry or abort

### Example Recovery Prompt:
```
❌ Invalid source name: "My-Tool"

Issues:
- Name must be lowercase
- Underscores allowed, hyphens not allowed

Valid examples: my_tool, mytool, custom_scanner

Would you like to:
1. Fix the name (enter new name)
2. Cancel and start over
→
```
