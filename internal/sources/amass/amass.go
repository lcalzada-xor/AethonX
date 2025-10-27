// Package amass implements integration with OWASP Amass CLI tool.
// It executes amass as a subprocess and parses its JSON output to create artifacts.
package amass

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

const (
	sourceName     = "amass"
	defaultTimeout = 300 * time.Second // 5 minutes for amass
)

// AmassSource implements ports.Source and ports.AdvancedSource.
// It wraps OWASP Amass CLI tool for subdomain enumeration and network mapping.
type AmassSource struct {
	logger     logx.Logger
	execPath   string        // Path to amass binary
	timeout    time.Duration
	activeMode bool          // Enable --active flag
	maxDNSQPS  int          // DNS queries per second (0 = unlimited)
	brute      bool         // Enable brute force
	alts       bool         // Enable alterations
	progressCh chan ports.ProgressUpdate

	// Process management
	mu  sync.Mutex
	cmd *exec.Cmd
}

// AmassConfig contiene la configuración para AmassSource.
type AmassConfig struct {
	ExecPath   string
	Timeout    time.Duration
	ActiveMode bool
	MaxDNSQPS  int
	Brute      bool
	Alts       bool
}

// New creates a new AmassSource with default configuration.
func New(logger logx.Logger) *AmassSource {
	return &AmassSource{
		logger:     logger.With("source", sourceName),
		execPath:   "amass",
		timeout:    defaultTimeout,
		activeMode: false,
		maxDNSQPS:  0,
		brute:      false,
		alts:       false,
		progressCh: make(chan ports.ProgressUpdate, 10),
	}
}

// NewWithConfig creates AmassSource with custom configuration.
func NewWithConfig(logger logx.Logger, cfg AmassConfig) *AmassSource {
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.ExecPath == "" {
		cfg.ExecPath = "amass"
	}

	return &AmassSource{
		logger:     logger.With("source", sourceName),
		execPath:   cfg.ExecPath,
		timeout:    cfg.Timeout,
		activeMode: cfg.ActiveMode,
		maxDNSQPS:  cfg.MaxDNSQPS,
		brute:      cfg.Brute,
		alts:       cfg.Alts,
		progressCh: make(chan ports.ProgressUpdate, 10),
	}
}

// Name returns the source name.
func (a *AmassSource) Name() string {
	return sourceName
}

// Mode returns the source operation mode (both/hybrid).
// Amass can operate in passive or active mode depending on configuration.
func (a *AmassSource) Mode() domain.SourceMode {
	return domain.SourceModeBoth
}

// Type returns the source type (CLI).
func (a *AmassSource) Type() domain.SourceType {
	return domain.SourceTypeCLI
}

