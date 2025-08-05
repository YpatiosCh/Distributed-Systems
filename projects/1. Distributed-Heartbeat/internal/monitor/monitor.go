package monitor

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/YpatiosCh/distributed-systems/projects/distributed-heartbeat/internal/config"
	"github.com/YpatiosCh/distributed-systems/projects/distributed-heartbeat/internal/logging"
	"github.com/YpatiosCh/distributed-systems/projects/distributed-heartbeat/internal/metrics"
	"github.com/YpatiosCh/distributed-systems/projects/distributed-heartbeat/internal/server"
)

func Start(ctx context.Context, cfg *config.Config, srv *server.Server) {
	log := logging.L()
	ticker := time.NewTicker(time.Duration(cfg.PingFreq) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Infow("Monitor stopped.")
			return

		case <-ticker.C:
			for _, peer := range cfg.PeerAddrs {
				go func(p string) {
					pingURL := fmt.Sprintf("%s/ping?from=http://localhost:%s", p, cfg.SelfPort)
					metrics.PingsTotal.Inc() // Count every attempt

					resp, err := http.Get(pingURL)
					if err != nil {
						metrics.PingsFailed.Inc()
						log.Warnw("Failed to ping peer", "peer", p, "error", err)
						return
					}
					metrics.PingsSuccess.Inc()
					resp.Body.Close()
					log.Infow("Pinged peer successfully", "peer", p)
				}(peer)

				lastPing := srv.GetLastPing(peer)
				if lastPing.IsZero() {
					log.Infow("No pings received yet", "peer", peer)
					continue
				}

				secs := time.Since(lastPing).Seconds()
				if secs > float64(cfg.PingTimeout) {
					log.Warnw("Peer appears offline", "peer", peer, "lastPingSecondsAgo", secs)
				} else {
					log.Infow("Peer is online", "peer", peer, "lastPingSecondsAgo", secs)
				}
			}
		}
	}
}
