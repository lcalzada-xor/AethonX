package httpx

import (
	"strconv"
	"strings"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/platform/logx"
)

// Parser handles conversion from HTTPXResponse to domain Artifacts.
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

// ParseResponse converts an HTTPXResponse into domain Artifacts.
// Returns multiple artifacts: URL, IP, Technologies, Certificate, Subdomains.
func (p *Parser) ParseResponse(resp *HTTPXResponse, target domain.Target) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0, 6)

	// Skip failed probes (no useful data)
	if resp.Failed || resp.StatusCode == 0 {
		p.logger.Debug("skipping failed probe", "url", resp.URL)
		return artifacts
	}

	// 1. Main artifact: URL with ServiceMetadata
	urlArtifact := p.createURLArtifact(resp)
	artifacts = append(artifacts, urlArtifact)

	// 2. Domain artifact marked as alive (NEW)
	if resp.Host != "" {
		domainArtifact := p.createAliveDomainArtifact(resp)
		artifacts = append(artifacts, domainArtifact)
	}

	// 3. IP artifact (if available)
	if resp.IP != "" {
		ipArtifact := p.createIPArtifact(resp)
		artifacts = append(artifacts, ipArtifact)
	}

	// 4. Technology artifacts (from tech detection)
	for _, tech := range resp.TechDetect {
		techArtifact := p.createTechnologyArtifact(tech, resp.URL)
		artifacts = append(artifacts, techArtifact)
	}

	// 5. Certificate artifact (if TLS data available)
	if resp.TLS != nil && resp.TLS.ProbeStatus {
		certArtifact := p.createCertificateArtifact(resp.TLS)
		artifacts = append(artifacts, certArtifact)

		// Extract SANs as subdomain artifacts
		for _, san := range resp.TLS.SubjectAN {
			if p.isValidDomain(san) {
				sanArtifact := p.createSubdomainArtifact(san, resp.URL)
				artifacts = append(artifacts, sanArtifact)
			}
		}
	}

	// 6. Extracted FQDNs (if -extract-fqdn was used)
	for _, fqdn := range resp.ExtractedFQDNs {
		if p.isValidDomain(fqdn) && fqdn != resp.Host {
			fqdnArtifact := p.createSubdomainArtifact(fqdn, resp.URL)
			artifacts = append(artifacts, fqdnArtifact)
		}
	}

	// 7. CNAME artifact (if different from host)
	if resp.CNAME != "" && resp.CNAME != resp.Host {
		cnameArtifact := domain.NewArtifact(domain.ArtifactTypeDNSRecord, resp.CNAME, p.sourceName)
		targetArtifact := domain.NewArtifact(domain.ArtifactTypeDomain, resp.Host, p.sourceName)
		cnameArtifact.Relations = []domain.ArtifactRelation{
			{
				TargetID: targetArtifact.ID,
				Type:     domain.RelationHasCNAME,
			},
		}
		artifacts = append(artifacts, cnameArtifact)
	}

	p.logger.Debug("parsed httpx response",
		"url", resp.URL,
		"artifacts_count", len(artifacts),
		"status_code", resp.StatusCode,
	)

	return artifacts
}

// createURLArtifact creates a URL artifact with ServiceMetadata.
func (p *Parser) createURLArtifact(resp *HTTPXResponse) *domain.Artifact {
	artifact := domain.NewArtifact(domain.ArtifactTypeURL, resp.URL, p.sourceName)

	// Create ServiceMetadata
	serviceMeta := &metadata.ServiceMetadata{
		Port:            parsePort(resp.Port),
		Protocol:        strings.ToLower(resp.Scheme),
		State:           "open",
		Banner:          resp.Webserver,
		Product:         extractProduct(resp.Webserver),
		Version:         extractVersion(resp.Webserver),
		DetectionMethod: "http_probe",
		Confidence:      1.0,
		ScanTool:        "httpx",
		ParentIP:        resp.IP,
	}

	// Add SSL info if HTTPS
	if resp.Scheme == "https" && resp.TLS != nil {
		serviceMeta.SSLEnabled = true
		serviceMeta.SSLCert = resp.TLS.SubjectCN
	}

	artifact.TypedMetadata = serviceMeta
	artifact.Confidence = 1.0

	// Add relation to parent domain
	if resp.Host != "" {
		// Create target artifact to get its ID
		targetArtifact := domain.NewArtifact(domain.ArtifactTypeDomain, resp.Host, p.sourceName)
		artifact.Relations = []domain.ArtifactRelation{
			{
				TargetID: targetArtifact.ID,
				Type:     domain.RelationHostedOn,
			},
		}
	}

	return artifact
}

