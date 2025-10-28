package registry

import (
	"fmt"
	"time"
)

// Type-safe configuration extraction helpers for source registry factories.
// These functions eliminate repetitive nil checks and type assertions when extracting
// custom configuration values from the cfg.Custom map.

// GetStringConfig extracts a string value from custom config map with a default fallback.
// Returns the default value if:
//   - custom map is nil
//   - key doesn't exist
//   - value is not a string
//   - value is an empty string
func GetStringConfig(custom map[string]interface{}, key, defaultValue string) string {
	if custom == nil {
		return defaultValue
	}

	if val, ok := custom[key].(string); ok && val != "" {
		return val
	}

	return defaultValue
}

// GetIntConfig extracts an int value from custom config map with a default fallback.
// Handles both int and float64 (JSON numbers are parsed as float64).
// Returns the default value if:
//   - custom map is nil
//   - key doesn't exist
//   - value is neither int nor float64
func GetIntConfig(custom map[string]interface{}, key string, defaultValue int) int {
	if custom == nil {
		return defaultValue
	}

	// Try direct int first
	if val, ok := custom[key].(int); ok {
		return val
	}

	// Try float64 (JSON numbers are typically float64)
	if val, ok := custom[key].(float64); ok {
		return int(val)
	}

	return defaultValue
}

// GetBoolConfig extracts a bool value from custom config map with a default fallback.
// Returns the default value if:
//   - custom map is nil
//   - key doesn't exist
//   - value is not a bool
func GetBoolConfig(custom map[string]interface{}, key string, defaultValue bool) bool {
	if custom == nil {
		return defaultValue
	}

	if val, ok := custom[key].(bool); ok {
		return val
	}

	return defaultValue
}

// GetDurationConfig extracts a time.Duration value from custom config map with a default fallback.
// Accepts duration as:
//   - time.Duration (direct)
//   - int64 (nanoseconds)
//   - float64 (nanoseconds)
//   - string (parsed via time.ParseDuration)
//
// Returns the default value if:
//   - custom map is nil
//   - key doesn't exist
//   - value cannot be converted to duration
func GetDurationConfig(custom map[string]interface{}, key string, defaultValue time.Duration) time.Duration {
	if custom == nil {
		return defaultValue
	}

	val, exists := custom[key]
	if !exists {
		return defaultValue
	}

	// Try time.Duration directly
	if d, ok := val.(time.Duration); ok {
		return d
	}

	// Try int64 (nanoseconds)
	if i, ok := val.(int64); ok {
		return time.Duration(i)
	}

	// Try float64 (nanoseconds, common for JSON)
	if f, ok := val.(float64); ok {
		return time.Duration(f)
	}

	// Try string (e.g., "5s", "10m")
	if s, ok := val.(string); ok {
		if d, err := time.ParseDuration(s); err == nil {
			return d
		}
	}

	return defaultValue
}

// GetSliceConfig extracts a []string slice from custom config map with a default fallback.
// Converts []interface{} to []string if necessary.
// Returns the default value if:
//   - custom map is nil
//   - key doesn't exist
//   - value cannot be converted to []string
func GetSliceConfig(custom map[string]interface{}, key string, defaultValue []string) []string {
	if custom == nil {
		return defaultValue
	}

	val, exists := custom[key]
	if !exists {
		return defaultValue
	}

	// Try []string directly
	if slice, ok := val.([]string); ok {
		return slice
	}

	// Try []interface{} (common for JSON arrays)
	if interfaceSlice, ok := val.([]interface{}); ok {
		stringSlice := make([]string, 0, len(interfaceSlice))
		for _, item := range interfaceSlice {
			if str, ok := item.(string); ok {
				stringSlice = append(stringSlice, str)
			} else {
				// If any item is not a string, return default
				return defaultValue
			}
		}
		return stringSlice
	}

	return defaultValue
}

// GetFloat64Config extracts a float64 value from custom config map with a default fallback.
// Handles both float64 and int (converts int to float64).
// Returns the default value if:
//   - custom map is nil
//   - key doesn't exist
//   - value is neither float64 nor int
func GetFloat64Config(custom map[string]interface{}, key string, defaultValue float64) float64 {
	if custom == nil {
		return defaultValue
	}

	// Try float64 directly
	if val, ok := custom[key].(float64); ok {
		return val
	}

	// Try int (convert to float64)
	if val, ok := custom[key].(int); ok {
		return float64(val)
	}

	return defaultValue
}

// ValidateRequiredString validates that a required string field is not empty.
// Returns an error if the value is empty.
func ValidateRequiredString(fieldName, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required and cannot be empty", fieldName)
	}
	return nil
}

// ValidatePositiveInt validates that an int field is positive (> 0).
// Returns an error if the value is <= 0.
func ValidatePositiveInt(fieldName string, value int) error {
	if value <= 0 {
		return fmt.Errorf("%s must be positive, got %d", fieldName, value)
	}
	return nil
}

// ValidateNonNegativeInt validates that an int field is non-negative (>= 0).
// Returns an error if the value is < 0.
func ValidateNonNegativeInt(fieldName string, value int) error {
	if value < 0 {
		return fmt.Errorf("%s cannot be negative, got %d", fieldName, value)
	}
	return nil
}

// ValidateIntRange validates that an int field is within a specified range [min, max].
// Returns an error if the value is outside the range.
func ValidateIntRange(fieldName string, value, min, max int) error {
	if value < min || value > max {
		return fmt.Errorf("%s must be between %d and %d, got %d", fieldName, min, max, value)
	}
	return nil
}

// ValidatePositiveDuration validates that a duration is positive.
// Returns an error if the value is <= 0.
func ValidatePositiveDuration(fieldName string, value time.Duration) error {
	if value <= 0 {
		return fmt.Errorf("%s must be positive, got %v", fieldName, value)
	}
	return nil
}

// ValidateNonEmptySlice validates that a slice is not empty.
// Returns an error if the slice has length 0.
func ValidateNonEmptySlice(fieldName string, value []string) error {
	if len(value) == 0 {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}
	return nil
}

// ValidateEnum validates that a string value is one of the allowed options.
// Returns an error if the value is not in the allowed list.
func ValidateEnum(fieldName, value string, allowed []string) error {
	for _, option := range allowed {
		if value == option {
			return nil
		}
	}
	return fmt.Errorf("%s must be one of %v, got %s", fieldName, allowed, value)
}
