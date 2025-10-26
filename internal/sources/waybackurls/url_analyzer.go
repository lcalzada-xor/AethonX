// Package waybackurls implements integration with waybackurls CLI tool.
// url_analyzer.go provides intelligent URL analysis to extract multiple artifact types.
package waybackurls

import (
	"net/url"
	"path/filepath"
	"strings"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/validator"
)

// URLAnalyzer performs intelligent analysis of URLs to extract multiple artifact types.
type URLAnalyzer struct {
	logger logx.Logger
}

// NewURLAnalyzer creates a new URLAnalyzer.
func NewURLAnalyzer(logger logx.Logger) *URLAnalyzer {
	return &URLAnalyzer{
		logger: logger,
	}
}

// Pattern definitions for detection
var (
	// Sensitive files that should never be exposed
	sensitivePatterns = []string{
		".env", "config.php", "database.yml", "credentials.json",
		"web.config", ".htpasswd", ".htaccess", "id_rsa", "id_dsa",
		"authorized_keys", "secrets.yml", "settings.py", "application.properties",
	}

	// Backup file extensions
	backupPatterns = []string{
		".bak", ".old", ".backup", ".sql", ".sql.gz", ".sql.bz2",
		".tar.gz", ".zip", ".rar", ".7z", ".dump", ".orig", ".save",
	}

	// Repository paths
	repoPatterns = []string{
		"/.git/", "/.svn/", "/.hg/", "/.bzr/", "/.cvs/",
	}

	// API path indicators
	apiPatterns = []string{
		"/api/", "/rest/", "/graphql", "/v1/", "/v2/", "/v3/", "/v4/",
		"/api-", "/restapi/", "/webapi/",
	}

	// Technology detection map (path -> technology name)
	techPatterns = map[string]string{
		"/wp-admin/":      "WordPress",
		"/wp-content/":    "WordPress",
		"/wp-includes/":   "WordPress",
		"/phpmyadmin/":    "phpMyAdmin",
		"/admin/":         "Admin Panel",
		"/administrator/": "Admin Panel",
		"/cpanel/":        "cPanel",
		"/plesk/":         "Plesk",
		"/webmail/":       "Webmail",
		"/joomla/":        "Joomla",
		"/drupal/":        "Drupal",
		"/magento/":       "Magento",
		"/moodle/":        "Moodle",
		"/typo3/":         "TYPO3",
	}
)

// AnalyzeURL analyzes a URL and extracts all possible artifact types.
func (a *URLAnalyzer) AnalyzeURL(u *url.URL, rawURL string, target domain.Target, timestamp string) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0, 10)

	// 1. Primary artifact: URL itself
	urlArtifact := a.createURLArtifact(rawURL, timestamp)
	artifacts = append(artifacts, urlArtifact)

	// 2. Extract subdomain if different from target
	if subArtifact := a.extractSubdomain(u.Host, target); subArtifact != nil {
		artifacts = append(artifacts, subArtifact)
	}

	// 3. Extract endpoint (clean path without query)
	if endpointArtifact := a.extractEndpoint(u.Path); endpointArtifact != nil {
		artifacts = append(artifacts, endpointArtifact)
	}

	// 4. Extract query parameters
	paramArtifacts := a.extractParameters(u.Query())
	artifacts = append(artifacts, paramArtifacts...)

	// 5. Detect JavaScript files
	if jsArtifact := a.detectJavaScript(u.Path, rawURL); jsArtifact != nil {
		artifacts = append(artifacts, jsArtifact)
	}

	// 6. Detect sensitive files (HIGH PRIORITY)
	if sensitiveArtifact := a.detectSensitiveFile(u.Path, rawURL); sensitiveArtifact != nil {
		artifacts = append(artifacts, sensitiveArtifact)
	}

	// 7. Detect backup files
	if backupArtifact := a.detectBackupFile(u.Path, rawURL); backupArtifact != nil {
		artifacts = append(artifacts, backupArtifact)
	}

	// 8. Detect repositories (CRITICAL)
	if repoArtifact := a.detectRepository(u.Path, rawURL); repoArtifact != nil {
		artifacts = append(artifacts, repoArtifact)
	}

	// 9. Detect API endpoints
	if apiArtifact := a.detectAPI(u.Path, rawURL); apiArtifact != nil {
		artifacts = append(artifacts, apiArtifact)
	}

	// 10. Detect technologies
	if techArtifact := a.detectTechnology(u); techArtifact != nil {
		artifacts = append(artifacts, techArtifact)
	}

	return artifacts
}

// createURLArtifact creates the primary URL artifact with metadata.
func (a *URLAnalyzer) createURLArtifact(rawURL string, timestamp string) *domain.Artifact {
	meta := metadata.NewDomainMetadata()
	if timestamp != "" {
		meta.LastProbed = timestamp
		meta.ProbeSource = "waybackurls"
	}

	artifact := domain.NewArtifactWithMetadata(
		domain.ArtifactTypeURL,
		rawURL,
		"waybackurls",
		meta,
	)

	// waybackurls provides historical data - low confidence until verified
	artifact.Confidence = domain.ConfidenceLow

	return artifact
}

