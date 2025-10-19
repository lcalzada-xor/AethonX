// internal/core/usecases/graph_service_test.go
package usecases

import (
	"testing"

	"aethonx/internal/core/domain"
	"aethonx/internal/core/domain/metadata"
	"aethonx/internal/platform/logx"
	"aethonx/internal/testutil"
)

// Test fixtures

func createTestArtifacts() []*domain.Artifact {
	// Create a test graph:
	// domain1 -> ns1 (has_nameserver)
	// domain1 -> cert1 (uses_cert)
	// domain1 -> email1 (has_contact)
	// subdomain1 -> domain1 (subdomain_of)
	// subdomain1 -> ip1 (resolves_to)
	// subdomain1 -> cert1 (uses_cert)
	// ip1 -> asn1 (owned_by)

	domain1 := domain.NewArtifact(domain.ArtifactTypeDomain, "example.com", "rdap")
	ns1 := domain.NewArtifact(domain.ArtifactTypeNameserver, "ns1.example.com", "rdap")
	cert1 := domain.NewArtifact(domain.ArtifactTypeCertificate, "abc123", "crtsh")
	email1 := domain.NewArtifact(domain.ArtifactTypeEmail, "admin@example.com", "rdap")
	subdomain1 := domain.NewArtifact(domain.ArtifactTypeSubdomain, "test.example.com", "crtsh")
	ip1 := domain.NewArtifact(domain.ArtifactTypeIP, "1.2.3.4", "dns")
	asn1 := domain.NewArtifact(domain.ArtifactTypeASN, "AS15169", "whois")

	// Add relations
	domain1.AddRelation(ns1.ID, domain.RelationHasNameserver, 1.0, "rdap")
	domain1.AddRelation(cert1.ID, domain.RelationUsesCert, 0.95, "crtsh")
	domain1.AddRelation(email1.ID, domain.RelationHasContact, 0.95, "rdap")

	subdomain1.AddRelation(domain1.ID, domain.RelationSubdomainOf, 1.0, "dns")
	subdomain1.AddRelation(ip1.ID, domain.RelationResolvesTo, 1.0, "dns")
	subdomain1.AddRelation(cert1.ID, domain.RelationUsesCert, 0.95, "crtsh")

	ip1.AddRelation(asn1.ID, domain.RelationOwnedBy, 0.9, "whois")

	return []*domain.Artifact{domain1, ns1, cert1, email1, subdomain1, ip1, asn1}
}

// Tests

func TestNewGraphService(t *testing.T) {
	logger := logx.New()
	artifacts := createTestArtifacts()

	graph := NewGraphService(artifacts, logger)

	testutil.AssertNotNil(t, graph, "graph should not be nil")
	testutil.AssertEqual(t, len(graph.artifacts), 7, "should have 7 artifacts")
	testutil.AssertTrue(t, len(graph.relationIndex) > 0, "relationIndex should be populated")
	testutil.AssertTrue(t, len(graph.reverseIndex) > 0, "reverseIndex should be populated")
}

func TestGraphService_GetArtifact(t *testing.T) {
	logger := logx.New()
	artifacts := createTestArtifacts()
	graph := NewGraphService(artifacts, logger)

	domain1 := artifacts[0] // example.com

	result := graph.GetArtifact(domain1.ID)

	testutil.AssertNotNil(t, result, "artifact should be found")
	testutil.AssertEqual(t, result.Value, "example.com", "artifact value should match")
}

func TestGraphService_GetArtifact_NotFound(t *testing.T) {
	logger := logx.New()
	artifacts := createTestArtifacts()
	graph := NewGraphService(artifacts, logger)

	result := graph.GetArtifact("nonexistent")

	// GetArtifact returns nil for non-existent artifacts (map lookup)
	if result != nil {
		t.Errorf("artifact should not be found: expected nil, got %v", result)
	}
}

