# 📋 Plan: Metadatos Estructurados por Tipo de Artefacto

## 🎯 Objetivo

Estandarizar y expandir los tipos de artefactos con esquemas de metadata específicos, permitiendo almacenar información rica y estructurada para cada descubrimiento durante el reconocimiento web.

---

## 📊 Investigación: Tipos de Artefactos en Reconocimiento Web

### Categorías Principales

#### 1. **Infraestructura de Red**
- Dominios y subdominios
- Direcciones IP (IPv4/IPv6)
- Rangos de red (CIDR)
- ASN (Autonomous System Numbers)
- Puertos abiertos
- DNS records

#### 2. **Aplicaciones Web**
- URLs y endpoints
- Tecnologías detectadas (frameworks, CMS, servidores)
- Headers HTTP
- Cookies
- APIs descubiertas
- Archivos estáticos (JS, CSS, imágenes)

#### 3. **Certificados y Seguridad**
- Certificados SSL/TLS
- Claves públicas
- Políticas de seguridad (CSP, HSTS, etc.)
- Vulnerabilidades conocidas

#### 4. **Información de Contacto**
- Emails
- Números de teléfono
- Redes sociales
- Información WHOIS

#### 5. **Contenido y Datos**
- Formularios
- Parámetros de URL
- Credenciales expuestas
- Archivos sensibles (.git, .env, backups)
- Metadatos de archivos

#### 6. **Servicios Cloud**
- Buckets S3
- Instancias cloud (AWS, Azure, GCP)
- CDN endpoints
- Servicios SaaS detectados

---

## 🗂️ Tipos de Artefactos Propuestos (Expandidos)

### ✅ Existentes (11 tipos actuales)
1. `domain` - Dominio principal
2. `subdomain` - Subdominios
3. `ip` - IPv4
4. `ipv6` - IPv6
5. `email` - Direcciones de correo
6. `certificate` - Certificados SSL/TLS
7. `url` - URLs completas
8. `port` - Puertos abiertos
9. `technology` - Tecnologías detectadas
10. `cidr` - Rangos de red
11. `asn` - ASN

### 🆕 Nuevos Tipos Propuestos (15+ tipos)

#### **Infraestructura**
12. `dns_record` - Registros DNS (A, AAAA, MX, TXT, CNAME, NS, SOA)
13. `nameserver` - Servidores DNS autoritativos
14. `mx_record` - Mail Exchange records

#### **Web Application**
15. `endpoint` - API endpoints / rutas HTTP
16. `http_header` - Headers HTTP relevantes
17. `cookie` - Cookies detectadas
18. `form` - Formularios HTML
19. `parameter` - Parámetros de URL/POST
20. `javascript` - Archivos JS (para análisis)
21. `redirect` - Redirecciones detectadas

#### **Security**
22. `vulnerability` - Vulnerabilidades identificadas
23. `security_header` - Headers de seguridad (CSP, HSTS, etc.)
24. `tls_config` - Configuración TLS (ciphers, versión)
25. `ssh_key` - Claves SSH públicas

#### **Cloud & Infrastructure**
26. `cloud_resource` - Recursos cloud (S3, Azure Blob, etc.)
27. `cdn_endpoint` - CDN endpoints
28. `container` - Contenedores Docker expuestos

#### **Data & Content**
29. `credential` - Credenciales expuestas (API keys, tokens)
30. `sensitive_file` - Archivos sensibles (.git, .env, backups)
31. `metadata` - Metadatos extraídos (EXIF, document properties)

#### **Social & Contact**
32. `phone` - Números de teléfono
33. `social_media` - Perfiles de redes sociales
34. `whois_contact` - Información de contacto WHOIS

---

## 📐 Esquema de Metadata por Tipo de Artefacto

### Metadata Común (Todos los tipos)

```go
// Campos comunes en todos los artifacts
{
    "discovered_at": "2024-01-15T10:30:00Z",
    "last_seen": "2024-01-15T10:30:00Z",
    "source": "crtsh,subfinder",
    "confidence": "0.95"
}
```

---

### 1️⃣ **Domain / Subdomain**

