package algorithm

import (
	"context"
	"fmt"
	"time"
)

type Algorithm interface {
	// Allow checks if a request should be allowed and consumes tokens if so.
	//
	// The cost parameter specifies how many tokens this request consumes.
	// For standard rate limiting, cost is 1. For cost-based limiting
	// (Feature 5), cost can vary per operation.
	//
	// Returns:
	//   - allowed: true if request should be allowed
	//   - state: current state after this check
	//   - error: if an error occurred (storage failure, etc.)
	//
	// If allowed is true, tokens are consumed. If false, state is unchanged.
	//
	// Example:
	//
	//	allowed, state, err := algo.Allow(ctx, "user:123", 1)
	//	if err != nil {
	//	    return err
	//	}
	//	if !allowed {
	//	    return ErrRateLimitExceeded
	//	}
	Allow(ctx context.Context, key string, cost int) (bool, *State, error)

	// State returns the current rate limiting state for a key without
	// consuming any tokens.
	State(ctx context.Context, key string) (*State, error)
	// Reset clears all state for a key, effectively giving them a fresh start.
	//
	// This is useful for:
	//   - Testing (reset between tests)
	//   - Admin actions (manually reset user's limit)
	//   - Upgrading user tier (reset to new limits)
	//
	// Example:
	//
	//	err := algo.Reset(ctx, "user:123")
	Reset(ctx context.Context, key string) error

	// Close releases any resources held by the algorithm.
	//
	// This is called when the limiter is shut down. Implementations
	// should cleanup goroutines, close connections, etc.
	//
	// Example:
	//
	//	defer algo.Close()
	Close() error
}

// State represents the current rate limiting state for a key.
//
// This is the algorithm's view of state - it contains calculated values
// that are ready to be shown to users or used for decisions.
//
// Compare with storage.State which contains raw data for persistence.
type State struct {
	// Key is the rate limit key
	Key string

	// Limit is the maximum requests allowed in the window
	Limit int64

	// Remaining is how many requests are left in the current window
	Remaining int64

	// ResetAt is when the limit will reset (for Fixed Window)
	// or when tokens will be fully replenished (for Token Bucket)
	ResetAt time.Time

	// RetryAfter is how long to wait before the next request will be allowed
	// This is 0 if requests are currently allowed
	RetryAfter time.Duration

	// Current is the current usage (requests made or tokens used)
	Current int64

	// Algorithm identifies which algorithm produced this state
	// ("token_bucket", "fixed_window", "sliding_window", "leaky_bucket")
	Algorithm string
}

// Config holds configuration for algorithm initialization.
//
// Different algorithms use different fields. Common fields:
//   - Rate: requests per window
//   - Window: time window duration
//   - BurstSize: maximum burst capacity (Token Bucket only)
type Config struct {
	// Rate is the number of requests allowed per window
	Rate int64

	// Window is the time period for the rate limit
	Window time.Duration

	// BurstSize is the maximum burst capacity (Token Bucket specific)
	// If 0, defaults to Rate (no extra burst capacity)
	BurstSize int64

	// Algorithm specifies which algorithm to use
	Algorithm string
}

// Validate checks if the config is valid.
func (c *Config) Validate() error {
	if c.Rate <= 0 {
		return &ConfigError{
			Field:  "rate",
			Value:  c.Rate,
			Reason: "must be positive",
		}
	}

	if c.Window <= 0 {
		return &ConfigError{
			Field:  "window",
			Value:  c.Window,
			Reason: "must be positive",
		}
	}

	if c.BurstSize < 0 {
		return &ConfigError{
			Field:  "burst_size",
			Value:  c.BurstSize,
			Reason: "cannot be negative",
		}
	}

	return nil
}

// ConfigError represents a configuration error.
type ConfigError struct {
	Field  string
	Value  interface{}
	Reason string
}

// Error implements the error interface.
func (e *ConfigError) Error() string {
	return fmt.Sprintf("algorithm config error: %s = %v (%s)",
		e.Field, e.Value, e.Reason)
}

// AlgorithmType represents the available algorithm types.
type AlgorithmType string

const (
	// TokenBucket allows bursts and has smooth token refills.
	// Best for: User-facing APIs, general rate limiting
	TokenBucket AlgorithmType = "token_bucket"

	// FixedWindow divides time into fixed intervals.
	// Best for: Billing, quotas, simple rate limiting
	FixedWindow AlgorithmType = "fixed_window"

	// SlidingWindow tracks requests in a rolling time window.
	// Best for: Accurate rate limiting, no boundary bursts
	SlidingWindow AlgorithmType = "sliding_window"

	// LeakyBucket enforces a strict constant rate.
	// Best for: Traffic shaping, smooth rate enforcement
	LeakyBucket AlgorithmType = "leaky_bucket"
)

// String returns the string representation of the algorithm type.
func (a AlgorithmType) String() string {
	return string(a)
}

// Validate checks if the algorithm type is valid.
func (a AlgorithmType) Validate() error {
	switch a {
	case TokenBucket, FixedWindow, SlidingWindow, LeakyBucket:
		return nil
	default:
		return &ConfigError{
			Field:  "algorithm",
			Value:  a,
			Reason: "must be one of: token_bucket, fixed_window, sliding_window, leaky_bucket",
		}
	}
}
