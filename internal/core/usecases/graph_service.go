// internal/core/usecases/graph_service.go
package usecases

import (
	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
)

// GraphService proporciona operaciones de grafo sobre artifacts y sus relaciones.
// Usa índices para queries O(1) y es escalable hasta 100K+ artifacts.
type GraphService struct {
	// artifacts almacena todos los artifacts por ID (O(1) lookup)
	artifacts map[string]*domain.Artifact

	// relationIndex almacena relaciones para lookups rápidos
	// relationIndex[relationType][sourceID] = []targetIDs
	relationIndex map[domain.RelationType]map[string][]string

	// reverseIndex almacena relaciones inversas para traversal bidireccional
	// reverseIndex[relationType][targetID] = []sourceIDs
	reverseIndex map[domain.RelationType]map[string][]string

	logger logx.Logger
}

// NewGraphService crea un nuevo GraphService con los artifacts dados.
func NewGraphService(artifacts []*domain.Artifact, logger logx.Logger) *GraphService {
	g := &GraphService{
		artifacts:     make(map[string]*domain.Artifact, len(artifacts)),
		relationIndex: make(map[domain.RelationType]map[string][]string),
		reverseIndex:  make(map[domain.RelationType]map[string][]string),
		logger:        logger.With("component", "graph_service"),
	}

	// Construir índices
	g.buildIndexes(artifacts)

	return g
}

// buildIndexes construye los índices de relaciones para queries O(1).
func (g *GraphService) buildIndexes(artifacts []*domain.Artifact) {
	// Primero, indexar todos los artifacts por ID
	for _, artifact := range artifacts {
		g.artifacts[artifact.ID] = artifact
	}

	// Segundo, construir índices de relaciones
	for _, artifact := range artifacts {
		for _, rel := range artifact.Relations {
			// Forward index: source -> targets
			if g.relationIndex[rel.Type] == nil {
				g.relationIndex[rel.Type] = make(map[string][]string)
			}
			g.relationIndex[rel.Type][artifact.ID] = append(
				g.relationIndex[rel.Type][artifact.ID],
				rel.TargetID,
			)

			// Reverse index: target -> sources
			if g.reverseIndex[rel.Type] == nil {
				g.reverseIndex[rel.Type] = make(map[string][]string)
			}
			g.reverseIndex[rel.Type][rel.TargetID] = append(
				g.reverseIndex[rel.Type][rel.TargetID],
				artifact.ID,
			)
		}
	}

	g.logger.Debug("graph indexes built",
		"artifacts", len(g.artifacts),
		"relation_types", len(g.relationIndex),
	)
}

// GetArtifact retorna un artifact por su ID.
func (g *GraphService) GetArtifact(artifactID string) *domain.Artifact {
	return g.artifacts[artifactID]
}

// GetRelated retorna todos los artifacts relacionados de un tipo específico.
// Complexity: O(1) lookup + O(k) donde k = número de relaciones de ese tipo.
func (g *GraphService) GetRelated(artifactID string, relType domain.RelationType) []*domain.Artifact {
	// Lookup en O(1) usando el índice
	targetIDs, found := g.relationIndex[relType][artifactID]
	if !found {
		return nil
	}

	// Convertir IDs a artifacts (O(k) donde k = número de targets)
	results := make([]*domain.Artifact, 0, len(targetIDs))
	for _, targetID := range targetIDs {
		if artifact := g.artifacts[targetID]; artifact != nil {
			results = append(results, artifact)
		}
	}

	return results
}

// GetReverseRelated retorna todos los artifacts que apuntan a este artifact.
// Útil para queries como "¿qué subdominios apuntan a esta IP?"
func (g *GraphService) GetReverseRelated(artifactID string, relType domain.RelationType) []*domain.Artifact {
	sourceIDs, found := g.reverseIndex[relType][artifactID]
	if !found {
		return nil
	}

	results := make([]*domain.Artifact, 0, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		if artifact := g.artifacts[sourceID]; artifact != nil {
			results = append(results, artifact)
		}
	}

	return results
}

// GetAllRelations retorna todas las relaciones de un artifact.
func (g *GraphService) GetAllRelations(artifactID string) []domain.ArtifactRelation {
	artifact := g.artifacts[artifactID]
	if artifact == nil {
		return nil
	}
	return artifact.GetAllRelations()
}

