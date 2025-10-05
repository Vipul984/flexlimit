// Package storage provides abstractions for rate limiter state persistence.
//
// The Storage interface allows the rate limiter to work with different
// backends (in-memory, Redis, etc.) without changing the core logic.
//
// This enables the "Hybrid Approach" (Feature 1) where you can start with
// in-memory storage and seamlessly switch to Redis when scaling.
package storage

import (
	"context"
	"fmt"
	"time"
)

// Storage defines the interface for persisting rate limiter state.
//
// Implementations must be safe for concurrent use by multiple goroutines.
// All methods accept a context for cancellation and timeout support.
//
// The Storage interface is designed to be backend-agnostic, allowing
// implementations for memory, Redis, PostgreSQL, or any other storage system.
type Storage interface {
	// Get retrieves the current state for a rate limit key.
	//
	// Returns ErrKeyNotFound if the key doesn't exist.
	//
	// The returned State contains all information needed for rate limiting:
	// - Tokens remaining
	// - Last refill time
	// - Request timestamps (for sliding window)
	//
	// Example:
	//
	//	state, err := storage.Get(ctx, "user:123")
	//	if errors.Is(err, storage.ErrKeyNotFound) {
	//	    // Key doesn't exist, create new state
	//	}
	Get(ctx context.Context, key string) (*State, error)

	// Set stores the state for a rate limit key with optional TTL.
	//
	// If ttl is zero, the key doesn't expire (use with caution).
	// If ttl is positive, the key expires after that duration.
	//
	// The implementation should overwrite any existing state for the key.
	//
	// Example:
	//
	//	state := &State{
	//	    Tokens: 100,
	//	    LastRefill: time.Now(),
	//	}
	//	err := storage.Set(ctx, "user:123", state, 1*time.Hour)
	Set(ctx context.Context, key string, state *State, ttl time.Duration) error

	// Incr atomically increments a counter and returns the new value.
	//
	// This is used for simple counting-based algorithms (like fixed window).
	// The operation must be atomic to prevent race conditions in distributed systems.
	//
	// If the key doesn't exist, it's created with the initial value of amount.
	// If ttl > 0, the key expires after that duration.
	//
	// Example:
	//
	//	count, err := storage.Incr(ctx, "user:123:count", 1, 1*time.Minute)
	//	// count is now the total number of requests
	Incr(ctx context.Context, key string, amount int64, ttl time.Duration) (int64, error)

	// Delete removes a key and its associated state.
	//
	// Returns nil if the key doesn't exist (idempotent operation).
	//
	// Example:
	//
	//	err := storage.Delete(ctx, "user:123")
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists without retrieving its value.
	//
	// This is more efficient than Get() when you only need to check existence.
	//
	// Example:
	//
	//	exists, err := storage.Exists(ctx, "user:123")
	//	if exists {
	//	    // Key exists
	//	}
	Exists(ctx context.Context, key string) (bool, error)

	// GetMulti retrieves state for multiple keys in a single operation.
	//
	// This is an optimization for composite limiters that need to check
	// multiple keys (per-IP, per-user, global) simultaneously.
	//
	// Keys that don't exist should return nil in the corresponding position.
	//
	// Example:
	//
	//	states, err := storage.GetMulti(ctx, []string{"user:123", "ip:1.2.3.4"})
	//	// states[0] = state for user:123
	//	// states[1] = state for ip:1.2.3.4
	GetMulti(ctx context.Context, keys []string) ([]*State, error)

	// SetMulti stores state for multiple keys in a single operation.
	//
	// This is an optimization for batch updates.
	//
	// Example:
	//
	//	err := storage.SetMulti(ctx, map[string]*State{
	//	    "user:123": state1,
	//	    "ip:1.2.3.4": state2,
	//	}, 1*time.Hour)
	SetMulti(ctx context.Context, states map[string]*State, ttl time.Duration) error

	// Keys returns all keys matching a pattern (for debugging/monitoring).
	//
	// Pattern syntax depends on the backend:
	// - Memory: simple prefix matching
	// - Redis: Redis SCAN pattern (*, ?, [])
	//
	// This operation can be expensive, use sparingly in production.
	//
	// Example:
	//
	//	keys, err := storage.Keys(ctx, "user:*")
	//	// Returns: ["user:123", "user:456", ...]
	Keys(ctx context.Context, pattern string) ([]string, error)

	// Close releases any resources held by the storage backend.
	//
	// After Close() is called, the storage should not be used.
	//
	// Example:
	//
	//	defer storage.Close()
	Close() error

	// Ping checks if the storage backend is reachable.
	//
	// This is used for health checks and circuit breaker logic.
	//
	// Example:
	//
	//	if err := storage.Ping(ctx); err != nil {
	//	    // Storage is down, activate fallback
	//	}
	Ping(ctx context.Context) error
}

