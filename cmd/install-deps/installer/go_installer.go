package installer

import (
	"context"
	"fmt"
	"os/exec"
)

// GoInstaller handles Go module dependencies.
type GoInstaller struct {
	minVersion string
}

// NewGoInstaller creates a new Go modules installer.
func NewGoInstaller(minVersion string) *GoInstaller {
	return &GoInstaller{
		minVersion: minVersion,
	}
}

// Name returns the installer name.
func (g *GoInstaller) Name() string {
	return "Go Modules"
}

// Check verifies if Go is installed and modules are up to date.
func (g *GoInstaller) Check(ctx context.Context, sys SystemInfo) (bool, string, error) {
	// Check if Go is installed
	if sys.GoVersion == "" {
		return false, "", fmt.Errorf("Go is not installed")
	}

	// Verify minimum version
	if CompareVersions(sys.GoVersion, g.minVersion) < 0 {
		return false, sys.GoVersion, fmt.Errorf("Go version %s is below minimum required %s", sys.GoVersion, g.minVersion)
	}

	return true, sys.GoVersion, nil
}

// Install downloads and verifies Go modules.
func (g *GoInstaller) Install(ctx context.Context, sys SystemInfo) error {
	// Run go mod download
	downloadCmd := exec.CommandContext(ctx, "go", "mod", "download")
	if output, err := downloadCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go mod download failed: %w\n%s", err, output)
	}

	// Run go mod tidy to ensure consistency
	tidyCmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	if output, err := tidyCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w\n%s", err, output)
	}

	return nil
}

// Validate verifies that Go modules are properly installed.
func (g *GoInstaller) Validate(ctx context.Context) error {
	// Run go mod verify
	cmd := exec.CommandContext(ctx, "go", "mod", "verify")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go mod verify failed: %w\n%s", err, output)
	}

	return nil
}
