package httpx

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// FlexibleString handles JSON fields that can be either string, []string, or null.
// This is needed because httpx can return different types for the same field.
type FlexibleString struct {
	value string
}

// UnmarshalJSON implements custom unmarshaling to handle multiple types.
func (fs *FlexibleString) UnmarshalJSON(data []byte) error {
	// Handle null
	if string(data) == "null" {
		fs.value = ""
		return nil
	}

	// Try to unmarshal as string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		fs.value = s
		return nil
	}

	// Try to unmarshal as []string
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		fs.value = strings.Join(arr, ", ")
		return nil
	}

	return fmt.Errorf("FlexibleString: cannot unmarshal %s", string(data))
}

// MarshalJSON implements JSON marshaling.
func (fs FlexibleString) MarshalJSON() ([]byte, error) {
	return json.Marshal(fs.value)
}

// String returns the string value.
func (fs FlexibleString) String() string {
	return fs.value
}

// IsEmpty returns true if the value is empty.
func (fs FlexibleString) IsEmpty() bool {
	return fs.value == ""
}

// FlexibleBool handles JSON fields that can be either bool, string, or null.
// This is needed because httpx can return different types for the same field.
type FlexibleBool struct {
	value bool
}

// UnmarshalJSON implements custom unmarshaling to handle multiple types.
func (fb *FlexibleBool) UnmarshalJSON(data []byte) error {
	// Handle null
	if string(data) == "null" {
		fb.value = false
		return nil
	}

	// Try to unmarshal as bool
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		fb.value = b
		return nil
	}

	// Try to unmarshal as string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		// Parse string as bool
		s = strings.ToLower(strings.TrimSpace(s))
		switch s {
		case "true", "1", "yes":
			fb.value = true
		case "false", "0", "no", "":
			fb.value = false
		default:
			return fmt.Errorf("FlexibleBool: cannot parse string '%s' as bool", s)
		}
		return nil
	}

	// Try to unmarshal as number (0 or 1)
	var n int
	if err := json.Unmarshal(data, &n); err == nil {
		fb.value = n != 0
		return nil
	}

	return fmt.Errorf("FlexibleBool: cannot unmarshal %s", string(data))
}

// MarshalJSON implements JSON marshaling.
func (fb FlexibleBool) MarshalJSON() ([]byte, error) {
	return json.Marshal(fb.value)
}

// Bool returns the bool value.
func (fb FlexibleBool) Bool() bool {
	return fb.value
}

// String returns the string representation of the bool.
func (fb FlexibleBool) String() string {
	return strconv.FormatBool(fb.value)
}
