# Artifact Architecture in AethonX

## Table of Contents
- [Overview](#overview)
- [The Artifact Entity](#the-artifact-entity)
- [Artifact Types](#artifact-types)
- [Metadata System](#metadata-system)
- [Relationship System](#relationship-system)
- [Lifecycle](#lifecycle)
- [Practical Examples](#practical-examples)

---

## Overview

The **Artifact** is the central data entity in AethonX. It represents any **discovered data** during reconnaissance: domains, IPs, URLs, certificates, technologies, vulnerabilities, etc.

### Design Principles

1. **Type-Safe**: Typed metadata with interfaces and concrete structs
2. **Extensible**: Easy to add new artifact types and metadata
3. **Normalized**: Automatic validation and normalization by type
4. **Relational**: Graph of relationships between artifacts
5. **Traceable**: Multiple sources, timestamps, confidence

### Architecture Location

```
internal/core/domain/
├── artifact.go              # Artifact entity + core logic
├── artifact_types.go        # 42 artifact types (enums)
├── enums.go                 # Mode and type enums
├── builders.go              # Specialized constructors
├── scan_result.go           # Artifact container
├── target.go                # Scan target
└── metadata/                # Typed metadata system
    ├── metadata.go          # Base ArtifactMetadata interface
    ├── serializer.go        # Polymorphic serialization
    ├── domain.go            # DomainMetadata
    ├── certificate.go       # CertificateMetadata
    ├── ip.go                # IPMetadata
    ├── service.go           # ServiceMetadata
    └── ... (9 more types)
```

---

## The Artifact Entity

### Structure

```go
type Artifact struct {
    // Unique ID (SHA256 hash of Type + Value)
    ID string

    // Artifact type (domain, ip, url, certificate, etc.)
    Type ArtifactType

    // Normalized artifact value
    Value string

    // Sources that discovered this artifact
    Sources []string

    // Typed and structured metadata (type-safe)
    TypedMetadata metadata.ArtifactMetadata `json:"-"`

    // Relations with other artifacts (graph)
    Relations []ArtifactRelation

    // Discovery confidence [0.0-1.0]
    Confidence float64

    // Discovery timestamp
    DiscoveredAt time.Time

    // Tags for additional categorization
    Tags []string
}
```

### Key Fields

**ID**: SHA256 hash (first 16 characters)
- Auto-generated from `Type:Value`
- Guarantees global uniqueness
- Used as primary key in relationship graph

**Type**: Enum `ArtifactType`
- 42 predefined types
- Categorized in 6 groups: infrastructure, web, security, cloud, data, contact
- Validates expected metadata type

**Value**: Normalized string
- Automatic normalization by type (lowercase, trim, etc.)
- Type-specific validation (regex, parsers)
- Used as part of deduplication key

**Sources**: []string
- List of sources that discovered the artifact
- Allows merging duplicate discoveries
- Example: `["crtsh", "rdap", "subfinder"]`

**TypedMetadata**: Polymorphic interface
- Structured and type-safe metadata
- Different struct per artifact type
- Custom serialization via MetadataEnvelope

**Relations**: []ArtifactRelation
- Directed graph of relationships with other artifacts
- Relation types: `resolves_to`, `subdomain_of`, `uses_cert`, etc.
- Additional metadata per relation

**Confidence**: float64 [0.0-1.0]
- Indicates discovery confidence
- Default: 1.0 (high confidence)
- Used in merge (maximum is taken)

---

## Artifact Types

AethonX defines **42 artifact types** organized in **6 categories**.

### 1. Network Infrastructure (11 types)

```go
const (
    ArtifactTypeDomain       ArtifactType = "domain"        // Main domain
    ArtifactTypeSubdomain    ArtifactType = "subdomain"     // Subdomain
    ArtifactTypeIP           ArtifactType = "ip"            // IPv4
    ArtifactTypeIPv6         ArtifactType = "ipv6"          // IPv6
    ArtifactTypeCIDR         ArtifactType = "cidr"          // Network range
    ArtifactTypeASN          ArtifactType = "asn"           // Autonomous System Number
    ArtifactTypePort         ArtifactType = "port"          // Open port
    ArtifactTypeService      ArtifactType = "service"       // Service on port
    ArtifactTypeDNSRecord    ArtifactType = "dns_record"    // DNS record
    ArtifactTypeNameserver   ArtifactType = "nameserver"    // Authoritative NS
    ArtifactTypeMXRecord     ArtifactType = "mx_record"     // Mail Exchange
)
```

### 2. Web Applications (11 types)

```go
const (
    ArtifactTypeURL          ArtifactType = "url"           // Full URL
    ArtifactTypeEndpoint     ArtifactType = "endpoint"      // API endpoint
    ArtifactTypeAPI          ArtifactType = "api"           // API with schema
    ArtifactTypeTechnology   ArtifactType = "technology"    // Detected technology
    ArtifactTypeHTTPHeader   ArtifactType = "http_header"   // HTTP header
    ArtifactTypeCookie       ArtifactType = "cookie"        // Detected cookie
    ArtifactTypeForm         ArtifactType = "form"          // HTML form
    ArtifactTypeParameter    ArtifactType = "parameter"     // URL/POST parameter
    ArtifactTypeJavaScript   ArtifactType = "javascript"    // JS file
    ArtifactTypeRedirect     ArtifactType = "redirect"      // Redirection
    ArtifactTypeWAF          ArtifactType = "waf"           // Detected WAF
)
```

### 3. Certificates and Security (5 types)

```go
const (
    ArtifactTypeCertificate      ArtifactType = "certificate"       // SSL/TLS cert
    ArtifactTypeVulnerability    ArtifactType = "vulnerability"     // Vulnerability
    ArtifactTypeSecurityHeader   ArtifactType = "security_header"   // Security header
    ArtifactTypeTLSConfig        ArtifactType = "tls_config"        // TLS config
    ArtifactTypeSSHKey           ArtifactType = "ssh_key"           // Public SSH key
)
```

### 4. Cloud (4 types)

```go
const (
    ArtifactTypeCloudResource ArtifactType = "cloud_resource"  // Cloud resource
    ArtifactTypeCDNEndpoint   ArtifactType = "cdn_endpoint"    // CDN endpoint
    ArtifactTypeContainer     ArtifactType = "container"       // Docker container
    ArtifactTypeStorageBucket ArtifactType = "storage_bucket"  // S3/Azure/GCP bucket
)
```

### 5. Data and Content (6 types)

```go
const (
    ArtifactTypeCredential      ArtifactType = "credential"       // Exposed credentials
    ArtifactTypeSensitiveFile   ArtifactType = "sensitive_file"   // Sensitive files
    ArtifactTypeBackupFile      ArtifactType = "backup_file"      // Backups (.bak, .old)
    ArtifactTypeRepository      ArtifactType = "repository"       // Code repo (.git)
    ArtifactTypeWebshell        ArtifactType = "webshell"         // Detected webshell
    ArtifactTypeMetadata        ArtifactType = "metadata"         // Extracted metadata
)
```

### 6. Contact Information (4 types)

```go
const (
    ArtifactTypeEmail         ArtifactType = "email"          // Email
    ArtifactTypePhone         ArtifactType = "phone"          // Phone
    ArtifactTypeSocialMedia   ArtifactType = "social_media"   // Social profile
    ArtifactTypeWhoisContact  ArtifactType = "whois_contact"  // WHOIS contact
)
```

---

## Metadata System

The metadata system is **type-safe** and **polymorphic**, allowing structured information specific to each artifact type.

### Metadata Interface

```go
type ArtifactMetadata interface {
    ToMap() map[string]string           // Convert to map for serialization
    FromMap(m map[string]string) error  // Load from map
    IsValid() bool                      // Validate metadata
    Type() string                       // Type name (for debugging)
}
```

### Available Metadata Types

#### 1. DomainMetadata

Metadata for domains and subdomains.

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

    // Detected technologies
    Technologies   []string

    // Web server
    WebServer      string
    ResponseCode   int
    ContentLength  int64
    Title          string
    IsAlive        bool
}
```

#### 2. CertificateMetadata

Metadata for SSL/TLS certificates.

```go
type CertificateMetadata struct {
    SerialNumber     string
    Fingerprint      string
    Issuer           string
    IssuerOrg        string
    Subject          string
    SubjectOrg       string
    NotBefore        string  // RFC3339
    NotAfter         string  // RFC3339
    IsExpired        bool
    DaysUntilExpiry  int
    SANs             []string
    SignatureAlgo    string
    PublicKeyAlgo    string
    KeySize          int
}
```

#### 3. IPMetadata

Metadata for IP addresses.

```go
type IPMetadata struct {
    // Geolocation
    Country       string
    City          string
    Latitude      float64
    Longitude     float64

    // Network
    ASN           int
    ASNOrg        string
    ISP           string
    Hostname      string  // Reverse DNS

    // Cloud
    IsCloud       bool
    CloudProvider string  // "aws", "azure", "gcp", "cloudflare"
    CloudRegion   string

    // Reputation
    ThreatScore   float64  // 0.0 (safe) - 1.0 (dangerous)
    IsBlacklisted bool
}
```

#### 4-13. Other Metadata Types

- **ServiceMetadata**: Network services (port, protocol, version, banner)
- **TechnologyMetadata**: Web technologies (name, version, category)
- **WAFMetadata**: WAF detection (name, manufacturer, type)
- **BackupFileMetadata**: Backup files (filename, size, extension, hash)
- **StorageBucketMetadata**: Storage buckets (provider, name, public, listable)
- **APIMetadata**: APIs (type, baseURL, version, auth, endpoints)
- **RepositoryMetadata**: Code repositories (type, url, exposed, files)
- **WebshellMetadata**: Webshells (name, type, language)
- **RegistrarMetadata**: Registrar info (WHOIS)
- **ContactMetadata**: Contact info (WHOIS, OSINT)

### Metadata Serialization

**Problem**: JSON doesn't support direct polymorphism (unknown interface).

**Solution**: `MetadataEnvelope` wrapper with type discriminator.

```go
type MetadataEnvelope struct {
    Type string          `json:"type"` // "domain", "certificate", "ip", etc.
    Data json.RawMessage `json:"data"` // Type-specific data
}
```

**Functions**:
```go
// MarshalMetadata: ArtifactMetadata -> MetadataEnvelope
func MarshalMetadata(meta ArtifactMetadata) (*MetadataEnvelope, error)

// UnmarshalMetadata: MetadataEnvelope -> concrete ArtifactMetadata
func UnmarshalMetadata(envelope *MetadataEnvelope) (ArtifactMetadata, error)
```

---

## Relationship System

Artifacts form a **directed graph** of relationships.

### Relation Structure

```go
type ArtifactRelation struct {
    Type         RelationType  // "resolves_to", "subdomain_of", etc.
    TargetID     string        // Related artifact ID
    Confidence   float64       // Relation confidence [0.0-1.0]
    DiscoveredAt time.Time     // Discovery timestamp
    Source       string        // Source that discovered relation
    Metadata     map[string]string  // Additional metadata
}
```

### Relation Types

**Infrastructure Relations**:
```go
RelationResolvesTo       // Domain → IP
RelationReverseResolves  // IP → Domain
RelationOwnedBy          // IP → ASN
RelationHostedOn         // URL → Domain
RelationSubdomainOf      // Subdomain → Domain
```

**Security Relations**:
```go
RelationUsesCert     // Domain → Certificate
RelationProtectedBy  // Domain → WAF
RelationHasVuln      // Service → Vulnerability
```

**Service Relations**:
```go
RelationRunsOn    // Service → Port
RelationListensOn // IP → Port
RelationServes    // Port → Service
```

**DNS Relations**:
```go
RelationHasNameserver // Domain → Nameserver
RelationHasMX         // Domain → MXRecord
RelationHasCNAME      // Domain → Domain
```

### Relation API

```go
// Add relation
artifact.AddRelation(targetID, relType, confidence, source)

// Query relations
rels := artifact.GetRelations(RelationResolvesTo)
allRels := artifact.GetAllRelations()

// Check existence
exists := artifact.HasRelation(targetID, RelationSubdomainOf)

// Remove relation
artifact.RemoveRelation(targetID, relType)

// Count
count := artifact.GetRelationCount()
```

---

## Lifecycle

### 1. Creation

**Basic constructor**:
```go
artifact := domain.NewArtifact(
    domain.ArtifactTypeSubdomain,
    "test.example.com",
    "crtsh",
)
```

**With metadata**:
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

### 2. Normalization

**Automatic on creation**: Calls `Normalize()` internally.

**Rules by type**:
- **Domain/Subdomain**: Lowercase, trim trailing dot, remove `*.`, remove `www.`
- **Email**: Lowercase, trim spaces
- **IP**: Canonical parsing via `net.ParseIP()`
- **URL**: Lowercase scheme/host, remove default ports (80/443), trim trailing slash

### 3. Validation

```go
if !artifact.IsValid() {
    log.Warn("invalid artifact", "artifact", artifact)
    continue
}
```

**Validations by type**:
- Type not empty and valid
- Value not empty after normalization
- Confidence in range [0.0-1.0]
- Type-specific validations (IP, email, URL, domain, port, etc.)

### 4. Enrichment

**Add metadata**:
```go
domainMeta := metadata.NewDomainMetadata()
domainMeta.HasSSL = true
artifact.SetDomainMetadata(domainMeta)
```

**Add relations**:
```go
artifact.AddRelation(parentDomain.ID, domain.RelationSubdomainOf, 1.0, "crtsh")
artifact.AddRelation(ip.ID, domain.RelationResolvesTo, 1.0, "dns")
```

**Add tags**:
```go
artifact.AddTag("production")
artifact.AddTag("external")
```

### 5. Merge (Deduplication)

When multiple sources discover the same artifact, they are **merged**.

```go
existingArtifact := dedupeMap[artifact.Key()]
if err := existingArtifact.Merge(newArtifact); err != nil {
    log.Error("merge failed", "error", err)
}
```

**Merge logic**:
1. Verify same `Key()` (Type:Value)
2. Combine `Sources` (no duplicates)
3. Combine `Tags` (no duplicates)
4. Combine `Relations` (no duplicates based on TargetID + Type)
5. Merge `TypedMetadata`: If current is nil, take from other
6. Confidence: Take maximum
7. DiscoveredAt: Take oldest (first discovery)

### 6. Serialization

**JSON custom** via `MarshalJSON()` and `UnmarshalJSON()`.

```go
// Serialize
jsonData, err := json.Marshal(artifact)

// Deserialize
var artifact domain.Artifact
err := json.Unmarshal(jsonData, &artifact)
```

---

## Practical Examples

### Example 1: Source Discovering Subdomains

```go
func (c *CRTsh) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
    result := domain.NewScanResult(target)

    // Query crt.sh API
    subdomains, err := c.queryCRTsh(target.Domain)
    if err != nil {
        return result, err
    }

    // Create artifacts
    for _, subdomain := range subdomains {
        artifact := domain.NewSubdomainArtifact(subdomain, "crtsh")

        // Add relation with parent domain
        parentID := generateID(domain.ArtifactTypeDomain, target.Domain)
        artifact.AddRelation(parentID, domain.RelationSubdomainOf, 1.0, "crtsh")

        result.Artifacts = append(result.Artifacts, artifact)
    }

    return result, nil
}
```

### Example 2: Source Enriching with Metadata

```go
func (r *RDAP) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
    result := domain.NewScanResult(target)

    // Query RDAP
    whoisData, err := r.queryRDAP(target.Domain)
    if err != nil {
        return result, err
    }

    // Create domain artifact
    artifact := domain.NewDomainArtifact(target.Domain, "rdap")

    // Enrich with metadata
    domainMeta := artifact.GetDomainMetadata()
    domainMeta.Nameservers = whoisData.Nameservers
    domainMeta.MXRecords = whoisData.MXRecords

    result.Artifacts = append(result.Artifacts, artifact)
    return result, nil
}
```

### Example 3: Graph Builder

```go
func (gs *GraphService) Build(artifacts []*domain.Artifact) error {
    // Index by ID
    index := make(map[string]*domain.Artifact)
    for _, artifact := range artifacts {
        index[artifact.ID] = artifact
    }

    // Detect implicit relations
    for _, artifact := range artifacts {
        switch artifact.Type {
        case domain.ArtifactTypeSubdomain:
            // Extract parent domain
            parentDomain := extractParentDomain(artifact.Value)

            // Find parent artifact
            for id, parent := range index {
                if parent.Type == domain.ArtifactTypeDomain &&
                   parent.Value == parentDomain {
                    artifact.AddRelation(id, domain.RelationSubdomainOf, 0.9, "graph_service")
                    break
                }
            }
        }
    }

    return nil
}
```

### Example 4: Deduplication with Merge

```go
func (ds *DedupeService) Deduplicate(artifacts []*domain.Artifact) []*domain.Artifact {
    dedupeMap := make(map[string]*domain.Artifact)

    for _, artifact := range artifacts {
        key := artifact.Key() // "type:value"

        if existing, found := dedupeMap[key]; found {
            // Merge with existing
            if err := existing.Merge(artifact); err != nil {
                ds.logger.Warn("merge failed", "key", key, "error", err)
                continue
            }
        } else {
            // First discovery
            dedupeMap[key] = artifact
        }
    }

    // Convert map to slice
    unique := make([]*domain.Artifact, 0, len(dedupeMap))
    for _, artifact := range dedupeMap {
        unique = append(unique, artifact)
    }

    return unique
}
```

---

## Summary

### Key Points

1. **Artifact is the central entity**: Represents any discovered data
2. **42 types in 6 categories**: Infrastructure, Web, Security, Cloud, Data, Contact
3. **Typed and polymorphic metadata**: 13 concrete metadata types
4. **Directed relationship graph**: 16 relation types between artifacts
5. **Automatic normalization**: Delegated to `internal/platform/validator`
6. **Type-specific validation**: Call `IsValid()` before use
7. **Custom JSON serialization**: Handles metadata polymorphism
8. **Specialized builders**: Simplify creation with pre-configured metadata
9. **Smart merge**: Combines sources, tags, relations, metadata
10. **Complete traceability**: Sources, timestamps, confidence, tags

### Key Files

| File | Purpose |
|------|---------|
| `artifact.go` | Artifact entity + core logic |
| `artifact_types.go` | 42 artifact types (enums) |
| `enums.go` | Mode and type enums |
| `builders.go` | Specialized constructors |
| `metadata/metadata.go` | ArtifactMetadata interface |
| `metadata/serializer.go` | MetadataEnvelope + polymorphism |
| `metadata/*.go` | 13 concrete metadata types |

### Typical Flow

```
1. Source discovers data
   ↓
2. Creates Artifacts (NewArtifact or builders)
   ↓
3. Automatic normalization (Normalize)
   ↓
4. Manual validation (IsValid)
   ↓
5. Enrichment (metadata, tags, relations)
   ↓
6. Aggregation in ScanResult
   ↓
7. Deduplication (Merge)
   ↓
8. Graph building (implicit relations)
   ↓
9. JSON serialization
   ↓
10. Export (JSON, Table)
```

---

**Author**: Lucas Calzada
**Date**: 2025-10-22
**Version**: 2.0.0
