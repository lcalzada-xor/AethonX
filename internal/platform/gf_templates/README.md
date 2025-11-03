# AethonX GF Templates

Modern collection of gf (grep filters) pattern templates for GoLinkfinderEVO integration.

## Overview

These templates are automatically loaded by the `golinkfinderevo` source to identify sensitive data, security issues, and interesting endpoints in JavaScript and HTML files.

## Available Templates

| Template | Description | Artifact Type |
|----------|-------------|---------------|
| `api-keys.json` | AWS, GCP, Azure, GitHub, Slack API keys | Credential |
| `jwt.json` | JWT tokens (Bearer, signed) | Credential |
| `credentials.json` | Passwords, secrets, tokens in code | Credential |
| `sqli.json` | SQL injection-prone parameters | Parameter |
| `xss.json` | XSS-vulnerable parameters | Parameter |
| `sensitive-files.json` | Config files, backups, .env files | SensitiveFile |
| `endpoints.json` | API endpoints, REST routes | Endpoint |
| `cloud-resources.json` | S3 buckets, Azure blobs, GCP storage | StorageBucket |
| `crypto.json` | Private keys, certificates, PGP keys | Credential |
| `custom-params.json` | Debug, admin, command injection params | Parameter |

## Usage

Templates are automatically used by golinkfinderevo when enabled:

```bash
# Use all templates (default)
./aethonx -t example.com --src.golinkfinderevo.gf-patterns=all

# Use specific templates
./aethonx -t example.com --src.golinkfinderevo.gf-patterns=jwt,api-keys,credentials

# Disable GF integration
./aethonx -t example.com --src.golinkfinderevo.gf-patterns=""
```

## Template Format

Templates use JSON format compatible with tomnomnom/gf:

### Single Pattern
```json
{
  "flags": "-HnriE",
  "pattern": "your-regex-here"
}
```

### Multiple Patterns
```json
{
  "flags": "-HnriE",
  "patterns": [
    "pattern1",
    "pattern2",
    "pattern3"
  ]
}
```

### Flags Explanation
- `-H`: Show filename with matches
- `-n`: Show line numbers
- `-r`: Recursive (not used in GoLinkfinderEVO context)
- `-i`: Case-insensitive
- `-E`: Extended regex (ERE)

## Adding Custom Templates

1. Create a new `.json` file in this directory
2. Follow the format above
3. Add pattern name to `--src.golinkfinderevo.gf-patterns` flag
4. Optionally update `gf_integration.go` to map pattern to artifact type

## Pattern Design Best Practices

- **Be Specific**: Avoid overly broad patterns that generate false positives
- **Use Anchors**: Use `^` and `$` when appropriate to match exact positions
- **Escape Special Chars**: Properly escape regex metacharacters: `\.`, `\(`, `\[`, etc.
- **Test Patterns**: Validate against sample data before deployment
- **Document Intent**: Add comments in commit messages explaining pattern purpose

## Security Considerations

These patterns detect potentially sensitive information:

- **High Severity**: API keys, private keys, credentials
- **Medium Severity**: JWT tokens, session identifiers
- **Low Severity**: Suspicious parameters, debug endpoints

Always handle findings with appropriate security protocols and never commit real credentials to version control.

## Examples

### Detecting AWS Keys
```bash
# Pattern: AKIA[0-9A-Z]{16}
# Matches: AKIAIOSFODNN7EXAMPLE
```

### Detecting JWT Tokens
```bash
# Pattern: eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*
# Matches: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U
```

### Detecting Sensitive Files
```bash
# Pattern: \.env(\..*)?$
# Matches: .env, .env.local, .env.production
```

## Contributing

When adding new patterns:

1. Test against real-world samples
2. Verify false positive rate is acceptable (<10%)
3. Document regex intent
4. Update this README with new template info
5. Add corresponding test cases

## References

- [tomnomnom/gf](https://github.com/tomnomnom/gf) - Original gf tool
- [GoLinkfinderEVO](https://github.com/lcalzada-xor/GoLinkfinderEVO) - Endpoint discovery tool
- [Regex101](https://regex101.com/) - Regex testing and debugging
