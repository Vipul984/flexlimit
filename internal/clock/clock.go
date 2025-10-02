// This package allows the rate limiter to work with controllable time
// in tests while using real time in production. This is critical for
// testing time-based logic without waiting for real time to pass.
//
// Usage in production code:
//
//	clock := clock.New()
//	now := clock.Now()
//
// Usage in tests:
//
//	clock := clock.NewMock()
//	clock.Set(specificTime)
//	clock.Advance(1 * time.Hour)
package clock

import (
	"sync"
	"time"
)

// Clock provides the current time.
//
// Implementations must be safe for concurrent use by multiple goroutines.
type Clock interface {
	// Now returns the current time.
	//
	// For real clocks, this returns time.Now().
	// For mock clocks, this returns the simulated time.
	Now() time.Time
}

// Real is a Clock that uses the system time.
//
// This is the production implementation and simply wraps time.Now().
type Real struct{}

// New creates a new real clock that uses system time.
//
// This is the default clock used in production.
func New() Clock {
	return &Real{}
}

// Now returns the current system time.
func (r *Real) Now() time.Time {
	return time.Now()
}

// Mock is a Clock with controllable time for testing.
//
// Mock is safe for concurrent use. All methods use a mutex to
// ensure thread-safety when reading or modifying the current time.
//
// Example usage:
//
//	clock := clock.NewMock()
//	clock.Set(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
//
//	// Test some logic
//	result := limiter.Allow("user")
//
//	// Advance time
//	clock.Advance(1 * time.Hour)
//
//	// Test again with new time
//	result = limiter.Allow("user")
type Mock struct {
	mu   sync.RWMutex
	now  time.Time
	auto bool // If true, advances time automatically on each Now() call
	step time.Duration
}

// NewMock creates a new mock clock starting at the current system time.
//
// You can change the time using Set() or Advance().
func NewMock() *Mock {
	return &Mock{
		now: time.Now(),
	}
}

// NewMockAt creates a new mock clock starting at the specified time.
func NewMockAt(t time.Time) *Mock {
	return &Mock{
		now: t,
	}
}

// Now returns the current mock time.
//
// If auto-advance is enabled, this will automatically advance
// the clock by the configured step duration.
func (m *Mock) Now() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := m.now

	if m.auto {
		// Auto-advance happens after reading, so next call sees advanced time
		m.mu.RUnlock()
		m.mu.Lock()
		m.now = m.now.Add(m.step)
		m.mu.Unlock()
		m.mu.RLock()
	}

	return now
}

// Set sets the mock clock to a specific time.
//
// This is useful for setting up test scenarios at exact times.
//
// Example:
//
//	clock.Set(time.Date(2025, 10, 1, 12, 0, 0, 0, time.UTC))
func (m *Mock) Set(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = t
}

// Advance moves the mock clock forward by the specified duration.
//
// This is useful for simulating the passage of time in tests.
//
// Example:
//
//	clock.Advance(5 * time.Minute)  // Jump forward 5 minutes
//	clock.Advance(1 * time.Hour)     // Jump forward 1 hour
func (m *Mock) Advance(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = m.now.Add(d)
}

// SetAutoAdvance enables automatic time advancement.
//
// When enabled, each call to Now() automatically advances the clock
// by the specified step duration. This is useful for simulating
// continuous time progression in tests.
//
// Example:
//
//	clock.SetAutoAdvance(1 * time.Second)
//	clock.Now() // Returns T
//	clock.Now() // Returns T + 1s
//	clock.Now() // Returns T + 2s
func (m *Mock) SetAutoAdvance(step time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.auto = true
	m.step = step
}

// DisableAutoAdvance disables automatic time advancement.
func (m *Mock) DisableAutoAdvance() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.auto = false
}

// Since returns the duration since the given time.
//
// This is a convenience method equivalent to:
//
//	clock.Now().Sub(t)
func (m *Mock) Since(t time.Time) time.Duration {
	return m.Now().Sub(t)
}
