package subfinder

import (
	"testing"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
)

func TestSubfinder_Name(t *testing.T) {
	logger := logx.New()
	src := New(logger)

	if src.Name() != "subfinder" {
		t.Errorf("expected name 'subfinder', got %s", src.Name())
	}
}

func TestSubfinder_Mode(t *testing.T) {
	logger := logx.New()
	src := New(logger)

	if src.Mode() != domain.SourceModePassive {
		t.Errorf("expected mode passive, got %s", src.Mode())
	}
}

func TestSubfinder_Type(t *testing.T) {
	logger := logx.New()
	src := New(logger)

	if src.Type() != domain.SourceTypeCLI {
		t.Errorf("expected type CLI, got %s", src.Type())
	}
}

func TestSubfinder_Validate(t *testing.T) {
	logger := logx.New()

	tests := []struct {
		name      string
		source    *SubfinderSource
		expectErr bool
	}{
		{
			name:      "valid configuration",
			source:    NewWithConfig(logger, "subfinder", 60*time.Second, 10, 0, []string{"anubis", "hackertarget"}),
			expectErr: false,
		},
		{
			name:      "empty exec path",
			source:    NewWithConfig(logger, "", 60*time.Second, 10, 0, []string{"anubis"}),
			expectErr: true,
		},
		{
			name:      "invalid timeout",
			source:    NewWithConfig(logger, "subfinder", 0, 10, 0, []string{"anubis"}),
			expectErr: true,
		},
		{
			name:      "invalid threads - too low",
			source:    NewWithConfig(logger, "subfinder", 60*time.Second, 0, 0, []string{"anubis"}),
			expectErr: true,
		},
		{
			name:      "invalid threads - too high",
			source:    NewWithConfig(logger, "subfinder", 60*time.Second, 1001, 0, []string{"anubis"}),
			expectErr: true,
		},
		{
			name:      "negative rate limit",
			source:    NewWithConfig(logger, "subfinder", 60*time.Second, 10, -1, []string{"anubis"}),
			expectErr: true,
		},
		{
			name:      "no sources configured",
			source:    NewWithConfig(logger, "subfinder", 60*time.Second, 10, 0, []string{}),
			expectErr: true,
		},
		{
			name:      "specific sources configured",
			source:    NewWithConfig(logger, "subfinder", 60*time.Second, 10, 0, []string{"crtsh", "hackertarget"}),
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.source.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("expected error: %v, got error: %v", tt.expectErr, err)
			}
		})
	}
}

func TestSubfinder_Close(t *testing.T) {
	logger := logx.New()
	src := New(logger)

	// Close should not error even without a running process
	err := src.Close()
	if err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}
}

func TestSubfinder_ProgressChannel(t *testing.T) {
	logger := logx.New()
	src := New(logger)

	ch := src.ProgressChannel()
	if ch == nil {
		t.Error("expected non-nil progress channel")
	}
}

func TestSubfinder_buildCommand(t *testing.T) {
	logger := logx.New()
	target := domain.Target{Root: "example.com", Mode: domain.ScanModePassive}

	tests := []struct {
		name       string
		source     *SubfinderSource
		expectArgs []string
	}{
		{
			name:       "default configuration with free sources",
			source:     NewWithConfig(logger, "subfinder", 60*time.Second, 10, 0, []string{"anubis", "hackertarget"}),
			expectArgs: []string{"-d", "example.com", "-oJ", "-silent", "-nc", "-s", "anubis,hackertarget", "-t", "10", "-timeout", "60"},
		},
		{
			name:       "specific sources",
			source:     NewWithConfig(logger, "subfinder", 60*time.Second, 10, 0, []string{"crtsh", "hackertarget"}),
			expectArgs: []string{"-d", "example.com", "-oJ", "-silent", "-nc", "-s", "crtsh,hackertarget", "-t", "10", "-timeout", "60"},
		},
		{
			name:       "with rate limit",
			source:     NewWithConfig(logger, "subfinder", 60*time.Second, 20, 100, []string{"anubis"}),
			expectArgs: []string{"-d", "example.com", "-oJ", "-silent", "-nc", "-s", "anubis", "-t", "20", "-rl", "100", "-timeout", "60"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := tt.source.buildCommandArgs(target)

			// Check if all expected args are present
			for _, expectedArg := range tt.expectArgs {
				found := false
				for _, arg := range args {
					if arg == expectedArg {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected arg %s not found in command: %v", expectedArg, args)
				}
			}
		})
	}
}

func TestJoinSources(t *testing.T) {
	tests := []struct {
		name     string
		sources  []string
		expected string
	}{
		{
			name:     "single source",
			sources:  []string{"crtsh"},
			expected: "crtsh",
		},
		{
			name:     "multiple sources",
			sources:  []string{"crtsh", "hackertarget", "virustotal"},
			expected: "crtsh,hackertarget,virustotal",
		},
		{
			name:     "empty sources",
			sources:  []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinSources(tt.sources)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