// createAliveDomainArtifact creates a domain/subdomain artifact marked as alive.
func (p *Parser) createAliveDomainArtifact(resp *HTTPXResponse) *domain.Artifact {
	// Determine if it's a subdomain or domain
	artifactType := domain.ArtifactTypeDomain
	if strings.Count(resp.Host, ".") > 1 {
		artifactType = domain.ArtifactTypeSubdomain
	}

	artifact := domain.NewArtifact(artifactType, resp.Host, p.sourceName)

	// Create DomainMetadata with alive status
	domainMeta := metadata.NewDomainMetadata()
	domainMeta.IsAlive = true
	domainMeta.ProbeStatus = "alive"
	domainMeta.LastProbed = time.Now().Format(time.RFC3339)
	domainMeta.ProbeSource = "httpx"

	// Add HTTP information
	domainMeta.HTTPStatus = resp.StatusCode
	domainMeta.HTTPTitle = resp.Title
	domainMeta.HTTPServer = resp.Webserver

	// Add redirect if present (use chain if available)
	if resp.StatusCode >= 300 && resp.StatusCode < 400 && len(resp.Chain) > 0 {
		// Get the final URL from the last chain item
		lastChainItem := resp.Chain[len(resp.Chain)-1]
		if lastChainItem.Location != "" {
			domainMeta.HTTPRedirect = lastChainItem.Location
		} else {
			domainMeta.HTTPRedirect = lastChainItem.RequestURL
		}
	}

	// Add SSL information if HTTPS
	if resp.Scheme == "https" && resp.TLS != nil {
		domainMeta.HasSSL = true
		domainMeta.SSLIssuer = resp.TLS.IssuerCN
		domainMeta.SSLValidFrom = resp.TLS.NotBefore
		domainMeta.SSLValidUntil = resp.TLS.NotAfter
		domainMeta.SSLWildcard = resp.TLS.WildcardCert
	}

	// Add CDN/WAF information
	if resp.CDN != "" {
		domainMeta.CDN = resp.CDNName
	}

	artifact.TypedMetadata = domainMeta
	artifact.Confidence = 1.0

	// Add "alive" tag
	artifact.AddTag("alive")

	return artifact
}

// createIPArtifact creates an IP artifact with network metadata.
func (p *Parser) createIPArtifact(resp *HTTPXResponse) *domain.Artifact {
	artifact := domain.NewArtifact(domain.ArtifactTypeIP, resp.IP, p.sourceName)

	ipMeta := metadata.NewIPMetadata()

	// Add ASN data if available
	if resp.ASN != nil {
		ipMeta.ASN = resp.ASN.ASN
		ipMeta.ASOrg = resp.ASN.Org
		ipMeta.Country = resp.ASN.Country
	}

	// Add CDN info
	if resp.CDN != "" {
		ipMeta.CloudProvider = resp.CDNName
	}

	artifact.TypedMetadata = ipMeta
	artifact.Confidence = 1.0

	// Add relation to host
	if resp.Host != "" {
		targetArtifact := domain.NewArtifact(domain.ArtifactTypeDomain, resp.Host, p.sourceName)
		artifact.Relations = []domain.ArtifactRelation{
			{
				TargetID: targetArtifact.ID,
				Type:     domain.RelationResolvesTo,
			},
		}
	}

	return artifact
}

// createTechnologyArtifact creates a technology artifact.
func (p *Parser) createTechnologyArtifact(techName, detectionURL string) *domain.Artifact {
	artifact := domain.NewArtifact(domain.ArtifactTypeTechnology, techName, p.sourceName)

	techMeta := metadata.NewTechnologyMetadata(techName, "")
	techMeta.DetectionMethod = "wappalyzer"
	techMeta.DetectionLocation = detectionURL
	techMeta.ConfidenceScore = 0.9

	artifact.TypedMetadata = techMeta
	artifact.Confidence = 0.9

	// Add relation to URL where it was detected
	targetArtifact := domain.NewArtifact(domain.ArtifactTypeURL, detectionURL, p.sourceName)
	artifact.Relations = []domain.ArtifactRelation{
		{
			TargetID: targetArtifact.ID,
			Type:     domain.RelationUsesTech,
		},
	}

	return artifact
}

