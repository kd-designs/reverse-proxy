package upstream

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/commulated/go-reverse-proxy/pkg/balancer"
	"github.com/commulated/go-reverse-proxy/pkg/circuit"
)

type UpstreamBackend struct {
	Backend *balancer.Backend
	Breaker *circuit.CircuitBreaker
}

type Pool struct {
	mu            sync.RWMutex
	backends      []*UpstreamBackend
	healthPath    string
	checkInterval time.Duration
	httpClient    *http.Client
	stopCh        chan struct{}
}

func NewPool(healthPath string, checkInterval time.Duration) *Pool {
	if healthPath == "" {
		healthPath = "/healthz"
	}
	if checkInterval <= 0 {
		checkInterval = 5 * time.Second
	}

	return &Pool{
		backends:      make([]*UpstreamBackend, 0),
		healthPath:    healthPath,
		checkInterval: checkInterval,
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
		stopCh: make(chan struct{}),
	}
}

func (p *Pool) Add(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	b := balancer.NewBackend(u, 1)
	cb := circuit.NewCircuitBreaker(circuit.Config{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		OpenTimeout:      3 * time.Second,
	})

	p.mu.Lock()
	defer p.mu.Unlock()
	p.backends = append(p.backends, &UpstreamBackend{
		Backend: b,
		Breaker: cb,
	})
	return nil
}

func (p *Pool) BalancerBackends() []*balancer.Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()

	res := make([]*balancer.Backend, len(p.backends))
	for i, ub := range p.backends {
		res[i] = ub.Backend
	}
	return res
}

func (p *Pool) FindUpstream(b *balancer.Backend) *UpstreamBackend {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, ub := range p.backends {
		if ub.Backend == b {
			return ub
		}
	}
	return nil
}

func (p *Pool) CheckHealthOnce() {
	p.mu.RLock()
	backends := make([]*UpstreamBackend, len(p.backends))
	copy(backends, p.backends)
	p.mu.RUnlock()

	for _, ub := range backends {
		target := ub.Backend.URL.ResolveReference(&url.URL{Path: p.healthPath}).String()
		resp, err := p.httpClient.Get(target)
		if err != nil || resp.StatusCode >= 500 {
			ub.Backend.Healthy.Store(false)
			ub.Breaker.RecordFailure()
		} else {
			ub.Backend.Healthy.Store(true)
			ub.Breaker.RecordSuccess()
		}
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}
}

func (p *Pool) StartHealthCheck(ctx context.Context) {
	ticker := time.NewTicker(p.checkInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.stopCh:
				return
			case <-ticker.C:
				p.CheckHealthOnce()
			}
		}
	}()
}

func (p *Pool) Stop() {
	close(p.stopCh)
}
