// internal/core/domain/artifact.go
package domain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
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
	// Usa custom serialization via MarshalJSON/UnmarshalJSON
	TypedMetadata metadata.ArtifactMetadata `json:"-"`

	// Relations contiene las relaciones con otros artifacts
	Relations []ArtifactRelation

	// Confidence indica la confianza del descubrimiento [0.0-1.0]
	Confidence float64

	// DiscoveredAt timestamp del descubrimiento
	DiscoveredAt time.Time

	// Tags permite categorización adicional
	Tags []string
}

// ArtifactRelation representa una relación dirigida entre dos artifacts.
type ArtifactRelation struct {
	// Type es el tipo de relación (e.g., "uses_cert", "resolves_to")
	Type RelationType

	// TargetID es el ID del artifact relacionado
	TargetID string

	// Confidence indica la confianza de esta relación [0.0-1.0]
	Confidence float64

	// DiscoveredAt es cuándo se descubrió esta relación
	DiscoveredAt time.Time

	// Source es la fuente que descubrió esta relación
	Source string

	// Metadata contiene contexto adicional específico de la relación
	Metadata map[string]string
}

// RelationType define el tipo de relación entre artifacts.
type RelationType string

// Relaciones de infraestructura
const (
	RelationResolvesTo       RelationType = "resolves_to"        // Domain/Subdomain -> IP
	RelationReverseResolves  RelationType = "reverse_resolves"   // IP -> Domain
	RelationOwnedBy          RelationType = "owned_by"           // IP -> ASN
	RelationHostedOn         RelationType = "hosted_on"          // URL -> Domain
	RelationSubdomainOf      RelationType = "subdomain_of"       // Subdomain -> Domain
)

// Relaciones de seguridad
const (
	RelationUsesCert     RelationType = "uses_cert"      // Domain -> Certificate
	RelationProtectedBy  RelationType = "protected_by"   // Domain -> WAF
	RelationHasVuln      RelationType = "has_vuln"       // Service -> Vulnerability
)

// Relaciones de servicios
const (
	RelationRunsOn    RelationType = "runs_on"     // Service -> Port
	RelationListensOn RelationType = "listens_on"  // IP -> Port
	RelationServes    RelationType = "serves"      // Port -> Service
)

// Relaciones DNS
const (
	RelationHasNameserver RelationType = "has_nameserver" // Domain -> Nameserver
	RelationHasMX         RelationType = "has_mx"         // Domain -> MXRecord
	RelationHasCNAME      RelationType = "has_cname"      // Domain -> Domain
)

// Relaciones de contacto
const (
	RelationHasContact RelationType = "has_contact"  // Domain -> Email
	RelationManagedBy  RelationType = "managed_by"   // Domain -> WhoisContact
)

// Relaciones de tecnología
const (
	RelationUsesTech RelationType = "uses_tech" // URL -> Technology
)

