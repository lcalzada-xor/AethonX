# Add Source Skill - AethonX

Generate a new reconnaissance source for AethonX with complete scaffolding, configuration, and tests.

## Skill Purpose

This skill automates the creation of new sources for AethonX, ensuring:
- Architectural consistency (Registry + Factory pattern)
- Best practices enforcement (BaseCLISource, registry helpers, proper cleanup)
- Complete configuration integration (ENV + CLI flags)
- Test scaffolding with fixtures
- Documentation generation

## Workflow

### Phase 1: Information Gathering (Interactive)

Ask the user for the following information:

1. **Source Name** (required)
   - Format: lowercase, alphanumeric with underscores
   - Examples: nuclei, amass, nmap, custom_scanner
   - Validate: no conflicts with existing sources

2. **Source Type** (required)
   - Options: CLI, API, Builtin
   - CLI: External binary subprocess (subfinder, httpx, nuclei)
   - API: HTTP API calls (crtsh, shodan)
   - Builtin: Native Go implementation (rdap)

3. **Source Mode** (required)
   - Options: Passive, Active, Both
   - Passive: No direct target contact (OSINT, APIs, archives)
   - Active: Direct target interaction (HTTP probes, port scans)
   - Both: Hybrid mode (adapts based on --active flag)

4. **Stage Hint** (required)
   - 0: Auto-detect based on dependencies
   - 1: Discovery stage (no inputs, produces domains/IPs)
   - 2: Probing stage (enriches discoveries with metadata)
   - 3: Crawl stage (deep analysis of active endpoints)

5. **Input Artifacts** (optional, comma-separated)
   - Artifact types this source consumes
   - Examples: Subdomain, IP, URL, Domain
   - Leave empty for Stage 0 sources (no dependencies)

6. **Output Artifacts** (required, comma-separated)
   - Artifact types this source produces
   - Examples: Subdomain, Vulnerability, Port, Technology

7. **Binary Name** (CLI only)
   - Command name or path
   - Default: same as source name
   - Will validate if binary exists in PATH

8. **Default Timeout** (seconds)
   - Default: 30s for API, 120s for CLI
   - Suggestion based on source type

9. **Default Priority** (0-100)
   - Higher = executes earlier
   - Stage 0: 10, Stage 1: 15, Stage 2: 20, Stage 3: 25
   - Suggest based on stage

10. **Custom Configuration** (key-value pairs)
    - API: api_key, base_url, rate_limit
    - CLI: threads, max_files, profile, custom_flags
    - Loop until user says done

11. **Requires Authentication** (yes/no)
    - For API keys or credentials

### Phase 2: Validation

Validate the collected information:

1. **Name Validation**
   - Check if source already exists in `internal/sources/`
   - Validate naming convention (lowercase, alphanumeric + underscores)
   - Check for conflicts in registry

2. **Semantic Validation**
   - Stage 3 with no InputArtifacts → warn and suggest
   - InputArtifacts not matching common patterns → warn
   - OutputArtifacts include unknown types → warn
   - Timeout too low/high for source type → suggest adjustment

3. **Dependency Validation**
   - CLI: Check if binary exists in PATH
   - API: Validate URL format if base_url provided
   - Builtin: No external dependencies

4. **Conflict Detection**
   - Check if source name conflicts with existing imports
   - Validate that Custom config keys don't clash

### Phase 3: Code Generation

Generate the following files using templates:

1. **Source Implementation**: `internal/sources/{name}/{name}.go`
   - Select template based on type (CLI/API/Builtin)
   - Populate with user data
   - Include interfaces implementation
   - Add proper error handling

2. **Registry File**: `internal/sources/{name}/registry.go`
   - Auto-registration in init()
   - Factory function with registry helpers
   - SourceMetadata with all fields

3. **Parser** (CLI/API only): `internal/sources/{name}/parser.go`
   - OutputHandler implementation for CLI
   - JSON/XML parsing logic for API
   - Artifact creation methods

4. **Tests**: `internal/sources/{name}/{name}_test.go`
   - Registry validation test
   - Config extraction test
   - Run() test with mocks
   - Race condition test

5. **Test Fixtures**: `internal/sources/{name}/fixtures_test.go`
   - Sample output data
   - Mock responses
   - Test helpers

6. **README**: `internal/sources/{name}/README.md`
   - Source description
   - Installation instructions
   - Configuration options
   - Usage examples

### Phase 4: Configuration Integration

Update configuration files:

1. **DefaultConfig** (`internal/platform/config/config.go`)
   - Add source to Sources map with defaults
   - Include Custom config fields

2. **loadFromEnv** (`internal/platform/config/config.go`)
   - Add ENV variable parsing
   - Format: AETHONX_SOURCES_{NAME}_{KEY}
   - Handle custom config fields

3. **loadFromFlags** (`internal/platform/config/config.go`)
   - Add pflag definitions
   - Format: --src.{name}.{key}
   - Handle type conversions

### Phase 5: Main Integration

Update main.go:

1. Add blank import: `_ "aethonx/internal/sources/{name}"`
2. Maintain alphabetical order
3. Add comment if special notes needed

### Phase 6: Formatting and Testing

1. Run `goimports -w` on all generated files
2. Run `go test -race ./internal/sources/{name}/...`
3. Validate no compilation errors
4. Check registry can build the source

### Phase 7: Summary and Next Steps

Display:
- ✅ Files created (list with paths)
- ✅ Configuration updated
- ✅ Tests passing
- 📋 Next steps for the user:
  1. Install binary (if CLI)
  2. Implement custom logic in Run()
  3. Implement parser logic
  4. Add test fixtures
  5. Run integration test
  6. Update documentation

## Templates Location

- `templates/cli_source.go.tmpl` - CLI source template
- `templates/api_source.go.tmpl` - API source template
- `templates/builtin_source.go.tmpl` - Builtin source template
- `templates/registry.go.tmpl` - Registry auto-registration
- `templates/parser_cli.go.tmpl` - CLI parser (OutputHandler)
- `templates/parser_api.go.tmpl` - API parser
- `templates/test.go.tmpl` - Test template
- `templates/fixtures.go.tmpl` - Fixtures template
- `templates/readme.md.tmpl` - README template

## Helper Scripts

- `helpers/validator.go` - Validation functions
- `helpers/template_engine.go` - Template rendering
- `helpers/file_updater.go` - File modification helpers

## Generators

- `generators/config_generator.go` - Config injection
- `generators/main_updater.go` - main.go updater
- `generators/doc_generator.go` - Documentation generator

## Important Notes

1. **Always use registry helpers** for config extraction (GetStringConfig, GetIntConfig, etc.)
2. **CLI sources must extend BaseCLISource** for subprocess management
3. **All sources MUST implement Close()** to prevent goroutine leaks
4. **Use fail-soft error handling** - partial results are OK
5. **Follow existing patterns** from crtsh (API), subfinder (CLI), rdap (builtin)
6. **Test with -race flag** to catch concurrency issues
7. **Maintain Clean Architecture** - respect dependency rules

## Usage Example

When invoked, the skill will guide the user through all steps interactively:

```
User: "Create a new source for nuclei"

Skill:
  1. Asks questions interactively
  2. Validates all inputs
  3. Generates all files
  4. Updates configuration
  5. Runs tests
  6. Provides next steps
```

## Error Handling

- If validation fails, explain the issue and ask for correction
- If file already exists, ask if user wants to overwrite
- If tests fail, show the error and ask how to proceed
- Always provide actionable feedback

## Success Criteria

- All files generated without errors
- Code compiles successfully
- Tests pass with -race flag
- Registry can build the source
- User receives clear next steps
