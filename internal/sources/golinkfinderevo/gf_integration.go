// Package golinkfinderevo - GF Integration
// Handles parsing and conversion of gf (grep filters) pattern matching results.
package golinkfinderevo

import (
	"fmt"
	"strings"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
)

// GFFinding represents a single gf pattern match.
// Example: {"pattern": "jwt", "match": "eyJhbGc...", "resource": "https://example.com/app.js", "line": 42}
type GFFinding struct {
	Pattern  string `json:"pattern"`
	Match    string `json:"match"`
	Resource string `json:"resource"`
	Line     int    `json:"line"`
}

// GFResults maps pattern names to their findings.
// Example: {"jwt": [{...}], "api-keys": [{...}]}
type GFResults map[string][]GFFinding

// GFParser handles conversion of gf results to domain artifacts.
type GFParser struct {
	logger logx.Logger
}

// NewGFParser creates a new GFParser instance.
func NewGFParser(logger logx.Logger) *GFParser {
	return &GFParser{
		logger: logger,
	}
}

// ConvertToArtifacts converts GF findings into domain artifacts.
// NOTE: This function is kept for backward compatibility but is no longer used.
// GF findings are now integrated directly in the JSON output from golinkfinder -o json.
func (gp *GFParser) ConvertToArtifacts(results GFResults, target domain.Target) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0)

	for pattern, findings := range results {
		for _, finding := range findings {
			artifactType := gp.inferArtifactType(pattern, finding.Match)

			// Skip if we can't determine a valid artifact type
			if artifactType == "" {
				continue
			}

			artifact := domain.NewArtifact(
				artifactType,
				finding.Match,
				"golinkfinderevo-gf",
			)

			// Add contextual tags
			artifact.AddTag("gf_pattern:" + pattern)
			artifact.AddTag("discovered_in:" + finding.Resource)
			if finding.Line > 0 {
				artifact.AddTag(fmt.Sprintf("line:%d", finding.Line))
			}

			// Category-specific tags
			gp.addCategoryTags(artifact, pattern)

			// Calculate confidence based on pattern reliability
			artifact.Confidence = gp.calculateConfidence(pattern, finding.Match)

			artifacts = append(artifacts, artifact)
		}
	}

	gp.logger.Info("converted gf findings to artifacts",
		"patterns", len(results),
		"artifacts", len(artifacts),
	)

	return artifacts
}

