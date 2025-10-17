// internal/core/domain/artifact_types.go
package domain

// ArtifactType representa los diferentes tipos de artefactos que pueden ser descubiertos.
type ArtifactType string

// Tipos de artefactos - Infraestructura de Red
const (
	// ArtifactTypeDomain representa un dominio principal
	ArtifactTypeDomain ArtifactType = "domain"

	// ArtifactTypeSubdomain representa un subdominio
	ArtifactTypeSubdomain ArtifactType = "subdomain"

	// ArtifactTypeIP representa una dirección IPv4
	ArtifactTypeIP ArtifactType = "ip"

	// ArtifactTypeIPv6 representa una dirección IPv6
	ArtifactTypeIPv6 ArtifactType = "ipv6"

	// ArtifactTypeCIDR representa un rango de red
	ArtifactTypeCIDR ArtifactType = "cidr"

	// ArtifactTypeASN representa un Autonomous System Number
	ArtifactTypeASN ArtifactType = "asn"

	// ArtifactTypePort representa un puerto abierto
	ArtifactTypePort ArtifactType = "port"

	// ArtifactTypeDNSRecord representa un registro DNS
	ArtifactTypeDNSRecord ArtifactType = "dns_record"

	// ArtifactTypeNameserver representa un servidor DNS autoritativo
	ArtifactTypeNameserver ArtifactType = "nameserver"

	// ArtifactTypeMXRecord representa un Mail Exchange record
	ArtifactTypeMXRecord ArtifactType = "mx_record"
)

// Tipos de artefactos - Aplicaciones Web
const (
	// ArtifactTypeURL representa una URL completa
	ArtifactTypeURL ArtifactType = "url"

	// ArtifactTypeEndpoint representa un API endpoint o ruta HTTP
	ArtifactTypeEndpoint ArtifactType = "endpoint"

	// ArtifactTypeTechnology representa una tecnología detectada
	ArtifactTypeTechnology ArtifactType = "technology"

	// ArtifactTypeHTTPHeader representa un header HTTP relevante
	ArtifactTypeHTTPHeader ArtifactType = "http_header"

	// ArtifactTypeCookie representa una cookie detectada
	ArtifactTypeCookie ArtifactType = "cookie"

	// ArtifactTypeForm representa un formulario HTML
	ArtifactTypeForm ArtifactType = "form"

	// ArtifactTypeParameter representa un parámetro de URL/POST
	ArtifactTypeParameter ArtifactType = "parameter"

	// ArtifactTypeJavaScript representa un archivo JavaScript
	ArtifactTypeJavaScript ArtifactType = "javascript"

	// ArtifactTypeRedirect representa una redirección detectada
	ArtifactTypeRedirect ArtifactType = "redirect"
)

// Tipos de artefactos - Certificados y Seguridad
const (
	// ArtifactTypeCertificate representa un certificado SSL/TLS
	ArtifactTypeCertificate ArtifactType = "certificate"

	// ArtifactTypeVulnerability representa una vulnerabilidad identificada
	ArtifactTypeVulnerability ArtifactType = "vulnerability"

	// ArtifactTypeSecurityHeader representa un header de seguridad
	ArtifactTypeSecurityHeader ArtifactType = "security_header"

	// ArtifactTypeTLSConfig representa configuración TLS
	ArtifactTypeTLSConfig ArtifactType = "tls_config"

	// ArtifactTypeSSHKey representa una clave SSH pública
	ArtifactTypeSSHKey ArtifactType = "ssh_key"
)

// Tipos de artefactos - Cloud
const (
	// ArtifactTypeCloudResource representa un recurso cloud
	ArtifactTypeCloudResource ArtifactType = "cloud_resource"

	// ArtifactTypeCDNEndpoint representa un CDN endpoint
	ArtifactTypeCDNEndpoint ArtifactType = "cdn_endpoint"

	// ArtifactTypeContainer representa un contenedor Docker expuesto
	ArtifactTypeContainer ArtifactType = "container"
)

