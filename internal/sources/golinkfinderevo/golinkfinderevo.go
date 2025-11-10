// Package golinkfinderevo implements integration with GoLinkfinderEVO CLI tool.
// GoLinkfinderEVO discovers endpoints, API routes, and sensitive data from JavaScript and HTML files.
package golinkfinderevo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
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

// GoLinkfinderJSONOutput represents the JSON output from golinkfinder -o json
type GoLinkfinderJSONOutput struct {
	Resources  []ResourceOutput `json:"resources"`
	GFFindings GFFindings       `json:"gf_findings"`
}

// ResourceOutput represents a single resource with its discovered endpoints
type ResourceOutput struct {
	Resource  string           `json:"Resource"`
	Endpoints []EndpointOutput `json:"Endpoints"`
}

// EndpointOutput represents a discovered endpoint with context
type EndpointOutput struct {
	Link    string `json:"Link"`
	Context string `json:"Context"`
	Line    int    `json:"Line"`
}

// GFFindings represents the gf pattern matching results
type GFFindings struct {
	Rules    []string          `json:"rules"`    // List of rules that were executed
	Total    int               `json:"total"`    // Total number of findings
	Findings []GFFindingDetail `json:"findings"` // All findings with their rules
}

// GFFindingDetail represents a single GF pattern match
type GFFindingDetail struct {
	Resource string   `json:"resource"` // URL where the finding was discovered
	Line     int      `json:"line"`     // Line number in the resource
	Evidence string   `json:"evidence"` // The matched text/evidence
	Context  string   `json:"context"`  // Surrounding context
	Rules    []string `json:"rules"`    // Rules that matched this finding
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

	// Debug: log artifact types received and sample URLs with metadata
	artifactTypes := make(map[domain.ArtifactType]int)
	urlArtifactCount := 0
	sampleURLs := make([]string, 0, 5)
	for _, a := range input.Artifacts {
		artifactTypes[a.Type]++
		if a.Type == domain.ArtifactTypeURL {
			urlArtifactCount++
			if len(sampleURLs) < 5 {
				metaType := "nil"
				hasTags := len(a.Tags) > 0
				if a.TypedMetadata != nil {
					metaType = fmt.Sprintf("%T", a.TypedMetadata)
				}
				sampleURLs = append(sampleURLs, fmt.Sprintf("%s (meta:%s, tags:%v)", a.Value, metaType, hasTags))
			}
		}
	}

	g.GetLogger().Info("starting golinkfinderevo scan",
		"target", target.Root,
		"profile", g.profile,
		"input_artifacts", len(input.Artifacts),
		"url_artifacts", urlArtifactCount,
		"artifact_types", artifactTypes,
	)

	if len(sampleURLs) > 0 {
		g.GetLogger().Debug("sample URL artifacts received",
			"samples", sampleURLs,
		)
	}

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

	// Check if we have any URLs to process after applying limits
	if len(processURLs) == 0 {
		g.GetLogger().Info("no URLs to process after applying limits")
		result := domain.NewScanResult(target)
		result.AddWarning(sourceName, "no URLs with JavaScript or HTML content available after filtering and limits")
		return result, nil
	}

	// Execute golinkfinderevo with stdin/stdout
	result, err := g.executeWithStdin(ctx, target, processURLs)

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

	// Debug counters
	urlCount := 0
	noMetadataCount := 0
	wrongMetadataTypeCount := 0
	badStatusCount := 0
	categoryOtherCount := 0

	for _, a := range artifacts {
		// Process URL and JavaScript artifacts
		if a.Type != domain.ArtifactTypeURL && a.Type != domain.ArtifactTypeJavaScript {
			continue
		}
		urlCount++

		// Categorize by URL extension and metadata
		category := g.categorizeURL(a)

		// Debug: Log first few URLs being processed WITH their category
		if urlCount <= 5 {
			contentType := g.extractContentType(a.Tags)
			g.GetLogger().Debug("processing URL artifact",
				"url", a.Value,
				"type", a.Type,
				"category", category,
				"content_type", contentType,
				"has_metadata", a.TypedMetadata != nil,
				"metadata_type", fmt.Sprintf("%T", a.TypedMetadata),
			)
		}

		// Only process JS and HTML
		if category == URLCategoryOther {
			categoryOtherCount++
			continue
		}

		// For JavaScript artifacts from waybackurls, accept them without metadata check
		// They are already categorized as JavaScript by their extension/URL
		if a.Type == domain.ArtifactTypeJavaScript {
			// Validate URL before accepting
			if a.Value == "" {
				continue
			}

			// Accept JavaScript artifacts directly
			contentType := g.extractContentType(a.Tags)

			// Set a reasonable default status code for historical URLs (200 assumed for waybackurls)
			statusCode := 200 // Assume alive if from waybackurls
			if domainMeta, ok := a.TypedMetadata.(*metadata.DomainMetadata); ok {
				// Override with actual status if metadata exists
				statusCode = domainMeta.HTTPStatus
			}

			candidates = append(candidates, URLCandidate{
				URL:         a.Value,
				Category:    URLCategoryJavaScript,
				StatusCode:  statusCode,
				ContentType: contentType,
			})
			continue
		}

		// For URL artifacts, check metadata and status code
		// Extract status code from metadata - REQUIRED
		// Skip URLs without proper metadata from httpx
		if a.TypedMetadata == nil {
			noMetadataCount++
			continue
		}

		domainMeta, ok := a.TypedMetadata.(*metadata.DomainMetadata)
		if !ok {
			wrongMetadataTypeCount++
			continue
		}

		statusCode := domainMeta.HTTPStatus

		// Filter: only successful HTTP responses (200-399)
		if statusCode < 200 || statusCode >= 400 {
			badStatusCount++
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

	// ALWAYS log filtering results for debugging
	g.GetLogger().Info("filtered URLs for golinkfinderevo",
		"total_urls", urlCount,
		"candidates", len(candidates),
		"rejected_category", categoryOtherCount,
		"rejected_no_metadata", noMetadataCount,
		"rejected_wrong_metadata_type", wrongMetadataTypeCount,
		"rejected_bad_status", badStatusCount,
	)

	// Log sample of rejected URLs if filtering rejected everything
	if len(candidates) == 0 && urlCount > 0 {
		g.GetLogger().Warn("all URLs were filtered out - no candidates found",
			"total_input_urls", urlCount,
			"primary_rejection_reason", map[string]int{
				"category_mismatch":    categoryOtherCount,
				"no_metadata":          noMetadataCount,
				"wrong_metadata_type":  wrongMetadataTypeCount,
				"bad_status_code":      badStatusCount,
			},
		)
	}

	// Log first few rejected URLs for debugging with actual samples
	if noMetadataCount > 0 || wrongMetadataTypeCount > 0 || badStatusCount > 0 {
		// Re-scan to collect sample rejected URLs
		sampleRejected := make(map[string][]string)
		for _, a := range artifacts {
			if a.Type != domain.ArtifactTypeURL && a.Type != domain.ArtifactTypeJavaScript {
				continue
			}
			if len(sampleRejected) >= 10 { // Limit samples
				break
			}

			category := g.categorizeURL(a)
			if category == URLCategoryOther {
				if len(sampleRejected["category_other"]) < 3 {
					contentType := g.extractContentType(a.Tags)
					sampleRejected["category_other"] = append(sampleRejected["category_other"],
						fmt.Sprintf("%s (ct:%s)", a.Value, contentType))
				}
				continue
			}

			if a.Type == domain.ArtifactTypeJavaScript {
				continue // JS artifacts bypass metadata check
			}

			if a.TypedMetadata == nil {
				if len(sampleRejected["no_metadata"]) < 3 {
					sampleRejected["no_metadata"] = append(sampleRejected["no_metadata"], a.Value)
				}
				continue
			}

			domainMeta, ok := a.TypedMetadata.(*metadata.DomainMetadata)
			if !ok {
				if len(sampleRejected["wrong_metadata_type"]) < 3 {
					sampleRejected["wrong_metadata_type"] = append(sampleRejected["wrong_metadata_type"],
						fmt.Sprintf("%s (type:%T)", a.Value, a.TypedMetadata))
				}
				continue
			}

			if domainMeta.HTTPStatus < 200 || domainMeta.HTTPStatus >= 400 {
				if len(sampleRejected["bad_status"]) < 3 {
					sampleRejected["bad_status"] = append(sampleRejected["bad_status"],
						fmt.Sprintf("%s (status:%d)", a.Value, domainMeta.HTTPStatus))
				}
			}
		}

		g.GetLogger().Warn("URL filtering rejected many URLs",
			"no_metadata_count", noMetadataCount,
			"wrong_metadata_type_count", wrongMetadataTypeCount,
			"bad_status_count", badStatusCount,
			"category_other_count", categoryOtherCount,
			"sample_rejected", sampleRejected,
		)
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

// buildCommandArgsStdout builds golinkfinderevo command arguments for stdout-based execution.
// Uses -o json for stdout output and -gf for integrated pattern matching.
func (g *GoLinkfinderEvoSource) buildCommandArgsStdout(target domain.Target) []string {
	profileCfg := GetProfile(g.profile)

	args := []string{
		"-input", "-", // Read from stdin (use full flag name)
		"-output", "json", // JSON output to stdout
		"-workers", strconv.Itoa(g.workers),
		"-timeout", profileCfg.Timeout.String(),
		"-scope", target.Root,
	}

	// Add scope-include-subdomains as separate flag (it's boolean)
	args = append(args, "-scope-include-subdomains")

	// Recursion depth
	if profileCfg.MaxRecursion > 0 {
		args = append(args, "-recursive", strconv.Itoa(profileCfg.MaxRecursion))
	}

	// GF integration - ENABLED with absolute path
	if len(g.gfPatterns) > 0 && g.gfPatterns[0] != "" {
		gfPatternsStr := "all"
		if g.gfPatterns[0] != "all" {
			gfPatternsStr = strings.Join(g.gfPatterns, ",")
		}

		// Resolve absolute path for GF templates
		absGFPath, err := filepath.Abs(g.gfTemplatesPath)
		if err == nil {
			args = append(args,
				"-gf", gfPatternsStr,
				"-gf-path", absGFPath,
			)
			g.GetLogger().Debug("GF integration enabled",
				"patterns", gfPatternsStr,
				"path", absGFPath,
			)
		} else {
			g.GetLogger().Warn("failed to resolve GF templates path",
				"error", err,
				"path", g.gfTemplatesPath,
			)
		}
	}

	// JavaScript rendering (for ProfileDeep)
	if profileCfg.EnableJSRendering {
		args = append(args, "-render")
	}

	// Custom flags
	if len(g.customFlags) > 0 {
		args = append(args, g.customFlags...)
	}

	g.GetLogger().Debug("built command args for stdout", "args", strings.Join(args, " "))

	return args
}

// convertOutputToArtifacts converts GoLinkfinderJSONOutput to domain artifacts.
func (g *GoLinkfinderEvoSource) convertOutputToArtifacts(
	output GoLinkfinderJSONOutput,
	target domain.Target,
) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0)

	endpointCount := 0
	urlCount := 0
	paramCount := 0
	subdomainCount := 0

	// Convert resources to artifacts with intelligent classification
	for _, resource := range output.Resources {
		g.GetLogger().Debug("processing resource",
			"resource_url", resource.Resource,
			"endpoints_count", len(resource.Endpoints),
		)

		for _, ep := range resource.Endpoints {
			fullURL := g.parser.normalizeEndpoint(resource.Resource, ep.Link)
			if fullURL == "" {
				continue
			}

			// Classify artifact type intelligently (URL vs Endpoint)
			artifactType := g.classifyArtifact(fullURL, ep.Link, target)

			artifact := domain.NewArtifact(
				artifactType,
				fullURL,
				g.Name(),
			)

			// Set discovery context
			artifact.SetDiscoveryContext(&domain.DiscoveryContext{
				SourceURL:  resource.Resource,
				LineNumber: ep.Line,
				Context:    ep.Context,
			})

			// Set classification for static resources
			if g.isStaticResource(fullURL) {
				artifact.SetClassification(&domain.Classification{
					ResourceType: domain.ResourceTypeStatic,
				})
				artifact.Confidence = 0.5 // Lower confidence for static resources
			} else {
				artifact.Confidence = g.parser.calculateConfidence(ep.Link)
			}

			artifacts = append(artifacts, artifact)

			// Count by type
			if artifactType == domain.ArtifactTypeURL {
				urlCount++
			} else {
				endpointCount++
			}

			// Extract parameters from endpoint/URL
			params := g.parser.ExtractParametersFromEndpoint(fullURL, target)
			artifacts = append(artifacts, params...)
			paramCount += len(params)

			// Extract subdomains from external URLs
			subdomains := g.extractSubdomainsFromURL(fullURL, target)
			artifacts = append(artifacts, subdomains...)
			subdomainCount += len(subdomains)
		}
	}

	// Convert GF findings to artifacts
	gfArtifactCount := 0
	if output.GFFindings.Total > 0 {
		gfArtifacts := g.convertGFFindingsToArtifacts(output.GFFindings, target)
		artifacts = append(artifacts, gfArtifacts...)
		gfArtifactCount = len(gfArtifacts)
	}

	g.GetLogger().Info("converted output to artifacts",
		"resources", len(output.Resources),
		"endpoints", endpointCount,
		"urls", urlCount,
		"subdomains", subdomainCount,
		"parameters", paramCount,
		"gf_findings", output.GFFindings.Total,
		"gf_artifacts", gfArtifactCount,
		"total_artifacts", len(artifacts),
	)

	return artifacts
}

// classifyArtifact intelligently classifies a discovered link as URL or Endpoint.
// URLs: Complete absolute URLs (especially external domains or static resources)
// Endpoints: Relative paths or same-domain API routes
func (g *GoLinkfinderEvoSource) classifyArtifact(fullURL string, originalLink string, target domain.Target) domain.ArtifactType {
	// 1. Check if it's an absolute URL
	if !strings.HasPrefix(fullURL, "http://") && !strings.HasPrefix(fullURL, "https://") {
		// Relative path → Endpoint
		return domain.ArtifactTypeEndpoint
	}

	// 2. Parse URL to extract domain
	parsedURL, err := url.Parse(fullURL)
	if err != nil {
		// Invalid URL, default to Endpoint
		return domain.ArtifactTypeEndpoint
	}

	hostname := parsedURL.Hostname()

	// 3. Check if it's an external domain (not target or subdomain of target)
	if !g.isSameDomain(hostname, target.Root) {
		// External domain → URL (e.g., CDNs, third-party APIs)
		return domain.ArtifactTypeURL
	}

	// 4. Same domain - check if it's a static resource
	if g.isStaticResource(fullURL) {
		// Static resource (JS, CSS, images) → URL
		return domain.ArtifactTypeURL
	}

	// 5. Same domain - check if it's an API path
	if g.isAPIPath(fullURL) {
		// API endpoint → Endpoint
		return domain.ArtifactTypeEndpoint
	}

	// 6. Default for same-domain absolute URLs: URL
	// (Complete URLs are more useful as URL artifacts for tracking)
	return domain.ArtifactTypeURL
}

// isSameDomain checks if hostname belongs to the target domain.
func (g *GoLinkfinderEvoSource) isSameDomain(hostname string, targetRoot string) bool {
	if hostname == targetRoot {
		return true
	}
	// Check if it's a subdomain of target (e.g., api.example.com for target example.com)
	return strings.HasSuffix(hostname, "."+targetRoot)
}

// isStaticResource checks if URL points to a static resource.
func (g *GoLinkfinderEvoSource) isStaticResource(urlStr string) bool {
	staticExts := []string{
		".js", ".mjs", ".jsx", // JavaScript (but might also be endpoints)
		".css", ".scss", ".sass", ".less", // Styles
		".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp", ".bmp", ".tiff", // Images
		".woff", ".woff2", ".ttf", ".eot", ".otf", // Fonts
		".mp4", ".avi", ".mov", ".webm", ".mkv", ".flv", // Videos
		".mp3", ".wav", ".ogg", ".flac", ".aac", // Audio
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", // Documents
		".zip", ".tar", ".gz", ".rar", ".7z", ".bz2", // Archives
	}

	urlLower := strings.ToLower(urlStr)

	// Check file extensions
	for _, ext := range staticExts {
		// Match extension at end of URL or before query params
		if strings.HasSuffix(urlLower, ext) || strings.Contains(urlLower, ext+"?") {
			return true
		}
	}

	// Check for common static resource paths
	staticPaths := []string{
		"/static/", "/assets/", "/dist/", "/build/",
		"/images/", "/img/", "/css/", "/js/", "/fonts/",
		"/media/", "/public/", "/resources/",
	}

	for _, path := range staticPaths {
		if strings.Contains(urlLower, path) {
			return true
		}
	}

	return false
}

// isAPIPath checks if URL contains API-related patterns.
func (g *GoLinkfinderEvoSource) isAPIPath(urlStr string) bool {
	apiPatterns := []string{
		"/api/", "/api-", "/apis/",
		"/v1/", "/v2/", "/v3/", "/v4/",
		"/graphql", "/gql/",
		"/rest/", "/restful/",
		"/oauth", "/oauth2",
		"/rpc/", "/jsonrpc",
		"/grpc/",
	}

	urlLower := strings.ToLower(urlStr)

	for _, pattern := range apiPatterns {
		if strings.Contains(urlLower, pattern) {
			return true
		}
	}

	return false
}

// extractSubdomainsFromURL extracts subdomains from discovered URLs.
// Only extracts external domains (not the target domain itself).
func (g *GoLinkfinderEvoSource) extractSubdomainsFromURL(fullURL string, target domain.Target) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0)

	// Parse URL
	parsedURL, err := url.Parse(fullURL)
	if err != nil || parsedURL.Hostname() == "" {
		return artifacts
	}

	hostname := parsedURL.Hostname()

	// Skip if it's the target root itself
	if hostname == target.Root {
		return artifacts
	}

	// Skip if it's a subdomain of the target (already discovered by other sources)
	if strings.HasSuffix(hostname, "."+target.Root) {
		return artifacts
	}

	// Skip localhost and IP addresses
	if hostname == "localhost" || g.isIPAddress(hostname) {
		return artifacts
	}

	// Extract external subdomain
	artifact := domain.NewArtifact(
		domain.ArtifactTypeSubdomain,
		hostname,
		g.Name(),
	)

	artifact.SetDiscoveryContext(&domain.DiscoveryContext{
		SourceURL: fullURL,
	})
	artifact.SetClassification(&domain.Classification{
		IsExternal: true,
	})
	artifact.Confidence = 0.8

	artifacts = append(artifacts, artifact)

	return artifacts
}

