// Package golinkfinderevo - Output Handler
// Implements OutputHandler interface for BaseCLISource integration.
package golinkfinderevo

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
)

// GoLinkfinderEvoHandler implements common.OutputHandler for golinkfinderevo.
// It processes stdout line-by-line and accumulates ResourceReports.
type GoLinkfinderEvoHandler struct {
	parser     *Parser
	gfParser   *GFParser
	target     domain.Target
	logger     logx.Logger
	reports    []*ResourceReport
	gfFindings GFResults
	mu         sync.Mutex
	lineBuffer bytes.Buffer
}

// ProcessLine processes a single line of output from golinkfinderevo.
// Implements common.OutputHandler interface.
func (h *GoLinkfinderEvoHandler) ProcessLine(line []byte) error {
	// golinkfinderevo outputs newline-delimited JSON
	// Each line is a complete ResourceReport JSON object

	lineStr := string(line)
	lineStr = strings.TrimSpace(lineStr)

	// Skip empty lines
	if lineStr == "" {
		return nil
	}

	// Skip non-JSON lines (stderr mixed in stdout, warnings, etc.)
	if !strings.HasPrefix(lineStr, "{") {
		h.logger.Debug("skipping non-JSON line", "line", lineStr)
		return nil
	}

	// Parse the JSON line
	report, err := h.parser.ParseReport([]byte(lineStr))
	if err != nil {
		h.logger.Warn("failed to parse report line",
			"error", err.Error(),
			"line", lineStr[:min(len(lineStr), 100)], // Truncate for logging
		)
		return nil // Don't fail on single parse errors
	}

	// Thread-safe append
	h.mu.Lock()
	h.reports = append(h.reports, report)
	h.mu.Unlock()

	h.logger.Debug("parsed resource report",
		"url", report.URL,
		"endpoints", len(report.Endpoints),
	)

	return nil
}

// Finalize is called after all lines have been processed.
// Implements common.OutputHandler interface.
func (h *GoLinkfinderEvoHandler) Finalize() error {
	h.logger.Debug("finalizing golinkfinderevo output",
		"total_reports", len(h.reports),
	)

	// GF results are written to separate files (gf.json, gf.txt)
	// They will be parsed separately by the source after execution

	return nil
}

// GetReports returns all accumulated ResourceReports.
func (h *GoLinkfinderEvoHandler) GetReports() []*ResourceReport {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.reports
}

// StdinHandler implements stdin-based input for golinkfinderevo.
// Writes URLs to stdin for processing.
type StdinHandler struct {
	urls   []string
	logger logx.Logger
}

// NewStdinHandler creates a new StdinHandler with URLs to process.
func NewStdinHandler(urls []string, logger logx.Logger) *StdinHandler {
	return &StdinHandler{
		urls:   urls,
		logger: logger,
	}
}

// WriteToStdin writes URLs to stdin for golinkfinderevo to process.
func (sh *StdinHandler) WriteToStdin(stdin io.WriteCloser) error {
	defer stdin.Close()

	for _, url := range sh.urls {
		_, err := fmt.Fprintln(stdin, url)
		if err != nil {
			return fmt.Errorf("failed to write URL to stdin: %w", err)
		}
	}

	sh.logger.Debug("wrote URLs to stdin", "count", len(sh.urls))
	return nil
}

// ProcessOutput processes golinkfinderevo output similar to httpx.
// This is an alternative to line-by-line processing for compatibility.
func (h *GoLinkfinderEvoHandler) ProcessOutput(
	stdout io.Reader,
	stderr io.Reader,
	progressCh chan<- ports.ProgressUpdate,
) ([]byte, error) {
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	// Read all stdout
	_, err := io.Copy(&stdoutBuf, stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to read stdout: %w", err)
	}

	// Read all stderr
	_, err = io.Copy(&stderrBuf, stderr)
	if err != nil {
		h.logger.Warn("error reading stderr", "error", err.Error())
	}

	// Parse stdout as newline-delimited JSON
	reports, err := h.parser.ParseMultipleReports(stdoutBuf.Bytes())
	if err != nil {
		return stderrBuf.Bytes(), fmt.Errorf("failed to parse reports: %w", err)
	}

	h.mu.Lock()
	h.reports = reports
	h.mu.Unlock()

	h.logger.Info("processed golinkfinderevo output",
		"reports", len(reports),
	)

	// Send progress update
	if progressCh != nil {
		totalEndpoints := 0
		for _, r := range reports {
			totalEndpoints += len(r.Endpoints)
		}
		select {
		case progressCh <- ports.ProgressUpdate{
			ArtifactCount: totalEndpoints,
			Message:       fmt.Sprintf("discovered %d endpoints", totalEndpoints),
		}:
		default:
		}
	}

	return stderrBuf.Bytes(), nil
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
