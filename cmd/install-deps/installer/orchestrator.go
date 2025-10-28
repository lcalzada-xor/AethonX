package installer

import (
	"context"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Orchestrator coordinates the installation of all dependencies.
type Orchestrator struct {
	config           Config
	systemInfo       SystemInfo
	installers       []Installer
	progressCallback ProgressCallback
}

// NewOrchestrator creates a new installation orchestrator.
func NewOrchestrator(configPath string, installDir string) (*Orchestrator, error) {
	// Load configuration
	config, err := loadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Override install directory if provided
	if installDir != "" {
		config.InstallDirectory = installDir
	}

	return &Orchestrator{
		config: config,
	}, nil
}

// SetProgressCallback sets the progress callback for real-time updates.
func (o *Orchestrator) SetProgressCallback(callback ProgressCallback) {
	o.progressCallback = callback
	// Set callback for all installers that support it
	for _, inst := range o.installers {
		if reporter, ok := inst.(ProgressReporter); ok {
			reporter.SetProgressCallback(callback)
		}
	}
}

// Initialize detects system and prepares installers.
func (o *Orchestrator) Initialize(ctx context.Context) error {
	// Detect system information
	sysInfo, err := DetectSystem(ctx, o.config.InstallDirectory)
	if err != nil {
		return fmt.Errorf("failed to detect system: %w", err)
	}
	o.systemInfo = sysInfo

	// Build installers list
	o.installers = []Installer{}

	// Add Go installer if modules required
	if o.config.Go.ModulesRequired {
		o.installers = append(o.installers, NewGoInstaller(o.config.Go.MinVersion))
	}

	// Add external tool installers
	for _, tool := range o.config.ExternalTools {
		if tool.Required {
			inst := NewExternalToolInstaller(tool)
			// Set progress callback if available
			if o.progressCallback != nil {
				inst.SetProgressCallback(o.progressCallback)
			}
			o.installers = append(o.installers, inst)
		}
	}

	return nil
}

// Check verifies the status of all dependencies.
func (o *Orchestrator) Check(ctx context.Context) ([]InstallationResult, error) {
	results := make([]InstallationResult, 0, len(o.installers))

	for _, inst := range o.installers {
		startTime := time.Now()

		installed, version, err := inst.Check(ctx, o.systemInfo)

		result := InstallationResult{
			Dependency: Dependency{
				Name: inst.Name(),
			},
			Duration: time.Since(startTime),
			Version:  version,
		}

		if err != nil {
			result.Status = StatusFailed
			result.Error = err
			result.Message = fmt.Sprintf("Check failed: %v", err)
		} else if installed {
			result.Status = StatusAlreadyInstalled
			result.Message = fmt.Sprintf("Already installed (version: %s)", version)
		} else {
			result.Status = StatusPending
			result.Message = "Not installed"
		}

		results = append(results, result)
	}

	return results, nil
}

// Install executes the installation of all dependencies.
func (o *Orchestrator) Install(ctx context.Context, force bool) ([]InstallationResult, error) {
	results := make([]InstallationResult, 0, len(o.installers))

	for _, inst := range o.installers {
		startTime := time.Now()

		result := InstallationResult{
			Dependency: Dependency{
				Name: inst.Name(),
			},
		}

		// Check if already installed
		installed, currentVersion, _ := inst.Check(ctx, o.systemInfo)

		// If installed and not forcing, check if update is needed
		if installed && !force {
			// Check if installer supports version updates (external tools)
			if extInst, ok := inst.(*ExternalToolInstaller); ok {
				needsUpdate, latestVersion, err := extInst.NeedsUpdate(ctx, currentVersion)
				if err != nil {
					// If we can't check for updates, assume current version is fine
					result.Status = StatusAlreadyInstalled
					result.Version = currentVersion
					result.Message = fmt.Sprintf("Already installed (version: %s, update check failed)", currentVersion)
					result.Duration = time.Since(startTime)
					results = append(results, result)
					continue
				}

				if !needsUpdate {
					result.Status = StatusAlreadyInstalled
					result.Version = currentVersion
					result.Message = fmt.Sprintf("Already installed (latest version: %s)", currentVersion)
					result.Duration = time.Since(startTime)
					results = append(results, result)
					continue
				}

				// Update needed - install newer version
				result.Message = fmt.Sprintf("Updating from %s to %s", currentVersion, latestVersion)
			} else {
				// For Go installer, just report already installed
				result.Status = StatusAlreadyInstalled
				result.Version = currentVersion
				result.Message = fmt.Sprintf("Already installed (version: %s)", currentVersion)
				result.Duration = time.Since(startTime)
				results = append(results, result)
				continue
			}
		}

		// Install or update
		if err := inst.Install(ctx, o.systemInfo); err != nil {
			result.Status = StatusFailed
			result.Error = err
			result.Phase = PhaseFailed
			result.ErrorContext = AnalyzeError(inst.Name(), "install", err, GetDocumentationURL(inst.Name()))
			result.Message = fmt.Sprintf("Installation failed: %v", err)
			result.Duration = time.Since(startTime)
			results = append(results, result)
			continue
		}

		// Validate
		if err := inst.Validate(ctx); err != nil {
			result.Status = StatusFailed
			result.Error = err
			result.Phase = PhaseFailed
			result.ErrorContext = AnalyzeError(inst.Name(), "validate", err, GetDocumentationURL(inst.Name()))
			result.Message = fmt.Sprintf("Validation failed: %v", err)
			result.Duration = time.Since(startTime)
			results = append(results, result)
			continue
		}

		// Success
		_, newVersion, _ := inst.Check(ctx, o.systemInfo)
		result.Status = StatusSuccess
		result.Version = newVersion
		result.Phase = PhaseCompleted

		// Determine install path
		if extInst, ok := inst.(*ExternalToolInstaller); ok {
			binaryName := extInst.tool.Install.Github.BinaryName
			if o.systemInfo.OS == "windows" {
				binaryName += ".exe"
			}
			result.InstallPath = fmt.Sprintf("%s/%s", o.systemInfo.InstallDir, binaryName)
		}

		if installed && !force {
			result.Message = fmt.Sprintf("Successfully updated (version: %s)", newVersion)
		} else {
			result.Message = fmt.Sprintf("Successfully installed (version: %s)", newVersion)
		}

		result.Duration = time.Since(startTime)
		results = append(results, result)
	}

	return results, nil
}

// CheckPath verifies if the install directory is in PATH.
func (o *Orchestrator) CheckPath() (bool, error) {
	if !o.config.AddToPath {
		return true, nil
	}

	return IsInPath(o.systemInfo.InstallDir, o.systemInfo.PathEntries), nil
}

// GetPathWarning returns a warning message if install dir is not in PATH.
func (o *Orchestrator) GetPathWarning() string {
	shellProfile := "~/.bashrc"
	if os.Getenv("SHELL") == "/bin/zsh" || os.Getenv("SHELL") == "/usr/bin/zsh" {
		shellProfile = "~/.zshrc"
	}

	return fmt.Sprintf(`
WARNING: %s is not in your PATH.

Add it by running:
  export PATH="%s:$PATH"

Or add it permanently to your shell profile (%s):
  echo 'export PATH="%s:$PATH"' >> %s

Note: ~/go/bin is the standard Go binary location. Most Go developers already have this in PATH.
`, o.systemInfo.InstallDir, o.systemInfo.InstallDir, shellProfile, o.systemInfo.InstallDir, shellProfile)
}

// loadConfig loads the deps.yaml configuration file.
func loadConfig(path string) (Config, error) {
	var config Config

	data, err := os.ReadFile(path)
	if err != nil {
		return config, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}