// NewArtifact crea un nuevo artefacto con valores por defecto.
func NewArtifact(artifactType ArtifactType, value, source string) *Artifact {
	a := &Artifact{
		Type:         artifactType,
		Value:        value,
		Sources:      []string{source},
		Relations:    []ArtifactRelation{},
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

// AddRelation añade una relación con otro artifact.
func (a *Artifact) AddRelation(targetID string, relType RelationType, confidence float64, source string) {
	// No añadir relaciones duplicadas
	if a.HasRelation(targetID, relType) {
		return
	}

	relation := ArtifactRelation{
		Type:         relType,
		TargetID:     targetID,
		Confidence:   confidence,
		DiscoveredAt: time.Now(),
		Source:       source,
		Metadata:     make(map[string]string),
	}

	a.Relations = append(a.Relations, relation)
}

// AddRelationWithMetadata añade una relación con metadata adicional.
func (a *Artifact) AddRelationWithMetadata(targetID string, relType RelationType, confidence float64, source string, metadata map[string]string) {
	if a.HasRelation(targetID, relType) {
		return
	}

	relation := ArtifactRelation{
		Type:         relType,
		TargetID:     targetID,
		Confidence:   confidence,
		DiscoveredAt: time.Now(),
		Source:       source,
		Metadata:     metadata,
	}

	a.Relations = append(a.Relations, relation)
}

// GetRelations retorna todas las relaciones de un tipo específico.
func (a *Artifact) GetRelations(relType RelationType) []ArtifactRelation {
	var results []ArtifactRelation
	for _, rel := range a.Relations {
		if rel.Type == relType {
			results = append(results, rel)
		}
	}
	return results
}

// GetAllRelations retorna todas las relaciones del artifact.
func (a *Artifact) GetAllRelations() []ArtifactRelation {
	return a.Relations
}

// HasRelation verifica si existe una relación específica.
func (a *Artifact) HasRelation(targetID string, relType RelationType) bool {
	for _, rel := range a.Relations {
		if rel.TargetID == targetID && rel.Type == relType {
			return true
		}
	}
	return false
}

// RemoveRelation elimina una relación específica.
func (a *Artifact) RemoveRelation(targetID string, relType RelationType) {
	newRelations := make([]ArtifactRelation, 0, len(a.Relations))
	for _, rel := range a.Relations {
		if !(rel.TargetID == targetID && rel.Type == relType) {
			newRelations = append(newRelations, rel)
		}
	}
	a.Relations = newRelations
}

// GetRelationCount retorna el número total de relaciones.
func (a *Artifact) GetRelationCount() int {
	return len(a.Relations)
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

	// Combinar relaciones (evitar duplicados)
	for _, rel := range other.Relations {
		if !a.HasRelation(rel.TargetID, rel.Type) {
			a.Relations = append(a.Relations, rel)
		}
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
	// Basic checks
	if a.Type == "" || a.Value == "" {
		return false
	}
	if !a.Type.IsValid() {
		return false
	}
	if a.Confidence < 0.0 || a.Confidence > 1.0 {
		return false
	}

	// Type-specific validation
	switch a.Type {
	case ArtifactTypeIP, ArtifactTypeIPv6:
		// IP was already normalized; if Value is empty after normalization, it was invalid
		if a.Value == "" {
			return false
		}
		// Additional check: verify it's a valid IP
		if net.ParseIP(a.Value) == nil {
			return false
		}

	case ArtifactTypeEmail:
		if !isValidEmail(a.Value) {
			return false
		}

	case ArtifactTypeURL:
		if _, err := url.ParseRequestURI(a.Value); err != nil {
			return false
		}

	case ArtifactTypeDomain, ArtifactTypeSubdomain:
		if !isValidDomain(a.Value) {
			return false
		}

	case ArtifactTypePort:
		if !isValidPort(a.Value) {
			return false
		}

	case ArtifactTypeCertificate:
		if !isValidCertSerial(a.Value) {
			return false
		}
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

	// Parse and validate IP address
	ip := net.ParseIP(v)
	if ip == nil {
		// If parsing fails, return empty string (invalid IP)
		return ""
	}

	// Return canonical form
	// IPv4 addresses are returned in dotted notation: "192.168.1.1"
	// IPv6 addresses are returned in canonical form: "2001:db8::1"
	return ip.String()
}

func normalizeURL(v string) string {
	v = strings.TrimSpace(v)

	// Parse URL
	u, err := url.Parse(v)
	if err != nil {
		// If parsing fails, return lowercase trimmed version as fallback
		return strings.ToLower(v)
	}

	// Normalize components
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)

	// Remove default ports
	if (u.Scheme == "http" && strings.HasSuffix(u.Host, ":80")) {
		u.Host = strings.TrimSuffix(u.Host, ":80")
	}
	if (u.Scheme == "https" && strings.HasSuffix(u.Host, ":443")) {
		u.Host = strings.TrimSuffix(u.Host, ":443")
	}

	// Remove trailing slash from path if it's the only character
	if u.Path == "/" && u.RawQuery == "" && u.Fragment == "" {
		u.Path = ""
	}

	return u.String()
}

// Validation functions

// emailRegex is a simplified RFC 5322 email validation regex
// Note: Full RFC 5322 is complex; this covers 99% of valid emails
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// isValidEmail validates email format (simplified RFC 5322)
func isValidEmail(email string) bool {
	if len(email) < 3 || len(email) > 254 {
		return false
	}
	return emailRegex.MatchString(email)
}

// Note: isValidDomain is defined in target.go and reused here

// isValidPort validates port range [1-65535]
func isValidPort(portStr string) bool {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return false
	}
	return port >= 1 && port <= 65535
}

// isValidCertSerial validates certificate serial number format (hex string)
func isValidCertSerial(serial string) bool {
	if len(serial) == 0 {
		return false
	}

	// Serial numbers are typically hex strings
	// Allow 0-9, a-f, A-F, and optional colons/spaces
	for _, ch := range serial {
		if !((ch >= '0' && ch <= '9') ||
			(ch >= 'a' && ch <= 'f') ||
			(ch >= 'A' && ch <= 'F') ||
			ch == ':' || ch == ' ') {
			return false
		}
	}

	return true
}

// artifactJSON es una estructura auxiliar para serialización custom.
type artifactJSON struct {
	ID            string                      `json:"id"`
	Type          ArtifactType                `json:"type"`
	Value         string                      `json:"value"`
	Sources       []string                    `json:"sources"`
	Metadata      *metadata.MetadataEnvelope  `json:"metadata,omitempty"`
	Relations     []ArtifactRelation          `json:"relations"`
	Confidence    float64                     `json:"confidence"`
	DiscoveredAt  time.Time                   `json:"discovered_at"`
	Tags          []string                    `json:"tags"`
}

// MarshalJSON implementa custom JSON marshaling para Artifact.
// Serializa TypedMetadata usando MetadataEnvelope.
func (a *Artifact) MarshalJSON() ([]byte, error) {
	// Serializar metadata tipado
	var metaEnvelope *metadata.MetadataEnvelope
	if a.TypedMetadata != nil {
		var err error
		metaEnvelope, err = metadata.MarshalMetadata(a.TypedMetadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal typed metadata: %w", err)
		}
	}

	// Crear estructura auxiliar
	aux := artifactJSON{
		ID:           a.ID,
		Type:         a.Type,
		Value:        a.Value,
		Sources:      a.Sources,
		Metadata:     metaEnvelope,
		Relations:    a.Relations,
		Confidence:   a.Confidence,
		DiscoveredAt: a.DiscoveredAt,
		Tags:         a.Tags,
	}

	return json.Marshal(aux)
}

// UnmarshalJSON implementa custom JSON unmarshaling para Artifact.
// Deserializa MetadataEnvelope a TypedMetadata concreto.
func (a *Artifact) UnmarshalJSON(data []byte) error {
	// Deserializar a estructura auxiliar
	var aux artifactJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Asignar campos simples
	a.ID = aux.ID
	a.Type = aux.Type
	a.Value = aux.Value
	a.Sources = aux.Sources
	a.Relations = aux.Relations
	a.Confidence = aux.Confidence
	a.DiscoveredAt = aux.DiscoveredAt
	a.Tags = aux.Tags

	// Deserializar metadata tipado
	if aux.Metadata != nil {
		var err error
		a.TypedMetadata, err = metadata.UnmarshalMetadata(aux.Metadata)
		if err != nil {
			return fmt.Errorf("failed to unmarshal typed metadata: %w", err)
		}
	}

	return nil
}