// Tipos de artefactos - Datos y Contenido
const (
	// ArtifactTypeCredential representa credenciales expuestas
	ArtifactTypeCredential ArtifactType = "credential"

	// ArtifactTypeSensitiveFile representa archivos sensibles
	ArtifactTypeSensitiveFile ArtifactType = "sensitive_file"

	// ArtifactTypeMetadata representa metadatos extraídos
	ArtifactTypeMetadata ArtifactType = "metadata"
)

// Tipos de artefactos - Información de Contacto
const (
	// ArtifactTypeEmail representa una dirección de correo electrónico
	ArtifactTypeEmail ArtifactType = "email"

	// ArtifactTypePhone representa un número de teléfono
	ArtifactTypePhone ArtifactType = "phone"

	// ArtifactTypeSocialMedia representa un perfil de red social
	ArtifactTypeSocialMedia ArtifactType = "social_media"

	// ArtifactTypeWhoisContact representa información de contacto WHOIS
	ArtifactTypeWhoisContact ArtifactType = "whois_contact"
)

// IsValid verifica si el tipo de artefacto es válido.
func (t ArtifactType) IsValid() bool {
	switch t {
	case ArtifactTypeDomain, ArtifactTypeSubdomain, ArtifactTypeIP, ArtifactTypeIPv6,
		ArtifactTypeCIDR, ArtifactTypeASN, ArtifactTypePort, ArtifactTypeDNSRecord,
		ArtifactTypeNameserver, ArtifactTypeMXRecord, ArtifactTypeURL, ArtifactTypeEndpoint,
		ArtifactTypeTechnology, ArtifactTypeHTTPHeader, ArtifactTypeCookie, ArtifactTypeForm,
		ArtifactTypeParameter, ArtifactTypeJavaScript, ArtifactTypeRedirect, ArtifactTypeCertificate,
		ArtifactTypeVulnerability, ArtifactTypeSecurityHeader, ArtifactTypeTLSConfig, ArtifactTypeSSHKey,
		ArtifactTypeCloudResource, ArtifactTypeCDNEndpoint, ArtifactTypeContainer, ArtifactTypeCredential,
		ArtifactTypeSensitiveFile, ArtifactTypeMetadata, ArtifactTypeEmail, ArtifactTypePhone,
		ArtifactTypeSocialMedia, ArtifactTypeWhoisContact:
		return true
	default:
		return false
	}
}

// Category retorna la categoría a la que pertenece el tipo de artefacto.
func (t ArtifactType) Category() string {
	switch t {
	case ArtifactTypeDomain, ArtifactTypeSubdomain, ArtifactTypeIP, ArtifactTypeIPv6,
		ArtifactTypeCIDR, ArtifactTypeASN, ArtifactTypePort, ArtifactTypeDNSRecord,
		ArtifactTypeNameserver, ArtifactTypeMXRecord:
		return "infrastructure"

	case ArtifactTypeURL, ArtifactTypeEndpoint, ArtifactTypeTechnology, ArtifactTypeHTTPHeader,
		ArtifactTypeCookie, ArtifactTypeForm, ArtifactTypeParameter, ArtifactTypeJavaScript,
		ArtifactTypeRedirect:
		return "web"

	case ArtifactTypeCertificate, ArtifactTypeVulnerability, ArtifactTypeSecurityHeader,
		ArtifactTypeTLSConfig, ArtifactTypeSSHKey:
		return "security"

	case ArtifactTypeCloudResource, ArtifactTypeCDNEndpoint, ArtifactTypeContainer:
		return "cloud"

	case ArtifactTypeCredential, ArtifactTypeSensitiveFile, ArtifactTypeMetadata:
		return "data"

	case ArtifactTypeEmail, ArtifactTypePhone, ArtifactTypeSocialMedia, ArtifactTypeWhoisContact:
		return "contact"

	default:
		return "unknown"
	}
}

// String retorna la representación string del tipo.
func (t ArtifactType) String() string {
	return string(t)
}