// inferArtifactType maps gf pattern names to artifact types with evidence validation.
// It analyzes both the pattern name AND the actual matched evidence for accurate classification.
func (gp *GFParser) inferArtifactType(pattern string, evidence string) domain.ArtifactType {
	patternLower := strings.ToLower(pattern)

	// Priority 1: Analyze evidence content first for high-confidence matches
	// This prevents misclassification when the match is clearly a different type

	// Email addresses in evidence
	if strings.Contains(evidence, "@") && !strings.Contains(evidence, "://") {
		// Contains @ but not :// (not a URL) → likely email
		if gp.looksLikeEmail(evidence) {
			return domain.ArtifactTypeEmail
		}
	}

	// Complete URLs in evidence (especially for endpoint/url patterns)
	if strings.HasPrefix(evidence, "http://") || strings.HasPrefix(evidence, "https://") {
		// If it's a static resource URL → URL artifact
		if gp.isStaticResourceURL(evidence) {
			return domain.ArtifactTypeURL
		}
		// If it's an API URL → could be Endpoint or URL depending on pattern context
		// Let pattern-based logic decide below
	}

	// Priority 2: Pattern-based classification with evidence validation

	// Internal IPs (RFC1918 private IPs)
	if strings.Contains(patternLower, "internal") && strings.Contains(patternLower, "ip") {
		return domain.ArtifactTypeInternalIP
	}

	// Database connections
	if strings.Contains(patternLower, "database") || strings.Contains(patternLower, "db") ||
		strings.Contains(patternLower, "connection") {
		return domain.ArtifactTypeDBConnection
	}

	// OAuth tokens (distinct from general credentials)
	if strings.Contains(patternLower, "oauth") || strings.Contains(patternLower, "bearer") ||
		(strings.Contains(patternLower, "token") && (strings.Contains(evidence, "access_token") || strings.Contains(evidence, "refresh_token"))) {
		return domain.ArtifactTypeOAuthToken
	}

	// Developer notes/comments
	if strings.Contains(patternLower, "comment") || strings.Contains(patternLower, "developer") ||
		strings.Contains(patternLower, "note") || strings.Contains(patternLower, "todo") {
		return domain.ArtifactTypeDeveloperNote
	}

	// Emails (pattern-based)
	if strings.Contains(patternLower, "email") || strings.Contains(patternLower, "mail") {
		return domain.ArtifactTypeEmail
	}

	// Credentials and secrets (general)
	if strings.Contains(patternLower, "key") ||
		strings.Contains(patternLower, "token") ||
		strings.Contains(patternLower, "secret") ||
		strings.Contains(patternLower, "password") ||
		strings.Contains(patternLower, "credential") ||
		patternLower == "jwt" ||
		strings.Contains(patternLower, "aws") ||
		strings.Contains(patternLower, "github") ||
		strings.Contains(patternLower, "slack") {
		return domain.ArtifactTypeCredential
	}

	// Storage buckets
	if strings.Contains(patternLower, "s3") ||
		strings.Contains(patternLower, "bucket") ||
		strings.Contains(patternLower, "storage") ||
		strings.Contains(patternLower, "blob") ||
		strings.Contains(patternLower, "cloud") {
		return domain.ArtifactTypeStorageBucket
	}

	// Sensitive files
	if strings.Contains(patternLower, "file") ||
		strings.Contains(patternLower, "backup") ||
		strings.Contains(patternLower, "config") ||
		strings.Contains(patternLower, "env") {
		return domain.ArtifactTypeSensitiveFile
	}

	// Parameters (for injection vectors)
	if strings.Contains(patternLower, "sqli") ||
		strings.Contains(patternLower, "xss") ||
		strings.Contains(patternLower, "param") ||
		strings.Contains(patternLower, "injection") {
		return domain.ArtifactTypeParameter
	}

	// Endpoints and APIs
	if strings.Contains(patternLower, "endpoint") ||
		strings.Contains(patternLower, "url") ||
		strings.Contains(patternLower, "api") ||
		strings.Contains(patternLower, "rest") ||
		strings.Contains(patternLower, "graphql") {
		// Check evidence: complete URL vs path
		if strings.HasPrefix(evidence, "http://") || strings.HasPrefix(evidence, "https://") {
			// Complete URL → URL artifact (more useful for tracking external resources)
			return domain.ArtifactTypeURL
		}
		// Relative path → Endpoint
		return domain.ArtifactTypeEndpoint
	}

	// Crypto material
	if strings.Contains(patternLower, "crypto") ||
		strings.Contains(patternLower, "private") ||
		strings.Contains(patternLower, "certificate") ||
		strings.Contains(patternLower, "rsa") ||
		strings.Contains(patternLower, "pgp") {
		return domain.ArtifactTypeCredential
	}

	// Default: analyze evidence to make best guess
	if strings.HasPrefix(evidence, "http://") || strings.HasPrefix(evidence, "https://") {
		return domain.ArtifactTypeURL
	}

	// Final fallback to endpoint for unrecognized patterns
	gp.logger.Debug("unknown gf pattern, defaulting to endpoint", "pattern", pattern)
	return domain.ArtifactTypeEndpoint
}

// looksLikeEmail performs basic email validation.
func (gp *GFParser) looksLikeEmail(s string) bool {
	// Basic check: has @ and at least one dot after @
	parts := strings.Split(s, "@")
	if len(parts) != 2 {
		return false
	}
	domain := parts[1]
	return strings.Contains(domain, ".")
}

