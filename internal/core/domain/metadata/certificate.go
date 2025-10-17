// internal/core/domain/metadata/certificate.go
package metadata

// CertificateMetadata contiene información detallada sobre un certificado SSL/TLS.
type CertificateMetadata struct {
	// Identificación
	SerialNumber       string
	FingerprintSHA256  string
	FingerprintSHA1    string

	// Emisor
	IssuerCN   string // Common Name
	IssuerO    string // Organization
	IssuerC    string // Country
	IssuerFull string // DN completo

	// Sujeto
	SubjectCN   string
	SubjectO    string
	SubjectC    string
	SubjectFull string

	// Validez
	ValidFrom     string // ISO 8601
	ValidUntil    string
	DaysRemaining int
	CertValid     bool
	CertExpired   bool
	IsSelfSigned  bool

	// SANs (Subject Alternative Names)
	SANDomains   []string
	SANCount     int
	WildcardCert bool

	// Algoritmos
	SignatureAlgorithm  string
	PublicKeyAlgorithm  string
	KeySize             int

	// Extensiones
	KeyUsage         string
	ExtendedKeyUsage string
	HasSCT           bool // Certificate Transparency

	// Validación
	ValidationType string // DV, OV, EV
	CTLogCount     int

	// Seguridad
	WeakSignature bool
	WeakKey       bool
	Revoked       bool
	RevocationReason string
}

func (c *CertificateMetadata) ToMap() map[string]string {
	m := make(map[string]string)
	SetIfNotEmpty(m, "serial_number", c.SerialNumber)
	SetIfNotEmpty(m, "fingerprint_sha256", c.FingerprintSHA256)
	SetIfNotEmpty(m, "fingerprint_sha1", c.FingerprintSHA1)
	SetIfNotEmpty(m, "issuer_cn", c.IssuerCN)
	SetIfNotEmpty(m, "issuer_o", c.IssuerO)
	SetIfNotEmpty(m, "issuer_c", c.IssuerC)
	SetIfNotEmpty(m, "subject_cn", c.SubjectCN)
	SetIfNotEmpty(m, "subject_o", c.SubjectO)
	SetIfNotEmpty(m, "subject_c", c.SubjectC)
	SetIfNotEmpty(m, "valid_from", c.ValidFrom)
	SetIfNotEmpty(m, "valid_until", c.ValidUntil)
	if c.DaysRemaining > 0 {
		SetInt(m, "days_remaining", c.DaysRemaining)
	}
	SetBool(m, "cert_valid", c.CertValid)
	SetBool(m, "cert_expired", c.CertExpired)
	SetBool(m, "is_self_signed", c.IsSelfSigned)
	if len(c.SANDomains) > 0 {
		m["san_domains"] = StringSliceToCSV(c.SANDomains)
		SetInt(m, "san_count", len(c.SANDomains))
	}
	SetBool(m, "wildcard_cert", c.WildcardCert)
	SetIfNotEmpty(m, "signature_algorithm", c.SignatureAlgorithm)
	SetIfNotEmpty(m, "public_key_algorithm", c.PublicKeyAlgorithm)
	if c.KeySize > 0 {
		SetInt(m, "key_size", c.KeySize)
	}
	SetBool(m, "weak_signature", c.WeakSignature)
	SetBool(m, "weak_key", c.WeakKey)
	SetBool(m, "revoked", c.Revoked)
	return m
}

func (c *CertificateMetadata) FromMap(m map[string]string) error {
	c.SerialNumber = GetString(m, "serial_number", "")
	c.FingerprintSHA256 = GetString(m, "fingerprint_sha256", "")
	c.FingerprintSHA1 = GetString(m, "fingerprint_sha1", "")
	c.IssuerCN = GetString(m, "issuer_cn", "")
	c.SubjectCN = GetString(m, "subject_cn", "")
	c.ValidFrom = GetString(m, "valid_from", "")
	c.ValidUntil = GetString(m, "valid_until", "")
	c.DaysRemaining = GetInt(m, "days_remaining", 0)
	c.CertValid = GetBool(m, "cert_valid", false)
	c.CertExpired = GetBool(m, "cert_expired", false)
	c.IsSelfSigned = GetBool(m, "is_self_signed", false)
	c.SANDomains = CSVToStringSlice(GetString(m, "san_domains", ""))
	c.WildcardCert = GetBool(m, "wildcard_cert", false)
	return nil
}

func (c *CertificateMetadata) IsValid() bool { return c.SerialNumber != "" }
func (c *CertificateMetadata) Type() string  { return "certificate" }