```go
Metadata: {
    // Resolución DNS
    "resolved_ips": "1.2.3.4,5.6.7.8",
    "dns_records": "A,AAAA,MX,TXT",

    // Registrador
    "registrar": "GoDaddy",
    "registrar_abuse_email": "abuse@godaddy.com",

    // Fechas
    "created_date": "2020-01-15",
    "updated_date": "2024-01-10",
    "expires_date": "2025-01-15",

    // Nameservers
    "nameservers": "ns1.example.com,ns2.example.com",

    // Estado
    "status": "active",
    "dnssec": "true",

    // Organización (WHOIS)
    "org_name": "Example Corp",
    "org_country": "US",

    // HTTP
    "http_status": "200",
    "http_redirect": "https://www.example.com",
    "http_title": "Welcome to Example",

    // Seguridad
    "has_ssl": "true",
    "ssl_issuer": "Let's Encrypt",
    "ssl_valid_from": "2024-01-01",
    "ssl_valid_until": "2024-04-01",

    // CDN/WAF
    "cdn": "Cloudflare",
    "waf": "Cloudflare",

    // Tags automáticos
    "wildcard_cert": "true",
    "subdomain_level": "2"
}
```

---

### 2️⃣ **IP Address (IPv4/IPv6)**

```go
Metadata: {
    // Geolocalización
    "country": "US",
    "country_code": "US",
    "region": "California",
    "city": "San Francisco",
    "latitude": "37.7749",
    "longitude": "-122.4194",
    "timezone": "America/Los_Angeles",

    // Red
    "asn": "13335",
    "as_org": "Cloudflare, Inc.",
    "isp": "Cloudflare",
    "cidr": "1.2.3.0/24",

    // Hosting
    "hosting_provider": "AWS",
    "datacenter": "us-west-1",

    // DNS
    "ptr_record": "ec2-1-2-3-4.compute-1.amazonaws.com",
    "reverse_dns": "server.example.com",

    // Puertos
    "open_ports": "80,443,22",
    "services": "http,https,ssh",

    // Reputación
    "reputation": "clean",
    "blacklisted": "false",
    "blocklist_count": "0",

    // Tipo
    "ip_type": "public", // public, private, reserved
    "ip_version": "4"    // 4 o 6
}
```

---

### 3️⃣ **Technology** ⭐ (Expandido)

```go
Metadata: {
    // Identificación
    "name": "nginx",                    // Nombre canónico
    "display_name": "Nginx",            // Nombre para mostrar
    "category": "web-server",           // web-server, framework, cms, cdn, analytics, etc.
    "subcategory": "reverse-proxy",

    // Versión
    "version": "1.24.0",                // Versión exacta detectada
    "version_detected": "true",         // Si se detectó versión o es inferida
    "version_confidence": "0.95",       // Confianza en la versión
    "latest_version": "1.25.3",         // Última versión conocida
    "version_outdated": "true",         // Si está desactualizada

    // Detalles de versión
    "major_version": "1",
    "minor_version": "24",
    "patch_version": "0",
    "build_number": "",

    // Detección
    "detection_method": "http_header",  // http_header, html_meta, js_library, favicon_hash, etc.
    "detection_pattern": "Server: nginx/1.24.0",
    "detection_location": "/",          // URL donde se detectó

    // Información adicional
    "vendor": "F5 Networks",
    "website": "https://nginx.org",
    "cpe": "cpe:2.3:a:nginx:nginx:1.24.0",  // Common Platform Enumeration
    "license": "BSD-2-Clause",

    // Seguridad
    "has_known_vulns": "true",
    "cve_count": "2",
    "cve_list": "CVE-2023-1234,CVE-2023-5678",
    "risk_level": "medium",             // low, medium, high, critical

    // Metadatos de uso
    "confidence_score": "0.95",
    "popularity_rank": "1245",          // En ranking global
    "first_release": "2004-10-04",

    // Módulos/Plugins detectados
    "modules": "ssl_module,gzip_module",
    "plugins": "",

    // Stack relacionado
    "implies": "linux,openssl",         // Tecnologías que implica
    "excludes": "apache",               // Tecnologías que excluye

    // URLs relevantes
    "icon_url": "https://icon.example.com/nginx.png",
    "documentation": "https://nginx.org/en/docs/",

    // Tags
    "tags": "web-server,reverse-proxy,load-balancer"
}
```

