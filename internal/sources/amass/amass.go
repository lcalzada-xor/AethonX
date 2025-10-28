// Package amass implements integration with OWASP Amass CLI tool.
// It executes amass as a subprocess and reads results from SQLite database.
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
	"aethonx/internal/platform/logx"
	"aethonx/internal/sources/common"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

const (
	sourceName     = "amass"
	defaultTimeout = 300 * time.Second // 5 minutes for amass
)

// AmassSource implements ports.Source and ports.AdvancedSource.
// It wraps OWASP Amass CLI tool for subdomain enumeration and network mapping.
type AmassSource struct {
	*common.BaseCLISource // Embedded base for subprocess management

	activeMode bool // Enable --active flag
	maxDNSQPS  int  // DNS queries per second (0 = unlimited)
	brute      bool // Enable brute force
	alts       bool // Enable alterations
}

// AmassConfig contains configuration for AmassSource.
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
		BaseCLISource: common.NewBaseCLISource(logger, common.BaseCLIConfig{
			SourceName:     sourceName,
			ExecPath:       "amass",
			Timeout:        defaultTimeout,
			ProgressBuffer: 10,
		}),
		activeMode: false,
		maxDNSQPS:  0,
		brute:      false,
		alts:       false,
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
		BaseCLISource: common.NewBaseCLISource(logger, common.BaseCLIConfig{
			SourceName:     sourceName,
			ExecPath:       cfg.ExecPath,
			Timeout:        cfg.Timeout,
			ProgressBuffer: 10,
		}),
		activeMode: cfg.ActiveMode,
		maxDNSQPS:  cfg.MaxDNSQPS,
		brute:      cfg.Brute,
		alts:       cfg.Alts,
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
// Note: Amass is special because it writes to a database file instead of stdout,
// so we don't use the standard ExecuteCLI pattern with OutputHandler.
func (a *AmassSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	result := domain.NewScanResult(target)
	startTime := time.Now()

	a.GetLogger().Info("starting amass scan",
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

	a.GetLogger().Debug("created temp directory for amass", "dir", tempDir)

	// Build command arguments
	args := a.buildCommandArgs(target, tempDir)

	// Build command manually (amass needs special handling for database output)
	cmd := exec.CommandContext(ctx, a.GetExecPath(), args...)

	// Create stderr pipe to capture progress/warnings
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start amass process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start amass: %w", err)
	}

	a.GetLogger().Debug("amass process started", "pid", cmd.Process.Pid)

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
			a.GetLogger().Debug("amass output", "line", line)
		}
		if err := scanner.Err(); err != nil {
			a.GetLogger().Warn("error reading stderr", "error", err.Error())
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
		a.GetLogger().Debug("amass produced output", "lines", stderrCount)
	}

	// Read results from SQLite database
	// Amass creates a subdirectory like: tempDir/db/amass.sqlite
	// Try multiple possible paths
	possibleDBPaths := []string{
		fmt.Sprintf("%s/db/amass.sqlite", tempDir), // Amass v4 default
		fmt.Sprintf("%s/amass.sqlite", tempDir),    // Direct path
	}

	var artifacts []*domain.Artifact
	var dbErr error
	dbFound := false

	for _, dbPath := range possibleDBPaths {
		a.GetLogger().Debug("trying database path", "path", dbPath)
		artifacts, dbErr = a.readDatabaseResults(dbPath, target)
		if dbErr == nil {
			dbFound = true
			a.GetLogger().Debug("successfully read database", "path", dbPath, "artifacts", len(artifacts))
			break
		}
		a.GetLogger().Debug("database not found at path", "path", dbPath, "error", dbErr.Error())
	}

	if !dbFound {
		// If database read fails, fall back to text file parsing
		a.GetLogger().Warn("failed to read database from any path, trying text file", "last_error", dbErr.Error())
		txtPath := fmt.Sprintf("%s/amass.txt", tempDir)
		artifacts, dbErr = a.readTextResults(txtPath, target)
		if dbErr != nil {
			return nil, fmt.Errorf("failed to read amass results from database or text file: %w", dbErr)
		}
	}

	// Log warning if no artifacts found
	if len(artifacts) == 0 {
		a.GetLogger().Warn("amass completed but found 0 artifacts",
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
	a.GetLogger().Info("amass scan completed",
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
			a.GetLogger().Warn("failed to scan row", "error", err.Error())
			continue
		}

		// Parse content JSON
		var content map[string]interface{}
		if err := json.Unmarshal([]byte(contentJSON), &content); err != nil {
			a.GetLogger().Warn("failed to parse content JSON", "content", contentJSON, "error", err.Error())
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

	a.GetLogger().Debug("read database results",
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

	a.GetLogger().Debug("read text results",
		"txt_path", txtPath,
		"artifacts", len(artifacts),
	)

	return artifacts, nil
}

// Stream implements ports.StreamingSource.
func (a *AmassSource) Stream(ctx context.Context, target domain.Target) (<-chan *domain.Artifact, <-chan error) {
	return a.DefaultStream(ctx, target, a.Run)
}

// Initialize verifies that amass is installed and accessible.
// Implements ports.AdvancedSource.
func (a *AmassSource) Initialize() error {
	return a.DefaultInitialize(
		"amass",
		"https://github.com/owasp-amass/amass",
	)
}

// Validate checks if the source configuration is valid.
// Implements ports.AdvancedSource.
func (a *AmassSource) Validate() error {
	// First check base validation
	if err := a.DefaultValidate(); err != nil {
		return err
	}

	// Additional amass-specific validation
	if a.maxDNSQPS < 0 {
		return fmt.Errorf("max DNS QPS cannot be negative")
	}

	return nil
}

// HealthCheck verifies that amass is responsive.
// Implements ports.AdvancedSource.
func (a *AmassSource) HealthCheck(ctx context.Context) error {
	return a.DefaultHealthCheck(ctx)
}

// buildCommandArgs constructs the amass command arguments.
func (a *AmassSource) buildCommandArgs(target domain.Target, outputDir string) []string {
	args := []string{
		"enum",            // Use enum subcommand
		"-d", target.Root, // Target domain
		"-dir", outputDir, // Output directory for database
		"-nocolor",        // No color in output
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
	timeoutMinutes := int(a.GetTimeout().Minutes())
	if timeoutMinutes <= 0 {
		// For timeouts less than 1 minute, still pass 1 minute to amass
		// This is a limitation of amass CLI which only accepts minute granularity
		timeoutMinutes = 1
		a.GetLogger().Debug("timeout less than 1 minute, rounding up to 1 minute for amass",
			"original_timeout", a.GetTimeout().String())
	}
	args = append(args, "-timeout", strconv.Itoa(timeoutMinutes))

	a.GetLogger().Debug("built amass command",
		"args", args,
		"timeout", a.GetTimeout().String(),
		"output_dir", outputDir,
	)

	return args
}
