package ratelimit

import (
	"sync"
	"time"
)

type TokenBucket struct {
	mu         sync.Mutex
	capacity   float64
	tokens     float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

func NewTokenBucket(capacity float64, refillRate float64) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

func (tb *TokenBucket) Allow() bool {
	return tb.AllowN(1.0)
}

func (tb *TokenBucket) AllowN(n float64) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.lastRefill = now

	tb.tokens = tb.tokens + elapsed*tb.refillRate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}

	if tb.tokens >= n {
		tb.tokens -= n
		return true
	}
	return false
}

type IPRateLimiter struct {
	mu         sync.RWMutex
	buckets    map[string]*TokenBucket
	capacity   float64
	refillRate float64
}

func NewIPRateLimiter(capacity float64, refillRate float64) *IPRateLimiter {
	return &IPRateLimiter{
		buckets:    make(map[string]*TokenBucket),
		capacity:   capacity,
		refillRate: refillRate,
	}
}

func (l *IPRateLimiter) Allow(ip string) bool {
	l.mu.RLock()
	bucket, exists := l.buckets[ip]
	l.mu.RUnlock()

	if !exists {
		l.mu.Lock()
		bucket, exists = l.buckets[ip]
		if !exists {
			bucket = NewTokenBucket(l.capacity, l.refillRate)
			l.buckets[ip] = bucket
		}
		l.mu.Unlock()
	}

	return bucket.Allow()
}
