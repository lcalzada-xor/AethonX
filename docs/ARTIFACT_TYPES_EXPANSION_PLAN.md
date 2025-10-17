# 📋 Plan: Expansión de Tipos de Artefactos - Fase 2

## 🎯 Objetivo

Añadir tipos críticos faltantes para reconocimiento web completo, enfocados en:
- Servicios de red detectados por port scanning
- Detección de WAF (crítico para evasión)
- APIs modernas
- Exposición de datos sensibles (repos, backups)

---

## 📊 Tipos a Implementar (7 tipos nuevos)

### 🔴 CRÍTICOS (5 tipos)

#### 1. **service**
**Relación con IP**: Service vive dentro del contexto de una IP
**Justificación**: Lo que devuelve Nmap/Masscan - servicios en puertos específicos
**Diferencia con technology**: Service es a nivel de red/puerto, technology es a nivel de aplicación web

#### 2. **waf**
**Justificación**: Crítico para pentesting - determina estrategia de evasión
**Importancia**: Alta - afecta toda la metodología de testing

#### 3. **api**
**Justificación**: APIs modernas (REST/GraphQL/gRPC) merecen tratamiento especial
**Diferencia con endpoint**: API tiene schema, versioning, autenticación compleja

#### 4. **repository**
**Justificación**: .git/.svn/.hg expuestos - alto riesgo de fuga de código
**Importancia**: Crítica - acceso a source code, credentials, history

#### 5. **backup_file**
**Justificación**: Backups expuestos (.bak, .zip, .sql, .tar.gz)
**Importancia**: Alta - pueden contener bases de datos, código, configuraciones

### 🟡 IMPORTANTES (2 tipos adicionales recomendados)

#### 6. **storage_bucket**
**Justificación**: Tipo específico para S3/Azure/GCP buckets (muy común)
**Diferencia con cloud_resource**: Storage bucket es específico y común
**Importancia**: Alta en cloud - misconfigurations frecuentes

#### 7. **webshell**
**Justificación**: Detección de webshells (c99, r57, b374k, China Chopper)
**Importancia**: Crítica - indica compromiso existente
**Uso**: Post-explotación, forensics, threat hunting

---

## 📐 Estructura de Metadata por Tipo

### 1️⃣ **ServiceMetadata** (Nuevo - Enfocado en Nmap/Masscan)

```go
type ServiceMetadata struct {
    // Identificación del servicio
    Name        string   // "mysql", "ssh", "http", "ftp", "smtp"
    Product     string   // "MySQL", "OpenSSH", "nginx", "vsftpd"
    Version     string   // "5.7.40", "8.9p1", "1.24.0"
    ExtraInfo   string   // "Ubuntu Linux; protocol 2.0"

    // Puerto y protocolo
    Port        int      // 3306, 22, 80, 21, 25
    Protocol    string   // "tcp", "udp"
    State       string   // "open", "filtered", "closed"

    // Banner y fingerprinting
    Banner      string   // Banner raw capturado
    ServiceFP   string   // Fingerprint del servicio

    // CPE (Common Platform Enumeration)
    CPE         string   // "cpe:/a:mysql:mysql:5.7.40"

    // SSL/TLS (si el servicio usa SSL)
    SSLEnabled  bool
    SSLCert     string   // Subject del certificado

    // Vulnerabilidades conocidas
    HasVulns    bool
    CVEList     []string
    RiskLevel   string   // "low", "medium", "high", "critical"

    // Script results (Nmap NSE scripts)
    ScriptResults map[string]string  // script_name -> output

    // Detección
    DetectionMethod string  // "banner", "probe", "inference"
    Confidence      float64 // 0.0-1.0
    ScanTool        string  // "nmap", "masscan", "naabu"
}
```

**Ejemplo real**:
```json
{
  "type": "service",
  "value": "MySQL 5.7.40 on tcp/3306",
  "metadata": {
    "name": "mysql",
    "product": "MySQL",
    "version": "5.7.40",
    "port": 3306,
    "protocol": "tcp",
    "state": "open",
    "banner": "5.7.40-0ubuntu0.18.04.1",
    "cpe": "cpe:/a:mysql:mysql:5.7.40",
    "has_vulns": true,
    "cve_list": "CVE-2023-21980,CVE-2023-22005",
    "risk_level": "high",
    "scan_tool": "nmap"
  }
}
```

