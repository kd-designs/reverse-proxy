package upstream

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPool_HealthCheck(t *testing.T) {
	// 1. Create a healthy mock server
	healthySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer healthySrv.Close()

	// 2. Create an unhealthy mock server
	unhealthySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer unhealthySrv.Close()

	pool := NewPool("/healthz", 100*time.Millisecond)
	_ = pool.Add(healthySrv.URL)
	_ = pool.Add(unhealthySrv.URL)

	pool.CheckHealthOnce()

	bList := pool.BalancerBackends()
	if !bList[0].Healthy.Load() {
		t.Fatalf("expected backend 0 to be healthy")
	}
	if bList[1].Healthy.Load() {
		t.Fatalf("expected backend 1 to be unhealthy")
	}
}