**Categorías de Technology:**
- `web-server` (nginx, apache, IIS, etc.)
- `framework` (React, Django, Laravel, etc.)
- `cms` (WordPress, Drupal, Joomla, etc.)
- `programming-language` (PHP, Python, Ruby, etc.)
- `database` (MySQL, PostgreSQL, MongoDB, etc.)
- `cdn` (Cloudflare, Fastly, Akamai, etc.)
- `analytics` (Google Analytics, Matomo, etc.)
- `javascript-library` (jQuery, Vue.js, Angular, etc.)
- `font-library` (Google Fonts, Font Awesome, etc.)
- `marketing` (HubSpot, Marketo, etc.)
- `payment` (Stripe, PayPal, etc.)
- `security` (reCAPTCHA, Auth0, etc.)
- `tag-manager` (Google Tag Manager, etc.)
- `video-player` (YouTube, Vimeo, etc.)
- `map` (Google Maps, Mapbox, etc.)
- `crm` (Salesforce, HubSpot, etc.)
- `ecommerce` (Shopify, WooCommerce, Magento, etc.)

---

### 4️⃣ **URL / Endpoint**

```go
Metadata: {
    // Parsing de URL
    "scheme": "https",
    "host": "api.example.com",
    "port": "443",
    "path": "/v1/users",
    "query": "page=1&limit=10",
    "fragment": "section-1",

    // HTTP Response
    "status_code": "200",
    "status_text": "OK",
    "content_type": "application/json",
    "content_length": "1234",
    "response_time_ms": "245",

    // Headers relevantes
    "server": "nginx/1.24.0",
    "x_powered_by": "PHP/8.1.0",
    "content_security_policy": "default-src 'self'",

    // Autenticación
    "requires_auth": "true",
    "auth_type": "Bearer",  // Basic, Bearer, API-Key, OAuth, etc.
    "auth_location": "header",

    // API específico
    "api_version": "v1",
    "method": "GET",        // GET, POST, PUT, DELETE, etc.
    "endpoint_type": "rest", // rest, graphql, soap, websocket

    // Contenido
    "title": "User List API",
    "description": "Returns paginated list of users",

    // Seguridad
    "https_only": "true",
    "hsts_enabled": "true",
    "has_cors": "true",
    "cors_origin": "*",

    // Tecnologías
    "technologies": "nginx,php,mysql",

    // Redirects
    "redirect_count": "2",
    "final_url": "https://www.example.com/users",

    // Parámetros
    "parameters": "page,limit,sort",
    "parameter_count": "3"
}
```

---

### 5️⃣ **Port**

```go
Metadata: {
    // Puerto
    "port_number": "443",
    "protocol": "tcp",      // tcp, udp
    "state": "open",        // open, closed, filtered

    // Servicio
    "service": "https",
    "service_product": "nginx",
    "service_version": "1.24.0",
    "service_extra_info": "Ubuntu",

    // Banner
    "banner": "HTTP/1.1 200 OK\\r\\nServer: nginx/1.24.0...",

    // SSL/TLS (si aplica)
    "ssl_enabled": "true",
    "ssl_version": "TLSv1.3",
    "ssl_cipher": "TLS_AES_256_GCM_SHA384",
    "ssl_cert_issuer": "Let's Encrypt",
    "ssl_cert_subject": "example.com",
    "ssl_cert_valid_from": "2024-01-01",
    "ssl_cert_valid_until": "2024-04-01",

    // Vulnerabilidades
    "vulnerable": "false",
    "vulnerability_list": "",

    // Metadatos
    "common_port": "true",
    "port_name": "https",
    "scan_method": "syn"    // syn, connect, udp
}
```

---

### 6️⃣ **Certificate**

