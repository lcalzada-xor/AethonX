// internal/core/domain/artifact_serialization_test.go
package domain

import (
	"encoding/json"
	"testing"

	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/testutil"
)

// TestArtifact_MarshalJSON_WithTypedMetadata verifica la serialización de TypedMetadata
func TestArtifact_MarshalJSON_WithTypedMetadata(t *testing.T) {
	// Crear artifact con TypedMetadata
	domainMeta := metadata.NewDomainMetadata()
	domainMeta.HasSSL = true
	domainMeta.SSLIssuer = "Let's Encrypt"
	domainMeta.SSLValidFrom = "2024-01-01"
	domainMeta.SSLValidUntil = "2025-01-01"
	domainMeta.Nameservers = []string{"ns1.example.com", "ns2.example.com"}
	domainMeta.Registrar = "Test Registrar"

	artifact := NewArtifactWithMetadata(
		ArtifactTypeDomain,
		"example.com",
		"test",
		domainMeta,
	)
	artifact.AddTag("production")
	artifact.AddRelation("target-id-123", RelationResolvesTo, 1.0, "test")

	// Serializar
	data, err := json.Marshal(artifact)
	testutil.AssertNoError(t, err, "marshal should succeed")

	// Verificar que JSON no está vacío
	testutil.AssertTrue(t, len(data) > 0, "marshaled data should not be empty")

	// Verificar estructura JSON
	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	testutil.AssertNoError(t, err, "unmarshal to map should succeed")

	// Verificar campos básicos
	testutil.AssertEqual(t, parsed["id"], artifact.ID, "ID should match")
	testutil.AssertEqual(t, parsed["type"], string(ArtifactTypeDomain), "type should match")
	testutil.AssertEqual(t, parsed["value"], "example.com", "value should match")

	// Verificar que metadata existe en JSON
	metadataInterface, metadataExists := parsed["metadata"]
	testutil.AssertTrue(t, metadataExists, "metadata field should exist")
	typedMeta, ok := metadataInterface.(map[string]interface{})
	testutil.AssertTrue(t, ok, "metadata should be a map")

	// Verificar estructura de metadata envelope
	testutil.AssertEqual(t, typedMeta["type"], "domain", "metadata type should be 'domain'")

	// Verificar que data existe dentro de metadata
	dataInterface, dataExists := typedMeta["data"]
	testutil.AssertTrue(t, dataExists, "metadata.data should exist")
	metaData, ok := dataInterface.(map[string]interface{})
	testutil.AssertTrue(t, ok, "metadata.data should be a map")

	// Verificar campos del metadata (campos JSON capitalizados porque no tienen tags json explícitos)
	testutil.AssertEqual(t, metaData["HasSSL"], true, "HasSSL should be true")
	testutil.AssertEqual(t, metaData["SSLIssuer"], "Let's Encrypt", "SSLIssuer should match")
	testutil.AssertEqual(t, metaData["SSLValidFrom"], "2024-01-01", "SSLValidFrom should match")
	testutil.AssertEqual(t, metaData["SSLValidUntil"], "2025-01-01", "SSLValidUntil should match")
	testutil.AssertEqual(t, metaData["Registrar"], "Test Registrar", "Registrar should match")

	// Verificar nameservers
	nameservers, ok := metaData["Nameservers"].([]interface{})
	testutil.AssertTrue(t, ok, "nameservers should be an array")
	testutil.AssertEqual(t, len(nameservers), 2, "should have 2 nameservers")
	testutil.AssertEqual(t, nameservers[0], "ns1.example.com", "first nameserver")
	testutil.AssertEqual(t, nameservers[1], "ns2.example.com", "second nameserver")

	// Verificar tags (omitempty - debería existir porque tiene valores)
	tags, tagsExist := parsed["tags"]
	testutil.AssertTrue(t, tagsExist, "tags should exist when not empty")
	tagsList, ok := tags.([]interface{})
	testutil.AssertTrue(t, ok, "tags should be an array")
	testutil.AssertEqual(t, len(tagsList), 1, "should have 1 tag")

	// Verificar relations (omitempty - debería existir porque tiene valores)
	relations, relationsExist := parsed["relations"]
	testutil.AssertTrue(t, relationsExist, "relations should exist when not empty")
	relationsList, ok := relations.([]interface{})
	testutil.AssertTrue(t, ok, "relations should be an array")
	testutil.AssertEqual(t, len(relationsList), 1, "should have 1 relation")
}

