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
	client     httpclient.Client
	logger     logx.Logger
	progressCh chan ports.ProgressUpdate
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
		client:     *httpclient.New(httpConfig, logger),
		logger:     logger.With("source", "crtsh"),
		progressCh: make(chan ports.ProgressUpdate, 10), // Buffered channel
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

	// Procesar records y extraer subdominios CON PROGRESO INCREMENTAL
	artifacts := c.processRecordsWithProgress(ctx, records, target)

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

// processRecordsWithProgress procesa los registros de certificados y extrae artifacts
// emitiendo actualizaciones de progreso en tiempo real.
func (c *CRT) processRecordsWithProgress(ctx context.Context, records []certRecord, target domain.Target) []*domain.Artifact {
	artifacts := make([]*domain.Artifact, 0)
	artifactCount := 0

	for _, record := range records {
		// Verificar cancelación de contexto
		select {
		case <-ctx.Done():
			c.logger.Debug("processRecords cancelled by context")
			return artifacts
		default:
		}

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
				IssuerCN:     record.IssuerName,
				ValidUntil:   record.NotAfter,
				ValidFrom:    record.NotBefore,
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

			// Passive discovery - medium confidence
			artifact.Confidence = domain.ConfidenceMedium

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
			certArtifact.Confidence = domain.ConfidenceMedium

			// Establecer relación: subdomain uses_cert certificate
			artifact.AddRelation(certArtifact.ID, domain.RelationUsesCert, 0.95, c.Name())

			artifacts = append(artifacts, artifact)
			artifacts = append(artifacts, certArtifact)
			artifactCount += 2

			// Emitir progreso (non-blocking)
			select {
			case c.progressCh <- ports.ProgressUpdate{
				ArtifactCount: artifactCount,
				Message:       fmt.Sprintf("Processing %s", host),
			}:
			default:
				// Canal lleno, skip update para no bloquear
			}
		}
	}

	return artifacts
}

// ProgressChannel implementa ports.StreamingSource
func (c *CRT) ProgressChannel() <-chan ports.ProgressUpdate {
	return c.progressCh
}

// Stream implementa ports.StreamingSource (no usado actualmente pero requerido por interfaz)
func (c *CRT) Stream(ctx context.Context, target domain.Target) (<-chan *domain.Artifact, <-chan error) {
	artifactCh := make(chan *domain.Artifact, 100)
	errorCh := make(chan error, 1)

	go func() {
		defer close(artifactCh)
		defer close(errorCh)

		result, err := c.Run(ctx, target)
		if err != nil {
			errorCh <- err
			return
		}

		for _, artifact := range result.Artifacts {
			select {
			case artifactCh <- artifact:
			case <-ctx.Done():
				return
			}
		}
	}()

	return artifactCh, errorCh
}

// Close implements ports.Source
// No hay recursos que liberar actualmente, pero implementamos el método
// para cumplir con la interfaz ports.Source.
func (c *CRT) Close() error {
	c.logger.Debug("closing crtsh source")
	// Close progress channel to prevent goroutine leaks
	close(c.progressCh)
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
