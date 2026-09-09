package ratelimit

import (
	"testing"
	"time"
)

func TestTokenBucket(t *testing.T) {
	// Capacity 3 tokens, 10 tokens per sec refill
	tb := NewTokenBucket(3, 10)

	if !tb.Allow() || !tb.Allow() || !tb.Allow() {
		t.Fatal("expected first 3 tokens allowed")
	}

	if tb.Allow() {
		t.Fatal("expected 4th immediate token to be rejected")
	}

	// Sleep 250ms -> should refill ~2.5 tokens
	time.Sleep(250 * time.Millisecond)

	if !tb.Allow() || !tb.Allow() {
		t.Fatal("expected 2 tokens to have refilled")
	}
}

func TestIPRateLimiter(t *testing.T) {
	limiter := NewIPRateLimiter(2, 5)

	ip1 := "192.168.1.1"
	ip2 := "192.168.1.2"

	if !limiter.Allow(ip1) || !limiter.Allow(ip1) {
		t.Fatal("ip1 should allow 2 tokens")
	}
	if limiter.Allow(ip1) {
		t.Fatal("ip1 should be blocked after 2 tokens")
	}

	// ip2 should still have its own fresh bucket
	if !limiter.Allow(ip2) {
		t.Fatal("ip2 should be independent and allowed")
	}
}
