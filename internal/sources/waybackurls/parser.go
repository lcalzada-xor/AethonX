// Package waybackurls implements integration with waybackurls CLI tool.
// parser.go handles parsing of waybackurls output lines.
package waybackurls

import (
	"net/url"
	"strings"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/validator"
)

// Parser parses waybackurls output and converts to artifacts.
type Parser struct {
	logger     logx.Logger
	sourceName string
	analyzer   *URLAnalyzer
}

// NewParser creates a new Parser.
func NewParser(logger logx.Logger, sourceName string) *Parser {
	return &Parser{
		logger:     logger.With("component", "parser"),
		sourceName: sourceName,
		analyzer:   NewURLAnalyzer(logger),
	}
}

// ParseLine parses a single line from waybackurls output.
// Line format can be:
//   - Simple: "https://example.com/path"
//   - With dates: "2020-09-30 22:51:11 https://example.com/path"
func (p *Parser) ParseLine(line string, target domain.Target) []*domain.Artifact {
	if line == "" {
		return nil
	}

	// Trim whitespace
	line = strings.TrimSpace(line)

	// Parse line to extract URL and optional timestamp
	urlStr, timestamp := p.ExtractURLAndTimestamp(line)

	// Validate URL format
	if !validator.IsURL(urlStr) {
		p.logger.Debug("invalid URL format", "url", urlStr)
		return nil
	}

	// Parse URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		p.logger.Debug("failed to parse URL", "url", urlStr, "error", err.Error())
		return nil
	}

	// Skip if URL is not in scope
	if !p.isInScope(parsedURL, target) {
		p.logger.Debug("URL out of scope", "url", urlStr, "target", target.Root)
		return nil
	}

	// Analyze URL and extract all artifact types
	artifacts := p.analyzer.AnalyzeURL(parsedURL, urlStr, target, timestamp)

	return artifacts
}

// ParseMultipleLines parses multiple lines and deduplicates artifacts.
func (p *Parser) ParseMultipleLines(lines []string, target domain.Target) []*domain.Artifact {
	seen := make(map[string]bool) // key: "type:value"
	var artifacts []*domain.Artifact

	for _, line := range lines {
		lineArtifacts := p.ParseLine(line, target)
		for _, artifact := range lineArtifacts {
			// Deduplicate based on type + value
			key := string(artifact.Type) + ":" + artifact.Value
			if !seen[key] {
				seen[key] = true
				artifacts = append(artifacts, artifact)
			}
		}
	}

	return artifacts
}

// ExtractURLAndTimestamp extracts URL and optional timestamp from line.
// Returns (url, timestamp) where timestamp is empty string if not present.
func (p *Parser) ExtractURLAndTimestamp(line string) (string, string) {
	// Check if line contains timestamp format: "YYYY-MM-DD HH:MM:SS URL"
	// Look for "http" or "https" in the line
	httpIdx := strings.Index(line, "http://")
	httpsIdx := strings.Index(line, "https://")

	var urlStart int
	if httpIdx >= 0 && (httpsIdx < 0 || httpIdx < httpsIdx) {
		urlStart = httpIdx
	} else if httpsIdx >= 0 {
		urlStart = httpsIdx
	} else {
		// No http/https found, assume entire line is URL
		return line, ""
	}

	// If URL starts at position > 0, everything before is timestamp
	if urlStart > 0 {
		timestamp := strings.TrimSpace(line[:urlStart])
		urlStr := strings.TrimSpace(line[urlStart:])
		return urlStr, timestamp
	}

	// No timestamp, entire line is URL
	return line, ""
}

// isInScope checks if a URL is in scope for the target.
func (p *Parser) isInScope(u *url.URL, target domain.Target) bool {
	host := strings.ToLower(u.Host)

	// Remove port if present
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Check if host matches target root exactly
	if host == target.Root {
		return true
	}

	// Check if host is a subdomain of target
	if strings.HasSuffix(host, "."+target.Root) {
		return true
	}

	return false
}
