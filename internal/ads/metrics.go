package ads

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	adsRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ads_requests_total",
			Help: "Total number of GET /ads requests",
		},
		[]string{"status"},
	)

	adsQueryDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "ads_db_query_duration_seconds",
			Help:    "Duration of ads DB query in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	registerOnce sync.Once
)

// InitAdMetrics registers ads-related Prometheus metrics (safe to call multiple times).
func InitAdMetrics() {
	registerOnce.Do(func() {
		prometheus.MustRegister(adsRequestCounter, adsQueryDuration)
	})
}
