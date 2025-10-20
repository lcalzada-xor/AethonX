// internal/core/domain/artifact_bench_test.go
package domain

import (
	"encoding/json"
	"testing"

	"aethonx/internal/core/domain/metadata"
)

// BenchmarkArtifact_MarshalJSON benchmarks artifact marshaling with TypedMetadata
func BenchmarkArtifact_MarshalJSON(b *testing.B) {
	domainMeta := metadata.NewDomainMetadata()
	domainMeta.HasSSL = true
	domainMeta.SSLIssuer = "Let's Encrypt"
	domainMeta.SSLValidFrom = "2024-01-01"
	domainMeta.SSLValidUntil = "2025-01-01"
	domainMeta.Nameservers = []string{"ns1.example.com", "ns2.example.com"}
	domainMeta.Registrar = "Test Registrar"
	domainMeta.ResolvedIPs = []string{"192.0.2.1", "192.0.2.2"}

	artifact := NewArtifactWithMetadata(
		ArtifactTypeDomain,
		"example.com",
		"test",
		domainMeta,
	)
	artifact.AddTag("production")
	artifact.AddRelation("target-id-123", RelationResolvesTo, 1.0, "test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(artifact)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkArtifact_MarshalJSON_NoMetadata benchmarks artifact marshaling without metadata
func BenchmarkArtifact_MarshalJSON_NoMetadata(b *testing.B) {
	artifact := NewArtifact(ArtifactTypeDomain, "example.com", "test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(artifact)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkArtifact_UnmarshalJSON benchmarks artifact unmarshaling with TypedMetadata
func BenchmarkArtifact_UnmarshalJSON(b *testing.B) {
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

	data, _ := json.Marshal(artifact)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var a Artifact
		err := json.Unmarshal(data, &a)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkArtifact_RoundTrip benchmarks full marshal + unmarshal cycle
func BenchmarkArtifact_RoundTrip(b *testing.B) {
	domainMeta := metadata.NewDomainMetadata()
	domainMeta.HasSSL = true
	domainMeta.SSLIssuer = "Let's Encrypt"
	domainMeta.Nameservers = []string{"ns1.example.com"}

	artifact := NewArtifactWithMetadata(
		ArtifactTypeDomain,
		"example.com",
		"test",
		domainMeta,
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := json.Marshal(artifact)
		if err != nil {
			b.Fatal(err)
		}

		var restored Artifact
		err = json.Unmarshal(data, &restored)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkArtifact_MarshalJSON_WithRelations benchmarks with many relations
func BenchmarkArtifact_MarshalJSON_WithRelations(b *testing.B) {
	domainMeta := metadata.NewDomainMetadata()
	domainMeta.HasSSL = true

	artifact := NewArtifactWithMetadata(
		ArtifactTypeDomain,
		"example.com",
		"test",
		domainMeta,
	)

	// Add 10 relations
	for i := 0; i < 10; i++ {
		artifact.AddRelation("target-id-"+string(rune(i)), RelationResolvesTo, 1.0, "test")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(artifact)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkArtifact_MarshalJSON_ComplexMetadata benchmarks with all metadata types
func BenchmarkArtifact_MarshalJSON_ComplexMetadata(b *testing.B) {
	domainMeta := metadata.NewDomainMetadata()
	domainMeta.HasSSL = true
	domainMeta.SSLIssuer = "Let's Encrypt"
	domainMeta.SSLValidFrom = "2024-01-01"
	domainMeta.SSLValidUntil = "2025-01-01"
	domainMeta.Nameservers = []string{"ns1.example.com", "ns2.example.com", "ns3.example.com"}
	domainMeta.Registrar = "Test Registrar"
	domainMeta.ResolvedIPs = []string{"192.0.2.1", "192.0.2.2", "192.0.2.3"}
	domainMeta.DNSRecords = []string{"A", "AAAA", "MX", "TXT"}
	domainMeta.HTTPStatus = 200
	domainMeta.HTTPServer = "nginx/1.20.1"
	domainMeta.CDN = "Cloudflare"
	domainMeta.WAF = "Cloudflare"
	domainMeta.Status = "active"
	domainMeta.DNSSEC = true

	artifact := NewArtifactWithMetadata(
		ArtifactTypeDomain,
		"example.com",
		"test",
		domainMeta,
	)
	artifact.AddTag("production")
	artifact.AddTag("critical")
	artifact.AddRelation("ip-1", RelationResolvesTo, 1.0, "test")
	artifact.AddRelation("cert-1", RelationUsesCert, 1.0, "test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(artifact)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkScanResult_MarshalJSON benchmarks full scan result serialization
func BenchmarkScanResult_MarshalJSON(b *testing.B) {
	target := *NewTarget("example.com", ScanModePassive)
	result := NewScanResult(target)

	// Add 100 artifacts
	for i := 0; i < 100; i++ {
		domainMeta := metadata.NewDomainMetadata()
		domainMeta.HasSSL = true
		domainMeta.SSLIssuer = "Let's Encrypt"

		artifact := NewArtifactWithMetadata(
			ArtifactTypeDomain,
			"sub"+string(rune(i))+".example.com",
			"test",
			domainMeta,
		)
		result.AddArtifact(artifact)
	}

	result.Finalize()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(result)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkScanResult_UnmarshalJSON benchmarks full scan result deserialization
func BenchmarkScanResult_UnmarshalJSON(b *testing.B) {
	target := *NewTarget("example.com", ScanModePassive)
	result := NewScanResult(target)

	// Add artifacts
	for i := 0; i < 100; i++ {
		domainMeta := metadata.NewDomainMetadata()
		domainMeta.HasSSL = true

		artifact := NewArtifactWithMetadata(
			ArtifactTypeDomain,
			"sub"+string(rune(i))+".example.com",
			"test",
			domainMeta,
		)
		result.AddArtifact(artifact)
	}

	result.Finalize()
	data, _ := json.Marshal(result)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var r ScanResult
		err := json.Unmarshal(data, &r)
		if err != nil {
			b.Fatal(err)
		}
	}
}
