// internal/sources/shodan/cli_executor.go
package shodan

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
	"aethonx/internal/sources/common"
)

const (
	cliSourceName    = "shodan-cli"
	cliDefaultTimeout = 120 * time.Second
)

// ShodanCLIExecutor wraps the Shodan CLI tool for subprocess execution.
// It leverages BaseCLISource for all subprocess management.
type ShodanCLIExecutor struct {
	*common.BaseCLISource
	parser *Parser
}

// NewCLIExecutor creates a new Shodan CLI executor.
func NewCLIExecutor(logger logx.Logger) *ShodanCLIExecutor {
	return &ShodanCLIExecutor{
		BaseCLISource: common.NewBaseCLISource(logger, common.BaseCLIConfig{
			SourceName:     cliSourceName,
			ExecPath:       "shodan",
			Timeout:        cliDefaultTimeout,
			ProgressBuffer: 10,
		}),
		parser: NewParser(logger, cliSourceName),
	}
}

// RunDomainSearch executes: shodan domain {domain}
// This command lists all subdomains for a domain (requires Shodan Membership).
func (e *ShodanCLIExecutor) RunDomainSearch(ctx context.Context, target domain.Target) ([]*domain.Artifact, error) {
	e.GetLogger().Info("executing shodan domain command", "target", target.Root)

	handler := &shodanDomainHandler{
		parser:    e.parser,
		target:    target,
		logger:    e.GetLogger(),
		artifacts: make([]*domain.Artifact, 0),
	}

	args := []string{"domain", target.Root}
	_, stderrOutput, err := e.ExecuteCLI(ctx, target, args, handler)

	// Log stderr if present
	if len(stderrOutput) > 0 {
		e.GetLogger().Debug("shodan domain stderr", "output", stderrOutput)
	}

	// Return partial results even on error
	if err != nil && len(handler.artifacts) > 0 {
		e.GetLogger().Warn("shodan domain exited with error but produced results",
			"error", err.Error(),
			"artifacts", len(handler.artifacts),
		)
		return handler.artifacts, nil
	}

	return handler.artifacts, err
}

// RunHostSearch executes: shodan host {ip}
// This command shows detailed information about a specific IP address.
func (e *ShodanCLIExecutor) RunHostSearch(ctx context.Context, ip string, target domain.Target) ([]*domain.Artifact, error) {
	e.GetLogger().Info("executing shodan host command", "ip", ip)

	handler := &shodanHostHandler{
		parser:    e.parser,
		target:    target,
		logger:    e.GetLogger(),
		artifacts: make([]*domain.Artifact, 0),
		jsonLines: make([]string, 0),
	}

	args := []string{"host", ip}
	_, stderrOutput, err := e.ExecuteCLI(ctx, target, args, handler)

	if len(stderrOutput) > 0 {
		e.GetLogger().Debug("shodan host stderr", "output", stderrOutput)
	}

	if err != nil && len(handler.artifacts) > 0 {
		e.GetLogger().Warn("shodan host exited with error but produced results",
			"error", err.Error(),
			"artifacts", len(handler.artifacts),
		)
		return handler.artifacts, nil
	}

	return handler.artifacts, err
}

// RunSearch executes: shodan search {query}
// This command searches Shodan database with custom query.
func (e *ShodanCLIExecutor) RunSearch(ctx context.Context, query string, target domain.Target) ([]*domain.Artifact, error) {
	e.GetLogger().Info("executing shodan search command", "query", query)

	handler := &shodanSearchHandler{
		parser:    e.parser,
		target:    target,
		logger:    e.GetLogger(),
		artifacts: make([]*domain.Artifact, 0),
	}

	args := []string{"search", query}
	_, stderrOutput, err := e.ExecuteCLI(ctx, target, args, handler)

	if len(stderrOutput) > 0 {
		e.GetLogger().Debug("shodan search stderr", "output", stderrOutput)
	}

	if err != nil && len(handler.artifacts) > 0 {
		e.GetLogger().Warn("shodan search exited with error but produced results",
			"error", err.Error(),
			"artifacts", len(handler.artifacts),
		)
		return handler.artifacts, nil
	}

	return handler.artifacts, err
}

