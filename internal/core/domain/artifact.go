// internal/core/domain/artifact.go
package domain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/platform/validator"
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
	Relations []ArtifactRelation `json:"relations,omitempty"`

	// Confidence indica la confianza del descubrimiento [0.0-1.0]
	Confidence float64

	// DiscoveredAt timestamp del descubrimiento
	DiscoveredAt time.Time

	// DiscoveryContext contains metadata about how/where the artifact was discovered
	DiscoveryContext *DiscoveryContext `json:"discovery_context,omitempty"`

	// SecurityContext contains security-related classification and severity
	SecurityContext *SecurityContext `json:"security_context,omitempty"`

	// Classification contains categorization metadata
	Classification *Classification `json:"classification,omitempty"`

	// Tags permite categorización adicional (solo para tags simples, no datos estructurados)
	Tags []string `json:"tags,omitempty"`
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
	Metadata map[string]string `json:"metadata,omitempty"`
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
		// Also remove session IDs from URLs
		a.Value = normalizeEndpoint(a.Value)
	case ArtifactTypeEndpoint:
		// Remove session IDs from endpoints
		a.Value = normalizeEndpoint(a.Value)
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

// SetDiscoveryContext sets the discovery context for the artifact.
func (a *Artifact) SetDiscoveryContext(ctx *DiscoveryContext) {
	if ctx != nil && !ctx.IsEmpty() {
		a.DiscoveryContext = ctx
	}
}

// SetSecurityContext sets the security context for the artifact.
func (a *Artifact) SetSecurityContext(ctx *SecurityContext) {
	if ctx != nil && !ctx.IsEmpty() {
		a.SecurityContext = ctx
	}
}

// SetClassification sets the classification for the artifact.
func (a *Artifact) SetClassification(cls *Classification) {
	if cls != nil && !cls.IsEmpty() {
		a.Classification = cls
	}
}

