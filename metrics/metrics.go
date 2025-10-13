// Package metrics provides interfaces for collecting rate limiting metrics.
//
// The Collector interface allows users to track when requests are allowed
// or denied, and to export metrics to various backends (Prometheus, logs, etc.).
//
// Example usage:
//
//	// Custom implementation
//	type MyCollector struct { /* ... */ }
//	func (m *MyCollector) RecordAllow(ctx context.Context, info LimitInfo) {
//	    // Your logic here
//	}
//
//	limiter := flexlimit.New(100, time.Minute,
//	    flexlimit.WithMetrics(myCollector),
//	)
package metrics

import (
	"context"
	"time"
)

// Collector defines the interface for metrics collection.
//
// Implementations must be safe for concurrent use by multiple goroutines.
//
// The context parameter allows custom implementations to:
//   - Make network calls (e.g., write to remote DB)
//   - Respect cancellation/timeouts
//   - Pass tracing information
type Collector interface {
	// RecordAllow is called when a request is allowed.
	//
	// This is called AFTER the request has been approved and tokens consumed.
	// Implementations should be fast to avoid adding latency.
	RecordAllow(ctx context.Context, info LimitInfo)

	// RecordDeny is called when a request is denied due to rate limiting.
	//
	// This is called when a key has exceeded its rate limit.
	RecordDeny(ctx context.Context, info LimitInfo)

	// RecordError is called when an error occurs in the rate limiter.
	//
	// This is called for operational errors like:
	//   - Storage backend failures (Redis down)
	//   - Configuration errors
	//   - Internal errors
	//
	// This does NOT include rate limit denials (those use RecordDeny).
	RecordError(ctx context.Context, key string, err error)

	// Close releases any resources held by the collector.
	//
	// This is called when the rate limiter is shutting down.
	Close() error
}

// LimitInfo contains information about a rate limiting decision.
//
// This struct is passed to RecordAllow and RecordDeny to provide
// context about what happened.
type LimitInfo struct {
	// Key is the rate limit key that was checked.
	// Examples: "user:123", "ip:192.168.1.5", "api_key:abc123"
	Key string

	// Limit is the maximum number of requests allowed in the window.
	// Example: 100 (for "100 requests per minute")
	Limit int64

	// Used is the number of requests already consumed.
	// Example: 73 (user has made 73 requests so far)
	Used int64

	// Remaining is the number of requests still available.
	// This is calculated as: Limit - Used
	// Example: 27 (user can make 27 more requests)
	Remaining int64

	// ResetAt is the absolute time when the limit will reset.
	ResetAt time.Time

	// RetryAfter is the duration until the next request will be allowed.
	// For denied requests: How long to wait before retrying
	// For allowed requests: Usually 0
	RetryAfter time.Duration

	// Algorithm identifies which rate limiting algorithm made this decision.
	// Possible values: "token_bucket", "fixed_window", "sliding_window", "leaky_bucket"
	Algorithm string

	// Cost is the number of tokens this request consumed.
	// For standard rate limiting: 1
	// For cost-based limiting: varies per operation
	Cost int

	// Timestamp is when this rate limiting decision was made.
	Timestamp time.Time
}
