// internal/sources/shodan/parser.go
package shodan

import (
	"fmt"
	"strings"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/platform/logx"
)

// Parser converts Shodan API responses into AethonX artifacts.
type Parser struct {
	logger     logx.Logger
	sourceName string
}

// NewParser creates a new Shodan response parser.
func NewParser(logger logx.Logger, sourceName string) *Parser {
	return &Parser{
		logger:     logger.With("component", "shodan-parser"),
		sourceName: sourceName,
	}
}

// ParseHostResponse converts a ShodanHostResponse into multiple artifacts.
// A single host response can generate:
// - IP artifact (with IPMetadata)
// - Subdomain artifacts (from hostnames)
// - Port artifact
// - Service artifact (with ServiceMetadata)
// - Vulnerability artifacts (from vulns list)
// - Certificate artifact (if SSL present)
// - Technology artifacts (from product/version)
// - ASN artifact
func (p *Parser) ParseHostResponse(resp *ShodanHostResponse, target domain.Target) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0, 10)

	// 1. Create IP artifact with rich metadata
	if resp.IPStr != "" {
		ipArtifact := p.createIPArtifact(resp, target)
		if ipArtifact != nil {
			artifacts = append(artifacts, ipArtifact)
		}
	}

	// 2. Create subdomain artifacts from hostnames
	for _, hostname := range resp.Hostnames {
		if hostname != "" && p.isRelevantHostname(hostname, target.Root) {
			subdomainArtifact := domain.NewArtifact(
				domain.ArtifactTypeSubdomain,
				hostname,
				p.sourceName,
			)
			artifacts = append(artifacts, subdomainArtifact)
		}
	}

	// 3. Create subdomain artifacts from domains list
	for _, domainName := range resp.Domains {
		if domainName != "" && p.isRelevantHostname(domainName, target.Root) {
			subdomainArtifact := domain.NewArtifact(
				domain.ArtifactTypeSubdomain,
				domainName,
				p.sourceName,
			)
			artifacts = append(artifacts, subdomainArtifact)
		}
	}

	// 4. Create port artifact
	if resp.Port > 0 {
		portValue := fmt.Sprintf("%s:%d", resp.IPStr, resp.Port)
		portArtifact := domain.NewArtifact(
			domain.ArtifactTypePort,
			portValue,
			p.sourceName,
		)
		artifacts = append(artifacts, portArtifact)
	}

	// 5. Create service artifact with detailed metadata
	if resp.Port > 0 {
		serviceArtifact := p.createServiceArtifact(resp, target)
		if serviceArtifact != nil {
			artifacts = append(artifacts, serviceArtifact)
		}
	}

	// 6. Create vulnerability artifacts
	if len(resp.Vulns) > 0 {
		vulnArtifacts := p.createVulnerabilityArtifacts(resp, target)
		artifacts = append(artifacts, vulnArtifacts...)
	}

	// 7. Create certificate artifact
	if resp.SSL != nil {
		certArtifact := p.createCertificateArtifact(resp, target)
		if certArtifact != nil {
			artifacts = append(artifacts, certArtifact)
		}
	}

	// 8. Create technology artifact
	if resp.Product != "" {
		techArtifact := p.createTechnologyArtifact(resp, target)
		if techArtifact != nil {
			artifacts = append(artifacts, techArtifact)
		}
	}

	// 9. Create ASN artifact
	if resp.ASN != "" {
		asnArtifact := domain.NewArtifact(
			domain.ArtifactTypeASN,
			resp.ASN,
			p.sourceName,
		)
		artifacts = append(artifacts, asnArtifact)
	}

	// 10. Create cloud resource artifact if cloud provider detected
	if resp.Cloud != nil && resp.Cloud.Provider != "" {
		cloudArtifact := p.createCloudArtifact(resp, target)
		if cloudArtifact != nil {
			artifacts = append(artifacts, cloudArtifact)
		}
	}

	p.logger.Debug("parsed host response",
		"ip", resp.IPStr,
		"artifacts", len(artifacts),
	)

	return artifacts
}

