// Package subfinder implements integration with Project Discovery's subfinder CLI tool.
// It executes subfinder as a subprocess and parses its JSON output to create artifacts.
package subfinder

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
)

const (
	sourceName     = "subfinder"
	defaultTimeout = 200 * time.Second // subfinder with all sources
	defaultThreads = 10
)

// SubfinderSource implements ports.Source and ports.AdvancedSource.
// It wraps Project Discovery's subfinder CLI tool for subdomain discovery.
type SubfinderSource struct {
	logger      logx.Logger
	execPath    string        // Path to subfinder binary
	timeout     time.Duration
	threads     int
	rateLimit   int
	allSources  bool     // Use -all flag
	sources     []string // Specific sources to use (-s flag)
	parser      *Parser
	progressCh  chan ports.ProgressUpdate

	// Process management
	mu  sync.Mutex
	cmd *exec.Cmd
}

// New creates a new SubfinderSource with default configuration.
func New(logger logx.Logger) *SubfinderSource {
	return &SubfinderSource{
		logger:     logger.With("source", sourceName),
		execPath:   "subfinder",
		timeout:    defaultTimeout,
		threads:    defaultThreads,
		rateLimit:  0, // No limit by default (subfinder manages this internally)
		allSources: true,
		sources:    []string{},
		parser:     NewParser(logger, sourceName),
		progressCh: make(chan ports.ProgressUpdate, 10),
	}
}

// NewWithConfig creates SubfinderSource with custom configuration.
func NewWithConfig(logger logx.Logger, execPath string, timeout time.Duration, threads, rateLimit int, allSources bool, sources []string) *SubfinderSource {
	return &SubfinderSource{
		logger:     logger.With("source", sourceName),
		execPath:   execPath,
		timeout:    timeout,
		threads:    threads,
		rateLimit:  rateLimit,
		allSources: allSources,
		sources:    sources,
		parser:     NewParser(logger, sourceName),
		progressCh: make(chan ports.ProgressUpdate, 10),
	}
}

// Name returns the source name.
func (s *SubfinderSource) Name() string {
	return sourceName
}

// Mode returns the source operation mode (passive).
func (s *SubfinderSource) Mode() domain.SourceMode {
	return domain.SourceModePassive
}

// Type returns the source type (CLI).
func (s *SubfinderSource) Type() domain.SourceType {
	return domain.SourceTypeCLI
}

// Run executes subfinder against the target domain.
func (s *SubfinderSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	result := domain.NewScanResult(target)
	startTime := time.Now()

	s.logger.Info("starting subfinder scan",
		"target", target.Root,
		"all_sources", s.allSources,
		"threads", s.threads,
		"rate_limit", s.rateLimit,
	)

	// Build command with context
	cmd := s.buildCommand(ctx, target)

	// Create stdout pipe for streaming JSON
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Create stderr pipe for warnings
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Store command reference for Close()
	s.mu.Lock()
	s.cmd = cmd
	s.mu.Unlock()

	// Start subfinder process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start subfinder: %w", err)
	}

	s.logger.Debug("subfinder process started", "pid", cmd.Process.Pid)

	// Parse stdout in real-time (streaming JSONL)
	responses := make([]*SubfinderResponse, 0, 100)
	scanner := bufio.NewScanner(stdout)

	// Increase buffer size for large responses
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max token size

	artifactCount := 0
	for scanner.Scan() {
		line := scanner.Text()

		var resp SubfinderResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			s.logger.Warn("failed to parse subfinder output", "line", line, "error", err.Error())
			continue
		}

		responses = append(responses, &resp)
		artifactCount++

		// Emit progress (non-blocking)
		select {
		case s.progressCh <- ports.ProgressUpdate{
			ArtifactCount: artifactCount,
			Message:       fmt.Sprintf("Found %s", resp.Host),
		}:
		default:
			// Channel full, skip update
		}

		s.logger.Debug("parsed subfinder response",
			"host", resp.Host,
			"sources", resp.Source,
		)
	}

	if err := scanner.Err(); err != nil {
		s.logger.Warn("scanner error", "error", err.Error())
	}

	// Capture stderr for warnings
	stderrBytes, _ := io.ReadAll(stderr)
	if len(stderrBytes) > 0 {
		stderrStr := string(stderrBytes)
		s.logger.Debug("subfinder stderr", "output", stderrStr)
		result.AddWarning("subfinder", fmt.Sprintf("stderr output: %s", stderrStr))
	}

	// Wait for process to complete
	if err := cmd.Wait(); err != nil {
		// Don't fail if we got some results
		if len(responses) > 0 {
			s.logger.Warn("subfinder exited with error but produced results", "error", err.Error())
			result.AddWarning("subfinder", fmt.Sprintf("process exited with error: %v", err))
		} else {
			return nil, fmt.Errorf("subfinder failed: %w", err)
		}
	}

	// Parse responses into artifacts
	artifacts := s.parser.ParseMultipleResponses(responses, target)
	for _, artifact := range artifacts {
		result.AddArtifact(artifact)
	}

	duration := time.Since(startTime)
	s.logger.Info("subfinder scan completed",
		"target", target.Root,
		"duration", duration.String(),
		"responses", len(responses),
		"artifacts", len(result.Artifacts),
	)

	return result, nil
}

