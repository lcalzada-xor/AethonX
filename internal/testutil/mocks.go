// internal/testutil/mocks.go
package testutil

import (
	"context"
)

// Nota: Los mocks específicos de domain/ports están en sus respectivos paquetes
// Este archivo contiene solo utilidades genéricas sin dependencias circulares

// MockHTTPClient es un mock genérico para HTTP clients (futuro uso)
type MockHTTPClient struct {
	DoFunc      func(ctx context.Context, method, url string, body []byte) ([]byte, error)
	CallCount   int
	LastURL     string
	LastMethod  string
}

// NewMockHTTPClient crea un mock de HTTP client
func NewMockHTTPClient() *MockHTTPClient {
	return &MockHTTPClient{
		CallCount: 0,
	}
}

// Do simula una llamada HTTP
func (m *MockHTTPClient) Do(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	m.CallCount++
	m.LastURL = url
	m.LastMethod = method
	if m.DoFunc != nil {
		return m.DoFunc(ctx, method, url, body)
	}
	return []byte("{}"), nil
}
