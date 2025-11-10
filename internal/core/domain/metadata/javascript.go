package metadata

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// JavaScriptMetadata contiene información sobre archivos JavaScript analizados.
type JavaScriptMetadata struct {
	FilePath        string   `json:"file_path"`                // Path del archivo analizado
	Component       string   `json:"component"`                // Nombre de la librería (jquery, lodash)
	Version         string   `json:"version"`                  // Versión detectada
	DetectionMethod string   `json:"detection_method"`         // filecontent|filename|hash
	License         string   `json:"license,omitempty"`        // Licencia (MIT, Apache)
	FileHash        string   `json:"file_hash,omitempty"`      // Hash del archivo
	VulnCount       int      `json:"vuln_count"`               // Número de vulnerabilidades
	HighestSeverity string   `json:"highest_severity"`         // critical|high|medium|low
	IsMinified      bool     `json:"is_minified"`              // Si está minificado
	SourceURL       string   `json:"source_url,omitempty"`     // URL original
	NPMName         string   `json:"npm_name,omitempty"`       // Nombre en npm
	Licenses        []string `json:"licenses,omitempty"`       // Array de licencias
}

// ToMap implementa ArtifactMetadata interface
func (m *JavaScriptMetadata) ToMap() map[string]string {
	result := map[string]string{
		"file_path":        m.FilePath,
		"component":        m.Component,
		"version":          m.Version,
		"detection_method": m.DetectionMethod,
		"license":          m.License,
		"file_hash":        m.FileHash,
		"vuln_count":       strconv.Itoa(m.VulnCount),
		"highest_severity": m.HighestSeverity,
		"is_minified":      strconv.FormatBool(m.IsMinified),
		"source_url":       m.SourceURL,
		"npm_name":         m.NPMName,
		"licenses":         StringSliceToCSV(m.Licenses),
	}
	return result
}

// FromMap implementa ArtifactMetadata interface
func (m *JavaScriptMetadata) FromMap(data map[string]string) error {
	m.FilePath = data["file_path"]
	m.Component = data["component"]
	m.Version = data["version"]
	m.DetectionMethod = data["detection_method"]
	m.License = data["license"]
	m.FileHash = data["file_hash"]
	m.SourceURL = data["source_url"]
	m.NPMName = data["npm_name"]
	m.HighestSeverity = data["highest_severity"]

	if vulnCount, err := strconv.Atoi(data["vuln_count"]); err == nil {
		m.VulnCount = vulnCount
	}

	if isMinified, err := strconv.ParseBool(data["is_minified"]); err == nil {
		m.IsMinified = isMinified
	}

	if licenses := data["licenses"]; licenses != "" {
		m.Licenses = CSVToStringSlice(licenses)
	}

	return nil
}

// IsValid implementa ArtifactMetadata interface
func (m *JavaScriptMetadata) IsValid() bool {
	return m.Component != "" && m.Version != ""
}

// Type implementa ArtifactMetadata interface
func (m *JavaScriptMetadata) Type() string {
	return "javascript"
}

// Serialize implementa metadata.Metadata interface
func (m *JavaScriptMetadata) Serialize() (map[string]interface{}, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize JavaScriptMetadata: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JavaScriptMetadata: %w", err)
	}

	return result, nil
}

// Deserialize implementa metadata.Metadata interface
func (m *JavaScriptMetadata) Deserialize(data map[string]interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	if err := json.Unmarshal(jsonData, m); err != nil {
		return fmt.Errorf("failed to unmarshal JavaScriptMetadata: %w", err)
	}

	return nil
}