// TestArtifact_UnmarshalJSON_WithTypedMetadata verifica la deserialización de TypedMetadata
func TestArtifact_UnmarshalJSON_WithTypedMetadata(t *testing.T) {
	// Crear artifact original con TypedMetadata
	domainMeta := metadata.NewDomainMetadata()
	domainMeta.HasSSL = true
	domainMeta.SSLIssuer = "Let's Encrypt"
	domainMeta.SSLValidFrom = "2024-01-01"
	domainMeta.SSLValidUntil = "2025-01-01"
	domainMeta.Nameservers = []string{"ns1.example.com", "ns2.example.com"}
	domainMeta.Registrar = "Test Registrar"
	domainMeta.ResolvedIPs = []string{"192.0.2.1", "192.0.2.2"}

	original := NewArtifactWithMetadata(
		ArtifactTypeDomain,
		"example.com",
		"test",
		domainMeta,
	)
	original.AddTag("production")
	original.AddRelation("target-id-123", RelationResolvesTo, 1.0, "test")

	// Serializar
	data, err := json.Marshal(original)
	testutil.AssertNoError(t, err, "marshal should succeed")

	// Deserializar
	var deserialized Artifact
	err = json.Unmarshal(data, &deserialized)
	testutil.AssertNoError(t, err, "unmarshal should succeed")

	// Verificar campos básicos
	testutil.AssertEqual(t, deserialized.ID, original.ID, "ID should match")
	testutil.AssertEqual(t, deserialized.Type, original.Type, "type should match")
	testutil.AssertEqual(t, deserialized.Value, original.Value, "value should match")
	testutil.AssertEqual(t, deserialized.Confidence, original.Confidence, "confidence should match")

	// Verificar sources
	testutil.AssertLen(t, deserialized.Sources, len(original.Sources), "sources length")
	testutil.AssertContains(t, deserialized.Sources, "test", "sources should contain 'test'")

	// Verificar TypedMetadata se restauró correctamente
	testutil.AssertNotNil(t, deserialized.TypedMetadata, "typed metadata should not be nil")

	// Cast a DomainMetadata
	domainMetaRestored, ok := deserialized.TypedMetadata.(*metadata.DomainMetadata)
	testutil.AssertTrue(t, ok, "should be DomainMetadata")
	testutil.AssertNotNil(t, domainMetaRestored, "restored metadata should not be nil")

	// Verificar todos los campos del metadata
	testutil.AssertEqual(t, domainMetaRestored.HasSSL, true, "HasSSL should match")
	testutil.AssertEqual(t, domainMetaRestored.SSLIssuer, "Let's Encrypt", "SSLIssuer should match")
	testutil.AssertEqual(t, domainMetaRestored.SSLValidFrom, "2024-01-01", "SSLValidFrom should match")
	testutil.AssertEqual(t, domainMetaRestored.SSLValidUntil, "2025-01-01", "SSLValidUntil should match")
	testutil.AssertEqual(t, domainMetaRestored.Registrar, "Test Registrar", "Registrar should match")

	// Verificar arrays
	testutil.AssertLen(t, domainMetaRestored.Nameservers, 2, "nameservers length")
	testutil.AssertContains(t, domainMetaRestored.Nameservers, "ns1.example.com", "nameservers")
	testutil.AssertContains(t, domainMetaRestored.Nameservers, "ns2.example.com", "nameservers")

	testutil.AssertLen(t, domainMetaRestored.ResolvedIPs, 2, "resolved IPs length")
	testutil.AssertContains(t, domainMetaRestored.ResolvedIPs, "192.0.2.1", "resolved IPs")
	testutil.AssertContains(t, domainMetaRestored.ResolvedIPs, "192.0.2.2", "resolved IPs")

	// Verificar tags
	testutil.AssertLen(t, deserialized.Tags, 1, "tags length")
	testutil.AssertContains(t, deserialized.Tags, "production", "tags")

	// Verificar relations
	testutil.AssertEqual(t, len(deserialized.Relations), 1, "relations length should be 1")
	testutil.AssertEqual(t, deserialized.Relations[0].Type, RelationResolvesTo, "relation type")
	testutil.AssertEqual(t, deserialized.Relations[0].TargetID, "target-id-123", "relation target ID")
}