```go
Metadata: {
    // Identificación
    "serial_number": "03:AF:...",
    "fingerprint_sha256": "A1:B2:C3:...",
    "fingerprint_sha1": "12:34:56:...",

    // Emisor
    "issuer_cn": "Let's Encrypt Authority X3",
    "issuer_o": "Let's Encrypt",
    "issuer_c": "US",
    "issuer_full": "CN=Let's Encrypt Authority X3, O=Let's Encrypt, C=US",

    // Sujeto
    "subject_cn": "example.com",
    "subject_o": "Example Corp",
    "subject_c": "US",
    "subject_full": "CN=example.com, O=Example Corp, C=US",

    // Validez
    "valid_from": "2024-01-01T00:00:00Z",
    "valid_until": "2024-04-01T23:59:59Z",
    "days_remaining": "45",
    "is_valid": "true",
    "is_expired": "false",
    "is_self_signed": "false",

    // SANs (Subject Alternative Names)
    "san_domains": "example.com,www.example.com,*.example.com",
    "san_count": "3",
    "wildcard_cert": "true",

    // Algoritmos
    "signature_algorithm": "SHA256-RSA",
    "public_key_algorithm": "RSA",
    "key_size": "2048",

    // Extensiones
    "key_usage": "Digital Signature, Key Encipherment",
    "extended_key_usage": "TLS Web Server Authentication",
    "has_sct": "true",  // Certificate Transparency

    // Validación
    "validation_type": "DV",  // DV, OV, EV
    "ct_log_count": "2",

    // Seguridad
    "weak_signature": "false",
    "weak_key": "false",
    "revoked": "false",
    "revocation_reason": ""
}
```

---

### 7️⃣ **Email**

```go
Metadata: {
    // Parsing
    "local_part": "contact",
    "domain": "example.com",
    "domain_mx": "mail.example.com",

    // Validación
    "format_valid": "true",
    "dns_valid": "true",
    "mx_exists": "true",
    "smtp_valid": "false",  // Si se verificó por SMTP

    // Tipo
    "email_type": "generic",  // generic, personal, role-based, disposable
    "role": "contact",        // contact, support, admin, sales, etc.

    // Fuente de descubrimiento
    "found_in": "whois",      // whois, webpage, dns_txt, certificate, etc.
    "context": "registrant",

    // Información adicional
    "has_spf": "true",
    "has_dkim": "true",
    "has_dmarc": "true",

    // Reputación
    "disposable": "false",
    "free_provider": "false",  // Gmail, Yahoo, etc.
    "corporate": "true"
}
```

---

### 8️⃣ **DNS Record** (Nuevo)

```go
Metadata: {
    // Tipo de record
    "record_type": "A",     // A, AAAA, CNAME, MX, TXT, NS, SOA, etc.
    "record_name": "example.com",
    "record_value": "1.2.3.4",
    "ttl": "300",

    // Detalles específicos por tipo
    // Para MX
    "mx_priority": "10",

    // Para TXT
    "txt_type": "spf",      // spf, dkim, dmarc, verification, etc.

    // Para SRV
    "srv_priority": "10",
    "srv_weight": "60",
    "srv_port": "5060",
    "srv_target": "sipserver.example.com",

    // Nameserver
    "nameserver": "ns1.example.com",
    "authoritative": "true",

    // DNSSEC
    "dnssec_signed": "true",
    "dnssec_valid": "true"
}
```

---

### 9️⃣ **Vulnerability** (Nuevo)

```go
Metadata: {
    // Identificación
    "vuln_id": "CVE-2023-1234",
    "cwe_id": "CWE-79",
    "osvdb_id": "",

    // Descripción
    "title": "Cross-Site Scripting in WordPress",
    "description": "An XSS vulnerability exists...",
    "severity": "high",     // low, medium, high, critical
    "cvss_score": "7.5",
    "cvss_vector": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:N/A:N",

    // Afectación
    "affected_component": "WordPress",
    "affected_version": "6.3.1",
    "fixed_version": "6.3.2",

    // Estado
    "exploited": "true",
    "exploit_available": "true",
    "exploit_maturity": "functional",  // unproven, poc, functional, high
    "patch_available": "true",

    // Referencias
    "references": "https://nvd.nist.gov/vuln/detail/CVE-2023-1234",
    "advisories": "https://wordpress.org/news/...",

    // Detección
    "detection_method": "version_check",  // version_check, signature, exploit
    "confidence": "0.95",

    // Remediación
    "solution": "Update to version 6.3.2 or later",
    "workaround": "Disable affected plugin",

    // Fechas
    "published_date": "2023-11-15",
    "disclosed_date": "2023-11-10",
    "last_modified": "2023-11-20"
}
```