func TestGraphService_GetRelated(t *testing.T) {
	logger := logx.New()
	artifacts := createTestArtifacts()
	graph := NewGraphService(artifacts, logger)

	domain1 := artifacts[0] // example.com

	// Get nameservers
	nameservers := graph.GetRelated(domain1.ID, domain.RelationHasNameserver)
	testutil.AssertEqual(t, len(nameservers), 1, "should have 1 nameserver")
	testutil.AssertEqual(t, nameservers[0].Value, "ns1.example.com", "nameserver value should match")

	// Get contacts
	contacts := graph.GetRelated(domain1.ID, domain.RelationHasContact)
	testutil.AssertEqual(t, len(contacts), 1, "should have 1 contact")
	testutil.AssertEqual(t, contacts[0].Value, "admin@example.com", "contact value should match")

	// Get certs
	certs := graph.GetRelated(domain1.ID, domain.RelationUsesCert)
	testutil.AssertEqual(t, len(certs), 1, "should have 1 certificate")
	testutil.AssertEqual(t, certs[0].Value, "abc123", "cert value should match")
}

func TestGraphService_GetRelated_NoRelations(t *testing.T) {
	logger := logx.New()
	artifacts := createTestArtifacts()
	graph := NewGraphService(artifacts, logger)

	ns1 := artifacts[1] // ns1.example.com (has no outgoing relations)

	result := graph.GetRelated(ns1.ID, domain.RelationHasNameserver)

	// GetRelated returns nil or empty slice for no relations
	if result != nil && len(result) > 0 {
		t.Errorf("should return nil or empty for no relations, got %d results", len(result))
	}
}

func TestGraphService_GetReverseRelated(t *testing.T) {
	logger := logx.New()
	artifacts := createTestArtifacts()
	graph := NewGraphService(artifacts, logger)

	cert1 := artifacts[2] // cert abc123

	// Find all artifacts that use this cert (reverse lookup)
	users := graph.GetReverseRelated(cert1.ID, domain.RelationUsesCert)

	testutil.AssertEqual(t, len(users), 2, "should have 2 artifacts using this cert")

	// Should be domain1 and subdomain1
	values := []string{users[0].Value, users[1].Value}
	testutil.AssertContains(t, values, "example.com", "should contain domain")
	testutil.AssertContains(t, values, "test.example.com", "should contain subdomain")
}

func TestGraphService_GetReverseRelated_NoRelations(t *testing.T) {
	logger := logx.New()
	artifacts := createTestArtifacts()
	graph := NewGraphService(artifacts, logger)

	domain1 := artifacts[0] // example.com (nothing points to it with has_nameserver)

	result := graph.GetReverseRelated(domain1.ID, domain.RelationHasNameserver)

	// GetReverseRelated returns nil or empty slice for no relations
	if result != nil && len(result) > 0 {
		t.Errorf("should return nil or empty for no reverse relations, got %d results", len(result))
	}
}

func TestGraphService_GetAllRelations(t *testing.T) {
	logger := logx.New()
	artifacts := createTestArtifacts()
	graph := NewGraphService(artifacts, logger)

	domain1 := artifacts[0] // example.com

	relations := graph.GetAllRelations(domain1.ID)

	testutil.AssertEqual(t, len(relations), 3, "should have 3 relations")

	// Check relation types
	relTypes := make(map[domain.RelationType]bool)
	for _, rel := range relations {
		relTypes[rel.Type] = true
	}

	testutil.AssertTrue(t, relTypes[domain.RelationHasNameserver], "should have has_nameserver")
	testutil.AssertTrue(t, relTypes[domain.RelationUsesCert], "should have uses_cert")
	testutil.AssertTrue(t, relTypes[domain.RelationHasContact], "should have has_contact")
}

func TestGraphService_GetNeighbors_Depth1(t *testing.T) {
	logger := logx.New()
	artifacts := createTestArtifacts()
	graph := NewGraphService(artifacts, logger)

	domain1 := artifacts[0] // example.com

	neighbors := graph.GetNeighbors(domain1.ID, 1)

	// Depth 1: ns1, cert1, email1
	testutil.AssertEqual(t, len(neighbors), 3, "should have 3 neighbors at depth 1")

	values := []string{neighbors[0].Value, neighbors[1].Value, neighbors[2].Value}
	testutil.AssertContains(t, values, "ns1.example.com", "should contain nameserver")
	testutil.AssertContains(t, values, "abc123", "should contain certificate")
	testutil.AssertContains(t, values, "admin@example.com", "should contain email")
}