**Relación con IP**:
```go
// El service se vincula a una IP mediante parent_ip o IP del artifact
artifact.Metadata["parent_ip"] = "1.2.3.4"
```

---

### 2️⃣ **WAFMetadata** (Nuevo - Crítico para evasión)

```go
type WAFMetadata struct {
    // Identificación del WAF
    Name        string   // "Cloudflare", "AWS WAF", "Akamai", "Imperva"
    Vendor      string   // "Cloudflare Inc.", "Amazon", "Akamai Technologies"
    Product     string   // "Cloudflare WAF", "AWS WAF v2"

    // Detección
    DetectionMethod  string    // "header", "response_pattern", "error_page", "timing"
    DetectionPattern string    // Patrón que matcheó
    Confidence       float64   // 0.0-1.0

    // Configuración detectada
    RulesMode        string    // "block", "challenge", "monitor"
    ChallengeType    string    // "captcha", "js_challenge", "managed_challenge"

    // Protecciones activas
    SQLiProtection   bool
    XSSProtection    bool
    RCEProtection    bool
    RateLimiting     bool
    BotProtection    bool
    DDoSProtection   bool

    // Fingerprinting
    Headers          []string  // Headers reveladores
    ErrorPages       []string  // Páginas de error características
    BlockedPayloads  []string  // Payloads que fueron bloqueados

    // Bypass potential
    BypassDifficulty string    // "trivial", "easy", "medium", "hard", "very_hard"
    KnownBypasses    []string  // Técnicas conocidas de bypass

    // Performance impact
    LatencyAdded     int       // Milisegundos de latencia añadidos

    // URL donde se detectó
    DetectedURL      string
}
```

**Ejemplo real**:
```json
{
  "type": "waf",
  "value": "Cloudflare",
  "metadata": {
    "name": "cloudflare",
    "vendor": "Cloudflare Inc.",
    "detection_method": "header",
    "detection_pattern": "cf-ray, __cfduid",
    "confidence": 0.98,
    "rules_mode": "challenge",
    "challenge_type": "js_challenge",
    "sqli_protection": true,
    "xss_protection": true,
    "bot_protection": true,
    "bypass_difficulty": "hard"
  }
}
```

---

### 3️⃣ **APIMetadata** (Nuevo - APIs modernas)

```go
type APIMetadata struct {
    // Identificación
    Name         string   // Nombre de la API
    Type         string   // "rest", "graphql", "soap", "grpc", "websocket"
    Version      string   // "v1", "v2", "2.0"

    // Endpoint base
    BaseURL      string   // "https://api.example.com/v1"

    // Documentación
    HasDocumentation bool
    DocsURL          string   // Swagger/OpenAPI URL
    DocsFormat       string   // "swagger", "openapi3", "raml", "graphql_schema"

    // Autenticación
    AuthRequired     bool
    AuthMethods      []string  // "bearer", "api_key", "oauth2", "basic", "jwt"
    AuthLocation     string    // "header", "query", "cookie"

    // GraphQL específico
    IntrospectionEnabled bool      // Si GraphQL introspection está activo
    HasMutations        bool
    HasSubscriptions    bool
    SchemaURL           string

    // REST específico
    Methods          []string      // GET, POST, PUT, DELETE, PATCH
    Endpoints        []string      // Lista de endpoints descubiertos
    HasRateLimit     bool
    RateLimitValue   string        // "100 req/min"

    // Seguridad
    HasCORS          bool
    CORSOrigin       string        // "*", "specific-domain.com"
    HasCSRF          bool
    HTTPS Only       bool

    // Versioning
    VersioningScheme string        // "url", "header", "query"
    SupportedVersions []string
    DeprecatedVersions []string

    // Response format
    ResponseFormat   []string       // "json", "xml", "protobuf", "msgpack"

    // Errores y comportamiento
    ErrorFormat      string         // "json", "xml", "plain"
    HasDetailedErrors bool

    // Technologies
    Framework        string         // "express", "fastapi", "spring-boot"
    Language         string         // "nodejs", "python", "java"
}
```

