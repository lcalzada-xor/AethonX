package retirejs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aethonx/internal/platform/logx"
)

// Downloader descarga archivos JavaScript desde URLs (fallback mode).
type Downloader struct {
	logger      logx.Logger
	httpClient  *http.Client
	tempDir     string
	maxFiles    int
	timeout     time.Duration
	downloaded  map[string]string // URL -> local path
	mu          sync.Mutex
}

// NewDownloader crea un nuevo Downloader
func NewDownloader(logger logx.Logger, maxFiles int, timeout time.Duration) *Downloader {
	return &Downloader{
		logger: logger,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		maxFiles:   maxFiles,
		timeout:    timeout,
		downloaded: make(map[string]string),
	}
}

// DownloadJSFiles descarga archivos JS desde URLs a un directorio temporal.
func (d *Downloader) DownloadJSFiles(ctx context.Context, urls []string) (string, error) {
	// Crear directorio temporal
	tempDir, err := os.MkdirTemp("", "retirejs-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	d.tempDir = tempDir

	d.logger.Info("downloading JS files",
		"total_urls", len(urls),
		"max_files", d.maxFiles,
		"temp_dir", tempDir,
	)

	// Deduplicar URLs
	uniqueURLs := d.deduplicateURLs(urls)

	// Limitar a maxFiles
	if len(uniqueURLs) > d.maxFiles {
		uniqueURLs = uniqueURLs[:d.maxFiles]
		d.logger.Info("limiting downloads to max files", "max_files", d.maxFiles)
	}

	// Descargar concurrentemente
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // Max 5 descargas concurrentes
	successCount := 0
	var successMu sync.Mutex

	for _, url := range uniqueURLs {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			// Adquirir semáforo
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return
			}

			// Descargar archivo
			if err := d.downloadFile(ctx, url); err != nil {
				d.logger.Debug("failed to download file", "url", url, "error", err)
			} else {
				successMu.Lock()
				successCount++
				successMu.Unlock()
			}
		}(url)
	}

	wg.Wait()

	d.logger.Info("download completed",
		"total_urls", len(uniqueURLs),
		"successful", successCount,
		"temp_dir", tempDir,
	)

	if successCount == 0 {
		return "", fmt.Errorf("failed to download any JS files")
	}

	return tempDir, nil
}

// downloadFile descarga un archivo individual
func (d *Downloader) downloadFile(ctx context.Context, url string) error {
	// Crear request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Ejecutar request
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	// Verificar status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Validar Content-Type (opcional, algunos servidores no lo envían correctamente)
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !strings.Contains(contentType, "javascript") && !strings.Contains(contentType, "text") {
		d.logger.Debug("unexpected content type", "url", url, "content_type", contentType)
	}

	// Generar nombre de archivo
	filename := d.generateFilename(url)
	destPath := filepath.Join(d.tempDir, filename)

	// Crear archivo
	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copiar contenido
	if _, err := io.Copy(file, resp.Body); err != nil {
		os.Remove(destPath)
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Registrar descarga exitosa
	d.mu.Lock()
	d.downloaded[url] = destPath
	d.mu.Unlock()

	d.logger.Debug("file downloaded successfully", "url", url, "path", destPath)

	return nil
}

// generateFilename genera un nombre de archivo a partir de una URL
func (d *Downloader) generateFilename(url string) string {
	// Extraer última parte de la URL
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		filename := parts[len(parts)-1]
		// Limpiar query params
		if idx := strings.Index(filename, "?"); idx != -1 {
			filename = filename[:idx]
		}
		if filename != "" && strings.HasSuffix(filename, ".js") {
			return filename
		}
	}

	// Fallback: generar nombre único
	return fmt.Sprintf("file_%d.js", time.Now().UnixNano())
}

// deduplicateURLs elimina URLs duplicadas
func (d *Downloader) deduplicateURLs(urls []string) []string {
	seen := make(map[string]bool)
	var unique []string

	for _, url := range urls {
		if !seen[url] {
			seen[url] = true
			unique = append(unique, url)
		}
	}

	return unique
}

// Cleanup elimina el directorio temporal
func (d *Downloader) Cleanup() error {
	if d.tempDir == "" {
		return nil
	}

	d.logger.Debug("cleaning up temp directory", "path", d.tempDir)

	if err := os.RemoveAll(d.tempDir); err != nil {
		return fmt.Errorf("failed to remove temp directory: %w", err)
	}

	d.tempDir = ""
	return nil
}
