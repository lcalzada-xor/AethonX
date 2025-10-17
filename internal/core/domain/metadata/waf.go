// internal/core/domain/metadata/waf.go
package metadata

import (
	"strconv"
)

// WAFMetadata contiene información detallada sobre un Web Application Firewall detectado.
type WAFMetadata struct {
	// Identificación del WAF
	Name    string // "Cloudflare", "AWS WAF", "Akamai", "Imperva"
	Vendor  string // "Cloudflare Inc.", "Amazon", "Akamai Technologies"
	Product string // "Cloudflare WAF", "AWS WAF v2"

	// Detección
	DetectionMethod  string  // "header", "response_pattern", "error_page", "timing"
	DetectionPattern string  // Patrón que matcheó
	Confidence       float64 // 0.0-1.0

	// Configuración detectada
	RulesMode     string // "block", "challenge", "monitor"
	ChallengeType string // "captcha", "js_challenge", "managed_challenge"

	// Protecciones activas
	SQLiProtection bool
	XSSProtection  bool
	RCEProtection  bool
	RateLimiting   bool
	BotProtection  bool
	DDoSProtection bool

	// Fingerprinting
	Headers        []string // Headers reveladores
	ErrorPages     []string // Páginas de error características
	BlockedPayloads []string // Payloads que fueron bloqueados

	// Bypass potential
	BypassDifficulty string   // "trivial", "easy", "medium", "hard", "very_hard"
	KnownBypasses    []string // Técnicas conocidas de bypass

	// Performance impact
	LatencyAdded int // Milisegundos de latencia añadidos

	// URL donde se detectó
	DetectedURL string
}

func (w *WAFMetadata) ToMap() map[string]string {
	m := make(map[string]string)
	SetIfNotEmpty(m, "name", w.Name)
	SetIfNotEmpty(m, "vendor", w.Vendor)
	SetIfNotEmpty(m, "product", w.Product)
	SetIfNotEmpty(m, "detection_method", w.DetectionMethod)
	SetIfNotEmpty(m, "detection_pattern", w.DetectionPattern)
	if w.Confidence > 0 {
		m["confidence"] = strconv.FormatFloat(w.Confidence, 'f', 2, 64)
	}
	SetIfNotEmpty(m, "rules_mode", w.RulesMode)
	SetIfNotEmpty(m, "challenge_type", w.ChallengeType)
	SetBool(m, "sqli_protection", w.SQLiProtection)
	SetBool(m, "xss_protection", w.XSSProtection)
	SetBool(m, "rce_protection", w.RCEProtection)
	SetBool(m, "rate_limiting", w.RateLimiting)
	SetBool(m, "bot_protection", w.BotProtection)
	SetBool(m, "ddos_protection", w.DDoSProtection)
	if len(w.Headers) > 0 {
		m["headers"] = StringSliceToCSV(w.Headers)
	}
	if len(w.ErrorPages) > 0 {
		m["error_pages"] = StringSliceToCSV(w.ErrorPages)
	}
	if len(w.BlockedPayloads) > 0 {
		m["blocked_payloads"] = StringSliceToCSV(w.BlockedPayloads)
	}
	SetIfNotEmpty(m, "bypass_difficulty", w.BypassDifficulty)
	if len(w.KnownBypasses) > 0 {
		m["known_bypasses"] = StringSliceToCSV(w.KnownBypasses)
	}
	if w.LatencyAdded > 0 {
		SetInt(m, "latency_added", w.LatencyAdded)
	}
	SetIfNotEmpty(m, "detected_url", w.DetectedURL)
	return m
}

func (w *WAFMetadata) FromMap(m map[string]string) error {
	w.Name = GetString(m, "name", "")
	w.Vendor = GetString(m, "vendor", "")
	w.Product = GetString(m, "product", "")
	w.DetectionMethod = GetString(m, "detection_method", "")
	w.DetectionPattern = GetString(m, "detection_pattern", "")
	confStr := GetString(m, "confidence", "0")
	if conf, err := strconv.ParseFloat(confStr, 64); err == nil {
		w.Confidence = conf
	}
	w.RulesMode = GetString(m, "rules_mode", "")
	w.ChallengeType = GetString(m, "challenge_type", "")
	w.SQLiProtection = GetBool(m, "sqli_protection", false)
	w.XSSProtection = GetBool(m, "xss_protection", false)
	w.RCEProtection = GetBool(m, "rce_protection", false)
	w.RateLimiting = GetBool(m, "rate_limiting", false)
	w.BotProtection = GetBool(m, "bot_protection", false)
	w.DDoSProtection = GetBool(m, "ddos_protection", false)
	w.Headers = CSVToStringSlice(GetString(m, "headers", ""))
	w.ErrorPages = CSVToStringSlice(GetString(m, "error_pages", ""))
	w.BlockedPayloads = CSVToStringSlice(GetString(m, "blocked_payloads", ""))
	w.BypassDifficulty = GetString(m, "bypass_difficulty", "")
	w.KnownBypasses = CSVToStringSlice(GetString(m, "known_bypasses", ""))
	w.LatencyAdded = GetInt(m, "latency_added", 0)
	w.DetectedURL = GetString(m, "detected_url", "")
	return nil
}

func (w *WAFMetadata) IsValid() bool { return w.Name != "" }
func (w *WAFMetadata) Type() string  { return "waf" }

// NewWAFMetadata crea una instancia de WAFMetadata con valores por defecto.
func NewWAFMetadata(name string) *WAFMetadata {
	return &WAFMetadata{
		Name:       name,
		Confidence: 1.0,
	}
}
