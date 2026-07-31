package balancer

import (
	"net/url"
	"testing"
)

func makeTestBackend(rawURL string) *Backend {
	u, _ := url.Parse(rawURL)
	return NewBackend(u, 1)
}

func TestRoundRobin(t *testing.T) {
	b1 := makeTestBackend("http://10.0.0.1:8080")
	b2 := makeTestBackend("http://10.0.0.2:8080")
	backends := []*Backend{b1, b2}

	rr := NewRoundRobin()

	p1, _ := rr.Next(backends, "")
	p2, _ := rr.Next(backends, "")
	p3, _ := rr.Next(backends, "")

	if p1.URL.Host != "10.0.0.1:8080" || p2.URL.Host != "10.0.0.2:8080" || p3.URL.Host != "10.0.0.1:8080" {
		t.Fatalf("round robin failed sequence: %s, %s, %s", p1.URL.Host, p2.URL.Host, p3.URL.Host)
	}

	// Mark b1 unhealthy
	b1.Healthy.Store(false)
	p4, _ := rr.Next(backends, "")
	if p4.URL.Host != "10.0.0.2:8080" {
		t.Fatalf("expected b2 when b1 is unhealthy, got %s", p4.URL.Host)
	}
}

func TestLeastConnections(t *testing.T) {
	b1 := makeTestBackend("http://10.0.0.1:8080")
	b2 := makeTestBackend("http://10.0.0.2:8080")
	backends := []*Backend{b1, b2}

	b1.ActiveConns.Store(10)
	b2.ActiveConns.Store(2)

	lc := NewLeastConnections()
	chosen, err := lc.Next(backends, "")
	if err != nil || chosen.URL.Host != "10.0.0.2:8080" {
		t.Fatalf("expected b2 with 2 conns, got %v", chosen)
	}
}

func TestIPHash(t *testing.T) {
	b1 := makeTestBackend("http://10.0.0.1:8080")
	b2 := makeTestBackend("http://10.0.0.2:8080")
	backends := []*Backend{b1, b2}

	ipHash := NewIPHash()
	target1, _ := ipHash.Next(backends, "192.168.1.50")
	target2, _ := ipHash.Next(backends, "192.168.1.50")

	if target1.URL.Host != target2.URL.Host {
		t.Fatalf("IP hash should be deterministic for same IP")
	}
}