// ParseDomainResponse converts a ShodanDomainResponse into an artifact.
func (p *Parser) ParseDomainResponse(resp *ShodanDomainResponse, target domain.Target) *domain.Artifact {
	if resp.Subdomain == "" {
		return nil
	}

	// Construct full subdomain
	fullSubdomain := resp.Subdomain
	if resp.Domain != "" && !strings.HasSuffix(resp.Subdomain, resp.Domain) {
		fullSubdomain = fmt.Sprintf("%s.%s", resp.Subdomain, resp.Domain)
	}

	// Only include if relevant to target
	if !p.isRelevantHostname(fullSubdomain, target.Root) {
		return nil
	}

	artifact := domain.NewArtifact(
		domain.ArtifactTypeSubdomain,
		fullSubdomain,
		p.sourceName,
	)

	p.logger.Debug("parsed domain response", "subdomain", fullSubdomain)

	return artifact
}

// createIPArtifact creates an IP artifact with rich IPMetadata.
func (p *Parser) createIPArtifact(resp *ShodanHostResponse, target domain.Target) *domain.Artifact {
	ipMeta := metadata.NewIPMetadata()

	// Geolocation
	ipMeta.Country = resp.Location.CountryName
	ipMeta.CountryCode = resp.Location.CountryCode
	ipMeta.City = resp.Location.City
	ipMeta.Region = resp.Location.RegionCode
	if resp.Location.Latitude != 0 || resp.Location.Longitude != 0 {
		ipMeta.Latitude = fmt.Sprintf("%.6f", resp.Location.Latitude)
		ipMeta.Longitude = fmt.Sprintf("%.6f", resp.Location.Longitude)
	}

	// Network info
	ipMeta.ASN = resp.ASN
	ipMeta.ASOrg = resp.Org
	ipMeta.ISP = resp.ISP

	// Open ports
	if resp.Port > 0 {
		ipMeta.OpenPorts = []int{resp.Port}
	}

	// Cloud provider
	if resp.Cloud != nil && resp.Cloud.Provider != "" {
		ipMeta.CloudProvider = resp.Cloud.Provider
	}

	// Service summary
	if resp.Product != "" && resp.Port > 0 {
		serviceSummary := metadata.ServiceSummary{
			Port:     resp.Port,
			Protocol: resp.Transport,
			Name:     resp.Product,
			Product:  resp.Product,
			Version:  resp.Version,
		}
		ipMeta.ServicesSummary = []metadata.ServiceSummary{serviceSummary}
	}

	return domain.NewArtifactWithMetadata(
		domain.ArtifactTypeIP,
		resp.IPStr,
		p.sourceName,
		ipMeta,
	)
}

// createServiceArtifact creates a Service artifact with ServiceMetadata.
func (p *Parser) createServiceArtifact(resp *ShodanHostResponse, target domain.Target) *domain.Artifact {
	serviceName := resp.Product
	if serviceName == "" {
		serviceName = "unknown"
	}

	serviceMeta := metadata.NewServiceMetadata(serviceName, resp.Port)
	serviceMeta.Product = resp.Product
	serviceMeta.Version = resp.Version
	serviceMeta.Protocol = resp.Transport
	serviceMeta.State = "open"
	serviceMeta.Banner = resp.Banner
	serviceMeta.ParentIP = resp.IPStr
	serviceMeta.ScanTool = "shodan"
	serviceMeta.ExtraInfo = resp.OS

	// CPE
	if len(resp.CPE) > 0 {
		serviceMeta.CPE = resp.CPE[0]
	}

	// Vulnerabilities
	if len(resp.Vulns) > 0 {
		serviceMeta.HasVulns = true
		serviceMeta.CVEList = resp.Vulns
		// Set risk level based on vulnerability count
		serviceMeta.RiskLevel = p.inferRiskLevel(len(resp.Vulns))
	}

	value := fmt.Sprintf("%s:%d", resp.IPStr, resp.Port)
	return domain.NewArtifactWithMetadata(
		domain.ArtifactTypeService,
		value,
		p.sourceName,
		serviceMeta,
	)
}

// createVulnerabilityArtifacts creates Vulnerability artifacts from CVE list.
func (p *Parser) createVulnerabilityArtifacts(resp *ShodanHostResponse, target domain.Target) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0, len(resp.Vulns))

	for _, cve := range resp.Vulns {
		if cve == "" {
			continue
		}

		vulnMeta := NewVulnerabilityMetadata(cve)
		vulnMeta.Severity = InferSeverityFromCVE(cve)
		vulnMeta.AffectedPorts = []int{resp.Port}
		vulnMeta.DiscoveryTool = "shodan"

		artifact := domain.NewArtifactWithMetadata(
			domain.ArtifactTypeVulnerability,
			cve,
			p.sourceName,
			vulnMeta,
		)

		artifacts = append(artifacts, artifact)
	}

	return artifacts
}

