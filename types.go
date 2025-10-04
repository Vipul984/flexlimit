package flexlimit

import (
	"time"
)

// State represents the current rate limiting state for a specific key.
//
// State provides a snapshot of the rate limiter's current status, including
// how many requests have been used, how many remain, and when the limit resets.
//
// Example:
//
//	state := limiter.State(ctx, "user:123")
//	fmt.Printf("Used: %d/%d\n", state.Used, state.Limit)
//	fmt.Printf("Remaining: %d\n", state.Remaining)
//	fmt.Printf("Resets in: %s\n", state.ResetIn)
type State struct {
	// Key is the rate limit key this state belongs to
	Key string

	// Limit is the maximum number of requests allowed in the window
	Limit int

	// Used is the number of requests already consumed in the current window
	Used int

	// Remaining is the number of requests remaining (Limit - Used)
	// This is a convenience field calculated from Limit and Used
	Remaining int

	// ResetAt is the absolute time when the rate limit window resets
	// and tokens are fully replenished
	ResetAt time.Time

	// ResetIn is the duration until the rate limit resets
	// This is a convenience field calculated from ResetAt
	ResetIn time.Duration

	// LastRequestAt is the time of the last request for this key
	// This may be zero if no requests have been made yet
	LastRequestAt time.Time

	// Window is the time window for this rate limit (e.g., 1 minute, 1 hour)
	Window time.Duration
}

// LimitInfo provides contextual information when a rate limit event occurs.
//
// This is passed to callback functions (OnLimit, OnAllow) to provide
// rich context about the rate limiting decision.
//
// Example:
//
//	limiter := flexlimit.New(100, time.Minute,
//	    flexlimit.OnLimit(func(info LimitInfo) {
//	        log.Warn("Rate limited",
//	            "key", info.Key,
//	            "limit", info.Limit,
//	            "used", info.Used,
//	            "reset_at", info.ResetAt)
//	    }),
//	)
type LimitInfo struct {
	// Key is the rate limit key that triggered this event
	Key string

	// Allowed indicates whether the request was allowed (true) or denied (false)
	Allowed bool

	// Limit is the maximum requests allowed
	Limit int

	// Used is the number of requests consumed so far
	Used int

	// Remaining is the number of requests left
	Remaining int

	// ResetAt is when the limit resets
	ResetAt time.Time

	// ResetIn is the duration until reset
	ResetIn time.Duration

	// Cost is the cost of this request (for cost-based limiting)
	// This will be 1 for standard limiters
	Cost int

	// Algorithm identifies which rate limiting algorithm was used
	// (e.g., "token_bucket", "sliding_window", "fixed_window")
	Algorithm string

	// Metadata allows passing custom data through callbacks
	// This can be used for request tracing, user context, etc.
	Metadata map[string]interface{}
}

// RequestContext provides multiple identifiers for composite rate limiting.
//
// When using composite limiters (Feature 2), a single request may need to
// be checked against multiple rate limits (per-IP, per-user, per-endpoint, global).
// RequestContext bundles all these identifiers together.
//
// Example:
//
//	ctx := flexlimit.RequestContext{
//	    IP:       r.RemoteAddr,           // "192.168.1.5"
//	    UserID:   getUserFromRequest(r),  // "user:123"
//	    Endpoint: r.URL.Path,             // "/api/search"
//	}
//
//	if !limiter.Allow(ctx) {
//	    http.Error(w, "Rate limited", 429)
//	}
type RequestContext struct {
	// IP is the IP address of the requester
	// Used for per-IP rate limiting (DDoS protection)
	IP string

	// UserID is the user identifier (user ID, API key, etc.)
	// Used for per-user rate limiting
	UserID string

	// Endpoint is the API endpoint or route being accessed
	// Used for per-endpoint rate limiting
	Endpoint string

	// SessionID is the session identifier
	// Used for per-session rate limiting
	SessionID string

	// Custom allows arbitrary key-value pairs for custom rate limiting strategies
	// Example: Custom["tenant_id"] = "acme_corp"
	Custom map[string]string

	// Metadata is for passing additional context through the system
	// This is NOT used for rate limiting keys, only for observability
	Metadata map[string]interface{}
}

// Key generates a rate limiting key from the context based on the strategy.
//
// This is used internally by composite limiters to extract the appropriate
// key for each sub-limiter.
//
// Example:
//
//	ctx := RequestContext{IP: "1.2.3.4", UserID: "user:123"}
//	ipKey := ctx.Key("ip")     // Returns "ip:1.2.3.4"
//	userKey := ctx.Key("user") // Returns "user:user:123"
func (rc RequestContext) Key(strategy string) string {
	switch strategy {
	case "ip":
		if rc.IP != "" {
			return "ip:" + rc.IP
		}
	case "user":
		if rc.UserID != "" {
			return "user:" + rc.UserID
		}
	case "endpoint":
		if rc.Endpoint != "" {
			return "endpoint:" + rc.Endpoint
		}
	case "session":
		if rc.SessionID != "" {
			return "session:" + rc.SessionID
		}
	case "global":
		return "global"
	default:
		// Check custom fields
		if rc.Custom != nil {
			if val, ok := rc.Custom[strategy]; ok {
				return strategy + ":" + val
			}
		}
	}
	return ""
}

