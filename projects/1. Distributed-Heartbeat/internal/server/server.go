package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/YpatiosCh/distributed-systems/projects/distributed-heartbeat/internal/config"
	"github.com/YpatiosCh/distributed-systems/projects/distributed-heartbeat/internal/logging"
)

type Server struct {
	httpServer *http.Server
	cfg        *config.Config
	mu         sync.Mutex
	lastPing   map[string]time.Time
}

func New(cfg *config.Config) *Server {
	s := &Server{
		cfg:      cfg,
		lastPing: make(map[string]time.Time),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ping", s.handlePing)
	s.httpServer = &http.Server{
		Addr:    ":" + cfg.SelfPort,
		Handler: mux,
	}
	return s
}

func (s *Server) Start() error {
	log := logging.L()
	log.Infow("HTTP server listening", "port", s.cfg.SelfPort)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	log := logging.L()
	log.Infow("Shutting down HTTP server...")
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	log := logging.L()

	from := r.URL.Query().Get("from")
	if from == "" {
		http.Error(w, "Missing ?from parameter", http.StatusBadRequest)
		log.Warnw("Received ping with missing 'from'", "remoteAddr", r.RemoteAddr)
		return
	}

	s.mu.Lock()
	s.lastPing[from] = time.Now()
	s.mu.Unlock()

	log.Infow("Received ping", "from", from)
	fmt.Fprintf(w, "pong")
}

func (s *Server) GetLastPing(peer string) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastPing[peer]
}
