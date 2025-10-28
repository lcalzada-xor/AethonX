package providers

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GitHubRelease represents a GitHub release API response.
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// GetVersion returns the version string from the release tag.
func (r *GitHubRelease) GetVersion() string {
	return strings.TrimPrefix(r.TagName, "v")
}

// GitHubProvider handles downloading binaries from GitHub releases.
type GitHubProvider struct {
	client *http.Client
	token  string
}

// NewGitHubProvider creates a new GitHub provider.
func NewGitHubProvider() *GitHubProvider {
	return &GitHubProvider{
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
		token: os.Getenv("GITHUB_TOKEN"),
	}
}

// GetLatestRelease fetches the latest release information from GitHub.
func (g *GitHubProvider) GetLatestRelease(ctx context.Context, repo string) (*GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add GitHub token if available for higher rate limits
	if g.token != "" {
		req.Header.Set("Authorization", "token "+g.token)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Include rate limit info in error for 403
		if resp.StatusCode == http.StatusForbidden {
			remaining := resp.Header.Get("X-RateLimit-Remaining")
			reset := resp.Header.Get("X-RateLimit-Reset")
			if remaining == "0" && reset != "" {
				// Convert Unix timestamp to human readable
				// For now, just say "in ~X minutes"
				return nil, fmt.Errorf("GitHub API rate limit exceeded (try again in ~60 minutes)")
			}
			return nil, fmt.Errorf("GitHub API returned status 403 (rate limit or authentication issue)")
		}
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode release: %w", err)
	}

	return &release, nil
}

// DownloadAsset downloads a specific asset from a GitHub release.
func (g *GitHubProvider) DownloadAsset(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add GitHub token if available
	if g.token != "" {
		req.Header.Set("Authorization", "token "+g.token)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("download failed with status 403 (rate limit or authentication issue)")
		}
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create destination file
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Copy content
	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// FindMatchingAsset finds the asset matching the platform pattern.
func (g *GitHubProvider) FindMatchingAsset(release *GitHubRelease, pattern string) (string, string, error) {
	for _, asset := range release.Assets {
		if matchPattern(asset.Name, pattern) {
			return asset.Name, asset.BrowserDownloadURL, nil
		}
	}
	return "", "", fmt.Errorf("no asset matching pattern %s found", pattern)
}

// ExtractZip extracts a zip file to a destination directory.
func (g *GitHubProvider) ExtractZip(zipPath, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		path := filepath.Join(destDir, file.Name)

		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.Mode())
			continue
		}

		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Extract file
		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		rc, err := file.Open()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("failed to open file in zip: %w", err)
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return fmt.Errorf("failed to extract file: %w", err)
		}
	}

	return nil
}

// ExtractTarGz extracts a tar.gz archive to the specified directory.
func (g *GitHubProvider) ExtractTarGz(tarGzPath, destDir string) error {
	// Open the tar.gz file
	file, err := os.Open(tarGzPath)
	if err != nil {
		return fmt.Errorf("failed to open tar.gz: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Construct target path
		target := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}

		case tar.TypeReg:
			// Create parent directory
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Create file
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			// Copy content
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to extract file: %w", err)
			}

			outFile.Close()
		}
	}

	return nil
}

// matchPattern checks if a filename matches a pattern (supports * wildcard).
func matchPattern(name, pattern string) bool {
	// Simple pattern matching with * wildcard
	parts := strings.Split(pattern, "*")

	if len(parts) == 1 {
		// No wildcard, exact match
		return name == pattern
	}

	// Check if name starts with first part
	if !strings.HasPrefix(name, parts[0]) {
		return false
	}

	// Check if name ends with last part
	if !strings.HasSuffix(name, parts[len(parts)-1]) {
		return false
	}

	// Check intermediate parts
	pos := len(parts[0])
	for i := 1; i < len(parts)-1; i++ {
		idx := strings.Index(name[pos:], parts[i])
		if idx == -1 {
			return false
		}
		pos += idx + len(parts[i])
	}

	return true
}
