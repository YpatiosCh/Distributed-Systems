package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	PingsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "heartbeat_pings_total",
		Help: "Total number of pings sent",
	})

	PingsSuccess = promauto.NewCounter(prometheus.CounterOpts{
		Name: "heartbeat_pings_success_total",
		Help: "Total number of successful pings",
	})

	PingsFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "heartbeat_pings_failed_total",
		Help: "Total number of failed pings",
	})
)

func StartMetricsServer(port string) {
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":"+port, nil)
}
