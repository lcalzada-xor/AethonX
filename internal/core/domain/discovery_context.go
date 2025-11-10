// internal/core/domain/discovery_context.go
package domain

// DiscoveryContext contains metadata about how and where an artifact was discovered.
// This provides traceability and context for each finding.
type DiscoveryContext struct {
	// SourceURL is the URL or file where the artifact was discovered
	SourceURL string `json:"source_url,omitempty"`

	// SourceResource is the specific resource containing the finding
	SourceResource string `json:"source_resource,omitempty"`

	// LineNumber is the line number in the source file/resource
	LineNumber int `json:"line_number,omitempty"`

	// Context is the surrounding text or code context
	Context string `json:"context,omitempty"`

	// MatchPattern is the pattern/rule that matched (e.g., GF pattern name)
	MatchPattern string `json:"match_pattern,omitempty"`
}

// NewDiscoveryContext creates a new DiscoveryContext with the provided values.
func NewDiscoveryContext() *DiscoveryContext {
	return &DiscoveryContext{}
}

// WithSourceURL sets the source URL.
func (dc *DiscoveryContext) WithSourceURL(url string) *DiscoveryContext {
	dc.SourceURL = url
	return dc
}

// WithSourceResource sets the source resource.
func (dc *DiscoveryContext) WithSourceResource(resource string) *DiscoveryContext {
	dc.SourceResource = resource
	return dc
}

// WithLineNumber sets the line number.
func (dc *DiscoveryContext) WithLineNumber(line int) *DiscoveryContext {
	dc.LineNumber = line
	return dc
}

// WithContext sets the context.
func (dc *DiscoveryContext) WithContext(ctx string) *DiscoveryContext {
	dc.Context = ctx
	return dc
}

// WithMatchPattern sets the match pattern.
func (dc *DiscoveryContext) WithMatchPattern(pattern string) *DiscoveryContext {
	dc.MatchPattern = pattern
	return dc
}

// IsEmpty returns true if all fields are empty/zero.
func (dc *DiscoveryContext) IsEmpty() bool {
	return dc.SourceURL == "" &&
		dc.SourceResource == "" &&
		dc.LineNumber == 0 &&
		dc.Context == "" &&
		dc.MatchPattern == ""
}
