// Package golinkfinderevo implements integration with GoLinkfinderEVO CLI tool.
// GoLinkfinderEVO discovers endpoints, API routes, and sensitive data from JavaScript and HTML files.
package golinkfinderevo

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/platform/logx"
	"aethonx/internal/sources/common"
)

const (
	sourceName            = "golinkfinderevo"
	defaultTimeout        = 90 * time.Second
	defaultMaxScriptFiles = 50
	defaultMaxHTMLFiles   = 50
)

// URLCategory categorizes URLs by content type.
type URLCategory string

const (
	URLCategoryJavaScript URLCategory = "javascript"
	URLCategoryHTML       URLCategory = "html"
	URLCategoryOther      URLCategory = "other"
)

// URLCandidate represents a URL candidate for crawling.
type URLCandidate struct {
	URL         string
	Category    URLCategory
	StatusCode  int
	ContentType string
}

// GoLinkfinderEvoSource implements ports.Source and ports.InputConsumer.
// It wraps GoLinkfinderEVO CLI tool for endpoint discovery and secret extraction.
type GoLinkfinderEvoSource struct {
	*common.BaseCLISource

	profile         ScanProfile
	workers         int
	maxScriptFiles  int
	maxHTMLFiles    int
	gfTemplatesPath string
	gfPatterns      []string
	parser          *Parser
	gfParser        *GFParser
	customFlags     []string
}

// New creates a new GoLinkfinderEvoSource with default configuration.
func New(logger logx.Logger) *GoLinkfinderEvoSource {
	return &GoLinkfinderEvoSource{
		BaseCLISource: common.NewBaseCLISource(logger, common.BaseCLIConfig{
			SourceName:     sourceName,
			ExecPath:       "golinkfinder",
			Timeout:        defaultTimeout,
			ProgressBuffer: 10,
		}),
		profile:         ProfileStandard,
		workers:         Profiles[ProfileStandard].Workers,
		maxScriptFiles:  defaultMaxScriptFiles,
		maxHTMLFiles:    defaultMaxHTMLFiles,
		gfTemplatesPath: "./internal/platform/gf_templates",
		gfPatterns:      []string{"all"},
		parser:          NewParser(logger, sourceName),
		gfParser:        NewGFParser(logger),
		customFlags:     []string{},
	}
}

// NewWithConfig creates GoLinkfinderEvoSource with custom configuration.
func NewWithConfig(
	logger logx.Logger,
	execPath string,
	profile ScanProfile,
	timeout time.Duration,
	workers int,
	maxScriptFiles int,
	maxHTMLFiles int,
	gfTemplatesPath string,
	gfPatterns []string,
) *GoLinkfinderEvoSource {
	return &GoLinkfinderEvoSource{
		BaseCLISource: common.NewBaseCLISource(logger, common.BaseCLIConfig{
			SourceName:     sourceName,
			ExecPath:       execPath,
			Timeout:        timeout,
			ProgressBuffer: 10,
		}),
		profile:         profile,
		workers:         workers,
		maxScriptFiles:  maxScriptFiles,
		maxHTMLFiles:    maxHTMLFiles,
		gfTemplatesPath: gfTemplatesPath,
		gfPatterns:      gfPatterns,
		parser:          NewParser(logger, sourceName),
		gfParser:        NewGFParser(logger),
		customFlags:     []string{},
	}
}

// Name returns the source name.
func (g *GoLinkfinderEvoSource) Name() string {
	return sourceName
}

// Mode returns the source operation mode (active).
func (g *GoLinkfinderEvoSource) Mode() domain.SourceMode {
	return domain.SourceModeActive
}

// Type returns the source type (CLI).
func (g *GoLinkfinderEvoSource) Type() domain.SourceType {
	return domain.SourceTypeCLI
}

// Run executes golinkfinderevo (fallback for non-InputConsumer usage).
func (g *GoLinkfinderEvoSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	// Create empty input for compatibility
	emptyInput := domain.NewScanResult(target)
	return g.RunWithInput(ctx, target, emptyInput)
}

