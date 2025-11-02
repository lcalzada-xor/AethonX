// internal/sources/shodan/metadata.go
package shodan

import (
	"aethonx/internal/core/domain/metadata"
)

// Re-export helper functions for convenience within shodan package.

var (
	// NewCloudMetadata creates a new CloudMetadata instance.
	NewCloudMetadata = metadata.NewCloudMetadata
)

// NewVulnerabilityMetadata creates a new VulnerabilityMetadata for Shodan.
func NewVulnerabilityMetadata(cve string) *metadata.VulnerabilityMetadata {
	return metadata.NewVulnerabilityMetadata(cve, "shodan-internetdb")
}
