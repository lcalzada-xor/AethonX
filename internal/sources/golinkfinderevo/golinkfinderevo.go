// Package golinkfinderevo implements integration with GoLinkfinderEVO CLI tool.
// GoLinkfinderEVO discovers endpoints, API routes, and sensitive data from JavaScript and HTML files.
package golinkfinderevo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/sources/common"
)

const (
	sourceName            = "golinkfinderevo"
	defaultTimeout        = 90 * time.Second
	defaultMaxScriptFiles = 50
	defaultMaxHTMLFiles   = 50
)

// GoLinkfinderResponse represents the JSON output structure from golinkfinder binary.
type GoLinkfinderResponse struct {
	Meta struct {
		GeneratedAt    string `json:"GeneratedAt"`
		TotalResources int    `json:"TotalResources"`
		TotalEndpoints int    `json:"TotalEndpoints"`
	} `json:"meta"`
	Resources []struct {
		Resource  string `json:"Resource"`
		Endpoints []struct {
			Link    string `json:"Link"`
			Context string `json:"Context"`
			Line    int    `json:"Line"`
		} `json:"Endpoints"`
	} `json:"resources"`
}

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

// Compile-time interface assertions
var (
	_ ports.Source        = (*GoLinkfinderEvoSource)(nil)
	_ ports.InputConsumer = (*GoLinkfinderEvoSource)(nil)
)

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

	// Validate input
	if input == nil {
		g.GetLogger().Warn("nil input provided to RunWithInput")
		result := domain.NewScanResult(target)
		result.AddWarning(sourceName, "nil input provided")
		return result, nil
	}

	if input.Artifacts == nil {
		g.GetLogger().Warn("nil artifacts array in input")
		result := domain.NewScanResult(target)
		result.AddWarning(sourceName, "nil artifacts array in input")
		return result, nil
	}

	// Debug: log artifact types received
	artifactTypes := make(map[domain.ArtifactType]int)
	for _, a := range input.Artifacts {
		artifactTypes[a.Type]++
	}

	g.GetLogger().Info("starting golinkfinderevo scan",
		"target", target.Root,
		"profile", g.profile,
		"input_artifacts", len(input.Artifacts),
		"artifact_types", artifactTypes,
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

	// Execute golinkfinderevo with temp files (golinkfinder doesn't support stdin)
	result, err := g.executeWithTempFiles(ctx, target, processURLs)

	if result == nil {
		return nil, fmt.Errorf("golinkfinderevo failed to start: %w", err)
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

		// Extract content-type for logging/debugging
		contentType := g.extractContentType(a.Tags)

		candidates = append(candidates, URLCandidate{
			URL:         a.Value,
			Category:    category,
			StatusCode:  statusCode,
			ContentType: contentType,
		})
	}

	g.GetLogger().Debug("filtered URL candidates",
		"total_input", len(artifacts),
		"candidates", len(candidates),
	)

	return candidates
}

// categorizeURL categorizes a URL based on content-type and file extension.
// JavaScript: Scripts with .js/.mjs/.jsx extension OR application/javascript content-type
// HTML: ANY endpoint that returns text/html content-type (regardless of extension)
func (g *GoLinkfinderEvoSource) categorizeURL(artifact *domain.Artifact) URLCategory {
	urlLower := strings.ToLower(artifact.Value)

	// Extract content-type from tags (format: "content-type:text/html; charset=utf-8")
	contentType := g.extractContentType(artifact.Tags)
	contentTypeLower := strings.ToLower(contentType)

	// Priority 1: JavaScript detection
	// Check content-type for JavaScript MIME types
	if strings.Contains(contentTypeLower, "application/javascript") ||
		strings.Contains(contentTypeLower, "application/x-javascript") ||
		strings.Contains(contentTypeLower, "text/javascript") ||
		strings.Contains(contentTypeLower, "application/ecmascript") {
		return URLCategoryJavaScript
	}

	// Check file extension for JavaScript files (.js, .mjs, .jsx)
	if strings.HasSuffix(urlLower, ".js") ||
		strings.HasSuffix(urlLower, ".mjs") ||
		strings.HasSuffix(urlLower, ".jsx") {
		return URLCategoryJavaScript
	}

	// Priority 2: HTML detection
	// ANY endpoint that returns text/html is categorized as HTML
	// This includes /api/users, /login, /dashboard, etc. - not just .html files
	if strings.Contains(contentTypeLower, "text/html") {
		return URLCategoryHTML
	}

	// Priority 3: Exclude known non-HTML/non-JS resources
	// Images, videos, documents, data files, etc.
	skipExtensions := []string{
		".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp", ".bmp", // Images
		".css", ".woff", ".woff2", ".ttf", ".eot", ".otf", // Styles/Fonts
		".pdf", ".zip", ".tar", ".gz", ".rar", ".7z", // Documents/Archives
		".mp4", ".avi", ".mov", ".webm", ".mkv", // Videos
		".mp3", ".wav", ".ogg", ".flac", // Audio
		".xml", ".json", ".txt", ".csv", ".yml", ".yaml", // Data files
	}

	for _, ext := range skipExtensions {
		if strings.HasSuffix(urlLower, ext) {
			return URLCategoryOther
		}
	}

	// Exclude known non-HTML content-types
	excludeContentTypes := []string{
		"application/json",
		"application/xml",
		"text/plain",
		"text/css",
		"image/",
		"video/",
		"audio/",
		"application/pdf",
		"application/zip",
	}

	for _, excludeType := range excludeContentTypes {
		if strings.Contains(contentTypeLower, excludeType) {
			return URLCategoryOther
		}
	}

	// Default: Unknown content-type, skip to avoid false positives
	// We only want explicit JavaScript files or HTML content
	return URLCategoryOther
}

