package errors

import (
	"fmt"
	"testing"

	"aethonx/internal/testutil"
)

func TestWrap(t *testing.T) {
	t.Run("wraps error with context", func(t *testing.T) {
		baseErr := New("base error")
		wrapped := Wrap(baseErr, "additional context")

		testutil.AssertNotNil(t, wrapped, "wrapped error should not be nil")
		testutil.AssertTrue(t, Is(wrapped, baseErr), "should be able to unwrap to base error")
		testutil.AssertTrue(t, wrapped.Error() == "additional context: base error", "error message should include context")
	})

	t.Run("returns nil when wrapping nil", func(t *testing.T) {
		wrapped := Wrap(nil, "context")
		testutil.AssertTrue(t, wrapped == nil, "wrapping nil should return nil")
	})

	t.Run("multiple wraps preserve chain", func(t *testing.T) {
		baseErr := New("base")
		wrapped1 := Wrap(baseErr, "layer 1")
		wrapped2 := Wrap(wrapped1, "layer 2")

		testutil.AssertTrue(t, Is(wrapped2, baseErr), "should unwrap to base error")
		testutil.AssertTrue(t, wrapped2.Error() == "layer 2: layer 1: base", "should show full chain")
	})
}

func TestWrapf(t *testing.T) {
	t.Run("wraps error with formatted context", func(t *testing.T) {
		baseErr := New("base error")
		wrapped := Wrapf(baseErr, "failed for id=%d", 42)

		testutil.AssertNotNil(t, wrapped, "wrapped error should not be nil")
		testutil.AssertTrue(t, Is(wrapped, baseErr), "should be able to unwrap to base error")
		testutil.AssertTrue(t, wrapped.Error() == "failed for id=42: base error", "error message should include formatted context")
	})

	t.Run("returns nil when wrapping nil", func(t *testing.T) {
		wrapped := Wrapf(nil, "context %s", "test")
		testutil.AssertTrue(t, wrapped == nil, "wrapping nil should return nil")
	})
}

func TestIs(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		target error
		want   bool
	}{
		{
			name:   "matches sentinel error",
			err:    ErrTimeout,
			target: ErrTimeout,
			want:   true,
		},
		{
			name:   "matches wrapped sentinel error",
			err:    Wrap(ErrTimeout, "context"),
			target: ErrTimeout,
			want:   true,
		},
		{
			name:   "does not match different error",
			err:    ErrTimeout,
			target: ErrNotFound,
			want:   false,
		},
		{
			name:   "nil does not match",
			err:    nil,
			target: ErrTimeout,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Is(tt.err, tt.target)
			testutil.AssertEqual(t, got, tt.want, "Is() result should match expected")
		})
	}
}

func TestAs(t *testing.T) {
	t.Run("finds wrapped error type", func(t *testing.T) {
		baseErr := &wrappedError{msg: "test", cause: ErrTimeout}
		wrapped := Wrap(baseErr, "outer")

		var target *wrappedError
		found := As(wrapped, &target)

		testutil.AssertTrue(t, found, "should find wrappedError type")
		testutil.AssertNotNil(t, target, "target should be set")
		// As finds the first matching type in the chain, which is the outer wrapper
		testutil.AssertEqual(t, target.msg, "outer", "should match wrapper error")
	})

	t.Run("returns false for different type", func(t *testing.T) {
		err := New("test")
		var target *wrappedError

		found := As(err, &target)
		testutil.AssertTrue(t, !found, "should not find wrappedError type")
	})
}

func TestUnwrap(t *testing.T) {
	t.Run("unwraps single layer", func(t *testing.T) {
		baseErr := New("base")
		wrapped := Wrap(baseErr, "context")

		unwrapped := Unwrap(wrapped)
		testutil.AssertEqual(t, unwrapped, baseErr, "should unwrap to base error")
	})

	t.Run("returns nil for non-wrapped error", func(t *testing.T) {
		err := New("test")
		unwrapped := Unwrap(err)
		testutil.AssertTrue(t, unwrapped == nil, "should return nil for non-wrapped error")
	})
}

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"ErrTimeout", ErrTimeout, "operation timed out"},
		{"ErrRateLimit", ErrRateLimit, "rate limit exceeded"},
		{"ErrNotFound", ErrNotFound, "resource not found"},
		{"ErrInvalidInput", ErrInvalidInput, "invalid input"},
		{"ErrConnectionFailed", ErrConnectionFailed, "connection failed"},
		{"ErrUnauthorized", ErrUnauthorized, "unauthorized"},
		{"ErrServiceUnavailable", ErrServiceUnavailable, "service unavailable"},
		{"ErrInvalidResponse", ErrInvalidResponse, "invalid response"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.AssertEqual(t, tt.err.Error(), tt.want, "error message should match")
		})
	}
}