// RunWithInput executes golinkfinderevo with filtered URLs from previous stages.
// Implements ports.InputConsumer interface.
func (g *GoLinkfinderEvoSource) RunWithInput(
	ctx context.Context,
	target domain.Target,
	input *domain.ScanResult,
) (*domain.ScanResult, error) {
	startTime := time.Now()

	g.GetLogger().Info("starting golinkfinderevo scan",
		"target", target.Root,
		"profile", g.profile,
		"input_artifacts", len(input.Artifacts),
	)

	// Filter input URLs: only alive HTTP URLs with JS/HTML content
	filteredURLs := g.filterURLsForCrawling(input.Artifacts)

	if len(filteredURLs) == 0 {
		g.GetLogger().Info("no suitable URLs found for crawling")
		result := domain.NewScanResult(target)
		result.AddWarning(sourceName, "no HTTP URLs with JavaScript or HTML content available")
		return result, nil
	}

	// Apply limits (max 50 JS + 50 HTML by default)
	processURLs := g.applyURLLimits(filteredURLs)

	g.GetLogger().Info("filtered URLs for crawling",
		"total_candidates", len(filteredURLs),
		"processing", len(processURLs),
		"max_script_files", g.maxScriptFiles,
		"max_html_files", g.maxHTMLFiles,
	)

	// Build command arguments
	args := g.buildCommandArgs(target, processURLs)

	// Execute golinkfinderevo with stdin input (similar to httpx pattern)
	result, stderrOutput, err := g.executeWithStdin(ctx, target, processURLs, args)

	if result == nil {
		return nil, fmt.Errorf("golinkfinderevo failed to start: %w", err)
	}

	// Handle stderr warnings
	if len(stderrOutput) > 0 {
		g.GetLogger().Debug("golinkfinderevo stderr", "output", stderrOutput)
		result.AddWarning(sourceName, fmt.Sprintf("stderr: %s", stderrOutput))
	}

	duration := time.Since(startTime)
	g.GetLogger().Info("golinkfinderevo completed",
		"artifacts", len(result.Artifacts),
		"duration", duration,
	)

	return result, err
}

// filterURLsForCrawling filters URLs suitable for golinkfinderevo crawling.
func (g *GoLinkfinderEvoSource) filterURLsForCrawling(artifacts []*domain.Artifact) []URLCandidate {
	candidates := make([]URLCandidate, 0)

	for _, a := range artifacts {
		// Only process URL artifacts
		if a.Type != domain.ArtifactTypeURL {
			continue
		}

		// Categorize by URL extension and metadata
		category := g.categorizeURL(a)

		// Only process JS and HTML
		if category == URLCategoryOther {
			continue
		}

		// Extract status code from metadata - REQUIRED
		// Skip URLs without proper metadata from httpx
		if a.TypedMetadata == nil {
			continue
		}

		domainMeta, ok := a.TypedMetadata.(*metadata.DomainMetadata)
		if !ok {
			continue
		}

		statusCode := domainMeta.HTTPStatus

		// Filter: only successful HTTP responses (200-399)
		if statusCode < 200 || statusCode >= 400 {
			continue
		}

		candidates = append(candidates, URLCandidate{
			URL:         a.Value,
			Category:    category,
			StatusCode:  statusCode,
			ContentType: "", // Content-type detection via extension/tags
		})
	}

	g.GetLogger().Debug("filtered URL candidates",
		"total_input", len(artifacts),
		"candidates", len(candidates),
	)

	return candidates
}

// categorizeURL categorizes a URL by file extension and metadata.
func (g *GoLinkfinderEvoSource) categorizeURL(artifact *domain.Artifact) URLCategory {
	urlLower := strings.ToLower(artifact.Value)

	// Check by file extension first - explicit JS files
	if strings.HasSuffix(urlLower, ".js") ||
		strings.HasSuffix(urlLower, ".mjs") ||
		strings.HasSuffix(urlLower, ".jsx") {
		return URLCategoryJavaScript
	}

	// Explicit HTML files
	if strings.HasSuffix(urlLower, ".html") ||
		strings.HasSuffix(urlLower, ".htm") {
		return URLCategoryHTML
	}

	// Skip non-HTML resource types by extension
	// (images, videos, documents, etc.)
	skipExtensions := []string{
		".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp", // Images
		".css", ".woff", ".woff2", ".ttf", ".eot", // Styles/Fonts
		".pdf", ".zip", ".tar", ".gz", ".rar", // Documents/Archives
		".mp4", ".avi", ".mov", ".webm", // Videos
		".mp3", ".wav", ".ogg", // Audio
		".xml", ".json", ".txt", ".csv", // Data files (not HTML)
	}
	for _, ext := range skipExtensions {
		if strings.HasSuffix(urlLower, ext) {
			return URLCategoryOther
		}
	}

	// Check tags for content-type hints
	hasHTMLContentType := false
	for _, tag := range artifact.Tags {
		tagLower := strings.ToLower(tag)

		// Exclude JSON/XML/plain text content types
		if strings.Contains(tagLower, "application/json") ||
			strings.Contains(tagLower, "application/xml") ||
			strings.Contains(tagLower, "text/plain") ||
			strings.Contains(tagLower, "text/css") {
			return URLCategoryOther
		}

		// Match "javascript" or "ecmascript" but not "json"
		if strings.Contains(tagLower, "javascript") || strings.Contains(tagLower, "ecmascript") {
			return URLCategoryJavaScript
		}
		// Match "text/javascript" explicitly
		if strings.Contains(tagLower, "text/javascript") {
			return URLCategoryJavaScript
		}
		// Match HTML content type
		if strings.Contains(tagLower, "text/html") {
			hasHTMLContentType = true
		}
	}

	// If explicit HTML content-type, categorize as HTML
	if hasHTMLContentType {
		return URLCategoryHTML
	}

	// Default: if it's an alive URL from httpx without a non-HTML extension,
	// assume it's HTML (most web pages don't have .html extension)
	// GoLinkfinderEVO will handle it and extract any embedded JS
	return URLCategoryHTML
}

