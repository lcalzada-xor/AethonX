// internal/platform/ui/artifact_summary.go
package ui

import (
	"fmt"
	"sort"
	"strings"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/ui/terminal"
)

const (
	MaxSamplesPerType        = 3  // Top 3 examples per type
	ShowSamplesThreshold     = 5  // Only show samples if type has > 5 artifacts
	MaxCategoryDisplayLength = 100 // Max characters for category line
)

// ArtifactCategory represents a logical grouping of artifact types
type ArtifactCategory struct {
	Name  string
	Icon  string
	Order int // Display order
}

var (
	CategoryHighValue = ArtifactCategory{Name: "High-Value Assets", Icon: "●", Order: 1}
	CategoryWebInfra  = ArtifactCategory{Name: "Web Infrastructure", Icon: "◆", Order: 2}
	CategorySecurity  = ArtifactCategory{Name: "Security & Technology", Icon: "▲", Order: 3}
	CategoryNetwork   = ArtifactCategory{Name: "Network Information", Icon: "◈", Order: 4}
	CategoryOther     = ArtifactCategory{Name: "Other", Icon: "○", Order: 5}
)

// ArtifactCategories maps artifact types to categories
var ArtifactCategories = map[domain.ArtifactType]ArtifactCategory{
	domain.ArtifactTypeSubdomain:  CategoryHighValue,
	domain.ArtifactTypeDomain:     CategoryHighValue,
	domain.ArtifactTypeEmail:      CategoryHighValue,
	domain.ArtifactTypeIP:         CategoryHighValue,
	domain.ArtifactTypeURL:        CategoryWebInfra,
	domain.ArtifactTypeCertificate: CategoryWebInfra,
	domain.ArtifactTypePort:       CategoryWebInfra,
	domain.ArtifactTypeService:    CategoryWebInfra,
	domain.ArtifactTypeVulnerability: CategorySecurity,
	domain.ArtifactTypeTechnology:    CategorySecurity,
	domain.ArtifactTypeHTTPHeader:    CategorySecurity,
	domain.ArtifactTypeSecurityHeader: CategorySecurity,
	domain.ArtifactTypeTLSConfig:     CategorySecurity,
	domain.ArtifactTypeWAF:            CategorySecurity,
	domain.ArtifactTypeASN:            CategoryNetwork,
	domain.ArtifactTypeCIDR:       CategoryNetwork,
	domain.ArtifactTypeNameserver: CategoryNetwork,
}

// ArtifactTypeIcons maps artifact types to their visual icons
// Using compatible Unicode characters (avoiding multi-code emojis)
var ArtifactTypeIcons = map[domain.ArtifactType]string{
	domain.ArtifactTypeSubdomain:       "●", // Bullet for subdomains
	domain.ArtifactTypeDomain:          "◉", // Fisheye for main domains
	domain.ArtifactTypeEmail:           "@", // At symbol (perfect for email)
	domain.ArtifactTypeIP:              "»", // Arrow for IPs
	domain.ArtifactTypeURL:             "~", // Tilde for URLs
	domain.ArtifactTypeCertificate:     "♦", // Diamond for certificates
	domain.ArtifactTypePort:            "▪", // Small square for ports
	domain.ArtifactTypeService:         "◈", // Cross square for services
	domain.ArtifactTypeVulnerability:   "⚠", // Warning (compatible)
	domain.ArtifactTypeTechnology:      "▶", // Triangle arrow for tech
	domain.ArtifactTypeHTTPHeader:      "≡", // Triple bar for headers
	domain.ArtifactTypeSecurityHeader:  "▲", // Triangle for security
	domain.ArtifactTypeTLSConfig:       "♢", // Lozenge for TLS
	domain.ArtifactTypeWAF:             "▣", // Square with fill for WAF
	domain.ArtifactTypeASN:             "▫", // Small white square for ASN
	domain.ArtifactTypeCIDR:            "▪", // Small black square for CIDR
	domain.ArtifactTypeNameserver:      "◆", // Black diamond for nameservers
}

// categorizedArtifactType holds artifact type info with samples
type categorizedArtifactType struct {
	Type     domain.ArtifactType
	Count    int
	Samples  []string
	Category ArtifactCategory
	Icon     string
}

// categorizedGroup holds a category with its artifact types
type categorizedGroup struct {
	Category ArtifactCategory
	Types    []categorizedArtifactType
	Total    int
}

