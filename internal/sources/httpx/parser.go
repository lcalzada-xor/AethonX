package httpx

import (
	"fmt"
	"net/url"
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

// extractHostname extracts the hostname from HTTPXResponse.
// Priority: Input > URL parsing > empty string
// Note: resp.Host contains the resolved IP, not the hostname.
func (p *Parser) extractHostname(resp *HTTPXResponse) string {
	// First priority: Input field (original target)
	if resp.Input != "" {
		// Input might be a URL or just a domain
		// If it starts with http:// or https://, parse it
		if strings.HasPrefix(resp.Input, "http://") || strings.HasPrefix(resp.Input, "https://") {
			if parsed, err := url.Parse(resp.Input); err == nil && parsed.Host != "" {
				return parsed.Host
			}
		}
		// Otherwise, use Input as-is (it's a domain)
		return resp.Input
	}

	// Second priority: parse URL field
	if resp.URL != "" {
		if parsed, err := url.Parse(resp.URL); err == nil && parsed.Host != "" {
			return parsed.Host
		}
	}

	return ""
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

	// Extract hostname once (resp.Host contains IP, not hostname)
	hostname := p.extractHostname(resp)

	// 1. Main artifact: URL with ServiceMetadata
	urlArtifact := p.createURLArtifact(resp, hostname)
	artifacts = append(artifacts, urlArtifact)

	// 2. Domain artifact marked as alive
	if hostname != "" {
		domainArtifact := p.createAliveDomainArtifact(resp, hostname)
		artifacts = append(artifacts, domainArtifact)
	}

	// 3. IP artifact (resp.Host contains resolved IP)
	if resp.Host != "" {
		ipArtifact := p.createIPArtifact(resp, hostname)
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
		if p.isValidDomain(fqdn) && fqdn != hostname {
			fqdnArtifact := p.createSubdomainArtifact(fqdn, resp.URL)
			artifacts = append(artifacts, fqdnArtifact)
		}
	}

	// 7. CNAME artifact (if different from hostname)
	cnameValue := resp.CNAME.String()
	if !resp.CNAME.IsEmpty() && cnameValue != hostname && hostname != "" {
		cnameArtifact := domain.NewArtifact(domain.ArtifactTypeDNSRecord, cnameValue, p.sourceName)
		targetArtifact := domain.NewArtifact(domain.ArtifactTypeDomain, hostname, p.sourceName)
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
func (p *Parser) createURLArtifact(resp *HTTPXResponse, hostname string) *domain.Artifact {
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
		ParentIP:        resp.Host, // resp.Host contains the resolved IP
	}

	// Add SSL info if HTTPS
	if resp.Scheme == "https" && resp.TLS != nil {
		serviceMeta.SSLEnabled = true
		serviceMeta.SSLCert = resp.TLS.SubjectCN
	}

	artifact.TypedMetadata = serviceMeta
	artifact.Confidence = 1.0

	// Add status-based tags to URL artifact
	p.addStatusTags(artifact, resp.StatusCode)

	// Add relation to parent domain
	if hostname != "" {
		// Create target artifact to get its ID
		targetArtifact := domain.NewArtifact(domain.ArtifactTypeDomain, hostname, p.sourceName)
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
func (p *Parser) createAliveDomainArtifact(resp *HTTPXResponse, hostname string) *domain.Artifact {
	// Determine if it's a subdomain or domain
	artifactType := domain.ArtifactTypeDomain
	if strings.Count(hostname, ".") > 1 {
		artifactType = domain.ArtifactTypeSubdomain
	}

	artifact := domain.NewArtifact(artifactType, hostname, p.sourceName)

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
	if resp.CDN.Bool() {
		domainMeta.CDN = resp.CDNName
	}

	artifact.TypedMetadata = domainMeta
	artifact.Confidence = 1.0

	// Add status-based tags
	p.addStatusTags(artifact, resp.StatusCode)

	return artifact
}

// addStatusTags adds tags to artifacts based on HTTP status code.
func (p *Parser) addStatusTags(artifact *domain.Artifact, statusCode int) {
	switch {
	case statusCode >= 200 && statusCode < 300:
		artifact.AddTag("alive")
		artifact.AddTag("http-success")
	case statusCode >= 300 && statusCode < 400:
		artifact.AddTag("alive")
		artifact.AddTag("http-redirect")
	case statusCode == 401:
		artifact.AddTag("alive")
		artifact.AddTag("http-auth-required")
	case statusCode == 403:
		artifact.AddTag("alive")
		artifact.AddTag("http-forbidden")
	case statusCode == 404:
		artifact.AddTag("dead")
		artifact.AddTag("http-not-found")
	case statusCode >= 400 && statusCode < 500:
		artifact.AddTag("dead")
		artifact.AddTag("http-client-error")
	case statusCode >= 500 && statusCode < 600:
		artifact.AddTag("alive")
		artifact.AddTag("http-server-error")
	default:
		if statusCode > 0 {
			artifact.AddTag("alive")
		}
	}
}

// createIPArtifact creates an IP artifact with network metadata.
func (p *Parser) createIPArtifact(resp *HTTPXResponse, hostname string) *domain.Artifact {
	artifact := domain.NewArtifact(domain.ArtifactTypeIP, resp.Host, p.sourceName) // resp.Host contains the resolved IP

	ipMeta := metadata.NewIPMetadata()

	// Add ASN data if available
	if resp.ASN != nil {
		ipMeta.ASN = resp.ASN.ASN
		ipMeta.ASOrg = resp.ASN.Org
		ipMeta.Country = resp.ASN.Country
	}

	// Add CDN info
	if resp.CDN.Bool() {
		ipMeta.CloudProvider = resp.CDNName
	}

	artifact.TypedMetadata = ipMeta
	artifact.Confidence = 1.0

	// Add relation to hostname
	if hostname != "" {
		targetArtifact := domain.NewArtifact(domain.ArtifactTypeDomain, hostname, p.sourceName)
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
// Parses techName in format "name:version" (e.g., "jQuery:3.6.0") or just "name".
func (p *Parser) createTechnologyArtifact(techName, detectionURL string) *domain.Artifact {
	// Parse tech name and version from format "tech:version"
	name, version := parseTechNameAndVersion(techName)

	artifact := domain.NewArtifact(domain.ArtifactTypeTechnology, name, p.sourceName)

	techMeta := metadata.NewTechnologyMetadata(name, version)
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

// parseTechNameAndVersion extracts technology name and version from format "name:version".
// Examples:
//   - "jQuery:3.6.0" -> ("jQuery", "3.6.0")
//   - "jQuery" -> ("jQuery", "")
//   - "nginx:1.24.0" -> ("nginx", "1.24.0")
func parseTechNameAndVersion(techName string) (name, version string) {
	techName = strings.TrimSpace(techName)
	if techName == "" {
		return "", ""
	}

	// Split by colon to separate name and version
	parts := strings.SplitN(techName, ":", 2)
	name = parts[0]

	if len(parts) == 2 {
		version = strings.TrimSpace(parts[1])
	}

	return name, version
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

// ParseMultipleResponsesWithInput parses multiple HTTPXResponse objects and upgrades confidence
// for artifacts that were verified alive (status 200-299).
func (p *Parser) ParseMultipleResponsesWithInput(responses []*HTTPXResponse, target domain.Target, inputArtifacts []*domain.Artifact) []*domain.Artifact {
	allArtifacts := make([]*domain.Artifact, 0, len(responses)*3)

	// Build map of input artifacts by URL for quick lookup
	inputMap := make(map[string]*domain.Artifact)
	for _, artifact := range inputArtifacts {
		if artifact.Type == domain.ArtifactTypeURL {
			inputMap[artifact.Value] = artifact
		}
	}

	// Statistics tracking
	var stats struct {
		upgradedCount int
		aliveCount    int
		deadCount     int
		newURLs       int
	}

	for _, resp := range responses {
		artifacts := p.ParseResponse(resp, target)

		// Check each artifact for confidence upgrade
		for _, artifact := range artifacts {
			// Only upgrade URL artifacts
			if artifact.Type == domain.ArtifactTypeURL {
				// Check if this URL was from input (low confidence)
				if inputArtifact, exists := inputMap[resp.URL]; exists {
					if inputArtifact.Confidence < domain.ConfidenceVerified {
						// Upgrade confidence if alive (status 200-299)
						if resp.StatusCode >= 200 && resp.StatusCode < 300 {
							artifact.Confidence = domain.ConfidenceVerified
							stats.upgradedCount++
							stats.aliveCount++
							p.logger.Debug("upgraded confidence for verified URL",
								"url", resp.URL,
								"old_confidence", inputArtifact.Confidence,
								"new_confidence", domain.ConfidenceVerified,
								"status_code", resp.StatusCode,
							)
						} else {
							// Keep original low confidence (dead URL)
							artifact.Confidence = inputArtifact.Confidence
							stats.deadCount++
							p.logger.Debug("keeping low confidence for dead URL",
								"url", resp.URL,
								"confidence", inputArtifact.Confidence,
								"status_code", resp.StatusCode,
							)
						}
					} else {
						// Already high confidence, keep verified
						artifact.Confidence = domain.ConfidenceVerified
						if resp.StatusCode >= 200 && resp.StatusCode < 300 {
							stats.aliveCount++
						}
					}
				} else {
					// New URL, set verified confidence
					artifact.Confidence = domain.ConfidenceVerified
					stats.newURLs++
					if resp.StatusCode >= 200 && resp.StatusCode < 300 {
						stats.aliveCount++
					}
				}
			}

			allArtifacts = append(allArtifacts, artifact)
		}
	}

	// Calculate verification rate
	totalVerified := stats.aliveCount + stats.deadCount
	var aliveRate float64
	if totalVerified > 0 {
		aliveRate = float64(stats.aliveCount) / float64(totalVerified) * 100
	}

	p.logger.Info("parsed httpx responses with confidence upgrade",
		"response_count", len(responses),
		"artifact_count", len(allArtifacts),
		"upgraded_count", stats.upgradedCount,
		"alive_count", stats.aliveCount,
		"dead_count", stats.deadCount,
		"new_urls", stats.newURLs,
		"alive_rate", fmt.Sprintf("%.1f%%", aliveRate),
	)

	return allArtifacts
}
