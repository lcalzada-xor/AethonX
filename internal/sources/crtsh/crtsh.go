// internal/sources/crtsh/crtsh.go
package crtsh

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"aethonx/internal/core"
	"aethonx/internal/platform/logx"
)

type CRT struct {
	http *http.Client
	log  logx.Logger
}

func New(logger logx.Logger) *CRT {
	cl := &http.Client{}
	return &CRT{http: cl, log: logger.With("source", "crtsh")}
}

func (c *CRT) Name() string          { return "crtsh" }
func (c *CRT) Mode() core.SourceMode { return core.PassiveOnly }

// API: https://crt.sh/?q=%25.<domain>&output=json
// Nota: name_value puede contener múltiples FQDNs separados por \n
type rec struct {
	NameValue string `json:"name_value"`
	Issuer    string `json:"issuer_name"`
	NotAfter  string `json:"not_after"`
}

func (c *CRT) Run(ctx context.Context, t core.Target) (core.RunResult, error) {
	u := fmt.Sprintf("https://crt.sh/?q=%%25.%s&output=json", t.RootDomain)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return core.RunResult{}, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return core.RunResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return core.RunResult{}, fmt.Errorf("crt.sh status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return core.RunResult{}, err
	}

	// A veces el content-type no es application/json, pero el cuerpo sí es JSON válido
	var rows []rec
	if err := json.Unmarshal(body, &rows); err != nil {
		// Cuando crt.sh devuelve demasiados resultados puede responder HTML/errores
		return core.RunResult{
			Warnings: []string{fmt.Sprintf("crtsh decode: %v", err)},
		}, nil
	}

	arts := make([]core.Artifact, 0, len(rows))
	for _, r := range rows {
		// name_value puede tener varios dominios línea a línea
		for _, host := range strings.Split(r.NameValue, "\n") {
			host = strings.TrimSpace(host)
			if host == "" {
				continue
			}
			arts = append(arts, core.Artifact{
				Type:   "subdomain",
				Value:  host,
				Source: c.Name(),
				Meta: map[string]string{
					"issuer":   r.Issuer,
					"notAfter": r.NotAfter,
				},
			})
		}
	}

	c.log.Debug("fetched crtsh records",
		"domain", t.RootDomain,
		"count_raw", len(arts),
	)

	return core.RunResult{Artifacts: arts}, nil
}