// createCertificateArtifact creates a Certificate artifact from SSL data.
func (p *Parser) createCertificateArtifact(resp *ShodanHostResponse, target domain.Target) *domain.Artifact {
	if resp.SSL == nil || resp.SSL.Cert.Subject.CN == "" {
		return nil
	}

	certMeta := &metadata.CertificateMetadata{}
	certMeta.SubjectCN = resp.SSL.Cert.Subject.CN
	certMeta.IssuerCN = resp.SSL.Cert.Issuer.CN
	certMeta.SerialNumber = resp.SSL.Cert.Serial
	certMeta.ValidUntil = resp.SSL.Cert.Expires
	certMeta.ValidFrom = resp.SSL.Cert.Issued
	certMeta.CertExpired = resp.SSL.Cert.Expired

	// Fingerprints
	if resp.SSL.Cert.Fingerprint.SHA1 != "" {
		certMeta.FingerprintSHA1 = resp.SSL.Cert.Fingerprint.SHA1
	}
	if resp.SSL.Cert.Fingerprint.SHA256 != "" {
		certMeta.FingerprintSHA256 = resp.SSL.Cert.Fingerprint.SHA256
	}

	// Organization info
	if resp.SSL.Cert.Subject.O != "" {
		certMeta.SubjectO = resp.SSL.Cert.Subject.O
	}
	if resp.SSL.Cert.Issuer.O != "" {
		certMeta.IssuerO = resp.SSL.Cert.Issuer.O
	}

	return domain.NewArtifactWithMetadata(
		domain.ArtifactTypeCertificate,
		resp.SSL.Cert.Subject.CN,
		p.sourceName,
		certMeta,
	)
}

// createTechnologyArtifact creates a Technology artifact from product/version.
func (p *Parser) createTechnologyArtifact(resp *ShodanHostResponse, target domain.Target) *domain.Artifact {
	techMeta := metadata.NewTechnologyMetadata(resp.Product, resp.Version)

	// CPE
	if len(resp.CPE) > 0 {
		techMeta.CPE = resp.CPE[0]
	}

	// Detection confidence (Shodan is highly accurate)
	techMeta.ConfidenceScore = 0.95
	techMeta.DetectionMethod = "shodan"

	value := resp.Product
	if resp.Version != "" {
		value = fmt.Sprintf("%s %s", resp.Product, resp.Version)
	}

	return domain.NewArtifactWithMetadata(
		domain.ArtifactTypeTechnology,
		value,
		p.sourceName,
		techMeta,
	)
}

// createCloudArtifact creates a CloudResource artifact.
func (p *Parser) createCloudArtifact(resp *ShodanHostResponse, target domain.Target) *domain.Artifact {
	cloudMeta := NewCloudMetadata(resp.Cloud.Provider)
	cloudMeta.Service = resp.Cloud.Service
	cloudMeta.Region = resp.Cloud.Region
	cloudMeta.Tags = resp.Tags

	value := fmt.Sprintf("%s:%s", resp.Cloud.Provider, resp.IPStr)

	return domain.NewArtifactWithMetadata(
		domain.ArtifactTypeCloudResource,
		value,
		p.sourceName,
		cloudMeta,
	)
}

// isRelevantHostname checks if a hostname is relevant to the target domain.
func (p *Parser) isRelevantHostname(hostname, targetRoot string) bool {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	targetRoot = strings.ToLower(strings.TrimSpace(targetRoot))

	if hostname == "" || targetRoot == "" {
		return false
	}

	// Exact match
	if hostname == targetRoot {
		return true
	}

	// Subdomain match
	if strings.HasSuffix(hostname, "."+targetRoot) {
		return true
	}

	return false
}

// inferRiskLevel infers risk level from vulnerability count.
func (p *Parser) inferRiskLevel(vulnCount int) string {
	switch {
	case vulnCount >= 10:
		return "critical"
	case vulnCount >= 5:
		return "high"
	case vulnCount >= 2:
		return "medium"
	case vulnCount >= 1:
		return "low"
	default:
		return "info"
	}
}
