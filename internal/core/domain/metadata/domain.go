// internal/core/domain/metadata/domain.go
package metadata

// DomainMetadata contiene información detallada sobre un dominio o subdominio.
type DomainMetadata struct {
	// Resolución DNS
	ResolvedIPs []string // IPs a las que resuelve
	DNSRecords  []string // Tipos de records (A, AAAA, MX, TXT, etc.)

	// Registrador (WHOIS)
	Registrar          string
	RegistrarAbuseEmail string
	RegistrarURL       string

	// Fechas
	CreatedDate string // YYYY-MM-DD
	UpdatedDate string
	ExpiresDate string

	// Nameservers
	Nameservers []string

	// Estado
	Status   string // active, inactive, pending, etc.
	DNSSEC   bool   // Si tiene DNSSEC habilitado

	// Estado de actividad (probing)
	IsAlive     bool   // Si el dominio responde a HTTP/HTTPS
	ProbeStatus string // "alive", "dead", "unknown"
	LastProbed  string // Timestamp del último probe (ISO 8601)
	ProbeSource string // Source que hizo el probe ("httpx", "nuclei", etc.)

	// Organización (WHOIS)
	OrgName    string
	OrgCountry string
	OrgEmail   string

	// HTTP
	HTTPStatus   int
	HTTPRedirect string
	HTTPTitle    string
	HTTPServer   string

	// SSL/TLS
	HasSSL        bool
	SSLIssuer     string
	SSLValidFrom  string
	SSLValidUntil string
	SSLWildcard   bool

	// CDN/WAF
	CDN string // Cloudflare, Fastly, Akamai, etc.
	WAF string // Cloudflare, AWS WAF, etc.

	// Tags automáticos
	SubdomainLevel int // Nivel de subdominio (www.example.com = 1)
}

// ToMap convierte DomainMetadata a map[string]string.
func (d *DomainMetadata) ToMap() map[string]string {
	m := make(map[string]string)

	// DNS
	if len(d.ResolvedIPs) > 0 {
		m["resolved_ips"] = StringSliceToCSV(d.ResolvedIPs)
	}
	if len(d.DNSRecords) > 0 {
		m["dns_records"] = StringSliceToCSV(d.DNSRecords)
	}

	// Registrador
	SetIfNotEmpty(m, "registrar", d.Registrar)
	SetIfNotEmpty(m, "registrar_abuse_email", d.RegistrarAbuseEmail)
	SetIfNotEmpty(m, "registrar_url", d.RegistrarURL)

	// Fechas
	SetIfNotEmpty(m, "created_date", d.CreatedDate)
	SetIfNotEmpty(m, "updated_date", d.UpdatedDate)
	SetIfNotEmpty(m, "expires_date", d.ExpiresDate)

	// Nameservers
	if len(d.Nameservers) > 0 {
		m["nameservers"] = StringSliceToCSV(d.Nameservers)
	}

	// Estado
	SetIfNotEmpty(m, "status", d.Status)
	SetBool(m, "dnssec", d.DNSSEC)

	// Estado de actividad
	SetBool(m, "is_alive", d.IsAlive)
	SetIfNotEmpty(m, "probe_status", d.ProbeStatus)
	SetIfNotEmpty(m, "last_probed", d.LastProbed)
	SetIfNotEmpty(m, "probe_source", d.ProbeSource)

	// Organización
	SetIfNotEmpty(m, "org_name", d.OrgName)
	SetIfNotEmpty(m, "org_country", d.OrgCountry)
	SetIfNotEmpty(m, "org_email", d.OrgEmail)

	// HTTP
	if d.HTTPStatus > 0 {
		SetInt(m, "http_status", d.HTTPStatus)
	}
	SetIfNotEmpty(m, "http_redirect", d.HTTPRedirect)
	SetIfNotEmpty(m, "http_title", d.HTTPTitle)
	SetIfNotEmpty(m, "http_server", d.HTTPServer)

	// SSL
	SetBool(m, "has_ssl", d.HasSSL)
	SetIfNotEmpty(m, "ssl_issuer", d.SSLIssuer)
	SetIfNotEmpty(m, "ssl_valid_from", d.SSLValidFrom)
	SetIfNotEmpty(m, "ssl_valid_until", d.SSLValidUntil)
	SetBool(m, "ssl_wildcard", d.SSLWildcard)

	// CDN/WAF
	SetIfNotEmpty(m, "cdn", d.CDN)
	SetIfNotEmpty(m, "waf", d.WAF)

	// Tags
	if d.SubdomainLevel > 0 {
		SetInt(m, "subdomain_level", d.SubdomainLevel)
	}

	return m
}

// FromMap carga DomainMetadata desde map[string]string.
func (d *DomainMetadata) FromMap(m map[string]string) error {
	// DNS
	d.ResolvedIPs = CSVToStringSlice(GetString(m, "resolved_ips", ""))
	d.DNSRecords = CSVToStringSlice(GetString(m, "dns_records", ""))

	// Registrador
	d.Registrar = GetString(m, "registrar", "")
	d.RegistrarAbuseEmail = GetString(m, "registrar_abuse_email", "")
	d.RegistrarURL = GetString(m, "registrar_url", "")

	// Fechas
	d.CreatedDate = GetString(m, "created_date", "")
	d.UpdatedDate = GetString(m, "updated_date", "")
	d.ExpiresDate = GetString(m, "expires_date", "")

	// Nameservers
	d.Nameservers = CSVToStringSlice(GetString(m, "nameservers", ""))

	// Estado
	d.Status = GetString(m, "status", "")
	d.DNSSEC = GetBool(m, "dnssec", false)

	// Estado de actividad
	d.IsAlive = GetBool(m, "is_alive", false)
	d.ProbeStatus = GetString(m, "probe_status", "")
	d.LastProbed = GetString(m, "last_probed", "")
	d.ProbeSource = GetString(m, "probe_source", "")

	// Organización
	d.OrgName = GetString(m, "org_name", "")
	d.OrgCountry = GetString(m, "org_country", "")
	d.OrgEmail = GetString(m, "org_email", "")

	// HTTP
	d.HTTPStatus = GetInt(m, "http_status", 0)
	d.HTTPRedirect = GetString(m, "http_redirect", "")
	d.HTTPTitle = GetString(m, "http_title", "")
	d.HTTPServer = GetString(m, "http_server", "")

	// SSL
	d.HasSSL = GetBool(m, "has_ssl", false)
	d.SSLIssuer = GetString(m, "ssl_issuer", "")
	d.SSLValidFrom = GetString(m, "ssl_valid_from", "")
	d.SSLValidUntil = GetString(m, "ssl_valid_until", "")
	d.SSLWildcard = GetBool(m, "ssl_wildcard", false)

	// CDN/WAF
	d.CDN = GetString(m, "cdn", "")
	d.WAF = GetString(m, "waf", "")

	// Tags
	d.SubdomainLevel = GetInt(m, "subdomain_level", 0)

	return nil
}

// IsValid verifica si el metadata tiene datos válidos mínimos.
func (d *DomainMetadata) IsValid() bool {
	return len(d.ResolvedIPs) > 0 || d.Registrar != "" || d.HTTPStatus > 0
}

// Type retorna el tipo de metadata.
func (d *DomainMetadata) Type() string {
	return "domain"
}

// NewDomainMetadata crea un nuevo DomainMetadata vacío.
func NewDomainMetadata() *DomainMetadata {
	return &DomainMetadata{
		ResolvedIPs: []string{},
		DNSRecords:  []string{},
		Nameservers: []string{},
	}
}
