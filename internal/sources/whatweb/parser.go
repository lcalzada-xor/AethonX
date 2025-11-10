package whatweb

import (
	"fmt"
	"strings"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/platform/logx"
)

// Parser handles parsing of whatweb output into domain artifacts.
type Parser struct {
	logger     logx.Logger
	sourceName string
}

// NewParser creates a new parser for whatweb output.
func NewParser(logger logx.Logger, sourceName string) *Parser {
	return &Parser{
		logger:     logger.With("component", "parser"),
		sourceName: sourceName,
	}
}

// WhatWebResponse represents a single response from whatweb JSON output.
type WhatWebResponse struct {
	Target        string            `json:"target"`
	HTTPStatus    int               `json:"http_status"`
	RequestConfig RequestConfig     `json:"request_config"`
	Plugins       map[string]Plugin `json:"plugins"`
}

// RequestConfig contains HTTP request details.
type RequestConfig struct {
	Headers map[string]string `json:"headers"`
}

// Plugin represents a detected technology/plugin.
type Plugin struct {
	String   []string               `json:"string,omitempty"`
	Version  []string               `json:"version,omitempty"`
	Account  []string               `json:"account,omitempty"`
	Category []string               `json:"category,omitempty"`
	OS       []string               `json:"os,omitempty"`
	Data     map[string]interface{} `json:",inline"` // Catch-all for other fields
}

// ParseMultipleResponses converts multiple whatweb responses into artifacts.
func (p *Parser) ParseMultipleResponses(responses []WhatWebResponse, target domain.Target) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0, len(responses)*2)
	seenTech := make(map[string]bool)    // Deduplication for technologies
	seenService := make(map[string]bool) // Deduplication for services
	seenIP := make(map[string]bool)      // Deduplication for IPs

	for _, resp := range responses {
		// Skip failed requests
		if resp.HTTPStatus == 0 {
			p.logger.Debug("skipping response with no HTTP status",
				"target", resp.Target,
			)
			continue
		}

		// Validate scope
		if !target.IsInScope(resp.Target) {
			p.logger.Debug("skipping out-of-scope target",
				"target", resp.Target,
				"root", target.Root,
			)
			continue
		}

		// Process each plugin with appropriate handler
		for pluginName, plugin := range resp.Plugins {
			// Handle special plugins that are NOT technologies
			if p.isMetadataPlugin(pluginName) {
				metaArtifacts := p.handleMetadataPlugin(pluginName, plugin, resp, target, seenIP)
				artifacts = append(artifacts, metaArtifacts...)
				continue
			}

			// Create Technology artifact only for real technologies
			techArtifacts := p.createTechnologyArtifacts(pluginName, plugin, resp, target, seenTech)
			artifacts = append(artifacts, techArtifacts...)

			// Create Service artifact for web servers, frameworks, etc.
			if p.isServicePlugin(pluginName) {
				serviceArtifact := p.createServiceArtifact(pluginName, plugin, resp, target, seenService)
				if serviceArtifact != nil {
					artifacts = append(artifacts, serviceArtifact)
				}
			}
		}
	}

	p.logger.Info("parsed responses",
		"total_responses", len(responses),
		"artifacts_created", len(artifacts),
	)

	return artifacts
}

// isMetadataPlugin determines if a plugin contains metadata rather than technology info.
func (p *Parser) isMetadataPlugin(pluginName string) bool {
	metadataPlugins := map[string]bool{
		"IP":                  true, // Contains IP address, not a technology
		"Country":             true, // Geographic metadata
		"HTTPServer":          true, // Contains actual server name in data
		"RedirectLocation":    true, // HTTP redirect metadata
		"TXT Record":          true, // DNS metadata
		"SPF Policy":          true, // Email security metadata
		"Domain Verification": true, // Domain verification metadata
		"UncommonHeaders":     true, // HTTP headers metadata
		"Title":               true, // Page title metadata
		"Email":               true, // Email addresses
		"X-Powered-By":        true, // Header that contains tech info in data
	}

	return metadataPlugins[pluginName]
}

