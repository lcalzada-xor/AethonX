# Arquitectura de Artefactos en AethonX

## Índice
- [Visión General](#visión-general)
- [El Artefacto: Entidad Central](#el-artefacto-entidad-central)
- [Tipos de Artefactos](#tipos-de-artefactos)
- [Sistema de Metadata](#sistema-de-metadata)
- [Sistema de Relaciones](#sistema-de-relaciones)
- [Ciclo de Vida de un Artefacto](#ciclo-de-vida-de-un-artefacto)
- [Normalización y Validación](#normalización-y-validación)
- [Serialización JSON](#serialización-json)
- [Builders y Helpers](#builders-y-helpers)
- [Casos de Uso Prácticos](#casos-de-uso-prácticos)

---

## Visión General

El **Artefacto** (`Artifact`) es la entidad central de datos en AethonX. Representa cualquier **dato descubierto** durante el reconocimiento: dominios, IPs, URLs, certificados, tecnologías, vulnerabilidades, etc.

### Principios de Diseño

1. **Type-Safe**: Metadata tipado con interfaces y structs concretos
2. **Extensible**: Fácil agregar nuevos tipos de artefactos y metadata
3. **Normalizado**: Validación y normalización automática por tipo
4. **Relacional**: Graph de relaciones entre artefactos (dominio → IP, subdomain → domain, etc.)
5. **Trazable**: Múltiples fuentes por artefacto, timestamps, confianza

### Ubicación en la Arquitectura

```
internal/core/domain/
├── artifact.go              # Entidad Artifact + lógica core
├── artifact_types.go        # 42 tipos de artefactos (enums)
├── enums.go                 # Enums de modos y tipos
├── builders.go              # Constructores especializados
├── scan_result.go           # Contenedor de artifacts
├── target.go                # Target de escaneo
└── metadata/                # Sistema de metadata tipado
    ├── metadata.go          # Interfaz base ArtifactMetadata
    ├── serializer.go        # Serialización polimórfica
    ├── domain.go            # DomainMetadata
    ├── certificate.go       # CertificateMetadata
    ├── ip.go                # IPMetadata
    ├── service.go           # ServiceMetadata
    ├── technology.go        # TechnologyMetadata
    ├── waf.go               # WAFMetadata
    ├── backup_file.go       # BackupFileMetadata
    ├── storage_bucket.go    # StorageBucketMetadata
    ├── api.go               # APIMetadata
    ├── repository.go        # RepositoryMetadata
    ├── webshell.go          # WebshellMetadata
    ├── registrar.go         # RegistrarMetadata
    └── contact.go           # ContactMetadata
```

---

## El Artefacto: Entidad Central

### Estructura del Artifact

```go
type Artifact struct {
    // ID único (SHA256 hash de Type + Value)
    ID string

    // Tipo de artefacto (domain, ip, url, certificate, etc.)
    Type ArtifactType

    // Valor normalizado del artefacto
    Value string

    // Fuentes que descubrieron este artefacto
    Sources []string

    // Metadata tipado y estructurado (type-safe)
    TypedMetadata metadata.ArtifactMetadata `json:"-"`

    // Relaciones con otros artifacts (graph)
    Relations []ArtifactRelation

    // Confianza del descubrimiento [0.0-1.0]
    Confidence float64

    // Timestamp del descubrimiento
    DiscoveredAt time.Time

    // Tags para categorización adicional
    Tags []string
}
```

### Campos Clave

**ID**: Hash SHA256 (primeros 16 caracteres)
- Generado automáticamente desde `Type:Value`
- Garantiza unicidad global
- Usado como clave primaria en el grafo de relaciones

**Type**: Enum `ArtifactType`
- 42 tipos predefinidos (ver sección siguiente)
- Categorizado en 6 grupos: infrastructure, web, security, cloud, data, contact
- Valida tipo de metadata esperado

**Value**: String normalizado
- Normalización automática según tipo (lowercase, trim, etc.)
- Validación tipo-específica (regex, parsers)
- Usado como parte de la clave de deduplicación

**Sources**: []string
- Lista de fuentes que descubrieron el artefacto
- Permite merge de descubrimientos duplicados
- Ejemplo: `["crtsh", "rdap", "subfinder"]`

**TypedMetadata**: Interface polimórfica
- Metadata estructurado y type-safe
- Diferente struct por tipo de artefacto
- Serialización custom vía MetadataEnvelope

**Relations**: []ArtifactRelation
- Graph de relaciones dirigidas con otros artifacts
- Tipos de relación: `resolves_to`, `subdomain_of`, `uses_cert`, etc.
- Metadata adicional por relación (confidence, source, timestamps)

**Confidence**: float64 [0.0-1.0]
- Indica confianza del descubrimiento
- Por defecto: 1.0 (alta confianza)
- Usado en merge (se toma el máximo)

**DiscoveredAt**: time.Time
- Timestamp del primer descubrimiento
- En merge, se mantiene el más antiguo
- Útil para tracking temporal

**Tags**: []string
- Categorización adicional libre
- Ejemplo: `["production", "high-value", "external"]`
- Sin duplicados (AddTag valida)

---

## Tipos de Artefactos

AethonX define **42 tipos de artefactos** organizados en **6 categorías**.

### 1. Infraestructura de Red (11 tipos)

```go
const (
    ArtifactTypeDomain       ArtifactType = "domain"        // Dominio principal
    ArtifactTypeSubdomain    ArtifactType = "subdomain"     // Subdominio
    ArtifactTypeIP           ArtifactType = "ip"            // IPv4
    ArtifactTypeIPv6         ArtifactType = "ipv6"          // IPv6
    ArtifactTypeCIDR         ArtifactType = "cidr"          // Rango de red
    ArtifactTypeASN          ArtifactType = "asn"           // Autonomous System Number
    ArtifactTypePort         ArtifactType = "port"          // Puerto abierto
    ArtifactTypeService      ArtifactType = "service"       // Servicio en puerto
    ArtifactTypeDNSRecord    ArtifactType = "dns_record"    // Registro DNS
    ArtifactTypeNameserver   ArtifactType = "nameserver"    // NS autoritativo
    ArtifactTypeMXRecord     ArtifactType = "mx_record"     // Mail Exchange
)
```

**Uso**: Enumeración de infraestructura básica (dominios, IPs, puertos, DNS).

### 2. Aplicaciones Web (11 tipos)

```go
const (
    ArtifactTypeURL          ArtifactType = "url"           // URL completa
    ArtifactTypeEndpoint     ArtifactType = "endpoint"      // API endpoint
    ArtifactTypeAPI          ArtifactType = "api"           // API con schema
    ArtifactTypeTechnology   ArtifactType = "technology"    // Tecnología detectada
    ArtifactTypeHTTPHeader   ArtifactType = "http_header"   // Header HTTP
    ArtifactTypeCookie       ArtifactType = "cookie"        // Cookie detectada
    ArtifactTypeForm         ArtifactType = "form"          // Formulario HTML
    ArtifactTypeParameter    ArtifactType = "parameter"     // Parámetro URL/POST
    ArtifactTypeJavaScript   ArtifactType = "javascript"    // Archivo JS
    ArtifactTypeRedirect     ArtifactType = "redirect"      // Redirección
    ArtifactTypeWAF          ArtifactType = "waf"           // WAF detectado
)
```

**Uso**: Enumeración de aplicaciones web y sus componentes.

### 3. Certificados y Seguridad (5 tipos)

```go
const (
    ArtifactTypeCertificate      ArtifactType = "certificate"       // Cert SSL/TLS
    ArtifactTypeVulnerability    ArtifactType = "vulnerability"     // Vulnerabilidad
    ArtifactTypeSecurityHeader   ArtifactType = "security_header"   // Header seguridad
    ArtifactTypeTLSConfig        ArtifactType = "tls_config"        // Config TLS
    ArtifactTypeSSHKey           ArtifactType = "ssh_key"           // SSH key pública
)
```

**Uso**: Análisis de seguridad y certificados.

### 4. Cloud (4 tipos)

```go
const (
    ArtifactTypeCloudResource ArtifactType = "cloud_resource"  // Recurso cloud
    ArtifactTypeCDNEndpoint   ArtifactType = "cdn_endpoint"    // CDN endpoint
    ArtifactTypeContainer     ArtifactType = "container"       // Contenedor Docker
    ArtifactTypeStorageBucket ArtifactType = "storage_bucket"  // S3/Azure/GCP bucket
)
```

**Uso**: Descubrimiento de recursos cloud.

### 5. Datos y Contenido (6 tipos)

```go
const (
    ArtifactTypeCredential      ArtifactType = "credential"       // Credenciales expuestas
    ArtifactTypeSensitiveFile   ArtifactType = "sensitive_file"   // Archivos sensibles
    ArtifactTypeBackupFile      ArtifactType = "backup_file"      // Backups (.bak, .old)
    ArtifactTypeRepository      ArtifactType = "repository"       // Repo de código (.git)
    ArtifactTypeWebshell        ArtifactType = "webshell"         // Webshell detectada
    ArtifactTypeMetadata        ArtifactType = "metadata"         // Metadatos extraídos
)
```

**Uso**: Detección de exposiciones y leaks de datos.

### 6. Información de Contacto (4 tipos)

```go
const (
    ArtifactTypeEmail         ArtifactType = "email"          // Email
    ArtifactTypePhone         ArtifactType = "phone"          // Teléfono
    ArtifactTypeSocialMedia   ArtifactType = "social_media"   // Perfil social
    ArtifactTypeWhoisContact  ArtifactType = "whois_contact"  // Contacto WHOIS
)
```

**Uso**: Información de contacto y OSINT.

### Métodos de ArtifactType

```go
// IsValid verifica si el tipo es válido
func (t ArtifactType) IsValid() bool

// Category retorna la categoría ("infrastructure", "web", "security", etc.)
func (t ArtifactType) Category() string

// String retorna la representación string
func (t ArtifactType) String() string
```

---

## Sistema de Metadata

El sistema de metadata de AethonX es **type-safe** y **polimórfico**, permitiendo adjuntar información estructurada específica a cada tipo de artefacto.

### Arquitectura del Metadata

```
┌──────────────────────────────────────────────────┐
│         ArtifactMetadata (Interface)             │
│  - ToMap() map[string]string                     │
│  - FromMap(map) error                            │
│  - IsValid() bool                                │
│  - Type() string                                 │
└──────────────────────────────────────────────────┘
                        ▲
                        │ implements
         ┌──────────────┴──────────────┬─────────────────┐
         │                             │                 │
┌────────┴──────────┐     ┌───────────┴──────────┐     ...
│  DomainMetadata   │     │  CertificateMetadata │
│  - HasSSL         │     │  - Issuer            │
│  - SSLIssuer      │     │  - SerialNumber      │
│  - DNSRecords[]   │     │  - NotBefore         │
│  - Technologies[] │     │  - NotAfter          │
└───────────────────┘     └──────────────────────┘
```

### Interface ArtifactMetadata

**Ubicación**: `internal/core/domain/metadata/metadata.go`

```go
type ArtifactMetadata interface {
    // ToMap convierte el metadata a mapa para serialización
    ToMap() map[string]string

    // FromMap carga el metadata desde un mapa
    FromMap(m map[string]string) error

    // IsValid verifica si el metadata es válido
    IsValid() bool

    // Type retorna el tipo de metadata (para debugging/logging)
    Type() string
}
```

**Propósito**: Interfaz común para todos los tipos de metadata.

### Tipos de Metadata Disponibles

#### 1. DomainMetadata (`domain.go`)

Metadata para dominios y subdominios.

```go
type DomainMetadata struct {
    // SSL/TLS
    HasSSL         bool
    SSLIssuer      string
    SSLExpiry      string
    SSLValid       bool

    // DNS Records
    DNSRecords     []string  // A, AAAA, CNAME, etc.
    Nameservers    []string
    MXRecords      []string

    // Tecnologías detectadas
    Technologies   []string

    // Servidor web
    WebServer      string
    ResponseCode   int

    // Información adicional
    ContentLength  int64
    Title          string
    IsAlive        bool
}
```

**Uso**: Enriquecer artefactos de tipo `domain` y `subdomain`.

#### 2. CertificateMetadata (`certificate.go`)

Metadata para certificados SSL/TLS.

```go
type CertificateMetadata struct {
    // Identificación
    SerialNumber     string
    Fingerprint      string

    // Emisor
    Issuer           string
    IssuerOrg        string

    // Sujeto
    Subject          string
    SubjectOrg       string

    // Validez
    NotBefore        string  // RFC3339
    NotAfter         string  // RFC3339
    IsExpired        bool
    DaysUntilExpiry  int

    // SANs (Subject Alternative Names)
    SANs             []string

    // Algoritmos
    SignatureAlgo    string
    PublicKeyAlgo    string
    KeySize          int

    // Cadena
    IssuingChain     []string
    IsSelfSigned     bool
}
```

**Uso**: Análisis de certificados descubiertos por crt.sh, Certificate Transparency logs, etc.

#### 3. IPMetadata (`ip.go`)

Metadata para direcciones IP.

```go
type IPMetadata struct {
    // Geolocalización
    Country       string
    City          string
    Latitude      float64
    Longitude     float64

    // Red
    ASN           int
    ASNOrg        string
    ISP           string
    Hostname      string  // Reverse DNS

    // Cloud
    IsCloud       bool
    CloudProvider string  // "aws", "azure", "gcp", "cloudflare"
    CloudRegion   string

    // Características
    IsVPN         bool
    IsTor         bool
    IsProxy       bool
    IsCDN         bool

    // Reputación
    ThreatScore   float64  // 0.0 (seguro) - 1.0 (peligroso)
    IsBlacklisted bool
    Blacklists    []string
}
```

**Uso**: Enriquecimiento de IPs con geolocalización, ASN, cloud providers, reputación.

#### 4. ServiceMetadata (`service.go`)

Metadata para servicios de red (puertos).

```go
type ServiceMetadata struct {
    // Identificación
    Name          string  // "http", "ssh", "mysql"
    Port          int
    Protocol      string  // "tcp", "udp"

    // Versión
    Version       string
    ProductName   string
    OSType        string

    // Banner
    Banner        string
    RawBanner     string

    // Estado
    State         string  // "open", "filtered", "closed"
    Reason        string

    // Información adicional
    ExtraInfo     string
    CPEs          []string  // Common Platform Enumeration
}
```

**Uso**: Datos de escaneo de puertos (Nmap, Masscan, Shodan).

#### 5. TechnologyMetadata (`technology.go`)

Metadata para tecnologías web detectadas.

```go
type TechnologyMetadata struct {
    // Identificación
    Name           string
    Version        string
    Category       string  // "cms", "framework", "server", "cdn", etc.

    // Detección
    Confidence     float64
    DetectionMethod string  // "header", "cookie", "body", "meta", "script"

    // Información adicional
    Website        string  // URL oficial
    Icon           string  // URL del icono
    CPE            string  // Common Platform Enumeration

    // Vulnerabilidades conocidas
    HasVulns       bool
    VulnCount      int
}
```

**Uso**: Wappalyzer, BuiltWith, detección de CMS/frameworks.

#### 6-13. Otros Metadatas

- **WAFMetadata**: WAF detectado (nombre, fabricante, tipo)
- **BackupFileMetadata**: Archivos de backup expuestos (filename, size, extension, hash)
- **StorageBucketMetadata**: Buckets de almacenamiento (provider, name, public, listable)
- **APIMetadata**: APIs descubiertas (type, baseURL, version, auth, endpoints)
- **RepositoryMetadata**: Repositorios de código (type, url, exposed, files)
- **WebshellMetadata**: Webshells detectadas (name, type, language, suspicious patterns)
- **RegistrarMetadata**: Información del registrar (WHOIS)
- **ContactMetadata**: Información de contacto (WHOIS, OSINT)

### Serialización de Metadata: MetadataEnvelope

**Problema**: JSON no soporta polimorfismo directo (interface desconocida).

**Solución**: `MetadataEnvelope` wrapper con type discriminator.

**Ubicación**: `internal/core/domain/metadata/serializer.go`

```go
type MetadataEnvelope struct {
    Type string          `json:"type"` // "domain", "certificate", "ip", etc.
    Data json.RawMessage `json:"data"` // Datos específicos del tipo
}
```

**TypeRegistry**: Mapea tipos string a factories.

```go
var TypeRegistry = map[string]func() ArtifactMetadata{
    "domain":        func() ArtifactMetadata { return &DomainMetadata{} },
    "certificate":   func() ArtifactMetadata { return &CertificateMetadata{} },
    "ip":            func() ArtifactMetadata { return &IPMetadata{} },
    // ... 10 tipos más
}
```

**Funciones de serialización**:

```go
// MarshalMetadata: ArtifactMetadata -> MetadataEnvelope
func MarshalMetadata(meta ArtifactMetadata) (*MetadataEnvelope, error)

// UnmarshalMetadata: MetadataEnvelope -> ArtifactMetadata concreto
func UnmarshalMetadata(envelope *MetadataEnvelope) (ArtifactMetadata, error)
```

**Flujo**:
1. Artifact se serializa a JSON
2. `MarshalJSON()` custom llama `MarshalMetadata()`
3. Metadata se wrappea en `MetadataEnvelope` con type discriminator
4. Al deserializar, `UnmarshalMetadata()` lee el `type` y crea instancia correcta

**Ejemplo JSON**:
```json
{
  "id": "a3f5b2c1d4e6f7g8",
  "type": "subdomain",
  "value": "api.example.com",
  "sources": ["crtsh", "rdap"],
  "metadata": {
    "type": "domain",
    "data": {
      "has_ssl": true,
      "ssl_issuer": "Let's Encrypt",
      "technologies": ["nginx", "nodejs"]
    }
  },
  "relations": [],
  "confidence": 1.0,
  "discovered_at": "2025-10-20T12:00:00Z",
  "tags": []
}
```

---

## Sistema de Relaciones

Los artifacts no existen aislados: forman un **grafo dirigido** de relaciones.

### Estructura de Relación

```go
type ArtifactRelation struct {
    // Tipo de relación
    Type RelationType  // "resolves_to", "subdomain_of", etc.

    // ID del artifact relacionado
    TargetID string

    // Confianza de la relación [0.0-1.0]
    Confidence float64

    // Timestamp del descubrimiento
    DiscoveredAt time.Time

    // Fuente que descubrió la relación
    Source string

    // Metadata adicional específico de la relación
    Metadata map[string]string
}
```

### Tipos de Relaciones

#### Relaciones de Infraestructura

```go
const (
    RelationResolvesTo       RelationType = "resolves_to"       // Domain → IP
    RelationReverseResolves  RelationType = "reverse_resolves"  // IP → Domain
    RelationOwnedBy          RelationType = "owned_by"          // IP → ASN
    RelationHostedOn         RelationType = "hosted_on"         // URL → Domain
    RelationSubdomainOf      RelationType = "subdomain_of"      // Subdomain → Domain
)
```

**Uso**: Mapear infraestructura de red.

#### Relaciones de Seguridad

```go
const (
    RelationUsesCert     RelationType = "uses_cert"     // Domain → Certificate
    RelationProtectedBy  RelationType = "protected_by"  // Domain → WAF
    RelationHasVuln      RelationType = "has_vuln"      // Service → Vulnerability
)
```

**Uso**: Análisis de seguridad.

#### Relaciones de Servicios

```go
const (
    RelationRunsOn    RelationType = "runs_on"     // Service → Port
    RelationListensOn RelationType = "listens_on"  // IP → Port
    RelationServes    RelationType = "serves"      // Port → Service
)
```

**Uso**: Mapeo de servicios en puertos.

#### Relaciones DNS

```go
const (
    RelationHasNameserver RelationType = "has_nameserver"  // Domain → Nameserver
    RelationHasMX         RelationType = "has_mx"          // Domain → MXRecord
    RelationHasCNAME      RelationType = "has_cname"       // Domain → Domain
)
```

**Uso**: Graph DNS.

#### Relaciones de Contacto

```go
const (
    RelationHasContact RelationType = "has_contact"  // Domain → Email
    RelationManagedBy  RelationType = "managed_by"   // Domain → WhoisContact
)
```

**Uso**: OSINT y puntos de contacto.

#### Relaciones de Tecnología

```go
const (
    RelationUsesTech RelationType = "uses_tech"  // URL → Technology
)
```

**Uso**: Stack tecnológico.

### API de Relaciones

**Agregar relación**:
```go
artifact.AddRelation(targetID, relType, confidence, source)
artifact.AddRelationWithMetadata(targetID, relType, confidence, source, metadata)
```

**Consultar relaciones**:
```go
// Todas las relaciones de un tipo
rels := artifact.GetRelations(RelationResolvesTo)

// Todas las relaciones
allRels := artifact.GetAllRelations()

// Verificar existencia
exists := artifact.HasRelation(targetID, RelationSubdomainOf)
```

**Eliminar relación**:
```go
artifact.RemoveRelation(targetID, relType)
```

**Conteo**:
```go
count := artifact.GetRelationCount()
```

### Ejemplo: Graph de Subdominios

```go
// Dominio principal
domain := domain.NewArtifact(ArtifactTypeDomain, "example.com", "manual")

// Subdominios descubiertos
sub1 := domain.NewArtifact(ArtifactTypeSubdomain, "api.example.com", "crtsh")
sub2 := domain.NewArtifact(ArtifactTypeSubdomain, "admin.example.com", "crtsh")

// IPs resueltas
ip1 := domain.NewArtifact(ArtifactTypeIP, "203.0.113.10", "dns")
ip2 := domain.NewArtifact(ArtifactTypeIP, "203.0.113.20", "dns")

// Crear relaciones
sub1.AddRelation(domain.ID, RelationSubdomainOf, 1.0, "crtsh")
sub2.AddRelation(domain.ID, RelationSubdomainOf, 1.0, "crtsh")
sub1.AddRelation(ip1.ID, RelationResolvesTo, 1.0, "dns")
sub2.AddRelation(ip2.ID, RelationResolvesTo, 1.0, "dns")

// Graph resultante:
// example.com <-- subdomain_of --- api.example.com --> resolves_to --> 203.0.113.10
//             <-- subdomain_of --- admin.example.com -> resolves_to --> 203.0.113.20
```

---

## Ciclo de Vida de un Artefacto

### 1. Creación

**Constructor básico**:
```go
artifact := domain.NewArtifact(
    domain.ArtifactTypeSubdomain,
    "test.example.com",
    "crtsh",
)
```

**Pasos automáticos**:
1. Asigna `Type`, `Value`, `Sources`
2. Llama `Normalize()` para normalizar `Value`
3. Genera `ID` único vía SHA256
4. Inicializa campos: `Confidence=1.0`, `DiscoveredAt=now()`, `Relations=[]`, `Tags=[]`

**Constructor con metadata**:
```go
domainMeta := metadata.NewDomainMetadata()
domainMeta.HasSSL = true
domainMeta.SSLIssuer = "Let's Encrypt"

artifact := domain.NewArtifactWithMetadata(
    domain.ArtifactTypeSubdomain,
    "test.example.com",
    "crtsh",
    domainMeta,
)
```

### 2. Normalización

**Automática en creación**: Llama `Normalize()` internamente.

**Reglas por tipo**:
- **Domain/Subdomain**: Lowercase, trim trailing dot, remove `*.`, remove `www.`
- **Email**: Lowercase, trim spaces
- **IP**: Parsing canónico vía `net.ParseIP()`
- **URL**: Lowercase scheme/host, remove default ports (80/443), trim trailing slash

**Validación**: Usa `internal/platform/validator` para validación centralizada.

### 3. Validación

**Manual**: Llamar `artifact.IsValid()` antes de usar.

```go
if !artifact.IsValid() {
    log.Warn("invalid artifact", "artifact", artifact)
    continue
}
```

**Validaciones por tipo**:
- Type no vacío y válido (`ArtifactType.IsValid()`)
- Value no vacío tras normalización
- Confidence en rango [0.0-1.0]
- Validaciones específicas:
  - IP: `validator.IsIP()`
  - Email: `validator.IsEmail()`
  - URL: `validator.IsURL()`
  - Domain: `validator.IsDomain()`
  - Port: `validator.IsPort()`
  - Certificate: `validator.IsCertSerial()`

### 4. Enriquecimiento

**Agregar metadata**:
```go
// Crear metadata
domainMeta := metadata.NewDomainMetadata()
domainMeta.HasSSL = true
domainMeta.SSLIssuer = "Let's Encrypt"
domainMeta.Technologies = []string{"nginx", "php"}

// Asignar al artifact
artifact.SetDomainMetadata(domainMeta)
```

**Agregar relaciones**:
```go
// Relación con dominio padre
artifact.AddRelation(parentDomain.ID, domain.RelationSubdomainOf, 1.0, "crtsh")

// Relación con IP
artifact.AddRelation(ip.ID, domain.RelationResolvesTo, 1.0, "dns")
```

**Agregar tags**:
```go
artifact.AddTag("production")
artifact.AddTag("external")
artifact.AddTag("high-value")
```

### 5. Merge (Deduplicación)

Cuando múltiples fuentes descubren el mismo artifact, se hace **merge**.

```go
// Artifact existente
existingArtifact := dedupeMap[artifact.Key()]

// Merge
if err := existingArtifact.Merge(newArtifact); err != nil {
    log.Error("merge failed", "error", err)
}
```

**Lógica de merge**:
1. Verificar misma `Key()` (Type:Value)
2. Combinar `Sources` (sin duplicados)
3. Combinar `Tags` (sin duplicados)
4. Combinar `Relations` (sin duplicados basado en TargetID + Type)
5. Merge `TypedMetadata`: Si actual es nil, tomar del otro; si ambos tienen, mantener actual
6. Confidence: Tomar máximo
7. DiscoveredAt: Tomar el más antiguo (primer descubrimiento)

### 6. Serialización

**JSON custom** vía `MarshalJSON()` y `UnmarshalJSON()`.

**Serialización**:
```go
jsonData, err := json.Marshal(artifact)
```

**Deserialización**:
```go
var artifact domain.Artifact
err := json.Unmarshal(jsonData, &artifact)
```

**Características**:
- TypedMetadata se serializa como `MetadataEnvelope` (type + data)
- Permite deserializar metadata concreto automáticamente
- Compatible con persistencia JSON (archivos, APIs)

### 7. Persistencia

**En ScanResult**:
```go
result := domain.NewScanResult(target)
result.Artifacts = append(result.Artifacts, artifact)
```

**Export a JSON**:
```go
output.OutputJSON(cfg.OutputDir, result)
```

**Export a Table**:
```go
output.OutputTable(result)
```

---

## Normalización y Validación

### Normalización Centralizada

**Ubicación**: `internal/platform/validator/validator.go`

**Funciones de normalización**:

```go
// Dominios
validator.NormalizeDomain(domain)  // Lowercase, trim dot/www

// Emails
validator.NormalizeEmail(email)    // Lowercase

// IPs
validator.NormalizeIP(ip)          // Canonical parsing

// URLs
validator.NormalizeURL(url)        // Lowercase scheme/host, remove default ports

// Otros
validator.NormalizeCertSerial(serial)  // Lowercase, trim
validator.NormalizeHash(hash)          // Lowercase, trim
```

### Validación Centralizada

**Funciones de validación**:

```go
// Dominios
validator.IsDomain(domain)              // RFC-compliant
validator.IsSubdomain(sub, base)        // Verifica si sub es subdomain de base

// Red
validator.IsIP(ip)                      // IPv4 o IPv6
validator.IsIPv4(ip)                    // IPv4 específico
validator.IsIPv6(ip)                    // IPv6 específico
validator.IsPort(port)                  // Rango [1-65535]

// Contacto
validator.IsEmail(email)                // RFC 5322 simplificado

// URLs
validator.IsURL(url)                    // Scheme + Host requeridos

// Certificados
validator.IsCertSerial(serial)          // Hexadecimal

// Hashes
validator.IsHash(hash)                  // MD5/SHA1/SHA256/SHA512
```

**Ventajas**:
- Consistencia en todo el codebase
- Fácil testing (validators centralizados)
- No circular imports (platform → core, no inverso)

### Flujo de Normalización en Artifact

```
1. NewArtifact(type, value, source)
   ↓
2. Artifact.Normalize()
   ↓
3. Switch por Type
   ├─ domain/subdomain → normalizeDomain(value)
   ├─ email → normalizeEmail(value)
   ├─ ip/ipv6 → normalizeIP(value)
   └─ url → normalizeURL(value)
   ↓
4. Value normalizado asignado
   ↓
5. GenerateID() usa Value normalizado
```

---

## Serialización JSON

### Custom Marshaling

**Problema**: `TypedMetadata` es interface, JSON no puede serializar polimórficamente.

**Solución**: Custom `MarshalJSON()` / `UnmarshalJSON()`.

### MarshalJSON (Serialización)

**Código** (`artifact.go:424-451`):

```go
func (a *Artifact) MarshalJSON() ([]byte, error) {
    // 1. Serializar metadata tipado a MetadataEnvelope
    var metaEnvelope *metadata.MetadataEnvelope
    if a.TypedMetadata != nil {
        var err error
        metaEnvelope, err = metadata.MarshalMetadata(a.TypedMetadata)
        if err != nil {
            return nil, fmt.Errorf("failed to marshal typed metadata: %w", err)
        }
    }

    // 2. Crear estructura auxiliar con envelope
    aux := artifactJSON{
        ID:           a.ID,
        Type:         a.Type,
        Value:        a.Value,
        Sources:      a.Sources,
        Metadata:     metaEnvelope,  // ← MetadataEnvelope
        Relations:    a.Relations,
        Confidence:   a.Confidence,
        DiscoveredAt: a.DiscoveredAt,
        Tags:         a.Tags,
    }

    // 3. Marshal estándar
    return json.Marshal(aux)
}
```

**Pasos**:
1. Convertir `TypedMetadata` (interface) a `MetadataEnvelope` (struct serializable)
2. Crear struct auxiliar `artifactJSON` con envelope
3. Marshal estándar a JSON

### UnmarshalJSON (Deserialización)

**Código** (`artifact.go:453-482`):

```go
func (a *Artifact) UnmarshalJSON(data []byte) error {
    // 1. Deserializar a estructura auxiliar
    var aux artifactJSON
    if err := json.Unmarshal(data, &aux); err != nil {
        return err
    }

    // 2. Asignar campos simples
    a.ID = aux.ID
    a.Type = aux.Type
    a.Value = aux.Value
    a.Sources = aux.Sources
    a.Relations = aux.Relations
    a.Confidence = aux.Confidence
    a.DiscoveredAt = aux.DiscoveredAt
    a.Tags = aux.Tags

    // 3. Deserializar metadata tipado desde envelope
    if aux.Metadata != nil {
        var err error
        a.TypedMetadata, err = metadata.UnmarshalMetadata(aux.Metadata)
        if err != nil {
            return fmt.Errorf("failed to unmarshal typed metadata: %w", err)
        }
    }

    return nil
}
```

**Pasos**:
1. Unmarshal JSON a struct auxiliar `artifactJSON`
2. Asignar campos simples
3. Convertir `MetadataEnvelope` (struct) a `TypedMetadata` (interface concreto)

### Ejemplo JSON Completo

```json
{
  "id": "a3f5b2c1d4e6f7g8",
  "type": "subdomain",
  "value": "api.example.com",
  "sources": ["crtsh", "subfinder"],
  "metadata": {
    "type": "domain",
    "data": {
      "has_ssl": true,
      "ssl_issuer": "Let's Encrypt",
      "ssl_expiry": "2025-12-31T23:59:59Z",
      "ssl_valid": true,
      "dns_records": ["A", "AAAA"],
      "nameservers": ["ns1.example.com", "ns2.example.com"],
      "technologies": ["nginx", "nodejs", "express"],
      "web_server": "nginx/1.18.0",
      "response_code": 200,
      "content_length": 4567,
      "title": "API Documentation",
      "is_alive": true
    }
  },
  "relations": [
    {
      "type": "subdomain_of",
      "target_id": "b4c6d8e1f3a5g7h9",
      "confidence": 1.0,
      "discovered_at": "2025-10-20T12:00:00Z",
      "source": "crtsh",
      "metadata": {}
    },
    {
      "type": "resolves_to",
      "target_id": "c5d7e9f1a3b5g7i9",
      "confidence": 1.0,
      "discovered_at": "2025-10-20T12:01:00Z",
      "source": "dns",
      "metadata": {}
    }
  ],
  "confidence": 1.0,
  "discovered_at": "2025-10-20T12:00:00Z",
  "tags": ["production", "api"]
}
```

---

## Builders y Helpers

### Constructores Especializados

**Ubicación**: `internal/core/domain/builders.go`

**Propósito**: Simplificar creación de artifacts con metadata pre-configurado.

**Ejemplos**:

```go
// Domain
artifact := domain.NewDomainArtifact("example.com", "manual")

// Subdomain
artifact := domain.NewSubdomainArtifact("api.example.com", "crtsh")

// IP
artifact := domain.NewIPArtifact("203.0.113.10", "dns")

// Technology
artifact := domain.NewTechnologyArtifact("nginx", "1.18.0", "wappalyzer")

// Service
artifact := domain.NewServiceArtifact("http", 80, "nmap")

// WAF
artifact := domain.NewWAFArtifact("Cloudflare", "wafwoof")

// API
artifact := domain.NewAPIArtifact("rest", "https://api.example.com", "manual")

// Repository
artifact := domain.NewRepositoryArtifact("git", "https://github.com/user/repo", "manual")

// Backup File
artifact := domain.NewBackupFileArtifact("database.sql.bak", "crawler")

// Storage Bucket
artifact := domain.NewStorageBucketArtifact("aws", "my-bucket", "s3scanner")

// Webshell
artifact := domain.NewWebshellArtifact("c99.php", "php", "scanner")
```

**Ventajas**:
- Constructor tipado (no confundir parámetros)
- Metadata inicializado automáticamente
- Menos boilerplate en sources

### Getters de Metadata

**Propósito**: Type assertion seguro para obtener metadata concreto.

```go
// Domain metadata
if domainMeta := artifact.GetDomainMetadata(); domainMeta != nil {
    log.Info("SSL Issuer", "issuer", domainMeta.SSLIssuer)
}

// IP metadata
if ipMeta := artifact.GetIPMetadata(); ipMeta != nil {
    log.Info("Country", "country", ipMeta.Country)
    log.Info("ASN", "asn", ipMeta.ASN)
}

// Technology metadata
if techMeta := artifact.GetTechnologyMetadata(); techMeta != nil {
    log.Info("Version", "version", techMeta.Version)
    log.Info("Category", "category", techMeta.Category)
}

// Certificate metadata
if certMeta := artifact.GetCertificateMetadata(); certMeta != nil {
    log.Info("Issuer", "issuer", certMeta.Issuer)
    log.Info("Expires", "expires", certMeta.NotAfter)
}
```

### Setters de Metadata

**Propósito**: Asignar metadata tipado a artifact existente.

```go
// Crear artifact básico
artifact := domain.NewArtifact(domain.ArtifactTypeDomain, "example.com", "manual")

// Crear y asignar metadata
domainMeta := metadata.NewDomainMetadata()
domainMeta.HasSSL = true
domainMeta.SSLIssuer = "Let's Encrypt"
domainMeta.Technologies = []string{"nginx", "php"}
artifact.SetDomainMetadata(domainMeta)
```

---

## Casos de Uso Prácticos

### Caso 1: Source que Descubre Subdominios

**Ejemplo**: Source crt.sh descubre subdominios vía Certificate Transparency.

```go
func (c *CRTsh) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
    result := domain.NewScanResult(target)

    // Consultar crt.sh API
    subdomains, err := c.queryCRTsh(target.Domain)
    if err != nil {
        return result, err
    }

    // Crear artifacts
    for _, subdomain := range subdomains {
        // Crear artifact de subdomain
        artifact := domain.NewSubdomainArtifact(subdomain, "crtsh")

        // Agregar relación con dominio padre
        parentID := generateID(domain.ArtifactTypeDomain, target.Domain)
        artifact.AddRelation(parentID, domain.RelationSubdomainOf, 1.0, "crtsh")

        // Agregar al resultado
        result.Artifacts = append(result.Artifacts, artifact)
    }

    return result, nil
}
```

### Caso 2: Source que Enriquece con Metadata

**Ejemplo**: Source RDAP enriquece dominios con metadata WHOIS.

```go
func (r *RDAP) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
    result := domain.NewScanResult(target)

    // Consultar RDAP
    whoisData, err := r.queryRDAP(target.Domain)
    if err != nil {
        return result, err
    }

    // Crear artifact de dominio
    artifact := domain.NewDomainArtifact(target.Domain, "rdap")

    // Enriquecer con metadata
    domainMeta := artifact.GetDomainMetadata()
    domainMeta.Nameservers = whoisData.Nameservers
    domainMeta.MXRecords = whoisData.MXRecords

    // Crear artifact de registrar
    registrarArtifact := domain.NewArtifact(
        domain.ArtifactTypeWhoisContact,
        whoisData.RegistrarName,
        "rdap",
    )

    // Relación: dominio managed_by registrar
    artifact.AddRelation(registrarArtifact.ID, domain.RelationManagedBy, 1.0, "rdap")

    // Agregar artifacts
    result.Artifacts = append(result.Artifacts, artifact, registrarArtifact)

    return result, nil
}
```

### Caso 3: Graph Builder

**Ejemplo**: Construir grafo de relaciones automáticamente.

```go
// GraphService en internal/core/usecases/graph_service.go
func (gs *GraphService) Build(artifacts []*domain.Artifact) error {
    // Índice por ID
    index := make(map[string]*domain.Artifact)
    for _, artifact := range artifacts {
        index[artifact.ID] = artifact
    }

    // Detectar relaciones implícitas
    for _, artifact := range artifacts {
        switch artifact.Type {
        case domain.ArtifactTypeSubdomain:
            // Extraer dominio padre
            parentDomain := extractParentDomain(artifact.Value)

            // Buscar artifact del dominio padre
            for id, parent := range index {
                if parent.Type == domain.ArtifactTypeDomain &&
                   parent.Value == parentDomain {
                    // Crear relación subdomain_of
                    artifact.AddRelation(id, domain.RelationSubdomainOf, 0.9, "graph_service")
                    break
                }
            }

        case domain.ArtifactTypeURL:
            // Extraer dominio de la URL
            urlDomain := extractDomainFromURL(artifact.Value)

            // Buscar artifact del dominio
            for id, parent := range index {
                if parent.Type == domain.ArtifactTypeDomain &&
                   parent.Value == urlDomain {
                    // Crear relación hosted_on
                    artifact.AddRelation(id, domain.RelationHostedOn, 1.0, "graph_service")
                    break
                }
            }
        }
    }

    return nil
}
```

### Caso 4: Deduplicación con Merge

**Ejemplo**: DedupeService elimina duplicados y merge sources.

```go
// DedupeService en internal/core/usecases/dedupe_service.go
func (ds *DedupeService) Deduplicate(artifacts []*domain.Artifact) []*domain.Artifact {
    dedupeMap := make(map[string]*domain.Artifact)

    for _, artifact := range artifacts {
        key := artifact.Key() // "type:value"

        if existing, found := dedupeMap[key]; found {
            // Merge con artifact existente
            if err := existing.Merge(artifact); err != nil {
                ds.logger.Warn("merge failed", "key", key, "error", err)
                continue
            }
        } else {
            // Primer descubrimiento
            dedupeMap[key] = artifact
        }
    }

    // Convertir mapa a slice
    unique := make([]*domain.Artifact, 0, len(dedupeMap))
    for _, artifact := range dedupeMap {
        unique = append(unique, artifact)
    }

    return unique
}
```

---

## Resumen

### Puntos Clave

1. **Artifact es la entidad central**: Representa cualquier dato descubierto.
2. **42 tipos organizados en 6 categorías**: Infrastructure, Web, Security, Cloud, Data, Contact.
3. **Metadata tipado y polimórfico**: 13 tipos de metadata concretos, serialización vía MetadataEnvelope.
4. **Grafo de relaciones dirigido**: 16 tipos de relaciones entre artifacts.
5. **Normalización automática**: Delegada a `internal/platform/validator`.
6. **Validación tipo-específica**: Antes de usar, llamar `IsValid()`.
7. **Serialización JSON custom**: Maneja polimorfismo de metadata.
8. **Builders especializados**: Simplifican creación con metadata pre-configurado.
9. **Merge inteligente**: Combina sources, tags, relations, metadata.
10. **Trazabilidad completa**: Sources, timestamps, confidence, tags.

### Archivos Clave

| Archivo | Propósito |
|---------|-----------|
| `artifact.go` | Entidad Artifact + lógica core |
| `artifact_types.go` | 42 tipos de artefactos (enums) |
| `enums.go` | Enums de modos (Scan, Source) y tipos (Source) |
| `builders.go` | Constructores especializados con metadata |
| `metadata/metadata.go` | Interfaz ArtifactMetadata |
| `metadata/serializer.go` | MetadataEnvelope + polimorfismo |
| `metadata/*.go` | 13 tipos concretos de metadata |

### Flujo Típico

```
1. Source descubre datos
   ↓
2. Crea Artifacts con NewArtifact() o builders
   ↓
3. Normalización automática (Normalize)
   ↓
4. Validación manual (IsValid)
   ↓
5. Enriquecimiento (metadata, tags, relaciones)
   ↓
6. Agregación en ScanResult
   ↓
7. Deduplicación (Merge)
   ↓
8. Graph building (relaciones implícitas)
   ↓
9. Serialización JSON
   ↓
10. Export (JSON, Table, DB)
```

---

## Referencias

- **Código fuente**: `internal/core/domain/`
- **CLAUDE.md**: Guía de arquitectura general
- **ARCHITECTURE.md**: Visión Clean Architecture
- **Tests**: `internal/core/domain/*_test.go`

**Autor**: Lucas Calzada
**Fecha**: 2025-10-20
**Versión**: 1.0.0
