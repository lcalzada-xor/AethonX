package httpx

import (
	"encoding/json"
	"testing"
)

func TestFlexibleString_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "null value",
			input:    `null`,
			expected: "",
			wantErr:  false,
		},
		{
			name:     "string value",
			input:    `"example.com"`,
			expected: "example.com",
			wantErr:  false,
		},
		{
			name:     "array with single element",
			input:    `["example.com"]`,
			expected: "example.com",
			wantErr:  false,
		},
		{
			name:     "array with multiple elements",
			input:    `["cdn1.example.com", "cdn2.example.com"]`,
			expected: "cdn1.example.com, cdn2.example.com",
			wantErr:  false,
		},
		{
			name:     "empty array",
			input:    `[]`,
			expected: "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fs FlexibleString
			err := json.Unmarshal([]byte(tt.input), &fs)

			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if fs.String() != tt.expected {
				t.Errorf("String() = %v, want %v", fs.String(), tt.expected)
			}
		})
	}
}

func TestFlexibleString_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "null value",
			input:    `null`,
			expected: true,
		},
		{
			name:     "empty string",
			input:    `""`,
			expected: true,
		},
		{
			name:     "empty array",
			input:    `[]`,
			expected: true,
		},
		{
			name:     "non-empty string",
			input:    `"example.com"`,
			expected: false,
		},
		{
			name:     "non-empty array",
			input:    `["example.com"]`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fs FlexibleString
			_ = json.Unmarshal([]byte(tt.input), &fs)

			if fs.IsEmpty() != tt.expected {
				t.Errorf("IsEmpty() = %v, want %v", fs.IsEmpty(), tt.expected)
			}
		})
	}
}

func TestFlexibleString_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{
			name:     "empty string",
			value:    "",
			expected: `""`,
		},
		{
			name:     "simple string",
			value:    "example.com",
			expected: `"example.com"`,
		},
		{
			name:     "complex string with spaces",
			value:    "cdn1.example.com, cdn2.example.com",
			expected: `"cdn1.example.com, cdn2.example.com"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := FlexibleString{value: tt.value}
			data, err := json.Marshal(fs)

			if err != nil {
				t.Errorf("MarshalJSON() error = %v", err)
				return
			}

			if string(data) != tt.expected {
				t.Errorf("MarshalJSON() = %v, want %v", string(data), tt.expected)
			}
		})
	}
}

func TestFlexibleBool_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
		wantErr  bool
	}{
		{
			name:     "null value",
			input:    `null`,
			expected: false,
			wantErr:  false,
		},
		{
			name:     "bool true",
			input:    `true`,
			expected: true,
			wantErr:  false,
		},
		{
			name:     "bool false",
			input:    `false`,
			expected: false,
			wantErr:  false,
		},
		{
			name:     "string true",
			input:    `"true"`,
			expected: true,
			wantErr:  false,
		},
		{
			name:     "string false",
			input:    `"false"`,
			expected: false,
			wantErr:  false,
		},
		{
			name:     "string yes",
			input:    `"yes"`,
			expected: true,
			wantErr:  false,
		},
		{
			name:     "string no",
			input:    `"no"`,
			expected: false,
			wantErr:  false,
		},
		{
			name:     "string 1",
			input:    `"1"`,
			expected: true,
			wantErr:  false,
		},
		{
			name:     "string 0",
			input:    `"0"`,
			expected: false,
			wantErr:  false,
		},
		{
			name:     "number 1",
			input:    `1`,
			expected: true,
			wantErr:  false,
		},
		{
			name:     "number 0",
			input:    `0`,
			expected: false,
			wantErr:  false,
		},
		{
			name:     "number non-zero",
			input:    `42`,
			expected: true,
			wantErr:  false,
		},
		{
			name:     "empty string",
			input:    `""`,
			expected: false,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fb FlexibleBool
			err := json.Unmarshal([]byte(tt.input), &fb)

			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if fb.Bool() != tt.expected {
				t.Errorf("Bool() = %v, want %v", fb.Bool(), tt.expected)
			}
		})
	}
}

func TestFlexibleBool_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		value    bool
		expected string
	}{
		{
			name:     "true",
			value:    true,
			expected: `true`,
		},
		{
			name:     "false",
			value:    false,
			expected: `false`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := FlexibleBool{value: tt.value}
			data, err := json.Marshal(fb)

			if err != nil {
				t.Errorf("MarshalJSON() error = %v", err)
				return
			}

			if string(data) != tt.expected {
				t.Errorf("MarshalJSON() = %v, want %v", string(data), tt.expected)
			}
		})
	}
}

func TestFlexibleBool_String(t *testing.T) {
	tests := []struct {
		name     string
		value    bool
		expected string
	}{
		{
			name:     "true",
			value:    true,
			expected: "true",
		},
		{
			name:     "false",
			value:    false,
			expected: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := FlexibleBool{value: tt.value}

			if fb.String() != tt.expected {
				t.Errorf("String() = %v, want %v", fb.String(), tt.expected)
			}
		})
	}
}

// Integration test: unmarshal HTTPXResponse with flexible fields
func TestHTTPXResponse_FlexibleFields(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		expectCDN   bool
		expectCNAME string
		wantErr     bool
	}{
		{
			name: "cdn as bool and cname as array",
			json: `{
				"url": "https://example.com",
				"cdn": true,
				"cdn_name": "cloudflare",
				"cname": ["example.cdn.net"],
				"status_code": 200
			}`,
			expectCDN:   true,
			expectCNAME: "example.cdn.net",
			wantErr:     false,
		},
		{
			name: "cdn as string and cname as string",
			json: `{
				"url": "https://example.com",
				"cdn": "true",
				"cdn_name": "cloudflare",
				"cname": "example.cdn.net",
				"status_code": 200
			}`,
			expectCDN:   true,
			expectCNAME: "example.cdn.net",
			wantErr:     false,
		},
		{
			name: "cdn as bool false",
			json: `{
				"url": "https://example.com",
				"cdn": false,
				"status_code": 200
			}`,
			expectCDN:   false,
			expectCNAME: "",
			wantErr:     false,
		},
		{
			name: "cname as multiple values",
			json: `{
				"url": "https://example.com",
				"cname": ["cdn1.example.net", "cdn2.example.net"],
				"status_code": 200
			}`,
			expectCDN:   false,
			expectCNAME: "cdn1.example.net, cdn2.example.net",
			wantErr:     false,
		},
		{
			name: "null values",
			json: `{
				"url": "https://example.com",
				"cdn": null,
				"cname": null,
				"status_code": 200
			}`,
			expectCDN:   false,
			expectCNAME: "",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp HTTPXResponse
			err := json.Unmarshal([]byte(tt.json), &resp)

			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if resp.CDN.Bool() != tt.expectCDN {
				t.Errorf("CDN.Bool() = %v, want %v", resp.CDN.Bool(), tt.expectCDN)
			}

			if resp.CNAME.String() != tt.expectCNAME {
				t.Errorf("CNAME.String() = %v, want %v", resp.CNAME.String(), tt.expectCNAME)
			}
		})
	}
}
