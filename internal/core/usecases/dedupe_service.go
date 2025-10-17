// internal/core/usecases/dedupe_service.go
package usecases

import (
	"sort"

	"aethonx/internal/core/domain"
)

// DedupeService maneja la deduplicación y normalización de artifacts.
type DedupeService struct{}

// NewDedupeService crea una nueva instancia del servicio.
func NewDedupeService() *DedupeService {
	return &DedupeService{}
}

// Deduplicate normaliza y elimina duplicados de una lista de artifacts.
// Si un mismo artifact aparece múltiples veces, combina sus fuentes y metadata.
func (d *DedupeService) Deduplicate(artifacts []*domain.Artifact) []*domain.Artifact {
	if len(artifacts) == 0 {
		return artifacts
	}

	// Mapa para tracking: key -> artifact
	seen := make(map[string]*domain.Artifact)

	for _, a := range artifacts {
		if a == nil || !a.IsValid() {
			continue
		}

		// Normalizar artifact
		a.Normalize()

		// Generar key única
		key := a.Key()

		// Si ya existe, merge
		if existing, found := seen[key]; found {
			if err := existing.Merge(a); err != nil {
				// Log error pero continuar
				continue
			}
		} else {
			// Nuevo artifact
			seen[key] = a
		}
	}

	// Convertir mapa a slice
	result := make([]*domain.Artifact, 0, len(seen))
	for _, a := range seen {
		result = append(result, a)
	}

	// Ordenar para output consistente
	d.sortArtifacts(result)

	return result
}

// sortArtifacts ordena artifacts por tipo y luego por valor.
func (d *DedupeService) sortArtifacts(artifacts []*domain.Artifact) {
	sort.Slice(artifacts, func(i, j int) bool {
		if artifacts[i].Type == artifacts[j].Type {
			return artifacts[i].Value < artifacts[j].Value
		}
		return artifacts[i].Type < artifacts[j].Type
	})
}

// FilterByType filtra artifacts por tipo.
func (d *DedupeService) FilterByType(artifacts []*domain.Artifact, types ...domain.ArtifactType) []*domain.Artifact {
	if len(types) == 0 {
		return artifacts
	}

	typeMap := make(map[domain.ArtifactType]bool)
	for _, t := range types {
		typeMap[t] = true
	}

	filtered := make([]*domain.Artifact, 0)
	for _, a := range artifacts {
		if typeMap[a.Type] {
			filtered = append(filtered, a)
		}
	}

	return filtered
}

// FilterByConfidence filtra artifacts por confianza mínima.
func (d *DedupeService) FilterByConfidence(artifacts []*domain.Artifact, minConfidence float64) []*domain.Artifact {
	filtered := make([]*domain.Artifact, 0)
	for _, a := range artifacts {
		if a.Confidence >= minConfidence {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

// FilterBySource filtra artifacts descubiertos por una fuente específica.
func (d *DedupeService) FilterBySource(artifacts []*domain.Artifact, source string) []*domain.Artifact {
	filtered := make([]*domain.Artifact, 0)
	for _, a := range artifacts {
		for _, s := range a.Sources {
			if s == source {
				filtered = append(filtered, a)
				break
			}
		}
	}
	return filtered
}

// GroupByType agrupa artifacts por tipo.
func (d *DedupeService) GroupByType(artifacts []*domain.Artifact) map[domain.ArtifactType][]*domain.Artifact {
	groups := make(map[domain.ArtifactType][]*domain.Artifact)
	for _, a := range artifacts {
		groups[a.Type] = append(groups[a.Type], a)
	}
	return groups
}
