package retirejs

import (
	"os"
	"path/filepath"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/platform/logx"
)

// FileManager gestiona archivos JS locales reutilizados de golinkfinderevo.
type FileManager struct {
	logger      logx.Logger
	maxFiles    int
	seenFiles   map[string]bool        // Deduplicación
	directories map[string][]string    // dir -> []files
}

// NewFileManager crea un nuevo FileManager
func NewFileManager(logger logx.Logger, maxFiles int) *FileManager {
	return &FileManager{
		logger:      logger,
		maxFiles:    maxFiles,
		seenFiles:   make(map[string]bool),
		directories: make(map[string][]string),
	}
}

// ExtractLocalFiles extrae file_paths de JavaScriptMetadata en artifacts.
func (fm *FileManager) ExtractLocalFiles(artifacts []*domain.Artifact) []string {
	var localFiles []string

	for _, artifact := range artifacts {
		if artifact.Type != domain.ArtifactTypeJavaScript {
			continue
		}

		// Extraer JavaScriptMetadata
		jsMeta, ok := artifact.TypedMetadata.(*metadata.JavaScriptMetadata)
		if !ok {
			continue
		}

		// Verificar que el archivo existe localmente
		if jsMeta.FilePath == "" {
			continue
		}

		if _, err := os.Stat(jsMeta.FilePath); os.IsNotExist(err) {
			fm.logger.Debug("file path does not exist", "path", jsMeta.FilePath)
			continue
		}

		// Deduplicar
		if fm.seenFiles[jsMeta.FilePath] {
			continue
		}

		fm.seenFiles[jsMeta.FilePath] = true
		localFiles = append(localFiles, jsMeta.FilePath)

		// Agrupar por directorio
		dir := filepath.Dir(jsMeta.FilePath)
		fm.directories[dir] = append(fm.directories[dir], jsMeta.FilePath)

		// Respetar maxFiles
		if len(localFiles) >= fm.maxFiles {
			fm.logger.Info("reached max files limit", "limit", fm.maxFiles)
			break
		}
	}

	fm.logger.Info("extracted local JS files from artifacts",
		"total_files", len(localFiles),
		"directories", len(fm.directories),
	)

	return localFiles
}

// GetDirectories retorna los directorios que contienen archivos JS.
func (fm *FileManager) GetDirectories() []string {
	dirs := make([]string, 0, len(fm.directories))
	for dir := range fm.directories {
		dirs = append(dirs, dir)
	}
	return dirs
}

// GetFilesInDirectory retorna archivos JS en un directorio específico.
func (fm *FileManager) GetFilesInDirectory(dir string) []string {
	return fm.directories[dir]
}