// AddVulnerabilityType adds a vulnerability type to the security context.
func (a *Artifact) AddVulnerabilityType(vulnType VulnerabilityType) {
	if a.SecurityContext == nil {
		a.SecurityContext = NewSecurityContext()
	}
	a.SecurityContext.AddVulnerabilityType(vulnType)
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
// mergeTypedMetadata intelligently merges two TypedMetadata instances.
// Prefers metadata with more "richness" (e.g., HTTPStatus > 0, IsAlive = true).
// This handles the case where waybackurls creates URLs with empty metadata
// and httpx later enriches them with probe results.
func mergeTypedMetadata(current, incoming metadata.ArtifactMetadata) metadata.ArtifactMetadata {
	// Type assertion to DomainMetadata (most common case for URLs)
	currentDM, currentIsDM := current.(*metadata.DomainMetadata)
	incomingDM, incomingIsDM := incoming.(*metadata.DomainMetadata)

	// Both are DomainMetadata: intelligent merge
	if currentIsDM && incomingIsDM {
		// Prefer incoming if it has richer HTTP probe data
		if incomingDM.HTTPStatus > 0 && currentDM.HTTPStatus == 0 {
			return incoming
		}
		// Prefer incoming if it's marked as alive
		if incomingDM.IsAlive && !currentDM.IsAlive {
			return incoming
		}
		// Prefer incoming if it has probe data
		if incomingDM.ProbeSource != "" && currentDM.ProbeSource == "" {
			return incoming
		}
		// Otherwise keep current (first discovery wins)
		return current
	}

	// Type assertion to IPMetadata (enrichment case)
	currentIP, currentIsIP := current.(*metadata.IPMetadata)
	incomingIP, incomingIsIP := incoming.(*metadata.IPMetadata)

	// Both are IPMetadata: prefer the one with more information
	if currentIsIP && incomingIsIP {
		// Check which one has richer data using IsValid()
		currentValid := currentIP.IsValid()
		incomingValid := incomingIP.IsValid()

		// If only one is valid, prefer it
		if incomingValid && !currentValid {
			return incoming
		}
		if currentValid && !incomingValid {
			return current
		}

		// Both valid or both empty: prefer incoming (enrichment sources like ipapi come later)
		if incomingValid {
			return incoming
		}

		// Both empty: keep current
		return current
	}

	// Different types or not DomainMetadata/IPMetadata: prefer non-nil incoming
	if incoming != nil {
		return incoming
	}
	return current
}

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
	} else if a.TypedMetadata != nil && other.TypedMetadata != nil {
		// Ambos tienen metadata: preferir el que tiene información más rica
		// Esto resuelve el caso donde waybackurls crea URLs con metadata vacía
		// y luego httpx las procesa con metadata completa
		a.TypedMetadata = mergeTypedMetadata(a.TypedMetadata, other.TypedMetadata)
	}
	// Si other no tiene metadata, mantener el actual

	// Merge DiscoveryContext (prefer most detailed)
	if a.DiscoveryContext == nil && other.DiscoveryContext != nil {
		a.DiscoveryContext = other.DiscoveryContext
	} else if a.DiscoveryContext != nil && other.DiscoveryContext != nil {
		// Merge fields preferring non-empty values
		if a.DiscoveryContext.SourceURL == "" {
			a.DiscoveryContext.SourceURL = other.DiscoveryContext.SourceURL
		}
		if a.DiscoveryContext.SourceResource == "" {
			a.DiscoveryContext.SourceResource = other.DiscoveryContext.SourceResource
		}
		if a.DiscoveryContext.LineNumber == 0 {
			a.DiscoveryContext.LineNumber = other.DiscoveryContext.LineNumber
		}
		if a.DiscoveryContext.Context == "" {
			a.DiscoveryContext.Context = other.DiscoveryContext.Context
		}
		if a.DiscoveryContext.MatchPattern == "" {
			a.DiscoveryContext.MatchPattern = other.DiscoveryContext.MatchPattern
		}
	}

	// Merge SecurityContext (prefer higher severity)
	if a.SecurityContext == nil && other.SecurityContext != nil {
		a.SecurityContext = other.SecurityContext
	} else if a.SecurityContext != nil && other.SecurityContext != nil {
		// Prefer higher severity
		if other.SecurityContext.Severity.Weight() > a.SecurityContext.Severity.Weight() {
			a.SecurityContext.Severity = other.SecurityContext.Severity
		}
		// Merge vulnerability types
		for _, vt := range other.SecurityContext.VulnerabilityTypes {
			a.AddVulnerabilityType(vt)
		}
		// Merge other fields (prefer non-empty)
		if a.SecurityContext.TokenType == "" {
			a.SecurityContext.TokenType = other.SecurityContext.TokenType
		}
		if a.SecurityContext.CloudProvider == "" {
			a.SecurityContext.CloudProvider = other.SecurityContext.CloudProvider
		}
		if a.SecurityContext.ExposureType == "" {
			a.SecurityContext.ExposureType = other.SecurityContext.ExposureType
		}
		if !a.SecurityContext.IsSensitive && other.SecurityContext.IsSensitive {
			a.SecurityContext.IsSensitive = true
		}
	}

	// Merge Classification (prefer non-empty)
	if a.Classification == nil && other.Classification != nil {
		a.Classification = other.Classification
	} else if a.Classification != nil && other.Classification != nil {
		if a.Classification.ResourceType == "" {
			a.Classification.ResourceType = other.Classification.ResourceType
		}
		if a.Classification.ParameterType == "" {
			a.Classification.ParameterType = other.Classification.ParameterType
		}
		if a.Classification.DataType == "" {
			a.Classification.DataType = other.Classification.DataType
		}
		if !a.Classification.IsExternal && other.Classification.IsExternal {
			a.Classification.IsExternal = true
		}
		if a.Classification.Category == "" {
			a.Classification.Category = other.Classification.Category
		}
	}

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
		// Additional check: verify it's a valid IP using centralized validator
		if !validator.IsIP(a.Value) {
			return false
		}

	case ArtifactTypeEmail:
		if !isValidEmail(a.Value) {
			return false
		}

	case ArtifactTypeURL:
		if !validator.IsURL(a.Value) {
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
	// Handle wildcard prefix specific to certificates
	v = strings.TrimPrefix(v, "*.")
	// Delegate to centralized validator
	return validator.NormalizeDomain(v)
}

func normalizeEmail(v string) string {
	return validator.NormalizeEmail(v)
}

func normalizeIP(v string) string {
	return validator.NormalizeIP(v)
}

func normalizeURL(v string) string {
	return validator.NormalizeURL(v)
}

