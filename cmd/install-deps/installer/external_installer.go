package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aethonx/cmd/install-deps/providers"
)

// ExternalToolInstaller handles installation of external binary tools from GitHub.
type ExternalToolInstaller struct {
	tool     ExternalTool
	provider *providers.GitHubProvider
}

// NewExternalToolInstaller creates a new external tool installer.
func NewExternalToolInstaller(tool ExternalTool) *ExternalToolInstaller {
	return &ExternalToolInstaller{
		tool:     tool,
		provider: providers.NewGitHubProvider(),
	}
}

// Name returns the tool name.
func (e *ExternalToolInstaller) Name() string {
	return e.tool.Name
}

// Check verifies if the tool is already installed and its version.
func (e *ExternalToolInstaller) Check(ctx context.Context, sys SystemInfo) (bool, string, error) {
	// Check if command is available
	if !IsCommandAvailable(ctx, e.tool.HealthCheck.Command) {
		return false, "", nil
	}

	// Try to get version
	versionOutput, err := GetCommandVersion(ctx, e.tool.HealthCheck.Command, e.tool.HealthCheck.Args)
	if err != nil {
		return false, "", err
	}

	// Extract clean version
	version := ExtractVersion(versionOutput)

	return true, version, nil
}

// NeedsUpdate checks if the installed version is older than the latest available.
func (e *ExternalToolInstaller) NeedsUpdate(ctx context.Context, currentVersion string) (bool, string, error) {
	// Fetch latest release
	release, err := e.provider.GetLatestRelease(ctx, e.tool.Install.Github.Repo)
	if err != nil {
		return false, "", fmt.Errorf("failed to check latest version: %w", err)
	}

	latestVersion := release.GetVersion()

	// Compare versions
	if CompareVersions(currentVersion, latestVersion) < 0 {
		return true, latestVersion, nil
	}

	return false, latestVersion, nil
}

// Install downloads and installs the external tool.
func (e *ExternalToolInstaller) Install(ctx context.Context, sys SystemInfo) error {
	// Get platform key
	platformKey := fmt.Sprintf("%s_%s", sys.OS, sys.Arch)

	// Get asset pattern for this platform
	assetPattern, ok := e.tool.Install.Github.AssetPatterns[platformKey]
	if !ok {
		return fmt.Errorf("no asset pattern for platform %s", platformKey)
	}

	// Fetch latest release
	release, err := e.provider.GetLatestRelease(ctx, e.tool.Install.Github.Repo)
	if err != nil {
		return fmt.Errorf("failed to get latest release: %w", err)
	}

	// Find matching asset
	assetName, assetURL, err := e.provider.FindMatchingAsset(release, assetPattern)
	if err != nil {
		return fmt.Errorf("failed to find matching asset: %w", err)
	}

	// Create temp directory for download
	tempDir, err := os.MkdirTemp("", "aethonx-install-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download asset
	downloadPath := filepath.Join(tempDir, assetName)
	if err := e.provider.DownloadAsset(ctx, assetURL, downloadPath); err != nil {
		return fmt.Errorf("failed to download asset: %w", err)
	}

	// Extract archive
	extractDir := filepath.Join(tempDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return fmt.Errorf("failed to create extract directory: %w", err)
	}

	if err := e.provider.ExtractZip(downloadPath, extractDir); err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	// Find binary in extracted files
	binaryName := e.tool.Install.Github.BinaryName
	if sys.OS == "windows" {
		binaryName += ".exe"
	}

	binaryPath := filepath.Join(extractDir, binaryName)
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return fmt.Errorf("binary %s not found in archive", binaryName)
	}

	// Ensure install directory exists
	if err := EnsureInstallDir(sys.InstallDir); err != nil {
		return err
	}

	// Copy binary to install directory
	destPath := filepath.Join(sys.InstallDir, binaryName)
	if err := copyFile(binaryPath, destPath); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	// Make binary executable (Unix-like systems)
	if sys.OS != "windows" {
		if err := os.Chmod(destPath, 0755); err != nil {
			return fmt.Errorf("failed to make binary executable: %w", err)
		}
	}

	return nil
}

// Validate runs health check on the installed tool.
func (e *ExternalToolInstaller) Validate(ctx context.Context) error {
	version, err := GetCommandVersion(ctx, e.tool.HealthCheck.Command, e.tool.HealthCheck.Args)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	// Check if version output contains expected string
	if e.tool.HealthCheck.ExpectedContains != "" {
		if !containsIgnoreCase(version, e.tool.HealthCheck.ExpectedContains) {
			return fmt.Errorf("version output does not contain expected string: %s", e.tool.HealthCheck.ExpectedContains)
		}
	}

	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := dstFile.ReadFrom(srcFile); err != nil {
		return err
	}

	return nil
}

// containsIgnoreCase checks if s contains substr (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}
