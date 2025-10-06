package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/sony/gobreaker"
)

var (
	ErrCircuitBreakerOpen = errors.New("circuit breaker is open")
	ErrServiceUnavailable = errors.New("service temporarily unavailable")
)

type CircuitBreakerManager struct {
	dbCircuitBreaker    *gobreaker.CircuitBreaker
	redisCircuitBreaker *gobreaker.CircuitBreaker
}

func NewCircuitBreakerManager() *CircuitBreakerManager {
	dbSettings := gobreaker.Settings{
		Name:        "database",
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Trip if failure rate is > 50% and we have at least 5 requests
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 5 && failureRatio >= 0.5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Printf("Circuit breaker '%s' changed from '%s' to '%s'", name, from, to)
		},
		IsSuccessful: func(err error) bool {
			// Context timeout/cancellation should not trip the breaker
			return err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
		},
	}

	redisSettings := gobreaker.Settings{
		Name:        "redis",
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     15 * time.Second, // Shorter timeout for Redis
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// More aggressive for Redis since it's optional
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Printf("Circuit breaker '%s' changed from '%s' to '%s'", name, from, to)
		},
		IsSuccessful: func(err error) bool {
			return err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
		},
	}

	return &CircuitBreakerManager{
		dbCircuitBreaker:    gobreaker.NewCircuitBreaker(dbSettings),
		redisCircuitBreaker: gobreaker.NewCircuitBreaker(redisSettings),
	}
}

func (cbm *CircuitBreakerManager) ExecuteDB(fn func() error) error {
	_, err := cbm.dbCircuitBreaker.Execute(func() (interface{}, error) {
		return nil, fn()
	})
	return err
}

func (cbm *CircuitBreakerManager) ExecuteRedis(fn func() error) error {
	_, err := cbm.redisCircuitBreaker.Execute(func() (interface{}, error) {
		return nil, fn()
	})
	return err
}

func (cbm *CircuitBreakerManager) GetState() gin.H {
	return gin.H{
		"database": gin.H{
			"state":  cbm.dbCircuitBreaker.State().String(),
			"counts": cbm.dbCircuitBreaker.Counts(),
		},
		"redis": gin.H{
			"state":  cbm.redisCircuitBreaker.State().String(),
			"counts": cbm.redisCircuitBreaker.Counts(),
		},
	}
}

// Middleware to handle circuit breaker errors gracefully
func CircuitBreakerMiddleware(cbm *CircuitBreakerManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if database circuit breaker is open
		if cbm.dbCircuitBreaker.State() == gobreaker.StateOpen {
			log.Println("Database circuit breaker is open, rejecting request")
			c.JSON(503, gin.H{
				"error":   "Service temporarily unavailable",
				"message": "Database is experiencing issues. Please try again later.",
				"retry_after": 30, // seconds
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Enhanced database operations with circuit breaker
func (cbm *CircuitBreakerManager) GetCodeWithBreaker(ctx context.Context, req Request) (string, error) {
	var code string
	var err error

	dbErr := cbm.ExecuteDB(func() error {
		code, err = getCode(ctx, req)
		return err
	})

	if dbErr != nil {
		if errors.Is(dbErr, ErrCircuitBreakerOpen) || dbErr == gobreaker.ErrOpenState {
			RecordRedemptionError("circuit_breaker_open")
			return "", ErrServiceUnavailable
		}
		return "", dbErr
	}

	return code, err
}

// Enhanced Redis operations with circuit breaker
func (cbm *CircuitBreakerManager) GetRulesWithBreaker(ctx context.Context, batchID string) (Rules, bool, error) {
	var rules Rules
	var expired bool
	var err error

	// Try Redis first with circuit breaker
	if redisClient != nil {
		redisErr := cbm.ExecuteRedis(func() error {
			cacheKey := fmt.Sprintf("batch_rules:%s", batchID)
			cached, redisErr := redisClient.Get(ctx, cacheKey).Result()
			if redisErr != nil {
				RecordCacheMiss("batch_rules")
				return redisErr
			}

			var cachedRules CachedRules
			if unmarshalErr := json.Unmarshal([]byte(cached), &cachedRules); unmarshalErr != nil {
				return unmarshalErr
			}

			RecordCacheHit("batch_rules")
			rules = cachedRules.Rules
			expired = cachedRules.Expired
			return nil
		})

		if redisErr == nil {
			return rules, expired, nil
		}

		// Log Redis circuit breaker issues but continue
		if errors.Is(redisErr, gobreaker.ErrOpenState) {
			log.Printf("Redis circuit breaker is open, falling back to database")
		}
	}

	// Fallback to database with circuit breaker
	dbErr := cbm.ExecuteDB(func() error {
		rules, expired, err = getRulesForBatch(ctx, batchID)
		return err
	})

	if dbErr != nil {
		if errors.Is(dbErr, gobreaker.ErrOpenState) {
			return Rules{}, false, ErrServiceUnavailable
		}
		return Rules{}, false, dbErr
	}

	return rules, expired, err
}

// Request timeout middleware
func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)

		finished := make(chan struct{})
		go func() {
			c.Next()
			finished <- struct{}{}
		}()

		select {
		case <-finished:
			// Request completed normally
		case <-ctx.Done():
			// Request timed out
			if ctx.Err() == context.DeadlineExceeded {
				log.Printf("Request timed out after %v: %s %s", timeout, c.Request.Method, c.Request.URL.Path)
				RecordRedemptionError("request_timeout")
				c.JSON(408, gin.H{
					"error":   "Request timeout",
					"message": "Request took too long to process",
				})
				c.Abort()
			}
		}
	}
}

// Exponential backoff retry utility
func RetryWithBackoff(operation func() error, maxRetries int, baseDelay time.Duration) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = operation()
		if err == nil {
			return nil
		}

		// Don't retry on certain errors
		if errors.Is(err, ErrNoCodeFound) || errors.Is(err, ErrConditionNotMet) {
			return err
		}

		if i < maxRetries-1 {
			delay := baseDelay * time.Duration(1<<uint(i)) // Exponential backoff
			time.Sleep(delay)
		}
	}
	return fmt.Errorf("operation failed after %d retries: %w", maxRetries, err)
}

// Bulkhead pattern: separate connection pools for different operations
type ConnectionPools struct {
	ReadPool  *pgxpool.Pool
	WritePool *pgxpool.Pool
}

func NewConnectionPools(databaseURL string) (*ConnectionPools, error) {
	// Read pool - optimized for queries
	readConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse read config: %w", err)
	}

	readConfig.MaxConns = 50
	readConfig.MinConns = 10
	readConfig.MaxConnIdleTime = 10 * time.Minute
	readConfig.MaxConnLifetime = 30 * time.Minute

	readPool, err := pgxpool.ConnectConfig(context.Background(), readConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create read pool: %w", err)
	}

	// Write pool - optimized for transactions
	writeConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		readPool.Close()
		return nil, fmt.Errorf("failed to parse write config: %w", err)
	}

	writeConfig.MaxConns = 25
	writeConfig.MinConns = 5
	writeConfig.MaxConnIdleTime = 5 * time.Minute
	writeConfig.MaxConnLifetime = 15 * time.Minute

	writePool, err := pgxpool.ConnectConfig(context.Background(), writeConfig)
	if err != nil {
		readPool.Close()
		return nil, fmt.Errorf("failed to create write pool: %w", err)
	}

	return &ConnectionPools{
		ReadPool:  readPool,
		WritePool: writePool,
	}, nil
}

func (cp *ConnectionPools) Close() {
	cp.ReadPool.Close()
	cp.WritePool.Close()
}