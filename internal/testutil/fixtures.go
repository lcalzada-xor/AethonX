// internal/testutil/fixtures.go
package testutil

// Fixture data para tests (valores primitivos solamente, sin dependencias de domain)

// FixtureDomains contiene dominios de prueba válidos.
var FixtureDomains = []string{
	"example.com",
	"test.example.com",
	"subdomain.example.com",
	"another.test.example.com",
}

// FixtureInvalidDomains contiene dominios inválidos.
var FixtureInvalidDomains = []string{
	"",
	"not a domain",
	"192.168.1.1",
	"2001:db8::1",
	"-invalid.com",
	"invalid-.com",
	".example.com",
	"example..com",
}

// FixtureIPs contiene IPs de prueba.
var FixtureIPs = []string{
	"192.168.1.1",
	"10.0.0.1",
	"172.16.0.1",
	"8.8.8.8",
}

// FixtureIPv6 contiene IPv6 de prueba.
var FixtureIPv6 = []string{
	"2001:db8::1",
	"fe80::1",
	"::1",
}

// FixtureEmails contiene emails de prueba.
var FixtureEmails = []string{
	"admin@example.com",
	"contact@example.com",
	"info@subdomain.example.com",
}

// FixtureURLs contiene URLs de prueba.
var FixtureURLs = []string{
	"https://example.com",
	"https://example.com/path",
	"https://subdomain.example.com/api/v1",
	"http://test.example.com:8080",
}