// isIPAddress checks if a string is an IP address.
func (g *GoLinkfinderEvoSource) isIPAddress(s string) bool {
	// Simple check: contains only digits and dots/colons
	for _, c := range s {
		if !((c >= '0' && c <= '9') || c == '.' || c == ':') {
			return false
		}
	}
	// Must contain at least one dot or colon
	return strings.Contains(s, ".") || strings.Contains(s, ":")
}

// convertGFFindingsToArtifacts converts GF findings to domain artifacts.
func (g *GoLinkfinderEvoSource) convertGFFindingsToArtifacts(
	gfFindings GFFindings,
	target domain.Target,
) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0)

	// Process each finding
	for _, finding := range gfFindings.Findings {
		// Each finding can match multiple rules
		for _, ruleName := range finding.Rules {
			artifactType := g.gfParser.inferArtifactType(ruleName, finding.Evidence)

			artifact := domain.NewArtifact(
				artifactType,
				finding.Evidence, // Use Evidence field instead of Match
				"golinkfinderevo-gf",
			)

			// Set discovery context
			artifact.SetDiscoveryContext(&domain.DiscoveryContext{
				SourceResource: finding.Resource,
				LineNumber:     finding.Line,
				Context:        finding.Context,
				MatchPattern:   ruleName,
			})

			// Add category-specific security context and classification
			g.gfParser.addCategoryContexts(artifact, ruleName)
			artifact.Confidence = g.gfParser.calculateConfidence(ruleName, finding.Evidence)

			artifacts = append(artifacts, artifact)
		}
	}

	g.GetLogger().Debug("converted GF findings to artifacts",
		"total_findings", len(gfFindings.Findings),
		"total_rules_executed", len(gfFindings.Rules),
		"total_artifacts", len(artifacts),
	)

	return artifacts
}

