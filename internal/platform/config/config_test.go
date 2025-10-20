// internal/platform/config/config_test.go
package config

import (
	"flag"
	"os"
	"testing"
)

func TestGetenv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		def      string
		envValue string
		expected string
	}{
		{
			name:     "env var exists",
			key:      "TEST_KEY_1",
			def:      "default",
			envValue: "custom",
			expected: "custom",
		},
		{
			name:     "env var missing - uses default",
			key:      "TEST_KEY_MISSING",
			def:      "default",
			envValue: "",
			expected: "default",
		},
		{
			name:     "env var empty string",
			key:      "TEST_KEY_EMPTY",
			def:      "default",
			envValue: "",
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			// Execute
			result := getenv(tt.key, tt.def)

			// Assert
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		// Truthy values
		{"1", true},
		{"t", true},
		{"T", true},
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"y", true},
		{"Y", true},
		{"yes", true},
		{"Yes", true},
		{"YES", true},
		{"on", true},
		{"On", true},
		{"ON", true},
		{" true ", true},
		{" 1 ", true},

		// Falsy values
		{"0", false},
		{"f", false},
		{"false", false},
		{"False", false},
		{"FALSE", false},
		{"n", false},
		{"no", false},
		{"off", false},
		{"", false},
		{"random", false},
		{"garbage", false},
		{" false ", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseBool(tt.input)
			if result != tt.expected {
				t.Errorf("parseBool(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		def      int
		expected int
	}{
		{
			name:     "valid integer",
			input:    "42",
			def:      10,
			expected: 42,
		},
		{
			name:     "negative integer",
			input:    "-5",
			def:      10,
			expected: -5,
		},
		{
			name:     "zero",
			input:    "0",
			def:      10,
			expected: 0,
		},
		{
			name:     "with spaces",
			input:    "  100  ",
			def:      10,
			expected: 100,
		},
		{
			name:     "invalid - returns default",
			input:    "abc",
			def:      10,
			expected: 10,
		},
		{
			name:     "empty - returns default",
			input:    "",
			def:      10,
			expected: 10,
		},
		{
			name:     "float - returns default",
			input:    "3.14",
			def:      10,
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseInt(tt.input, tt.def)
			if result != tt.expected {
				t.Errorf("parseInt(%q, %d) = %d, expected %d", tt.input, tt.def, result, tt.expected)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name     string
		input    Config
		expected Config
	}{
		{
			name: "target normalization - lowercase and trim",
			input: Config{
				Target:    "  EXAMPLE.COM  ",
				Workers:   4,
				TimeoutS:  30,
				OutputDir: "out",
			},
			expected: Config{
				Target:    "example.com",
				Workers:   4,
				TimeoutS:  30,
				OutputDir: "out",
			},
		},
		{
			name: "target normalization - trailing dot",
			input: Config{
				Target:    "example.com.",
				Workers:   4,
				TimeoutS:  30,
				OutputDir: "out",
			},
			expected: Config{
				Target:    "example.com",
				Workers:   4,
				TimeoutS:  30,
				OutputDir: "out",
			},
		},
		{
			name: "workers minimum is 1",
			input: Config{
				Target:    "example.com",
				Workers:   0,
				TimeoutS:  30,
				OutputDir: "out",
			},
			expected: Config{
				Target:    "example.com",
				Workers:   1,
				TimeoutS:  30,
				OutputDir: "out",
			},
		},
		{
			name: "negative workers becomes 1",
			input: Config{
				Target:    "example.com",
				Workers:   -5,
				TimeoutS:  30,
				OutputDir: "out",
			},
			expected: Config{
				Target:    "example.com",
				Workers:   1,
				TimeoutS:  30,
				OutputDir: "out",
			},
		},
		{
			name: "negative timeout becomes 0",
			input: Config{
				Target:    "example.com",
				Workers:   4,
				TimeoutS:  -10,
				OutputDir: "out",
			},
			expected: Config{
				Target:    "example.com",
				Workers:   4,
				TimeoutS:  0,
				OutputDir: "out",
			},
		},
		{
			name: "empty output dir gets default",
			input: Config{
				Target:    "example.com",
				Workers:   4,
				TimeoutS:  30,
				OutputDir: "",
			},
			expected: Config{
				Target:    "example.com",
				Workers:   4,
				TimeoutS:  30,
				OutputDir: "aethonx_out",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.input
			normalize(&cfg)

			if cfg.Target != tt.expected.Target {
				t.Errorf("Target: expected %q, got %q", tt.expected.Target, cfg.Target)
			}
			if cfg.Workers != tt.expected.Workers {
				t.Errorf("Workers: expected %d, got %d", tt.expected.Workers, cfg.Workers)
			}
			if cfg.TimeoutS != tt.expected.TimeoutS {
				t.Errorf("TimeoutS: expected %d, got %d", tt.expected.TimeoutS, cfg.TimeoutS)
			}
			if cfg.OutputDir != tt.expected.OutputDir {
				t.Errorf("OutputDir: expected %q, got %q", tt.expected.OutputDir, cfg.OutputDir)
			}
		})
	}
}

func TestConfig_Timeout(t *testing.T) {
	tests := []struct {
		name     string
		timeoutS int
		expected string // duration string representation
	}{
		{
			name:     "30 seconds",
			timeoutS: 30,
			expected: "30s",
		},
		{
			name:     "zero timeout",
			timeoutS: 0,
			expected: "0s",
		},
		{
			name:     "negative timeout",
			timeoutS: -5,
			expected: "0s",
		},
		{
			name:     "large timeout",
			timeoutS: 3600,
			expected: "1h0m0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{TimeoutS: tt.timeoutS}
			result := cfg.Timeout()

			if result.String() != tt.expected {
				t.Errorf("Timeout(): expected %s, got %s", tt.expected, result.String())
			}
		})
	}
}

func TestLoad_FromEnv(t *testing.T) {
	// Save and restore original flags
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Reset flag.CommandLine to avoid conflicts
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	// Setup environment variables
	os.Setenv("AETHONX_TARGET", "example.com")
	os.Setenv("AETHONX_ACTIVE", "true")
	os.Setenv("AETHONX_WORKERS", "8")
	os.Setenv("AETHONX_TIMEOUT", "60")
	os.Setenv("AETHONX_OUTPUT_DIR", "custom_out")
	os.Setenv("AETHONX_SOURCES_CRTSH_ENABLED", "false")
	os.Setenv("AETHONX_SOURCES_RDAP_ENABLED", "true")
	os.Setenv("AETHONX_OUTPUTS_TABLE_DISABLED", "false")
	os.Setenv("AETHONX_PROXY_URL", "http://proxy.example.com:8080")

	defer func() {
		os.Unsetenv("AETHONX_TARGET")
		os.Unsetenv("AETHONX_ACTIVE")
		os.Unsetenv("AETHONX_WORKERS")
		os.Unsetenv("AETHONX_TIMEOUT")
		os.Unsetenv("AETHONX_OUTPUT_DIR")
		os.Unsetenv("AETHONX_SOURCES_CRTSH_ENABLED")
		os.Unsetenv("AETHONX_SOURCES_RDAP_ENABLED")
		os.Unsetenv("AETHONX_OUTPUTS_TABLE_DISABLED")
		os.Unsetenv("AETHONX_PROXY_URL")
	}()

	// Simulate no CLI arguments (only ENV)
	os.Args = []string{"cmd"}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify values from ENV (normalized)
	if cfg.Target != "example.com" {
		t.Errorf("Target: expected %q, got %q", "example.com", cfg.Target)
	}
	if cfg.Active != true {
		t.Errorf("Active: expected true, got %v", cfg.Active)
	}
	if cfg.Workers != 8 {
		t.Errorf("Workers: expected 8, got %d", cfg.Workers)
	}
	if cfg.TimeoutS != 60 {
		t.Errorf("TimeoutS: expected 60, got %d", cfg.TimeoutS)
	}
	if cfg.OutputDir != "custom_out" {
		t.Errorf("OutputDir: expected %q, got %q", "custom_out", cfg.OutputDir)
	}
	if crtshCfg, exists := cfg.Sources["crtsh"]; !exists || crtshCfg.Enabled != false {
		t.Errorf("Sources[\"crtsh\"].Enabled: expected false, got %v", crtshCfg.Enabled)
	}
	if rdapCfg, exists := cfg.Sources["rdap"]; !exists || rdapCfg.Enabled != true {
		t.Errorf("Sources[\"rdap\"].Enabled: expected true, got %v", rdapCfg.Enabled)
	}
	if cfg.Outputs.TableDisabled != false {
		t.Errorf("Outputs.TableDisabled: expected false, got %v", cfg.Outputs.TableDisabled)
	}
	if cfg.ProxyURL != "http://proxy.example.com:8080" {
		t.Errorf("ProxyURL: expected %q, got %q", "http://proxy.example.com:8080", cfg.ProxyURL)
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Save and restore original flags
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Reset flag.CommandLine
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	// Clear any environment variables
	envVars := []string{
		"AETHONX_TARGET",
		"AETHONX_ACTIVE",
		"AETHONX_WORKERS",
		"AETHONX_TIMEOUT",
		"AETHONX_OUTPUT_DIR",
		"AETHONX_SOURCES_CRTSH_ENABLED",
		"AETHONX_SOURCES_RDAP_ENABLED",
		"AETHONX_OUTPUTS_TABLE_DISABLED",
		"AETHONX_PROXY_URL",
	}

	for _, env := range envVars {
		os.Unsetenv(env)
	}

	// Simulate no CLI arguments
	os.Args = []string{"cmd"}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify default values
	if cfg.Target != "" {
		t.Errorf("Target: expected empty, got %q", cfg.Target)
	}
	if cfg.Active != false {
		t.Errorf("Active: expected false, got %v", cfg.Active)
	}
	if cfg.Workers != 4 {
		t.Errorf("Workers: expected 4, got %d", cfg.Workers)
	}
	if cfg.TimeoutS != 30 {
		t.Errorf("TimeoutS: expected 30, got %d", cfg.TimeoutS)
	}
	if cfg.OutputDir != "aethonx_out" {
		t.Errorf("OutputDir: expected %q, got %q", "aethonx_out", cfg.OutputDir)
	}
	if crtshCfg, exists := cfg.Sources["crtsh"]; !exists || crtshCfg.Enabled != true {
		t.Errorf("Sources[\"crtsh\"].Enabled: expected true, got %v", crtshCfg.Enabled)
	}
	if rdapCfg, exists := cfg.Sources["rdap"]; !exists || rdapCfg.Enabled != true {
		t.Errorf("Sources[\"rdap\"].Enabled: expected true, got %v", rdapCfg.Enabled)
	}
	if cfg.Outputs.TableDisabled != false {
		t.Errorf("Outputs.TableDisabled: expected false, got %v", cfg.Outputs.TableDisabled)
	}
	if cfg.ProxyURL != "" {
		t.Errorf("ProxyURL: expected empty, got %q", cfg.ProxyURL)
	}
}
