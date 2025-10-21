package httpx

// HTTPXResponse represents the JSON output structure from httpx CLI tool.
// This struct maps directly to the JSONL output format when using -json flag.
type HTTPXResponse struct {
	Timestamp    string     `json:"timestamp"`
	Hash         *HashData  `json:"hash,omitempty"`
	Port         string     `json:"port"`
	URL          string     `json:"url"`
	Input        string     `json:"input"`
	Title        string     `json:"title,omitempty"`
	Scheme       string     `json:"scheme"`
	Webserver    string     `json:"webserver,omitempty"`
	ContentType  string     `json:"content_type,omitempty"`
	Method       string     `json:"method"`
	Host         string     `json:"host"`
	Path         string     `json:"path"`
	Favicon      string     `json:"favicon,omitempty"`
	FaviconMMH3  string     `json:"favicon_mmh3,omitempty"`
	JARM         string     `json:"jarm,omitempty"`
	JARMHash     string     `json:"jarm_hash,omitempty"`
	ResponseTime string     `json:"response_time,omitempty"`
	Time         string     `json:"time,omitempty"`
	Lines        int        `json:"lines,omitempty"`
	Words        int        `json:"words,omitempty"`
	StatusCode   int        `json:"status_code"`
	ContentLength int       `json:"content_length,omitempty"`
	Failed       bool       `json:"failed"`
	TechDetect   []string   `json:"tech,omitempty"`

	// TLS/Certificate fields
	TLS *TLSData `json:"tls,omitempty"`

	// Network fields
	IP      string         `json:"ip,omitempty"`
	CNAME   FlexibleString `json:"cname,omitempty"`
	ASN     *ASNData       `json:"asn,omitempty"`
	CDN     FlexibleBool   `json:"cdn,omitempty"`
	CDNName string         `json:"cdn_name,omitempty"`

	// Redirect chain
	Chain            []ChainItem `json:"chain,omitempty"`
	ChainStatusCodes []int       `json:"chain_status_codes,omitempty"`

	// Extracted FQDNs (when using -extract-fqdn)
	ExtractedFQDNs []string `json:"fqdn,omitempty"`

	// Websocket
	Websocket bool `json:"websocket,omitempty"`

	// HTTP Pipeline support
	Pipeline bool `json:"pipeline,omitempty"`

	// HTTP/2 support
	HTTP2 bool `json:"http2,omitempty"`
}

// HashData contains hash information for body and headers.
type HashData struct {
	BodyMD5       string `json:"body_md5,omitempty"`
	BodySHA256    string `json:"body_sha256,omitempty"`
	BodySHA512    string `json:"body_sha512,omitempty"`
	HeaderMD5     string `json:"header_md5,omitempty"`
	HeaderSHA256  string `json:"header_sha256,omitempty"`
	HeaderSHA512  string `json:"header_sha512,omitempty"`
}

// FingerprintHashData contains certificate fingerprint hashes.
type FingerprintHashData struct {
	MD5    string `json:"md5,omitempty"`
	SHA1   string `json:"sha1,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

// TLSData contains TLS/SSL certificate information from httpx.
type TLSData struct {
	Host            string               `json:"host"`
	Port            string               `json:"port"`
	ProbeStatus     bool                 `json:"probe_status"`
	Version         string               `json:"tls_version,omitempty"`
	Cipher          string               `json:"cipher"`
	TLSConnection   string               `json:"tls_connection"`
	SubjectDN       string               `json:"subject_dn"`
	IssuerDN        string               `json:"issuer_dn"`
	SubjectCN       string               `json:"subject_cn"`
	IssuerCN        string               `json:"issuer_cn"`
	SubjectAN       []string             `json:"subject_an"`
	NotBefore       string               `json:"not_before"`
	NotAfter        string               `json:"not_after"`
	Serial          string               `json:"serial,omitempty"`
	FingerprintHash *FingerprintHashData `json:"fingerprint_hash,omitempty"`
	WildcardCert    bool                 `json:"wildcard_certificate,omitempty"`
	SNI             string               `json:"sni,omitempty"`
}

// ASNData contains Autonomous System Number information.
type ASNData struct {
	ASN     string `json:"asn"`
	Country string `json:"country"`
	Org     string `json:"org"`
}

// ChainItem represents a single redirect in the chain.
type ChainItem struct {
	Request    string `json:"request"`
	Response   string `json:"response"`
	StatusCode int    `json:"status_code"`
	Location   string `json:"location,omitempty"`
	RequestURL string `json:"request-url"`
}
