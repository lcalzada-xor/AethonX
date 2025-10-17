// internal/core/domain/errors.go
package domain

import "errors"

// Errores de dominio comunes.
var (
	// Target errors
	ErrEmptyTarget     = errors.New("target cannot be empty")
	ErrInvalidDomain   = errors.New("invalid domain format")
	ErrInvalidScanMode = errors.New("invalid scan mode")
	ErrInvalidScope    = errors.New("invalid scope configuration")

	// Artifact errors
	ErrInvalidArtifact     = errors.New("invalid artifact")
	ErrEmptyArtifactValue  = errors.New("artifact value cannot be empty")
	ErrInvalidArtifactType = errors.New("invalid artifact type")
	ErrArtifactMergeFailed = errors.New("failed to merge artifacts")

	// Source errors
	ErrSourceNotFound      = errors.New("source not found")
	ErrSourceNotCompatible = errors.New("source not compatible with scan mode")
	ErrSourceInitFailed    = errors.New("source initialization failed")
	ErrSourceExecutionFailed = errors.New("source execution failed")
	ErrSourceTimeout       = errors.New("source execution timeout")

	// Scan errors
	ErrScanFailed        = errors.New("scan failed")
	ErrNoSourcesAvailable = errors.New("no sources available for scan")
	ErrScanCanceled      = errors.New("scan was canceled")
	ErrScanTimeout       = errors.New("scan timeout exceeded")

	// Configuration errors
	ErrInvalidConfig      = errors.New("invalid configuration")
	ErrMissingConfig      = errors.New("missing required configuration")
	ErrConfigLoadFailed   = errors.New("failed to load configuration")
	ErrConfigParseFailed  = errors.New("failed to parse configuration")

	// Export errors
	ErrExportFailed      = errors.New("export failed")
	ErrUnsupportedFormat = errors.New("unsupported export format")
	ErrInvalidOutputPath = errors.New("invalid output path")
)
