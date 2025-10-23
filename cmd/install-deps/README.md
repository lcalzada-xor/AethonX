# AethonX Dependency Installer

Automated dependency installer for AethonX reconnaissance engine.

## Overview

`install-deps` is a standalone CLI tool that automatically downloads, installs, and validates all dependencies required by AethonX, including:

- **Go modules** - Project dependencies via `go mod`
- **External tools** - Binary tools like httpx from GitHub releases
- **Cross-platform support** - Linux, macOS, Windows (amd64, arm64)

## Features

- **Automatic detection** - Detects OS, architecture, and existing installations
- **Smart installation** - Skips already-installed dependencies
- **Auto-update** - Automatically detects and updates outdated tools to latest version
- **Version checking** - Queries GitHub releases to verify latest available version
- **Visual feedback** - Beautiful UI with progress tracking (using AethonX Presenter)
- **Health checks** - Validates installations after download
- **Configurable** - YAML-based configuration for easy extension
- **Force reinstall** - Option to force reinstallation of dependencies
- **Standard paths** - Installs to `~/go/bin` (Go standard location)

## Quick Start

### Check dependencies status

```bash
make check-deps
```

### Install all dependencies

```bash
make install-deps
```

### Manual usage

```bash
# Build the installer
go build -o install-deps ./cmd/install-deps

# Check dependencies
./install-deps --check

# Install dependencies
./install-deps

# Force reinstall
./install-deps --force

# Install to custom directory
./install-deps --dir /usr/local/bin

# Quiet mode (no UI)
./install-deps --quiet
```

## Command-Line Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--config` | - | Path to deps.yaml configuration | `deps.yaml` |
| `--dir` | - | Installation directory (overrides config) | `$HOME/.aethonx/bin` |
| `--check` | - | Only check dependencies, don't install | `false` |
| `--force` | - | Force reinstall even if already installed | `false` |
| `--quiet` | `-q` | Quiet mode (no UI, minimal output) | `false` |
| `--skip-go` | - | Skip Go module dependencies | `false` |
| `--skip-external` | - | Skip external tool dependencies | `false` |
| `--version` | `-v` | Show version and exit | - |

## Configuration

Dependencies are defined in `deps.yaml` at the project root:

```yaml
go:
  min_version: "1.24.0"
  modules_required: true

external_tools:
  - name: httpx
    description: "Project Discovery's HTTP probing tool"
    required: true
    type: binary
    install:
      github:
        repo: projectdiscovery/httpx
        asset_patterns:
          linux_amd64: "httpx_*_linux_amd64.zip"
          darwin_amd64: "httpx_*_macOS_amd64.zip"
          windows_amd64: "httpx_*_windows_amd64.zip"
        binary_name: "httpx"
    health_check:
      command: "httpx"
      args: ["-version"]
      expected_contains: "Current Version"
    min_version: "1.6.0"

install_directory: "$HOME/.aethonx/bin"
add_to_path: true
```

## Adding New Dependencies

To add a new external tool:

1. Add tool configuration to `deps.yaml`:

```yaml
external_tools:
  - name: mytool
    description: "My awesome tool"
    required: true
    type: binary
    install:
      github:
        repo: owner/repo
        asset_patterns:
          linux_amd64: "mytool_*_linux_amd64.zip"
        binary_name: "mytool"
    health_check:
      command: "mytool"
      args: ["--version"]
      expected_contains: "version"
```

2. Run `make check-deps` to verify configuration

3. Run `make install-deps` to install

## Architecture

```
cmd/install-deps/
├── main.go                      # CLI entry point
├── installer/
│   ├── types.go                # Core types and interfaces
│   ├── detector.go             # System detection
│   ├── orchestrator.go         # Installation orchestration
│   ├── go_installer.go         # Go modules installer
│   └── external_installer.go   # External tools installer
└── providers/
    └── github_provider.go      # GitHub releases downloader
```

### Key Components

**Orchestrator**
- Coordinates installation of all dependencies
- Handles system detection
- Manages installer lifecycle

**Installers**
- `GoInstaller` - Manages Go module dependencies
- `ExternalToolInstaller` - Downloads binaries from GitHub releases

**Providers**
- `GitHubProvider` - Fetches releases and downloads assets

**Presenter Integration**
- Reuses AethonX's Presenter pattern for visual feedback
- Supports compact and quiet modes

## Auto-Update Behavior

The installer automatically checks for updates when you run it:

1. **Check installed version** - Queries the tool's version via command
2. **Fetch latest release** - Queries GitHub API for latest release
3. **Compare versions** - Uses semantic versioning comparison
4. **Auto-update** - If outdated, downloads and installs the latest version

Example output:
```bash
# Current version (1.7.1) matches latest
✓ httpx: Already installed (latest version: 1.7.1)

# Outdated version detected - auto-updates
✓ httpx: Successfully updated (version: 1.7.1)
```

To force reinstall regardless of version:
```bash
./install-deps --force
```

## Examples

### CI/CD Integration

```bash
# In your CI pipeline
./install-deps --quiet --check || ./install-deps --quiet
```

### Docker Usage

```dockerfile
FROM golang:1.24

WORKDIR /app
COPY . .

# Install dependencies
RUN go run ./cmd/install-deps --quiet

# Build AethonX
RUN make build
```

### Custom Installation Path

```bash
# Install to /usr/local/bin (requires permissions)
sudo ./install-deps --dir /usr/local/bin

# Install to project directory
./install-deps --dir ./bin
```

## Troubleshooting

### httpx installation fails

- Check internet connectivity
- Verify GitHub API is accessible
- Try with `--force` to reinstall

### Permission denied

- Use `--dir` to specify a directory you have write access to
- Or run with appropriate permissions

### PATH warnings

Add the install directory to your PATH:

```bash
# Temporary (current session)
export PATH="$HOME/.aethonx/bin:$PATH"

# Permanent (add to ~/.bashrc or ~/.zshrc)
echo 'export PATH="$HOME/.aethonx/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

## Development

### Building

```bash
make build-installer
```

### Testing

```bash
# Test check mode
./install-deps --check

# Test installation to temp directory
./install-deps --dir /tmp/test-install

# Test with debug logging
AETHONX_LOG_LEVEL=debug ./install-deps
```

## License

Part of the AethonX project - MIT License
