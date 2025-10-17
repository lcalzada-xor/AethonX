// internal/core/ports/repository.go
package ports

import (
	"context"
	"time"

	"aethonx/internal/core/domain"
)

// Repository es el port para persistencia de resultados de escaneos.
// Permite almacenar y recuperar históricos de escaneos.
type Repository interface {
	// SaveScan guarda un resultado de escaneo completo
	SaveScan(ctx context.Context, result *domain.ScanResult) error

	// GetScan recupera un escaneo por su ID
	GetScan(ctx context.Context, id string) (*domain.ScanResult, error)

	// ListScans lista escaneos aplicando filtros opcionales
	ListScans(ctx context.Context, filter ScanFilter) ([]*domain.ScanResult, error)

	// DeleteScan elimina un escaneo por su ID
	DeleteScan(ctx context.Context, id string) error

	// Close cierra la conexión con el repositorio
	Close() error
}

// ArtifactRepository maneja la persistencia específica de artifacts.
type ArtifactRepository interface {
	// SaveArtifact guarda un artifact individual
	SaveArtifact(ctx context.Context, artifact *domain.Artifact) error

	// GetArtifactsByTarget recupera todos los artifacts de un target
	GetArtifactsByTarget(ctx context.Context, target string) ([]*domain.Artifact, error)

	// GetArtifactsByType recupera artifacts por tipo
	GetArtifactsByType(ctx context.Context, artifactType domain.ArtifactType) ([]*domain.Artifact, error)

	// SearchArtifacts busca artifacts por valor (partial match)
	SearchArtifacts(ctx context.Context, query string) ([]*domain.Artifact, error)
}

// ScanFilter define filtros para búsqueda de escaneos.
type ScanFilter struct {
	// Target filtrar por dominio objetivo
	Target string

	// Mode filtrar por modo de escaneo
	Mode domain.ScanMode

	// StartDate fecha mínima de inicio
	StartDate time.Time

	// EndDate fecha máxima de inicio
	EndDate time.Time

	// MinArtifacts número mínimo de artifacts
	MinArtifacts int

	// HasErrors solo escaneos con errores
	HasErrors *bool

	// Limit límite de resultados
	Limit int

	// Offset offset para paginación
	Offset int

	// SortBy campo por el cual ordenar (start_time, target, artifacts_count)
	SortBy string

	// SortDesc orden descendente si es true
	SortDesc bool
}

// DefaultScanFilter retorna un filtro por defecto.
func DefaultScanFilter() ScanFilter {
	return ScanFilter{
		Target:       "",
		StartDate:    time.Time{},
		EndDate:      time.Time{},
		MinArtifacts: 0,
		HasErrors:    nil,
		Limit:        100,
		Offset:       0,
		SortBy:       "start_time",
		SortDesc:     true,
	}
}

// RepositoryStats contiene estadísticas del repositorio.
type RepositoryStats struct {
	TotalScans     int
	TotalArtifacts int
	UniqueTargets  int
	OldestScan     time.Time
	NewestScan     time.Time
	StorageSize    int64 // bytes
}

// StatsRepository proporciona estadísticas del repositorio.
type StatsRepository interface {
	Repository

	// GetStats retorna estadísticas del repositorio
	GetStats(ctx context.Context) (*RepositoryStats, error)
}