// handleMetadataPlugin processes metadata plugins and creates appropriate artifacts.
func (p *Parser) handleMetadataPlugin(pluginName string, plugin Plugin, resp WhatWebResponse, target domain.Target, seenIP map[string]bool) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0)

	switch pluginName {
	case "IP":
		// Extract IP addresses and create IP artifacts
		for _, ipStr := range plugin.String {
			ipStr = strings.TrimSpace(ipStr)
			if ipStr == "" || seenIP[ipStr] {
				continue
			}
			seenIP[ipStr] = true

			// Create IP artifact with metadata
			ipMeta := metadata.NewIPMetadata()
			artifact := domain.NewArtifactWithMetadata(
				domain.ArtifactTypeIP,
				ipStr,
				p.sourceName,
				ipMeta,
			)
			artifact.Confidence = domain.ConfidenceHigh

			artifacts = append(artifacts, artifact)
			p.logger.Debug("created IP artifact from IP plugin",
				"ip", ipStr,
				"target", resp.Target,
			)
		}

	case "HTTPServer":
		// Extract actual server technology from the plugin data
		// Example: {"string": ["nginx/1.18.0"]} -> Technology: "nginx", Version: "1.18.0"
		for _, serverStr := range plugin.String {
			techName, version := p.parseServerString(serverStr)
			if techName == "" {
				continue
			}

			// Create Technology artifact with the real server name
			meta := metadata.NewTechnologyMetadata(techName, version)
			meta.Category = "Web Server"

			artifact := domain.NewArtifactWithMetadata(
				domain.ArtifactTypeTechnology,
				techName,
				p.sourceName,
				meta,
			)
			artifact.Confidence = domain.ConfidenceHigh

			if version != "" {
				artifact.AddTag(fmt.Sprintf("version:%s", version))
			}
			artifact.AddTag("web-server")

			artifacts = append(artifacts, artifact)
			p.logger.Debug("created technology artifact from HTTPServer plugin",
				"technology", techName,
				"version", version,
				"target", resp.Target,
			)
		}

	case "Country":
		// Country data enriches IP metadata but doesn't create standalone artifacts
		// This could be used to enrich existing IP artifacts in a future enhancement
		p.logger.Debug("country metadata detected (not creating artifact)",
			"country", plugin.String,
			"module", plugin.Data["module"],
		)

	case "RedirectLocation":
		// Extract redirect URLs
		for _, urlStr := range plugin.String {
			urlStr = strings.TrimSpace(urlStr)
			if urlStr == "" || !strings.HasPrefix(urlStr, "http") {
				continue
			}

			artifact := domain.NewArtifact(
				domain.ArtifactTypeURL,
				urlStr,
				p.sourceName,
			)
			artifact.Confidence = domain.ConfidenceHigh
			artifact.AddTag("redirect")

			artifacts = append(artifacts, artifact)
			p.logger.Debug("created URL artifact from redirect",
				"url", urlStr,
			)
		}

	case "Email":
		// Extract email addresses
		for _, emailStr := range plugin.String {
			emailStr = strings.TrimSpace(emailStr)
			if emailStr == "" {
				continue
			}

			artifact := domain.NewArtifact(
				domain.ArtifactTypeEmail,
				emailStr,
				p.sourceName,
			)
			artifact.Confidence = domain.ConfidenceMedium

			artifacts = append(artifacts, artifact)
			p.logger.Debug("created email artifact",
				"email", emailStr,
			)
		}

	case "X-Powered-By":
		// Extract technology from X-Powered-By header
		// Example: "PHP/7.4.3" -> Technology: "PHP", Version: "7.4.3"
		for _, poweredBy := range plugin.String {
			techName, version := p.parseServerString(poweredBy)
			if techName == "" {
				continue
			}

			meta := metadata.NewTechnologyMetadata(techName, version)
			meta.Category = "Programming Language"
			meta.DetectionMethod = "X-Powered-By Header"

			artifact := domain.NewArtifactWithMetadata(
				domain.ArtifactTypeTechnology,
				techName,
				p.sourceName,
				meta,
			)
			artifact.Confidence = domain.ConfidenceHigh

			if version != "" {
				artifact.AddTag(fmt.Sprintf("version:%s", version))
			}

			artifacts = append(artifacts, artifact)
			p.logger.Debug("created technology artifact from X-Powered-By",
				"technology", techName,
				"version", version,
			)
		}

	default:
		// Other metadata plugins are logged but don't create artifacts
		p.logger.Debug("metadata plugin processed (no artifact created)",
			"plugin", pluginName,
			"data", plugin.String,
		)
	}

	return artifacts
}

// parseServerString extracts technology name and version from server strings.
// Examples: "nginx/1.18.0" -> ("nginx", "1.18.0")
//           "Apache/2.4.41 (Ubuntu)" -> ("Apache", "2.4.41")
//           "PHP/7.4.3" -> ("PHP", "7.4.3")
func (p *Parser) parseServerString(serverStr string) (name string, version string) {
	serverStr = strings.TrimSpace(serverStr)
	if serverStr == "" {
		return "", ""
	}

	// Remove trailing parenthetical info: "Apache/2.4.41 (Ubuntu)" -> "Apache/2.4.41"
	if idx := strings.Index(serverStr, " ("); idx > 0 {
		serverStr = serverStr[:idx]
	}

	// Split by "/" to separate name and version
	parts := strings.SplitN(serverStr, "/", 2)
	name = strings.TrimSpace(parts[0])

	if len(parts) > 1 {
		version = strings.TrimSpace(parts[1])
	}

	return name, version
}