---

### 🔟 **Cloud Resource** (Nuevo)

```go
Metadata: {
    // Proveedor
    "provider": "aws",      // aws, azure, gcp, digitalocean, etc.
    "service": "s3",        // s3, lambda, ec2, rds, etc.
    "region": "us-east-1",

    // Identificación
    "resource_id": "my-bucket-name",
    "resource_arn": "arn:aws:s3:::my-bucket-name",
    "resource_url": "https://my-bucket-name.s3.amazonaws.com",

    // Acceso
    "public_access": "true",
    "permissions": "read",  // read, write, list, delete
    "requires_auth": "false",

    // Contenido (para buckets)
    "file_count": "1234",
    "total_size_mb": "5678",
    "file_types": "jpg,png,pdf",

    // Configuración
    "versioning_enabled": "false",
    "encryption_enabled": "false",
    "logging_enabled": "false",

    // Seguridad
    "misconfigured": "true",
    "risk_level": "high",
    "exposed_data": "true"
}
```

---

## 🏗️ Implementación Técnica

### Opción 1: Map Genérico (Actual - Flexible pero sin tipo)

```go
type Artifact struct {
    Type     ArtifactType
    Value    string
    Metadata map[string]string  // ✅ Flexible ❌ Sin validación
}
```

**Pros:**
- ✅ Flexible, fácil de extender
- ✅ JSON serializable
- ✅ Sin cambios en la estructura

**Contras:**
- ❌ Sin type safety
- ❌ Sin validación
- ❌ Difícil autocompletado en IDEs

---

### Opción 2: Structs Tipados (Recomendado)

```go
type Artifact struct {
    Type     ArtifactType
    Value    string
    Metadata ArtifactMetadata  // Interface
}

type ArtifactMetadata interface {
    IsValid() bool
    ToMap() map[string]string
}

// Implementaciones específicas
type DomainMetadata struct {
    ResolvedIPs     []string
    Registrar       string
    CreatedDate     *time.Time
    ExpiresDate     *time.Time
    Nameservers     []string
    HasSSL          bool
    SSLIssuer       string
    CDN             string
    HTTPStatus      int
    HTTPTitle       string
}

type TechnologyMetadata struct {
    Name            string
    DisplayName     string
    Category        string
    Version         string
    VersionDetected bool
    LatestVersion   string
    Outdated        bool
    DetectionMethod string
    Vendor          string
    CVEList         []string
    RiskLevel       string
    Implies         []string
}

type IPMetadata struct {
    Country         string
    City            string
    ASN             string
    ISP             string
    HostingProvider string
    OpenPorts       []int
    Reputation      string
    Blacklisted     bool
}
```

**Pros:**
- ✅ Type safety completo
- ✅ Validación en compile-time
- ✅ Autocompletado en IDEs
- ✅ Documentación clara

**Contras:**
- ❌ Más código
- ❌ Menos flexible
- ❌ Requiere cambios en serialización

---

### Opción 3: Híbrida (Balance - RECOMENDADA) ⭐

```go
type Artifact struct {
    Type     ArtifactType
    Value    string

    // Metadata tipado (opcional)
    TypedMetadata ArtifactMetadata

    // Metadata genérico (backward compatible)
    Metadata map[string]string
}

// Al crear artifact:
artifact := &Artifact{
    Type:  ArtifactTypeTechnology,
    Value: "nginx",
    TypedMetadata: &TechnologyMetadata{
        Name:    "nginx",
        Version: "1.24.0",
        Category: "web-server",
    },
    Metadata: map[string]string{
        "name":     "nginx",
        "version":  "1.24.0",
        "category": "web-server",
    },
}
```