// extractSubdomain extracts subdomain from hostname if in scope.
func (a *URLAnalyzer) extractSubdomain(host string, target domain.Target) *domain.Artifact {
	// Remove port if present
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Normalize hostname
	host = strings.ToLower(strings.TrimSpace(host))

	// Skip if it's the target root itself
	if host == target.Root {
		return nil
	}

	// Check if it's a subdomain of target
	if !strings.HasSuffix(host, "."+target.Root) && host != target.Root {
		// Out of scope
		return nil
	}

	// Validate domain format
	if !validator.IsDomain(host) {
		return nil
	}

	artifact := domain.NewArtifact(
		domain.ArtifactTypeSubdomain,
		host,
		"waybackurls",
	)
	artifact.Confidence = domain.ConfidenceLow // Historical subdomain

	return artifact
}

// extractEndpoint extracts clean endpoint path (no query params).
func (a *URLAnalyzer) extractEndpoint(path string) *domain.Artifact {
	// Skip empty or root paths
	if path == "" || path == "/" {
		return nil
	}

	// Clean path
	path = strings.TrimSpace(path)

	// Skip very long paths (likely not meaningful endpoints)
	if len(path) > 500 {
		return nil
	}

	artifact := domain.NewArtifact(
		domain.ArtifactTypeEndpoint,
		path,
		"waybackurls",
	)
	artifact.Confidence = domain.ConfidenceLow

	return artifact
}

// extractParameters extracts query parameters from URL.
func (a *URLAnalyzer) extractParameters(query url.Values) []*domain.Artifact {
	if len(query) == 0 {
		return nil
	}

	artifacts := make([]*domain.Artifact, 0, len(query))
	seen := make(map[string]bool)

	for paramName := range query {
		// Skip empty parameter names
		if paramName == "" {
			continue
		}

		// Deduplicate parameters
		if seen[paramName] {
			continue
		}
		seen[paramName] = true

		artifacts = append(artifacts, domain.NewArtifact(
			domain.ArtifactTypeParameter,
			paramName,
			"waybackurls",
		))
	}

	return artifacts
}

// detectJavaScript detects JavaScript files.
func (a *URLAnalyzer) detectJavaScript(path string, rawURL string) *domain.Artifact {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".js" {
		return domain.NewArtifact(
			domain.ArtifactTypeJavaScript,
			rawURL,
			"waybackurls",
		)
	}
	return nil
}

// detectSensitiveFile detects potentially sensitive files.
func (a *URLAnalyzer) detectSensitiveFile(path string, rawURL string) *domain.Artifact {
	pathLower := strings.ToLower(path)
	fileName := filepath.Base(pathLower)

	for _, pattern := range sensitivePatterns {
		if strings.Contains(fileName, pattern) || strings.HasSuffix(fileName, pattern) {
			a.logger.Warn("detected sensitive file", "url", rawURL, "pattern", pattern)
			return domain.NewArtifact(
				domain.ArtifactTypeSensitiveFile,
				rawURL,
				"waybackurls",
			)
		}
	}

	return nil
}

// detectBackupFile detects backup files.
func (a *URLAnalyzer) detectBackupFile(path string, rawURL string) *domain.Artifact {
	pathLower := strings.ToLower(path)
	fileName := filepath.Base(pathLower)

	for _, pattern := range backupPatterns {
		if strings.HasSuffix(fileName, pattern) {
			a.logger.Info("detected backup file", "url", rawURL, "pattern", pattern)
			return domain.NewArtifact(
				domain.ArtifactTypeBackupFile,
				rawURL,
				"waybackurls",
			)
		}
	}

	return nil
}

// detectRepository detects exposed repository paths.
func (a *URLAnalyzer) detectRepository(path string, rawURL string) *domain.Artifact {
	pathLower := strings.ToLower(path)

	for _, pattern := range repoPatterns {
		if strings.Contains(pathLower, pattern) {
			a.logger.Warn("detected repository exposure", "url", rawURL, "pattern", pattern)
			return domain.NewArtifact(
				domain.ArtifactTypeRepository,
				rawURL,
				"waybackurls",
			)
		}
	}

	return nil
}

// detectAPI detects API endpoints.
func (a *URLAnalyzer) detectAPI(path string, rawURL string) *domain.Artifact {
	pathLower := strings.ToLower(path)

	for _, pattern := range apiPatterns {
		if strings.Contains(pathLower, pattern) {
			return domain.NewArtifact(
				domain.ArtifactTypeAPI,
				rawURL,
				"waybackurls",
			)
		}
	}

	return nil
}

// detectTechnology detects technologies based on URL patterns.
func (a *URLAnalyzer) detectTechnology(u *url.URL) *domain.Artifact {
	pathLower := strings.ToLower(u.Path)

	for pattern, techName := range techPatterns {
		if strings.Contains(pathLower, pattern) {
			return domain.NewArtifact(
				domain.ArtifactTypeTechnology,
				techName,
				"waybackurls",
			)
		}
	}

	return nil
}