// GetNeighbors retorna todos los artifacts relacionados hasta una profundidad dada.
// Usa BFS para traversal iterativo (evita stack overflow).
// Complexity: O(V + E) donde V = vértices visitados, E = aristas exploradas.
func (g *GraphService) GetNeighbors(artifactID string, depth int) []*domain.Artifact {
	if depth < 1 {
		return nil
	}

	visited := make(map[string]bool)
	queue := []string{artifactID}
	visited[artifactID] = true
	currentDepth := 0

	var results []*domain.Artifact

	for len(queue) > 0 && currentDepth < depth {
		levelSize := len(queue)

		for i := 0; i < levelSize; i++ {
			currentID := queue[0]
			queue = queue[1:]

			current := g.artifacts[currentID]
			if current == nil {
				continue
			}

			// Explorar todas las relaciones del artifact actual
			for _, rel := range current.Relations {
				if !visited[rel.TargetID] {
					visited[rel.TargetID] = true
					queue = append(queue, rel.TargetID)

					if target := g.artifacts[rel.TargetID]; target != nil {
						results = append(results, target)
					}
				}
			}
		}

		currentDepth++
	}

	return results
}

// FindPath encuentra el camino más corto entre dos artifacts usando BFS.
// Retorna la secuencia de relaciones desde source hasta target.
// Complexity: O(V + E) en el peor caso.
func (g *GraphService) FindPath(fromID, toID string) []domain.ArtifactRelation {
	if fromID == toID {
		return nil
	}

	visited := make(map[string]bool)
	parent := make(map[string]*pathNode)
	queue := []string{fromID}
	visited[fromID] = true

	// BFS
	found := false
	for len(queue) > 0 && !found {
		currentID := queue[0]
		queue = queue[1:]

		current := g.artifacts[currentID]
		if current == nil {
			continue
		}

		// Explorar relaciones
		for _, rel := range current.Relations {
			if !visited[rel.TargetID] {
				visited[rel.TargetID] = true
				parent[rel.TargetID] = &pathNode{
					artifactID: currentID,
					relation:   rel,
				}
				queue = append(queue, rel.TargetID)

				if rel.TargetID == toID {
					found = true
					break
				}
			}
		}
	}

	if !found {
		return nil
	}

	// Reconstruir el path desde toID hacia fromID
	var path []domain.ArtifactRelation
	currentID := toID

	for currentID != fromID {
		node := parent[currentID]
		if node == nil {
			break
		}
		path = append([]domain.ArtifactRelation{node.relation}, path...)
		currentID = node.artifactID
	}

	return path
}

// FindByType retorna todos los artifacts de un tipo específico.
// Complexity: O(n) donde n = número total de artifacts.
// Para escalabilidad, considera añadir un índice por tipo si esto se usa frecuentemente.
func (g *GraphService) FindByType(artifactType domain.ArtifactType) []*domain.Artifact {
	var results []*domain.Artifact
	for _, artifact := range g.artifacts {
		if artifact.Type == artifactType {
			results = append(results, artifact)
		}
	}
	return results
}

// GetStats retorna estadísticas del grafo.
func (g *GraphService) GetStats() GraphStats {
	totalRelations := 0
	relationsByType := make(map[domain.RelationType]int)

	for _, artifact := range g.artifacts {
		totalRelations += len(artifact.Relations)
		for _, rel := range artifact.Relations {
			relationsByType[rel.Type]++
		}
	}

	return GraphStats{
		TotalArtifacts:   len(g.artifacts),
		TotalRelations:   totalRelations,
		RelationsByType:  relationsByType,
		UniqueRelations:  len(g.relationIndex),
		IndexSizeForward: len(g.relationIndex),
		IndexSizeReverse: len(g.reverseIndex),
	}
}

// pathNode representa un nodo en el camino de BFS.
type pathNode struct {
	artifactID string
	relation   domain.ArtifactRelation
}

// GraphStats contiene estadísticas del grafo.
type GraphStats struct {
	TotalArtifacts   int
	TotalRelations   int
	RelationsByType  map[domain.RelationType]int
	UniqueRelations  int
	IndexSizeForward int
	IndexSizeReverse int
}