// isStaticResourceURL checks if a URL points to a static resource.
func (gp *GFParser) isStaticResourceURL(urlStr string) bool {
	staticExts := []string{
		".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg",
		".ico", ".webp", ".woff", ".woff2", ".ttf", ".eot",
		".mp4", ".mp3", ".pdf", ".zip",
	}

	urlLower := strings.ToLower(urlStr)

	for _, ext := range staticExts {
		if strings.HasSuffix(urlLower, ext) || strings.Contains(urlLower, ext+"?") {
			return true
		}
	}

	return false
}

// addCategoryTags adds semantic tags based on pattern category.
func (gp *GFParser) addCategoryTags(artifact *domain.Artifact, pattern string) {
	patternLower := strings.ToLower(pattern)

	// Security tags
	if strings.Contains(patternLower, "sqli") {
		artifact.AddTag("vulnerability:sqli")
		artifact.AddTag("severity:high")
	}
	if strings.Contains(patternLower, "xss") {
		artifact.AddTag("vulnerability:xss")
		artifact.AddTag("severity:medium")
	}

	// Cloud provider tags
	if strings.Contains(patternLower, "aws") {
		artifact.AddTag("provider:aws")
	}
	if strings.Contains(patternLower, "gcp") || strings.Contains(patternLower, "google") {
		artifact.AddTag("provider:gcp")
	}
	if strings.Contains(patternLower, "azure") {
		artifact.AddTag("provider:azure")
	}

	// Token type tags
	if patternLower == "jwt" {
		artifact.AddTag("token_type:jwt")
		artifact.AddTag("severity:high")
	}
	if strings.Contains(patternLower, "github") {
		artifact.AddTag("token_type:github")
		artifact.AddTag("severity:critical")
	}
	if strings.Contains(patternLower, "slack") {
		artifact.AddTag("token_type:slack")
		artifact.AddTag("severity:high")
	}

	// Sensitive data tags
	if strings.Contains(patternLower, "password") || strings.Contains(patternLower, "secret") {
		artifact.AddTag("data_type:credential")
		artifact.AddTag("severity:critical")
	}

	// Internal IP tags
	if strings.Contains(patternLower, "internal") && strings.Contains(patternLower, "ip") {
		artifact.AddTag("data_type:internal_ip")
		artifact.AddTag("severity:high")
		artifact.AddTag("exposure:internal_network")
	}

	// Database connection tags
	if strings.Contains(patternLower, "database") || strings.Contains(patternLower, "connection") {
		artifact.AddTag("data_type:db_connection")
		artifact.AddTag("severity:critical")
		artifact.AddTag("exposure:connection_string")
	}

	// OAuth token tags
	if strings.Contains(patternLower, "oauth") || strings.Contains(patternLower, "bearer") {
		artifact.AddTag("data_type:oauth_token")
		artifact.AddTag("severity:high")
		artifact.AddTag("token_type:oauth2")
	}

	// Developer comment tags
	if strings.Contains(patternLower, "comment") || strings.Contains(patternLower, "developer") {
		artifact.AddTag("data_type:developer_note")
		artifact.AddTag("severity:low")
		artifact.AddTag("info:code_comment")
	}

	// Email tags
	if strings.Contains(patternLower, "email") {
		artifact.AddTag("data_type:email")
		artifact.AddTag("severity:low")
	}
}

