// Package httpx implements an HTTP probing source that validates live hosts.
// It consumes subdomains from Stage 0 and probes them to discover live URLs and IPs.
package httpx

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/platform/registry"
)

// Auto-registro de la source al importar el package
func init() {
	if err := registry.Global().Register(
		"httpx",
		func(cfg ports.SourceConfig, logger logx.Logger) (ports.Source, error) {
			return New(logger), nil
		},
		ports.SourceMetadata{
			Name:         "httpx",
			Description:  "HTTP probing tool for live host detection and URL discovery",
			Version:      "1.0.0",
			Author:       "AethonX",
			Mode:         domain.SourceModeActive, // Active probing
			Type:         domain.SourceTypeBuiltin,
			RequiresAuth: false,
			RateLimit:    0,

			// Dependency declaration (Stage 1: consumes subdomains)
			InputArtifacts: []domain.ArtifactType{
				domain.ArtifactTypeSubdomain,
				domain.ArtifactTypeDomain,
			},
			OutputArtifacts: []domain.ArtifactType{
				domain.ArtifactTypeURL,
				domain.ArtifactTypeIP,
			},
			Priority:  7, // Media-alta prioridad para active validation
			StageHint: 0, // Auto-detect (será Stage 1)
		},
	); err != nil {
		logx.New().Warn("failed to register httpx source", "error", err.Error())
	}
}

const (
	sourceName      = "httpx"
	defaultTimeout  = 5 * time.Second
	maxRedirects    = 3
	defaultUserAgent = "AethonX-HTTPx/1.0 (+https://github.com/yourusername/aethonx)"
)

// HTTPx implements ports.Source and ports.InputConsumer for HTTP probing.
type HTTPx struct {
	client *http.Client
	logger logx.Logger

	// Configuration
	timeout    time.Duration
	userAgent  string
	followRedirects bool
}

// New crea una nueva instancia de HTTPx.
func New(logger logx.Logger) ports.Source {
	return &HTTPx{
		client: &http.Client{
			Timeout: defaultTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= maxRedirects {
					return http.ErrUseLastResponse
				}
				return nil
			},
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
				TLSHandshakeTimeout: 5 * time.Second,
				DisableKeepAlives:   false,
			},
		},
		logger:          logger.With("source", sourceName),
		timeout:         defaultTimeout,
		userAgent:       defaultUserAgent,
		followRedirects: true,
	}
}

// Name retorna el nombre de la source.
func (h *HTTPx) Name() string {
	return sourceName
}

// Mode retorna el modo de operación (active).
func (h *HTTPx) Mode() domain.SourceMode {
	return domain.SourceModeActive
}

// Type retorna el tipo de source (builtin).
func (h *HTTPx) Type() domain.SourceType {
	return domain.SourceTypeBuiltin
}

// Run ejecuta HTTP probing sobre el target root (fallback sin inputs).
// Este método se llama cuando HTTPx se ejecuta sin dependencias previas.
func (h *HTTPx) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	result := domain.NewScanResult(target)

	h.logger.Debug("running httpx without inputs (probing root domain only)")

	// Probe solo el root domain
	hosts := []string{target.Root}
	h.probeHosts(ctx, hosts, result)

	return result, nil
}

// RunWithInput implementa ports.InputConsumer.
// Recibe subdomains/domains de Stage 0 y los prueba con HTTP/HTTPS.
func (h *HTTPx) RunWithInput(ctx context.Context, target domain.Target, input *domain.ScanResult) (*domain.ScanResult, error) {
	result := domain.NewScanResult(target)

	if len(input.Artifacts) == 0 {
		h.logger.Warn("no input artifacts provided, falling back to root domain")
		return h.Run(ctx, target)
	}

	h.logger.Info("running httpx with input artifacts", "count", len(input.Artifacts))

	// Extraer hostnames de los artifacts de input
	hosts := h.extractHosts(input.Artifacts)
	h.logger.Debug("extracted hosts from inputs", "count", len(hosts))

	// Probe todos los hosts
	h.probeHosts(ctx, hosts, result)

	stats := result.Stats()
	h.logger.Info("httpx probing completed",
		"input_hosts", len(hosts),
		"discovered_urls", stats[string(domain.ArtifactTypeURL)],
		"discovered_ips", stats[string(domain.ArtifactTypeIP)],
	)

	return result, nil
}