// applyURLLimits applies max file limits to URL candidates.
func (g *GoLinkfinderEvoSource) applyURLLimits(candidates []URLCandidate) []string {
	jsCount := 0
	htmlCount := 0
	urls := make([]string, 0)

	for _, c := range candidates {
		if c.Category == URLCategoryJavaScript && jsCount < g.maxScriptFiles {
			urls = append(urls, c.URL)
			jsCount++
		} else if c.Category == URLCategoryHTML && htmlCount < g.maxHTMLFiles {
			urls = append(urls, c.URL)
			htmlCount++
		}

		// Stop if both limits reached
		if jsCount >= g.maxScriptFiles && htmlCount >= g.maxHTMLFiles {
			break
		}
	}

	g.GetLogger().Debug("applied URL limits",
		"js_files", jsCount,
		"html_files", htmlCount,
		"total", len(urls),
	)

	return urls
}

// buildCommandArgs builds golinkfinderevo command arguments.
func (g *GoLinkfinderEvoSource) buildCommandArgs(target domain.Target, urls []string) []string {
	profileCfg := GetProfile(g.profile)

	args := []string{
		"-i", "-", // Read from stdin
		"--json", // JSON output
		"--workers", strconv.Itoa(g.workers),
		"--timeout", fmt.Sprintf("%ds", int(profileCfg.Timeout.Seconds())),
		"--scope", target.Root, // Restrict to target domain
		"--scope-include-subdomains",
	}

	// Recursion depth
	if profileCfg.MaxRecursion > 0 {
		args = append(args, "--recursion-depth", strconv.Itoa(profileCfg.MaxRecursion))
	}

	// GF integration
	if len(g.gfPatterns) > 0 && g.gfPatterns[0] != "" {
		args = append(args,
			"--gf", strings.Join(g.gfPatterns, ","),
			"--gf-path", g.gfTemplatesPath,
		)
	} else if len(profileCfg.GFPatterns) > 0 {
		args = append(args,
			"--gf", strings.Join(profileCfg.GFPatterns, ","),
			"--gf-path", g.gfTemplatesPath,
		)
	}

	// JavaScript rendering (for ProfileDeep)
	if profileCfg.EnableJSRendering {
		args = append(args, "--render")
	}

	// Custom flags
	if len(g.customFlags) > 0 {
		args = append(args, g.customFlags...)
	}

	g.GetLogger().Debug("built command args", "args", strings.Join(args, " "))

	return args
}

// parseGFResults parses gf.json results if they exist.
func (g *GoLinkfinderEvoSource) parseGFResults() (GFResults, error) {
	// GF writes results to gf.json in the working directory
	gfJSONPath := filepath.Join(".", "gf.json")

	findings, err := g.gfParser.ParseGFJSON(gfJSONPath)
	if err != nil {
		return nil, err
	}

	return findings, nil
}

// convertToArtifacts converts reports and GF findings to domain artifacts.
func (g *GoLinkfinderEvoSource) convertToArtifacts(
	reports []*ResourceReport,
	gfFindings GFResults,
	target domain.Target,
) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0)

	// Convert endpoint reports
	reportArtifacts := g.parser.ConvertMultipleReports(reports, target)
	artifacts = append(artifacts, reportArtifacts...)

	// Extract parameters from endpoints
	for _, artifact := range reportArtifacts {
		if artifact.Type == domain.ArtifactTypeEndpoint {
			params := g.parser.ExtractParametersFromEndpoint(artifact.Value, target)
			artifacts = append(artifacts, params...)
		}
	}

	// Convert GF findings
	if len(gfFindings) > 0 {
		gfArtifacts := g.gfParser.ConvertToArtifacts(gfFindings, target)
		artifacts = append(artifacts, gfArtifacts...)
	}

	g.GetLogger().Debug("converted to artifacts",
		"reports", len(reports),
		"gf_patterns", len(gfFindings),
		"total_artifacts", len(artifacts),
	)

	return artifacts
}