func TestGraphService_GetNeighbors_Depth2(t *testing.T) {
	logger := logx.New()
	artifacts := createTestArtifacts()
	graph := NewGraphService(artifacts, logger)

	subdomain1 := artifacts[4] // test.example.com

	neighbors := graph.GetNeighbors(subdomain1.ID, 2)

	// Depth 1: domain1, ip1, cert1
	// Depth 2: ns1, email1, asn1 (through domain1 and ip1)
	// Note: cert1 is already visited at depth 1, so won't be counted again

	testutil.AssertTrue(t, len(neighbors) >= 5, "should have at least 5 neighbors at depth 2")

	// Extract values
	values := make([]string, len(neighbors))
	for i, n := range neighbors {
		values[i] = n.Value
	}

	// Check some expected values
	testutil.AssertContains(t, values, "example.com", "should contain domain")
	testutil.AssertContains(t, values, "1.2.3.4", "should contain IP")
	testutil.AssertContains(t, values, "abc123", "should contain cert")
}

func TestGraphService_GetNeighbors_DepthZero(t *testing.T) {
	logger := logx.New()
	artifacts := createTestArtifacts()
	graph := NewGraphService(artifacts, logger)

	domain1 := artifacts[0]

	neighbors := graph.GetNeighbors(domain1.ID, 0)

	// GetNeighbors returns nil or empty slice for depth 0
	if neighbors != nil && len(neighbors) > 0 {
		t.Errorf("should return nil or empty for depth 0, got %d results", len(neighbors))
	}
}

func TestGraphService_FindPath_DirectConnection(t *testing.T) {
	logger := logx.New()
	artifacts := createTestArtifacts()
	graph := NewGraphService(artifacts, logger)

	domain1 := artifacts[0]  // example.com
	cert1 := artifacts[2]    // cert abc123

	path := graph.FindPath(domain1.ID, cert1.ID)

	testutil.AssertNotNil(t, path, "path should exist")
	testutil.AssertEqual(t, len(path), 1, "path should have 1 hop")
	testutil.AssertEqual(t, path[0].Type, domain.RelationUsesCert, "relation type should be uses_cert")
	testutil.AssertEqual(t, path[0].TargetID, cert1.ID, "target should be cert1")
}

func TestGraphService_FindPath_TwoHops(t *testing.T) {
	logger := logx.New()
	artifacts := createTestArtifacts()
	graph := NewGraphService(artifacts, logger)

	subdomain1 := artifacts[4] // test.example.com
	ns1 := artifacts[1]        // ns1.example.com

	// Path: subdomain1 -> domain1 -> ns1
	path := graph.FindPath(subdomain1.ID, ns1.ID)

	testutil.AssertNotNil(t, path, "path should exist")
	testutil.AssertEqual(t, len(path), 2, "path should have 2 hops")
	testutil.AssertEqual(t, path[0].Type, domain.RelationSubdomainOf, "first hop should be subdomain_of")
	testutil.AssertEqual(t, path[1].Type, domain.RelationHasNameserver, "second hop should be has_nameserver")
}

func TestGraphService_FindPath_ThreeHops(t *testing.T) {
	logger := logx.New()
	artifacts := createTestArtifacts()
	graph := NewGraphService(artifacts, logger)

	subdomain1 := artifacts[4] // test.example.com
	asn1 := artifacts[6]       // AS15169

	// Path: subdomain1 -> ip1 -> asn1
	path := graph.FindPath(subdomain1.ID, asn1.ID)

	testutil.AssertNotNil(t, path, "path should exist")
	testutil.AssertEqual(t, len(path), 2, "path should have 2 hops")
	testutil.AssertEqual(t, path[0].Type, domain.RelationResolvesTo, "first hop should be resolves_to")
	testutil.AssertEqual(t, path[1].Type, domain.RelationOwnedBy, "second hop should be owned_by")
}

