// Package retirejs implements integration with retire.js CLI tool.
// retire.js detects JavaScript libraries with known vulnerabilities.
package retirejs

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/platform/logx"
	"aethonx/internal/sources/common"
)

const (
	sourceName           = "retirejs"
	defaultTimeout       = 180 * time.Second
	defaultMaxFiles      = 100
	defaultDownloadTimeout = 10 * time.Second
)

// RetireJSSource implements ports.Source and ports.InputConsumer.
// It wraps retire.js CLI tool for JavaScript vulnerability detection.
type RetireJSSource struct {
	*common.BaseCLISource

	severity         string
	exitCode         int
	deep             bool
	includeOSV       bool
	extensions       []string
	maxFiles         int
	downloadTimeout  time.Duration
	cachePath        string
	noCache          bool
	preferLocal      bool
	fallbackDownload bool
	parser           *Parser
	downloader       *Downloader
	fileManager      *FileManager
}

// Compile-time interface assertions
var (
	_ ports.Source        = (*RetireJSSource)(nil)
	_ ports.InputConsumer = (*RetireJSSource)(nil)
)

// New creates a new RetireJSSource with default configuration.
func New(logger logx.Logger) *RetireJSSource {
	return &RetireJSSource{
		BaseCLISource: common.NewBaseCLISource(logger, common.BaseCLIConfig{
			SourceName:     sourceName,
			ExecPath:       "retire",
			Timeout:        defaultTimeout,
			ProgressBuffer: 10,
		}),
		severity:         "medium",
		exitCode:         0, // Don't fail on vulnerabilities (exit 0 always)
		deep:             false,
		includeOSV:       false,
		extensions:       []string{"js"},
		maxFiles:         defaultMaxFiles,
		downloadTimeout:  defaultDownloadTimeout,
		noCache:          false,
		preferLocal:      true,
		fallbackDownload: true,
		parser:           NewParser(logger, sourceName),
		fileManager:      NewFileManager(logger, defaultMaxFiles),
	}
}

// NewWithConfig creates RetireJSSource with custom configuration.
func NewWithConfig(
	logger logx.Logger,
	execPath string,
	timeout time.Duration,
	severity string,
	exitCode int,
	deep bool,
	includeOSV bool,
	maxFiles int,
	extensions []string,
	cachePath string,
	noCache bool,
	downloadTimeout time.Duration,
	preferLocal bool,
	fallbackDownload bool,
) *RetireJSSource {
	return &RetireJSSource{
		BaseCLISource: common.NewBaseCLISource(logger, common.BaseCLIConfig{
			SourceName:     sourceName,
			ExecPath:       execPath,
			Timeout:        timeout,
			ProgressBuffer: 10,
		}),
		severity:         severity,
		exitCode:         exitCode,
		deep:             deep,
		includeOSV:       includeOSV,
		extensions:       extensions,
		maxFiles:         maxFiles,
		downloadTimeout:  downloadTimeout,
		cachePath:        cachePath,
		noCache:          noCache,
		preferLocal:      preferLocal,
		fallbackDownload: fallbackDownload,
		parser:           NewParser(logger, sourceName),
		downloader:       NewDownloader(logger, maxFiles, downloadTimeout),
		fileManager:      NewFileManager(logger, maxFiles),
	}
}

// Name returns the source name.
func (s *RetireJSSource) Name() string {
	return sourceName
}

// Mode returns the source operation mode.
func (s *RetireJSSource) Mode() domain.SourceMode {
	return domain.SourceModeActive
}

// Type returns the source type.
func (s *RetireJSSource) Type() domain.SourceType {
	return domain.SourceTypeCLI
}

// Run executes retire.js without inputs (minimal scan).
func (s *RetireJSSource) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
	s.GetLogger().Info("retirejs run called without inputs",
		"target", target.Root,
		"note", "Stage 4 source - use RunWithInput for best results",
	)

	result := domain.NewScanResult(target)
	result.AddWarning(sourceName, "retirejs is a Stage 4 source - provide JS artifacts via RunWithInput for best results")

	return result, nil
}

