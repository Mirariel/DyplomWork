package metrics

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ─── Metric definitions ──────────────────────────────────────────────────────

var (
	// HTTP metrics
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests by method, path and status code.",
	}, []string{"method", "path", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	}, []string{"method", "path"})

	// WebSocket
	WSClientsConnected = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ws_clients_connected",
		Help: "Number of currently connected WebSocket clients.",
	})

	// Trading bots
	ActiveGridBots = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "active_grid_bots_total",
		Help: "Number of running grid bots.",
	})

	ActiveDCABots = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "active_dca_bots_total",
		Help: "Number of running DCA bots.",
	})

	ActiveSmartOrders = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "active_smart_orders_total",
		Help: "Number of active smart orders (not yet triggered or cancelled).",
	})

	// Exchange sync errors
	SyncExchangeErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "sync_exchange_errors_total",
		Help: "Total number of exchange sync errors by exchange, operation and HTTP status.",
	}, []string{"exchange", "operation", "status"})

	// Scheduler
	SchedulerJobDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "scheduler_job_duration_seconds",
		Help:    "Duration of scheduler jobs in seconds.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 15, 30},
	}, []string{"job"})

	SchedulerJobErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "scheduler_job_errors_total",
		Help: "Total number of scheduler job errors.",
	}, []string{"job"})
)

// ─── HTTP middleware ──────────────────────────────────────────────────────────

// Middleware записує HTTP метрики для кожного запиту.
// Шлях нормалізується — параметри (:id) зберігаються як шаблони, не значення.
func Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		method := c.Method()
		// Використовуємо шаблон маршруту, а не конкретний path — щоб уникнути
		// cardinality explosion (один label на /orders/1, /orders/2, ...)
		path := c.Route().Path
		if path == "" {
			path = c.Path()
		}
		status := strconv.Itoa(c.Response().StatusCode())
		duration := time.Since(start).Seconds()

		HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)

		return err
	}
}
