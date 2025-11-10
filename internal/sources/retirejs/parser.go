package retirejs

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/platform/logx"
)

// RetireJSOutput matches retire --outputformat jsonsimple
type RetireJSOutput []struct {
	File    string          `json:"file"`
	Results []VulnerableLib `json:"results"`
}

// VulnerableLib representa una librería JavaScript vulnerable
type VulnerableLib struct {
	Version         string          `json:"version"`
	Component       string          `json:"component"`
	NPMName         string          `json:"npmname"`
	Detection       string          `json:"detection"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	Licenses        []string        `json:"licenses"`
}

// Vulnerability representa una vulnerabilidad detectada
type Vulnerability struct {
	Info        []string        `json:"info"`
	Below       string          `json:"below"`
	AtOrAbove   string          `json:"atOrAbove"`
	Severity    string          `json:"severity"`
	Identifiers VulnIdentifiers `json:"identifiers"`
	CWE         []string        `json:"cwe"`
}

// VulnIdentifiers contiene identificadores de vulnerabilidad
type VulnIdentifiers struct {
	Summary  string   `json:"summary"`
	CVE      []string `json:"CVE"`
	GitHubID string   `json:"githubID"`
	Bug      string   `json:"bug"`
	Issue    string   `json:"issue"`
	RETID    string   `json:"retid"`
	PR       string   `json:"PR"`
}

// Parser parsea el output JSON de retire.js
type Parser struct {
	logger     logx.Logger
	sourceName string
}

// NewParser crea un nuevo parser de retire.js
func NewParser(logger logx.Logger, sourceName string) *Parser {
	return &Parser{
		logger:     logger,
		sourceName: sourceName,
	}
}

// Parse parsea el output JSON de retire.js y retorna artifacts
func (p *Parser) Parse(data []byte) ([]*domain.Artifact, error) {
	if len(data) == 0 {
		return []*domain.Artifact{}, nil
	}

	var output RetireJSOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("failed to unmarshal retire.js output: %w", err)
	}

	var artifacts []*domain.Artifact

	for _, fileResult := range output {
		for _, lib := range fileResult.Results {
			// Crear artifact de JavaScript
			jsArtifact := p.createJavaScriptArtifact(fileResult.File, lib)
			artifacts = append(artifacts, jsArtifact)

			// Crear artifact de Technology
			techArtifact := p.createTechnologyArtifact(lib)
			artifacts = append(artifacts, techArtifact)

			// Crear artifact por cada vulnerabilidad
			for _, vuln := range lib.Vulnerabilities {
				vulnArtifact := p.createVulnerabilityArtifact(fileResult.File, lib, vuln)
				artifacts = append(artifacts, vulnArtifact)
			}
		}
	}

	p.logger.Info("parsed retire.js output",
		"files", len(output),
		"artifacts", len(artifacts),
	)

	return artifacts, nil
}

// createJavaScriptArtifact crea un artifact de tipo JavaScript
func (p *Parser) createJavaScriptArtifact(filePath string, lib VulnerableLib) *domain.Artifact {
	highestSeverity := p.getHighestSeverity(lib.Vulnerabilities)

	jsMeta := &metadata.JavaScriptMetadata{
		FilePath:        filePath,
		Component:       lib.Component,
		Version:         lib.Version,
		DetectionMethod: lib.Detection,
		VulnCount:       len(lib.Vulnerabilities),
		HighestSeverity: highestSeverity,
		IsMinified:      p.isMinified(filePath),
		NPMName:         lib.NPMName,
		Licenses:        lib.Licenses,
	}

	if len(lib.Licenses) > 0 {
		jsMeta.License = lib.Licenses[0]
	}

	artifact := domain.NewArtifactWithMetadata(
		domain.ArtifactTypeJavaScript,
		filepath.Base(filePath),
		p.sourceName,
		jsMeta,
	)

	// Discovery context
	artifact.SetDiscoveryContext(&domain.DiscoveryContext{
		SourceResource: filePath,
		MatchPattern:   lib.Detection,
	})

	// Security context
	severity := p.mapSeverityToDomain(highestSeverity)
	secCtx := domain.NewSecurityContext().
		WithSeverity(severity).
		WithSensitive(len(lib.Vulnerabilities) > 0)

	artifact.SetSecurityContext(secCtx)

	return artifact
}

// createTechnologyArtifact crea un artifact de tipo Technology
func (p *Parser) createTechnologyArtifact(lib VulnerableLib) *domain.Artifact {
	techMeta := &metadata.TechnologyMetadata{
		Name:            lib.Component,
		Version:         lib.Version,
		Category:        "JavaScript Library",
		DetectionMethod: "retire.js",
	}

	artifact := domain.NewArtifactWithMetadata(
		domain.ArtifactTypeTechnology,
		lib.Component,
		p.sourceName,
		techMeta,
	)

	return artifact
}

// createVulnerabilityArtifact crea un artifact de tipo Vulnerability
func (p *Parser) createVulnerabilityArtifact(filePath string, lib VulnerableLib, vuln Vulnerability) *domain.Artifact {
	// Extraer primer CVE ID si existe
	cveID := "UNKNOWN"
	if len(vuln.Identifiers.CVE) > 0 {
		cveID = vuln.Identifiers.CVE[0]
	} else if vuln.Identifiers.GitHubID != "" {
		cveID = vuln.Identifiers.GitHubID
	} else if vuln.Identifiers.RETID != "" {
		cveID = "RETID-" + vuln.Identifiers.RETID
	}

	// Convert info strings to References
	refs := make([]metadata.Reference, 0, len(vuln.Info))
	for _, infoURL := range vuln.Info {
		refs = append(refs, metadata.Reference{
			URL:    infoURL,
			Source: "retire.js",
		})
	}

	vulnMeta := &metadata.VulnerabilityMetadata{
		CVE:              cveID,
		Description:      vuln.Identifiers.Summary,
		CVSSv3Severity:   strings.ToUpper(vuln.Severity),
		CWEs:             vuln.CWE,
		References:       refs,
		DiscoveryTool:    "retire.js (" + lib.Component + " " + lib.Version + ")",
		EnrichmentSource: "retire.js",
	}

	// Mapear severity a CVSS score aproximado
	vulnMeta.CVSSv3Score = p.severityToCVSSScore(vuln.Severity)

	artifact := domain.NewArtifactWithMetadata(
		domain.ArtifactTypeVulnerability,
		cveID,
		p.sourceName,
		vulnMeta,
	)

	// Discovery context
	artifact.SetDiscoveryContext(&domain.DiscoveryContext{
		SourceResource: filepath.Base(filePath),
		Context:        fmt.Sprintf("%s %s", lib.Component, lib.Version),
	})

	// Security context
	severity := p.mapSeverityToDomain(vuln.Severity)
	secCtx := domain.NewSecurityContext().
		WithSeverity(severity).
		WithSensitive(true)

	// Determinar vulnerability types basados en CWE
	vulnTypes := p.determineVulnerabilityTypes(vuln.CWE)
	for _, vt := range vulnTypes {
		secCtx = secCtx.WithVulnerabilityType(vt)
	}

	artifact.SetSecurityContext(secCtx)

	return artifact
}

// getHighestSeverity retorna la severidad más alta de una lista de vulnerabilidades
func (p *Parser) getHighestSeverity(vulns []Vulnerability) string {
	if len(vulns) == 0 {
		return "none"
	}

	severityOrder := map[string]int{
		"critical": 4,
		"high":     3,
		"medium":   2,
		"low":      1,
		"none":     0,
	}

	highest := "none"
	highestValue := 0

	for _, vuln := range vulns {
		if val, ok := severityOrder[strings.ToLower(vuln.Severity)]; ok {
			if val > highestValue {
				highestValue = val
				highest = strings.ToLower(vuln.Severity)
			}
		}
	}

	return highest
}

// mapSeverityToDomain mapea severity de retire.js a domain.Severity
func (p *Parser) mapSeverityToDomain(severity string) domain.Severity {
	switch strings.ToLower(severity) {
	case "critical":
		return domain.SeverityCritical
	case "high":
		return domain.SeverityHigh
	case "medium":
		return domain.SeverityMedium
	case "low":
		return domain.SeverityLow
	default:
		return domain.SeverityInfo
	}
}

// severityToCVSSScore convierte severity a CVSS score aproximado
func (p *Parser) severityToCVSSScore(severity string) float64 {
	switch strings.ToLower(severity) {
	case "critical":
		return 9.0
	case "high":
		return 7.5
	case "medium":
		return 5.0
	case "low":
		return 3.0
	default:
		return 0.0
	}
}

// determineVulnerabilityTypes determina tipos de vulnerabilidad basados en CWE
func (p *Parser) determineVulnerabilityTypes(cwes []string) []domain.VulnerabilityType {
	var types []domain.VulnerabilityType
	seen := make(map[domain.VulnerabilityType]bool)

	for _, cwe := range cwes {
		var vType domain.VulnerabilityType

		switch cwe {
		case "CWE-79", "CWE-80", "CWE-83", "CWE-87":
			vType = domain.VulnTypeXSS
		case "CWE-89":
			vType = domain.VulnTypeSQLi
		case "CWE-1321", "CWE-915":
			// Prototype pollution - mapear a genérico
			continue
		case "CWE-400", "CWE-770":
			// DoS, resource exhaustion - no hay tipo específico
			continue
		default:
			continue
		}

		if !seen[vType] {
			types = append(types, vType)
			seen[vType] = true
		}
	}

	return types
}

// buildVersionRange construye el rango de versiones afectadas
func (p *Parser) buildVersionRange(vuln Vulnerability) string {
	parts := []string{}

	if vuln.AtOrAbove != "" {
		parts = append(parts, ">="+vuln.AtOrAbove)
	}

	if vuln.Below != "" {
		parts = append(parts, "<"+vuln.Below)
	}

	if len(parts) == 0 {
		return "all"
	}

	return strings.Join(parts, " ")
}

// isMinified detecta si un archivo está minificado
func (p *Parser) isMinified(filePath string) bool {
	base := filepath.Base(filePath)
	return strings.Contains(base, ".min.") || strings.HasSuffix(base, ".min.js")
}