// extractContentType extracts the content-type value from artifact tags.
// Tags format: "content-type:text/html; charset=utf-8"
func (g *GoLinkfinderEvoSource) extractContentType(tags []string) string {
	for _, tag := range tags {
		if strings.HasPrefix(tag, "content-type:") {
			// Extract everything after "content-type:"
			contentType := strings.TrimPrefix(tag, "content-type:")
			// Remove charset and other parameters (e.g., "text/html; charset=utf-8" -> "text/html")
			if idx := strings.Index(contentType, ";"); idx != -1 {
				contentType = contentType[:idx]
			}
			return strings.TrimSpace(contentType)
		}
	}
	return ""
}

// applyURLLimits applies max file limits to URL candidates.
func (g *GoLinkfinderEvoSource) applyURLLimits(candidates []URLCandidate) []string {
	jsCount := 0
	htmlCount := 0
	urls := make([]string, 0)

	for _, c := range candidates {
		// Add JavaScript files up to limit
		if c.Category == URLCategoryJavaScript {
			if jsCount < g.maxScriptFiles {
				urls = append(urls, c.URL)
				jsCount++
			}
			// Skip if limit reached, but continue processing other categories
			continue
		}

		// Add HTML files up to limit
		if c.Category == URLCategoryHTML {
			if htmlCount < g.maxHTMLFiles {
				urls = append(urls, c.URL)
				htmlCount++
			}
			// Skip if limit reached, but continue processing
			continue
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
// Note: golinkfinder uses single-dash flags (e.g., -input, -output, -workers)
func (g *GoLinkfinderEvoSource) buildCommandArgs(target domain.Target, inputFile, outputFile string) []string {
	profileCfg := GetProfile(g.profile)

	args := []string{
		"-input", inputFile, // Input URLs file
		"-output", fmt.Sprintf("json=%s", outputFile), // JSON output file
		"-workers", strconv.Itoa(g.workers),
		"-timeout", profileCfg.Timeout.String(), // e.g., "60s"
		"-scope", target.Root,                   // Restrict to target domain
		"-scope-include-subdomains",
	}

	// Recursion depth
	if profileCfg.MaxRecursion > 0 {
		args = append(args, "-recursive", strconv.Itoa(profileCfg.MaxRecursion))
	}

	// GF integration - temporarily disabled due to incompatibility
	// TODO: Fix GF integration - golinkfinder may not support these flags or template format
	// if len(g.gfPatterns) > 0 && g.gfPatterns[0] != "" {
	// 	args = append(args,
	// 		"-gf", strings.Join(g.gfPatterns, ","),
	// 		"-gf-path", g.gfTemplatesPath,
	// 	)
	// } else if len(profileCfg.GFPatterns) > 0 {
	// 	args = append(args,
	// 		"-gf", strings.Join(profileCfg.GFPatterns, ","),
	// 		"-gf-path", g.gfTemplatesPath,
	// 	)
	// }

	// JavaScript rendering (for ProfileDeep)
	if profileCfg.EnableJSRendering {
		args = append(args, "-render")
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

// executeWithTempFiles executes golinkfinderevo using temporary input/output files.
// golinkfinder doesn't support stdin, so we write URLs to a temp file and read JSON output from another temp file.
func (g *GoLinkfinderEvoSource) executeWithTempFiles(
	ctx context.Context,
	target domain.Target,
	urls []string,
) (*domain.ScanResult, error) {
	result := domain.NewScanResult(target)

	// Apply timeout to context if not already set
	execCtx := ctx
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > defaultTimeout {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
	}

	// Create temporary input file with URLs
	inputFile, err := os.CreateTemp("", "golinkfinder-input-*.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp input file: %w", err)
	}
	defer os.Remove(inputFile.Name())

	// Write URLs to input file
	for _, url := range urls {
		if _, err := fmt.Fprintln(inputFile, url); err != nil {
			return nil, fmt.Errorf("failed to write URL to input file: %w", err)
		}
	}

	// Close input file before executing command
	if err := inputFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close input file: %w", err)
	}

	// Create temporary output file
	outputFile, err := os.CreateTemp("", "golinkfinder-output-*.json")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp output file: %w", err)
	}
	outputFileName := outputFile.Name()
	outputFile.Close()
	defer os.Remove(outputFileName)

	// Build command arguments
	args := g.buildCommandArgs(target, inputFile.Name(), outputFileName)

	// Build command with timeout-enforced context
	cmd := exec.CommandContext(execCtx, g.GetExecPath(), args...)

	// Capture stderr for debugging
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start golinkfinderevo: %w", err)
	}

	g.GetLogger().Debug("golinkfinderevo process started", "pid", cmd.Process.Pid)

	// Read stderr in background
	stderrChan := make(chan string, 1)
	go func() {
		stderrBytes, err := io.ReadAll(stderrPipe)
		if err != nil {
			g.GetLogger().Debug("failed to read stderr", "error", err)
		}
		stderrChan <- string(stderrBytes)
	}()

	// Wait for process to complete
	waitErr := cmd.Wait()

	// Get stderr output
	stderrStr := <-stderrChan
	if len(stderrStr) > 0 {
		g.GetLogger().Debug("golinkfinderevo stderr", "output", stderrStr)
	}

	// Handle process error
	if waitErr != nil {
		g.GetLogger().Warn("golinkfinderevo process failed",
			"error", waitErr.Error(),
			"stderr", stderrStr,
			"exit_code", cmd.ProcessState.ExitCode(),
		)
		return nil, fmt.Errorf("golinkfinderevo failed: %w (stderr: %s)", waitErr, stderrStr)
	}

	// Read and parse JSON output file
	jsonData, err := os.ReadFile(outputFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read output file: %w", err)
	}

	// Parse JSON response
	var response GoLinkfinderResponse
	if err := json.Unmarshal(jsonData, &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w", err)
	}

	// Convert resources to reports
	reports := make([]*ResourceReport, 0, len(response.Resources))
	for _, resource := range response.Resources {
		// Extract endpoint URLs from the structured endpoints
		endpoints := make([]string, 0, len(resource.Endpoints))
		for _, ep := range resource.Endpoints {
			endpoints = append(endpoints, ep.Link)
		}

		report := &ResourceReport{
			URL:       resource.Resource,
			Endpoints: endpoints,
		}
		reports = append(reports, report)
	}

	// Parse GF results if they exist
	gfFindings, gfErr := g.parseGFResults()
	if gfErr != nil {
		g.GetLogger().Debug("no gf results found", "error", gfErr.Error())
		gfFindings = make(GFResults) // Empty results
	}

	// Convert reports and GF findings to artifacts
	artifacts := g.convertToArtifacts(reports, gfFindings, target)
	for _, artifact := range artifacts {
		result.AddArtifact(artifact)
	}

	// Store metadata
	if result.Metadata.Environment == nil {
		result.Metadata.Environment = make(map[string]string)
	}
	result.Metadata.Environment["golinkfinderevo_urls_processed"] = strconv.Itoa(len(urls))
	result.Metadata.Environment["golinkfinderevo_profile"] = string(g.profile)
	result.Metadata.Environment["golinkfinderevo_resources"] = strconv.Itoa(response.Meta.TotalResources)
	result.Metadata.Environment["golinkfinderevo_endpoints"] = strconv.Itoa(response.Meta.TotalEndpoints)
	result.Metadata.Environment["golinkfinderevo_gf_patterns"] = strconv.Itoa(len(gfFindings))

	g.GetLogger().Info("golinkfinderevo execution successful",
		"resources", response.Meta.TotalResources,
		"endpoints", response.Meta.TotalEndpoints,
		"gf_findings", len(gfFindings),
		"artifacts", len(result.Artifacts),
	)

	return result, nil
}

// Close releases resources.
func (g *GoLinkfinderEvoSource) Close() error {
	return g.BaseCLISource.Close()
}
