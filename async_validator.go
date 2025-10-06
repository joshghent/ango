package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/redis/go-redis/v9"
)

type AsyncValidator struct {
	redis      *redis.Client
	db         *pgxpool.Pool
	jobQueue   chan ValidationJob
	workers    int
	shutdownCh chan struct{}
}

type ValidationJob struct {
	CodeID     int    `json:"code_id"`
	Code       string `json:"code"`
	BatchID    string `json:"batch_id"`
	ClientID   string `json:"client_id"`
	CustomerID string `json:"customer_id"`
	Rules      Rules  `json:"rules"`
	Timestamp  time.Time `json:"timestamp"`
}

type ValidationResult struct {
	Valid      bool   `json:"valid"`
	Reason     string `json:"reason,omitempty"`
	ProcessedAt time.Time `json:"processed_at"`
}

func NewAsyncValidator(redisClient *redis.Client, dbPool *pgxpool.Pool, workers int) *AsyncValidator {
	return &AsyncValidator{
		redis:      redisClient,
		db:         dbPool,
		jobQueue:   make(chan ValidationJob, 1000), // Buffer for 1000 jobs
		workers:    workers,
		shutdownCh: make(chan struct{}),
	}
}

func (av *AsyncValidator) Start() {
	log.Printf("Starting async validator with %d workers", av.workers)

	// Start worker goroutines
	for i := 0; i < av.workers; i++ {
		go av.worker(i)
	}

	// Start Redis job processor
	go av.processRedisJobs()
}

func (av *AsyncValidator) Stop() {
	close(av.shutdownCh)
	close(av.jobQueue)
}

func (av *AsyncValidator) QueueValidation(job ValidationJob) error {
	// Try to queue in memory first (fast path)
	select {
	case av.jobQueue <- job:
		return nil
	default:
		// Memory queue full, fallback to Redis
		return av.queueToRedis(job)
	}
}

func (av *AsyncValidator) queueToRedis(job ValidationJob) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	jobBytes, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal validation job: %w", err)
	}

	return av.redis.LPush(ctx, "validation_jobs", jobBytes).Err()
}

func (av *AsyncValidator) processRedisJobs() {
	for {
		select {
		case <-av.shutdownCh:
			return
		default:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			result, err := av.redis.BRPop(ctx, 1*time.Second, "validation_jobs").Result()
			cancel()

			if err == redis.Nil {
				continue // No jobs available
			}
			if err != nil {
				log.Printf("Error reading from Redis validation queue: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			var job ValidationJob
			if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
				log.Printf("Error unmarshaling validation job: %v", err)
				continue
			}

			// Try to queue to memory, otherwise process directly
			select {
			case av.jobQueue <- job:
			default:
				go av.processValidationJob(job)
			}
		}
	}
}

func (av *AsyncValidator) worker(workerID int) {
	log.Printf("Starting validation worker %d", workerID)

	for {
		select {
		case <-av.shutdownCh:
			log.Printf("Shutting down validation worker %d", workerID)
			return
		case job, ok := <-av.jobQueue:
			if !ok {
				return
			}
			av.processValidationJob(job)
		}
	}
}

func (av *AsyncValidator) processValidationJob(job ValidationJob) {
	start := time.Now()

	// Check if job is too old (older than 30 seconds)
	if time.Since(job.Timestamp) > 30*time.Second {
		log.Printf("Skipping stale validation job for code %s (age: %v)", job.Code, time.Since(job.Timestamp))
		return
	}

	// Validate rules
	valid := av.validateRules(job.Rules, job.CustomerID)

	result := ValidationResult{
		Valid:       valid,
		ProcessedAt: time.Now(),
	}

	if !valid {
		result.Reason = "Rule validation failed"
		// Mark code as invalid and notify
		av.invalidateCode(job.CodeID, job.Code, "Rule validation failed")
	}

	// Store result in Redis for potential querying
	av.storeValidationResult(job.Code, result)

	duration := time.Since(start)
	if duration > 100*time.Millisecond {
		log.Printf("Validation job for code %s took %v", job.Code, duration)
	}
}

func (av *AsyncValidator) validateRules(rules Rules, customerID string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use the existing checkRules function but with proper context handling
	return checkRulesWithContext(ctx, av.db, rules, customerID)
}

func (av *AsyncValidator) invalidateCode(codeID int, code string, reason string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Mark code as invalid
	_, err := av.db.Exec(ctx,
		"UPDATE codes SET customer_id = NULL, used_at = NULL, invalid_reason = $1 WHERE id = $2",
		reason, codeID)
	if err != nil {
		log.Printf("Failed to invalidate code %s: %v", code, err)
		return
	}

	log.Printf("Code %s invalidated: %s", code, reason)

	// TODO: Notify customer about invalid code redemption
	// This could be via webhook, email, or other notification system
}

func (av *AsyncValidator) storeValidationResult(code string, result ValidationResult) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultBytes, err := json.Marshal(result)
	if err != nil {
		log.Printf("Failed to marshal validation result for code %s: %v", code, err)
		return
	}

	// Store with 1 hour expiration
	err = av.redis.SetEx(ctx, fmt.Sprintf("validation_result:%s", code), resultBytes, time.Hour).Err()
	if err != nil {
		log.Printf("Failed to store validation result for code %s: %v", code, err)
	}
}

// Enhanced rule checking with context and proper database connection
func checkRulesWithContext(ctx context.Context, db *pgxpool.Pool, rules Rules, customerID string) bool {
	if rules.MaxPerCustomer <= 0 {
		return true // No rules to check
	}

	query := `
		SELECT COUNT(*)
		FROM codes
		WHERE customer_id = $1 AND used_at IS NOT NULL`
	args := []interface{}{customerID}

	if rules.TimeLimit > 0 {
		query += ` AND used_at >= $2`
		args = append(args, time.Now().AddDate(0, 0, -rules.TimeLimit))
	}

	var count int
	err := db.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		log.Printf("Error checking rules for customer %s: %v", customerID, err)
		return false // Fail safe - deny if we can't check
	}

	return count < rules.MaxPerCustomer
}