// Run executes amass enum against the target domain.
func (a *AmassSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	result := domain.NewScanResult(target)
	startTime := time.Now()

	a.logger.Info("starting amass scan",
		"target", target.Root,
		"active", a.activeMode,
		"brute", a.brute,
		"alts", a.alts,
		"max_dns_qps", a.maxDNSQPS,
	)

	// Create temporary directory for amass output
	tempDir, err := os.MkdirTemp("", "aethonx-amass-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up on exit

	a.logger.Debug("created temp directory for amass", "dir", tempDir)

	// Build command with context and temp directory
	cmd := a.buildCommand(ctx, target, tempDir)

	// Create stderr pipe to capture progress/warnings
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Store command reference for Close()
	a.mu.Lock()
	a.cmd = cmd
	a.mu.Unlock()

	// Start amass process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start amass: %w", err)
	}

	a.logger.Debug("amass process started", "pid", cmd.Process.Pid)

	// Read stderr in background (contains progress output and discovered FQDNs)
	var stderrLines []string
	var stderrMu sync.Mutex
	var stderrWg sync.WaitGroup
	stderrWg.Add(1)

	go func() {
		defer stderrWg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			stderrMu.Lock()
			stderrLines = append(stderrLines, line)
			stderrMu.Unlock()

			// Log progress lines
			a.logger.Debug("amass output", "line", line)
		}
		if err := scanner.Err(); err != nil {
			a.logger.Warn("error reading stderr", "error", err.Error())
		}
	}()

	// Wait for process to complete
	if err := cmd.Wait(); err != nil {
		// Wait for stderr goroutine to finish before returning error
		stderrWg.Wait()
		return nil, fmt.Errorf("amass failed: %w", err)
	}

	// Wait for stderr goroutine to finish reading all output
	stderrWg.Wait()

	// Get stderr output
	stderrMu.Lock()
	stderrCount := len(stderrLines)
	stderrMu.Unlock()

	if stderrCount > 0 {
		a.logger.Debug("amass produced output", "lines", stderrCount)
	}

	// Read results from SQLite database
	// Amass creates a subdirectory like: tempDir/db/amass.sqlite
	// Try multiple possible paths
	possibleDBPaths := []string{
		fmt.Sprintf("%s/db/amass.sqlite", tempDir),      // Amass v4 default
		fmt.Sprintf("%s/amass.sqlite", tempDir),          // Direct path
	}

	var artifacts []*domain.Artifact
	var dbErr error
	dbFound := false

	for _, dbPath := range possibleDBPaths {
		a.logger.Debug("trying database path", "path", dbPath)
		artifacts, dbErr = a.readDatabaseResults(dbPath, target)
		if dbErr == nil {
			dbFound = true
			a.logger.Debug("successfully read database", "path", dbPath, "artifacts", len(artifacts))
			break
		}
		a.logger.Debug("database not found at path", "path", dbPath, "error", dbErr.Error())
	}

	if !dbFound {
		// If database read fails, fall back to text file parsing
		a.logger.Warn("failed to read database from any path, trying text file", "last_error", dbErr.Error())
		txtPath := fmt.Sprintf("%s/amass.txt", tempDir)
		artifacts, dbErr = a.readTextResults(txtPath, target)
		if dbErr != nil {
			return nil, fmt.Errorf("failed to read amass results from database or text file: %w", dbErr)
		}
	}

	// Log warning if no artifacts found
	if len(artifacts) == 0 {
		a.logger.Warn("amass completed but found 0 artifacts",
			"target", target.Root,
			"stderr_lines", stderrCount,
			"temp_dir", tempDir,
		)
		result.AddWarning("amass", "scan completed but no artifacts were found - target may have no exposed subdomains or amass sources are rate-limited")
	}

	// Add artifacts to result
	for _, artifact := range artifacts {
		result.AddArtifact(artifact)
	}

	duration := time.Since(startTime)
	a.logger.Info("amass scan completed",
		"target", target.Root,
		"duration", duration.String(),
		"artifacts", len(result.Artifacts),
	)

	return result, nil
}

