# Main.go Updater Guide

Guide for adding new source imports to `cmd/aethonx/main.go`.

## Location

File: `cmd/aethonx/main.go`

Section: Import block with comment `// Import sources for auto-registration via init()`

## Current Structure

```go
import (
    "context"
    "fmt"
    "os"
    // ... other imports

    // Import sources for auto-registration via init()
    _ "aethonx/internal/sources/crtsh"
    _ "aethonx/internal/sources/dnsx"
    _ "aethonx/internal/sources/golinkfinderevo"
    _ "aethonx/internal/sources/httpx"
    _ "aethonx/internal/sources/rdap"
    _ "aethonx/internal/sources/shodan"
    _ "aethonx/internal/sources/subfinder"
    _ "aethonx/internal/sources/waybackurls"
)
```

## Rules for Adding Sources

### 1. Blank Import
- Use `_` prefix for blank import
- This triggers the source's `init()` function
- Format: `_ "aethonx/internal/sources/{source_name}"`

### 2. Alphabetical Order
- Maintain alphabetical order within the source imports block
- Makes it easy to find and avoid duplicates

### 3. Comment Preservation
- Keep the comment `// Import sources for auto-registration via init()`
- Add additional comments only if source requires special notes

## Insertion Algorithm

### Step 1: Locate Import Block
Find the line containing: `// Import sources for auto-registration via init()`

### Step 2: Find Insertion Point
- Read all source import lines
- Find alphabetical position for new source
- Insert before first source that comes alphabetically after

### Step 3: Insert New Import
Add line: `_ "aethonx/internal/sources/{source_name}"`

### Step 4: Format
Run `goimports -w cmd/aethonx/main.go`

## Examples

### Example 1: Add "nuclei" Source

**Current:**
```go
// Import sources for auto-registration via init()
_ "aethonx/internal/sources/crtsh"
_ "aethonx/internal/sources/dnsx"
_ "aethonx/internal/sources/golinkfinderevo"
_ "aethonx/internal/sources/httpx"
_ "aethonx/internal/sources/rdap"
_ "aethonx/internal/sources/shodan"
_ "aethonx/internal/sources/subfinder"
_ "aethonx/internal/sources/waybackurls"
```

**After adding "nuclei":**
```go
// Import sources for auto-registration via init()
_ "aethonx/internal/sources/crtsh"
_ "aethonx/internal/sources/dnsx"
_ "aethonx/internal/sources/golinkfinderevo"
_ "aethonx/internal/sources/httpx"
_ "aethonx/internal/sources/nuclei"        // <-- Added here
_ "aethonx/internal/sources/rdap"
_ "aethonx/internal/sources/shodan"
_ "aethonx/internal/sources/subfinder"
_ "aethonx/internal/sources/waybackurls"
```

### Example 2: Add "amass" Source

**Current:**
```go
// Import sources for auto-registration via init()
_ "aethonx/internal/sources/crtsh"
_ "aethonx/internal/sources/dnsx"
```

**After adding "amass":**
```go
// Import sources for auto-registration via init()
_ "aethonx/internal/sources/amass"         // <-- Added here (before "crtsh")
_ "aethonx/internal/sources/crtsh"
_ "aethonx/internal/sources/dnsx"
```

### Example 3: Add "zap" Source (Last alphabetically)

**Current:**
```go
// Import sources for auto-registration via init()
_ "aethonx/internal/sources/subfinder"
_ "aethonx/internal/sources/waybackurls"
```

**After adding "zap":**
```go
// Import sources for auto-registration via init()
_ "aethonx/internal/sources/subfinder"
_ "aethonx/internal/sources/waybackurls"
_ "aethonx/internal/sources/zap"           // <-- Added here (after all others)
```

## Special Cases

### Case 1: First Source Import
If no source imports exist yet:

```go
import (
    // ... other imports

    // Import sources for auto-registration via init()
    _ "aethonx/internal/sources/newsource"
)
```

### Case 2: Source with Special Requirements
Add inline comment if source has special notes:

```go
_ "aethonx/internal/sources/shodan"       // Requires API key
```

### Case 3: Experimental Sources
Consider adding block comment for experimental sources:

```go
// Experimental sources (may change or be removed)
_ "aethonx/internal/sources/experimental_source"
```

## Validation Steps

After updating main.go:

### 1. Syntax Check
```bash
go build cmd/aethonx/main.go
```

Should compile without errors.

### 2. Import Verification
```bash
go list -f '{{.Imports}}' cmd/aethonx
```

Should include new source package.

### 3. Registration Check
```bash
./aethonx --help
```

Should show new source in available sources (if help includes source list).

### 4. Functional Test
```bash
./aethonx -t example.com --src.{newsource}
```

Should execute without "source not registered" error.

## Automation Pseudocode

