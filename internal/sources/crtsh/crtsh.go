// internal/sources/crtsh/crtsh.go
package crtsh

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/httpclient"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/registry"
)

// Auto-registro de la source al importar el package
func init() {
	if err := registry.Global().Register(
		"crtsh",
		func(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
			return New(logger), nil
		},
		ports.SourceMetadata{
			Name:         "crtsh",
			Description:  "Certificate Transparency log search via crt.sh",
			Version:      "1.0.0",
			Author:       "AethonX",
			Mode:         domain.SourceModePassive,
			Type:         domain.SourceTypeAPI,
			RequiresAuth: false,
			RateLimit:    0, // No documented rate limit

			// Dependency declaration (Stage 0: sin inputs)
			InputArtifacts:  []domain.ArtifactType{}, // Sin inputs = Stage 0
			OutputArtifacts: []domain.ArtifactType{
				domain.ArtifactTypeSubdomain,
				domain.ArtifactTypeCertificate,
			},
			Priority:  10, // Alta prioridad (passive discovery)
			StageHint: 0,  // Stage 0 explícito
		},
	); err != nil {
		// Log error but don't panic - allow application to start
		// Registry will skip this source during Build()
		logx.New().Warn("failed to register crtsh source", "error", err.Error())
	}
}

// CRT implementa una fuente que consulta la base de datos crt.sh
// para descubrir certificados SSL/TLS y subdominios asociados.
type CRT struct {
	client httpclient.Client
	logger logx.Logger
}

// New crea una nueva instancia de la fuente crt.sh con resilience completa.
func New(logger logx.Logger) ports.Source {
	// Configuración específica para crt.sh
	httpConfig := httpclient.Config{
		Timeout:          30 * time.Second,
		MaxRetries:       3,
		RetryBackoff:     2 * time.Second,
		MaxRetryBackoff:  30 * time.Second,
		UserAgent:        "AethonX/1.0 (RDAP-like reconnaissance tool; +https://github.com/yourusername/aethonx)",
		RateLimit:        2.0, // 2 req/s - ser respetuoso con crt.sh
		RateLimitBurst:   1,
	}

	return &CRT{
		client: *httpclient.New(httpConfig, logger),
		logger: logger.With("source", "crtsh"),
	}
}

// Name retorna el nombre de la fuente.
func (c *CRT) Name() string {
	return "crtsh"
}

// Mode retorna el modo de operación (pasivo).
func (c *CRT) Mode() domain.SourceMode {
	return domain.SourceModePassive
}

// Type retorna el tipo de fuente (API).
func (c *CRT) Type() domain.SourceType {
	return domain.SourceTypeAPI
}

// Run ejecuta la fuente contra el target.
func (c *CRT) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	c.logger.Debug("starting crtsh scan", "target", target.Root)

	result := domain.NewScanResult(target)
	result.Metadata.SourcesUsed = []string{c.Name()}

	// Construir URL de la API
	url := fmt.Sprintf("https://crt.sh/?q=%%25.%s&output=json", target.Root)

	// Fetch JSON usando httpx.Client (con retry, rate limiting, etc.)
	body, err := c.client.FetchJSON(ctx, url)
	if err != nil {
		errMsg := fmt.Sprintf("HTTP request failed: %v", err)
		result.AddError(c.Name(), errMsg, false) // No fatal - el scan puede continuar
		c.logger.Warn("crtsh request failed", "target", target.Root, "error", err.Error())
		return result, err
	}

	// Parsear JSON
	var records []certRecord
	if err := json.Unmarshal(body, &records); err != nil {
		// Si falla el parsing, puede ser que crt.sh devolvió HTML/error
		result.AddWarning(c.Name(), fmt.Sprintf("failed to parse JSON: %v", err))
		return result, nil
	}

	c.logger.Debug("parsed crtsh records", "count", len(records))

	// Procesar records y extraer subdominios
	artifacts := c.processRecords(records, target)

	// Añadir artifacts al resultado
	for _, a := range artifacts {
		result.AddArtifact(a)
	}

	c.logger.Info("crtsh scan completed",
		"target", target.Root,
		"artifacts", len(artifacts),
	)

	return result, nil
}

// processRecords procesa los registros de certificados y extrae artifacts.
func (c *CRT) processRecords(records []certRecord, target domain.Target) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0)

	for _, record := range records {
		// name_value puede contener múltiples dominios separados por \n
		hosts := strings.Split(record.NameValue, "\n")

		for _, host := range hosts {
			host = strings.TrimSpace(host)
			if host == "" {
				continue
			}

			// Verificar que el host está en scope
			if !target.IsInScope(host) {
				continue
			}

			// Crear metadata de certificado
			certMeta := &metadata.CertificateMetadata{
				IssuerCN:    record.IssuerName,
				ValidUntil:  record.NotAfter,
				ValidFrom:   record.NotBefore,
				SerialNumber: record.SerialNumber,
			}

			// Crear metadata de dominio con información del certificado
			domainMeta := metadata.NewDomainMetadata()
			domainMeta.HasSSL = true
			domainMeta.SSLIssuer = record.IssuerName
			domainMeta.SSLValidUntil = record.NotAfter
			domainMeta.SSLValidFrom = record.NotBefore
			domainMeta.SSLWildcard = strings.HasPrefix(host, "*.")

			// Crear artifact con metadata tipado
			artifact := domain.NewArtifactWithMetadata(
				domain.ArtifactTypeSubdomain,
				host,
				c.Name(),
				domainMeta,
			)

			// Confianza alta para crt.sh (datos públicos oficiales)
			artifact.Confidence = 0.95

			// Tag automático si es wildcard
			if strings.HasPrefix(host, "*.") {
				artifact.AddTag("wildcard")
			}

			// También guardamos el certificado como artifact separado
			certArtifact := domain.NewArtifactWithMetadata(
				domain.ArtifactTypeCertificate,
				record.SerialNumber,
				c.Name(),
				certMeta,
			)
			certArtifact.Confidence = 0.95

			// Establecer relación: subdomain uses_cert certificate
			artifact.AddRelation(certArtifact.ID, domain.RelationUsesCert, 0.95, c.Name())

			artifacts = append(artifacts, artifact)
			artifacts = append(artifacts, certArtifact)
		}
	}

	return artifacts
}

// Close implements ports.Source
// No hay recursos que liberar actualmente, pero implementamos el método
// para cumplir con la interfaz ports.Source.
func (c *CRT) Close() error {
	c.logger.Debug("closing crtsh source")
	// http.Client no requiere Close() explícito
	return nil
}

// certRecord representa un registro de certificado de crt.sh.
type certRecord struct {
	IssuerName   string `json:"issuer_name"`
	NameValue    string `json:"name_value"`
	NotAfter     string `json:"not_after"`
	NotBefore    string `json:"not_before"`
	SerialNumber string `json:"serial_number"`
}
