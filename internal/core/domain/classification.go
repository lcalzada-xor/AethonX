// internal/core/domain/classification.go
package domain

// ResourceType represents types of web resources.
type ResourceType string

const (
	ResourceTypeStatic   ResourceType = "static"
	ResourceTypeDynamic  ResourceType = "dynamic"
	ResourceTypeAPI      ResourceType = "api"
	ResourceTypeEndpoint ResourceType = "endpoint"
)

// ParameterType represents types of parameters.
type ParameterType string

const (
	ParamTypeQuery  ParameterType = "query"
	ParamTypePath   ParameterType = "path"
	ParamTypeBody   ParameterType = "body"
	ParamTypeHeader ParameterType = "header"
)

// DataType represents types of data discovered.
type DataType string

const (
	DataTypeCredential    DataType = "credential"
	DataTypeEmail         DataType = "email"
	DataTypeInternalIP    DataType = "internal_ip"
	DataTypeDBConnection  DataType = "db_connection"
	DataTypeOAuthToken    DataType = "oauth_token"
	DataTypeDeveloperNote DataType = "developer_note"
	DataTypeAPIKey        DataType = "api_key"
	DataTypeJWT           DataType = "jwt"
	DataTypePassword      DataType = "password"
	DataTypeSecret        DataType = "secret"
)

// Classification contains categorization metadata for artifacts.
type Classification struct {
	// ResourceType categorizes the type of web resource
	ResourceType ResourceType `json:"resource_type,omitempty"`

	// ParameterType categorizes the type of parameter
	ParameterType ParameterType `json:"parameter_type,omitempty"`

	// DataType categorizes the type of data discovered
	DataType DataType `json:"data_type,omitempty"`

	// IsExternal indicates if this artifact belongs to an external domain
	IsExternal bool `json:"is_external,omitempty"`

	// Category provides additional free-form categorization
	Category string `json:"category,omitempty"`
}

// NewClassification creates a new Classification.
func NewClassification() *Classification {
	return &Classification{}
}

// WithResourceType sets the resource type.
func (c *Classification) WithResourceType(rt ResourceType) *Classification {
	c.ResourceType = rt
	return c
}

// WithParameterType sets the parameter type.
func (c *Classification) WithParameterType(pt ParameterType) *Classification {
	c.ParameterType = pt
	return c
}

// WithDataType sets the data type.
func (c *Classification) WithDataType(dt DataType) *Classification {
	c.DataType = dt
	return c
}

// WithExternal marks the artifact as external.
func (c *Classification) WithExternal(external bool) *Classification {
	c.IsExternal = external
	return c
}

// WithCategory sets a custom category.
func (c *Classification) WithCategory(category string) *Classification {
	c.Category = category
	return c
}

// IsEmpty returns true if all fields are empty/zero.
func (c *Classification) IsEmpty() bool {
	return c.ResourceType == "" &&
		c.ParameterType == "" &&
		c.DataType == "" &&
		!c.IsExternal &&
		c.Category == ""
}
