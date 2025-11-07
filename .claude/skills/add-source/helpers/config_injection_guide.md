# Configuration Injection Guide

This guide explains how to inject new source configuration into AethonX config files.

## Files to Modify

1. `internal/platform/config/config.go`
   - `DefaultConfig()` function - Add source to Sources map
   - `loadFromEnv()` function - Add ENV variable parsing
   - `loadFromFlags()` function - Add CLI flag definitions

## Step 1: DefaultConfig() Injection

### Location
Find the `DefaultConfig()` function and the `Source: SourceConfig { Sources: map[string]ports.SourceConfig {` section.

### Template
```go
"{{.Name}}": {
    Enabled:   {{.DefaultEnabled}},
    Timeout:   {{.DefaultTimeout}} * time.Second,
    Retries:   2,
    RateLimit: {{.RateLimit}},
    Priority:  {{.Priority}},
    Custom: map[string]interface{}{
{{- range .CustomConfigFields}}
        "{{.Key}}": {{.DefaultValueGo}},
{{- end}}
    },
},
```

### Insertion Point
Add alphabetically among existing sources (after "rdap" but before "subfinder" for example).

### Example
```go
"nuclei": {
    Enabled:   true,
    Timeout:   120 * time.Second,
    Retries:   2,
    RateLimit: 0,
    Priority:  25,
    Custom: map[string]interface{}{
        "exec_path":      "nuclei",
        "templates_path": "./nuclei-templates",
        "severity":       []string{"critical", "high", "medium"},
        "rate_limit":     150,
    },
},
```

## Step 2: loadFromEnv() Injection

### Location
Find the `loadFromEnv()` function, after the comment `// === SOURCE-SPECIFIC CONFIG ===`

### Template for Source-Specific ENV Loading
```go
// {{.Name}}
if v := getenv("AETHONX_SOURCES_{{.EnvPrefix}}_ENABLED", ""); v != "" {
    if sourceCfg, ok := cfg.Source.Sources["{{.Name}}"]; ok {
        sourceCfg.Enabled = parseBool(v)
        cfg.Source.Sources["{{.Name}}"] = sourceCfg
    }
}

if name == "{{.Name}}" {
{{- range .CustomConfigFields}}
    {{.EnvLoadingCode}}
{{- end}}
}
```

### ENV Variable Format
- Pattern: `AETHONX_SOURCES_{SOURCE_NAME_UPPER}_{KEY_UPPER}`
- Examples:
  - `AETHONX_SOURCES_NUCLEI_ENABLED`
  - `AETHONX_SOURCES_NUCLEI_TEMPLATES_PATH`
  - `AETHONX_SOURCES_NUCLEI_RATE_LIMIT`

### Type-Specific Loading

#### String Fields
```go
if v := getenv(prefix+"TEMPLATES_PATH", ""); v != "" {
    sourceCfg.Custom["templates_path"] = v
}
```

#### Integer Fields
```go
if v := getenv(prefix+"RATE_LIMIT", ""); v != "" {
    if i, err := strconv.Atoi(v); err == nil {
        sourceCfg.Custom["rate_limit"] = i
    }
}
```

#### Float Fields
```go
if v := getenv(prefix+"THRESHOLD", ""); v != "" {
    if f, err := strconv.ParseFloat(v, 64); err == nil {
        sourceCfg.Custom["threshold"] = f
    }
}
```

#### Boolean Fields
```go
if v := getenv(prefix+"ENABLE_FEATURE", ""); v != "" {
    sourceCfg.Custom["enable_feature"] = parseBool(v)
}
```

#### Slice Fields
```go
if v := getenv(prefix+"SEVERITY", ""); v != "" {
    sourceCfg.Custom["severity"] = strings.Split(v, ",")
}
```

### Example for nuclei
```go
// nuclei
if v := getenv("AETHONX_SOURCES_NUCLEI_ENABLED", ""); v != "" {
    if sourceCfg, ok := cfg.Source.Sources["nuclei"]; ok {
        sourceCfg.Enabled = parseBool(v)
        cfg.Source.Sources["nuclei"] = sourceCfg
    }
}

if name == "nuclei" {
    if v := getenv(prefix+"EXEC_PATH", ""); v != "" {
        sourceCfg.Custom["exec_path"] = v
    }
    if v := getenv(prefix+"TEMPLATES_PATH", ""); v != "" {
        sourceCfg.Custom["templates_path"] = v
    }
    if v := getenv(prefix+"SEVERITY", ""); v != "" {
        sourceCfg.Custom["severity"] = strings.Split(v, ",")
    }
    if v := getenv(prefix+"RATE_LIMIT", ""); v != "" {
        if i, err := strconv.Atoi(v); err == nil {
            sourceCfg.Custom["rate_limit"] = i
        }
    }
}
```