// calculateConfidence estimates confidence based on pattern and match characteristics.
func (gp *GFParser) calculateConfidence(pattern, match string) float64 {
	patternLower := strings.ToLower(pattern)
	matchLower := strings.ToLower(match)

	// High confidence patterns (specific regexes)
	if patternLower == "jwt" && strings.HasPrefix(match, "eyJ") {
		return 0.95 // JWT signature is very distinctive
	}

	if strings.Contains(patternLower, "aws") && strings.HasPrefix(match, "AKIA") {
		return 0.98 // AWS access key pattern
	}

	if strings.Contains(patternLower, "github") && strings.HasPrefix(match, "ghp_") {
		return 0.97 // GitHub personal access token
	}

	// Cloud storage patterns
	if strings.Contains(patternLower, "s3") && strings.Contains(matchLower, ".s3.") {
		return 0.9
	}

	// API endpoints
	if strings.Contains(patternLower, "endpoint") {
		if strings.HasPrefix(match, "/api/") {
			return 0.85
		}
		return 0.7
	}

	// Generic credentials
	if strings.Contains(patternLower, "credential") || strings.Contains(patternLower, "password") {
		// Check if match looks like actual credential (length, complexity)
		if len(match) >= 16 && gp.hasComplexity(match) {
			return 0.8
		}
		return 0.6
	}

	// Injection patterns (lower confidence - could be false positives)
	if strings.Contains(patternLower, "sqli") || strings.Contains(patternLower, "xss") {
		return 0.65
	}

	// Sensitive files
	if strings.Contains(patternLower, "file") || strings.Contains(patternLower, "backup") {
		if strings.Contains(matchLower, ".env") || strings.Contains(matchLower, "config") {
			return 0.85
		}
		return 0.7
	}

	// Internal IPs (RFC1918)
	if strings.Contains(patternLower, "internal") && strings.Contains(patternLower, "ip") {
		// High confidence for standard private IP ranges
		if strings.HasPrefix(match, "10.") || strings.HasPrefix(match, "192.168.") || strings.Contains(match, "172.") {
			return 0.9
		}
		return 0.75
	}

	// Database connections
	if strings.Contains(patternLower, "database") || strings.Contains(patternLower, "connection") {
		// Very high confidence for protocol-prefixed connection strings
		if strings.Contains(matchLower, "://") && (strings.Contains(matchLower, "mongodb") ||
			strings.Contains(matchLower, "postgres") || strings.Contains(matchLower, "mysql") ||
			strings.Contains(matchLower, "redis")) {
			return 0.95
		}
		return 0.8
	}

	// OAuth tokens
	if strings.Contains(patternLower, "oauth") || strings.Contains(patternLower, "bearer") {
		// High confidence for well-formatted tokens
		if len(match) >= 20 && gp.hasComplexity(match) {
			return 0.88
		}
		return 0.75
	}

	// Developer comments
	if strings.Contains(patternLower, "comment") || strings.Contains(patternLower, "developer") {
		// Medium-low confidence (many false positives possible)
		if strings.Contains(matchLower, "todo") || strings.Contains(matchLower, "fixme") {
			return 0.7
		}
		return 0.6
	}

	// Email addresses
	if strings.Contains(patternLower, "email") {
		// High confidence for valid email format
		if gp.looksLikeEmail(match) {
			return 0.92
		}
		return 0.7
	}

	// Default confidence
	return 0.7
}

// hasComplexity checks if a string has sufficient complexity to be a credential.
func (gp *GFParser) hasComplexity(s string) bool {
	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false

	for _, c := range s {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}

	// Require at least 2 character types
	complexity := 0
	if hasUpper {
		complexity++
	}
	if hasLower {
		complexity++
	}
	if hasDigit {
		complexity++
	}
	if hasSpecial {
		complexity++
	}

	return complexity >= 2
}

// countFindings returns total number of findings across all patterns.
func (gp *GFParser) countFindings(results GFResults) int {
	total := 0
	for _, findings := range results {
		total += len(findings)
	}
	return total
}

// SummarizeFindings creates a human-readable summary of GF results.
func (gp *GFParser) SummarizeFindings(results GFResults) map[string]int {
	summary := make(map[string]int)

	for pattern, findings := range results {
		summary[pattern] = len(findings)
	}

	return summary
}