func TestIsTimeout(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"direct timeout error", ErrTimeout, true},
		{"wrapped timeout error", Wrap(ErrTimeout, "context"), true},
		{"different error", ErrNotFound, false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTimeout(tt.err)
			testutil.AssertEqual(t, got, tt.want, "IsTimeout result should match")
		})
	}
}

func TestIsRateLimit(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"direct rate limit error", ErrRateLimit, true},
		{"wrapped rate limit error", Wrap(ErrRateLimit, "context"), true},
		{"different error", ErrTimeout, false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRateLimit(tt.err)
			testutil.AssertEqual(t, got, tt.want, "IsRateLimit result should match")
		})
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"direct not found error", ErrNotFound, true},
		{"wrapped not found error", Wrap(ErrNotFound, "context"), true},
		{"different error", ErrTimeout, false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNotFound(tt.err)
			testutil.AssertEqual(t, got, tt.want, "IsNotFound result should match")
		})
	}
}

func TestIsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"direct invalid input error", ErrInvalidInput, true},
		{"wrapped invalid input error", Wrap(ErrInvalidInput, "context"), true},
		{"different error", ErrTimeout, false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsInvalidInput(tt.err)
			testutil.AssertEqual(t, got, tt.want, "IsInvalidInput result should match")
		})
	}
}

func TestIsConnectionFailed(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"direct connection failed error", ErrConnectionFailed, true},
		{"wrapped connection failed error", Wrap(ErrConnectionFailed, "context"), true},
		{"different error", ErrTimeout, false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsConnectionFailed(tt.err)
			testutil.AssertEqual(t, got, tt.want, "IsConnectionFailed result should match")
		})
	}
}

func TestIsUnauthorized(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"direct unauthorized error", ErrUnauthorized, true},
		{"wrapped unauthorized error", Wrap(ErrUnauthorized, "context"), true},
		{"different error", ErrTimeout, false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsUnauthorized(tt.err)
			testutil.AssertEqual(t, got, tt.want, "IsUnauthorized result should match")
		})
	}
}

func TestIsServiceUnavailable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"direct service unavailable error", ErrServiceUnavailable, true},
		{"wrapped service unavailable error", Wrap(ErrServiceUnavailable, "context"), true},
		{"different error", ErrTimeout, false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsServiceUnavailable(tt.err)
			testutil.AssertEqual(t, got, tt.want, "IsServiceUnavailable result should match")
		})
	}
}

func TestIsInvalidResponse(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"direct invalid response error", ErrInvalidResponse, true},
		{"wrapped invalid response error", Wrap(ErrInvalidResponse, "context"), true},
		{"different error", ErrTimeout, false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsInvalidResponse(tt.err)
			testutil.AssertEqual(t, got, tt.want, "IsInvalidResponse result should match")
		})
	}
}

func TestJoin(t *testing.T) {
	t.Run("joins multiple errors", func(t *testing.T) {
		err1 := New("error 1")
		err2 := New("error 2")
		err3 := New("error 3")

		joined := Join(err1, err2, err3)
		testutil.AssertNotNil(t, joined, "joined error should not be nil")

		// All errors should be findable in the joined error
		testutil.AssertTrue(t, Is(joined, err1), "should find first error")
		testutil.AssertTrue(t, Is(joined, err2), "should find second error")
		testutil.AssertTrue(t, Is(joined, err3), "should find third error")
	})

	t.Run("discards nil errors", func(t *testing.T) {
		err1 := New("error 1")
		err2 := New("error 2")

		joined := Join(err1, nil, err2, nil)
		testutil.AssertNotNil(t, joined, "joined error should not be nil")
		testutil.AssertTrue(t, Is(joined, err1), "should find first error")
		testutil.AssertTrue(t, Is(joined, err2), "should find second error")
	})

	t.Run("returns nil when all errors are nil", func(t *testing.T) {
		joined := Join(nil, nil, nil)
		testutil.AssertTrue(t, joined == nil, "should return nil when all errors are nil")
	})
}

func TestErrorf(t *testing.T) {
	err := Errorf("test error: %d", 42)
	testutil.AssertNotNil(t, err, "error should not be nil")
	testutil.AssertEqual(t, err.Error(), "test error: 42", "error message should be formatted")
}

func ExampleWrap() {
	baseErr := New("database connection failed")
	wrapped := Wrap(baseErr, "failed to save user")
	fmt.Println(wrapped.Error())
	// Output: failed to save user: database connection failed
}

func ExampleWrapf() {
	baseErr := New("invalid format")
	wrapped := Wrapf(baseErr, "failed to parse file %s", "config.yaml")
	fmt.Println(wrapped.Error())
	// Output: failed to parse file config.yaml: invalid format
}

func ExampleIs() {
	err := Wrap(ErrTimeout, "operation failed")
	if Is(err, ErrTimeout) {
		fmt.Println("timeout detected")
	}
	// Output: timeout detected
}
