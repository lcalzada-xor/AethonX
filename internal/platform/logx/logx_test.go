// internal/platform/logx/logx_test.go
package logx

import (
	"bytes"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	logger := New()
	if logger == nil {
		t.Fatal("New() should return a logger, got nil")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		// Debug
		{"debug", LevelDebug},
		{"Debug", LevelDebug},
		{"DEBUG", LevelDebug},
		{"dbg", LevelDebug},
		{"  debug  ", LevelDebug},

		// Info
		{"info", LevelInfo},
		{"Info", LevelInfo},
		{"INFO", LevelInfo},
		{"inf", LevelInfo},
		{"", LevelInfo}, // empty defaults to Info
		{"  info  ", LevelInfo},

		// Warn
		{"warn", LevelWarn},
		{"Warn", LevelWarn},
		{"WARN", LevelWarn},
		{"warning", LevelWarn},
		{"Warning", LevelWarn},
		{"  warn  ", LevelWarn},

		// Error
		{"err", LevelError},
		{"Err", LevelError},
		{"ERR", LevelError},
		{"error", LevelError},
		{"Error", LevelError},
		{"ERROR", LevelError},
		{"  error  ", LevelError},

		// Invalid defaults to Info
		{"invalid", LevelInfo},
		{"random", LevelInfo},
		{"garbage", LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("parseLevel(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestKVPairs(t *testing.T) {
	tests := []struct {
		name     string
		input    []any
		expected []string
	}{
		{
			name:     "empty input",
			input:    []any{},
			expected: []string{},
		},
		{
			name:     "single pair",
			input:    []any{"key", "value"},
			expected: []string{"key=value"},
		},
		{
			name:     "multiple pairs",
			input:    []any{"key1", "value1", "key2", "value2"},
			expected: []string{"key1=value1", "key2=value2"},
		},
		{
			name:     "odd number of elements",
			input:    []any{"key1", "value1", "key2"},
			expected: []string{"key1=value1", "key2=(missing)"},
		},
		{
			name:     "numeric values",
			input:    []any{"count", 42, "enabled", true},
			expected: []string{"count=42", "enabled=true"},
		},
		{
			name:     "mixed types",
			input:    []any{"string", "value", "int", 123, "bool", false},
			expected: []string{"string=value", "int=123", "bool=false"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := kvPairs(tt.input...)

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d pairs, got %d", len(tt.expected), len(result))
			}

			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Errorf("pair %d: expected %q, got %q", i, exp, result[i])
				}
			}
		})
	}
}

func TestLogger_With(t *testing.T) {
	var buf bytes.Buffer
	logger := &simpleLogger{
		lvl:   LevelDebug,
		scope: []string{},
		lg:    log.New(&buf, "", 0),
	}

	// Create scoped logger
	scoped := logger.With("service", "test", "version", "1.0")

	// Log with scoped logger
	scoped.Info("test message")

	output := buf.String()

	// Should contain scoped fields
	if !strings.Contains(output, "service=test") {
		t.Errorf("output should contain 'service=test', got: %s", output)
	}
	if !strings.Contains(output, "version=1.0") {
		t.Errorf("output should contain 'version=1.0', got: %s", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("output should contain 'test message', got: %s", output)
	}
}

func TestLogger_With_Immutable(t *testing.T) {
	var buf1, buf2 bytes.Buffer

	logger := &simpleLogger{
		lvl:   LevelDebug,
		scope: []string{},
		lg:    log.New(&buf1, "", 0),
	}

	// Create scoped logger
	scoped := logger.With("service", "test")

	// Original logger should not have scope
	if len(logger.scope) != 0 {
		t.Errorf("original logger should not have scope, got: %v", logger.scope)
	}

	// Scoped logger should have scope
	scopedImpl := scoped.(*simpleLogger)
	if len(scopedImpl.scope) != 1 {
		t.Errorf("scoped logger should have 1 scope pair, got: %d", len(scopedImpl.scope))
	}

	// Change output for verification
	scopedImpl.lg = log.New(&buf2, "", 0)

	logger.Info("original")
	scoped.Info("scoped")

	if strings.Contains(buf1.String(), "service=test") {
		t.Errorf("original logger output should not contain scope: %s", buf1.String())
	}
	if !strings.Contains(buf2.String(), "service=test") {
		t.Errorf("scoped logger output should contain scope: %s", buf2.String())
	}
}

func TestLogger_Debug(t *testing.T) {
	var buf bytes.Buffer
	logger := &simpleLogger{
		lvl:   LevelDebug,
		scope: []string{},
		lg:    log.New(&buf, "", 0),
	}

	logger.Debug("debug message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "DBG") {
		t.Errorf("output should contain 'DBG', got: %s", output)
	}
	if !strings.Contains(output, "debug message") {
		t.Errorf("output should contain message, got: %s", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("output should contain kv pair, got: %s", output)
	}
}

func TestLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	logger := &simpleLogger{
		lvl:   LevelInfo,
		scope: []string{},
		lg:    log.New(&buf, "", 0),
	}

	logger.Info("info message", "count", 42)

	output := buf.String()
	if !strings.Contains(output, "INF") {
		t.Errorf("output should contain 'INF', got: %s", output)
	}
	if !strings.Contains(output, "info message") {
		t.Errorf("output should contain message, got: %s", output)
	}
	if !strings.Contains(output, "count=42") {
		t.Errorf("output should contain kv pair, got: %s", output)
	}
}

func TestLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	logger := &simpleLogger{
		lvl:   LevelWarn,
		scope: []string{},
		lg:    log.New(&buf, "", 0),
	}

	logger.Warn("warning message", "enabled", true)

	output := buf.String()
	if !strings.Contains(output, "WRN") {
		t.Errorf("output should contain 'WRN', got: %s", output)
	}
	if !strings.Contains(output, "warning message") {
		t.Errorf("output should contain message, got: %s", output)
	}
	if !strings.Contains(output, "enabled=true") {
		t.Errorf("output should contain kv pair, got: %s", output)
	}
}

