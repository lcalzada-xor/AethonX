# whatweb Source

Web technology fingerprinting via WhatWeb CLI tool.

## Overview

- **Type**: CLI
- **Mode**: Active
- **Stage**: 2 (Probing/Enrichment)
- **Priority**: 16
- **Requires Auth**: No

## Installation

### Binary Installation

```bash
# Debian/Ubuntu
sudo apt-get install whatweb

# From source
git clone https://github.com/urbanadventurer/WhatWeb.git
cd WhatWeb
sudo make install

# Via RubyGems
gem install whatweb
```

Verify installation:

```bash
whatweb --version
```

## Configuration

### Environment Variables

```bash
# Enable/disable the source
export AETHONX_SOURCES_WHATWEB_ENABLED=true

# Priority (higher = runs earlier)
export AETHONX_SOURCES_WHATWEB_PRIORITY=16

# Timeout in seconds
export AETHONX_SOURCES_WHATWEB_TIMEOUT=120

# Binary path (optional, defaults to PATH lookup)
export AETHONX_SOURCES_WHATWEB_EXEC_PATH=whatweb

# Aggression level (1-4)
export AETHONX_SOURCES_WHATWEB_AGGRESSION=1

# Concurrent threads
export AETHONX_SOURCES_WHATWEB_THREADS=25

# Custom user agent
export AETHONX_SOURCES_WHATWEB_USER_AGENT="AethonX/1.0"
```

### CLI Flags

```bash
# Enable the source
./aethonx -t example.com --src.whatweb

# Disable the source
./aethonx -t example.com --src.whatweb=false

# Set priority
./aethonx -t example.com --src.whatweb.priority=16

# Set aggression level (1=stealthy, 2=polite, 3=aggressive, 4=heavy)
./aethonx -t example.com --src.whatweb.aggression=3

# Set threads
./aethonx -t example.com --src.whatweb.threads=50

# Set custom user agent
./aethonx -t example.com --src.whatweb.user_agent="Custom UA"
```

### .env File

```bash
# Add to .env file
AETHONX_SOURCES_WHATWEB_ENABLED=true
AETHONX_SOURCES_WHATWEB_AGGRESSION=1
AETHONX_SOURCES_WHATWEB_THREADS=25
AETHONX_SOURCES_WHATWEB_USER_AGENT="AethonX/1.0"
```

## Usage

### Basic Scan

```bash
./aethonx -t example.com --src.whatweb --active
```

Note: whatweb is an **active** source - requires `--active` flag.

### With Custom Configuration

```bash
./aethonx -t example.com --active \
  --src.whatweb \
  --src.whatweb.aggression=3 \
  --src.whatweb.threads=50 \
  -o output_dir
```

### Active Mode

```bash
# Enable active reconnaissance
./aethonx -t example.com --active --src.whatweb
```

This source operates in **active mode** and requires the `--active` flag.

### Stage-Based Execution

This source implements `InputConsumer` and runs in Stage 2.

It consumes the following artifact types from previous stages:
- Subdomain
- URL

The orchestrator will automatically provide these artifacts as input.

## Output Artifacts

This source produces the following artifact types:

- **Technology**: Detected web technologies, frameworks, CMS, libraries
- **Service**: Web servers, application servers, and services

## Examples

### Example 1: Basic Discovery

```bash
./aethonx -t hackerone.com --active --src.whatweb -o results/
```

### Example 2: Integration with Other Sources

```bash
# whatweb will run in stage 2
./aethonx -t example.com --active \
  --src.crtsh \
  --src.subfinder \
  --src.httpx \
  --src.whatweb \
  -o full_scan/
```

### Example 3: Aggressive Scan

```bash
./aethonx -t example.com --active \
  --src.whatweb \
  --src.whatweb.aggression=4 \
  --src.whatweb.threads=100 \
  -o aggressive_scan/
```

## Aggression Levels

WhatWeb supports 4 aggression levels:

- **Level 1 (Stealthy)**: Single HTTP GET request per target
- **Level 2 (Polite)**: Limited additional requests
- **Level 3 (Aggressive)**: More requests for better detection
- **Level 4 (Heavy)**: Maximum detection, many requests

Default: Level 1 (stealthy)

## Output Format

Results are saved in JSON format:

```json
{
  "target": {
    "root": "example.com",
    "type": "domain"
  },
  "artifacts": [
    {
      "type": "Technology",
      "value": "Apache",
      "sources": ["whatweb"],
      "confidence": 0.9,
      "metadata": {
        "name": "Apache",
        "version": "2.4.41",
        "detected_at": "https://example.com",
        "category": "Web Server"
      }
    },
    {
      "type": "Service",
      "value": "Apache",
      "sources": ["whatweb"],
      "confidence": 0.9,
      "metadata": {
        "name": "Apache",
        "version": "2.4.41",
        "protocol": "HTTP",
        "port": 443
      }
    }
  ]
}
```

## Performance

- **Default Timeout**: 120s
- **Rate Limit**: Managed by threads parameter
- **Concurrency**: 25 threads (configurable)
- **Memory**: Streaming enabled for large result sets (threshold: 1000 artifacts)

## Troubleshooting

### Binary Not Found

```
Error: whatweb not found in PATH
```

**Solution**: Install whatweb and ensure it's in your PATH:

```bash
which whatweb
# or specify path explicitly
export AETHONX_SOURCES_WHATWEB_EXEC_PATH=/custom/path/whatweb
```

### Permission Denied

```
Error: permission denied: whatweb
```

**Solution**: Make the binary executable:

```bash
chmod +x $(which whatweb)
```

### Timeout Errors

```
Warning: whatweb timed out after 120s
```

**Solution**: Increase timeout:

```bash
./aethonx -t example.com --active --src.whatweb.timeout=300
```

### No Results Found

```
Warning: whatweb completed but found 0 results
```

**Possible causes**:
- Target has no web services
- Target blocks scanning
- Network connectivity issues
- Aggression level too low (try level 2 or 3)

## Development

### Running Tests

```bash
# Unit tests
go test -v ./internal/sources/whatweb/

# With race detection
go test -race ./internal/sources/whatweb/

# Coverage
go test -cover ./internal/sources/whatweb/
```

### Debugging

Enable debug logging:

```bash
AETHONX_LOG_LEVEL=debug ./aethonx -t example.com --active --src.whatweb
```

## Architecture

### Interfaces Implemented

- `ports.Source` (required)
- `ports.AdvancedSource` (Initialize, Validate, HealthCheck)
- `ports.StreamingSource` (Stream)
- `ports.InputConsumer` (RunWithInput)

### Files

- `whatweb.go` - Main implementation
- `parser.go` - Output parsing logic
- `registry.go` - Auto-registration
- `whatweb_test.go` - Unit tests
- `README.md` - This file

## Contributing

To modify this source:

1. Update implementation in `whatweb.go`
2. Update parser in `parser.go`
3. Add tests in `whatweb_test.go`
4. Run tests: `make test`
5. Update this README if needed

## References

- Binary: https://github.com/urbanadventurer/WhatWeb
- Documentation: https://github.com/urbanadventurer/WhatWeb/wiki
- AethonX Documentation: https://github.com/yourusername/aethonx