// SetCustomFlags sets custom command-line flags.
func (g *GoLinkfinderEvoSource) SetCustomFlags(flags []string) {
	g.customFlags = flags
}

// executeWithStdin executes golinkfinderevo with stdin input (follows httpx pattern).
func (g *GoLinkfinderEvoSource) executeWithStdin(
	ctx context.Context,
	target domain.Target,
	urls []string,
	args []string,
) (*domain.ScanResult, string, error) {
	result := domain.NewScanResult(target)

	// Create handler for processing output
	handler := &GoLinkfinderEvoHandler{
		parser:     g.parser,
		gfParser:   g.gfParser,
		target:     target,
		logger:     g.GetLogger(),
		reports:    make([]*ResourceReport, 0),
		gfFindings: make(GFResults),
	}

	// Build command with context
	cmd := exec.CommandContext(ctx, g.GetExecPath(), args...)

	// Create stdout pipe for streaming JSON
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Create stderr pipe for warnings
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Create stdin pipe to send URLs
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, "", fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Start golinkfinderevo process
	if err := cmd.Start(); err != nil {
		return nil, "", fmt.Errorf("failed to start golinkfinderevo: %w", err)
	}

	g.GetLogger().Debug("golinkfinderevo process started", "pid", cmd.Process.Pid)

	// Write URLs to stdin in goroutine
	go func() {
		defer stdin.Close()
		for _, url := range urls {
			fmt.Fprintln(stdin, url)
		}
		g.GetLogger().Debug("wrote URLs to stdin", "count", len(urls))
	}()

	// Process stdout line-by-line using ProcessOutput
	if err := g.ProcessOutput(stdout, handler); err != nil {
		g.GetLogger().Warn("output processing error", "error", err.Error())
	}

	// Capture stderr for warnings
	stderrBytes, _ := io.ReadAll(stderr)
	stderrStr := string(stderrBytes)

	// Wait for process to complete
	waitErr := cmd.Wait()

	// Finalize handler (always, even with error)
	if err := handler.Finalize(); err != nil {
		g.GetLogger().Warn("handler finalization error", "error", err.Error())
	}

	// Parse GF results if they exist
	gfFindings, gfErr := g.parseGFResults()
	if gfErr != nil {
		g.GetLogger().Warn("failed to parse gf results", "error", gfErr.Error())
	}

	// Convert reports and GF findings to artifacts (always, even with error)
	artifacts := g.convertToArtifacts(handler.GetReports(), gfFindings, target)
	for _, artifact := range artifacts {
		result.AddArtifact(artifact)
	}

	// Store metadata
	if result.Metadata.Environment == nil {
		result.Metadata.Environment = make(map[string]string)
	}
	result.Metadata.Environment["golinkfinderevo_urls_processed"] = strconv.Itoa(len(urls))
	result.Metadata.Environment["golinkfinderevo_profile"] = string(g.profile)
	result.Metadata.Environment["golinkfinderevo_reports"] = strconv.Itoa(len(handler.GetReports()))
	result.Metadata.Environment["golinkfinderevo_gf_patterns"] = strconv.Itoa(len(gfFindings))

	artifactCount := len(result.Artifacts)

	// Handle errors (partial results tolerated and returned)
	if waitErr != nil {
		if artifactCount > 0 {
			g.GetLogger().Warn("golinkfinderevo exited with error but produced partial results",
				"error", waitErr.Error(),
				"artifacts", artifactCount,
			)
			result.AddWarning(sourceName, fmt.Sprintf("process exited with error: %v", waitErr))
			// Return partial results + error
			return result, stderrStr, fmt.Errorf("golinkfinderevo timeout/cancelled but produced %d partial results: %w", artifactCount, waitErr)
		}

		// No results: return error
		return nil, stderrStr, fmt.Errorf("golinkfinderevo failed without results: %w", waitErr)
	}

	g.GetLogger().Info("golinkfinderevo execution successful",
		"reports", len(handler.GetReports()),
		"gf_patterns", len(gfFindings),
		"artifacts", artifactCount,
	)

	return result, stderrStr, nil
}

// Close releases resources.
func (g *GoLinkfinderEvoSource) Close() error {
	return g.BaseCLISource.Close()
}
