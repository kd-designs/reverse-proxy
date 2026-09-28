package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/commulated/go-reverse-proxy/pkg/balancer"
	"github.com/commulated/go-reverse-proxy/pkg/upstream"
)

func TestProxyHandler_EndToEnd(t *testing.T) {
	// 1. Backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded := r.Header.Get("X-Forwarded-For")
		w.Header().Set("X-Custom-Echo", "pong")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("backend-ok: " + forwarded))
	}))
	defer backend.Close()

	pool := upstream.NewPool("/healthz", 1*time.Second)
	_ = pool.Add(backend.URL)

	bal := balancer.NewRoundRobin()
	handler := NewProxyHandler(pool, bal, Config{
		RateLimitCap:    10,
		RateLimitRefill: 10,
	})

	proxyServer := httptest.NewServer(handler)
	defer proxyServer.Close()

	// 2. Perform request
	req, _ := http.NewRequest(http.MethodGet, proxyServer.URL+"/test-path", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.195")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Custom-Echo") != "pong" {
		t.Fatalf("expected echoed header")
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "backend-ok: 203.0.113.195, 203.0.113.195" {
		t.Logf("Response body: %s", string(body))
	}
}

func TestProxyHandler_RateLimit(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	pool := upstream.NewPool("/healthz", 1*time.Second)
	_ = pool.Add(backend.URL)

	bal := balancer.NewRoundRobin()
	handler := NewProxyHandler(pool, bal, Config{
		RateLimitCap:    2,
		RateLimitRefill: 0.1, // very slow refill
	})

	proxyServer := httptest.NewServer(handler)
	defer proxyServer.Close()

	client := &http.Client{}

	// Request 1: OK
	r1, _ := client.Get(proxyServer.URL)
	if r1.StatusCode != http.StatusOK {
		t.Fatalf("r1 should be 200, got %d", r1.StatusCode)
	}
	// Request 2: OK
	r2, _ := client.Get(proxyServer.URL)
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("r2 should be 200, got %d", r2.StatusCode)
	}
	// Request 3: 429 Too Many Requests
	r3, _ := client.Get(proxyServer.URL)
	if r3.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("r3 should be 429, got %d", r3.StatusCode)
	}
}
