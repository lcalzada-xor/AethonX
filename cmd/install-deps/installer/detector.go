package installer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// DetectSystem gathers system information for dependency installation.
func DetectSystem(ctx context.Context, installDir string) (SystemInfo, error) {
	info := SystemInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	// Expand home directory in install path
	if strings.HasPrefix(installDir, "$HOME") {
		home, err := os.UserHomeDir()
		if err != nil {
			return info, fmt.Errorf("failed to get home directory: %w", err)
		}
		installDir = strings.Replace(installDir, "$HOME", home, 1)
	}
	info.InstallDir = installDir

	// Detect Go version
	goVersion, err := detectGoVersion(ctx)
	if err == nil {
		info.GoVersion = goVersion
	}

	// Get PATH entries
	pathEnv := os.Getenv("PATH")
	info.PathEntries = strings.Split(pathEnv, string(os.PathListSeparator))

	return info, nil
}

// detectGoVersion detects the installed Go version.
func detectGoVersion(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "go", "version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go not found in PATH: %w", err)
	}

	// Parse "go version go1.24.4 linux/amd64"
	parts := strings.Fields(string(output))
	if len(parts) >= 3 {
		version := strings.TrimPrefix(parts[2], "go")
		return version, nil
	}

	return "", fmt.Errorf("unexpected go version output: %s", output)
}

// IsCommandAvailable checks if a command is available in PATH.
func IsCommandAvailable(ctx context.Context, command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// GetCommandVersion executes a command with version flag and returns output.
func GetCommandVersion(ctx context.Context, command string, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get version for %s: %w", command, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// EnsureInstallDir creates the installation directory if it doesn't exist.
func EnsureInstallDir(installDir string) error {
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("failed to create install directory %s: %w", installDir, err)
	}
	return nil
}

// IsInPath checks if a directory is in the PATH environment variable.
func IsInPath(dir string, pathEntries []string) bool {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}

	for _, entry := range pathEntries {
		absEntry, err := filepath.Abs(entry)
		if err != nil {
			continue
		}
		if absEntry == absDir {
			return true
		}
	}
	return false
}

// CompareVersions compares two semantic versions (simple implementation).
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func CompareVersions(v1, v2 string) int {
	v1 = cleanVersion(v1)
	v2 = cleanVersion(v2)

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 int
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &p1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &p2)
		}

		if p1 < p2 {
			return -1
		}
		if p1 > p2 {
			return 1
		}
	}

	return 0
}

// cleanVersion normalizes version strings for comparison.
func cleanVersion(version string) string {
	// Remove common prefixes
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "V")

	// Extract version from complex strings like "Current Version: v1.7.1"
	if strings.Contains(version, "Current Version") {
		parts := strings.Split(version, ":")
		if len(parts) >= 2 {
			version = strings.TrimSpace(parts[len(parts)-1])
			version = strings.TrimPrefix(version, "v")
		}
	}

	// Remove any trailing/leading whitespace
	version = strings.TrimSpace(version)

	return version
}

// ExtractVersion attempts to extract a version number from command output.
func ExtractVersion(output string) string {
	// Simple extraction - find first occurrence of version pattern
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "version") {
			// Extract numbers
			parts := strings.Fields(line)
			for _, part := range parts {
				clean := cleanVersion(part)
				if isValidVersion(clean) {
					return clean
				}
			}
		}
	}

	return cleanVersion(output)
}

// isValidVersion checks if a string looks like a version number.
func isValidVersion(v string) bool {
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return false
	}

	for _, part := range parts {
		var num int
		if _, err := fmt.Sscanf(part, "%d", &num); err != nil {
			return false
		}
	}

	return true
}
