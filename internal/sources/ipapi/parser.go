// internal/sources/ipapi/parser.go
package ipapi

import (
	"fmt"
	"regexp"
	"strings"

	domainmetadata "aethonx/internal/core/domain/metadata"
	"aethonx/internal/platform/logx"
)

// Parser converts ip-api.com responses into AethonX metadata structures.
type Parser struct {
	logger     logx.Logger
	sourceName string
}

// NewParser creates a new parser instance.
func NewParser(logger logx.Logger, sourceName string) *Parser {
	return &Parser{
		logger:     logger.With("component", "ipapi_parser"),
		sourceName: sourceName,
	}
}

// ParseToIPMetadata converts an IPAPIResponse into IPMetadata.
// It extracts geolocation, network info, and detects cloud providers.
func (p *Parser) ParseToIPMetadata(resp *IPAPIResponse) *domainmetadata.IPMetadata {
	ipMeta := domainmetadata.NewIPMetadata()

	// Geolocation fields
	ipMeta.Country = resp.Country
	ipMeta.CountryCode = resp.CountryCode
	ipMeta.Region = resp.RegionName // Use full region name instead of code
	ipMeta.City = resp.City
	ipMeta.Latitude = fmt.Sprintf("%.4f", resp.Lat)
	ipMeta.Longitude = fmt.Sprintf("%.4f", resp.Lon)
	ipMeta.Timezone = resp.Timezone

	// Network fields
	ipMeta.ISP = resp.ISP
	ipMeta.ASN = extractASN(resp.AS)
	ipMeta.ASOrg = extractASOrg(resp.AS)

	// Hosting/Cloud detection
	ipMeta.HostingProvider = resp.Org
	ipMeta.CloudProvider = detectCloudProvider(resp.Org, resp.ISP, resp.AS)

	// IP type detection
	ipMeta.IPType = detectIPType(resp.Org, resp.ISP)

	p.logger.Debug("parsed ip-api response",
		"ip", resp.Query,
		"country", ipMeta.Country,
		"asn", ipMeta.ASN,
		"cloud", ipMeta.CloudProvider,
	)

	return ipMeta
}

// extractASN extracts the ASN number from the "AS" field.
// Example: "AS15169 Google LLC" -> "AS15169"
func extractASN(asField string) string {
	if asField == "" {
		return ""
	}

	// Regex to match ASN format: AS followed by digits
	re := regexp.MustCompile(`^(AS\d+)`)
	matches := re.FindStringSubmatch(asField)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// extractASOrg extracts the organization name from the "AS" field.
// Example: "AS15169 Google LLC" -> "Google LLC"
func extractASOrg(asField string) string {
	if asField == "" {
		return ""
	}

	// Remove "AS<digits> " prefix
	re := regexp.MustCompile(`^AS\d+\s+(.+)`)
	matches := re.FindStringSubmatch(asField)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// If no match, return original (fallback)
	return asField
}

// detectCloudProvider identifies the cloud provider from Org, ISP, or AS fields.
// Supports: AWS, GCP, Azure, DigitalOcean, Linode, Vultr, OVH, Hetzner, Cloudflare.
func detectCloudProvider(org, isp, as string) string {
	// Combine all fields for matching
	combined := strings.ToLower(org + " " + isp + " " + as)

	// Cloud provider patterns (order matters - check specific before generic)
	providers := []struct {
		name     string
		patterns []string
	}{
		{"aws", []string{"amazon", "aws", "amazon.com", "amazon data services"}},
		{"gcp", []string{"google cloud", "google llc", "google inc", "google fiber"}},
		{"azure", []string{"microsoft", "azure", "microsoft corporation"}},
		{"digitalocean", []string{"digitalocean", "digital ocean"}},
		{"linode", []string{"linode", "akamai connected cloud"}},
		{"vultr", []string{"vultr", "choopa"}},
		{"ovh", []string{"ovh", "ovh sas"}},
		{"hetzner", []string{"hetzner", "hetzner online"}},
		{"cloudflare", []string{"cloudflare", "cloudflare inc"}},
		{"oracle", []string{"oracle cloud", "oracle corporation"}},
		{"alibaba", []string{"alibaba", "aliyun"}},
		{"scaleway", []string{"scaleway", "online sas"}},
		{"contabo", []string{"contabo"}},
		{"ionos", []string{"ionos", "1&1"}},
	}

	for _, provider := range providers {
		for _, pattern := range provider.patterns {
			if strings.Contains(combined, pattern) {
				return provider.name
			}
		}
	}

	return "" // Not a recognized cloud provider
}

// detectIPType categorizes the IP based on usage patterns.
// Types: datacenter, residential, mobile, hosting, cdn, vpn, tor, proxy
func detectIPType(org, isp string) string {
	combined := strings.ToLower(org + " " + isp)

	// Datacenter/Hosting indicators
	if strings.Contains(combined, "data center") ||
		strings.Contains(combined, "datacenter") ||
		strings.Contains(combined, "hosting") ||
		strings.Contains(combined, "server") ||
		strings.Contains(combined, "colocation") {
		return "datacenter"
	}

	// CDN indicators
	if strings.Contains(combined, "cloudflare") ||
		strings.Contains(combined, "fastly") ||
		strings.Contains(combined, "akamai") ||
		strings.Contains(combined, "cdn") {
		return "cdn"
	}

	// VPN indicators
	if strings.Contains(combined, "vpn") ||
		strings.Contains(combined, "virtual private") ||
		strings.Contains(combined, "nordvpn") ||
		strings.Contains(combined, "expressvpn") {
		return "vpn"
	}

	// Mobile indicators
	if strings.Contains(combined, "mobile") ||
		strings.Contains(combined, "wireless") ||
		strings.Contains(combined, "cellular") {
		return "mobile"
	}

	// Residential (default assumption if not datacenter/cdn/vpn)
	if strings.Contains(combined, "broadband") ||
		strings.Contains(combined, "telecom") ||
		strings.Contains(combined, "cable") ||
		strings.Contains(combined, "dsl") {
		return "residential"
	}

	// Default to public (generic)
	return "public"
}
