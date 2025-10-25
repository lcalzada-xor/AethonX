// internal/platform/ui/raw_presenter.go
package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// LogFormat define el formato de salida para el modo raw
type LogFormat string

const (
	LogFormatText LogFormat = "text" // Formato logfmt (default)
	LogFormatJSON LogFormat = "json" // Formato JSON estructurado
)

// RawPresenter implementa el Presenter para modo raw (logs sin formato visual)
type RawPresenter struct {
	format    LogFormat
	mu        sync.Mutex
	startTime time.Time
}

// NewRawPresenter crea un nuevo RawPresenter
func NewRawPresenter(format LogFormat) *RawPresenter {
	return &RawPresenter{
		format:    format,
		startTime: time.Now(),
	}
}

// log escribe un log en el formato configurado
func (r *RawPresenter) log(level, message string, fields map[string]interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	timestamp := time.Now().UTC().Format(time.RFC3339)

	if r.format == LogFormatJSON {
		r.logJSON(timestamp, level, message, fields)
	} else {
		r.logText(timestamp, level, message, fields)
	}
}

// logText escribe en formato logfmt: timestamp LEVEL message key=value key2=value2
func (r *RawPresenter) logText(timestamp, level, message string, fields map[string]interface{}) {
	var parts []string
	parts = append(parts, timestamp)
	parts = append(parts, fmt.Sprintf("%-5s", level))
	parts = append(parts, message)

	for k, v := range fields {
		parts = append(parts, fmt.Sprintf("%s=%v", k, r.formatValue(v)))
	}

	fmt.Fprintln(os.Stdout, strings.Join(parts, " "))
}

// logJSON escribe en formato JSON estructurado
func (r *RawPresenter) logJSON(timestamp, level, message string, fields map[string]interface{}) {
	logEntry := map[string]interface{}{
		"timestamp": timestamp,
		"level":     level,
		"message":   message,
	}

	if len(fields) > 0 {
		logEntry["data"] = fields
	}

	jsonBytes, _ := json.Marshal(logEntry)
	fmt.Fprintln(os.Stdout, string(jsonBytes))
}

// formatValue formatea valores para logfmt (entrecomilla strings con espacios)
func (r *RawPresenter) formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		if strings.Contains(val, " ") {
			return fmt.Sprintf("%q", val)
		}
		return val
	case time.Duration:
		return val.String()
	case float64:
		return fmt.Sprintf("%.1f", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// Start inicia la presentación
func (r *RawPresenter) Start(info ScanInfo) {
	r.startTime = time.Now()
	r.log("INFO", "scan_started", map[string]interface{}{
		"target":     info.Target,
		"mode":       info.Mode,
		"workers":    info.Workers,
		"timeout":    fmt.Sprintf("%ds", info.TimeoutSeconds),
		"streaming":  info.StreamingOn,
		"ui_mode":    string(info.UIMode),
		"log_format": string(r.format),
	})
}

// StartStage notifica el inicio de un stage
func (r *RawPresenter) StartStage(stage StageInfo) {
	r.log("INFO", "stage_started", map[string]interface{}{
		"stage":   stage.Number,
		"name":    stage.Name,
		"sources": strings.Join(stage.Sources, ","),
	})
}

// FinishStage notifica la finalización de un stage
func (r *RawPresenter) FinishStage(stageNum int, duration time.Duration) {
	r.log("INFO", "stage_completed", map[string]interface{}{
		"stage":    stageNum,
		"duration": duration,
	})
}

// StartSource notifica el inicio de un source
func (r *RawPresenter) StartSource(stageNum int, sourceName string) {
	r.log("INFO", "source_started", map[string]interface{}{
		"stage":  stageNum,
		"source": sourceName,
	})
}

// UpdateSource actualiza el progreso de un source
func (r *RawPresenter) UpdateSource(sourceName string, metrics ProgressMetrics) {
	fields := map[string]interface{}{
		"source":    sourceName,
		"artifacts": metrics.Current,
		"rate":      fmt.Sprintf("%.1f", metrics.Rate),
	}

	if metrics.Phase != "" {
		fields["phase"] = metrics.Phase
	}

	if metrics.Total > 0 {
		fields["percentage"] = fmt.Sprintf("%.1f%%", metrics.Percentage)
	}

	if metrics.EstimatedTime > 0 {
		fields["eta"] = metrics.EstimatedTime
	}

	r.log("INFO", "source_progress", fields)
}

// UpdateSourcePhase actualiza la fase de un source
func (r *RawPresenter) UpdateSourcePhase(sourceName string, phase string) {
	r.log("INFO", "source_phase", map[string]interface{}{
		"source": sourceName,
		"phase":  phase,
	})
}

// FinishSource notifica la finalización de un source
func (r *RawPresenter) FinishSource(sourceName string, status Status, duration time.Duration, artifactCount int) {
	r.log("INFO", "source_completed", map[string]interface{}{
		"source":    sourceName,
		"status":    status.String(),
		"duration":  duration,
		"artifacts": artifactCount,
	})
}

// UpdateDiscoveries actualiza estadísticas de descubrimiento
func (r *RawPresenter) UpdateDiscoveries(discoveries DiscoveryStats) {
	r.log("INFO", "discoveries_update", map[string]interface{}{
		"subdomains": discoveries.Subdomains,
		"ips":        discoveries.IPs,
		"urls":       discoveries.URLs,
		"emails":     discoveries.Emails,
		"ports":      discoveries.Ports,
		"total":      discoveries.Total,
		"unique":     discoveries.Unique,
	})
}

// Info muestra un mensaje informativo
func (r *RawPresenter) Info(msg string) {
	r.log("INFO", msg, nil)
}

// Warning muestra una advertencia
func (r *RawPresenter) Warning(msg string) {
	r.log("WARN", msg, nil)
}

// Error muestra un error
func (r *RawPresenter) Error(msg string) {
	r.log("ERROR", msg, nil)
}

// Finish finaliza la presentación con estadísticas finales
func (r *RawPresenter) Finish(stats ScanStats) {
	fields := map[string]interface{}{
		"duration":       stats.TotalDuration,
		"total":          stats.TotalArtifacts,
		"unique":         stats.UniqueArtifacts,
		"sources_ok":     stats.SourcesSucceeded,
		"sources_failed": stats.SourcesFailed,
		"relationships":  stats.RelationshipsBuilt,
	}

	r.log("INFO", "scan_completed", fields)

	// Log artifact breakdown
	if len(stats.ArtifactsByType) > 0 {
		r.log("INFO", "artifacts_by_type", map[string]interface{}{
			"breakdown": stats.ArtifactsByType,
		})
	}
}

// Close limpia recursos
func (r *RawPresenter) Close() error {
	return nil
}
