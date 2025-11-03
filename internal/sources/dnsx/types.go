// Package dnsx implements integration with Project Discovery's dnsx DNS toolkit.
// It performs DNS resolution and enrichment of subdomains discovered in Stage 1.
package dnsx

import "time"

// DNSXResponse represents the JSON output from dnsx CLI tool.
// dnsx provides comprehensive DNS record information with validation.
type DNSXResponse struct {
	Host       string      `json:"host"`
	TTL        int         `json:"ttl"`
	Resolver   []string    `json:"resolver"`
	A          []string    `json:"a"`
	AAAA       []string    `json:"aaaa"`
	CNAME      []string    `json:"cname"`
	MX         []string    `json:"mx"`
	NS         []string    `json:"ns"`
	TXT        []string    `json:"txt"`
	SOA        []SOARecord `json:"soa"`
	PTR        []string    `json:"ptr"`
	SRV        []string    `json:"srv"`
	CAA        []string    `json:"caa"`
	All        []string    `json:"all"`
	StatusCode string      `json:"status_code"`
	CDN        string      `json:"cdn,omitempty"`
	ASN        string      `json:"asn,omitempty"`
	RawResp    interface{} `json:"raw_resp,omitempty"`
	Timestamp  string      `json:"timestamp"`
}

// SOARecord represents Start of Authority DNS record.
type SOARecord struct {
	Name     string `json:"name"`
	NS       string `json:"ns"`
	Mailbox  string `json:"mailbox"`
	Serial   int64  `json:"serial"`
	Refresh  int    `json:"refresh"`
	Retry    int    `json:"retry"`
	Expire   int    `json:"expire"`
	MinTTL   int    `json:"minttl"`
}

// DNSXConfig holds configuration for dnsx source.
type DNSXConfig struct {
	Threads        int
	RateLimit      int
	QueryTypes     []string
	EnableCDN      bool
	EnableASN      bool
	WildcardFilter bool
	ExecPath       string
	Timeout        time.Duration
}

// DefaultDNSXConfig returns default configuration.
func DefaultDNSXConfig() DNSXConfig {
	return DNSXConfig{
		Threads:        100,
		RateLimit:      -1, // unlimited
		QueryTypes:     []string{"a", "aaaa", "cname", "mx", "txt", "ns", "soa"},
		EnableCDN:      true,
		EnableASN:      true,
		WildcardFilter: true,
		ExecPath:       "dnsx",
		Timeout:        60 * time.Second,
	}
}