// RunWithInput executes retire.js with JavaScript artifacts from previous stages.
func (s *RetireJSSource) RunWithInput(ctx context.Context, target domain.Target, input *domain.ScanResult) (*domain.ScanResult, error) {
	startTime := time.Now()

	s.GetLogger().Info("retirejs starting with input from previous stages",
		"target", target.Root,
		"input_artifacts", len(input.Artifacts),
		"prefer_local", s.preferLocal,
	)

	// 1. Intentar extraer archivos locales de golinkfinderevo
	var artifacts []*domain.Artifact
	var err error

	if s.preferLocal {
		localFiles := s.fileManager.ExtractLocalFiles(input.Artifacts)

		if len(localFiles) > 0 {
			s.GetLogger().Info("found local JS files from previous stages, scanning locally",
				"files", len(localFiles),
				"directories", len(s.fileManager.GetDirectories()),
			)
			artifacts, err = s.scanLocalFiles(ctx, target, localFiles)
			if err != nil {
				s.GetLogger().Warn("failed to scan local files", "error", err)
			} else {
				// Éxito con archivos locales
				result := domain.NewScanResult(target)
				result.Artifacts = artifacts
				s.logCompletionStats(startTime, len(artifacts), len(localFiles), 0)
				return result, nil
			}
		}
	}

	// 2. FALLBACK: extraer URLs y descargar
	if !s.fallbackDownload {
		s.GetLogger().Warn("no local files found and fallback download is disabled")
		result := domain.NewScanResult(target)
		result.AddWarning(sourceName, "no local files found and fallback download is disabled")
		return result, nil
	}

	s.GetLogger().Info("no local files found or scan failed, falling back to download mode")
	jsURLs := s.extractJavaScriptURLs(input.Artifacts)

	if len(jsURLs) == 0 {
		s.GetLogger().Warn("no JavaScript files found in input")
		result := domain.NewScanResult(target)
		result.AddWarning(sourceName, "no JavaScript files found to scan")
		return result, nil
	}

	// Descargar archivos
	s.downloader = NewDownloader(s.GetLogger(), s.maxFiles, s.downloadTimeout)
	tempDir, err := s.downloader.DownloadJSFiles(ctx, jsURLs)
	if err != nil {
		return nil, fmt.Errorf("failed to download JS files: %w", err)
	}
	defer s.downloader.Cleanup()

	artifacts, err = s.scanDirectory(ctx, tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to scan downloaded files: %w", err)
	}

	result := domain.NewScanResult(target)
	result.Artifacts = artifacts
	s.logCompletionStats(startTime, len(artifacts), 0, len(jsURLs))
	return result, nil
}

// scanLocalFiles escanea archivos JS locales (reutilizados de golinkfinder)
func (s *RetireJSSource) scanLocalFiles(ctx context.Context, target domain.Target, files []string) ([]*domain.Artifact, error) {
	// Agrupar por directorio para minimizar llamadas a retire
	directories := s.fileManager.GetDirectories()

	var allArtifacts []*domain.Artifact

	for _, dir := range directories {
		s.GetLogger().Debug("scanning directory with retire", "dir", dir)

		artifacts, err := s.scanDirectory(ctx, dir)
		if err != nil {
			s.GetLogger().Warn("failed to scan directory", "dir", dir, "error", err)
			continue
		}

		allArtifacts = append(allArtifacts, artifacts...)
	}

	return allArtifacts, nil
}

// retireHandler captures JSON output from retire.js
type retireHandler struct {
	output []byte
	logger logx.Logger
}

func (h *retireHandler) ProcessLine(line []byte) error {
	// Accumulate all output
	h.output = append(h.output, line...)
	h.output = append(h.output, '\n')
	return nil
}

func (h *retireHandler) Finalize() error {
	h.logger.Debug("retire output captured", "bytes", len(h.output))
	return nil
}