// readDatabaseResults reads and parses the SQLite database created by amass.
func (a *AmassSource) readDatabaseResults(dbPath string, target domain.Target) ([]*domain.Artifact, error) {
	// Check if database file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("database file not found: %s", dbPath)
	}

	// Open SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Query all assets
	rows, err := db.Query("SELECT type, content FROM assets ORDER BY created_at")
	if err != nil {
		return nil, fmt.Errorf("failed to query assets: %w", err)
	}
	defer rows.Close()

	artifacts := make([]*domain.Artifact, 0, 100)
	seenFQDNs := make(map[string]bool) // Deduplicate FQDNs

	for rows.Next() {
		var assetType string
		var contentJSON string

		if err := rows.Scan(&assetType, &contentJSON); err != nil {
			a.logger.Warn("failed to scan row", "error", err.Error())
			continue
		}

		// Parse content JSON
		var content map[string]interface{}
		if err := json.Unmarshal([]byte(contentJSON), &content); err != nil {
			a.logger.Warn("failed to parse content JSON", "content", contentJSON, "error", err.Error())
			continue
		}

		// Process based on asset type
		switch assetType {
		case "FQDN":
			fqdn, ok := content["name"].(string)
			if !ok || fqdn == "" {
				continue
			}

			// Skip duplicates
			if seenFQDNs[fqdn] {
				continue
			}
			seenFQDNs[fqdn] = true

			// Create subdomain artifact
			artifact := domain.NewArtifact(
				domain.ArtifactTypeSubdomain,
				fqdn,
				sourceName,
			)
			// Set confidence based on active mode
			if a.activeMode {
				artifact.Confidence = domain.ConfidenceHigh // Active DNS validation
			} else {
				artifact.Confidence = domain.ConfidenceMedium // Passive discovery
			}
			artifacts = append(artifacts, artifact)

		case "IPAddress":
			if addr, ok := content["address"].(string); ok && addr != "" {
				artifact := domain.NewArtifact(
					domain.ArtifactTypeIP,
					addr,
					sourceName,
				)
				if a.activeMode {
					artifact.Confidence = domain.ConfidenceHigh
				} else {
					artifact.Confidence = domain.ConfidenceMedium
				}
				artifacts = append(artifacts, artifact)
			}

		case "Netblock":
			if cidr, ok := content["cidr"].(string); ok && cidr != "" {
				artifact := domain.NewArtifact(
					domain.ArtifactTypeCIDR,
					cidr,
					sourceName,
				)
				if a.activeMode {
					artifact.Confidence = domain.ConfidenceHigh
				} else {
					artifact.Confidence = domain.ConfidenceMedium
				}
				artifacts = append(artifacts, artifact)
			}

		case "ASN":
			if asn, ok := content["number"].(float64); ok {
				asnValue := fmt.Sprintf("AS%d", int(asn))
				artifact := domain.NewArtifact(
					domain.ArtifactTypeASN,
					asnValue,
					sourceName,
				)
				if a.activeMode {
					artifact.Confidence = domain.ConfidenceHigh
				} else {
					artifact.Confidence = domain.ConfidenceMedium
				}
				artifacts = append(artifacts, artifact)
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	a.logger.Debug("read database results",
		"db_path", dbPath,
		"artifacts", len(artifacts),
	)

	return artifacts, nil
}

// readTextResults reads and parses the text file created by amass (fallback).
func (a *AmassSource) readTextResults(txtPath string, target domain.Target) ([]*domain.Artifact, error) {
	// Check if text file exists
	if _, err := os.Stat(txtPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("text file not found: %s", txtPath)
	}

	file, err := os.Open(txtPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open text file: %w", err)
	}
	defer file.Close()

	artifacts := make([]*domain.Artifact, 0, 100)
	seenFQDNs := make(map[string]bool)

	// Regex to extract FQDN from lines like: "example.com (FQDN) --> ns_record --> a.iana-servers.net (FQDN)"
	fqdnRegex := regexp.MustCompile(`([a-zA-Z0-9][-a-zA-Z0-9.]*[a-zA-Z0-9])\s*\(FQDN\)`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Extract all FQDNs from the line
		matches := fqdnRegex.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}

			fqdn := strings.TrimSpace(match[1])
			if fqdn == "" || seenFQDNs[fqdn] {
				continue
			}

			seenFQDNs[fqdn] = true

			// Create subdomain artifact
			artifact := domain.NewArtifact(
				domain.ArtifactTypeSubdomain,
				fqdn,
				sourceName,
			)
			// Set confidence based on active mode
			if a.activeMode {
				artifact.Confidence = domain.ConfidenceHigh
			} else {
				artifact.Confidence = domain.ConfidenceMedium
			}
			artifacts = append(artifacts, artifact)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading text file: %w", err)
	}

	a.logger.Debug("read text results",
		"txt_path", txtPath,
		"artifacts", len(artifacts),
	)

	return artifacts, nil
}

// ProgressChannel implements ports.StreamingSource
func (a *AmassSource) ProgressChannel() <-chan ports.ProgressUpdate {
	return a.progressCh
}

// Stream implements ports.StreamingSource (no usado actualmente pero requerido por interfaz)
func (a *AmassSource) Stream(ctx context.Context, target domain.Target) (<-chan *domain.Artifact, <-chan error) {
	artifactCh := make(chan *domain.Artifact, 100)
	errorCh := make(chan error, 1)

	go func() {
		defer close(artifactCh)
		defer close(errorCh)

		result, err := a.Run(ctx, target)
		if err != nil {
			errorCh <- err
			return
		}

		for _, artifact := range result.Artifacts {
			select {
			case artifactCh <- artifact:
			case <-ctx.Done():
				return
			}
		}
	}()

	return artifactCh, errorCh
}

// Close terminates the amass process and cleans up resources.
func (a *AmassSource) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.logger.Debug("closing amass source")

	// Kill process if still running
	if a.cmd != nil && a.cmd.Process != nil {
		// Check if process is still running
		if a.cmd.ProcessState == nil || !a.cmd.ProcessState.Exited() {
			// Try SIGTERM first
			if err := a.cmd.Process.Signal(os.Interrupt); err != nil {
				// Check if process already finished (not a real error)
				if err != os.ErrProcessDone {
					a.logger.Warn("SIGTERM failed, forcing kill", "error", err.Error())
					if killErr := a.cmd.Process.Kill(); killErr != nil && killErr != os.ErrProcessDone {
						a.logger.Warn("failed to kill amass process", "error", killErr.Error())
					}
				}
			}
		}

		a.cmd = nil
	}

	a.logger.Debug("amass source closed")
	return nil
}

