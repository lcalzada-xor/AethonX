// Package waybackurls implements integration with waybackurls CLI tool.
// It executes waybackurls as a subprocess and parses its output to create artifacts.
package waybackurls

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/urlfilter"
	"aethonx/internal/sources/common"
)

const (
	sourceName     = "waybackurls"
	defaultTimeout = 120 * time.Second // Wayback Machine can be slow
)

// WaybackurlsSource implements ports.Source and ports.AdvancedSource.
// It wraps waybackurls CLI tool for historical URL discovery.
type WaybackurlsSource struct {
	*common.BaseCLISource // Embedded base for subprocess management

	withDates bool                   // -dates flag
	noSubs    bool                   // -no-subs flag
	parser    *Parser                // Output parser
	filter    *urlfilter.FilterEngine // URL filter engine
	filterCfg urlfilter.FilterConfig  // Filter configuration
}

// New creates a new WaybackurlsSource with default configuration.
func New(logger logx.Logger) *WaybackurlsSource {
	filterCfg := urlfilter.DefaultConfig()

	return &WaybackurlsSource{
		BaseCLISource: common.NewBaseCLISource(logger, common.BaseCLIConfig{
			SourceName:     sourceName,
			ExecPath:       "waybackurls",
			Timeout:        defaultTimeout,
			ProgressBuffer: 100,
		}),
		withDates: false,
		noSubs:    false,
		parser:    NewParser(logger, sourceName),
		filter:    urlfilter.NewFilterEngine(filterCfg, logger),
		filterCfg: filterCfg,
	}
}

// NewWithConfig creates WaybackurlsSource with custom configuration.
func NewWithConfig(logger logx.Logger, execPath string, timeout time.Duration, withDates, noSubs bool, filterCfg urlfilter.FilterConfig) *WaybackurlsSource {
	return &WaybackurlsSource{
		BaseCLISource: common.NewBaseCLISource(logger, common.BaseCLIConfig{
			SourceName:     sourceName,
			ExecPath:       execPath,
			Timeout:        timeout,
			ProgressBuffer: 100,
		}),
		withDates: withDates,
		noSubs:    noSubs,
		parser:    NewParser(logger, sourceName),
		filter:    urlfilter.NewFilterEngine(filterCfg, logger),
		filterCfg: filterCfg,
	}
}

// Name returns the source name.
func (w *WaybackurlsSource) Name() string {
	return sourceName
}

// Mode returns the source operation mode (passive).
func (w *WaybackurlsSource) Mode() domain.SourceMode {
	return domain.SourceModePassive
}

// Type returns the source type (CLI).
func (w *WaybackurlsSource) Type() domain.SourceType {
	return domain.SourceTypeCLI
}