// TestArtifact_MarshalJSON_NoTypedMetadata verifica serialización sin TypedMetadata
func TestArtifact_MarshalJSON_NoTypedMetadata(t *testing.T) {
	// Crear artifact sin TypedMetadata
	artifact := NewArtifact(ArtifactTypeDomain, "example.com", "test")

	// Serializar
	data, err := json.Marshal(artifact)
	testutil.AssertNoError(t, err, "marshal should succeed")

	// Verificar estructura JSON
	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	testutil.AssertNoError(t, err, "unmarshal to map should succeed")

	// Verificar que metadata no existe cuando TypedMetadata es nil (omitempty)
	_, metadataExists := parsed["metadata"]
	testutil.AssertFalse(t, metadataExists, "metadata field should not exist when nil (omitempty)")
}

// TestArtifact_OmitEmpty verifica que omitempty funciona correctamente
func TestArtifact_OmitEmpty(t *testing.T) {
	// Crear artifact vacío (sin tags, relations, metadata)
	artifact := NewArtifact(ArtifactTypeDomain, "example.com", "test")

	// Serializar
	data, err := json.Marshal(artifact)
	testutil.AssertNoError(t, err, "marshal should succeed")

	// Verificar estructura JSON
	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	testutil.AssertNoError(t, err, "unmarshal to map should succeed")

	// Verificar que tags no existe (omitempty - array vacío)
	_, tagsExist := parsed["tags"]
	testutil.AssertFalse(t, tagsExist, "tags should be omitted when empty")

	// Verificar que relations no existe (omitempty - array vacío)
	_, relationsExist := parsed["relations"]
	testutil.AssertFalse(t, relationsExist, "relations should be omitted when empty")

	// Verificar que metadata no existe (omitempty - nil)
	_, metadataExists := parsed["metadata"]
	testutil.AssertFalse(t, metadataExists, "metadata should be omitted when nil")
}

// TestArtifact_RoundTrip verifica que serialización + deserialización preserva datos
func TestArtifact_RoundTrip(t *testing.T) {
	// Crear varios tipos de metadata
	tests := []struct {
		name     string
		artifactType ArtifactType
		metadata metadata.ArtifactMetadata
	}{
		{
			name:         "DomainMetadata",
			artifactType: ArtifactTypeDomain,
			metadata: &metadata.DomainMetadata{
				HasSSL:      true,
				SSLIssuer:   "Test CA",
				Nameservers: []string{"ns1.test.com"},
			},
		},
		{
			name:         "CertificateMetadata",
			artifactType: ArtifactTypeCertificate,
			metadata: &metadata.CertificateMetadata{
				IssuerCN:     "Test CA",
				SerialNumber: "123456",
				ValidFrom:    "2024-01-01",
				ValidUntil:   "2025-01-01",
			},
		},
		{
			name:         "IPMetadata",
			artifactType: ArtifactTypeIP,
			metadata: &metadata.IPMetadata{
				Country: "US",
				City:    "San Francisco",
				ASN:     "AS12345",
				ASOrg:   "Test AS",
				IPType:  "public",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Crear artifact con metadata
			original := NewArtifactWithMetadata(
				tt.artifactType,
				"test-value",
				"test-source",
				tt.metadata,
			)

			// Serializar
			data, err := json.Marshal(original)
			testutil.AssertNoError(t, err, "marshal should succeed")

			// Deserializar
			var restored Artifact
			err = json.Unmarshal(data, &restored)
			testutil.AssertNoError(t, err, "unmarshal should succeed")

			// Verificar que metadata se restauró
			testutil.AssertNotNil(t, restored.TypedMetadata, "typed metadata should be restored")

			// Verificar tipo de metadata
			originalType := metadata.GetMetadataType(original.TypedMetadata)
			restoredType := metadata.GetMetadataType(restored.TypedMetadata)
			testutil.AssertEqual(t, restoredType, originalType, "metadata type should match")
		})
	}
}
