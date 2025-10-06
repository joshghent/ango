package main

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

var (
	// Request metrics
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ango_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "endpoint", "status"},
	)

	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ango_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	// Code redemption metrics
	codeRedemptionDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "ango_code_redemption_duration_seconds",
			Help:    "Code redemption duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
	)

	codesRedeemed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ango_codes_redeemed_total",
			Help: "Total codes redeemed",
		},
		[]string{"batch_id", "client_id"},
	)

	codeRedemptionErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ango_code_redemption_errors_total",
			Help: "Total code redemption errors",
		},
		[]string{"error_type"},
	)

	preAllocatedCodesUsed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ango_preallocated_codes_used_total",
			Help: "Total pre-allocated codes used",
		},
		[]string{"batch_id", "client_id"},
	)

	// Database metrics
	dbConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ango_db_connections",
			Help: "Current database connections",
		},
		[]string{"state"}, // active, idle, total
	)

	dbQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ango_db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"query_type"},
	)

	// Redis metrics
	redisConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ango_redis_connections",
			Help: "Current Redis connections",
		},
		[]string{"state"},
	)

	cacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ango_cache_hits_total",
			Help: "Total cache hits",
		},
		[]string{"cache_type"}, // batch_rules, prealloc
	)

	cacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ango_cache_misses_total",
			Help: "Total cache misses",
		},
		[]string{"cache_type"},
	)

	// Business metrics
	codesRemaining = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ango_codes_remaining",
			Help: "Codes remaining per batch",
		},
		[]string{"batch_id", "client_id"},
	)

	asyncValidationJobs = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ango_async_validation_jobs",
			Help: "Async validation jobs in queue",
		},
		[]string{"state"}, // queued, processing
	)

	validationResults = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ango_validation_results_total",
			Help: "Async validation results",
		},
		[]string{"result"}, // valid, invalid
	)
)

func init() {
	// Register all metrics
	prometheus.MustRegister(
		requestDuration,
		requestsTotal,
		codeRedemptionDuration,
		codesRedeemed,
		codeRedemptionErrors,
		preAllocatedCodesUsed,
		dbConnections,
		dbQueryDuration,
		redisConnections,
		cacheHits,
		cacheMisses,
		codesRemaining,
		asyncValidationJobs,
		validationResults,
	)
}

// Middleware for request metrics
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		requestDuration.WithLabelValues(c.Request.Method, c.Request.URL.Path, status).Observe(duration)
		requestsTotal.WithLabelValues(c.Request.Method, c.Request.URL.Path, status).Inc()
	}
}

// Start metrics collection goroutines
func StartMetricsCollection(dbPool *pgxpool.Pool, redisClient *redis.Client) {
	// Collect database metrics every 30 seconds
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			collectDatabaseMetrics(dbPool)
			if redisClient != nil {
				collectRedisMetrics(redisClient)
			}
			collectBusinessMetrics(dbPool)
		}
	}()
}

func collectDatabaseMetrics(pool *pgxpool.Pool) {
	stats := pool.Stat()
	dbConnections.WithLabelValues("total").Set(float64(stats.TotalConns()))
	dbConnections.WithLabelValues("active").Set(float64(stats.AcquiredConns()))
	dbConnections.WithLabelValues("idle").Set(float64(stats.IdleConns()))
}

func collectRedisMetrics(client *redis.Client) {
	// Get Redis connection pool stats
	stats := client.PoolStats()
	redisConnections.WithLabelValues("total").Set(float64(stats.TotalConns))
	redisConnections.WithLabelValues("idle").Set(float64(stats.IdleConns))
	redisConnections.WithLabelValues("stale").Set(float64(stats.StaleConns))
}

func collectBusinessMetrics(pool *pgxpool.Pool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Collect codes remaining per batch
	rows, err := pool.Query(ctx, `
		SELECT batch_id, client_id, COUNT(*) as remaining
		FROM codes
		WHERE customer_id IS NULL
		GROUP BY batch_id, client_id
	`)
	if err != nil {
		log.Printf("Error collecting codes remaining metrics: %v", err)
		return
	}
	defer rows.Close()

	// Reset all gauges first (remove stale metrics)
	codesRemaining.Reset()

	for rows.Next() {
		var batchID, clientID string
		var remaining int64
		if err := rows.Scan(&batchID, &clientID, &remaining); err != nil {
			log.Printf("Error scanning codes remaining: %v", err)
			continue
		}
		codesRemaining.WithLabelValues(batchID, clientID).Set(float64(remaining))
	}
}

// Helper functions to record specific metrics
func RecordCodeRedemption(duration time.Duration, batchID, clientID string, preAllocated bool) {
	codeRedemptionDuration.Observe(duration.Seconds())
	codesRedeemed.WithLabelValues(batchID, clientID).Inc()

	if preAllocated {
		preAllocatedCodesUsed.WithLabelValues(batchID, clientID).Inc()
	}
}

func RecordRedemptionError(errorType string) {
	codeRedemptionErrors.WithLabelValues(errorType).Inc()
}

func RecordCacheHit(cacheType string) {
	cacheHits.WithLabelValues(cacheType).Inc()
}

func RecordCacheMiss(cacheType string) {
	cacheMisses.WithLabelValues(cacheType).Inc()
}

func RecordValidationResult(valid bool) {
	result := "valid"
	if !valid {
		result = "invalid"
	}
	validationResults.WithLabelValues(result).Inc()
}

func RecordDatabaseQuery(queryType string, duration time.Duration) {
	dbQueryDuration.WithLabelValues(queryType).Observe(duration.Seconds())
}

// Health check endpoint with dependency checks
func HealthCheckHandler(dbPool *pgxpool.Pool, redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		health := gin.H{
			"status":    "healthy",
			"timestamp": time.Now().UTC(),
			"version":   "1.0.0", // TODO: get from build info
		}

		checks := gin.H{}

		// Database health check
		if err := dbPool.Ping(ctx); err != nil {
			checks["database"] = gin.H{
				"status": "unhealthy",
				"error":  err.Error(),
			}
			health["status"] = "unhealthy"
		} else {
			stats := dbPool.Stat()
			checks["database"] = gin.H{
				"status":      "healthy",
				"connections": stats.TotalConns(),
				"idle":        stats.IdleConns(),
				"active":      stats.AcquiredConns(),
			}
		}

		// Redis health check
		if redisClient != nil {
			if err := redisClient.Ping(ctx).Err(); err != nil {
				checks["redis"] = gin.H{
					"status": "unhealthy",
					"error":  err.Error(),
				}
				// Redis is optional, don't fail the whole service
			} else {
				stats := redisClient.PoolStats()
				checks["redis"] = gin.H{
					"status":      "healthy",
					"connections": stats.TotalConns,
					"idle":        stats.IdleConns,
				}
			}
		} else {
			checks["redis"] = gin.H{
				"status": "disabled",
			}
		}

		health["checks"] = checks

		statusCode := 200
		if health["status"] == "unhealthy" {
			statusCode = 503
		}

		c.JSON(statusCode, health)
	}
}

// Metrics endpoint
func MetricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}