// internal/platform/validator/validator.go
package validator

import (
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Domain validators

// IsDomain verifica si un string es un dominio válido.
// Soporta dominios internacionales (IDN) y punycode.
func IsDomain(domain string) bool {
	if len(domain) == 0 || len(domain) > 253 {
		return false
	}

	// Regex para validar dominios
	// Permite dominios internacionales (IDN) y punycode
	domainRegex := regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?$`)
	if !domainRegex.MatchString(domain) {
		return false
	}

	// Verificar que no sea una IP
	if net.ParseIP(domain) != nil {
		return false
	}

	return true
}

// IsSubdomain verifica si subdomain es un subdominio válido de baseDomain.
func IsSubdomain(subdomain, baseDomain string) bool {
	subdomain = strings.ToLower(strings.TrimSpace(subdomain))
	baseDomain = strings.ToLower(strings.TrimSpace(baseDomain))

	if subdomain == baseDomain {
		return false
	}

	return strings.HasSuffix(subdomain, "."+baseDomain)
}

// NormalizeDomain normaliza un dominio a su forma canónica.
func NormalizeDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	domain = strings.TrimSuffix(domain, ".")
	domain = strings.TrimPrefix(domain, "www.")
	return domain
}

// Email validators

// IsEmail valida formato de email (RFC 5322 simplificado).
func IsEmail(email string) bool {
	if len(email) == 0 || len(email) > 254 {
		return false
	}

	// Regex simplificada para email
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// NormalizeEmail normaliza un email a su forma canónica.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Network validators

// IsIP verifica si un string es una dirección IP válida (v4 o v6).
func IsIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// IsIPv4 verifica si un string es una dirección IPv4 válida.
func IsIPv4(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return parsed.To4() != nil
}

// IsIPv6 verifica si un string es una dirección IPv6 válida.
func IsIPv6(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return parsed.To4() == nil
}

// IsPort valida que un puerto esté en el rango válido [1-65535].
func IsPort(portStr string) bool {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return false
	}
	return port >= 1 && port <= 65535
}

// NormalizeIP normaliza una IP a su forma canónica.
// Si la IP es inválida, retorna string vacío.
func NormalizeIP(ip string) string {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return "" // Invalid IP
	}
	return parsed.String()
}

// URL validators

// IsURL verifica si un string es una URL válida.
func IsURL(urlStr string) bool {
	if len(urlStr) == 0 {
		return false
	}

	parsed, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Debe tener scheme y host
	return parsed.Scheme != "" && parsed.Host != ""
}

// NormalizeURL normaliza una URL a su forma canónica.
func NormalizeURL(urlStr string) string {
	urlStr = strings.TrimSpace(urlStr)

	parsed, err := url.Parse(urlStr)
	if err != nil {
		return strings.ToLower(urlStr)
	}

	// Normalizar scheme (case-insensitive)
	parsed.Scheme = strings.ToLower(parsed.Scheme)

	// Normalizar host (case-insensitive)
	parsed.Host = strings.ToLower(parsed.Host)

	// Remover puertos por defecto
	if parsed.Scheme == "http" && strings.HasSuffix(parsed.Host, ":80") {
		parsed.Host = strings.TrimSuffix(parsed.Host, ":80")
	}
	if parsed.Scheme == "https" && strings.HasSuffix(parsed.Host, ":443") {
		parsed.Host = strings.TrimSuffix(parsed.Host, ":443")
	}

	// Remover trailing slash si no hay path adicional
	if parsed.Path == "/" && parsed.RawQuery == "" && parsed.Fragment == "" {
		parsed.Path = ""
	}

	// Note: Path, query, and fragment are case-sensitive and NOT normalized
	return parsed.String()
}

// Certificate validators

// IsCertSerial valida que un serial de certificado sea un string hexadecimal válido.
// Permite colons y espacios como separadores (formato común en certificados).
func IsCertSerial(serial string) bool {
	if len(serial) == 0 {
		return false
	}

	// Debe ser hexadecimal (0-9, a-f, A-F) con colons/espacios opcionales
	hexRegex := regexp.MustCompile(`^[0-9a-fA-F: ]+$`)
	return hexRegex.MatchString(serial)
}

// NormalizeCertSerial normaliza un serial de certificado.
func NormalizeCertSerial(serial string) string {
	return strings.ToLower(strings.TrimSpace(serial))
}

// Hash validators

// IsHash verifica si un string es un hash válido (MD5, SHA1, SHA256, etc.).
func IsHash(hash string) bool {
	hash = strings.TrimSpace(hash)
	length := len(hash)

	// MD5: 32 chars, SHA1: 40 chars, SHA256: 64 chars, SHA512: 128 chars
	validLengths := map[int]bool{32: true, 40: true, 64: true, 128: true}
	if !validLengths[length] {
		return false
	}

	hexRegex := regexp.MustCompile(`^[0-9a-fA-F]+$`)
	return hexRegex.MatchString(hash)
}

// NormalizeHash normaliza un hash a su forma canónica.
func NormalizeHash(hash string) string {
	return strings.ToLower(strings.TrimSpace(hash))
}

// Generic validators

// IsEmpty verifica si un string está vacío o solo contiene espacios.
func IsEmpty(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}

// IsAlphanumeric verifica si un string contiene solo caracteres alfanuméricos.
func IsAlphanumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	alphanumericRegex := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	return alphanumericRegex.MatchString(s)
}

// MaxLength verifica que un string no exceda una longitud máxima.
func MaxLength(s string, max int) bool {
	return len(s) <= max
}

// MinLength verifica que un string tenga al menos una longitud mínima.
func MinLength(s string, min int) bool {
	return len(s) >= min
}