// Initialize verifies that amass is installed and accessible.
// Implements ports.AdvancedSource.
func (a *AmassSource) Initialize() error {
	a.logger.Debug("initializing amass source", "exec_path", a.execPath)

	// Check if amass binary exists
	execPath, err := exec.LookPath(a.execPath)
	if err != nil {
		return fmt.Errorf("amass not found in PATH: %w (install from: https://github.com/owasp-amass/amass)", err)
	}

	a.execPath = execPath
	a.logger.Debug("found amass binary", "path", execPath)

	// Check version
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.execPath, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check amass version: %w", err)
	}

	version := string(output)
	a.logger.Info("amass initialized successfully", "version", version)

	return nil
}

// Validate checks if the source configuration is valid.
// Implements ports.AdvancedSource.
func (a *AmassSource) Validate() error {
	// Note: execPath defaults to "amass" in NewWithConfig
	// So we only check if it's been explicitly set to empty after construction
	if a.execPath == "" {
		return fmt.Errorf("amass exec path is empty")
	}

	// Note: timeout defaults to defaultTimeout in NewWithConfig
	// But after construction, it should never be <= 0
	if a.timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got %v", a.timeout)
	}

	if a.maxDNSQPS < 0 {
		return fmt.Errorf("max DNS QPS cannot be negative")
	}

	return nil
}

// HealthCheck verifies that amass is responsive.
// Implements ports.AdvancedSource.
func (a *AmassSource) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.execPath, "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("amass health check failed: %w", err)
	}

	return nil
}

// buildCommand constructs the amass command with appropriate flags.
func (a *AmassSource) buildCommand(ctx context.Context, target domain.Target, outputDir string) *exec.Cmd {
	args := []string{
		"enum",               // Use enum subcommand
		"-d", target.Root,    // Target domain
		"-dir", outputDir,    // Output directory for database
		"-nocolor",           // No color in output
	}

	// Active mode flag
	if a.activeMode {
		args = append(args, "-active")
	}

	// Brute force flag
	if a.brute {
		args = append(args, "-brute")
	}

	// Alterations flag
	if a.alts {
		args = append(args, "-alts")
	}

	// DNS rate limiting
	if a.maxDNSQPS > 0 {
		args = append(args, "-dns-qps", strconv.Itoa(a.maxDNSQPS))
	}

	// Timeout (in minutes) - round up to at least 1 minute
	timeoutMinutes := int(a.timeout.Minutes())
	if timeoutMinutes <= 0 {
		// For timeouts less than 1 minute, still pass 1 minute to amass
		// This is a limitation of amass CLI which only accepts minute granularity
		timeoutMinutes = 1
		a.logger.Debug("timeout less than 1 minute, rounding up to 1 minute for amass",
			"original_timeout", a.timeout.String())
	}
	args = append(args, "-timeout", strconv.Itoa(timeoutMinutes))

	// Use parent context directly
	cmd := exec.CommandContext(ctx, a.execPath, args...)

	a.logger.Debug("built amass command",
		"args", args,
		"timeout", a.timeout.String(),
		"output_dir", outputDir,
	)

	return cmd
}
