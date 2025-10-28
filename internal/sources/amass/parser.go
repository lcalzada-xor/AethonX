// Package amass implements integration with OWASP Amass CLI tool.
// It executes amass as a subprocess and parses its JSON output to create artifacts.
package amass

import (
	"fmt"
	"strconv"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/platform/logx"
)

// AmassResponse representa una línea del output JSON de amass enum.
// Amass output is JSONL format (one JSON object per line, not an array).
type AmassResponse struct {
	Timestamp string         `json:"Timestamp"` // ISO 8601 timestamp
	Name      string         `json:"name"`      // FQDN discovered (subdomain)
	Domain    string         `json:"domain"`    // Root domain
	Addresses []AmassAddress `json:"addresses"` // IP addresses with network info
	Tag       string         `json:"tag"`       // Classification: "ext", "int", etc.
	Source    string         `json:"source"`    // Data source that found this entry
}

// AmassAddress representa información de dirección IP del output de amass.
type AmassAddress struct {
	IP   string `json:"ip"`   // IPv4 or IPv6 address
	CIDR string `json:"cidr"` // Network CIDR notation
	ASN  int    `json:"asn"`  // Autonomous System Number
	Desc string `json:"desc"` // AS organization description
}

// Parser parsea el output JSON de amass a artifacts de AethonX.
type Parser struct {
	logger     logx.Logger
	sourceName string
}

// NewParser crea un nuevo Parser para amass.
func NewParser(logger logx.Logger, sourceName string) *Parser {
	return &Parser{
		logger:     logger.With("component", "amass_parser"),
		sourceName: sourceName,
	}
}

// ParseResponse convierte un AmassResponse a múltiples artifacts.
// Genera artifacts para: subdomain, IPs, CIDRs, ASNs.
func (p *Parser) ParseResponse(resp *AmassResponse, target domain.Target) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0, 1+len(resp.Addresses)*3)

	// 1. Subdomain artifact (principal)
	subdomainArtifact := p.parseSubdomain(resp, target)
	if subdomainArtifact != nil {
		artifacts = append(artifacts, subdomainArtifact)
	}

	// 2. IP, CIDR, ASN artifacts (uno por cada address)
	for _, addr := range resp.Addresses {
		// IP artifact with metadata
		if addr.IP != "" {
			ipArtifact := p.parseIP(addr)
			if ipArtifact != nil {
				artifacts = append(artifacts, ipArtifact)
			}
		}

		// CIDR artifact
		if addr.CIDR != "" {
			cidrArtifact := domain.NewArtifact(
				domain.ArtifactTypeCIDR,
				addr.CIDR,
				p.sourceName,
			)
			artifacts = append(artifacts, cidrArtifact)
		}

		// ASN artifact
		if addr.ASN > 0 {
			asnValue := fmt.Sprintf("AS%d", addr.ASN)
			asnArtifact := domain.NewArtifact(
				domain.ArtifactTypeASN,
				asnValue,
				p.sourceName,
			)
			artifacts = append(artifacts, asnArtifact)
		}
	}

	p.logger.Debug("parsed amass response",
		"name", resp.Name,
		"addresses", len(resp.Addresses),
		"artifacts", len(artifacts),
	)

	return artifacts
}

// parseSubdomain crea el artifact de subdomain con metadata.
func (p *Parser) parseSubdomain(resp *AmassResponse, target domain.Target) *domain.Artifact {
	if resp.Name == "" {
		p.logger.Warn("empty name in amass response", "domain", resp.Domain)
		return nil
	}

	// Create domain metadata
	domainMeta := metadata.NewDomainMetadata()

	// Extract IPs from addresses
	ips := make([]string, 0, len(resp.Addresses))
	for _, addr := range resp.Addresses {
		if addr.IP != "" {
			ips = append(ips, addr.IP)
		}
	}
	domainMeta.ResolvedIPs = ips

	// Set status from tag ("ext", "int", etc.)
	if resp.Tag != "" {
		domainMeta.Status = resp.Tag
	}

	// Create artifact with metadata
	artifact := domain.NewArtifactWithMetadata(
		domain.ArtifactTypeSubdomain,
		resp.Name,
		p.sourceName,
		domainMeta,
	)

	return artifact
}

// parseIP crea el artifact de IP con metadata de red.
func (p *Parser) parseIP(addr AmassAddress) *domain.Artifact {
	// Create IP metadata
	ipMeta := metadata.NewIPMetadata()

	// Network info
	if addr.ASN > 0 {
		ipMeta.ASN = strconv.Itoa(addr.ASN)
	}
	if addr.Desc != "" {
		ipMeta.ASOrg = addr.Desc
	}
	if addr.CIDR != "" {
		ipMeta.CIDR = addr.CIDR
	}

	// Create artifact with metadata
	artifact := domain.NewArtifactWithMetadata(
		domain.ArtifactTypeIP,
		addr.IP,
		p.sourceName,
		ipMeta,
	)

	return artifact
}

// ParseMultipleResponses parsea múltiples AmassResponse a artifacts.
func (p *Parser) ParseMultipleResponses(responses []*AmassResponse, target domain.Target) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0, len(responses)*4) // Estimate: 4 artifacts per response

	for _, resp := range responses {
		parsed := p.ParseResponse(resp, target)
		artifacts = append(artifacts, parsed...)
	}

	p.logger.Debug("parsed multiple amass responses",
		"responses", len(responses),
		"total_artifacts", len(artifacts),
	)

	return artifacts
}
