// Package golinkfinderevo implements integration with GoLinkfinderEVO.
// Parser handles JSON output parsing from golinkfinderevo CLI tool.
package golinkfinderevo

import (
	"net/url"
	"strings"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
)

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
		// Validate URL has a proper domain with TLD
		if !p.isValidURL(endpoint) {
			p.logger.Debug("skipping invalid absolute URL", "url", endpoint)
			return ""
		}
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

// isValidURL validates that a URL has a proper domain with TLD and is not a test/placeholder URL.
func (p *Parser) isValidURL(urlStr string) bool {
	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Reject special schemes that are not real endpoints
	if u.Scheme == "about" || u.Scheme == "data" || u.Scheme == "javascript" || u.Scheme == "file" {
		return false
	}

	// Get hostname without port
	host := u.Hostname()
	if host == "" {
		return false
	}

	// Allow localhost explicitly (important for local testing)
	if host == "localhost" {
		return true
	}

	// Reject single-character domains (e.g., https://a, https://x)
	if len(host) <= 2 {
		return false
	}

	// Reject domains without dots (no TLD) - except localhost which was already checked
	if !strings.Contains(host, ".") {
		return false
	}

	// Reject known test/placeholder domains (but allow localhost - important for local testing)
	testDomains := []string{
		"example.com", "example.org", "example.net",
		"test.com", "test.org", "test.net",
		"dummy.com", "placeholder.com",
	}
	hostLower := strings.ToLower(host)
	for _, testDomain := range testDomains {
		if hostLower == testDomain {
			return false
		}
	}

	return true
}