// Run executes waybackurls against the target domain.
func (w *WaybackurlsSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	startTime := time.Now()

	w.GetLogger().Info("starting waybackurls scan",
		"target", target.Root,
		"with_dates", w.withDates,
		"no_subs", w.noSubs,
		"timeout", w.GetTimeout().String(),
		"filter_enabled", w.filter != nil,
		"max_urls", w.filterCfg.MaxURLs,
	)

	// Build command arguments
	args := w.buildCommandArgs()

	// Create result
	result := domain.NewScanResult(target)

	// Create handler for processing output
	handler := &waybackurlsHandler{
		parser:    w.parser,
		filter:    w.filter,
		filterCfg: w.filterCfg,
		target:    target,
		logger:    w.GetLogger(),
		result:    result,
		rawURLs:   make([]string, 0, 10000),
	}

	// waybackurls requires the domain to be passed via stdin (not as argument)
	// We need to manually create the command with StdinPipe
	cmd := exec.CommandContext(ctx, w.GetExecPath(), args...)

	// Create stdout pipe for streaming output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Create stderr pipe for warnings
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Create stdin pipe to send domain
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Start waybackurls process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start waybackurls: %w", err)
	}

	w.GetLogger().Debug("waybackurls process started", "pid", cmd.Process.Pid)

	// Write domain to stdin in goroutine
	go func() {
		defer stdin.Close()
		fmt.Fprintln(stdin, target.Root)
	}()

	// Read stderr in background to prevent blocking (critical for Ctrl-C handling)
	var stderrBytes []byte
	var stderrMu sync.Mutex
	var stderrWg sync.WaitGroup
	stderrWg.Add(1)

	go func() {
		defer stderrWg.Done()
		data, readErr := io.ReadAll(stderr)
		if readErr != nil {
			w.GetLogger().Warn("error reading stderr", "error", readErr.Error())
		}
		stderrMu.Lock()
		stderrBytes = data
		stderrMu.Unlock()
	}()

	// Process stdout using handler
	if err := w.ProcessOutput(stdout, handler); err != nil {
		w.GetLogger().Warn("output processing error", "error", err.Error())
	}

	// Finalize handler (applies filtering and creates artifacts)
	if err := handler.Finalize(); err != nil {
		w.GetLogger().Warn("handler finalization error", "error", err.Error())
	}

	// Wait for stderr goroutine to finish before checking process exit
	stderrWg.Wait()

	// Capture stderr for warnings
	stderrMu.Lock()
	stderrOutput := string(stderrBytes)
	stderrMu.Unlock()

	if len(stderrOutput) > 0 {
		w.GetLogger().Debug("waybackurls stderr", "output", stderrOutput)
		result.AddWarning("waybackurls", fmt.Sprintf("stderr output: %s", stderrOutput))
	}

	// Wait for process to complete
	waitErr := cmd.Wait()
	artifactCount := len(result.Artifacts)

	if waitErr != nil {
		// Log error pero SIEMPRE retornar resultados parciales
		w.GetLogger().Warn("waybackurls exited with error",
			"error", waitErr.Error(),
			"artifacts", artifactCount,
		)
		result.AddWarning("waybackurls", fmt.Sprintf("process exited with error: %v", waitErr))

		// Si hay resultados parciales, retornarlos con error (orchestrator los consolidará)
		if artifactCount > 0 {
			w.GetLogger().Info("returning partial results from waybackurls",
				"artifacts", artifactCount,
			)
			// Retornar resultado parcial + error para que orchestrator decida
			return result, fmt.Errorf("waybackurls timeout/cancelled but produced %d partial results: %w", artifactCount, waitErr)
		}

		// Sin resultados: retornar error total
		return nil, fmt.Errorf("waybackurls failed without results: %w", waitErr)
	}

	// Log warnings if no results
	urlCount := handler.urlCount
	if urlCount == 0 {
		w.GetLogger().Warn("waybackurls completed but found 0 URLs", "target", target.Root)
		result.AddWarning("waybackurls", "scan completed but no URLs were found - target may not be archived in Wayback Machine")
	}

	// Log final statistics
	duration := time.Since(startTime)
	w.GetLogger().Info("waybackurls scan completed",
		"target", target.Root,
		"duration", duration.String(),
		"urls_processed", urlCount,
		"artifacts", len(result.Artifacts),
	)

	// Log artifact breakdown by type
	typeCount := make(map[domain.ArtifactType]int)
	for _, artifact := range result.Artifacts {
		typeCount[artifact.Type]++
	}
	w.GetLogger().Debug("artifact breakdown", "counts", typeCount)

	return result, nil
}

// waybackurlsHandler implements common.OutputHandler for waybackurls output processing.
type waybackurlsHandler struct {
	parser    *Parser
	filter    *urlfilter.FilterEngine
	filterCfg urlfilter.FilterConfig
	target    domain.Target
	logger    logx.Logger
	result    *domain.ScanResult // Store result to populate artifacts

	// State
	rawURLs  []string
	urlCount int
	mu       sync.Mutex
}

// ProcessLine handles each line of waybackurls stdout.
func (h *waybackurlsHandler) ProcessLine(line []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.urlCount++
	lineStr := string(line)

	// Extract just the URL (remove timestamp if present)
	urlStr, _ := h.parser.ExtractURLAndTimestamp(lineStr)
	if urlStr != "" {
		h.rawURLs = append(h.rawURLs, urlStr)
	}

	// Apply volume control early if filtering is enabled
	if h.filter != nil && h.filterCfg.EnableVolumeControl &&
		h.filterCfg.MaxURLs > 0 && len(h.rawURLs) >= h.filterCfg.MaxURLs {
		h.logger.Warn("reached max URLs, stopping collection",
			"max", h.filterCfg.MaxURLs,
			"current", len(h.rawURLs),
		)
		// Return error to stop processing (non-fatal)
		return fmt.Errorf("max URLs reached: %d", h.filterCfg.MaxURLs)
	}

	return nil
}

