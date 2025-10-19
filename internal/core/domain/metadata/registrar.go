// internal/core/domain/metadata/registrar.go
package metadata

import (
	"time"
)

// RegistrarMetadata contiene información sobre el registrador de dominios (WHOIS/RDAP)
type RegistrarMetadata struct {
	// Registrar information
	RegistrarName string
	RegistrarURL  string
	RegistrarIANA string // IANA ID

	// Domain status
	Status []string // e.g., "clientTransferProhibited", "active"

	// Important dates
	CreatedDate  string
	UpdatedDate  string
	ExpiryDate   string

	// DNSSEC
	DNSSECEnabled bool

	// Nameservers
	Nameservers []string

	// Additional info
	Organization string
	Country      string
}

// NewRegistrarMetadata crea un nuevo RegistrarMetadata vacío
func NewRegistrarMetadata() *RegistrarMetadata {
	return &RegistrarMetadata{
		Status:      []string{},
		Nameservers: []string{},
	}
}

// ToMap implementa ArtifactMetadata
func (r *RegistrarMetadata) ToMap() map[string]string {
	m := make(map[string]string)

	SetIfNotEmpty(m, "registrar_name", r.RegistrarName)
	SetIfNotEmpty(m, "registrar_url", r.RegistrarURL)
	SetIfNotEmpty(m, "registrar_iana", r.RegistrarIANA)
	SetIfNotEmpty(m, "status", StringSliceToCSV(r.Status))
	SetIfNotEmpty(m, "created_date", r.CreatedDate)
	SetIfNotEmpty(m, "updated_date", r.UpdatedDate)
	SetIfNotEmpty(m, "expiry_date", r.ExpiryDate)
	SetBool(m, "dnssec_enabled", r.DNSSECEnabled)
	SetIfNotEmpty(m, "nameservers", StringSliceToCSV(r.Nameservers))
	SetIfNotEmpty(m, "organization", r.Organization)
	SetIfNotEmpty(m, "country", r.Country)

	return m
}

// FromMap implementa ArtifactMetadata
func (r *RegistrarMetadata) FromMap(m map[string]string) error {
	r.RegistrarName = GetString(m, "registrar_name", "")
	r.RegistrarURL = GetString(m, "registrar_url", "")
	r.RegistrarIANA = GetString(m, "registrar_iana", "")
	r.Status = CSVToStringSlice(GetString(m, "status", ""))
	r.CreatedDate = GetString(m, "created_date", "")
	r.UpdatedDate = GetString(m, "updated_date", "")
	r.ExpiryDate = GetString(m, "expiry_date", "")
	r.DNSSECEnabled = GetBool(m, "dnssec_enabled", false)
	r.Nameservers = CSVToStringSlice(GetString(m, "nameservers", ""))
	r.Organization = GetString(m, "organization", "")
	r.Country = GetString(m, "country", "")

	return nil
}

// IsValid implementa ArtifactMetadata
func (r *RegistrarMetadata) IsValid() bool {
	// Al menos debe tener el nombre del registrador o la organización
	return r.RegistrarName != "" || r.Organization != ""
}

// Type implementa ArtifactMetadata
func (r *RegistrarMetadata) Type() string {
	return "registrar"
}

// IsExpired verifica si el dominio ha expirado
func (r *RegistrarMetadata) IsExpired() bool {
	if r.ExpiryDate == "" {
		return false
	}

	// Parse common date formats
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, r.ExpiryDate); err == nil {
			return time.Now().After(t)
		}
	}

	return false
}

// DaysUntilExpiry calcula los días hasta la expiración
func (r *RegistrarMetadata) DaysUntilExpiry() int {
	if r.ExpiryDate == "" {
		return -1
	}

	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, r.ExpiryDate); err == nil {
			duration := time.Until(t)
			return int(duration.Hours() / 24)
		}
	}

	return -1
}