func TestLogger_Err(t *testing.T) {
	var buf bytes.Buffer
	logger := &simpleLogger{
		lvl:   LevelError,
		scope: []string{},
		lg:    log.New(&buf, "", 0),
	}

	testErr := errors.New("test error")
	logger.Err(testErr, "source", "database")

	output := buf.String()
	if !strings.Contains(output, "ERR") {
		t.Errorf("output should contain 'ERR', got: %s", output)
	}
	if !strings.Contains(output, "error=test error") {
		t.Errorf("output should contain error, got: %s", output)
	}
	if !strings.Contains(output, "source=database") {
		t.Errorf("output should contain kv pair, got: %s", output)
	}
}

func TestLogger_Err_Nil(t *testing.T) {
	var buf bytes.Buffer
	logger := &simpleLogger{
		lvl:   LevelError,
		scope: []string{},
		lg:    log.New(&buf, "", 0),
	}

	logger.Err(nil, "source", "database")

	output := buf.String()
	if output != "" {
		t.Errorf("nil error should not log anything, got: %s", output)
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	tests := []struct {
		name         string
		logLevel     Level
		shouldAppear map[string]bool
	}{
		{
			name:     "debug level - all appear",
			logLevel: LevelDebug,
			shouldAppear: map[string]bool{
				"debug": true,
				"info":  true,
				"warn":  true,
				"error": true,
			},
		},
		{
			name:     "info level - no debug",
			logLevel: LevelInfo,
			shouldAppear: map[string]bool{
				"debug": false,
				"info":  true,
				"warn":  true,
				"error": true,
			},
		},
		{
			name:     "warn level - only warn and error",
			logLevel: LevelWarn,
			shouldAppear: map[string]bool{
				"debug": false,
				"info":  false,
				"warn":  true,
				"error": true,
			},
		},
		{
			name:     "error level - only error",
			logLevel: LevelError,
			shouldAppear: map[string]bool{
				"debug": false,
				"info":  false,
				"warn":  false,
				"error": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := &simpleLogger{
				lvl:   tt.logLevel,
				scope: []string{},
				lg:    log.New(&buf, "", 0),
			}

			logger.Debug("debug")
			logger.Info("info")
			logger.Warn("warn")
			logger.Err(errors.New("error"))

			output := buf.String()

			for level, shouldAppear := range tt.shouldAppear {
				var tag string
				switch level {
				case "debug":
					tag = "DBG"
				case "info":
					tag = "INF"
				case "warn":
					tag = "WRN"
				case "error":
					tag = "ERR"
				}

				contains := strings.Contains(output, tag)
				if contains != shouldAppear {
					if shouldAppear {
						t.Errorf("output should contain %s at level %v, got: %s", tag, tt.logLevel, output)
					} else {
						t.Errorf("output should NOT contain %s at level %v, got: %s", tag, tt.logLevel, output)
					}
				}
			}
		})
	}
}

func TestLogger_ThreadSafety(t *testing.T) {
	var buf bytes.Buffer
	logger := &simpleLogger{
		lvl:   LevelInfo,
		scope: []string{},
		lg:    log.New(&buf, "", 0),
	}

	var wg sync.WaitGroup
	iterations := 100

	// Multiple goroutines logging concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				logger.Info("concurrent log", "id", id, "iteration", j)
			}
		}(i)
	}

	wg.Wait()

	// Should have logged 10 * 100 = 1000 lines
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	expectedLines := 10 * iterations

	if len(lines) != expectedLines {
		t.Errorf("expected %d log lines, got %d", expectedLines, len(lines))
	}
}

func TestNew_WithEnv(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		logLevel Level
	}{
		{
			name:     "debug level from env",
			envValue: "debug",
			logLevel: LevelDebug,
		},
		{
			name:     "info level from env",
			envValue: "info",
			logLevel: LevelInfo,
		},
		{
			name:     "warn level from env",
			envValue: "warn",
			logLevel: LevelWarn,
		},
		{
			name:     "error level from env",
			envValue: "error",
			logLevel: LevelError,
		},
		{
			name:     "empty defaults to info",
			envValue: "",
			logLevel: LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("AETHONX_LOG_LEVEL", tt.envValue)
			defer os.Unsetenv("AETHONX_LOG_LEVEL")

			logger := New()
			impl := logger.(*simpleLogger)

			if impl.lvl != tt.logLevel {
				t.Errorf("expected log level %v, got %v", tt.logLevel, impl.lvl)
			}
		})
	}
}

func TestLogger_EmptyMessage(t *testing.T) {
	var buf bytes.Buffer
	logger := &simpleLogger{
		lvl:   LevelError,
		scope: []string{},
		lg:    log.New(&buf, "", 0),
	}

	// Err with no message, only fields
	logger.Err(errors.New("test error"), "source", "test")

	output := buf.String()

	// Should not have double spaces
	if strings.Contains(output, "  ") {
		t.Errorf("output should not contain double spaces: %s", output)
	}

	// Should contain error field
	if !strings.Contains(output, "error=test error") {
		t.Errorf("output should contain error field: %s", output)
	}
}
