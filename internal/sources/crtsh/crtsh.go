// internal/sources/crtsh/crtsh.go
package crtsh

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
)

// CRT implementa una fuente que consulta la base de datos crt.sh
// para descubrir certificados SSL/TLS y subdominios asociados.
type CRT struct {
	client *http.Client
	logger logx.Logger
}

// New crea una nueva instancia de la fuente crt.sh.
func New(logger logx.Logger) ports.Source {
	return &CRT{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
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

	// Crear request con contexto
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		result.AddError(c.Name(), fmt.Sprintf("failed to create request: %v", err), false)
		return result, err
	}

	// Ejecutar request
	resp, err := c.client.Do(req)
	if err != nil {
		result.AddError(c.Name(), fmt.Sprintf("request failed: %v", err), false)
		return result, err
	}
	defer resp.Body.Close()

	// Verificar status code
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("crt.sh returned status %d", resp.StatusCode)
		result.AddError(c.Name(), err.Error(), false)
		return result, err
	}

	// Leer respuesta
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.AddError(c.Name(), fmt.Sprintf("failed to read response: %v", err), false)
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

			// Crear artifact
			artifact := domain.NewArtifact(
				domain.ArtifactTypeSubdomain,
				host,
				c.Name(),
			)

			// Añadir metadata del certificado
			artifact.Metadata["issuer"] = record.IssuerName
			artifact.Metadata["not_after"] = record.NotAfter
			artifact.Metadata["cert_serial"] = record.SerialNumber

			// Confianza alta para crt.sh (datos públicos oficiales)
			artifact.Confidence = 0.95

			artifacts = append(artifacts, artifact)
		}
	}

	return artifacts
}

// certRecord representa un registro de certificado de crt.sh.
type certRecord struct {
	IssuerName   string `json:"issuer_name"`
	NameValue    string `json:"name_value"`
	NotAfter     string `json:"not_after"`
	NotBefore    string `json:"not_before"`
	SerialNumber string `json:"serial_number"`
}
