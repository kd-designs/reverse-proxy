package circuit

import (
	"errors"
	"sync"
	"time"
)

var ErrCircuitOpen = errors.New("circuit breaker: open")

type State string

const (
	StateClosed   State = "CLOSED"
	StateHalfOpen State = "HALF_OPEN"
	StateOpen     State = "OPEN"
)

type Config struct {
	FailureThreshold    int           // consecutive failures to trip
	SuccessThreshold    int           // consecutive successes in Half-Open to reset
	OpenTimeout         time.Duration // duration in Open state before Half-Open
}

type CircuitBreaker struct {
	mu                  sync.Mutex
	cfg                 Config
	state               State
	consecutiveFailures int
	consecutiveSuccess int
	lastStateChange     time.Time
}

func NewCircuitBreaker(cfg Config) *CircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 2
	}
	if cfg.OpenTimeout <= 0 {
		cfg.OpenTimeout = 5 * time.Second
	}

	return &CircuitBreaker{
		cfg:             cfg,
		state:           StateClosed,
		lastStateChange: time.Now(),
	}
}

func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.checkTimeoutLocked()
	return cb.state
}

func (cb *CircuitBreaker) checkTimeoutLocked() {
	if cb.state == StateOpen && time.Since(cb.lastStateChange) >= cb.cfg.OpenTimeout {
		cb.state = StateHalfOpen
		cb.lastStateChange = time.Now()
		cb.consecutiveSuccess = 0
		cb.consecutiveFailures = 0
	}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.checkTimeoutLocked()
	return cb.state != StateOpen
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.checkTimeoutLocked()

	if cb.state == StateHalfOpen {
		cb.consecutiveSuccess++
		if cb.consecutiveSuccess >= cb.cfg.SuccessThreshold {
			cb.state = StateClosed
			cb.lastStateChange = time.Now()
			cb.consecutiveFailures = 0
			cb.consecutiveSuccess = 0
		}
	} else if cb.state == StateClosed {
		cb.consecutiveFailures = 0
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.checkTimeoutLocked()

	if cb.state == StateHalfOpen {
		cb.state = StateOpen
		cb.lastStateChange = time.Now()
		cb.consecutiveFailures = 0
		cb.consecutiveSuccess = 0
	} else if cb.state == StateClosed {
		cb.consecutiveFailures++
		if cb.consecutiveFailures >= cb.cfg.FailureThreshold {
			cb.state = StateOpen
			cb.lastStateChange = time.Now()
		}
	}
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.Allow() {
		return ErrCircuitOpen
	}

	err := fn()
	if err != nil {
		cb.RecordFailure()
		return err
	}

	cb.RecordSuccess()
	return nil
}
