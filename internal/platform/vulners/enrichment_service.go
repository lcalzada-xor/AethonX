// internal/platform/vulners/enrichment_service.go
package vulners

import (
	"context"
	"fmt"
	"sync"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/platform/logx"
)

// ServiceVulnEnrichmentService enriches Service artifacts with vulnerability data from Vulners.com.
type ServiceVulnEnrichmentService struct {
	client         *VulnersClient
	logger         logx.Logger
	maxConcurrent  int           // Max concurrent API requests
	timeout        time.Duration // Timeout per enrichment request
	enabled        bool          // Whether enrichment is enabled
}

// NewServiceVulnEnrichmentService creates a new vulnerability enrichment service.
func NewServiceVulnEnrichmentService(logger logx.Logger, apiKey string, maxConcurrent int, timeout time.Duration, enabled bool) *ServiceVulnEnrichmentService {
	return &ServiceVulnEnrichmentService{
		client:        NewVulnersClient(logger, apiKey),
		logger:        logger.With("component", "vuln_enrichment"),
		maxConcurrent: maxConcurrent,
		timeout:       timeout,
		enabled:       enabled,
	}
}

// EnrichServices enriches Service artifacts with CVE data from Vulners.com.
// It processes artifacts concurrently (up to maxConcurrent) and modifies them in-place.
func (s *ServiceVulnEnrichmentService) EnrichServices(artifacts []*domain.Artifact) {
	if !s.enabled {
		s.logger.Debug("vulnerability enrichment disabled, skipping")
		return
	}

	if len(artifacts) == 0 {
		return
	}

	s.logger.Info("starting vulnerability enrichment", "artifacts", len(artifacts))
	startTime := time.Now()

	// Filter for Service artifacts with Product and Version
	servicesToEnrich := make([]*domain.Artifact, 0)
	for _, artifact := range artifacts {
		if artifact.Type != domain.ArtifactTypeService {
			continue
		}

		serviceMeta, ok := artifact.TypedMetadata.(*metadata.ServiceMetadata)
		if !ok || serviceMeta.Product == "" || serviceMeta.Version == "" {
			s.logger.Debug("skipping service without product/version",
				"artifact_id", artifact.ID,
				"value", artifact.Value,
			)
			continue
		}

		servicesToEnrich = append(servicesToEnrich, artifact)
	}

	if len(servicesToEnrich) == 0 {
		s.logger.Info("no services to enrich (no Product/Version metadata)")
		return
	}

	s.logger.Info("enriching services", "count", len(servicesToEnrich))

	// Create semaphore for concurrent control
	sem := make(chan struct{}, s.maxConcurrent)
	var wg sync.WaitGroup

	// Enrich each service concurrently
	for _, artifact := range servicesToEnrich {
		wg.Add(1)
		go func(art *domain.Artifact) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			s.enrichService(art)
		}(artifact)
	}

	// Wait for all enrichments to complete
	wg.Wait()

	duration := time.Since(startTime)
	s.logger.Info("vulnerability enrichment completed",
		"duration", duration.String(),
		"services_enriched", len(servicesToEnrich),
	)
}

// enrichService enriches a single Service artifact with CVE data.
func (s *ServiceVulnEnrichmentService) enrichService(artifact *domain.Artifact) {
	serviceMeta, ok := artifact.TypedMetadata.(*metadata.ServiceMetadata)
	if !ok {
		return
	}

	product := serviceMeta.Product
	version := serviceMeta.Version

	s.logger.Debug("enriching service",
		"product", product,
		"version", version,
		"artifact_id", artifact.ID,
	)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	// Query Vulners API
	cves, err := s.client.SearchVulnerabilities(ctx, product, version)
	if err != nil {
		s.logger.Warn("failed to enrich service",
			"product", product,
			"version", version,
			"error", err.Error(),
		)
		return
	}

	// If no CVEs found, mark as clean and return
	if len(cves) == 0 {
		serviceMeta.HasVulns = false
		serviceMeta.RiskLevel = "none"
		s.logger.Debug("no vulnerabilities found",
			"product", product,
			"version", version,
		)
		return
	}

	// Update service metadata with CVE data
	serviceMeta.HasVulns = true
	serviceMeta.CVEList = extractCVEIDs(cves)
	serviceMeta.RiskLevel = calculateRiskLevel(cves)

	// Add vulnerability tags to artifact
	artifact.AddTag(fmt.Sprintf("vulns:%d", len(cves)))
	artifact.AddTag(fmt.Sprintf("risk:%s", serviceMeta.RiskLevel))

	// Tag critical/high severity
	if serviceMeta.RiskLevel == "critical" || serviceMeta.RiskLevel == "high" {
		artifact.AddTag("high-risk")
	}

	// Tag if exploits exist
	exploitCVEs := filterWithExploits(cves)
	if len(exploitCVEs) > 0 {
		artifact.AddTag(fmt.Sprintf("exploits:%d", len(exploitCVEs)))
	}

	s.logger.Info("service enriched",
		"product", product,
		"version", version,
		"cves", len(cves),
		"risk", serviceMeta.RiskLevel,
		"exploits", len(exploitCVEs),
	)
}

// Close cleans up resources.
func (s *ServiceVulnEnrichmentService) Close() error {
	return s.client.Close()
}