## Step 3: loadFromFlags() Injection

### Location
Find the `loadFromFlags()` function, in the section after source enable/priority flags.

### Template for Flag Variables
```go
// {{.Name}}-specific flags
var {{.Name}}{{.FieldName1}} {{.Type1}}
var {{.Name}}{{.FieldName2}} {{.Type2}}
// ... more fields

if {{.Name}}Cfg, ok := cfg.Source.Sources["{{.Name}}"]; ok {
    // Extract current values with type assertions
    if v, ok := {{.Name}}Cfg.Custom["{{.key1}}"].({{.Type1}}); ok {
        {{.Name}}{{.FieldName1}} = v
    } else {
        {{.Name}}{{.FieldName1}} = {{.Default1}}
    }
    // ... more extractions

    // Register flags
    pflag.{{.FlagType1}}Var(&{{.Name}}{{.FieldName1}}, "src.{{.Name}}.{{.key1}}", {{.Name}}{{.FieldName1}},
        "{{.Description1}}")
    // ... more flags
}
```

### Flag Types by Data Type
- `string` → `pflag.StringVar`
- `int` → `pflag.IntVar`
- `bool` → `pflag.BoolVar`
- `float64` → `pflag.Float64Var`
- `[]string` → parse as comma-separated, use `StringVar` then split
- `time.Duration` → `pflag.DurationVar`

### Example for nuclei
```go
// Nuclei flags
var nucleiExecPath string
var nucleiTemplatesPath string
var nucleiSeverityStr string
var nucleiRateLimit int

if nucleiCfg, ok := cfg.Source.Sources["nuclei"]; ok {
    if v, ok := nucleiCfg.Custom["exec_path"].(string); ok {
        nucleiExecPath = v
    } else {
        nucleiExecPath = "nuclei"
    }
    if v, ok := nucleiCfg.Custom["templates_path"].(string); ok {
        nucleiTemplatesPath = v
    } else {
        nucleiTemplatesPath = "./nuclei-templates"
    }
    if v, ok := nucleiCfg.Custom["severity"].([]string); ok {
        nucleiSeverityStr = strings.Join(v, ",")
    } else {
        nucleiSeverityStr = "critical,high,medium"
    }
    if v, ok := nucleiCfg.Custom["rate_limit"].(int); ok {
        nucleiRateLimit = v
    } else {
        nucleiRateLimit = 150
    }

    pflag.StringVar(&nucleiExecPath, "src.nuclei.exec_path", nucleiExecPath,
        "Path to nuclei binary")
    pflag.StringVar(&nucleiTemplatesPath, "src.nuclei.templates_path", nucleiTemplatesPath,
        "Path to nuclei templates directory")
    pflag.StringVar(&nucleiSeverityStr, "src.nuclei.severity", nucleiSeverityStr,
        "Comma-separated severity levels (critical,high,medium,low,info)")
    pflag.IntVar(&nucleiRateLimit, "src.nuclei.rate_limit", nucleiRateLimit,
        "Rate limit in requests per second")
}
```

### Step 4: Apply Flag Values Back to Config

Add this AFTER `pflag.Parse()`:

```go
// Apply {{.Name}} flags back to config
if {{.Name}}Cfg, ok := cfg.Source.Sources["{{.Name}}"]; ok {
    {{.Name}}Cfg.Custom["{{.key1}}"] = {{.Name}}{{.FieldName1}}
    {{.Name}}Cfg.Custom["{{.key2}}"] = {{.Name}}{{.FieldName2}}
    // Handle slice types (comma-separated)
    {{.Name}}Cfg.Custom["{{.sliceKey}}"] = strings.Split({{.Name}}{{.SliceField}}, ",")
    cfg.Source.Sources["{{.Name}}"] = {{.Name}}Cfg
}
```

### Example for nuclei (after pflag.Parse)
```go
// Apply nuclei flags back to config
if nucleiCfg, ok := cfg.Source.Sources["nuclei"]; ok {
    nucleiCfg.Custom["exec_path"] = nucleiExecPath
    nucleiCfg.Custom["templates_path"] = nucleiTemplatesPath
    nucleiCfg.Custom["severity"] = strings.Split(nucleiSeverityStr, ",")
    nucleiCfg.Custom["rate_limit"] = nucleiRateLimit
    cfg.Source.Sources["nuclei"] = nucleiCfg
}
```

