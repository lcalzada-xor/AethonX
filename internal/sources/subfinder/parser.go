// Package subfinder implements integration with Project Discovery's subfinder CLI tool.
// It executes subfinder as a subprocess and parses its JSON output to create artifacts.
package subfinder

import (
	"encoding/json"
	"fmt"
	"strings"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/platform/logx"
)

// StringOrArray is a custom type that can unmarshal both string and []string from JSON
type StringOrArray []string

// UnmarshalJSON implements custom unmarshaling to handle both string and array
func (sa *StringOrArray) UnmarshalJSON(data []byte) error {
	// Try unmarshaling as array first
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*sa = StringOrArray(arr)
		return nil
	}

	// If that fails, try as string
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	*sa = StringOrArray([]string{str})
	return nil
}

// SubfinderResponse represents a single JSON record from subfinder output.
// Subfinder outputs JSONL format (one JSON object per line).
type SubfinderResponse struct {
	Host      string        `json:"host"`
	Source    StringOrArray `json:"source"`
	Timestamp string        `json:"timestamp,omitempty"`
}

// Parser handles parsing of subfinder JSON output into domain artifacts.
type Parser struct {
	logger     logx.Logger
	sourceName string
}

// NewParser creates a new subfinder output parser.
func NewParser(logger logx.Logger, sourceName string) *Parser {
	return &Parser{
		logger:     logger.With("component", "subfinder_parser"),
		sourceName: sourceName,
	}
}

// ParseResponse converts a single SubfinderResponse into artifacts.
func (p *Parser) ParseResponse(resp *SubfinderResponse, target domain.Target) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0)

	if resp == nil || resp.Host == "" {
		return artifacts
	}

	// Normalize host
	host := strings.TrimSpace(strings.ToLower(resp.Host))

	// Skip wildcards
	if strings.HasPrefix(host, "*.") {
		p.logger.Debug("skipping wildcard subdomain", "host", host)
		return artifacts
	}

	// Verify target is in scope
	if !target.IsInScope(host) {
		p.logger.Debug("host out of scope", "host", host, "target", target.Root)
		return artifacts
	}

	// Create domain metadata
	domainMeta := metadata.NewDomainMetadata()

	// Create artifact
	artifact := domain.NewArtifactWithMetadata(
		domain.ArtifactTypeSubdomain,
		host,
		p.sourceName,
		domainMeta,
	)

	// Add tags for subfinder sources that discovered this subdomain
	if len(resp.Source) > 0 {
		for _, src := range resp.Source {
			artifact.AddTag(fmt.Sprintf("source:%s", src))
		}
	}

	// High confidence for subfinder (aggregates from trusted public sources)
	artifact.Confidence = 0.90

	// Add tags based on subdomain characteristics
	if strings.Count(host, ".") > strings.Count(target.Root, ".")+1 {
		artifact.AddTag("deep-subdomain")
	}

	artifacts = append(artifacts, artifact)

	return artifacts
}

// ParseMultipleResponses processes multiple subfinder responses.
func (p *Parser) ParseMultipleResponses(responses []*SubfinderResponse, target domain.Target) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0, len(responses))
	seen := make(map[string]bool)

	for _, resp := range responses {
		parsedArtifacts := p.ParseResponse(resp, target)

		for _, artifact := range parsedArtifacts {
			// Deduplicate by value (subfinder might return duplicates from different sources)
			if !seen[artifact.Value] {
				artifacts = append(artifacts, artifact)
				seen[artifact.Value] = true
			} else {
				// Merge sources if duplicate found
				p.logger.Debug("duplicate subdomain found",
					"host", artifact.Value,
					"existing_sources", len(artifacts),
				)
			}
		}
	}

	p.logger.Debug("parsed subfinder responses",
		"total_responses", len(responses),
		"unique_artifacts", len(artifacts),
		"duplicates_filtered", len(responses)-len(artifacts),
	)

	return artifacts
}

// ValidateResponse checks if a SubfinderResponse is valid.
func (p *Parser) ValidateResponse(resp *SubfinderResponse) error {
	if resp == nil {
		return fmt.Errorf("response is nil")
	}

	if resp.Host == "" {
		return fmt.Errorf("host is empty")
	}

	// Host should not contain protocol
	if strings.Contains(resp.Host, "://") {
		return fmt.Errorf("host contains protocol: %s", resp.Host)
	}

	return nil
}
