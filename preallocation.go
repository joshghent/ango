package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/redis/go-redis/v9"
)

const (
	PreAllocationThreshold = 10000 // Batches with >10k total codes use pre-allocation
	PreAllocationBuffer    = 1000  // Number of codes to keep pre-allocated
	PreAllocationRefillAt  = 100   // Refill when buffer drops below this
)

type PreAllocator struct {
	redis      *redis.Client
	db         *pgxpool.Pool
	shutdownCh chan struct{}
}

type BatchStats struct {
	TotalCodes     int `json:"total_codes"`
	AvailableCodes int `json:"available_codes"`
	EnablePreAlloc bool `json:"enable_prealloc"`
}

func NewPreAllocator(redisClient *redis.Client, dbPool *pgxpool.Pool) *PreAllocator {
	return &PreAllocator{
		redis:      redisClient,
		db:         dbPool,
		shutdownCh: make(chan struct{}),
	}
}

func (pa *PreAllocator) Start() {
	log.Println("Starting pre-allocator")
	go pa.monitorAndRefill()
}

func (pa *PreAllocator) Stop() {
	close(pa.shutdownCh)
}

func (pa *PreAllocator) monitorAndRefill() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-pa.shutdownCh:
			return
		case <-ticker.C:
			pa.checkAndRefillBatches()
		}
	}
}

func (pa *PreAllocator) checkAndRefillBatches() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get all active batches
	rows, err := pa.db.Query(ctx, `
		SELECT DISTINCT batch_id, client_id
		FROM codes
		WHERE customer_id IS NULL
		GROUP BY batch_id, client_id
		HAVING COUNT(*) > $1
	`, PreAllocationThreshold)
	if err != nil {
		log.Printf("Error querying high-volume batches: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var batchID, clientID string
		if err := rows.Scan(&batchID, &clientID); err != nil {
			log.Printf("Error scanning batch row: %v", err)
			continue
		}

		pa.checkAndRefillBatch(ctx, batchID, clientID)
	}
}

func (pa *PreAllocator) checkAndRefillBatch(ctx context.Context, batchID, clientID string) {
	cacheKey := fmt.Sprintf("prealloc:%s:%s", batchID, clientID)

	// Check current buffer size
	bufferSize, err := pa.redis.ZCard(ctx, cacheKey).Result()
	if err != nil {
		log.Printf("Error checking buffer size for %s:%s: %v", batchID, clientID, err)
		return
	}

	if bufferSize < PreAllocationRefillAt {
		log.Printf("Refilling pre-allocation buffer for batch %s, client %s (current: %d)",
			batchID, clientID, bufferSize)
		pa.refillBuffer(ctx, batchID, clientID, cacheKey)
	}
}

func (pa *PreAllocator) refillBuffer(ctx context.Context, batchID, clientID, cacheKey string) {
	// Get codes from database
	rows, err := pa.db.Query(ctx, `
		SELECT id, code
		FROM codes
		WHERE batch_id = $1 AND client_id = $2 AND customer_id IS NULL
		ORDER BY id
		LIMIT $3
	`, batchID, clientID, PreAllocationBuffer)
	if err != nil {
		log.Printf("Error fetching codes for pre-allocation: %v", err)
		return
	}
	defer rows.Close()

	pipeline := pa.redis.Pipeline()
	count := 0

	for rows.Next() {
		var codeID int
		var code string
		if err := rows.Scan(&codeID, &code); err != nil {
			log.Printf("Error scanning code row: %v", err)
			continue
		}

		// Add to sorted set with timestamp as score for FIFO behavior
		score := float64(time.Now().UnixNano())
		pipeline.ZAdd(ctx, cacheKey, redis.Z{
			Score:  score,
			Member: fmt.Sprintf("%d:%s", codeID, code),
		})
		count++
	}

	// Set expiration on the sorted set
	pipeline.Expire(ctx, cacheKey, 1*time.Hour)

	if count > 0 {
		_, err = pipeline.Exec(ctx)
		if err != nil {
			log.Printf("Error executing pre-allocation pipeline: %v", err)
			return
		}
		log.Printf("Pre-allocated %d codes for batch %s, client %s", count, batchID, clientID)
	}
}

// TryGetPreAllocatedCode attempts to get a code from the pre-allocation cache
func (pa *PreAllocator) TryGetPreAllocatedCode(ctx context.Context, batchID, clientID string) (int, string, bool) {
	if pa.redis == nil {
		return 0, "", false
	}

	cacheKey := fmt.Sprintf("prealloc:%s:%s", batchID, clientID)

	// Pop the oldest code from the sorted set
	result, err := pa.redis.ZPopMin(ctx, cacheKey).Result()
	if err != nil || len(result) == 0 {
		return 0, "", false
	}

	// Parse the code ID and code from the member
	member := result[0].Member.(string)
	parts := parseMember(member)
	if len(parts) != 2 {
		log.Printf("Invalid pre-allocated code format: %s", member)
		return 0, "", false
	}

	codeID, err := strconv.Atoi(parts[0])
	if err != nil {
		log.Printf("Invalid code ID in pre-allocated code: %s", parts[0])
		return 0, "", false
	}

	return codeID, parts[1], true
}

// Helper function to parse the member string "codeID:code"
func parseMember(member string) []string {
	for i, r := range member {
		if r == ':' {
			return []string{member[:i], member[i+1:]}
		}
	}
	return []string{member}
}

// GetBatchStats returns statistics about a batch for monitoring
func (pa *PreAllocator) GetBatchStats(ctx context.Context, batchID, clientID string) BatchStats {
	stats := BatchStats{}

	// Get total codes count
	err := pa.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM codes
		WHERE batch_id = $1 AND client_id = $2
	`, batchID, clientID).Scan(&stats.TotalCodes)
	if err != nil {
		log.Printf("Error getting total codes count: %v", err)
		return stats
	}

	// Get available codes count
	err = pa.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM codes
		WHERE batch_id = $1 AND client_id = $2 AND customer_id IS NULL
	`, batchID, clientID).Scan(&stats.AvailableCodes)
	if err != nil {
		log.Printf("Error getting available codes count: %v", err)
		return stats
	}

	stats.EnablePreAlloc = stats.TotalCodes > PreAllocationThreshold

	return stats
}