**Pros:**
- ✅ Type safety donde se necesita
- ✅ Backward compatible
- ✅ Flexible para nuevos campos
- ✅ Fácil serialización

---

## 📝 Plan de Implementación

### Fase 1: Expandir Tipos de Artefactos (1 día)

```bash
✓ Añadir nuevos ArtifactType al enum
✓ Documentar cada tipo en comentarios
✓ Actualizar validaciones
```

**Archivos:**
- `internal/core/domain/artifact.go` (añadir tipos al enum)

---

### Fase 2: Crear Metadata Structs (2-3 días)

```bash
✓ Crear internal/core/domain/metadata/
  ├── base.go           (Interface ArtifactMetadata)
  ├── domain.go         (DomainMetadata)
  ├── technology.go     (TechnologyMetadata) ⭐
  ├── ip.go             (IPMetadata)
  ├── url.go            (URLMetadata)
  ├── certificate.go    (CertificateMetadata)
  ├── port.go           (PortMetadata)
  ├── email.go          (EmailMetadata)
  ├── dns.go            (DNSRecordMetadata)
  ├── vulnerability.go  (VulnerabilityMetadata)
  └── cloud.go          (CloudResourceMetadata)
```

**Cada struct debe:**
- Implementar `ArtifactMetadata` interface
- Tener método `ToMap() map[string]string`
- Tener método `IsValid() bool`
- Tener método `FromMap(map[string]string) error`
- Tener tags JSON para serialización

---

### Fase 3: Builders y Helpers (1 día)

```bash
✓ Crear builders para facilitar creación
✓ Crear helpers de validación
✓ Crear helpers de conversión
```

**Ejemplo:**
```go
// Builder para Technology
func NewTechnologyArtifact(name, version string) *Artifact {
    meta := &TechnologyMetadata{
        Name:    name,
        Version: version,
    }

    artifact := &Artifact{
        Type:          ArtifactTypeTechnology,
        Value:         fmt.Sprintf("%s@%s", name, version),
        TypedMetadata: meta,
        Metadata:      meta.ToMap(),
    }

    return artifact
}
```

---

### Fase 4: Actualizar Fuentes Existentes (1 día)

```bash
✓ Actualizar crtsh para usar nuevos metadata
✓ Añadir más información en metadata
```

---

### Fase 5: Testing (1 día)

```bash
✓ Tests unitarios para cada metadata struct
✓ Tests de serialización JSON
✓ Tests de conversión ToMap/FromMap
✓ Tests de validación
```

---

### Fase 6: Documentación (0.5 día)

```bash
✓ Actualizar ARCHITECTURE.md
✓ Crear ejemplos de uso
✓ Documentar campos de cada metadata
```

---

## 🎯 Prioridades de Implementación

### Sprint 1 (Alta Prioridad - 3 días)
1. ⭐ **Technology** (con versiones y CVEs)
2. **Domain/Subdomain** (enriquecer con WHOIS, DNS, SSL)
3. **IP** (geolocalización, ASN, puertos)
4. **URL/Endpoint** (HTTP details, API info)

### Sprint 2 (Media Prioridad - 2 días)
5. **Certificate** (detalles SSL/TLS completos)
6. **Port** (servicios, banners, vulnerabilidades)
7. **DNS Record** (todos los tipos de records)
8. **Email** (validación, SPF/DKIM/DMARC)

### Sprint 3 (Baja Prioridad - 2 días)
9. **Vulnerability** (CVEs, CVSS, exploits)
10. **Cloud Resource** (S3, Azure, GCP)
11. Resto de tipos según necesidad

---

## 📊 Ejemplo de Uso Final