// Close libera recursos (no hay background workers).
func (h *HTTPx) Close() error {
	h.logger.Debug("closing httpx source")
	h.client.CloseIdleConnections()
	return nil
}

// extractHosts extrae hostnames de artifacts (subdomains/domains).
func (h *HTTPx) extractHosts(artifacts []*domain.Artifact) []string {
	hosts := make([]string, 0, len(artifacts))
	seen := make(map[string]bool)

	for _, artifact := range artifacts {
		if artifact == nil {
			continue
		}

		// Solo procesar subdomains y domains
		if artifact.Type != domain.ArtifactTypeSubdomain && artifact.Type != domain.ArtifactTypeDomain {
			continue
		}

		host := strings.ToLower(strings.TrimSpace(artifact.Value))
		if host == "" || seen[host] {
			continue
		}

		seen[host] = true
		hosts = append(hosts, host)
	}

	return hosts
}

// probeHosts prueba una lista de hosts con HTTP y HTTPS.
func (h *HTTPx) probeHosts(ctx context.Context, hosts []string, result *domain.ScanResult) {
	for _, host := range hosts {
		select {
		case <-ctx.Done():
			h.logger.Warn("context cancelled, stopping probes")
			return
		default:
			h.probeHost(ctx, host, result)
		}
	}
}

// probeHost prueba un host individual con HTTP y HTTPS.
func (h *HTTPx) probeHost(ctx context.Context, host string, result *domain.ScanResult) {
	schemes := []string{"https", "http"}

	for _, scheme := range schemes {
		url := fmt.Sprintf("%s://%s", scheme, host)

		if h.probe(ctx, url, result) {
			// Si HTTPS funciona, skip HTTP
			if scheme == "https" {
				break
			}
		}
	}
}

// probe ejecuta una petición HTTP/HTTPS a una URL.
func (h *HTTPx) probe(ctx context.Context, url string, result *domain.ScanResult) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		h.logger.Debug("failed to create request", "url", url, "error", err.Error())
		return false
	}

	req.Header.Set("User-Agent", h.userAgent)
	req.Header.Set("Accept", "*/*")

	resp, err := h.client.Do(req)
	if err != nil {
		h.logger.Debug("probe failed", "url", url, "error", err.Error())
		return false
	}
	defer resp.Body.Close()

	h.logger.Debug("probe successful",
		"url", url,
		"status", resp.StatusCode,
		"content_length", resp.ContentLength,
	)

	// Crear artifact de URL
	urlArtifact := domain.NewArtifact(domain.ArtifactTypeURL, url, sourceName)

	// TODO: Agregar URLMetadata cuando esté disponible
	// Por ahora, el artifact tiene solo la URL sin metadata adicional

	result.AddArtifact(urlArtifact)

	// Extraer IP del remote address
	if resp.Request != nil && resp.Request.URL != nil {
		if ip := h.extractIP(resp.Request.URL.Host); ip != "" {
			ipArtifact := domain.NewArtifact(domain.ArtifactTypeIP, ip, sourceName)
			result.AddArtifact(ipArtifact)

			h.logger.Debug("extracted IP from response", "url", url, "ip", ip)
		}
	}

	return true
}

// extractIP extrae la dirección IP de un host (puede incluir puerto).
func (h *HTTPx) extractIP(host string) string {
	// Remover puerto si existe
	hostname := host
	if strings.Contains(host, ":") {
		var err error
		hostname, _, err = net.SplitHostPort(host)
		if err != nil {
			h.logger.Debug("failed to split host:port", "host", host)
			return ""
		}
	}

	// Verificar si ya es una IP
	if net.ParseIP(hostname) != nil {
		return hostname
	}

	// Resolver hostname a IP
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", hostname)
	if err != nil || len(ips) == 0 {
		return ""
	}

	return ips[0].String()
}
