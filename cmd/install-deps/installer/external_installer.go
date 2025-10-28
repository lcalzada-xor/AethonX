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
	tool             ExternalTool
	provider         *providers.GitHubProvider
	progressCallback ProgressCallback
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

// SetProgressCallback sets the progress callback function.
func (e *ExternalToolInstaller) SetProgressCallback(callback ProgressCallback) {
	e.progressCallback = callback
}

// reportProgress calls the progress callback if set.
func (e *ExternalToolInstaller) reportProgress(phase InstallationPhase, message string) {
	if e.progressCallback != nil {
		e.progressCallback(e.tool.Name, phase, message)
	}
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
	e.reportProgress(PhaseDownloading, "Fetching latest release info...")
	release, err := e.provider.GetLatestRelease(ctx, e.tool.Install.Github.Repo)
	if err != nil {
		return fmt.Errorf("failed to get latest release: %w", err)
	}

	version := release.GetVersion()
	e.reportProgress(PhaseDownloading, fmt.Sprintf("Found version %s", version))

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
	e.reportProgress(PhaseDownloading, fmt.Sprintf("Downloading %s...", assetName))
	downloadPath := filepath.Join(tempDir, assetName)
	if err := e.provider.DownloadAsset(ctx, assetURL, downloadPath); err != nil {
		return fmt.Errorf("failed to download asset: %w", err)
	}
	e.reportProgress(PhaseDownloading, "Download complete")

	var binaryPath string
	binaryName := e.tool.Install.Github.BinaryName
	if sys.OS == "windows" {
		binaryName += ".exe"
	}

	// Check if the downloaded file needs extraction
	lowerAssetName := strings.ToLower(assetName)
	needsExtraction := strings.HasSuffix(lowerAssetName, ".zip") ||
		strings.HasSuffix(lowerAssetName, ".tar.gz") ||
		strings.HasSuffix(lowerAssetName, ".tgz")

	if needsExtraction {
		// Extract archive
		e.reportProgress(PhaseExtracting, "Extracting archive...")
		extractDir := filepath.Join(tempDir, "extracted")
		if err := os.MkdirAll(extractDir, 0755); err != nil {
			return fmt.Errorf("failed to create extract directory: %w", err)
		}

		// Extract based on file extension
		if strings.HasSuffix(lowerAssetName, ".zip") {
			if err := e.provider.ExtractZip(downloadPath, extractDir); err != nil {
				return fmt.Errorf("failed to extract zip: %w", err)
			}
		} else if strings.HasSuffix(lowerAssetName, ".tar.gz") || strings.HasSuffix(lowerAssetName, ".tgz") {
			if err := e.provider.ExtractTarGz(downloadPath, extractDir); err != nil {
				return fmt.Errorf("failed to extract tar.gz: %w", err)
			}
		}

		// Find binary in extracted files (may be in subdirectory)
		binaryPath, err = findBinaryInDir(extractDir, binaryName)
		if err != nil {
			return fmt.Errorf("binary %s not found in archive: %w", binaryName, err)
		}
		e.reportProgress(PhaseExtracting, "Extraction complete")
	} else {
		// Direct binary download - use downloaded file directly
		binaryPath = downloadPath
	}

	// Ensure install directory exists
	e.reportProgress(PhaseInstalling, fmt.Sprintf("Installing to %s...", sys.InstallDir))
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

	e.reportProgress(PhaseInstalling, fmt.Sprintf("Installed to %s", destPath))

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

// findBinaryInDir searches for a binary file in a directory and its subdirectories.
func findBinaryInDir(dir, binaryName string) (string, error) {
	var foundPath string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if this is the binary we're looking for
		if info.Name() == binaryName {
			foundPath = path
			return filepath.SkipDir // Stop walking once found
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	if foundPath == "" {
		return "", fmt.Errorf("binary not found")
	}

	return foundPath, nil
}