## Complete Example: Adding "nuclei" Source

### 1. DefaultConfig()
```go
"nuclei": {
    Enabled:   true,
    Timeout:   120 * time.Second,
    Retries:   2,
    RateLimit: 0,
    Priority:  25,
    Custom: map[string]interface{}{
        "exec_path":      "nuclei",
        "templates_path": "./nuclei-templates",
        "severity":       []string{"critical", "high", "medium"},
        "rate_limit":     150,
    },
},
```

### 2. loadFromEnv() - Add to source-specific section
```go
if name == "nuclei" {
    if v := getenv(prefix+"EXEC_PATH", ""); v != "" {
        sourceCfg.Custom["exec_path"] = v
    }
    if v := getenv(prefix+"TEMPLATES_PATH", ""); v != "" {
        sourceCfg.Custom["templates_path"] = v
    }
    if v := getenv(prefix+"SEVERITY", ""); v != "" {
        sourceCfg.Custom["severity"] = strings.Split(v, ",")
    }
    if v := getenv(prefix+"RATE_LIMIT", ""); v != "" {
        if i, err := strconv.Atoi(v); err == nil {
            sourceCfg.Custom["rate_limit"] = i
        }
    }
}
```

### 3. loadFromFlags() - Declare variables
```go
var nucleiExecPath string
var nucleiTemplatesPath string
var nucleiSeverityStr string
var nucleiRateLimit int
```

### 4. loadFromFlags() - Extract and register
```go
if nucleiCfg, ok := cfg.Source.Sources["nuclei"]; ok {
    if v, ok := nucleiCfg.Custom["exec_path"].(string); ok {
        nucleiExecPath = v
    } else {
        nucleiExecPath = "nuclei"
    }
    // ... (other extractions)

    pflag.StringVar(&nucleiExecPath, "src.nuclei.exec_path", nucleiExecPath,
        "Path to nuclei binary")
    // ... (other flags)
}
```

### 5. loadFromFlags() - Apply back (after Parse)
```go
if nucleiCfg, ok := cfg.Source.Sources["nuclei"]; ok {
    nucleiCfg.Custom["exec_path"] = nucleiExecPath
    nucleiCfg.Custom["templates_path"] = nucleiTemplatesPath
    nucleiCfg.Custom["severity"] = strings.Split(nucleiSeverityStr, ",")
    nucleiCfg.Custom["rate_limit"] = nucleiRateLimit
    cfg.Source.Sources["nuclei"] = nucleiCfg
}
```

## Verification Checklist

After injection, verify:

- [ ] DefaultConfig compiles without errors
- [ ] Source appears in default config map
- [ ] ENV variables parse correctly
- [ ] CLI flags parse correctly
- [ ] Flag priority overrides ENV
- [ ] Custom config fields accessible in factory
- [ ] No Go fmt errors
- [ ] No import conflicts

## Testing Configuration

```bash
# Test ENV loading
export AETHONX_SOURCES_NUCLEI_ENABLED=true
export AETHONX_SOURCES_NUCLEI_TEMPLATES_PATH=/custom/templates
./aethonx -t example.com --src.nuclei

# Test flag loading
./aethonx -t example.com --src.nuclei --src.nuclei.templates_path=/custom/templates

# Test flag priority (flag should override ENV)
export AETHONX_SOURCES_NUCLEI_RATE_LIMIT=50
./aethonx -t example.com --src.nuclei --src.nuclei.rate_limit=100
# Should use 100, not 50
```

## Common Pitfalls

1. **Forgetting to apply flags back to config** after `pflag.Parse()`
   - Symptoms: Flags accepted but values not used
   - Fix: Add apply-back block

2. **Type assertion failures**
   - Symptoms: Config fields always use defaults
   - Fix: Provide else clause with default value

3. **Slice handling**
   - Symptoms: Slice config stored as string
   - Fix: Use `strings.Split()` when applying back

4. **Case sensitivity**
   - Symptoms: ENV variables not recognized
   - Fix: Use exact case in `getenv()` calls

5. **Missing imports**
   - Symptoms: Compilation errors
   - Fix: Ensure `strings`, `strconv` imported if needed

## Automation Script Pseudocode

```
function injectConfig(sourceName, fields):
    1. Parse config.go to find insertion points
    2. Generate DefaultConfig entry
    3. Find loadFromEnv() and insert source block
    4. Find loadFromFlags() and insert variable declarations
    5. Insert flag registrations
    6. Insert apply-back block after Parse()
    7. Format with goimports
    8. Validate compilation
```