// State represents the rate limiter state stored in the backend.
//
// Different algorithms use different fields:
// - Token Bucket: Tokens, LastRefill
// - Sliding Window: Timestamps
// - Fixed Window: Count, WindowStart
//
// The storage implementation serializes this to the appropriate format
// (JSON for memory, hash for Redis, etc.)
type State struct {
	// Tokens is the current number of tokens available (token bucket algorithm)
	Tokens float64

	// LastRefill is when tokens were last refilled (token bucket algorithm)
	LastRefill time.Time

	// Count is the number of requests in the current window (fixed window)
	Count int64

	// WindowStart is when the current window started (fixed window)
	WindowStart time.Time

	// Timestamps stores individual request times (sliding window algorithm)
	// This can grow large for high-rate limiters
	Timestamps []time.Time

	// CreatedAt is when this state was first created
	CreatedAt time.Time

	// UpdatedAt is when this state was last modified
	UpdatedAt time.Time

	// Metadata allows storing algorithm-specific data
	Metadata map[string]interface{}
}

// Config holds configuration for storage backends.
//
// Different backends use different fields. For example:
// - Memory: MaxKeys, CleanupInterval
// - Redis: Addr, Password, DB
type Config struct {
	// Backend identifies the storage type ("memory", "redis", etc.)
	Backend string

	// MaxKeys is the maximum number of keys to store (memory only)
	// Prevents memory exhaustion. Default: 10000
	MaxKeys int

	// CleanupInterval is how often to clean up expired keys (memory only)
	// Default: 5 minutes
	CleanupInterval time.Duration

	// Redis-specific config (used in Phase 4)
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	RedisPoolSize int

	// Connection timeouts
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
}

// Error sentinel values for storage operations
var (
	// ErrKeyNotFound is returned when a key doesn't exist in storage
	ErrKeyNotFound = &StorageError{
		Op:  "get",
		Err: "key not found",
	}

	// ErrStorageUnavailable is returned when storage backend is unreachable
	ErrStorageUnavailable = &StorageError{
		Op:  "connect",
		Err: "storage backend unavailable",
	}

	// ErrInvalidState is returned when stored state is corrupted
	ErrInvalidState = &StorageError{
		Op:  "deserialize",
		Err: "invalid state format",
	}
)

// StorageError wraps storage operation errors with context.
type StorageError struct {
	// Op is the operation that failed (get, set, incr, etc.)
	Op string

	// Key is the rate limit key (may be empty)
	Key string

	// Err is the error message or underlying error
	Err interface{}
}

// Error implements the error interface
func (e *StorageError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("storage error [%s] for key %q: %v", e.Op, e.Key, e.Err)
	}
	return fmt.Sprintf("storage error [%s]: %v", e.Op, e.Err)
}

// Unwrap allows error chain inspection
func (e *StorageError) Unwrap() error {
	if err, ok := e.Err.(error); ok {
		return err
	}
	return nil
}
