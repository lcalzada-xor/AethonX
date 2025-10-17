// internal/core/domain/metadata/ip.go
package metadata

// ServiceSummary representa un resumen de un servicio detectado en una IP.
// Se usa para referencia rápida en IPMetadata, mientras que el servicio completo
// se almacena como un artifact separado con ServiceMetadata.
type ServiceSummary struct {
	Port     int
	Protocol string
	Name     string
	Product  string
	Version  string
}

// IPMetadata contiene información detallada sobre una dirección IP.
type IPMetadata struct {
	// Geolocalización
	Country     string
	CountryCode string
	Region      string
	City        string
	Latitude    string
	Longitude   string
	Timezone    string

	// Red
	ASN    string // Autonomous System Number
	ASOrg  string // Organización del AS
	ISP    string
	CIDR   string // Rango de red

	// Hosting
	HostingProvider string
	Datacenter      string
	CloudProvider   string // aws, azure, gcp, etc.

	// DNS
	PTRRecord  string // Reverse DNS
	ReverseDNS string // Hostname

	// Puertos y servicios
	OpenPorts       []int            // Lista simple de puertos abiertos
	Services        []string         // Lista simple de nombres de servicios (legacy)
	ServicesSummary []ServiceSummary // Resumen estructurado de servicios detectados

	// Reputación
	Reputation      string // clean, suspicious, malicious
	Blacklisted     bool
	BlocklistCount  int
	ThreatScore     float64 // 0.0-1.0

	// Tipo
	IPType    string // public, private, reserved, bogon
	IPVersion string // 4 o 6
}

// ToMap convierte IPMetadata a map[string]string.
func (i *IPMetadata) ToMap() map[string]string {
	m := make(map[string]string)

	// Geolocalización
	SetIfNotEmpty(m, "country", i.Country)
	SetIfNotEmpty(m, "country_code", i.CountryCode)
	SetIfNotEmpty(m, "region", i.Region)
	SetIfNotEmpty(m, "city", i.City)
	SetIfNotEmpty(m, "latitude", i.Latitude)
	SetIfNotEmpty(m, "longitude", i.Longitude)
	SetIfNotEmpty(m, "timezone", i.Timezone)

	// Red
	SetIfNotEmpty(m, "asn", i.ASN)
	SetIfNotEmpty(m, "as_org", i.ASOrg)
	SetIfNotEmpty(m, "isp", i.ISP)
	SetIfNotEmpty(m, "cidr", i.CIDR)

	// Hosting
	SetIfNotEmpty(m, "hosting_provider", i.HostingProvider)
	SetIfNotEmpty(m, "datacenter", i.Datacenter)
	SetIfNotEmpty(m, "cloud_provider", i.CloudProvider)

	// DNS
	SetIfNotEmpty(m, "ptr_record", i.PTRRecord)
	SetIfNotEmpty(m, "reverse_dns", i.ReverseDNS)

	// Puertos
	if len(i.OpenPorts) > 0 {
		m["open_ports"] = IntSliceToCSV(i.OpenPorts)
	}
	if len(i.Services) > 0 {
		m["services"] = StringSliceToCSV(i.Services)
	}

	// Reputación
	SetIfNotEmpty(m, "reputation", i.Reputation)
	SetBool(m, "blacklisted", i.Blacklisted)
	if i.BlocklistCount > 0 {
		SetInt(m, "blocklist_count", i.BlocklistCount)
	}
	if i.ThreatScore > 0 {
		SetFloat(m, "threat_score", i.ThreatScore)
	}

	// Tipo
	SetIfNotEmpty(m, "ip_type", i.IPType)
	SetIfNotEmpty(m, "ip_version", i.IPVersion)

	return m
}

// FromMap carga IPMetadata desde map[string]string.
func (i *IPMetadata) FromMap(m map[string]string) error {
	// Geolocalización
	i.Country = GetString(m, "country", "")
	i.CountryCode = GetString(m, "country_code", "")
	i.Region = GetString(m, "region", "")
	i.City = GetString(m, "city", "")
	i.Latitude = GetString(m, "latitude", "")
	i.Longitude = GetString(m, "longitude", "")
	i.Timezone = GetString(m, "timezone", "")

	// Red
	i.ASN = GetString(m, "asn", "")
	i.ASOrg = GetString(m, "as_org", "")
	i.ISP = GetString(m, "isp", "")
	i.CIDR = GetString(m, "cidr", "")

	// Hosting
	i.HostingProvider = GetString(m, "hosting_provider", "")
	i.Datacenter = GetString(m, "datacenter", "")
	i.CloudProvider = GetString(m, "cloud_provider", "")

	// DNS
	i.PTRRecord = GetString(m, "ptr_record", "")
	i.ReverseDNS = GetString(m, "reverse_dns", "")

	// Puertos
	i.OpenPorts = CSVToIntSlice(GetString(m, "open_ports", ""))
	i.Services = CSVToStringSlice(GetString(m, "services", ""))

	// Reputación
	i.Reputation = GetString(m, "reputation", "")
	i.Blacklisted = GetBool(m, "blacklisted", false)
	i.BlocklistCount = GetInt(m, "blocklist_count", 0)
	i.ThreatScore = GetFloat(m, "threat_score", 0.0)

	// Tipo
	i.IPType = GetString(m, "ip_type", "")
	i.IPVersion = GetString(m, "ip_version", "")

	return nil
}

// IsValid verifica si el metadata tiene datos válidos mínimos.
func (i *IPMetadata) IsValid() bool {
	return i.Country != "" || i.ASN != "" || i.ISP != ""
}

// Type retorna el tipo de metadata.
func (i *IPMetadata) Type() string {
	return "ip"
}

// NewIPMetadata crea un nuevo IPMetadata vacío.
func NewIPMetadata() *IPMetadata {
	return &IPMetadata{
		OpenPorts:       []int{},
		Services:        []string{},
		ServicesSummary: []ServiceSummary{},
	}
}