```
function addSourceImport(sourceName, mainFilePath):
    1. Read main.go content
    2. Parse import block
    3. Find source imports section (after comment marker)
    4. Read existing source imports into array
    5. Append new import: _ "aethonx/internal/sources/{sourceName}"
    6. Sort array alphabetically
    7. Replace source imports section with sorted array
    8. Write back to file
    9. Run goimports -w main.go
    10. Verify compilation with go build
    11. Return success/failure
```

## Python Implementation Example

```python
import re

def add_source_import(source_name, main_file_path="cmd/aethonx/main.go"):
    with open(main_file_path, 'r') as f:
        content = f.read()

    # Find source imports block
    pattern = r'(// Import sources for auto-registration via init\(\)\n)((?:\s*_ "aethonx/internal/sources/\w+"\n)+)'

    match = re.search(pattern, content)
    if not match:
        raise Exception("Could not find source imports block")

    comment = match.group(1)
    imports_block = match.group(2)

    # Parse existing imports
    import_pattern = r'_ "aethonx/internal/sources/(\w+)"'
    existing = re.findall(import_pattern, imports_block)

    # Add new source if not exists
    if source_name not in existing:
        existing.append(source_name)
        existing.sort()

    # Rebuild imports block
    new_imports = '\n'.join([f'\t_ "aethonx/internal/sources/{name}"' for name in existing])
    new_block = comment + new_imports + '\n'

    # Replace in content
    content = content.replace(match.group(0), new_block)

    # Write back
    with open(main_file_path, 'w') as f:
        f.write(content)

    # Format
    os.system(f"goimports -w {main_file_path}")

    return True
```

## Go Implementation Example

```go
package main

import (
    "fmt"
    "go/ast"
    "go/parser"
    "go/printer"
    "go/token"
    "os"
    "sort"
)

func addSourceImport(sourceName, mainFilePath string) error {
    fset := token.NewFileSet()

    // Parse main.go
    node, err := parser.ParseFile(fset, mainFilePath, nil, parser.ParseComments)
    if err != nil {
        return err
    }

    // Find import declaration
    for _, decl := range node.Decls {
        genDecl, ok := decl.(*ast.GenDecl)
        if !ok || genDecl.Tok != token.IMPORT {
            continue
        }

        // Collect source imports
        sourceImports := make([]string, 0)
        newImportPath := fmt.Sprintf("aethonx/internal/sources/%s", sourceName)

        for _, spec := range genDecl.Specs {
            importSpec := spec.(*ast.ImportSpec)
            path := importSpec.Path.Value

            // Check if it's a source import
            if strings.Contains(path, "aethonx/internal/sources/") {
                sourceImports = append(sourceImports, path)
            }
        }

        // Add new import
        newImport := fmt.Sprintf(`"aethonx/internal/sources/%s"`, sourceName)
        sourceImports = append(sourceImports, newImport)
        sort.Strings(sourceImports)

        // Update AST
        // ... (AST manipulation code)
    }

    // Write back
    f, err := os.Create(mainFilePath)
    if err != nil {
        return err
    }
    defer f.Close()

    return printer.Fprint(f, fset, node)
}
```

## Error Handling

### Error: Import Already Exists
```
Error: Source 'nuclei' is already imported in main.go
```

**Resolution**: Skip import, continue with other setup steps.

### Error: Invalid Source Name
```
Error: Source name 'my-tool' is invalid (contains hyphens)
```

**Resolution**: Validate source name before attempting import.

### Error: Cannot Parse main.go
```
Error: Failed to parse main.go: unexpected token
```

**Resolution**: Check main.go syntax, ensure it compiles before modification.

### Error: Permission Denied
```
Error: Permission denied writing to main.go
```

**Resolution**: Check file permissions, run with appropriate privileges.

## Rollback Procedure

If import addition fails:

1. Restore from backup (created before modification)
2. Or remove the added line manually
3. Run `goimports -w main.go` to clean up
4. Verify with `go build`

## Testing the Import

After successful import addition:

```bash
# Test 1: Compilation
go build -o aethonx cmd/aethonx/main.go
# Should succeed

# Test 2: Registry check (add to main.go temporarily for debugging)
fmt.Println(registry.Global().List())
# Should include new source

# Test 3: Run with new source
./aethonx -t example.com --src.{newsource}
# Should attempt to run (may fail if binary not installed, but shouldn't fail registration)
```

## Best Practices

1. **Always backup main.go** before modification
2. **Use goimports** after manual edits
3. **Validate compilation** after changes
4. **Keep alphabetical order** for easy maintenance
5. **Add comments** for non-obvious sources
6. **Test registry** before declaring success
7. **Document special requirements** in source README

## Integration with Skill

The skill should:

1. Read current imports from main.go
2. Determine insertion point alphabetically
3. Insert new import line
4. Format with goimports
5. Validate compilation
6. Report success with verification command