// FormatArtifactSummary creates a detailed, categorized summary of artifacts
func FormatArtifactSummary(artifactsByType map[string]int, allArtifacts []*domain.Artifact) string {
	if len(artifactsByType) == 0 {
		return ""
	}

	// Build artifact type map for quick lookup
	artifactsByTypeObj := make(map[domain.ArtifactType][]*domain.Artifact)
	for _, artifact := range allArtifacts {
		artifactsByTypeObj[artifact.Type] = append(artifactsByTypeObj[artifact.Type], artifact)
	}

	// Categorize artifacts
	categories := categorizeArtifacts(artifactsByType, artifactsByTypeObj)

	// Sort categories by order
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Category.Order < categories[j].Category.Order
	})

	// Build output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("\n%s %s\n\n",
		terminal.Colorize(IconStats, terminal.RGB(255, 107, 53)),
		terminal.BoldText("ARTIFACTS BY TYPE")))

	for i, group := range categories {
		if group.Total == 0 {
			continue
		}

		// Category header
		output.WriteString(fmt.Sprintf("  %s %s %s\n",
			terminal.Colorize(group.Category.Icon, terminal.BrightCyan),
			terminal.BoldText(group.Category.Name),
			terminal.Colorize(fmt.Sprintf("(%s artifacts)", formatNumber(group.Total)), terminal.Gray)))

		// Sort types by count (descending)
		sort.Slice(group.Types, func(i, j int) bool {
			return group.Types[i].Count > group.Types[j].Count
		})

		// Display each artifact type
		for j, typeInfo := range group.Types {
			isLast := j == len(group.Types)-1
			connector := "├─"
			subConnector := "│  "
			if isLast {
				connector = "└─"
				subConnector = "   "
			}

			// Type line
			output.WriteString(fmt.Sprintf("  %s %s %s: %s\n",
				connector,
				terminal.Colorize(typeInfo.Icon, terminal.White),
				terminal.Colorize(string(typeInfo.Type), terminal.White),
				terminal.Colorize(formatNumber(typeInfo.Count), terminal.BrightCyan)))

			// Samples (if applicable)
			if len(typeInfo.Samples) > 0 {
				for k, sample := range typeInfo.Samples {
					sampleConnector := "├─"
					if k == len(typeInfo.Samples)-1 {
						sampleConnector = "└─"
					}

					// Truncate long samples
					truncatedSample := truncateString(sample, 60)
					output.WriteString(fmt.Sprintf("  %s %s %s\n",
						subConnector,
						sampleConnector,
						terminal.Colorize(truncatedSample, terminal.Gray)))
				}

				// "... N more" indicator
				remaining := typeInfo.Count - len(typeInfo.Samples)
				if remaining > 0 {
					output.WriteString(fmt.Sprintf("  %s    %s\n",
						subConnector,
						terminal.Colorize(fmt.Sprintf("... %s more", formatNumber(remaining)), terminal.Gray)))
				}
			}
		}

		// Add spacing between categories (except last)
		if i < len(categories)-1 {
			output.WriteString("\n")
		}
	}

	// Footer hint
	output.WriteString(fmt.Sprintf("\n  %s %s\n",
		terminal.Colorize("💡", terminal.BrightYellow),
		terminal.Colorize("Full details available in JSON output", terminal.Gray)))

	return output.String()
}

// categorizeArtifacts groups artifacts by category
func categorizeArtifacts(artifactsByType map[string]int, artifactsByTypeObj map[domain.ArtifactType][]*domain.Artifact) []categorizedGroup {
	categoryMap := make(map[string]*categorizedGroup)

	// Initialize all categories
	for _, cat := range []ArtifactCategory{CategoryHighValue, CategoryWebInfra, CategorySecurity, CategoryNetwork, CategoryOther} {
		categoryMap[cat.Name] = &categorizedGroup{
			Category: cat,
			Types:    []categorizedArtifactType{},
			Total:    0,
		}
	}

	// Group artifact types by category
	for typeStr, count := range artifactsByType {
		artifactType := domain.ArtifactType(typeStr)

		// Determine category
		category, exists := ArtifactCategories[artifactType]
		if !exists {
			category = CategoryOther
		}

		// Get icon
		icon, iconExists := ArtifactTypeIcons[artifactType]
		if !iconExists {
			icon = "📄"
		}

		// Get samples
		samples := getSamples(artifactsByTypeObj[artifactType], count)

		// Add to category
		group := categoryMap[category.Name]
		group.Types = append(group.Types, categorizedArtifactType{
			Type:     artifactType,
			Count:    count,
			Samples:  samples,
			Category: category,
			Icon:     icon,
		})
		group.Total += count
	}

	// Convert to slice and filter empty categories
	result := []categorizedGroup{}
	for _, group := range categoryMap {
		if group.Total > 0 {
			result = append(result, *group)
		}
	}

	return result
}

// getSamples extracts sample values from artifacts
func getSamples(artifacts []*domain.Artifact, totalCount int) []string {
	if totalCount <= MaxSamplesPerType {
		// Show all if count is small
		samples := make([]string, 0, len(artifacts))
		for _, a := range artifacts {
			samples = append(samples, a.Value)
		}
		return samples
	}

	if totalCount < ShowSamplesThreshold {
		// Don't show samples for very small types
		return nil
	}

	// Extract top samples (by confidence, then alphabetically)
	sorted := make([]*domain.Artifact, len(artifacts))
	copy(sorted, artifacts)

	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Confidence != sorted[j].Confidence {
			return sorted[i].Confidence > sorted[j].Confidence
		}
		return sorted[i].Value < sorted[j].Value
	})

	samples := make([]string, 0, MaxSamplesPerType)
	for i := 0; i < MaxSamplesPerType && i < len(sorted); i++ {
		samples = append(samples, sorted[i].Value)
	}

	return samples
}

// formatNumber formats a number with thousand separators
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	// Simple comma formatting
	str := fmt.Sprintf("%d", n)
	var result strings.Builder
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(c)
	}
	return result.String()
}

// truncateString truncates a string to maxLen with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
