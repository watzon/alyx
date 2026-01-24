package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alyx_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "alyx_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "alyx_http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed",
		},
	)

	httpResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "alyx_http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000, 10000000},
		},
		[]string{"method", "path"},
	)

	dbConnectionsOpen = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "alyx_db_connections_open",
			Help: "Number of open database connections",
		},
	)

	dbConnectionsInUse = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "alyx_db_connections_in_use",
			Help: "Number of database connections currently in use",
		},
	)

	dbConnectionsIdle = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "alyx_db_connections_idle",
			Help: "Number of idle database connections",
		},
	)

	realtimeConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "alyx_realtime_connections",
			Help: "Number of active WebSocket connections",
		},
	)

	realtimeSubscriptions = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "alyx_realtime_subscriptions",
			Help: "Number of active subscriptions",
		},
	)

	functionInvocations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alyx_function_invocations_total",
			Help: "Total number of function invocations",
		},
		[]string{"function", "runtime", "status"},
	)

	functionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "alyx_function_duration_seconds",
			Help:    "Function execution time in seconds",
			Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60},
		},
		[]string{"function", "runtime"},
	)

	functionPoolSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "alyx_function_pool_size",
			Help: "Number of containers in the function pool",
		},
		[]string{"runtime", "state"},
	)
)

func Handler() http.Handler {
	return promhttp.Handler()
}

func RecordHTTPRequest(method, path string, status int, duration time.Duration, responseSize int) {
	statusStr := strconv.Itoa(status)
	httpRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
	httpRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
	httpResponseSize.WithLabelValues(method, path).Observe(float64(responseSize))
}

func IncrementInFlight() {
	httpRequestsInFlight.Inc()
}

func DecrementInFlight() {
	httpRequestsInFlight.Dec()
}

func UpdateDBStats(open, inUse, idle int) {
	dbConnectionsOpen.Set(float64(open))
	dbConnectionsInUse.Set(float64(inUse))
	dbConnectionsIdle.Set(float64(idle))
}

func UpdateRealtimeStats(connections, subscriptions int) {
	realtimeConnections.Set(float64(connections))
	realtimeSubscriptions.Set(float64(subscriptions))
}

func RecordFunctionInvocation(name, runtime, status string, duration time.Duration) {
	functionInvocations.WithLabelValues(name, runtime, status).Inc()
	functionDuration.WithLabelValues(name, runtime).Observe(duration.Seconds())
}

func UpdateFunctionPoolStats(runtime string, ready, busy int) {
	functionPoolSize.WithLabelValues(runtime, "ready").Set(float64(ready))
	functionPoolSize.WithLabelValues(runtime, "busy").Set(float64(busy))
}

func NormalizePath(path string) string {
	if len(path) > 100 {
		path = path[:100]
	}

	normalized := ""
	inParam := false
	for i := 0; i < len(path); i++ {
		if path[i] == '{' {
			inParam = true
			normalized += ":"
			continue
		}
		if path[i] == '}' {
			inParam = false
			continue
		}
		if !inParam {
			normalized += string(path[i])
		}
	}
	return normalized
}