// createCertificateArtifact creates a certificate artifact from TLS data.
func (p *Parser) createCertificateArtifact(tls *TLSData) *domain.Artifact {
	// Use SubjectCN as the certificate value
	certValue := tls.SubjectCN
	if certValue == "" {
		certValue = tls.Host
	}

	artifact := domain.NewArtifact(domain.ArtifactTypeCertificate, certValue, p.sourceName)

	certMeta := &metadata.CertificateMetadata{
		IssuerCN:      tls.IssuerCN,
		IssuerFull:    tls.IssuerDN,
		SubjectCN:     tls.SubjectCN,
		SubjectFull:   tls.SubjectDN,
		ValidFrom:     tls.NotBefore,
		ValidUntil:    tls.NotAfter,
		SANDomains:    tls.SubjectAN,
		SerialNumber:  tls.Serial,
		WildcardCert:  tls.WildcardCert,
	}

	// Add fingerprints if available
	if tls.FingerprintHash != nil {
		certMeta.FingerprintSHA256 = tls.FingerprintHash.SHA256
		certMeta.FingerprintSHA1 = tls.FingerprintHash.SHA1
	}

	artifact.TypedMetadata = certMeta
	artifact.Confidence = 1.0

	return artifact
}

// createSubdomainArtifact creates a subdomain artifact.
func (p *Parser) createSubdomainArtifact(subdomain, sourceURL string) *domain.Artifact {
	artifact := domain.NewArtifact(domain.ArtifactTypeSubdomain, subdomain, p.sourceName)
	artifact.Confidence = 0.95

	// Add relation to source URL
	targetArtifact := domain.NewArtifact(domain.ArtifactTypeURL, sourceURL, p.sourceName)
	artifact.Relations = []domain.ArtifactRelation{
		{
			TargetID: targetArtifact.ID,
			Type:     domain.RelationUsesCert,
		},
	}

	return artifact
}

// parsePort extracts port number from port string.
func parsePort(portStr string) int {
	if portStr == "" {
		return 0
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0
	}
	return port
}

// extractProduct extracts product name from server banner.
// Example: "nginx/1.24.0" -> "nginx"
func extractProduct(banner string) string {
	if banner == "" {
		return ""
	}

	// Split by space (e.g., "nginx/1.24.0 (Ubuntu)")
	parts := strings.Fields(banner)
	if len(parts) == 0 {
		return ""
	}

	// Split by slash to get product name
	productParts := strings.Split(parts[0], "/")
	return productParts[0]
}

// extractVersion extracts version from server banner.
// Example: "nginx/1.24.0" -> "1.24.0"
func extractVersion(banner string) string {
	if banner == "" {
		return ""
	}

	// Split by space
	parts := strings.Fields(banner)
	if len(parts) == 0 {
		return ""
	}

	// Split by slash
	productParts := strings.Split(parts[0], "/")
	if len(productParts) < 2 {
		return ""
	}

	return productParts[1]
}

// isValidDomain checks if a string is a valid domain name.
func (p *Parser) isValidDomain(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))

	// Basic validation
	if s == "" || len(s) > 253 {
		return false
	}

	// Must contain at least one dot
	if !strings.Contains(s, ".") {
		return false
	}

	// Check for invalid characters
	for _, char := range s {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '.' || char == '-' || char == '*') {
			return false
		}
	}

	return true
}

// ParseMultipleResponses parses multiple HTTPXResponse objects.
func (p *Parser) ParseMultipleResponses(responses []*HTTPXResponse, target domain.Target) []*domain.Artifact {
	allArtifacts := make([]*domain.Artifact, 0, len(responses)*3)

	for _, resp := range responses {
		artifacts := p.ParseResponse(resp, target)
		allArtifacts = append(allArtifacts, artifacts...)
	}

	p.logger.Info("parsed multiple httpx responses",
		"response_count", len(responses),
		"artifact_count", len(allArtifacts),
	)

	return allArtifacts
}
