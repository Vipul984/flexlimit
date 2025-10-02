package flexlimit

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Sentinel errors that can be checked with errors.Is()
var (
	// ErrRateLimitExceeded is returned when a rate limit has been exceeded.
	// This is a sentinel error that can be checked with errors.Is().
	//
	// Example:
	//
	//	if errors.Is(err, flexlimit.ErrRateLimitExceeded) {
	//	    // Handle rate limit
	//	}
	ErrRateLimitExceeded = errors.New("rate limit exceeded")

	// ErrInvalidConfig is returned when limiter configuration is invalid.
	//
	// Example:
	//
	//	limiter := flexlimit.New(0, time.Minute) // Invalid: rate must be > 0
	//	// Returns ErrInvalidConfig
	ErrInvalidConfig = errors.New("invalid configuration")

	// ErrStorageUnavailable is returned when the storage backend is unavailable.
	// This typically happens with distributed storage (Redis) failures.
	//
	// Example:
	//
	//	if errors.Is(err, flexlimit.ErrStorageUnavailable) {
	//	    // Redis is down, using fallback
	//	}
	ErrStorageUnavailable = errors.New("storage backend unavailable")

	// ErrKeyNotFound is returned when attempting to access state for a non-existent key.
	ErrKeyNotFound = errors.New("key not found")

	// ErrContextCanceled is returned when the context is canceled during an operation.
	ErrContextCanceled = errors.New("context canceled")

	// ErrContextDeadlineExceeded is returned when the context deadline is exceeded.
	ErrContextDeadlineExceeded = errors.New("context deadline exceeded")
)

// LimitExceededError is returned when a rate limit is exceeded and provides
// additional context about the limit and when it will reset.
//
// This error can be used to provide detailed feedback to users about their
// rate limit status.
//
// Example:
//
//	var limitErr *flexlimit.LimitExceededError
//	if errors.As(err, &limitErr) {
//	    fmt.Printf("Rate limit: %d requests per %s\n",
//	        limitErr.Limit, limitErr.Window)
//	    fmt.Printf("Retry after: %s\n", limitErr.RetryAfter)
//	}
type LimitExceededError struct {
	// Key is the rate limit key that was exceeded
	Key string

	// Limit is the maximum number of requests allowed
	Limit int

	// Window is the time window for the rate limit
	Window time.Duration

	// Used is the number of requests already consumed
	Used int

	// RetryAfter is the duration until the rate limit resets
	RetryAfter time.Duration

	// ResetAt is the absolute time when the rate limit resets
	ResetAt time.Time
}

// Error implements the error interface.
func (e *LimitExceededError) Error() string {
	return fmt.Sprintf("rate limit exceeded for key %q: %d/%d requests used, retry after %s",
		e.Key, e.Used, e.Limit, e.RetryAfter.Round(time.Second))
}

// Is allows this error to be matched with errors.Is(err, ErrRateLimitExceeded)
func (e *LimitExceededError) Is(target error) bool {
	return target == ErrRateLimitExceeded
}

// Unwrap allows error unwrapping for errors.As()
func (e *LimitExceededError) Unwrap() error {
	return ErrRateLimitExceeded
}

// InvalidConfigError is returned when limiter configuration is invalid.
//
// This provides detailed information about what configuration value was invalid.
//
// Example:
//
//	var configErr *flexlimit.InvalidConfigError
//	if errors.As(err, &configErr) {
//	    fmt.Printf("Invalid config: %s = %v (%s)\n",
//	        configErr.Field, configErr.Value, configErr.Reason)
//	}
type InvalidConfigError struct {
	// Field is the name of the configuration field that is invalid
	Field string

	// Value is the invalid value that was provided
	Value interface{}

	// Reason explains why the value is invalid
	Reason string
}

// Error implements the error interface.
func (e *InvalidConfigError) Error() string {
	return fmt.Sprintf("invalid configuration: %s = %v (%s)",
		e.Field, e.Value, e.Reason)
}

// Is allows this error to be matched with errors.Is(err, ErrInvalidConfig)
func (e *InvalidConfigError) Is(target error) bool {
	return target == ErrInvalidConfig
}

// Unwrap allows error unwrapping for errors.As()
func (e *InvalidConfigError) Unwrap() error {
	return ErrInvalidConfig
}

// StorageError wraps errors from the storage backend with additional context.
//
// This is useful for debugging storage-related issues, especially in
// distributed deployments.
//
// Example:
//
//	var storageErr *flexlimit.StorageError
//	if errors.As(err, &storageErr) {
//	    log.Error("Storage failure",
//	        "backend", storageErr.Backend,
//	        "operation", storageErr.Operation,
//	        "key", storageErr.Key,
//	        "cause", storageErr.Err)
//	}
type StorageError struct {
	// Backend identifies which storage backend failed (e.g., "redis", "memory")
	Backend string

	// Operation is the operation that failed (e.g., "get", "set", "incr")
	Operation string

	// Key is the rate limit key that was being accessed (may be empty)
	Key string

	// Err is the underlying error from the storage backend
	Err error
}

// Error implements the error interface.
func (e *StorageError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("storage error [%s/%s] for key %q: %v",
			e.Backend, e.Operation, e.Key, e.Err)
	}
	return fmt.Sprintf("storage error [%s/%s]: %v",
		e.Backend, e.Operation, e.Err)
}

// Is allows checking for ErrStorageUnavailable
func (e *StorageError) Is(target error) bool {
	return target == ErrStorageUnavailable
}

// Unwrap returns the underlying error for error chain inspection
func (e *StorageError) Unwrap() error {
	return e.Err
}

// wrapContextError wraps context errors to our custom error types.
// This is an internal helper function.
func wrapContextError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, context.Canceled):
		return ErrContextCanceled
	case errors.Is(err, context.DeadlineExceeded):
		return ErrContextDeadlineExceeded
	default:
		return err
	}
}
