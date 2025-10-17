// internal/core/domain/enums.go
package domain

// ScanMode define el modo de ejecución del escaneo.
type ScanMode string

const (
	// ScanModePassive solo utiliza técnicas pasivas (OSINT, APIs públicas)
	ScanModePassive ScanMode = "passive"

	// ScanModeActive utiliza técnicas activas (DNS resolution, HTTP probing, port scanning)
	ScanModeActive ScanMode = "active"

	// ScanModeHybrid combina técnicas pasivas y activas
	ScanModeHybrid ScanMode = "hybrid"
)

// IsValid verifica si el modo de escaneo es válido.
func (m ScanMode) IsValid() bool {
	switch m {
	case ScanModePassive, ScanModeActive, ScanModeHybrid:
		return true
	default:
		return false
	}
}

// String retorna la representación string del modo.
func (m ScanMode) String() string {
	return string(m)
}

// SourceMode define el modo de operación de una fuente.
type SourceMode string

const (
	// SourceModePassive indica que la fuente solo opera de forma pasiva
	SourceModePassive SourceMode = "passive"

	// SourceModeActive indica que la fuente requiere interacción activa
	SourceModeActive SourceMode = "active"

	// SourceModeBoth indica que la fuente puede operar en ambos modos
	SourceModeBoth SourceMode = "both"
)

// IsValid verifica si el modo de fuente es válido.
func (m SourceMode) IsValid() bool {
	switch m {
	case SourceModePassive, SourceModeActive, SourceModeBoth:
		return true
	default:
		return false
	}
}

// String retorna la representación string del modo.
func (m SourceMode) String() string {
	return string(m)
}

// CompatibleWith verifica si el modo de fuente es compatible con el modo de escaneo.
func (m SourceMode) CompatibleWith(scanMode ScanMode) bool {
	switch m {
	case SourceModePassive:
		return scanMode == ScanModePassive || scanMode == ScanModeHybrid
	case SourceModeActive:
		return scanMode == ScanModeActive || scanMode == ScanModeHybrid
	case SourceModeBoth:
		return true
	default:
		return false
	}
}

// SourceType clasifica fuentes por su tipo de implementación.
type SourceType string

const (
	// SourceTypeAPI fuentes que consumen APIs HTTP/REST
	SourceTypeAPI SourceType = "api"

	// SourceTypeCLI fuentes que ejecutan binarios externos
	SourceTypeCLI SourceType = "cli"

	// SourceTypeBuiltin fuentes implementadas nativamente en Go
	SourceTypeBuiltin SourceType = "builtin"

	// SourceTypeFile fuentes que leen de archivos
	SourceTypeFile SourceType = "file"

	// SourceTypeDatabase fuentes que consultan bases de datos
	SourceTypeDatabase SourceType = "database"
)

// IsValid verifica si el tipo de fuente es válido.
func (t SourceType) IsValid() bool {
	switch t {
	case SourceTypeAPI, SourceTypeCLI, SourceTypeBuiltin, SourceTypeFile, SourceTypeDatabase:
		return true
	default:
		return false
	}
}

// String retorna la representación string del tipo.
func (t SourceType) String() string {
	return string(t)
}
