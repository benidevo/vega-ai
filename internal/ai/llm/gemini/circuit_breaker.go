package gemini

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// CircuitState represents the current state of the circuit breaker.
type CircuitState int32

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

// String returns the string representation of the circuit state.
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements the circuit breaker pattern for fault tolerance.
type CircuitBreaker struct {
	maxFailures      int
	resetTimeout     time.Duration
	halfOpenRequests int

	state           atomic.Value
	failures        atomic.Int32
	lastFailureTime atomic.Value
	successCount    atomic.Int32

	halfOpenMu       sync.Mutex
	halfOpenAttempts int

	totalRequests   atomic.Int64
	blockedRequests atomic.Int64
	successfulCalls atomic.Int64
	failedCalls     atomic.Int64

	onStateChange func(from, to CircuitState)
}

// CircuitBreakerConfig contains configuration options for the circuit breaker.
type CircuitBreakerConfig struct {
	MaxFailures      int
	ResetTimeout     time.Duration
	HalfOpenRequests int
	OnStateChange    func(from, to CircuitState)
}

// DefaultCircuitBreakerConfig returns sensible default configuration.
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		MaxFailures:      5,
		ResetTimeout:     30 * time.Second,
		HalfOpenRequests: 3,
		OnStateChange:    nil,
	}
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration.
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

	cb := &CircuitBreaker{
		maxFailures:      config.MaxFailures,
		resetTimeout:     config.ResetTimeout,
		halfOpenRequests: config.HalfOpenRequests,
		onStateChange:    config.OnStateChange,
	}

	cb.state.Store(StateClosed)
	cb.lastFailureTime.Store(time.Time{})

	return cb
}

// Call executes the given function through the circuit breaker.
func (cb *CircuitBreaker) Call(ctx context.Context, fn func() error) error {
	cb.totalRequests.Add(1)

	if !cb.canAttempt() {
		cb.blockedRequests.Add(1)
		return ErrCircuitBreakerOpen
	}

	err := fn()

	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}

	return err
}

func (cb *CircuitBreaker) canAttempt() bool {
	state := cb.GetState()

	switch state {
	case StateClosed:
		return true

	case StateOpen:
		if cb.shouldAttemptReset() {
			cb.transitionTo(StateHalfOpen)
			return cb.canAttemptHalfOpen()
		}
		return false

	case StateHalfOpen:
		return cb.canAttemptHalfOpen()

	default:
		return false
	}
}

func (cb *CircuitBreaker) canAttemptHalfOpen() bool {
	cb.halfOpenMu.Lock()
	defer cb.halfOpenMu.Unlock()

	if cb.halfOpenAttempts >= cb.halfOpenRequests {
		return false
	}

	cb.halfOpenAttempts++
	return true
}

func (cb *CircuitBreaker) shouldAttemptReset() bool {
	val := cb.lastFailureTime.Load()
	if val == nil {
		return true
	}
	lastFailure, ok := val.(time.Time)
	if !ok {
		return true
	}
	if lastFailure.IsZero() {
		return true
	}

	return time.Since(lastFailure) >= cb.resetTimeout
}

func (cb *CircuitBreaker) recordFailure() {
	cb.failedCalls.Add(1)
	cb.failures.Add(1)
	cb.lastFailureTime.Store(time.Now())

	state := cb.GetState()

	switch state {
	case StateClosed:
		if int(cb.failures.Load()) >= cb.maxFailures {
			cb.transitionTo(StateOpen)
		}

	case StateHalfOpen:
		cb.transitionTo(StateOpen)
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	cb.successfulCalls.Add(1)

	state := cb.GetState()

	switch state {
	case StateHalfOpen:
		cb.successCount.Add(1)
		if int(cb.successCount.Load()) >= cb.halfOpenRequests {
			cb.transitionTo(StateClosed)
		}

	case StateClosed:
		cb.failures.Store(0)
	}
}

func (cb *CircuitBreaker) transitionTo(newState CircuitState) {
	oldState := cb.GetState()
	if oldState == newState {
		return
	}

	cb.state.Store(newState)

	switch newState {
	case StateClosed:
		cb.failures.Store(0)
		cb.successCount.Store(0)
		cb.halfOpenMu.Lock()
		cb.halfOpenAttempts = 0
		cb.halfOpenMu.Unlock()

	case StateOpen:
		cb.successCount.Store(0)
		cb.halfOpenMu.Lock()
		cb.halfOpenAttempts = 0
		cb.halfOpenMu.Unlock()

	case StateHalfOpen:
		cb.failures.Store(0)
		cb.successCount.Store(0)
		cb.halfOpenMu.Lock()
		cb.halfOpenAttempts = 0
		cb.halfOpenMu.Unlock()
	}

	if cb.onStateChange != nil {
		cb.onStateChange(oldState, newState)
	}
}

// GetState returns the current state of the circuit breaker.
func (cb *CircuitBreaker) GetState() CircuitState {
	val := cb.state.Load()
	if val == nil {
		return StateClosed
	}
	state, ok := val.(CircuitState)
	if !ok {
		return StateClosed
	}
	return state
}

// Reset manually resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.transitionTo(StateClosed)
}

// GetStats returns current circuit breaker statistics.
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	state := cb.GetState()

	return CircuitBreakerStats{
		State:           state.String(),
		TotalRequests:   cb.totalRequests.Load(),
		BlockedRequests: cb.blockedRequests.Load(),
		SuccessfulCalls: cb.successfulCalls.Load(),
		FailedCalls:     cb.failedCalls.Load(),
		CurrentFailures: int(cb.failures.Load()),
		MaxFailures:     cb.maxFailures,
	}
}

// CircuitBreakerStats contains metrics about circuit breaker operations.
type CircuitBreakerStats struct {
	State           string
	TotalRequests   int64
	BlockedRequests int64
	SuccessfulCalls int64
	FailedCalls     int64
	CurrentFailures int
	MaxFailures     int
}

// ErrCircuitBreakerOpen is returned when the circuit breaker is in open state.
var ErrCircuitBreakerOpen = errors.New("circuit breaker is open")

// IsCircuitBreakerError checks if an error is due to circuit breaker being open.
func IsCircuitBreakerError(err error) bool {
	return errors.Is(err, ErrCircuitBreakerOpen)
}
