// internal/core/domain/metadata/cloud.go
package metadata

// CloudMetadata contains information about cloud infrastructure.
type CloudMetadata struct {
	Provider string   // aws, azure, gcp, digitalocean, etc.
	Service  string   // ec2, s3, compute-engine, etc.
	Region   string   // us-east-1, westeurope, etc.
	Tags     []string // Additional tags
}

// ToMap converts CloudMetadata to map[string]string.
func (c *CloudMetadata) ToMap() map[string]string {
	m := make(map[string]string)
	SetIfNotEmpty(m, "provider", c.Provider)
	SetIfNotEmpty(m, "service", c.Service)
	SetIfNotEmpty(m, "region", c.Region)
	if len(c.Tags) > 0 {
		m["tags"] = StringSliceToCSV(c.Tags)
	}
	return m
}

// FromMap loads CloudMetadata from map[string]string.
func (c *CloudMetadata) FromMap(m map[string]string) error {
	c.Provider = GetString(m, "provider", "")
	c.Service = GetString(m, "service", "")
	c.Region = GetString(m, "region", "")
	c.Tags = CSVToStringSlice(GetString(m, "tags", ""))
	return nil
}

// IsValid verifies if the metadata has valid minimum data.
func (c *CloudMetadata) IsValid() bool {
	return c.Provider != ""
}

// Type returns the metadata type.
func (c *CloudMetadata) Type() string {
	return "cloud"
}

// NewCloudMetadata creates a new CloudMetadata instance.
func NewCloudMetadata(provider string) *CloudMetadata {
	return &CloudMetadata{
		Provider: provider,
		Tags:     []string{},
	}
}
