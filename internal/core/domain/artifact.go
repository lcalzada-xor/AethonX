// internal/core/domain/artifact.go
package domain

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"aethonx/internal/core/domain/metadata"
)

// Artifact representa un dato descubierto durante el reconocimiento.
// Es la entidad principal de datos en AethonX.
type Artifact struct {
	// ID es un hash único generado a partir de Type + Value
	ID string

	// Type clasifica el artefacto
	Type ArtifactType

	// Value es el valor normalizado del artefacto
	Value string

	// Sources lista las fuentes que descubrieron este artefacto
	Sources []string

	// TypedMetadata contiene metadata estructurado y tipado
	TypedMetadata metadata.ArtifactMetadata

	// Confidence indica la confianza del descubrimiento [0.0-1.0]
	Confidence float64

	// DiscoveredAt timestamp del descubrimiento
	DiscoveredAt time.Time

	// Tags permite categorización adicional
	Tags []string
}

// NewArtifact crea un nuevo artefacto con valores por defecto.
func NewArtifact(artifactType ArtifactType, value, source string) *Artifact {
	a := &Artifact{
		Type:         artifactType,
		Value:        value,
		Sources:      []string{source},
		Confidence:   1.0,
		DiscoveredAt: time.Now(),
		Tags:         []string{},
	}
	a.Normalize()
	a.ID = a.GenerateID()
	return a
}

// NewArtifactWithMetadata crea un artifact con metadata tipado.
func NewArtifactWithMetadata(artifactType ArtifactType, value, source string, typedMeta metadata.ArtifactMetadata) *Artifact {
	a := NewArtifact(artifactType, value, source)
	a.TypedMetadata = typedMeta
	return a
}

// Normalize normaliza el valor del artefacto según su tipo.
func (a *Artifact) Normalize() {
	a.Value = strings.TrimSpace(a.Value)

	switch a.Type {
	case ArtifactTypeDomain, ArtifactTypeSubdomain:
		a.Value = normalizeDomain(a.Value)
	case ArtifactTypeEmail:
		a.Value = normalizeEmail(a.Value)
	case ArtifactTypeIP, ArtifactTypeIPv6:
		a.Value = normalizeIP(a.Value)
	case ArtifactTypeURL:
		a.Value = normalizeURL(a.Value)
	}
}

// GenerateID genera un ID único basado en el tipo y valor del artefacto.
func (a *Artifact) GenerateID() string {
	h := sha256.New()
	h.Write([]byte(string(a.Type) + ":" + a.Value))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// Key retorna una clave única para el artefacto (type:value).
func (a *Artifact) Key() string {
	return string(a.Type) + ":" + a.Value
}

// AddSource añade una fuente a la lista sin duplicados.
func (a *Artifact) AddSource(source string) {
	if source == "" {
		return
	}
	for _, s := range a.Sources {
		if s == source {
			return
		}
	}
	a.Sources = append(a.Sources, source)
}

// AddTag añade un tag sin duplicados.
func (a *Artifact) AddTag(tag string) {
	if tag == "" {
		return
	}
	for _, t := range a.Tags {
		if t == tag {
			return
		}
	}
	a.Tags = append(a.Tags, tag)
}

// Merge combina datos de otro artefacto del mismo tipo y valor.
func (a *Artifact) Merge(other *Artifact) error {
	if a.Key() != other.Key() {
		return fmt.Errorf("cannot merge artifacts with different keys: %s != %s", a.Key(), other.Key())
	}

	// Combinar sources
	for _, s := range other.Sources {
		a.AddSource(s)
	}

	// Combinar tags
	for _, t := range other.Tags {
		a.AddTag(t)
	}

	// Merge TypedMetadata si existe
	// Si el artifact actual no tiene metadata, tomar el del otro
	if a.TypedMetadata == nil && other.TypedMetadata != nil {
		a.TypedMetadata = other.TypedMetadata
	}
	// Si ambos tienen metadata, mantener el actual (no sobreescribir)
	// En el futuro podríamos implementar un Merge() más inteligente en cada tipo de metadata

	// Usar la confianza máxima
	if other.Confidence > a.Confidence {
		a.Confidence = other.Confidence
	}

	// Usar el timestamp más antiguo (primer descubrimiento)
	if other.DiscoveredAt.Before(a.DiscoveredAt) {
		a.DiscoveredAt = other.DiscoveredAt
	}

	return nil
}

// IsValid verifica si el artefacto tiene datos válidos.
func (a *Artifact) IsValid() bool {
	if a.Type == "" || a.Value == "" {
		return false
	}
	if !a.Type.IsValid() {
		return false
	}
	if a.Confidence < 0.0 || a.Confidence > 1.0 {
		return false
	}
	return true
}

// String retorna una representación legible del artefacto.
func (a *Artifact) String() string {
	return fmt.Sprintf("[%s] %s (sources: %d, confidence: %.2f)",
		a.Type, a.Value, len(a.Sources), a.Confidence)
}

// Funciones de normalización privadas

func normalizeDomain(v string) string {
	v = strings.ToLower(v)
	v = strings.TrimSpace(v)
	v = strings.TrimSuffix(v, ".")
	v = strings.TrimPrefix(v, "*.")
	v = strings.TrimPrefix(v, "www.")
	return v
}

func normalizeEmail(v string) string {
	v = strings.ToLower(v)
	v = strings.TrimSpace(v)
	return v
}

func normalizeIP(v string) string {
	v = strings.TrimSpace(v)
	// Aquí podríamos usar net.ParseIP para validación adicional
	return v
}

func normalizeURL(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ToLower(v)
	// Normalización básica, podría mejorarse con url.Parse
	return v
}