// SetCustomFlags sets custom command-line flags.
func (g *GoLinkfinderEvoSource) SetCustomFlags(flags []string) {
	g.customFlags = flags
}

// executeWithStdin executes golinkfinderevo using stdin for URLs and captures JSON from stdout.
func (g *GoLinkfinderEvoSource) executeWithStdin(
	ctx context.Context,
	target domain.Target,
	urls []string,
) (*domain.ScanResult, error) {
	result := domain.NewScanResult(target)

	// Apply timeout to context if not already set or if existing timeout is too long
	execCtx := ctx
	var cancel context.CancelFunc
	if deadline, ok := ctx.Deadline(); !ok {
		// No deadline set, apply default timeout
		execCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
	} else if timeRemaining := time.Until(deadline); timeRemaining > defaultTimeout {
		// Existing deadline is too long, apply shorter timeout
		execCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
	}
	// If existing deadline is shorter than defaultTimeout, use it as-is

	// Build command arguments (no input/output files)
	args := g.buildCommandArgsStdout(target)

	// Build command with timeout-enforced context
	cmd := exec.CommandContext(execCtx, g.GetExecPath(), args...)

	// Configure stdin with URLs
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Capture stdout (JSON)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Capture stderr for logs
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start golinkfinderevo: %w", err)
	}

	// Log first few URLs being processed
	urlPreview := urls
	if len(urlPreview) > 5 {
		urlPreview = urls[:5]
	}
	g.GetLogger().Debug("golinkfinderevo process started",
		"pid", cmd.Process.Pid,
		"urls_count", len(urls),
		"url_preview", urlPreview,
	)

	// Write URLs to stdin in goroutine
	go func() {
		defer stdin.Close()
		urlsWritten := 0
		for _, url := range urls {
			if _, err := fmt.Fprintln(stdin, url); err != nil {
				g.GetLogger().Debug("failed to write URL to stdin", "error", err, "urls_written", urlsWritten)
				return
			}
			urlsWritten++
		}
		g.GetLogger().Debug("finished writing URLs to stdin", "total_written", urlsWritten)
	}()

	// Read stderr in goroutine
	stderrChan := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(stderr)
		stderrChan <- string(data)
	}()

	// Read stdout (complete JSON)
	stdoutData, err := io.ReadAll(stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to read stdout: %w", err)
	}

	// Wait for process to complete
	waitErr := cmd.Wait()

	// Get stderr output
	stderrStr := <-stderrChan
	if len(stderrStr) > 0 {
		g.GetLogger().Debug("golinkfinderevo stderr", "output", stderrStr)
	}

	// Log stdout data received
	previewLen := len(stdoutData)
	if previewLen > 300 {
		previewLen = 300
	}
	g.GetLogger().Debug("golinkfinderevo stdout received",
		"total_bytes", len(stdoutData),
		"preview", string(stdoutData[:previewLen]),
	)

	// Handle process error with graceful degradation
	// Try to parse any stdout data even if the process failed
	var output GoLinkfinderJSONOutput
	hasValidOutput := false

	if len(stdoutData) > 0 {
		if err := json.Unmarshal(stdoutData, &output); err == nil {
			hasValidOutput = true
			g.GetLogger().Debug("JSON output parsed successfully despite process error",
				"resources_count", len(output.Resources),
				"gf_findings_total", output.GFFindings.Total,
			)
		}
	}

	// If process failed, decide whether to return error or partial results
	if waitErr != nil {
		g.GetLogger().Warn("golinkfinderevo process failed",
			"error", waitErr.Error(),
			"stderr", stderrStr,
			"exit_code", cmd.ProcessState.ExitCode(),
			"stdout_bytes", len(stdoutData),
			"has_valid_output", hasValidOutput,
		)

		// If we have valid partial results, continue with warning
		if hasValidOutput {
			result.AddWarning(sourceName, fmt.Sprintf("partial results: process exited with error (exit code %d): %s",
				cmd.ProcessState.ExitCode(),
				truncateStderr(stderrStr, 200)))
			g.GetLogger().Info("continuing with partial results despite process error")
		} else {
			// No valid output - return error
			return nil, fmt.Errorf("golinkfinderevo failed: %w (stderr: %s)", waitErr, truncateStderr(stderrStr, 500))
		}
	} else {
		// Process succeeded, parse output normally
		if err := json.Unmarshal(stdoutData, &output); err != nil {
			previewLen := len(stdoutData)
			if previewLen > 500 {
				previewLen = 500
			}
			g.GetLogger().Warn("failed to parse JSON output",
				"error", err,
				"output_preview", string(stdoutData[:previewLen]),
			)
			return nil, fmt.Errorf("failed to parse JSON output: %w", err)
		}
	}

	g.GetLogger().Debug("JSON output parsed successfully",
		"resources_count", len(output.Resources),
		"gf_findings_total", output.GFFindings.Total,
		"gf_rules_count", len(output.GFFindings.Rules),
	)

	// Convert output to artifacts
	artifacts := g.convertOutputToArtifacts(output, target)
	for _, artifact := range artifacts {
		result.AddArtifact(artifact)
	}

	// Store metadata
	if result.Metadata.Environment == nil {
		result.Metadata.Environment = make(map[string]string)
	}
	result.Metadata.Environment["golinkfinderevo_urls_processed"] = strconv.Itoa(len(urls))
	result.Metadata.Environment["golinkfinderevo_profile"] = string(g.profile)
	result.Metadata.Environment["golinkfinderevo_resources"] = strconv.Itoa(len(output.Resources))
	result.Metadata.Environment["golinkfinderevo_gf_total"] = strconv.Itoa(output.GFFindings.Total)
	result.Metadata.Environment["golinkfinderevo_gf_rules"] = strconv.Itoa(len(output.GFFindings.Rules))

	g.GetLogger().Info("golinkfinderevo execution successful",
		"resources", len(output.Resources),
		"gf_total_findings", output.GFFindings.Total,
		"gf_rules", len(output.GFFindings.Rules),
		"artifacts", len(result.Artifacts),
	)

	return result, nil
}

// truncateStderr truncates stderr output to maxLen characters.
func truncateStderr(stderr string, maxLen int) string {
	if len(stderr) <= maxLen {
		return stderr
	}
	return stderr[:maxLen] + "..."
}

// Close releases resources.
func (g *GoLinkfinderEvoSource) Close() error {
	return g.BaseCLISource.Close()
}