// Finalize is called after all lines are processed.
func (h *waybackurlsHandler) Finalize() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.logger.Info("collected raw URLs", "count", len(h.rawURLs))

	// Apply intelligent filtering (if enabled)
	var filteredURLs []string

	if h.filter != nil && len(h.rawURLs) > 0 {
		h.logger.Info("applying URL filtering", "input_urls", len(h.rawURLs))

		ctx := context.Background()
		scoredURLs, stats, err := h.filter.Filter(ctx, h.rawURLs)
		if err != nil {
			h.logger.Warn("filter failed, using unfiltered URLs", "error", err.Error())
			filteredURLs = h.rawURLs
		} else {
			// Extract URLs from scored results (already sorted by priority)
			filteredURLs = make([]string, len(scoredURLs))
			for i, s := range scoredURLs {
				filteredURLs[i] = s.URL
			}

			h.logger.Info("filtering complete",
				"input", stats.InputURLs,
				"output", stats.OutputURLs,
				"reduction", fmt.Sprintf("%.1f%%", stats.ReductionRatio()),
				"duplicates", stats.DuplicatesSkipped,
				"low_priority", stats.LowPrioritySkipped,
				"clusters", stats.ClustersMerged,
				"patterns", stats.PatternsFound,
			)

			// Store filter stats in result metadata
			if h.result.Metadata.Environment == nil {
				h.result.Metadata.Environment = make(map[string]string)
			}
			h.result.Metadata.Environment["waybackurls_filter_input_urls"] = fmt.Sprintf("%d", stats.InputURLs)
			h.result.Metadata.Environment["waybackurls_filter_output_urls"] = fmt.Sprintf("%d", stats.OutputURLs)
			h.result.Metadata.Environment["waybackurls_filter_reduction_ratio"] = fmt.Sprintf("%.1f%%", stats.ReductionRatio())
			h.result.Metadata.Environment["waybackurls_filter_duplicates"] = fmt.Sprintf("%d", stats.DuplicatesSkipped)
			h.result.Metadata.Environment["waybackurls_filter_low_priority"] = fmt.Sprintf("%d", stats.LowPrioritySkipped)
			h.result.Metadata.Environment["waybackurls_filter_clusters"] = fmt.Sprintf("%d", stats.ClustersMerged)
			h.result.Metadata.Environment["waybackurls_filter_patterns"] = fmt.Sprintf("%d", stats.PatternsFound)
			h.result.Metadata.Environment["waybackurls_filter_duration_ms"] = fmt.Sprintf("%d", stats.DurationMs)
		}
	} else {
		// No filtering, use all collected URLs
		filteredURLs = h.rawURLs
	}

	// Parse filtered URLs to artifacts
	h.logger.Info("parsing URLs to artifacts", "count", len(filteredURLs))

	seen := make(map[string]bool)
	artifactCount := 0

	for _, urlStr := range filteredURLs {
		artifacts := h.parser.ParseLine(urlStr, h.target)

		for _, artifact := range artifacts {
			// Deduplicate artifacts
			key := string(artifact.Type) + ":" + artifact.Value
			if !seen[key] {
				seen[key] = true
				h.result.AddArtifact(artifact)
				artifactCount++
			}
		}
	}

	h.logger.Info("finalization complete",
		"filtered_urls", len(filteredURLs),
		"artifacts", artifactCount,
	)

	return nil
}

// Stream implements ports.StreamingSource.
func (w *WaybackurlsSource) Stream(ctx context.Context, target domain.Target) (<-chan *domain.Artifact, <-chan error) {
	return w.DefaultStream(ctx, target, w.Run)
}

// Initialize verifies that waybackurls is installed and accessible.
// Implements ports.AdvancedSource.
func (w *WaybackurlsSource) Initialize() error {
	return w.DefaultInitialize(
		"waybackurls",
		"go install github.com/tomnomnom/waybackurls@latest",
	)
}

// Validate checks if the source configuration is valid.
// Implements ports.AdvancedSource.
func (w *WaybackurlsSource) Validate() error {
	return w.DefaultValidate()
}

// HealthCheck verifies that waybackurls is responsive.
// Implements ports.AdvancedSource.
func (w *WaybackurlsSource) HealthCheck(ctx context.Context) error {
	return w.DefaultHealthCheck(ctx)
}

// buildCommandArgs constructs the waybackurls command arguments.
func (w *WaybackurlsSource) buildCommandArgs() []string {
	args := []string{}

	// Add optional flags
	if w.withDates {
		args = append(args, "-dates")
	}

	if w.noSubs {
		args = append(args, "-no-subs")
	}

	w.GetLogger().Debug("built waybackurls command",
		"args", args,
		"timeout", w.GetTimeout().String(),
	)

	return args
}
