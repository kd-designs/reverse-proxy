# go-reverse-proxy

A high-performance L7 HTTP Reverse Proxy and Load Balancer built in Go.

## Features

- **Load Balancing Algorithms**:
  - Round-Robin
  - Least-Connections
  - IP Hash (consistent client affinity)
- **Rate Limiting**:
  - Token-Bucket algorithm per remote IP address with automatic refill.
- **Circuit Breaker**:
  - Three-state resilience pattern (Closed $\to$ Open $\to$ Half-Open $\to$ Closed) per upstream backend.
- **Active Health Checks**:
  - Periodic background HTTP probes with automatic ejection of failing upstreams.
- **Header Management**:
  - Standard headers manipulation (`X-Forwarded-For`, `X-Forwarded-Proto`, `X-Real-IP`).

## Running Tests

```bash
go test -v -race ./...
```
