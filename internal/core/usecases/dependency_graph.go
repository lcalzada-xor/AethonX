// internal/core/usecases/dependency_graph.go
package usecases

import (
	"fmt"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
)

// dependencyGraph representa el grafo de dependencias entre sources.
type dependencyGraph struct {
	// nodes mapea source name a su índice en el slice
	nodes map[string]int

	// sources lista ordenada de sources
	sources []ports.Source

	// metadata mapea source name a sus metadatos
	metadata map[string]ports.SourceMetadata

	// adjacencyList mapea índice de source a lista de sources dependientes
	// adjacencyList[A] = [B, C] significa que B y C dependen de A
	adjacencyList map[int][]int

	// inDegree mapea índice de source a número de dependencias entrantes
	inDegree map[int]int

	// outputTypes mapea índice de source a tipos de artifacts que produce
	outputTypes map[int]map[domain.ArtifactType]bool
}

// buildDependencyGraph construye el grafo de dependencias entre sources.
func (p *PipelineOrchestrator) buildDependencyGraph(sources []ports.Source) *dependencyGraph {
	graph := &dependencyGraph{
		nodes:         make(map[string]int),
		sources:       make([]ports.Source, 0, len(sources)),
		metadata:      make(map[string]ports.SourceMetadata),
		adjacencyList: make(map[int][]int),
		inDegree:      make(map[int]int),
		outputTypes:   make(map[int]map[domain.ArtifactType]bool),
	}

	// Fase 1: Construir nodes y extraer metadata
	for i, source := range sources {
		sourceName := source.Name()
		graph.nodes[sourceName] = i
		graph.sources = append(graph.sources, source)
		graph.inDegree[i] = 0

		// Obtener metadata (desde registry o default)
		meta, exists := p.sourceMetadata[sourceName]
		if !exists {
			// Metadata por defecto: source sin dependencias (Stage 0)
			meta = ports.SourceMetadata{
				Name:            sourceName,
				InputArtifacts:  []domain.ArtifactType{}, // Sin inputs = Stage 0
				OutputArtifacts: []domain.ArtifactType{}, // Sin outputs declarados
				Priority:        0,
			}
		}
		graph.metadata[sourceName] = meta

		// Construir mapa de output types para búsqueda rápida
		outputMap := make(map[domain.ArtifactType]bool)
		for _, artifactType := range meta.OutputArtifacts {
			outputMap[artifactType] = true
		}
		graph.outputTypes[i] = outputMap
	}

	// Fase 2: Construir aristas (dependencias)
	for i, source := range sources {
		sourceName := source.Name()
		meta := graph.metadata[sourceName]

		// Si no tiene InputArtifacts, es un source de Stage 0 (sin dependencias)
		if len(meta.InputArtifacts) == 0 {
			continue
		}

		// Para cada artifact type requerido, buscar sources que lo producen
		for _, requiredType := range meta.InputArtifacts {
			// Buscar sources que producen este tipo
			for j, _ := range sources {
				if i == j {
					continue // Skip self
				}

				// Verificar si source j produce el tipo requerido
				if graph.outputTypes[j][requiredType] {
					// Crear arista: j -> i (i depende de j)
					graph.adjacencyList[j] = append(graph.adjacencyList[j], i)
					graph.inDegree[i]++
				}
			}
		}
	}

	return graph
}

// topologicalSortByLevels ejecuta topological sort y agrupa sources por niveles (stages).
// Usa algoritmo de Kahn con BFS para agrupar sources en stages concurrentes.
func (p *PipelineOrchestrator) topologicalSortByLevels(graph *dependencyGraph) ([]Stage, error) {
	n := len(graph.sources)
	if n == 0 {
		return nil, fmt.Errorf("no sources in graph")
	}

	// Copiar inDegree para no modificar el original
	currentInDegree := make(map[int]int)
	for i := 0; i < n; i++ {
		currentInDegree[i] = graph.inDegree[i]
	}

	// Queue para BFS (contiene índices de sources)
	queue := make([]int, 0)
	processed := make(map[int]bool)

	// Inicializar queue con sources sin dependencias (inDegree == 0)
	for i := 0; i < n; i++ {
		if currentInDegree[i] == 0 {
			queue = append(queue, i)
		}
	}

	if len(queue) == 0 {
		return nil, fmt.Errorf("circular dependency detected: no sources with zero in-degree")
	}

	// BFS por niveles (stages)
	stages := make([]Stage, 0)
	stageID := 0

	for len(queue) > 0 {
		// Todos los sources en el queue actual forman un stage
		stageSize := len(queue)
		stageSources := make([]ports.Source, 0, stageSize)

		// Procesar todos los sources del stage actual
		for i := 0; i < stageSize; i++ {
			idx := queue[0]
			queue = queue[1:] // Dequeue

			stageSources = append(stageSources, graph.sources[idx])
			processed[idx] = true

			// Reducir inDegree de sources dependientes
			for _, dependentIdx := range graph.adjacencyList[idx] {
				currentInDegree[dependentIdx]--

				// Si inDegree llega a 0, agregar al queue (próximo stage)
				if currentInDegree[dependentIdx] == 0 {
					queue = append(queue, dependentIdx)
				}
			}
		}

		// Crear stage con sources procesadas
		stage := NewStage(stageID, stageSources)
		stages = append(stages, *stage)
		stageID++
	}

	// Verificar que todas las sources fueron procesadas
	if len(processed) != n {
		// Encontrar sources no procesadas (involucradas en ciclo)
		unprocessed := make([]string, 0)
		for i := 0; i < n; i++ {
			if !processed[i] {
				unprocessed = append(unprocessed, graph.sources[i].Name())
			}
		}
		return nil, fmt.Errorf("circular dependency detected involving sources: %v", unprocessed)
	}

	return stages, nil
}