**Ejemplo real - REST API**:
```json
{
  "type": "api",
  "value": "https://api.example.com/v2",
  "metadata": {
    "name": "Example API",
    "type": "rest",
    "version": "v2",
    "base_url": "https://api.example.com/v2",
    "has_documentation": true,
    "docs_url": "https://api.example.com/docs",
    "docs_format": "openapi3",
    "auth_required": true,
    "auth_methods": "bearer,api_key",
    "methods": "GET,POST,PUT,DELETE",
    "has_rate_limit": true,
    "rate_limit_value": "1000/hour",
    "https_only": true,
    "response_format": "json"
  }
}
```

**Ejemplo real - GraphQL**:
```json
{
  "type": "api",
  "value": "https://api.example.com/graphql",
  "metadata": {
    "type": "graphql",
    "introspection_enabled": true,
    "has_mutations": true,
    "has_subscriptions": false,
    "schema_url": "https://api.example.com/graphql?introspection",
    "auth_required": false
  }
}
```

---

### 4️⃣ **RepositoryMetadata** (Nuevo - Repos expuestos)

```go
type RepositoryMetadata struct {
    // Tipo de repositorio
    Type         string   // "git", "svn", "mercurial", "cvs"

    // Accesibilidad
    Accessible   bool     // Si el repo es accesible públicamente
    Protected    bool     // Si tiene algún tipo de protección

    // Git específico
    GitURL       string   // URL del .git/
    HasConfig    bool     // Si .git/config es accesible
    HasHead      bool     // Si .git/HEAD es accesible
    HasLogs      bool     // Si .git/logs/ es accesible
    HasRefs      bool     // Si .git/refs/ es accesible
    HasObjects   bool     // Si .git/objects/ es accesible

    // Información extraída
    Branches     []string // Branches accesibles
    Tags         []string // Tags encontradas
    RemoteURL    string   // URL remota del repo (de .git/config)
    LastCommit   string   // Hash del último commit
    CommitCount  int      // Número de commits accesibles

    // Contenido sensible
    HasSecrets   bool     // Si se encontraron secrets en commits
    SecretTypes  []string // "api_key", "password", "token", "private_key"

    // Riesgo
    RiskLevel    string   // "low", "medium", "high", "critical"
    CanDownload  bool     // Si el repo completo es descargable

    // Metadatos adicionales
    Size         int64    // Tamaño estimado en bytes
    FileCount    int      // Número de archivos

    // Tools para download
    DownloadTool string   // "git-dumper", "dvcs-ripper", "wget"
}
```

**Ejemplo real**:
```json
{
  "type": "repository",
  "value": ".git exposed at https://example.com/.git/",
  "metadata": {
    "type": "git",
    "accessible": true,
    "protected": false,
    "git_url": "https://example.com/.git/",
    "has_config": true,
    "has_head": true,
    "has_logs": true,
    "has_objects": true,
    "remote_url": "git@github.com:company/secret-project.git",
    "commit_count": 247,
    "has_secrets": true,
    "secret_types": "api_key,password,aws_key",
    "risk_level": "critical",
    "can_download": true,
    "download_tool": "git-dumper"
  }
}
```

---

### 5️⃣ **BackupFileMetadata** (Nuevo - Backups expuestos)

