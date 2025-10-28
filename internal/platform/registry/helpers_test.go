package registry

import (
	"testing"
	"time"
)

// TestGetStringConfig tests string extraction from custom config
func TestGetStringConfig(t *testing.T) {
	tests := []struct {
		name         string
		custom       map[string]interface{}
		key          string
		defaultValue string
		expected     string
	}{
		{
			name:         "existing string value",
			custom:       map[string]interface{}{"key": "value"},
			key:          "key",
			defaultValue: "default",
			expected:     "value",
		},
		{
			name:         "missing key",
			custom:       map[string]interface{}{"other": "value"},
			key:          "key",
			defaultValue: "default",
			expected:     "default",
		},
		{
			name:         "nil map",
			custom:       nil,
			key:          "key",
			defaultValue: "default",
			expected:     "default",
		},
		{
			name:         "empty string value",
			custom:       map[string]interface{}{"key": ""},
			key:          "key",
			defaultValue: "default",
			expected:     "default",
		},
		{
			name:         "wrong type (int)",
			custom:       map[string]interface{}{"key": 123},
			key:          "key",
			defaultValue: "default",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetStringConfig(tt.custom, tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestGetIntConfig tests int extraction from custom config
func TestGetIntConfig(t *testing.T) {
	tests := []struct {
		name         string
		custom       map[string]interface{}
		key          string
		defaultValue int
		expected     int
	}{
		{
			name:         "existing int value",
			custom:       map[string]interface{}{"key": 42},
			key:          "key",
			defaultValue: 10,
			expected:     42,
		},
		{
			name:         "existing float64 value",
			custom:       map[string]interface{}{"key": float64(42)},
			key:          "key",
			defaultValue: 10,
			expected:     42,
		},
		{
			name:         "missing key",
			custom:       map[string]interface{}{"other": 42},
			key:          "key",
			defaultValue: 10,
			expected:     10,
		},
		{
			name:         "nil map",
			custom:       nil,
			key:          "key",
			defaultValue: 10,
			expected:     10,
		},
		{
			name:         "wrong type (string)",
			custom:       map[string]interface{}{"key": "42"},
			key:          "key",
			defaultValue: 10,
			expected:     10,
		},
		{
			name:         "zero value",
			custom:       map[string]interface{}{"key": 0},
			key:          "key",
			defaultValue: 10,
			expected:     0, // Zero is valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetIntConfig(tt.custom, tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestGetBoolConfig tests bool extraction from custom config
func TestGetBoolConfig(t *testing.T) {
	tests := []struct {
		name         string
		custom       map[string]interface{}
		key          string
		defaultValue bool
		expected     bool
	}{
		{
			name:         "existing true value",
			custom:       map[string]interface{}{"key": true},
			key:          "key",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "existing false value",
			custom:       map[string]interface{}{"key": false},
			key:          "key",
			defaultValue: true,
			expected:     false, // False is valid
		},
		{
			name:         "missing key",
			custom:       map[string]interface{}{"other": true},
			key:          "key",
			defaultValue: false,
			expected:     false,
		},
		{
			name:         "nil map",
			custom:       nil,
			key:          "key",
			defaultValue: true,
			expected:     true,
		},
		{
			name:         "wrong type (int)",
			custom:       map[string]interface{}{"key": 1},
			key:          "key",
			defaultValue: false,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetBoolConfig(tt.custom, tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestGetDurationConfig tests duration extraction from custom config
func TestGetDurationConfig(t *testing.T) {
	tests := []struct {
		name         string
		custom       map[string]interface{}
		key          string
		defaultValue time.Duration
		expected     time.Duration
	}{
		{
			name:         "existing duration value",
			custom:       map[string]interface{}{"key": 5 * time.Second},
			key:          "key",
			defaultValue: 10 * time.Second,
			expected:     5 * time.Second,
		},
		{
			name:         "int64 nanoseconds",
			custom:       map[string]interface{}{"key": int64(5000000000)},
			key:          "key",
			defaultValue: 10 * time.Second,
			expected:     5 * time.Second,
		},
		{
			name:         "float64 nanoseconds",
			custom:       map[string]interface{}{"key": float64(5000000000)},
			key:          "key",
			defaultValue: 10 * time.Second,
			expected:     5 * time.Second,
		},
		{
			name:         "string duration",
			custom:       map[string]interface{}{"key": "5s"},
			key:          "key",
			defaultValue: 10 * time.Second,
			expected:     5 * time.Second,
		},
		{
			name:         "missing key",
			custom:       map[string]interface{}{"other": 5 * time.Second},
			key:          "key",
			defaultValue: 10 * time.Second,
			expected:     10 * time.Second,
		},
		{
			name:         "nil map",
			custom:       nil,
			key:          "key",
			defaultValue: 10 * time.Second,
			expected:     10 * time.Second,
		},
		{
			name:         "invalid string duration",
			custom:       map[string]interface{}{"key": "invalid"},
			key:          "key",
			defaultValue: 10 * time.Second,
			expected:     10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetDurationConfig(tt.custom, tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestGetSliceConfig tests []string slice extraction from custom config
func TestGetSliceConfig(t *testing.T) {
	tests := []struct {
		name         string
		custom       map[string]interface{}
		key          string
		defaultValue []string
		expected     []string
	}{
		{
			name:         "existing []string value",
			custom:       map[string]interface{}{"key": []string{"a", "b", "c"}},
			key:          "key",
			defaultValue: []string{"default"},
			expected:     []string{"a", "b", "c"},
		},
		{
			name:         "existing []interface{} value",
			custom:       map[string]interface{}{"key": []interface{}{"a", "b", "c"}},
			key:          "key",
			defaultValue: []string{"default"},
			expected:     []string{"a", "b", "c"},
		},
		{
			name:         "missing key",
			custom:       map[string]interface{}{"other": []string{"a"}},
			key:          "key",
			defaultValue: []string{"default"},
			expected:     []string{"default"},
		},
		{
			name:         "nil map",
			custom:       nil,
			key:          "key",
			defaultValue: []string{"default"},
			expected:     []string{"default"},
		},
		{
			name:         "[]interface{} with non-string",
			custom:       map[string]interface{}{"key": []interface{}{"a", 123, "c"}},
			key:          "key",
			defaultValue: []string{"default"},
			expected:     []string{"default"},
		},
		{
			name:         "empty slice",
			custom:       map[string]interface{}{"key": []string{}},
			key:          "key",
			defaultValue: []string{"default"},
			expected:     []string{}, // Empty slice is valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetSliceConfig(tt.custom, tt.key, tt.defaultValue)

			if len(result) != len(tt.expected) {
				t.Fatalf("expected length %d, got %d", len(tt.expected), len(result))
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("at index %d: expected %q, got %q", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

// TestGetFloat64Config tests float64 extraction from custom config
func TestGetFloat64Config(t *testing.T) {
	tests := []struct {
		name         string
		custom       map[string]interface{}
		key          string
		defaultValue float64
		expected     float64
	}{
		{
			name:         "existing float64 value",
			custom:       map[string]interface{}{"key": 3.14},
			key:          "key",
			defaultValue: 1.0,
			expected:     3.14,
		},
		{
			name:         "existing int value (converted)",
			custom:       map[string]interface{}{"key": 42},
			key:          "key",
			defaultValue: 1.0,
			expected:     42.0,
		},
		{
			name:         "missing key",
			custom:       map[string]interface{}{"other": 3.14},
			key:          "key",
			defaultValue: 1.0,
			expected:     1.0,
		},
		{
			name:         "nil map",
			custom:       nil,
			key:          "key",
			defaultValue: 1.0,
			expected:     1.0,
		},
		{
			name:         "wrong type (string)",
			custom:       map[string]interface{}{"key": "3.14"},
			key:          "key",
			defaultValue: 1.0,
			expected:     1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFloat64Config(tt.custom, tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}

// TestValidateRequiredString tests required string validation
func TestValidateRequiredString(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		value     string
		expectErr bool
	}{
		{"valid string", "field", "value", false},
		{"empty string", "field", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequiredString(tt.fieldName, tt.value)
			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidatePositiveInt tests positive int validation
func TestValidatePositiveInt(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		value     int
		expectErr bool
	}{
		{"positive value", "field", 5, false},
		{"zero", "field", 0, true},
		{"negative", "field", -5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePositiveInt(tt.fieldName, tt.value)
			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidateNonNegativeInt tests non-negative int validation
func TestValidateNonNegativeInt(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		value     int
		expectErr bool
	}{
		{"positive value", "field", 5, false},
		{"zero", "field", 0, false},
		{"negative", "field", -5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNonNegativeInt(tt.fieldName, tt.value)
			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidateIntRange tests int range validation
func TestValidateIntRange(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		value     int
		min       int
		max       int
		expectErr bool
	}{
		{"in range", "field", 5, 1, 10, false},
		{"at min", "field", 1, 1, 10, false},
		{"at max", "field", 10, 1, 10, false},
		{"below min", "field", 0, 1, 10, true},
		{"above max", "field", 11, 1, 10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIntRange(tt.fieldName, tt.value, tt.min, tt.max)
			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidatePositiveDuration tests positive duration validation
func TestValidatePositiveDuration(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		value     time.Duration
		expectErr bool
	}{
		{"positive duration", "field", 5 * time.Second, false},
		{"zero duration", "field", 0, true},
		{"negative duration", "field", -5 * time.Second, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePositiveDuration(tt.fieldName, tt.value)
			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidateNonEmptySlice tests non-empty slice validation
func TestValidateNonEmptySlice(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		value     []string
		expectErr bool
	}{
		{"non-empty slice", "field", []string{"a", "b"}, false},
		{"empty slice", "field", []string{}, true},
		{"nil slice", "field", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNonEmptySlice(tt.fieldName, tt.value)
			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidateEnum tests enum validation
func TestValidateEnum(t *testing.T) {
	allowed := []string{"option1", "option2", "option3"}

	tests := []struct {
		name      string
		fieldName string
		value     string
		allowed   []string
		expectErr bool
	}{
		{"valid option", "field", "option1", allowed, false},
		{"another valid option", "field", "option3", allowed, false},
		{"invalid option", "field", "invalid", allowed, true},
		{"empty string", "field", "", allowed, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEnum(tt.fieldName, tt.value, tt.allowed)
			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestRealWorldScenario tests a realistic config extraction scenario
func TestRealWorldScenario(t *testing.T) {
	// Simulates JSON-decoded config (all numbers are float64)
	custom := map[string]interface{}{
		"exec_path":  "subfinder",
		"threads":    float64(10),
		"rate_limit": float64(100),
		"timeout":    "30s",
		"sources":    []interface{}{"crtsh", "dnsdumpster", "hackertarget"},
		"enabled":    true,
	}

	execPath := GetStringConfig(custom, "exec_path", "default")
	threads := GetIntConfig(custom, "threads", 5)
	rateLimit := GetIntConfig(custom, "rate_limit", 50)
	timeout := GetDurationConfig(custom, "timeout", 10*time.Second)
	sources := GetSliceConfig(custom, "sources", []string{"default"})
	enabled := GetBoolConfig(custom, "enabled", false)

	if execPath != "subfinder" {
		t.Errorf("execPath: expected 'subfinder', got '%s'", execPath)
	}
	if threads != 10 {
		t.Errorf("threads: expected 10, got %d", threads)
	}
	if rateLimit != 100 {
		t.Errorf("rateLimit: expected 100, got %d", rateLimit)
	}
	if timeout != 30*time.Second {
		t.Errorf("timeout: expected 30s, got %v", timeout)
	}
	if len(sources) != 3 || sources[0] != "crtsh" {
		t.Errorf("sources: expected [crtsh dnsdumpster hackertarget], got %v", sources)
	}
	if !enabled {
		t.Error("enabled: expected true, got false")
	}
}