// normalizeEndpoint removes session IDs and other transient parameters from endpoints/URLs.
// This prevents duplicate artifacts for the same logical endpoint with different session tokens.
func normalizeEndpoint(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return v
	}

	// Session ID patterns to remove (Java, PHP, ASP.NET, generic)
	// Pattern format: ;sessiontype=HEXADECIMAL or ALPHANUMERIC
	sessionPatterns := []string{
		`;jsessionid=[A-F0-9]+`,              // Java (case-insensitive hex)
		`;JSESSIONID=[A-F0-9]+`,
		`;jsessionid=[a-f0-9]+`,
		`;PHPSESSID=[A-Za-z0-9]+`,            // PHP
		`;phpsessid=[A-Za-z0-9]+`,
		`;ASPSESSIONID[A-Z]+=[A-Z0-9]+`,      // ASP.NET
		`;aspsessionid[a-z]+=[a-z0-9]+`,
		`;sessionid=[A-Za-z0-9_-]+`,          // Generic sessionid
		`;SESSIONID=[A-Za-z0-9_-]+`,
		`;sid=[A-Za-z0-9_-]+`,                // Short session ID
		`;SID=[A-Za-z0-9_-]+`,
	}

	// First, remove path-based session IDs (;jsessionid=...)
	// This must be done before URL parsing as url.Parse doesn't handle ; properly
	for _, pattern := range sessionPatterns {
		re := regexp.MustCompile(pattern)
		v = re.ReplaceAllString(v, "")
	}

	// Then, normalize query parameter session IDs
	if strings.Contains(v, "?") || strings.Contains(v, "&") {
		v = normalizeSessionQueryParams(v)
	}

	return v
}

// normalizeSessionQueryParams removes session IDs from query parameters.
// Common session parameters: jsessionid, PHPSESSID, sessionid, sid, etc.
func normalizeSessionQueryParams(urlStr string) string {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		// If parsing fails, return original (better than losing data)
		return urlStr
	}

	query := parsed.Query()

	// List of common session parameter names to remove
	sessionKeys := []string{
		"jsessionid", "JSESSIONID",
		"PHPSESSID", "phpsessid",
		"sessionid", "SESSIONID",
		"sid", "SID",
		"session", "SESSION",
		"sess", "SESS",
		"s", "S",
		"ASPSESSIONID", "aspsessionid",
	}

	// Remove all session parameters
	for _, key := range sessionKeys {
		query.Del(key)
	}

	// Also remove any parameter that looks like ASP.NET session (ASPSESSIONID*)
	for key := range query {
		if strings.HasPrefix(strings.ToUpper(key), "ASPSESSIONID") {
			query.Del(key)
		}
	}

	parsed.RawQuery = query.Encode()
	return parsed.String()
}

// Validation functions - delegate to centralized validator

func isValidEmail(email string) bool {
	return validator.IsEmail(email)
}

func isValidDomain(domain string) bool {
	return validator.IsDomain(domain)
}

func isValidPort(portStr string) bool {
	return validator.IsPort(portStr)
}

func isValidCertSerial(serial string) bool {
	return validator.IsCertSerial(serial)
}

// artifactJSON es una estructura auxiliar para serialización custom.
type artifactJSON struct {
	ID               string                      `json:"id"`
	Type             ArtifactType                `json:"type"`
	Value            string                      `json:"value"`
	Sources          []string                    `json:"sources"`
	Metadata         *metadata.MetadataEnvelope  `json:"metadata,omitempty"`
	Relations        []ArtifactRelation          `json:"relations,omitempty"`
	Confidence       float64                     `json:"confidence"`
	DiscoveredAt     time.Time                   `json:"discovered_at"`
	DiscoveryContext *DiscoveryContext           `json:"discovery_context,omitempty"`
	SecurityContext  *SecurityContext            `json:"security_context,omitempty"`
	Classification   *Classification             `json:"classification,omitempty"`
	Tags             []string                    `json:"tags,omitempty"`
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
		ID:               a.ID,
		Type:             a.Type,
		Value:            a.Value,
		Sources:          a.Sources,
		Metadata:         metaEnvelope,
		Relations:        a.Relations,
		Confidence:       a.Confidence,
		DiscoveredAt:     a.DiscoveredAt,
		DiscoveryContext: a.DiscoveryContext,
		SecurityContext:  a.SecurityContext,
		Classification:   a.Classification,
		Tags:             a.Tags,
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
	a.DiscoveryContext = aux.DiscoveryContext
	a.SecurityContext = aux.SecurityContext
	a.Classification = aux.Classification
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