```go
type BackupFileMetadata struct {
    // Identificación del backup
    Filename     string   // "database.sql.bak", "site-backup.zip"
    Extension    string   // ".bak", ".old", ".zip", ".sql", ".tar.gz"

    // Tipo de backup
    BackupType   string   // "database", "source_code", "config", "full_site", "unknown"
    Format       string   // "sql", "zip", "tar", "gz", "7z", "rar"

    // Ubicación
    URL          string   // URL completa del archivo
    Path         string   // Path relativo

    // Tamaño y fecha
    Size         int64    // Tamaño en bytes
    SizeHuman    string   // "45.2 MB"
    LastModified string   // Fecha de última modificación

    // Accesibilidad
    Accessible   bool     // Si es accesible públicamente
    StatusCode   int      // 200, 403, etc.
    RequiresAuth bool     // Si requiere autenticación

    // Contenido (si se pudo analizar)
    IsCompressed bool
    ContainsSQL  bool     // Si contiene dumps SQL
    ContainsCode bool     // Si contiene código fuente
    ContainsConfig bool   // Si contiene configs (.env, etc.)

    // Información sensible detectada
    HasPasswords    bool
    HasAPIKeys      bool
    HasCredentials  bool
    HasPII          bool   // Personally Identifiable Information

    // Metadata del archivo
    CreatedBy    string   // Software que lo creó (si detectable)
    CompressionRatio float64

    // Riesgo
    RiskLevel    string   // "low", "medium", "high", "critical"
    Severity     string   // Impacto potencial

    // Descarga
    Downloadable bool
    Downloaded   bool     // Si ya fue descargado
    LocalPath    string   // Path local si fue descargado

    // Hashes (para deduplicación)
    MD5          string
    SHA256       string
}
```

**Ejemplo real**:
```json
{
  "type": "backup_file",
  "value": "database_backup_2024.sql.bak",
  "metadata": {
    "filename": "database_backup_2024.sql.bak",
    "extension": ".bak",
    "backup_type": "database",
    "format": "sql",
    "url": "https://example.com/backups/database_backup_2024.sql.bak",
    "size": 47482880,
    "size_human": "45.3 MB",
    "accessible": true,
    "status_code": 200,
    "contains_sql": true,
    "has_passwords": true,
    "has_credentials": true,
    "risk_level": "critical",
    "downloadable": true
  }
}
```

---

### 6️⃣ **StorageBucketMetadata** (Nuevo - S3/Azure/GCP buckets)

```go
type StorageBucketMetadata struct {
    // Proveedor
    Provider     string   // "aws_s3", "azure_blob", "gcp_storage", "digitalocean_spaces"

    // Identificación
    BucketName   string   // Nombre del bucket
    BucketURL    string   // URL del bucket
    Region       string   // us-east-1, eu-west-1, etc.

    // Acceso
    PublicAccess bool     // Si tiene acceso público
    Permissions  []string // "read", "write", "list", "delete"
    RequiresAuth bool
    AuthMethod   string   // "none", "api_key", "iam", "sas"

    // Listado
    IsListable   bool     // Si se puede listar contenido
    FileCount    int      // Número de archivos (si listable)
    TotalSize    int64    // Tamaño total en bytes

    // Contenido detectado
    FileTypes    []string // Extensiones encontradas
    HasHTML      bool
    HasJS        bool
    HasImages    bool
    HasBackups   bool
    HasLogs      bool
    HasConfigs   bool

    // Información sensible
    HasSecrets   bool
    SecretTypes  []string // "api_key", "password", "certificate"

    // Configuración
    Versioning   bool     // Si tiene versioning habilitado
    Encryption   bool     // Si tiene encryption
    Logging      bool     // Si tiene logging
    Website      bool     // Si está configurado como website

    // CORS
    HasCORS      bool
    CORSPolicy   string   // Resumen de política CORS

    // Riesgo
    RiskLevel    string   // "low", "medium", "high", "critical"
    Misconfigured bool    // Si está mal configurado

    // Metadatos AWS específicos
    S3ACL        string   // "public-read", "private", etc.
    S3Policy     string   // Bucket policy (si accesible)

    // Metadatos Azure específicos
    AzureContainer string // Nombre del container
    AzureSAS      bool    // Si usa SAS tokens

    // Detection
    DetectionMethod string // "dns", "permutation", "google_dork"
}
```