// createTechnologyArtifacts creates Technology artifacts from a plugin.
func (p *Parser) createTechnologyArtifacts(pluginName string, plugin Plugin, resp WhatWebResponse, target domain.Target, seen map[string]bool) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0)

	// Normalize plugin name for technology
	techName := normalizeTechName(pluginName)

	// Get version info
	version := ""
	if len(plugin.Version) > 0 {
		version = plugin.Version[0]
	}

	// Create unique key for deduplication
	key := fmt.Sprintf("tech:%s:%s:%s", resp.Target, techName, version)
	if seen[key] {
		return artifacts
	}
	seen[key] = true

	// Create metadata
	meta := metadata.NewTechnologyMetadata(techName, version)

	// Add categories if available
	if len(plugin.Category) > 0 {
		meta.Category = plugin.Category[0]
	}

	// Create artifact
	artifact := domain.NewArtifactWithMetadata(
		domain.ArtifactTypeTechnology,
		techName,
		p.sourceName,
		meta,
	)

	// Set confidence based on detection method
	artifact.Confidence = calculateConfidence(plugin)

	// Add version as tag if available
	if version != "" {
		artifact.AddTag(fmt.Sprintf("version:%s", version))
	}

	// Add categories as tags
	for _, cat := range plugin.Category {
		artifact.AddTag(strings.ToLower(cat))
	}

	artifacts = append(artifacts, artifact)

	p.logger.Debug("created technology artifact",
		"name", techName,
		"version", version,
		"target", resp.Target,
	)

	return artifacts
}

// createServiceArtifact creates a Service artifact from a plugin.
func (p *Parser) createServiceArtifact(pluginName string, plugin Plugin, resp WhatWebResponse, target domain.Target, seen map[string]bool) *domain.Artifact {
	serviceName := normalizeServiceName(pluginName)

	// Get version
	version := ""
	if len(plugin.Version) > 0 {
		version = plugin.Version[0]
	}

	// Create unique key
	key := fmt.Sprintf("service:%s:%s:%s", resp.Target, serviceName, version)
	if seen[key] {
		return nil
	}
	seen[key] = true

	// Extract port from target URL first
	port := 80
	if strings.Contains(resp.Target, ":443") || strings.HasPrefix(resp.Target, "https://") {
		port = 443
	} else if strings.Contains(resp.Target, ":") {
		// Try to extract port from URL
		parts := strings.Split(resp.Target, ":")
		if len(parts) >= 3 {
			// Format like http://example.com:8080
			portPart := strings.TrimSuffix(parts[len(parts)-1], "/")
			fmt.Sscanf(portPart, "%d", &port)
		}
	}

	// Create metadata
	meta := metadata.NewServiceMetadata(serviceName, port)
	meta.Version = version
	meta.Protocol = "HTTP"

	// Create artifact
	artifact := domain.NewArtifactWithMetadata(
		domain.ArtifactTypeService,
		serviceName,
		p.sourceName,
		meta,
	)

	artifact.Confidence = calculateConfidence(plugin)

	if version != "" {
		artifact.AddTag(fmt.Sprintf("version:%s", version))
	}

	p.logger.Debug("created service artifact",
		"name", serviceName,
		"version", version,
		"port", meta.Port,
	)

	return artifact
}

// isServicePlugin determines if a plugin represents a service.
func (p *Parser) isServicePlugin(pluginName string) bool {
	servicePlugins := map[string]bool{
		"Apache":    true,
		"Nginx":     true,
		"IIS":       true,
		"LiteSpeed": true,
		"Cherokee":  true,
		"Tomcat":    true,
		"Jetty":     true,
		"WebLogic":  true,
		"JBoss":     true,
		"Werkzeug":  true,
		"Gunicorn":  true,
		"uWSGI":     true,
		"Passenger": true,
		"Unicorn":   true,
		"Puma":      true,
	}

	return servicePlugins[pluginName]
}

// normalizeTechName normalizes technology names.
func normalizeTechName(pluginName string) string {
	// Remove special characters and normalize
	name := strings.TrimSpace(pluginName)
	name = strings.ReplaceAll(name, "-", " ")
	return name
}

// normalizeServiceName normalizes service names.
func normalizeServiceName(pluginName string) string {
	// Services typically keep their original names
	return strings.TrimSpace(pluginName)
}

// calculateConfidence calculates confidence score based on plugin data.
func calculateConfidence(plugin Plugin) float64 {
	// Base confidence
	confidence := domain.ConfidenceMedium

	// Higher confidence if version detected
	if len(plugin.Version) > 0 {
		confidence = domain.ConfidenceHigh
	}

	// Multiple detection methods increase confidence
	detectionMethods := 0
	if len(plugin.String) > 0 {
		detectionMethods++
	}
	if len(plugin.Version) > 0 {
		detectionMethods++
	}
	if len(plugin.Account) > 0 {
		detectionMethods++
	}

	if detectionMethods >= 3 {
		confidence = domain.ConfidenceHigh
	} else if detectionMethods >= 2 {
		confidence = domain.ConfidenceMedium
	}

	return confidence
}

// ParseSingle converts a single whatweb response into artifacts.
func (p *Parser) ParseSingle(resp WhatWebResponse, target domain.Target) ([]*domain.Artifact, error) {
	if !target.IsInScope(resp.Target) {
		return nil, fmt.Errorf("target out of scope: %s", resp.Target)
	}

	artifacts := p.ParseMultipleResponses([]WhatWebResponse{resp}, target)
	return artifacts, nil
}