func TestGraphService_FindPath_NoPath(t *testing.T) {
	logger := logx.New()
	artifacts := createTestArtifacts()
	graph := NewGraphService(artifacts, logger)

	ns1 := artifacts[1]   // ns1.example.com
	asn1 := artifacts[6]  // AS15169

	// No path from ns1 to asn1 (ns1 has no outgoing relations)
	path := graph.FindPath(ns1.ID, asn1.ID)

	// FindPath returns nil or empty slice when no path exists
	if path != nil && len(path) > 0 {
		t.Errorf("path should not exist, got %d hops", len(path))
	}
}

func TestGraphService_FindPath_SameNode(t *testing.T) {
	logger := logx.New()
	artifacts := createTestArtifacts()
	graph := NewGraphService(artifacts, logger)

	domain1 := artifacts[0]

	path := graph.FindPath(domain1.ID, domain1.ID)

	// FindPath returns nil or empty slice for same node
	if path != nil && len(path) > 0 {
		t.Errorf("path to self should return nil or empty, got %d hops", len(path))
	}
}

func TestGraphService_FindByType(t *testing.T) {
	logger := logx.New()
	artifacts := createTestArtifacts()
	graph := NewGraphService(artifacts, logger)

	// Find all domains
	domains := graph.FindByType(domain.ArtifactTypeDomain)
	testutil.AssertEqual(t, len(domains), 1, "should have 1 domain")
	testutil.AssertEqual(t, domains[0].Value, "example.com", "domain value should match")

	// Find all subdomains
	subdomains := graph.FindByType(domain.ArtifactTypeSubdomain)
	testutil.AssertEqual(t, len(subdomains), 1, "should have 1 subdomain")
	testutil.AssertEqual(t, subdomains[0].Value, "test.example.com", "subdomain value should match")

	// Find all IPs
	ips := graph.FindByType(domain.ArtifactTypeIP)
	testutil.AssertEqual(t, len(ips), 1, "should have 1 IP")
	testutil.AssertEqual(t, ips[0].Value, "1.2.3.4", "IP value should match")

	// Find all certificates
	certs := graph.FindByType(domain.ArtifactTypeCertificate)
	testutil.AssertEqual(t, len(certs), 1, "should have 1 certificate")
	testutil.AssertEqual(t, certs[0].Value, "abc123", "cert value should match")
}

func TestGraphService_FindByType_NoMatches(t *testing.T) {
	logger := logx.New()
	artifacts := createTestArtifacts()
	graph := NewGraphService(artifacts, logger)

	// Find all URLs (none exist)
	urls := graph.FindByType(domain.ArtifactTypeURL)
	testutil.AssertEqual(t, len(urls), 0, "should have 0 URLs")
}

func TestGraphService_GetStats(t *testing.T) {
	logger := logx.New()
	artifacts := createTestArtifacts()
	graph := NewGraphService(artifacts, logger)

	stats := graph.GetStats()

	testutil.AssertEqual(t, stats.TotalArtifacts, 7, "should have 7 artifacts")
	testutil.AssertEqual(t, stats.TotalRelations, 7, "should have 7 relations")

	// Check relations by type
	testutil.AssertEqual(t, stats.RelationsByType[domain.RelationHasNameserver], 1, "should have 1 has_nameserver")
	testutil.AssertEqual(t, stats.RelationsByType[domain.RelationUsesCert], 2, "should have 2 uses_cert")
	testutil.AssertEqual(t, stats.RelationsByType[domain.RelationHasContact], 1, "should have 1 has_contact")
	testutil.AssertEqual(t, stats.RelationsByType[domain.RelationSubdomainOf], 1, "should have 1 subdomain_of")
	testutil.AssertEqual(t, stats.RelationsByType[domain.RelationResolvesTo], 1, "should have 1 resolves_to")
	testutil.AssertEqual(t, stats.RelationsByType[domain.RelationOwnedBy], 1, "should have 1 owned_by")

	testutil.AssertTrue(t, stats.IndexSizeForward > 0, "forward index should be populated")
	testutil.AssertTrue(t, stats.IndexSizeReverse > 0, "reverse index should be populated")
}