**Ejemplo real**:
```json
{
  "type": "storage_bucket",
  "value": "company-backups",
  "metadata": {
    "provider": "aws_s3",
    "bucket_name": "company-backups",
    "bucket_url": "https://company-backups.s3.amazonaws.com",
    "region": "us-east-1",
    "public_access": true,
    "permissions": "read,list",
    "is_listable": true,
    "file_count": 1247,
    "has_backups": true,
    "has_logs": true,
    "has_secrets": true,
    "secret_types": "api_key,database_password",
    "versioning": false,
    "encryption": false,
    "risk_level": "critical",
    "misconfigured": true,
    "s3_acl": "public-read"
  }
}
```

---

### 7️⃣ **WebshellMetadata** (Nuevo - Post-explotación/Forensics)

```go
type WebshellMetadata struct {
    // Identificación
    Name         string   // "c99", "r57", "b374k", "wso", "china_chopper"
    Type         string   // "php", "jsp", "asp", "aspx", "perl"
    Variant      string   // Variante específica

    // Ubicación
    URL          string   // URL del webshell
    Path         string   // Path del archivo

    // Detección
    DetectionMethod string    // "signature", "behavior", "static_analysis"
    Confidence      float64   // 0.0-1.0
    Signature       string    // Firma que matcheó

    // Características
    HasFileUpload    bool
    HasFileDownload  bool
    HasCommandExec   bool
    HasSQLClient     bool
    HasPortScanner   bool
    HasBackconnect   bool
    HasBruteForce    bool

    // Funcionalidades avanzadas
    Obfuscated       bool     // Si está ofuscado
    Encrypted        bool     // Si usa encriptación
    HasPassword      bool     // Si tiene password
    PasswordProtected bool

    // Timestamps
    FileCreated      string   // Fecha de creación
    FileModified     string   // Última modificación
    LastAccessed     string   // Último acceso

    // Hash
    MD5              string
    SHA256           string

    // Metadatos del archivo
    Size             int64
    Permissions      string   // "755", "644", etc.
    Owner            string   // Usuario propietario

    // Indicadores de compromiso
    IOCs             []string // IPs, dominios, strings únicos
    C2Servers        []string // Servidores C2 si los hay

    // Severidad
    RiskLevel        string   // "high", "critical"
    ThreatLevel      string   // Nivel de amenaza

    // Remediación
    RemediationSteps []string
}
```

**Ejemplo real**:
```json
{
  "type": "webshell",
  "value": "c99.php",
  "metadata": {
    "name": "c99",
    "type": "php",
    "url": "https://example.com/uploads/c99.php",
    "detection_method": "signature",
    "confidence": 0.95,
    "has_file_upload": true,
    "has_file_download": true,
    "has_command_exec": true,
    "has_sql_client": true,
    "obfuscated": false,
    "password_protected": true,
    "file_modified": "2024-01-15T14:30:00Z",
    "md5": "d99f51160076c93a981f34c43c6a2412",
    "risk_level": "critical",
    "threat_level": "active_compromise"
  }
}
```

---

## 🔄 Relación Service con IP

### Approach: Service como child de IP

```go
// Cuando se descubre un service en una IP:
service := domain.NewArtifact(domain.ArtifactTypeService, "MySQL 5.7.40", "nmap")
service.Metadata["parent_ip"] = "1.2.3.4"
service.Metadata["port"] = "3306"
service.Metadata["protocol"] = "tcp"

// Alternativamente, en IPMetadata añadir:
type IPMetadata struct {
    // ... campos existentes ...

    // Servicios detectados en esta IP
    Services []ServiceSummary  // Lista resumida de servicios
}

type ServiceSummary struct {
    Port     int
    Protocol string
    Name     string
    Product  string
    Version  string
}
```

**Output en JSON**:
```json
{
  "type": "ip",
  "value": "1.2.3.4",
  "metadata": {
    "country": "United States",
    "services": [
      {"port": 22, "protocol": "tcp", "name": "ssh", "product": "OpenSSH", "version": "8.9p1"},
      {"port": 80, "protocol": "tcp", "name": "http", "product": "nginx", "version": "1.24.0"},
      {"port": 3306, "protocol": "tcp", "name": "mysql", "product": "MySQL", "version": "5.7.40"}
    ]
  }
}

// Y services como artifacts separados con más detalle:
{
  "type": "service",
  "value": "MySQL 5.7.40 on tcp/3306",
  "metadata": {
    "parent_ip": "1.2.3.4",
    "port": 3306,
    "product": "MySQL",
    "version": "5.7.40",
    "has_vulns": true,
    "cve_list": ["CVE-2023-21980"]
  }
}
```

