// Package golinkfinderevo implements integration with GoLinkfinderEVO.
// Parser handles JSON output parsing from golinkfinderevo CLI tool.
package golinkfinderevo

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
)

// ResourceReport represents golinkfinderevo JSON output format.
// Example: {"url": "https://example.com/app.js", "endpoints": ["/api/users", "/api/products"]}
type ResourceReport struct {
	URL       string   `json:"url"`
	Endpoints []string `json:"endpoints"`
}

// Parser handles parsing of golinkfinderevo JSON output.
type Parser struct {
	logger     logx.Logger
	sourceName string
}

// NewParser creates a new Parser instance.
func NewParser(logger logx.Logger, sourceName string) *Parser {
	return &Parser{
		logger:     logger,
		sourceName: sourceName,
	}
}

// ParseReport parses a single ResourceReport from JSON bytes.
func (p *Parser) ParseReport(data []byte) (*ResourceReport, error) {
	var report ResourceReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("failed to parse golinkfinderevo report: %w", err)
	}

	// Validate required fields
	if report.URL == "" {
		return nil, fmt.Errorf("missing required field: url")
	}

	return &report, nil
}

// ParseMultipleReports parses multiple ResourceReports from newline-delimited JSON.
func (p *Parser) ParseMultipleReports(data []byte) ([]*ResourceReport, error) {
	reports := make([]*ResourceReport, 0)
	lines := strings.Split(string(data), "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		report, err := p.ParseReport([]byte(line))
		if err != nil {
			p.logger.Warn("failed to parse report line",
				"line_number", i+1,
				"error", err.Error(),
			)
			continue
		}

		reports = append(reports, report)
	}

	return reports, nil
}

// ConvertToArtifacts converts a ResourceReport into domain artifacts.
func (p *Parser) ConvertToArtifacts(report *ResourceReport, target domain.Target) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0, len(report.Endpoints))

	for _, endpoint := range report.Endpoints {
		// Normalize endpoint to full URL
		fullURL := p.normalizeEndpoint(report.URL, endpoint)
		if fullURL == "" {
			continue
		}

		artifact := domain.NewArtifact(
			domain.ArtifactTypeEndpoint,
			fullURL,
			p.sourceName,
		)

		// Add context tags
		artifact.AddTag("discovered_from:" + report.URL)
		artifact.AddTag("method:linkfinder")

		// Set confidence based on endpoint quality
		artifact.Confidence = p.calculateConfidence(endpoint)

		artifacts = append(artifacts, artifact)
	}

	return artifacts
}

// ConvertMultipleReports converts multiple ResourceReports into artifacts.
func (p *Parser) ConvertMultipleReports(reports []*ResourceReport, target domain.Target) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0)

	for _, report := range reports {
		reportArtifacts := p.ConvertToArtifacts(report, target)
		artifacts = append(artifacts, reportArtifacts...)
	}

	p.logger.Debug("converted reports to artifacts",
		"total_reports", len(reports),
		"total_artifacts", len(artifacts),
	)

	return artifacts
}

// normalizeEndpoint converts a relative endpoint to a full URL.
func (p *Parser) normalizeEndpoint(baseURL, endpoint string) string {
	// Skip empty or invalid endpoints
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}

	// Parse base URL
	base, err := url.Parse(baseURL)
	if err != nil {
		p.logger.Warn("invalid base URL", "url", baseURL, "error", err.Error())
		return ""
	}

	// Handle absolute URLs
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}

	// Handle protocol-relative URLs (//example.com/path)
	if strings.HasPrefix(endpoint, "//") {
		return base.Scheme + ":" + endpoint
	}

	// Handle absolute paths (/api/users)
	if strings.HasPrefix(endpoint, "/") {
		base.Path = endpoint
		base.RawQuery = ""
		base.Fragment = ""
		return base.String()
	}

	// Handle relative paths (api/users)
	// Extract directory from base path
	basePath := base.Path
	if strings.Contains(basePath, "/") {
		// Remove filename from base path
		lastSlash := strings.LastIndex(basePath, "/")
		basePath = basePath[:lastSlash+1]
	}

	// Combine base path + relative endpoint
	base.Path = basePath + endpoint
	base.RawQuery = ""
	base.Fragment = ""

	return base.String()
}

// calculateConfidence estimates confidence based on endpoint characteristics.
func (p *Parser) calculateConfidence(endpoint string) float64 {
	confidence := 0.7 // Base confidence

	// High confidence patterns
	if strings.HasPrefix(endpoint, "/api/") {
		confidence = 0.95
	} else if strings.HasPrefix(endpoint, "/v1/") || strings.HasPrefix(endpoint, "/v2/") {
		confidence = 0.9
	} else if strings.Contains(endpoint, "/graphql") || strings.Contains(endpoint, "/rest") {
		confidence = 0.95
	}

	// Medium confidence patterns
	if strings.HasSuffix(endpoint, ".json") || strings.HasSuffix(endpoint, ".xml") {
		confidence = 0.85
	}

	// Lower confidence for very generic paths
	if endpoint == "/" || endpoint == "/index" || endpoint == "/home" {
		confidence = 0.5
	}

	// Check for URL parameters (higher confidence)
	if strings.Contains(endpoint, "?") {
		confidence += 0.05
		if confidence > 1.0 {
			confidence = 1.0
		}
	}

	return confidence
}

// ExtractParametersFromEndpoint extracts URL parameters as separate artifacts.
func (p *Parser) ExtractParametersFromEndpoint(endpoint string, target domain.Target) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0)

	// Parse URL to extract query parameters
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return artifacts
	}

	query := parsedURL.Query()
	for paramName := range query {
		artifact := domain.NewArtifact(
			domain.ArtifactTypeParameter,
			paramName,
			p.sourceName,
		)

		artifact.AddTag("endpoint:" + endpoint)
		artifact.AddTag("type:query_param")

		// Assess parameter sensitivity
		if p.isSensitiveParameter(paramName) {
			artifact.AddTag("sensitive")
			artifact.Confidence = 0.9
		} else {
			artifact.Confidence = 0.7
		}

		artifacts = append(artifacts, artifact)
	}

	return artifacts
}

// isSensitiveParameter checks if a parameter name indicates sensitive data.
func (p *Parser) isSensitiveParameter(paramName string) bool {
	paramLower := strings.ToLower(paramName)

	sensitivePatterns := []string{
		"token", "key", "secret", "password", "pwd", "pass",
		"auth", "session", "cookie", "api_key", "apikey",
		"access", "refresh", "jwt", "bearer",
		"admin", "user", "username", "email",
		"id", "uid", "userid", "account",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(paramLower, pattern) {
			return true
		}
	}

	return false
}
