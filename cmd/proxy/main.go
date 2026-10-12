package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/commulated/go-reverse-proxy/pkg/balancer"
	"github.com/commulated/go-reverse-proxy/pkg/proxy"
	"github.com/commulated/go-reverse-proxy/pkg/upstream"
)

func main() {
	listenAddr := flag.String("listen", ":8000", "Listen address")
	upstreams := flag.String("upstreams", "http://127.0.0.1:8081,http://127.0.0.1:8082", "Comma-separated upstream URLs")
	strategy := flag.String("strategy", "round-robin", "Balancer strategy: round-robin | least-conn | ip-hash")
	flag.Parse()

	log.Printf("Starting go-reverse-proxy on %s, routing to: %s (strategy: %s)...", *listenAddr, *upstreams, *strategy)

	pool := upstream.NewPool("/healthz", 5*time.Second)
	for _, u := range strings.Split(*upstreams, ",") {
		u = strings.TrimSpace(u)
		if u != "" {
			if err := pool.Add(u); err != nil {
				log.Fatalf("Invalid upstream URL %s: %v", u, err)
			}
		}
	}

	var bal balancer.Balancer
	switch *strategy {
	case "least-conn":
		bal = balancer.NewLeastConnections()
	case "ip-hash":
		bal = balancer.NewIPHash()
	default:
		bal = balancer.NewRoundRobin()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.StartHealthCheck(ctx)

	handler := proxy.NewProxyHandler(pool, bal, proxy.Config{
		RateLimitCap:    100,
		RateLimitRefill: 50,
	})

	srv := &http.Server{
		Addr:    *listenAddr,
		Handler: handler,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Proxy server failed: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down go-reverse-proxy...")
	pool.Stop()
	_ = srv.Close()
	log.Println("Shutdown complete.")
}
