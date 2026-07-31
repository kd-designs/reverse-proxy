package balancer

import (
	"errors"
	"hash/fnv"
	"net/url"
	"sync"
	"sync/atomic"
)

var ErrNoHealthyBackend = errors.New("balancer: no healthy backend available")

type Backend struct {
	URL         *url.URL
	Weight      int
	ActiveConns atomic.Int64
	Healthy     atomic.Bool
}

func NewBackend(u *url.URL, weight int) *Backend {
	b := &Backend{
		URL:    u,
		Weight: weight,
	}
	b.Healthy.Store(true)
	return b
}

type Balancer interface {
	Next(backends []*Backend, clientIP string) (*Backend, error)
}

// RoundRobin Balancer
type RoundRobin struct {
	index atomic.Uint64
}

func NewRoundRobin() *RoundRobin {
	return &RoundRobin{}
}

func (r *RoundRobin) Next(backends []*Backend, _ string) (*Backend, error) {
	healthy := filterHealthy(backends)
	if len(healthy) == 0 {
		return nil, ErrNoHealthyBackend
	}

	idx := r.index.Add(1) - 1
	return healthy[idx%uint64(len(healthy))], nil
}

// LeastConnections Balancer
type LeastConnections struct {
	mu sync.Mutex
}

func NewLeastConnections() *LeastConnections {
	return &LeastConnections{}
}

func (l *LeastConnections) Next(backends []*Backend, _ string) (*Backend, error) {
	healthy := filterHealthy(backends)
	if len(healthy) == 0 {
		return nil, ErrNoHealthyBackend
	}

	best := healthy[0]
	minConns := best.ActiveConns.Load()

	for _, b := range healthy[1:] {
		conns := b.ActiveConns.Load()
		if conns < minConns {
			minConns = conns
			best = b
		}
	}
	return best, nil
}

// IPHash Balancer
type IPHash struct{}

func NewIPHash() *IPHash {
	return &IPHash{}
}

func (h *IPHash) Next(backends []*Backend, clientIP string) (*Backend, error) {
	healthy := filterHealthy(backends)
	if len(healthy) == 0 {
		return nil, ErrNoHealthyBackend
	}

	hasher := fnv.New32a()
	hasher.Write([]byte(clientIP))
	hashVal := hasher.Sum32()

	idx := int(hashVal) % len(healthy)
	return healthy[idx], nil
}

func filterHealthy(backends []*Backend) []*Backend {
	res := make([]*Backend, 0, len(backends))
	for _, b := range backends {
		if b.Healthy.Load() {
			res = append(res, b)
		}
	}
	return res
}
