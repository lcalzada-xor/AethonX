// internal/sources/shodan/responses.go
package shodan

import "time"

// ShodanHostResponse represents the response from /shodan/host/{ip} or /shodan/host/search endpoints.
// It contains detailed information about a host including services, vulnerabilities, and metadata.
type ShodanHostResponse struct {
	IPStr      string       `json:"ip_str"`
	Port       int          `json:"port"`
	Transport  string       `json:"transport"`
	Hostnames  []string     `json:"hostnames"`
	Domains    []string     `json:"domains"`
	Org        string       `json:"org"`
	ASN        string       `json:"asn"`
	ISP        string       `json:"isp"`
	Location   LocationData `json:"location"`
	SSL        *SSLData     `json:"ssl,omitempty"`
	Vulns      []string     `json:"vulns,omitempty"`
	Tags       []string     `json:"tags,omitempty"`
	Product    string       `json:"product,omitempty"`
	Version    string       `json:"version,omitempty"`
	CPE        []string     `json:"cpe,omitempty"`
	Banner     string       `json:"data,omitempty"`
	Timestamp  string       `json:"timestamp"`
	DeviceType string       `json:"devicetype,omitempty"`
	OS         string       `json:"os,omitempty"`
	Cloud      *CloudData   `json:"cloud,omitempty"`
}

// ShodanSearchResponse represents the response from /shodan/host/search endpoint.
type ShodanSearchResponse struct {
	Total   int                   `json:"total"`
	Matches []ShodanHostResponse  `json:"matches"`
}

// ShodanDomainResponse represents the response from /dns/domain/{domain} endpoint.
// Each record contains DNS information for a subdomain.
type ShodanDomainResponse struct {
	Domain    string   `json:"domain"`
	Subdomain string   `json:"subdomain"`
	Type      string   `json:"type"`     // A, AAAA, CNAME, NS, MX, TXT, SOA
	Value     string   `json:"value"`    // IP address or CNAME target
	LastSeen  string   `json:"last_seen"`
	Tags      []string `json:"tags,omitempty"`
}

// LocationData represents geographic location information.
type LocationData struct {
	CountryCode string  `json:"country_code"`
	CountryName string  `json:"country_name"`
	City        string  `json:"city"`
	RegionCode  string  `json:"region_code"`
	PostalCode  string  `json:"postal_code,omitempty"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	AreaCode    int     `json:"area_code,omitempty"`
}

// SSLData represents SSL/TLS certificate information.
type SSLData struct {
	Cert    CertData  `json:"cert"`
	Cipher  CipherData `json:"cipher,omitempty"`
	Version string    `json:"version,omitempty"` // TLSv1.2, TLSv1.3, etc.
}

// CertData represents SSL certificate details.
type CertData struct {
	Subject   CertName  `json:"subject"`
	Issuer    CertName  `json:"issuer"`
	Serial    string    `json:"serial"`
	Expired   bool      `json:"expired"`
	Expires   string    `json:"expires"`
	Issued    string    `json:"issued,omitempty"`
	Fingerprint FingerprintData `json:"fingerprint,omitempty"`
}

// CertName represents certificate subject or issuer name.
type CertName struct {
	CN string `json:"CN"` // Common Name
	C  string `json:"C,omitempty"`  // Country
	L  string `json:"L,omitempty"`  // Locality
	O  string `json:"O,omitempty"`  // Organization
	OU string `json:"OU,omitempty"` // Organizational Unit
}

// FingerprintData represents certificate fingerprints.
type FingerprintData struct {
	SHA1   string `json:"sha1,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

// CipherData represents SSL cipher information.
type CipherData struct {
	Version string   `json:"version,omitempty"`
	Name    string   `json:"name,omitempty"`
	Bits    int      `json:"bits,omitempty"`
}

// CloudData represents cloud provider information.
type CloudData struct {
	Provider string `json:"provider"` // aws, azure, gcp, digitalocean, etc.
	Service  string `json:"service,omitempty"`  // ec2, s3, compute-engine, etc.
	Region   string `json:"region,omitempty"`   // us-east-1, westeurope, etc.
}

// HTTPData represents HTTP-specific information.
type HTTPData struct {
	Title        string            `json:"title,omitempty"`
	StatusCode   int               `json:"status,omitempty"`
	Server       string            `json:"server,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	HTMLHash     string            `json:"html_hash,omitempty"`
	Redirects    []string          `json:"redirects,omitempty"`
}

// ParsedTime safely parses Shodan timestamp strings.
func ParsedTime(timestamp string) (time.Time, error) {
	// Shodan uses ISO 8601 format: "2024-01-15T10:30:45.123456"
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, timestamp); err == nil {
			return t, nil
		}
	}

	return time.Time{}, nil
}
