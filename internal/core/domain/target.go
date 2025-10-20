// internal/core/domain/target.go
package domain

import (
	"fmt"
	"strings"

	"aethonx/internal/platform/validator"
)

// Target representa el objetivo del reconocimiento.
type Target struct {
	// Root es el dominio raíz objetivo
	Root string

	// Mode define el tipo de escaneo (pasivo, activo, híbrido)
	Mode ScanMode

	// Scope define qué incluir/excluir del reconocimiento
	Scope ScopeConfig

	// Tags adicionales para el target
	Tags []string

	// Metadata adicional
	Metadata map[string]string
}

// ScopeConfig define el alcance del reconocimiento.
type ScopeConfig struct {
	// IncludeSubdomains indica si se deben incluir subdominios
	IncludeSubdomains bool

	// ExcludeDomains lista de dominios a excluir
	ExcludeDomains []string

	// MaxDepth profundidad máxima para subdominios recursivos (0 = sin límite)
	MaxDepth int

	// OnlyInScope si es true, solo incluye dominios que hagan match con Root
	OnlyInScope bool
}

// NewTarget crea un nuevo target con valores por defecto.
func NewTarget(root string, mode ScanMode) *Target {
	return &Target{
		Root: root,
		Mode: mode,
		Scope: ScopeConfig{
			IncludeSubdomains: true,
			ExcludeDomains:    []string{},
			MaxDepth:          0,
			OnlyInScope:       true,
		},
		Tags:     []string{},
		Metadata: make(map[string]string),
	}
}

// Validate verifica que el target sea válido.
func (t *Target) Validate() error {
	if t.Root == "" {
		return ErrEmptyTarget
	}

	// Normalizar usando validator centralizado
	t.Root = validator.NormalizeDomain(t.Root)

	// Validar formato de dominio usando validator centralizado
	if !validator.IsDomain(t.Root) {
		return fmt.Errorf("%w: %s", ErrInvalidDomain, t.Root)
	}

	// Validar modo
	if !t.Mode.IsValid() {
		return fmt.Errorf("%w: %s", ErrInvalidScanMode, t.Mode)
	}

	// Validar scope
	if t.Scope.MaxDepth < 0 {
		return ErrInvalidScope
	}

	return nil
}

// IsInScope verifica si un dominio está dentro del alcance del target.
func (t *Target) IsInScope(domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))

	// Verificar si está en la lista de exclusión
	for _, excluded := range t.Scope.ExcludeDomains {
		if domain == excluded || strings.HasSuffix(domain, "."+excluded) {
			return false
		}
	}

	// Si OnlyInScope está activado, verificar que pertenece al root
	if t.Scope.OnlyInScope {
		if domain == t.Root {
			return true
		}
		if t.Scope.IncludeSubdomains && strings.HasSuffix(domain, "."+t.Root) {
			// Verificar profundidad máxima si está configurada
			if t.Scope.MaxDepth > 0 {
				depth := t.calculateSubdomainDepth(domain)
				if depth > t.Scope.MaxDepth {
					return false
				}
			}
			return true
		}
		return false
	}

	return true
}

// calculateSubdomainDepth calcula la profundidad de un subdominio relativo al root.
// Ejemplo: para root="example.com"
//   - "example.com" = 0
//   - "test.example.com" = 1
//   - "api.test.example.com" = 2
func (t *Target) calculateSubdomainDepth(domain string) int {
	if domain == t.Root {
		return 0
	}

	// Contar cuántos niveles adicionales hay
	// Remover el root del final
	if !strings.HasSuffix(domain, "."+t.Root) {
		return 0
	}

	// Extraer la parte del subdominio
	subdomain := strings.TrimSuffix(domain, "."+t.Root)

	// Contar los puntos en la parte del subdominio
	return strings.Count(subdomain, ".") + 1
}

// AddExclusion añade un dominio a la lista de exclusión.
func (t *Target) AddExclusion(domain string) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	for _, ex := range t.Scope.ExcludeDomains {
		if ex == domain {
			return
		}
	}
	t.Scope.ExcludeDomains = append(t.Scope.ExcludeDomains, domain)
}

// String retorna una representación legible del target.
func (t *Target) String() string {
	return fmt.Sprintf("Target{root=%s, mode=%s, scope=%v}", t.Root, t.Mode, t.Scope.IncludeSubdomains)
}