```go
// Crear artifact de technology con metadata rica
tech := &Artifact{
    Type:  ArtifactTypeTechnology,
    Value: "nginx@1.24.0",
    TypedMetadata: &TechnologyMetadata{
        Name:            "nginx",
        DisplayName:     "Nginx",
        Category:        "web-server",
        Version:         "1.24.0",
        VersionDetected: true,
        LatestVersion:   "1.25.3",
        Outdated:        true,
        DetectionMethod: "http_header",
        Vendor:          "F5 Networks",
        CVEList:         []string{"CVE-2023-1234", "CVE-2023-5678"},
        RiskLevel:       "medium",
        Implies:         []string{"linux", "openssl"},
    },
    Confidence: 0.95,
    Sources:    []string{"httpx"},
}

// Serializar a JSON
json, _ := json.Marshal(tech)

// Output JSON:
{
  "type": "technology",
  "value": "nginx@1.24.0",
  "metadata": {
    "name": "nginx",
    "display_name": "Nginx",
    "category": "web-server",
    "version": "1.24.0",
    "version_detected": "true",
    "latest_version": "1.25.3",
    "version_outdated": "true",
    "detection_method": "http_header",
    "vendor": "F5 Networks",
    "cve_list": "CVE-2023-1234,CVE-2023-5678",
    "risk_level": "medium",
    "implies": "linux,openssl"
  },
  "confidence": 0.95,
  "sources": ["httpx"]
}
```

---

## 🔍 Fuentes de Datos para Metadata

### Technology Detection:
- [Wappalyzer](https://www.wappalyzer.com/) - Base de datos de tecnologías web
- [WhatWeb signatures](https://github.com/urbanadventurer/WhatWeb)
- [Nuclei templates](https://github.com/projectdiscovery/nuclei-templates)
- [httpx](https://github.com/projectdiscovery/httpx) - Headers y tech detection

### CVE/Vulnerability Data:
- [NVD](https://nvd.nist.gov/) - National Vulnerability Database
- [CVE.org](https://www.cve.org/)
- [VulnDB](https://vulndb.cyberriskanalytics.com/)
- [Snyk Vulnerability DB](https://security.snyk.io/)

### IP Intelligence:
- [MaxMind GeoIP2](https://www.maxmind.com/)
- [IPinfo](https://ipinfo.io/)
- [AbuseIPDB](https://www.abuseipdb.com/)

### WHOIS/Domain:
- [WHOIS API](https://www.whoisxmlapi.com/)
- Standard WHOIS protocol

---

## ✅ Checklist de Implementación

### Definición
- [ ] Expandir enum de ArtifactType (11 → 30+ tipos)
- [ ] Crear interfaz ArtifactMetadata
- [ ] Documentar campos de cada metadata

### Implementación
- [ ] Crear package `internal/core/domain/metadata/`
- [ ] Implementar structs para cada tipo
- [ ] Crear builders y helpers
- [ ] Implementar ToMap/FromMap

### Integración
- [ ] Actualizar Artifact para soportar TypedMetadata
- [ ] Mantener backward compatibility con map[string]string
- [ ] Actualizar serialización JSON
- [ ] Actualizar DedupeService para merge de metadata

### Testing
- [ ] Tests unitarios por cada metadata struct
- [ ] Tests de serialización
- [ ] Tests de merge
- [ ] Benchmarks de performance

### Documentación
- [ ] Actualizar ARCHITECTURE.md
- [ ] Crear guía de metadata por tipo
- [ ] Ejemplos de uso
- [ ] Actualizar README

---

## 🎓 Recomendaciones

1. **Empezar por Technology** ⭐
   - Es el tipo más solicitado
   - Alto valor para pentesters
   - Datos ricos (versiones, CVEs)

2. **Approach incremental**
   - Implementar 3-4 tipos priority
   - Validar con fuentes reales
   - Iterar basado en feedback

3. **Mantener flexibilidad**
   - Usar approach híbrido
   - No forzar todos los campos
   - Permitir metadata custom

4. **Performance**
   - Lazy loading de metadata pesado
   - Cache de lookups externos (GeoIP, CVE)
   - Batch processing donde sea posible

---

## 🚀 Siguiente Paso

¿Quieres que implemente la **Fase 1 + 2 (Technology metadata completo)** como prototipo funcional?

Esto incluiría:
- Expandir enum de tipos
- Crear `TechnologyMetadata` struct completo
- Builders y helpers
- Ejemplo de uso en una fuente

Tiempo estimado: **2-3 horas**
