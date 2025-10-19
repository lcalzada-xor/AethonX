// internal/core/domain/metadata/contact.go
package metadata

// ContactMetadata contiene información de contacto de dominios (WHOIS/RDAP)
type ContactMetadata struct {
	// Contact type: registrant, admin, tech, billing
	ContactType string

	// Personal/Organization info
	Name         string
	Organization string
	Email        string
	Phone        string

	// Address
	Street     string
	City       string
	State      string
	PostalCode string
	Country    string

	// Privacy
	Redacted bool // Si la información está redactada por privacidad
}

// NewContactMetadata crea un nuevo ContactMetadata vacío
func NewContactMetadata(contactType string) *ContactMetadata {
	return &ContactMetadata{
		ContactType: contactType,
	}
}

// ToMap implementa ArtifactMetadata
func (c *ContactMetadata) ToMap() map[string]string {
	m := make(map[string]string)

	SetIfNotEmpty(m, "contact_type", c.ContactType)
	SetIfNotEmpty(m, "name", c.Name)
	SetIfNotEmpty(m, "organization", c.Organization)
	SetIfNotEmpty(m, "email", c.Email)
	SetIfNotEmpty(m, "phone", c.Phone)
	SetIfNotEmpty(m, "street", c.Street)
	SetIfNotEmpty(m, "city", c.City)
	SetIfNotEmpty(m, "state", c.State)
	SetIfNotEmpty(m, "postal_code", c.PostalCode)
	SetIfNotEmpty(m, "country", c.Country)
	SetBool(m, "redacted", c.Redacted)

	return m
}

// FromMap implementa ArtifactMetadata
func (c *ContactMetadata) FromMap(m map[string]string) error {
	c.ContactType = GetString(m, "contact_type", "")
	c.Name = GetString(m, "name", "")
	c.Organization = GetString(m, "organization", "")
	c.Email = GetString(m, "email", "")
	c.Phone = GetString(m, "phone", "")
	c.Street = GetString(m, "street", "")
	c.City = GetString(m, "city", "")
	c.State = GetString(m, "state", "")
	c.PostalCode = GetString(m, "postal_code", "")
	c.Country = GetString(m, "country", "")
	c.Redacted = GetBool(m, "redacted", false)

	return nil
}

// IsValid implementa ArtifactMetadata
func (c *ContactMetadata) IsValid() bool {
	// Al menos debe tener email o nombre u organización
	return c.Email != "" || c.Name != "" || c.Organization != ""
}

// Type implementa ArtifactMetadata (método Type)
func (c *ContactMetadata) Type() string {
	return "contact"
}

// HasPrivateInfo verifica si contiene información privada no redactada
func (c *ContactMetadata) HasPrivateInfo() bool {
	return !c.Redacted && (c.Email != "" || c.Phone != "" || c.Name != "")
}
