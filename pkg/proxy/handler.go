package proxy

import (
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/commulated/go-reverse-proxy/pkg/balancer"
	"github.com/commulated/go-reverse-proxy/pkg/circuit"
	"github.com/commulated/go-reverse-proxy/pkg/ratelimit"
	"github.com/commulated/go-reverse-proxy/pkg/upstream"
)

type Config struct {
	RateLimitCap    float64
	RateLimitRefill float64
	RequestTimeout  time.Duration
}

type ProxyHandler struct {
	pool       *upstream.Pool
	balancer   balancer.Balancer
	rateLimiter *ratelimit.IPRateLimiter
	transport  *http.Transport
	cfg        Config
}

func NewProxyHandler(pool *upstream.Pool, bal balancer.Balancer, cfg Config) *ProxyHandler {
	if cfg.RateLimitCap <= 0 {
		cfg.RateLimitCap = 100
	}
	if cfg.RateLimitRefill <= 0 {
		cfg.RateLimitRefill = 50
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 10 * time.Second
	}

	transport := &http.Transport{
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
	}

	return &ProxyHandler{
		pool:        pool,
		balancer:    bal,
		rateLimiter: ratelimit.NewIPRateLimiter(cfg.RateLimitCap, cfg.RateLimitRefill),
		transport:   transport,
		cfg:         cfg,
	}
}

func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	clientIP := extractClientIP(r)

	// 1. Rate Limiting Check
	if !h.rateLimiter.Allow(clientIP) {
		http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
		return
	}

	// 2. Select Backend via Balancer
	backends := h.pool.BalancerBackends()
	selected, err := h.balancer.Next(backends, clientIP)
	if err != nil {
		http.Error(w, `{"error":"no healthy upstreams available"}`, http.StatusBadGateway)
		return
	}

	up := h.pool.FindUpstream(selected)
	if up == nil {
		http.Error(w, `{"error":"upstream configuration error"}`, http.StatusInternalServerError)
		return
	}

	// 3. Track Active Connections
	selected.ActiveConns.Add(1)
	defer selected.ActiveConns.Add(-1)

	// 4. Execute through Circuit Breaker
	err = up.Breaker.Execute(func() error {
		outReq := r.Clone(r.Context())
		outReq.URL.Scheme = selected.URL.Scheme
		outReq.URL.Host = selected.URL.Host
		outReq.Host = selected.URL.Host

		// Set forwarding headers
		if prior := r.Header.Get("X-Forwarded-For"); prior != "" {
			outReq.Header.Set("X-Forwarded-For", prior+", "+clientIP)
		} else {
			outReq.Header.Set("X-Forwarded-For", clientIP)
		}
		outReq.Header.Set("X-Real-IP", clientIP)
		if r.TLS != nil {
			outReq.Header.Set("X-Forwarded-Proto", "https")
		} else {
			outReq.Header.Set("X-Forwarded-Proto", "http")
		}

		resp, err := h.transport.RoundTrip(outReq)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// Copy headers to response
		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
		return nil
	})

	if err != nil {
		if errors.Is(err, circuit.ErrCircuitOpen) {
			http.Error(w, `{"error":"upstream circuit breaker open"}`, http.StatusServiceUnavailable)
		} else {
			http.Error(w, `{"error":"upstream proxy failure: `+err.Error()+`"}`, http.StatusBadGateway)
		}
	}
}

func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
