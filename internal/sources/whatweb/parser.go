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

		// Extract technologies from plugins
		for pluginName, plugin := range resp.Plugins {
			// Create Technology artifact
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