---

## 📋 Resumen de Implementación

### Tipos Nuevos: 7
1. ✅ **service** - Servicios en puertos (Nmap output)
2. ✅ **waf** - Web Application Firewalls
3. ✅ **api** - APIs REST/GraphQL/SOAP
4. ✅ **repository** - Repos expuestos (.git, .svn)
5. ✅ **backup_file** - Backups expuestos
6. ✅ **storage_bucket** - S3/Azure/GCP buckets
7. ✅ **webshell** - Webshells detectadas

### Metadata Structures: 7
- ServiceMetadata (25+ campos)
- WAFMetadata (20+ campos)
- APIMetadata (30+ campos)
- RepositoryMetadata (20+ campos)
- BackupFileMetadata (25+ campos)
- StorageBucketMetadata (25+ campos)
- WebshellMetadata (25+ campos)

### Archivos a Crear: 7
- `internal/core/domain/metadata/service.go`
- `internal/core/domain/metadata/waf.go`
- `internal/core/domain/metadata/api.go`
- `internal/core/domain/metadata/repository.go`
- `internal/core/domain/metadata/backup_file.go`
- `internal/core/domain/metadata/storage_bucket.go`
- `internal/core/domain/metadata/webshell.go`

### Archivos a Actualizar: 3
- `internal/core/domain/artifact_types.go` (añadir 7 tipos)
- `internal/core/domain/builders.go` (añadir builders)
- `internal/core/domain/metadata/ip.go` (añadir ServiceSummary)

---

## 🎯 Casos de Uso por Tipo

### **service** → Nmap/Masscan/Naabu
```bash
nmap -sV -sC target.com
→ Genera artifacts de tipo "service" para cada puerto
→ Incluye versiones, vulnerabilidades, NSE scripts
```

### **waf** → WAFw00f/whatwaf
```bash
wafw00f https://target.com
→ Genera artifact tipo "waf"
→ Incluye vendor, protecciones activas, bypass difficulty
```

### **api** → API discovery tools
```bash
# Descubrimiento de API + documentación
→ artifact tipo "api"
→ Incluye schema, autenticación, endpoints
```

### **repository** → GitDumper/dvcs-ripper
```bash
# Detección de .git exposed
→ artifact tipo "repository"
→ Incluye commits, branches, secrets
```

### **backup_file** → Dirsearch/ffuf
```bash
# Fuzzing de backups
→ artifact tipo "backup_file"
→ Incluye tamaño, contenido, riesgo
```

### **storage_bucket** → S3Scanner/CloudBrute
```bash
# Enumeración de buckets
→ artifact tipo "storage_bucket"
→ Incluye permisos, contenido, misconfigurations
```

### **webshell** → WebShell detection
```bash
# Forensics/threat hunting
→ artifact tipo "webshell"
→ Incluye tipo, capabilities, IOCs
```

---

## ✅ Validación del Plan

### Tipos críticos cubiertos:
- ✅ Servicios de red (Nmap output)
- ✅ WAF detection (evasión)
- ✅ APIs modernas (REST/GraphQL)
- ✅ Exposición de código (.git)
- ✅ Exposición de datos (backups)
- ✅ Cloud storage (S3/Azure)
- ✅ Indicators of compromise (webshells)

### Total de tipos después de implementar:
- Actuales: 34 tipos
- Nuevos: 7 tipos
- **Total: 41 tipos de artefactos**

---

## 🚀 Siguiente Paso

¿Aprobar implementación de estos 7 tipos con sus metadata structures?

**Tiempo estimado**: 2-3 horas
**Impacto**: Cobertura completa del dominio de reconocimiento web
**Backward compatible**: 100%
