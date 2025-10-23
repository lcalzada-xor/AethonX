// Package installer provides dependency installation and validation capabilities.
package installer

import (
	"context"
	"time"
)

// DependencyType represents the type of dependency.
type DependencyType string

const (
	DependencyTypeGo     DependencyType = "go"
	DependencyTypeBinary DependencyType = "binary"
	DependencyTypeSystem DependencyType = "system"
)

// Status represents the installation status of a dependency.
type Status string

const (
	StatusSuccess           Status = "success"
	StatusFailed            Status = "failed"
	StatusSkipped           Status = "skipped"
	StatusAlreadyInstalled  Status = "already_installed"
	StatusPending           Status = "pending"
)

// SystemInfo contains system detection information.
type SystemInfo struct {
	OS           string // linux, darwin, windows
	Arch         string // amd64, arm64
	GoVersion    string
	InstallDir   string
	PathEntries  []string
}

// Dependency represents a single dependency requirement.
type Dependency struct {
	Name        string
	Description string
	Type        DependencyType
	Required    bool
	MinVersion  string
	InstallFunc func(ctx context.Context, sys SystemInfo) error
	HealthCheck func(ctx context.Context) error
}

// InstallationResult represents the result of a dependency installation.
type InstallationResult struct {
	Dependency  Dependency
	Status      Status
	Version     string
	Error       error
	Duration    time.Duration
	Message     string
}

// Config represents the parsed deps.yaml configuration.
type Config struct {
	Go struct {
		MinVersion      string `yaml:"min_version"`
		ModulesRequired bool   `yaml:"modules_required"`
	} `yaml:"go"`
	ExternalTools []ExternalTool `yaml:"external_tools"`
	InstallDirectory string      `yaml:"install_directory"`
	AddToPath        bool        `yaml:"add_to_path"`
}

// ExternalTool represents an external tool dependency configuration.
type ExternalTool struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Type        string `yaml:"type"`
	Install     struct {
		Github struct {
			Repo          string            `yaml:"repo"`
			AssetPatterns map[string]string `yaml:"asset_patterns"`
			BinaryName    string            `yaml:"binary_name"`
		} `yaml:"github"`
	} `yaml:"install"`
	HealthCheck struct {
		Command         string   `yaml:"command"`
		Args            []string `yaml:"args"`
		ExpectedContains string  `yaml:"expected_contains"`
	} `yaml:"health_check"`
	MinVersion string `yaml:"min_version"`
}

// Installer interface defines the contract for dependency installers.
type Installer interface {
	Name() string
	Check(ctx context.Context, sys SystemInfo) (bool, string, error)
	Install(ctx context.Context, sys SystemInfo) error
	Validate(ctx context.Context) error
}
