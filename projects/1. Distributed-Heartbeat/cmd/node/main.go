package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/YpatiosCh/distributed-systems/projects/distributed-heartbeat/internal/config"
	"github.com/YpatiosCh/distributed-systems/projects/distributed-heartbeat/internal/logging"
	"github.com/YpatiosCh/distributed-systems/projects/distributed-heartbeat/internal/metrics"
	"github.com/YpatiosCh/distributed-systems/projects/distributed-heartbeat/internal/monitor"
	"github.com/YpatiosCh/distributed-systems/projects/distributed-heartbeat/internal/server"
)

func main() {
	logging.Init()
	defer logging.Sync()

	log := logging.L()
	log.Infow("Starting Distributed Heartbeat Node...")

	// Load configuration
	cfg := config.Load()
	log.Infow("Configuration loaded", "port", cfg.SelfPort, "peers", cfg.PeerAddrs)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel to receive shutdown signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Start HTTP server in background
	srv := server.New(cfg)
	go func() {
		if err := srv.Start(); err != nil {
			log.Errorw("HTTP server failed", "error", err)
		}
	}()

	// Start monitor
	go monitor.Start(ctx, cfg, srv)

	// Start Prometheus metrics server (on port 9100 by default)
	go metrics.StartMetricsServer("9100")

	// Wait for shutdown signal
	<-stop
	log.Infow("Shutdown signal received, cleaning up...")

	// Graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		log.Errorw("HTTP server shutdown failed", "error", err)
	} else {
		log.Infow("HTTP server stopped gracefully")
	}
}