func TestGraphService_EmptyGraph(t *testing.T) {
	logger := logx.New()
	artifacts := []*domain.Artifact{}

	graph := NewGraphService(artifacts, logger)

	testutil.AssertNotNil(t, graph, "graph should not be nil")

	stats := graph.GetStats()
	testutil.AssertEqual(t, stats.TotalArtifacts, 0, "should have 0 artifacts")
	testutil.AssertEqual(t, stats.TotalRelations, 0, "should have 0 relations")
}

func TestGraphService_SingleArtifact_NoRelations(t *testing.T) {
	logger := logx.New()
	artifact := domain.NewArtifact(domain.ArtifactTypeDomain, "example.com", "test")
	artifacts := []*domain.Artifact{artifact}

	graph := NewGraphService(artifacts, logger)

	testutil.AssertEqual(t, len(graph.artifacts), 1, "should have 1 artifact")

	neighbors := graph.GetNeighbors(artifact.ID, 1)
	testutil.AssertEqual(t, len(neighbors), 0, "should have 0 neighbors")

	stats := graph.GetStats()
	testutil.AssertEqual(t, stats.TotalArtifacts, 1, "should have 1 artifact")
	testutil.AssertEqual(t, stats.TotalRelations, 0, "should have 0 relations")
}

func TestGraphService_ComplexGraph_WithCycles(t *testing.T) {
	logger := logx.New()

	// Create a graph with cycles
	// A -> B -> C -> A
	a := domain.NewArtifact(domain.ArtifactTypeDomain, "a.com", "test")
	b := domain.NewArtifact(domain.ArtifactTypeDomain, "b.com", "test")
	c := domain.NewArtifact(domain.ArtifactTypeDomain, "c.com", "test")

	a.AddRelation(b.ID, domain.RelationHasCNAME, 1.0, "test")
	b.AddRelation(c.ID, domain.RelationHasCNAME, 1.0, "test")
	c.AddRelation(a.ID, domain.RelationHasCNAME, 1.0, "test")

	artifacts := []*domain.Artifact{a, b, c}
	graph := NewGraphService(artifacts, logger)

	// GetNeighbors should handle cycles without infinite loop
	neighbors := graph.GetNeighbors(a.ID, 5)

	// Should visit each node once: b and c
	testutil.AssertEqual(t, len(neighbors), 2, "should have 2 unique neighbors despite cycle")

	values := []string{neighbors[0].Value, neighbors[1].Value}
	testutil.AssertContains(t, values, "b.com", "should contain b.com")
	testutil.AssertContains(t, values, "c.com", "should contain c.com")
}

func TestGraphService_BuildIndexes_WithMetadata(t *testing.T) {
	logger := logx.New()

	domain1 := domain.NewArtifactWithMetadata(
		domain.ArtifactTypeDomain,
		"example.com",
		"rdap",
		metadata.NewDomainMetadata(),
	)

	cert1 := domain.NewArtifactWithMetadata(
		domain.ArtifactTypeCertificate,
		"abc123",
		"crtsh",
		&metadata.CertificateMetadata{
			IssuerCN:     "Let's Encrypt",
			SerialNumber: "abc123",
		},
	)

	relationMeta := map[string]string{
		"issuer": "Let's Encrypt",
		"valid":  "true",
	}
	domain1.AddRelationWithMetadata(cert1.ID, domain.RelationUsesCert, 0.95, "crtsh", relationMeta)

	artifacts := []*domain.Artifact{domain1, cert1}
	graph := NewGraphService(artifacts, logger)

	// Verify relation with metadata exists
	relations := graph.GetAllRelations(domain1.ID)
	testutil.AssertEqual(t, len(relations), 1, "should have 1 relation")
	testutil.AssertEqual(t, relations[0].Metadata["issuer"], "Let's Encrypt", "metadata should be preserved")
	testutil.AssertEqual(t, relations[0].Metadata["valid"], "true", "metadata should be preserved")
}
