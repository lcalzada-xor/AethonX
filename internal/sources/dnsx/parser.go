// Package dnsx - DNS response parser
package dnsx

import (
	"strings"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/platform/logx"
)

// Parser handles parsing of dnsx JSON responses into artifacts.
type Parser struct {
	logger     logx.Logger
	sourceName string
}

// NewParser creates a new dnsx parser.
func NewParser(logger logx.Logger, sourceName string) *Parser {
	return &Parser{
		logger:     logger,
		sourceName: sourceName,
	}
}

// ParseMultipleResponses parses multiple dnsx responses into artifacts.
// inputArtifacts is used to upgrade confidence of existing artifacts.
func (p *Parser) ParseMultipleResponses(responses []*DNSXResponse, target domain.Target, inputArtifacts []*domain.Artifact) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0, len(responses)*10)

	for _, resp := range responses {
		parsed := p.parseResponse(resp, target)
		artifacts = append(artifacts, parsed...)
	}

	p.logger.Info("parsed dnsx responses to artifacts",
		"responses", len(responses),
		"artifacts", len(artifacts),
	)

	return artifacts
}

// parseResponse parses a single dnsx response into multiple artifacts.
func (p *Parser) parseResponse(resp *DNSXResponse, target domain.Target) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0, 20)

	// Only process NOERROR responses (successful DNS resolution)
	if resp.StatusCode != "NOERROR" {
		p.logger.Debug("skipping non-NOERROR response",
			"host", resp.Host,
			"status", resp.StatusCode,
		)
		return artifacts
	}

	// 1. SUBDOMAIN with full DNS metadata (confidence 1.0)
	domainMeta := &metadata.DomainMetadata{
		DNSResolved:   true,
		TTL:           resp.TTL,
		Resolvers:     resp.Resolver,
		Nameservers:   resp.NS,
		CDN:           resp.CDN,
		ASN:           resp.ASN,
		WildcardMatch: false, // dnsx already filtered wildcards
		ResolvedIPs:   resp.A, // Store A records
	}

	// Add AAAA to resolved IPs
	if len(resp.AAAA) > 0 {
		domainMeta.ResolvedIPs = append(domainMeta.ResolvedIPs, resp.AAAA...)
	}

	// Add CNAME info
	if len(resp.CNAME) > 0 {
		domainMeta.AliasTarget = resp.CNAME[0]
	}

	// Add SOA metadata
	if len(resp.SOA) > 0 {
		soa := resp.SOA[0]
		domainMeta.Zone = &metadata.SOARecord{
			Name:     soa.Name,
			NS:       soa.NS,
			Mailbox:  soa.Mailbox,
			Serial:   soa.Serial,
			Refresh:  soa.Refresh,
			Retry:    soa.Retry,
			Expire:   soa.Expire,
			MinTTL:   soa.MinTTL,
		}
	}

	// Record types for metadata
	recordTypes := make([]string, 0, 7)
	if len(resp.A) > 0 {
		recordTypes = append(recordTypes, "A")
	}
	if len(resp.AAAA) > 0 {
		recordTypes = append(recordTypes, "AAAA")
	}
	if len(resp.CNAME) > 0 {
		recordTypes = append(recordTypes, "CNAME")
	}
	if len(resp.MX) > 0 {
		recordTypes = append(recordTypes, "MX")
	}
	if len(resp.TXT) > 0 {
		recordTypes = append(recordTypes, "TXT")
	}
	if len(resp.NS) > 0 {
		recordTypes = append(recordTypes, "NS")
	}
	if len(resp.SOA) > 0 {
		recordTypes = append(recordTypes, "SOA")
	}
	domainMeta.DNSRecords = recordTypes

	artifact := domain.NewArtifactWithMetadata(
		domain.ArtifactTypeSubdomain,
		resp.Host,
		p.sourceName,
		domainMeta,
	)
	artifact.Confidence = 1.0 // DNS validated
	artifacts = append(artifacts, artifact)

	// 2. IP ADDRESSES (A records)
	for _, ip := range resp.A {
		ipMeta := &metadata.IPMetadata{
			PTRRecords:  resp.PTR,
			CDNProvider: resp.CDN,
			ASN:         resp.ASN,
			DNSSource:   p.sourceName,
		}

		// Legacy PTR support
		if len(resp.PTR) > 0 {
			ipMeta.PTRRecord = resp.PTR[0]
			ipMeta.ReverseDNS = resp.PTR[0]
		}

		artifact := domain.NewArtifactWithMetadata(
			domain.ArtifactTypeIP,
			ip,
			p.sourceName,
			ipMeta,
		)
		artifact.Confidence = 1.0
		artifacts = append(artifacts, artifact)
	}

	// 3. IPv6 ADDRESSES (AAAA records)
	for _, ipv6 := range resp.AAAA {
		ipMeta := &metadata.IPMetadata{
			ASN:        resp.ASN,
			DNSSource:  p.sourceName,
			IPVersion:  "6",
		}

		artifact := domain.NewArtifactWithMetadata(
			domain.ArtifactTypeIP,
			ipv6,
			p.sourceName,
			ipMeta,
		)
		artifact.Confidence = 1.0
		artifacts = append(artifacts, artifact)
	}

	// 4. CNAME (alias subdomains)
	for _, cname := range resp.CNAME {
		// Clean CNAME (remove trailing dot)
		cname = strings.TrimSuffix(cname, ".")

		cnMeta := &metadata.DomainMetadata{
			AliasTarget: cname,
			DNSResolved: true,
		}

		artifact := domain.NewArtifactWithMetadata(
			domain.ArtifactTypeSubdomain,
			cname,
			p.sourceName,
			cnMeta,
		)
		artifact.Confidence = 1.0
		artifacts = append(artifacts, artifact)
	}

	// 5. MX RECORDS (mail servers)
	for _, mx := range resp.MX {
		if mx == "" || mx == "." {
			continue
		}

		// Clean MX record (remove trailing dot and priority prefix)
		mx = strings.TrimSuffix(mx, ".")

		svcMeta := &metadata.ServiceMetadata{
			Name:     "MX Record",
			Protocol: "smtp",
			Port:     25, // SMTP default port
			State:    "discovered",
		}

		artifact := domain.NewArtifactWithMetadata(
			domain.ArtifactTypeService,
			mx,
			p.sourceName,
			svcMeta,
		)
		artifact.Confidence = 1.0
		artifacts = append(artifacts, artifact)
	}

	// 6. TXT RECORDS (technologies/policies)
	for _, txt := range resp.TXT {
		techMeta := p.parseTXTRecord(txt)

		artifact := domain.NewArtifactWithMetadata(
			domain.ArtifactTypeTechnology,
			techMeta.Name,
			p.sourceName,
			techMeta,
		)
		artifact.Confidence = 1.0
		artifacts = append(artifacts, artifact)
	}

	// 7. NAMESERVERS (NS records)
	for _, ns := range resp.NS {
		// Clean NS (remove trailing dot)
		ns = strings.TrimSuffix(ns, ".")

		nsMeta := &metadata.DomainMetadata{
			DNSResolved: true,
		}

		artifact := domain.NewArtifactWithMetadata(
			domain.ArtifactTypeNameserver,
			ns,
			p.sourceName,
			nsMeta,
		)
		artifact.Confidence = 1.0
		artifacts = append(artifacts, artifact)
	}

	// 8. PTR RECORDS (reverse DNS -> subdomains)
	for _, ptr := range resp.PTR {
		if ptr == "" {
			continue
		}

		// Clean PTR (remove trailing dot)
		ptr = strings.TrimSuffix(ptr, ".")

		ptrMeta := &metadata.DomainMetadata{
			DNSResolved: true,
		}

		artifact := domain.NewArtifactWithMetadata(
			domain.ArtifactTypeSubdomain,
			ptr,
			p.sourceName,
			ptrMeta,
		)
		artifact.Confidence = 1.0
		artifacts = append(artifacts, artifact)
	}

	return artifacts
}

