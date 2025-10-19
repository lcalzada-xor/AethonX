// Package errors provides error types and utilities for AethonX.
// It extends the standard errors package with additional context and wrapping capabilities.
package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for common failure scenarios
var (
	// ErrTimeout indicates an operation exceeded its time limit
	ErrTimeout = errors.New("operation timed out")

	// ErrRateLimit indicates a rate limit was exceeded
	ErrRateLimit = errors.New("rate limit exceeded")

	// ErrNotFound indicates a requested resource was not found
	ErrNotFound = errors.New("resource not found")

	// ErrInvalidInput indicates invalid input was provided
	ErrInvalidInput = errors.New("invalid input")

	// ErrConnectionFailed indicates a connection could not be established
	ErrConnectionFailed = errors.New("connection failed")

	// ErrUnauthorized indicates authentication or authorization failed
	ErrUnauthorized = errors.New("unauthorized")

	// ErrServiceUnavailable indicates a service is temporarily unavailable
	ErrServiceUnavailable = errors.New("service unavailable")

	// ErrInvalidResponse indicates a response could not be parsed or was malformed
	ErrInvalidResponse = errors.New("invalid response")
)

// wrappedError wraps an error with additional context
type wrappedError struct {
	msg   string
	cause error
}

// Error implements the error interface
func (e *wrappedError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.msg, e.cause)
	}
	return e.msg
}

// Unwrap returns the underlying error
func (e *wrappedError) Unwrap() error {
	return e.cause
}

// Wrap wraps an error with additional context message.
// If err is nil, Wrap returns nil.
//
// Example:
//   err := someOperation()
//   if err != nil {
//       return errors.Wrap(err, "failed to perform operation")
//   }
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return &wrappedError{
		msg:   msg,
		cause: err,
	}
}

// Wrapf wraps an error with a formatted context message.
// If err is nil, Wrapf returns nil.
//
// Example:
//   err := someOperation(id)
//   if err != nil {
//       return errors.Wrapf(err, "failed to process item %d", id)
//   }
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return &wrappedError{
		msg:   fmt.Sprintf(format, args...),
		cause: err,
	}
}

// Is reports whether any error in err's chain matches target.
// This is a convenience wrapper around errors.Is from the standard library.
//
// Example:
//   if errors.Is(err, errors.ErrTimeout) {
//       // Handle timeout
//   }
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's chain that matches target type.
// This is a convenience wrapper around errors.As from the standard library.
//
// Example:
//   var netErr *net.Error
//   if errors.As(err, &netErr) {
//       // Handle network error
//   }
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// Unwrap returns the result of calling the Unwrap method on err.
// This is a convenience wrapper around errors.Unwrap from the standard library.
func Unwrap(err error) error {
	return errors.Unwrap(err)
}

// New creates a new error with the given message.
// This is a convenience wrapper around errors.New from the standard library.
func New(msg string) error {
	return errors.New(msg)
}

// Errorf formats according to a format specifier and returns the string as a value that satisfies error.
// This is a convenience wrapper around fmt.Errorf from the standard library.
func Errorf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

// Join returns an error that wraps the given errors.
// Any nil error values are discarded.
// This is a convenience wrapper around errors.Join from the standard library.
func Join(errs ...error) error {
	return errors.Join(errs...)
}

// IsTimeout reports whether the error is a timeout error
func IsTimeout(err error) bool {
	return Is(err, ErrTimeout)
}

// IsRateLimit reports whether the error is a rate limit error
func IsRateLimit(err error) bool {
	return Is(err, ErrRateLimit)
}

// IsNotFound reports whether the error is a not found error
func IsNotFound(err error) bool {
	return Is(err, ErrNotFound)
}

// IsInvalidInput reports whether the error is an invalid input error
func IsInvalidInput(err error) bool {
	return Is(err, ErrInvalidInput)
}

// IsConnectionFailed reports whether the error is a connection failed error
func IsConnectionFailed(err error) bool {
	return Is(err, ErrConnectionFailed)
}

// IsUnauthorized reports whether the error is an unauthorized error
func IsUnauthorized(err error) bool {
	return Is(err, ErrUnauthorized)
}

// IsServiceUnavailable reports whether the error is a service unavailable error
func IsServiceUnavailable(err error) bool {
	return Is(err, ErrServiceUnavailable)
}

// IsInvalidResponse reports whether the error is an invalid response error
func IsInvalidResponse(err error) bool {
	return Is(err, ErrInvalidResponse)
}