// shodanDomainHandler processes output from `shodan domain` command.
// Output format: one subdomain per line (plain text).
type shodanDomainHandler struct {
	parser    *Parser
	target    domain.Target
	logger    logx.Logger
	artifacts []*domain.Artifact
	mu        sync.Mutex
}

// ProcessLine handles each line of output (one subdomain per line).
func (h *shodanDomainHandler) ProcessLine(line []byte) error {
	subdomain := strings.TrimSpace(string(line))
	if subdomain == "" {
		return nil
	}

	// Skip header lines or errors
	if strings.HasPrefix(subdomain, "Error:") || strings.HasPrefix(subdomain, "Usage:") {
		h.logger.Warn("shodan domain error", "line", subdomain)
		return nil
	}

	// Create subdomain artifact
	artifact := domain.NewArtifact(
		domain.ArtifactTypeSubdomain,
		subdomain,
		h.parser.sourceName,
	)

	h.mu.Lock()
	h.artifacts = append(h.artifacts, artifact)
	h.mu.Unlock()

	h.logger.Debug("discovered subdomain", "subdomain", subdomain)

	return nil
}

// Finalize is called after all lines are processed.
func (h *shodanDomainHandler) Finalize() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.logger.Info("shodan domain parsing completed", "count", len(h.artifacts))
	return nil
}

// shodanHostHandler processes output from `shodan host` command.
// Output format: JSON object (possibly multi-line).
type shodanHostHandler struct {
	parser    *Parser
	target    domain.Target
	logger    logx.Logger
	artifacts []*domain.Artifact
	jsonLines []string
	mu        sync.Mutex
}

// ProcessLine accumulates JSON lines.
func (h *shodanHostHandler) ProcessLine(line []byte) error {
	lineStr := strings.TrimSpace(string(line))
	if lineStr == "" {
		return nil
	}

	h.mu.Lock()
	h.jsonLines = append(h.jsonLines, lineStr)
	h.mu.Unlock()

	return nil
}

// Finalize parses the accumulated JSON.
func (h *shodanHostHandler) Finalize() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.jsonLines) == 0 {
		h.logger.Warn("no output from shodan host")
		return nil
	}

	// Join all lines into single JSON
	jsonData := strings.Join(h.jsonLines, "\n")

	var hostResp ShodanHostResponse
	if err := json.Unmarshal([]byte(jsonData), &hostResp); err != nil {
		h.logger.Warn("failed to parse shodan host JSON",
			"error", err.Error(),
			"data", jsonData,
		)
		return nil // Non-fatal
	}

	// Parse response into artifacts
	artifacts := h.parser.ParseHostResponse(&hostResp, h.target)
	h.artifacts = append(h.artifacts, artifacts...)

	h.logger.Info("shodan host parsing completed",
		"ip", hostResp.IPStr,
		"artifacts", len(artifacts),
	)

	return nil
}

// shodanSearchHandler processes output from `shodan search` command.
// Output format: one JSON object per line.
type shodanSearchHandler struct {
	parser    *Parser
	target    domain.Target
	logger    logx.Logger
	artifacts []*domain.Artifact
	mu        sync.Mutex
}

// ProcessLine parses each JSON line.
func (h *shodanSearchHandler) ProcessLine(line []byte) error {
	lineStr := strings.TrimSpace(string(line))
	if lineStr == "" {
		return nil
	}

	var hostResp ShodanHostResponse
	if err := json.Unmarshal([]byte(lineStr), &hostResp); err != nil {
		h.logger.Warn("failed to parse shodan search line",
			"error", err.Error(),
			"line", lineStr,
		)
		return nil // Non-fatal, continue processing
	}

	// Parse response into artifacts
	artifacts := h.parser.ParseHostResponse(&hostResp, h.target)

	h.mu.Lock()
	h.artifacts = append(h.artifacts, artifacts...)
	h.mu.Unlock()

	h.logger.Debug("parsed search result",
		"ip", hostResp.IPStr,
		"artifacts", len(artifacts),
	)

	return nil
}

// Finalize is called after all lines are processed.
func (h *shodanSearchHandler) Finalize() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.logger.Info("shodan search parsing completed", "count", len(h.artifacts))
	return nil
}
