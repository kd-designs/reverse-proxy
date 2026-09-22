package circuit

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		OpenTimeout:      100 * time.Millisecond,
	})

	testErr := errors.New("backend failed")

	// Closed state: 2 failures
	_ = cb.Execute(func() error { return testErr })
	_ = cb.Execute(func() error { return testErr })
	if cb.State() != StateClosed {
		t.Fatalf("expected CLOSED state after 2 failures, got %s", cb.State())
	}

	// 3rd failure trips the breaker to OPEN
	_ = cb.Execute(func() error { return testErr })
	if cb.State() != StateOpen {
		t.Fatalf("expected OPEN state after 3 failures, got %s", cb.State())
	}

	// Execution rejected immediately
	err := cb.Execute(func() error { return nil })
	if err != ErrCircuitOpen {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}

	// Wait for open timeout -> transitions to HALF_OPEN
	time.Sleep(120 * time.Millisecond)
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected HALF_OPEN state, got %s", cb.State())
	}

	// 1st success in Half-Open
	_ = cb.Execute(func() error { return nil })
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected HALF_OPEN after 1 success, got %s", cb.State())
	}

	// 2nd success in Half-Open -> resets to CLOSED
	_ = cb.Execute(func() error { return nil })
	if cb.State() != StateClosed {
		t.Fatalf("expected CLOSED after 2 successes, got %s", cb.State())
	}
}
