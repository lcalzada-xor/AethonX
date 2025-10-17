// internal/core/domain/metadata/metadata.go
package metadata

import (
	"fmt"
	"strconv"
	"strings"
)

// ArtifactMetadata es la interfaz base que deben implementar todos los metadata tipados.
type ArtifactMetadata interface {
	// ToMap convierte el metadata a un mapa string->string para serialización
	ToMap() map[string]string

	// FromMap carga el metadata desde un mapa string->string
	FromMap(m map[string]string) error

	// IsValid verifica si el metadata contiene datos válidos
	IsValid() bool

	// Type retorna el tipo de metadata (para debugging)
	Type() string
}

// Helper functions para conversión de tipos comunes

// StringSliceToCSV convierte un slice de strings a CSV
func StringSliceToCSV(slice []string) string {
	return strings.Join(slice, ",")
}

// CSVToStringSlice convierte un CSV a slice de strings
func CSVToStringSlice(csv string) []string {
	if csv == "" {
		return []string{}
	}
	result := strings.Split(csv, ",")
	// Trim spaces
	for i := range result {
		result[i] = strings.TrimSpace(result[i])
	}
	return result
}

// IntSliceToCSV convierte un slice de ints a CSV
func IntSliceToCSV(slice []int) string {
	strs := make([]string, len(slice))
	for i, v := range slice {
		strs[i] = strconv.Itoa(v)
	}
	return strings.Join(strs, ",")
}

// CSVToIntSlice convierte un CSV a slice de ints
func CSVToIntSlice(csv string) []int {
	if csv == "" {
		return []int{}
	}
	parts := strings.Split(csv, ",")
	result := make([]int, 0, len(parts))
	for _, p := range parts {
		if v, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
			result = append(result, v)
		}
	}
	return result
}

// BoolToString convierte un bool a string
func BoolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// StringToBool convierte un string a bool
func StringToBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes" || s == "y"
}

// GetString obtiene un valor string del mapa, con valor por defecto
func GetString(m map[string]string, key, defaultValue string) string {
	if v, ok := m[key]; ok && v != "" {
		return v
	}
	return defaultValue
}

// GetInt obtiene un valor int del mapa, con valor por defecto
func GetInt(m map[string]string, key string, defaultValue int) int {
	if v, ok := m[key]; ok && v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultValue
}

// GetFloat obtiene un valor float64 del mapa, con valor por defecto
func GetFloat(m map[string]string, key string, defaultValue float64) float64 {
	if v, ok := m[key]; ok && v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

// GetBool obtiene un valor bool del mapa, con valor por defecto
func GetBool(m map[string]string, key string, defaultValue bool) bool {
	if v, ok := m[key]; ok {
		return StringToBool(v)
	}
	return defaultValue
}

// GetInt64 obtiene un valor int64 del mapa, con valor por defecto
func GetInt64(m map[string]string, key string, defaultValue int64) int64 {
	if v, ok := m[key]; ok && v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return defaultValue
}

// SetIfNotEmpty añade una clave al mapa solo si el valor no está vacío
func SetIfNotEmpty(m map[string]string, key, value string) {
	if value != "" {
		m[key] = value
	}
}

// SetInt añade un int al mapa
func SetInt(m map[string]string, key string, value int) {
	m[key] = strconv.Itoa(value)
}

// SetFloat añade un float al mapa
func SetFloat(m map[string]string, key string, value float64) {
	m[key] = fmt.Sprintf("%.2f", value)
}

// SetBool añade un bool al mapa
func SetBool(m map[string]string, key string, value bool) {
	m[key] = BoolToString(value)
}

// SetInt64 añade un int64 al mapa
func SetInt64(m map[string]string, key string, value int64) {
	m[key] = strconv.FormatInt(value, 10)
}