// Options holds the configuration for a rate limiter.
//
// This is used internally to collect all options passed via the functional
// options pattern. Users don't interact with this directly - they use
// option functions like WithAlgorithm(), WithStorage(), etc.
//
// Example (internal use):
//
//	opts := &Options{
//	    algorithm: defaultAlgorithm,
//	    storage:   defaultStorage,
//	}
//	for _, opt := range userProvidedOptions {
//	    opt(opts)
//	}
type Options struct {
	// algorithm specifies which rate limiting algorithm to use
	// (token_bucket, sliding_window, fixed_window, leaky_bucket)
	algorithm string

	// storage is the backend for storing rate limit state
	// (memory, redis, etc.)
	storage interface{} // Will be storage.Storage once we define it

	// clock is the time source (real or mock for testing)
	clock interface{} // Will be clock.Clock once we define it

	// metrics is the metrics collector for observability
	metrics interface{} // Will be metrics.Collector once we define it

	// onLimit is called when a request is denied
	onLimit func(LimitInfo)

	// onAllow is called when a request is allowed
	onAllow func(LimitInfo)

	// fallbackStrategy defines behavior when storage fails
	// ("allow_all", "deny_all", "local_memory")
	fallbackStrategy string

	// onFallback is called when fallback is activated
	onFallback func(error)

	// maxKeys is the maximum number of keys to track (prevents memory exhaustion)
	maxKeys int

	// cleanupInterval is how often to cleanup expired keys
	cleanupInterval time.Duration

	// burstSize allows a burst of requests above the rate limit
	// (only for token bucket algorithm)
	burstSize int
}

// defaultOptions returns the default configuration.
//
// These are sensible defaults that work for most use cases.
func defaultOptions() *Options {
	return &Options{
		algorithm:        "token_bucket", // Most common algorithm
		fallbackStrategy: "allow_all",    // Fail open by default (availability over protection)
		maxKeys:          10000,          // Reasonable memory limit
		cleanupInterval:  5 * time.Minute,
		burstSize:        0, // No burst by default (strict rate limiting)
	}
}

// AlgorithmType represents the available rate limiting algorithms.
//
// This is used for type-safe algorithm selection.
type AlgorithmType string

const (
	// TokenBucket is the default algorithm. It allows bursts and smooths traffic.
	// Tokens are added at a constant rate, and each request consumes tokens.
	TokenBucket AlgorithmType = "token_bucket"

	// SlidingWindow is more accurate than fixed window but uses more memory.
	// It tracks individual request timestamps within a rolling window.
	SlidingWindow AlgorithmType = "sliding_window"

	// FixedWindow divides time into fixed intervals. Simple but can allow
	// bursts at window boundaries (2x rate for brief periods).
	FixedWindow AlgorithmType = "fixed_window"

	// LeakyBucket enforces a strict constant rate with no bursts.
	// Requests are processed at a fixed rate, excess requests are dropped.
	LeakyBucket AlgorithmType = "leaky_bucket"
)

// FallbackStrategy defines how the limiter behaves when storage fails.
type FallbackStrategy string

const (
	// AllowAll allows all requests when storage fails (fail open).
	// Prioritizes availability over protection.
	AllowAll FallbackStrategy = "allow_all"

	// DenyAll denies all requests when storage fails (fail closed).
	// Prioritizes protection over availability.
	DenyAll FallbackStrategy = "deny_all"

	// LocalMemory falls back to in-memory rate limiting when distributed
	// storage fails. Best of both worlds but uses local memory.
	LocalMemory FallbackStrategy = "local_memory"
)

// String returns the string representation of the algorithm type.
func (a AlgorithmType) String() string {
	return string(a)
}

// String returns the string representation of the fallback strategy.
func (f FallbackStrategy) String() string {
	return string(f)
}

// Validate checks if the algorithm type is valid.
func (a AlgorithmType) Validate() error {
	switch a {
	case TokenBucket, SlidingWindow, FixedWindow, LeakyBucket:
		return nil
	default:
		return &InvalidConfigError{
			Field:  "algorithm",
			Value:  a,
			Reason: "must be one of: token_bucket, sliding_window, fixed_window, leaky_bucket",
		}
	}
}

// Validate checks if the fallback strategy is valid.
func (f FallbackStrategy) Validate() error {
	switch f {
	case AllowAll, DenyAll, LocalMemory:
		return nil
	default:
		return &InvalidConfigError{
			Field:  "fallback_strategy",
			Value:  f,
			Reason: "must be one of: allow_all, deny_all, local_memory",
		}
	}
}
