// internal/testutil/helpers.go
package testutil

import (
	"encoding/json"
	"testing"
	"time"
)

// AssertEqual verifica que dos valores sean iguales.
func AssertEqual(t *testing.T, got, want interface{}, msg string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", msg, got, want)
	}
}

// AssertNotEqual verifica que dos valores sean diferentes.
func AssertNotEqual(t *testing.T, got, want interface{}, msg string) {
	t.Helper()
	if got == want {
		t.Errorf("%s: got %v, should not equal %v", msg, got, want)
	}
}

// AssertNil verifica que un valor sea nil.
func AssertNil(t *testing.T, got interface{}, msg string) {
	t.Helper()
	if got != nil {
		t.Errorf("%s: expected nil, got %v", msg, got)
	}
}

// AssertNotNil verifica que un valor no sea nil.
func AssertNotNil(t *testing.T, got interface{}, msg string) {
	t.Helper()
	if got == nil {
		t.Errorf("%s: expected non-nil value", msg)
	}
}

// AssertError verifica que un error no sea nil.
func AssertError(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: expected error, got nil", msg)
	}
}

// AssertNoError verifica que no haya error.
func AssertNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Errorf("%s: unexpected error: %v", msg, err)
	}
}

// AssertTrue verifica que una condición sea verdadera.
func AssertTrue(t *testing.T, condition bool, msg string) {
	t.Helper()
	if !condition {
		t.Errorf("%s: expected true, got false", msg)
	}
}

// AssertFalse verifica que una condición sea falsa.
func AssertFalse(t *testing.T, condition bool, msg string) {
	t.Helper()
	if condition {
		t.Errorf("%s: expected false, got true", msg)
	}
}

// AssertContains verifica que un slice contenga un elemento O que un string contenga un substring.
func AssertContains(t *testing.T, container interface{}, element string, msg string) {
	t.Helper()

	switch v := container.(type) {
	case []string:
		for _, item := range v {
			if item == element {
				return
			}
		}
		t.Errorf("%s: slice %v does not contain %s", msg, v, element)
	case string:
		if !ContainsStr(v, element) {
			t.Errorf("%s: string %q does not contain %q", msg, v, element)
		}
	default:
		t.Errorf("%s: unsupported type for AssertContains", msg)
	}
}

// ContainsStr verifica si un string contiene un substring.
func ContainsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexStr(s, substr) >= 0)
}

// indexStr encuentra la primera ocurrencia de substr en s.
func indexStr(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// AssertLen verifica la longitud de un slice.
func AssertLen(t *testing.T, slice interface{}, want int, msg string) {
	t.Helper()
	var got int
	switch v := slice.(type) {
	case []string:
		got = len(v)
	default:
		// Use reflection for other types
		t.Errorf("%s: use len() directly for this type", msg)
		return
	}
	if got != want {
		t.Errorf("%s: got length %d, want %d", msg, got, want)
	}
}

// Sleep es un helper para tests que necesitan delays (usar con precaución).
func Sleep(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

// NewTestLogger crea un logger para tests que no imprime nada.
type TestLogger struct{}

func (l *TestLogger) Debug(msg string, args ...interface{}) {}
func (l *TestLogger) Info(msg string, args ...interface{})  {}
func (l *TestLogger) Warn(msg string, args ...interface{})  {}
func (l *TestLogger) Err(err error, args ...interface{})    {}
func (l *TestLogger) With(args ...interface{}) interface{}  { return l }

// NewTestLogger retorna un logger silencioso para tests.
func NewTestLogger() *TestLogger {
	return &TestLogger{}
}

// UnmarshalJSON is a helper for unmarshaling JSON in tests.
func UnmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