// parseTXTRecord parses a TXT record and categorizes it.
func (p *Parser) parseTXTRecord(txt string) *metadata.TechnologyMetadata {
	meta := &metadata.TechnologyMetadata{
		DetectionMethod:   "dns_txt",
		DetectionPattern:  txt,
		DetectionLocation: "dns",
		ConfidenceScore:   1.0,
	}

	// Detect category and set name
	txtLower := strings.ToLower(txt)

	if strings.HasPrefix(txtLower, "v=spf") {
		meta.Category = "email_security"
		meta.Subcategory = "spf"
		meta.Name = "SPF Policy"
		meta.DisplayName = "SPF Policy"
		meta.Vendor = "Email Security"
	} else if strings.Contains(txtLower, "v=dkim") || strings.Contains(txtLower, "k=rsa") {
		meta.Category = "email_security"
		meta.Subcategory = "dkim"
		meta.Name = "DKIM Key"
		meta.DisplayName = "DKIM Key"
		meta.Vendor = "Email Security"
	} else if strings.HasPrefix(txtLower, "v=dmarc") {
		meta.Category = "email_security"
		meta.Subcategory = "dmarc"
		meta.Name = "DMARC Policy"
		meta.DisplayName = "DMARC Policy"
		meta.Vendor = "Email Security"
	} else if strings.Contains(txtLower, "-site-verification") || strings.Contains(txtLower, "verification") {
		meta.Category = "verification"
		meta.Subcategory = "domain_verification"
		meta.Name = "Domain Verification"
		meta.DisplayName = "Domain Verification"

		// Detect provider
		if strings.Contains(txtLower, "google") {
			meta.Vendor = "Google"
		} else if strings.Contains(txtLower, "ms=") {
			meta.Vendor = "Microsoft"
		} else if strings.Contains(txtLower, "facebook") {
			meta.Vendor = "Facebook"
		} else {
			meta.Vendor = "Unknown"
		}
	} else if strings.HasPrefix(txtLower, "_globalsign") || strings.HasPrefix(txtLower, "_acme") {
		meta.Category = "certificate"
		meta.Subcategory = "ca_validation"
		meta.Name = "Certificate Validation"
		meta.DisplayName = "Certificate Authority Validation"
		meta.Vendor = "Certificate Authority"
	} else {
		meta.Category = "txt_record"
		meta.Subcategory = "generic"
		meta.Name = "TXT Record"
		meta.DisplayName = "Generic TXT Record"
		meta.Vendor = "DNS"
	}

	return meta
}
