// internal/core/domain/metadata/serializer.go
package metadata

import (
	"encoding/json"
	"fmt"
)

// MetadataEnvelope es un contenedor para serializar ArtifactMetadata polimórfico.
// Permite serializar/deserializar diferentes tipos de metadata de forma type-safe.
type MetadataEnvelope struct {
	Type string          `json:"type"` // "domain", "cert", "ip", etc.
	Data json.RawMessage `json:"data"` // Datos específicos del tipo
}

// TypeRegistry mapea tipos de metadata a sus nombres.
var TypeRegistry = map[string]func() ArtifactMetadata{
	"domain":        func() ArtifactMetadata { return &DomainMetadata{} },
	"certificate":   func() ArtifactMetadata { return &CertificateMetadata{} },
	"ip":            func() ArtifactMetadata { return &IPMetadata{} },
	"service":       func() ArtifactMetadata { return &ServiceMetadata{} },
	"technology":    func() ArtifactMetadata { return &TechnologyMetadata{} },
	"waf":           func() ArtifactMetadata { return &WAFMetadata{} },
	"backup_file":   func() ArtifactMetadata { return &BackupFileMetadata{} },
	"storage_bucket": func() ArtifactMetadata { return &StorageBucketMetadata{} },
	"api":           func() ArtifactMetadata { return &APIMetadata{} },
	"repository":    func() ArtifactMetadata { return &RepositoryMetadata{} },
	"webshell":      func() ArtifactMetadata { return &WebshellMetadata{} },
	"registrar":     func() ArtifactMetadata { return &RegistrarMetadata{} },
	"contact":       func() ArtifactMetadata { return &ContactMetadata{} },
}

// MarshalMetadata serializa ArtifactMetadata a MetadataEnvelope.
func MarshalMetadata(meta ArtifactMetadata) (*MetadataEnvelope, error) {
	if meta == nil {
		return nil, nil
	}

	// Obtener tipo
	metaType := GetMetadataType(meta)
	if metaType == "" {
		return nil, fmt.Errorf("unknown metadata type: %T", meta)
	}

	// Serializar datos
	data, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	return &MetadataEnvelope{
		Type: metaType,
		Data: data,
	}, nil
}

// UnmarshalMetadata deserializa MetadataEnvelope a ArtifactMetadata concreto.
func UnmarshalMetadata(envelope *MetadataEnvelope) (ArtifactMetadata, error) {
	if envelope == nil {
		return nil, nil
	}

	// Buscar factory para el tipo
	factory, exists := TypeRegistry[envelope.Type]
	if !exists {
		return nil, fmt.Errorf("unknown metadata type: %s", envelope.Type)
	}

	// Crear instancia vacía
	meta := factory()

	// Deserializar datos
	if err := json.Unmarshal(envelope.Data, meta); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata type %s: %w", envelope.Type, err)
	}

	return meta, nil
}

// GetMetadataType retorna el nombre del tipo de metadata.
func GetMetadataType(meta ArtifactMetadata) string {
	switch meta.(type) {
	case *DomainMetadata:
		return "domain"
	case *CertificateMetadata:
		return "certificate"
	case *IPMetadata:
		return "ip"
	case *ServiceMetadata:
		return "service"
	case *TechnologyMetadata:
		return "technology"
	case *WAFMetadata:
		return "waf"
	case *BackupFileMetadata:
		return "backup_file"
	case *StorageBucketMetadata:
		return "storage_bucket"
	case *APIMetadata:
		return "api"
	case *RepositoryMetadata:
		return "repository"
	case *WebshellMetadata:
		return "webshell"
	case *RegistrarMetadata:
		return "registrar"
	case *ContactMetadata:
		return "contact"
	default:
		return ""
	}
}
