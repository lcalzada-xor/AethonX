// internal/platform/cveapi/converter.go
package cveapi

import (
	"aethonx/internal/core/domain/metadata"
)

// ConvertToVulnerabilityMetadata converts EnrichedCVE to VulnerabilityMetadata.
// This bridges the gap between the CVE API data structures and AethonX's domain model.
func ConvertToVulnerabilityMetadata(enriched *EnrichedCVE, existing *metadata.VulnerabilityMetadata) {
	if enriched == nil || existing == nil {
		return
	}

	// Core fields
	existing.SourceIdentifier = enriched.SourceIdentifier
	existing.VulnStatus = enriched.VulnStatus

	// Dates
	existing.PublishedDate = enriched.PublishedDate
	existing.LastModifiedDate = enriched.LastModifiedDate
	existing.EnrichedAt = enriched.EnrichedAt

	// Description - get English description
	if len(enriched.Descriptions) > 0 {
		for _, desc := range enriched.Descriptions {
			if desc.Lang == "en" {
				existing.Description = desc.Value
				break
			}
		}
		// If no English found, use first available
		if existing.Description == "" {
			existing.Description = enriched.Descriptions[0].Value
		}

		// Store all descriptions
		existing.Descriptions = make([]metadata.Description, len(enriched.Descriptions))
		for i, desc := range enriched.Descriptions {
			existing.Descriptions[i] = metadata.Description{
				Lang:  desc.Lang,
				Value: desc.Value,
			}
		}
	}

	// Classification
	existing.CWEs = enriched.CWEs
	existing.CPEs = enriched.CPEs

	// CVSS v2
	if enriched.CVSSv2 != nil {
		existing.CVSSv2Vector = enriched.CVSSv2.Vector
		existing.CVSSv2Score = enriched.CVSSv2.Score
		existing.CVSSv2Severity = enriched.CVSSv2.Severity
		existing.CVSSv2ExploitScore = enriched.CVSSv2.ExploitabilityScore
		existing.CVSSv2ImpactScore = enriched.CVSSv2.ImpactScore
	}

	// CVSS v3
	if enriched.CVSSv3 != nil {
		existing.CVSSv3Vector = enriched.CVSSv3.Vector
		existing.CVSSv3Score = enriched.CVSSv3.Score
		existing.CVSSv3Severity = enriched.CVSSv3.Severity
		existing.CVSSv3ExploitScore = enriched.CVSSv3.ExploitabilityScore
		existing.CVSSv3ImpactScore = enriched.CVSSv3.ImpactScore

		// Attack metrics (from CVSS v3)
		existing.AttackVector = enriched.CVSSv3.AttackVector
		existing.AttackComplexity = enriched.CVSSv3.AttackComplexity
		existing.PrivilegesRequired = enriched.CVSSv3.PrivilegesRequired
		existing.UserInteraction = enriched.CVSSv3.UserInteraction
		existing.Scope = enriched.CVSSv3.Scope

		// Impact metrics
		existing.ConfidentialityImpact = enriched.CVSSv3.ConfidentialityImpact
		existing.IntegrityImpact = enriched.CVSSv3.IntegrityImpact
		existing.AvailabilityImpact = enriched.CVSSv3.AvailabilityImpact
	}

	// References
	if len(enriched.References) > 0 {
		existing.References = make([]metadata.Reference, len(enriched.References))
		for i, ref := range enriched.References {
			existing.References[i] = metadata.Reference{
				URL:    ref.URL,
				Source: ref.Source,
				Tags:   ref.Tags,
			}
		}
	}

	// Enrichment metadata
	existing.EnrichmentSource = enriched.EnrichmentSource
}