// scanDirectory ejecuta retire --path DIR --outputformat jsonsimple
func (s *RetireJSSource) scanDirectory(ctx context.Context, dir string) ([]*domain.Artifact, error) {
	args := s.buildRetireArgs(dir)

	s.GetLogger().Debug("executing retire command",
		"path", dir,
		"args", strings.Join(args, " "),
	)

	// Create handler to capture output
	handler := &retireHandler{
		output: make([]byte, 0, 1024),
		logger: s.GetLogger(),
	}

	_, _, err := s.ExecuteCLI(ctx, domain.Target{Root: "retire-scan"}, args, handler)
	if err != nil {
		// retire.js puede retornar exit code != 0 si encuentra vulnerabilidades
		// Intentar parsear output de todas formas
		s.GetLogger().Debug("retire execution finished with error", "error", err)
	}

	if len(handler.output) == 0 {
		s.GetLogger().Debug("no output from retire", "path", dir)
		return []*domain.Artifact{}, nil
	}

	artifacts, err := s.parser.Parse(handler.output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse retire output: %w", err)
	}

	s.GetLogger().Debug("parsed retire output",
		"path", dir,
		"artifacts", len(artifacts),
	)

	return artifacts, nil
}

// buildRetireArgs construye los argumentos para retire CLI
func (s *RetireJSSource) buildRetireArgs(path string) []string {
	args := []string{
		"--path", path,
		"--outputformat", "jsonsimple",
		"--exitwith", strconv.Itoa(s.exitCode),
	}

	if s.severity != "" && s.severity != "none" {
		args = append(args, "--severity", s.severity)
	}

	if s.deep {
		args = append(args, "--deep")
	}

	if s.includeOSV {
		args = append(args, "--includeOsv")
	}

	if s.noCache {
		args = append(args, "--nocache")
	}

	if s.cachePath != "" {
		args = append(args, "--cachedir", s.cachePath)
	}

	if len(s.extensions) > 0 {
		args = append(args, "--ext", strings.Join(s.extensions, ","))
	}

	return args
}

// extractJavaScriptURLs extrae URLs de archivos JS de input artifacts (fallback)
func (s *RetireJSSource) extractJavaScriptURLs(artifacts []*domain.Artifact) []string {
	urlSet := make(map[string]bool)

	for _, artifact := range artifacts {
		// Buscar ArtifactTypeJavaScript con SourceURL
		if artifact.Type == domain.ArtifactTypeJavaScript {
			if artifact.DiscoveryContext != nil && artifact.DiscoveryContext.SourceURL != "" {
				urlSet[artifact.DiscoveryContext.SourceURL] = true
			}
		}

		// Buscar ArtifactTypeURL con extensión .js
		if artifact.Type == domain.ArtifactTypeURL {
			if strings.HasSuffix(strings.ToLower(artifact.Value), ".js") {
				urlSet[artifact.Value] = true
			}
		}
	}

	urls := make([]string, 0, len(urlSet))
	for url := range urlSet {
		urls = append(urls, url)
		if len(urls) >= s.maxFiles {
			break
		}
	}

	s.GetLogger().Debug("extracted JavaScript URLs from artifacts",
		"total", len(urls),
	)

	return urls
}

// logCompletionStats registra estadísticas de completación
func (s *RetireJSSource) logCompletionStats(startTime time.Time, artifactCount, localFiles, downloads int) {
	duration := time.Since(startTime)

	s.GetLogger().Info("retirejs scan completed",
		"duration", duration.String(),
		"artifacts", artifactCount,
		"local_files", localFiles,
		"downloads", downloads,
	)
}

// Initialize implements ports.AdvancedSource
func (s *RetireJSSource) Initialize(ctx context.Context) error {
	return s.DefaultInitialize(
		"retire.js",
		"npm install -g retire (see https://github.com/RetireJS/retire.js)",
	)
}

// Validate implements ports.AdvancedSource
func (s *RetireJSSource) Validate() error {
	return s.DefaultValidate()
}

// HealthCheck implements ports.AdvancedSource
func (s *RetireJSSource) HealthCheck(ctx context.Context) error {
	return s.DefaultHealthCheck(ctx)
}