// ProgressChannel implements ports.StreamingSource
func (s *SubfinderSource) ProgressChannel() <-chan ports.ProgressUpdate {
	return s.progressCh
}

// Stream implements ports.StreamingSource (no usado actualmente pero requerido por interfaz)
func (s *SubfinderSource) Stream(ctx context.Context, target domain.Target) (<-chan *domain.Artifact, <-chan error) {
	artifactCh := make(chan *domain.Artifact, 100)
	errorCh := make(chan error, 1)

	go func() {
		defer close(artifactCh)
		defer close(errorCh)

		result, err := s.Run(ctx, target)
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

// Close terminates the subfinder process and cleans up resources.
func (s *SubfinderSource) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Debug("closing subfinder source")

	// Kill process if still running
	if s.cmd != nil && s.cmd.Process != nil {
		// Check if process is still running
		if s.cmd.ProcessState == nil || !s.cmd.ProcessState.Exited() {
			// Try SIGTERM first
			if err := s.cmd.Process.Signal(os.Interrupt); err != nil {
				// Only log if it's not "already finished"
				if err.Error() != "os: process already finished" {
					s.logger.Warn("SIGTERM failed, forcing kill", "error", err.Error())
					if killErr := s.cmd.Process.Kill(); killErr != nil && killErr.Error() != "os: process already finished" {
						s.logger.Warn("failed to kill subfinder process", "error", killErr.Error())
					}
				}
			}
		}

		s.cmd = nil
	}

	s.logger.Debug("subfinder source closed")
	return nil
}

// Initialize verifies that subfinder is installed and accessible.
// Implements ports.AdvancedSource.
func (s *SubfinderSource) Initialize() error {
	s.logger.Debug("initializing subfinder source", "exec_path", s.execPath)

	// Check if subfinder binary exists
	execPath, err := exec.LookPath(s.execPath)
	if err != nil {
		return fmt.Errorf("subfinder not found in PATH: %w (install from: https://github.com/projectdiscovery/subfinder)", err)
	}

	s.execPath = execPath
	s.logger.Debug("found subfinder binary", "path", execPath)

	// Check version
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.execPath, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check subfinder version: %w", err)
	}

	version := string(output)
	s.logger.Info("subfinder initialized successfully", "version", version)

	return nil
}

// Validate checks if the source configuration is valid.
// Implements ports.AdvancedSource.
func (s *SubfinderSource) Validate() error {
	if s.execPath == "" {
		return fmt.Errorf("subfinder exec path is empty")
	}

	if s.timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if s.threads <= 0 || s.threads > 1000 {
		return fmt.Errorf("threads must be between 1 and 1000")
	}

	if s.rateLimit < 0 {
		return fmt.Errorf("rate limit cannot be negative")
	}

	if !s.allSources && len(s.sources) == 0 {
		return fmt.Errorf("either allSources must be true or sources must be specified")
	}

	return nil
}

// HealthCheck verifies that subfinder is responsive.
// Implements ports.AdvancedSource.
func (s *SubfinderSource) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.execPath, "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("subfinder health check failed: %w", err)
	}

	return nil
}

// buildCommand constructs the subfinder command with appropriate flags.
func (s *SubfinderSource) buildCommand(ctx context.Context, target domain.Target) *exec.Cmd {
	args := []string{
		"-d", target.Root, // Target domain
		"-oJ",             // JSON output
		"-silent",         // No progress output
		"-nc",             // No color
	}

	// Add source selection flags
	if s.allSources {
		args = append(args, "-all")
	} else if len(s.sources) > 0 {
		args = append(args, "-s", joinSources(s.sources))
	}

	// Add performance flags
	args = append(args, "-t", strconv.Itoa(s.threads))

	if s.rateLimit > 0 {
		args = append(args, "-rl", strconv.Itoa(s.rateLimit))
	}

	// Add timeout flag (in seconds)
	args = append(args, "-timeout", strconv.Itoa(int(s.timeout.Seconds())))

	// Use parent context directly
	cmd := exec.CommandContext(ctx, s.execPath, args...)

	s.logger.Debug("built subfinder command",
		"args", args,
		"timeout", s.timeout.String(),
	)

	return cmd
}

// joinSources joins source names with commas for -s flag.
func joinSources(sources []string) string {
	result := ""
	for i, src := range sources {
		if i > 0 {
			result += ","
		}
		result += src
	}
	return result
}
