package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
)

var (
	ErrNoCodeFound     = errors.New("no codes were found")
	ErrConditionNotMet = errors.New("the request did not meet the rule conditions defined for the batch")
	ErrNoBatchFound    = errors.New("no batch was found")
	ErrBatchExpired   = errors.New("the batch is expired")
	batchCache         = sync.Map{}       // Cache for storing batch rules
	cacheExpiration    = 15 * time.Minute // Cache expiration time
)

type Rules struct {
	MaxPerCustomer int `json:"maxpercustomer"`
	TimeLimit      int `json:"timelimit"`
}

type CachedRules struct {
	Rules     Rules
	CacheTime time.Time
}

func getCode(ctx context.Context, req Request) (string, error) {
	// Validate UUIDs
	if _, err := uuid.Parse(req.BatchID); err != nil {
		return "", gin.Error{
			Err:  errors.New("invalid batch_id format"),
			Type: gin.ErrorTypePublic,
		}
	}
	if _, err := uuid.Parse(req.ClientID); err != nil {
		return "", gin.Error{
			Err:  errors.New("invalid client_id format"),
			Type: gin.ErrorTypePublic,
		}
	}
	if _, err := uuid.Parse(req.CustomerID); err != nil {
		return "", gin.Error{
			Err:  errors.New("invalid customer_id format"),
			Type: gin.ErrorTypePublic,
		}
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	selectCodeTime := time.Now()

	// First, check if the batch is expired
	var batchExpired bool
	err = tx.QueryRow(ctx, `
		SELECT expired
		FROM batches
		WHERE id = $1
	`, req.BatchID).Scan(&batchExpired)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", ErrNoBatchFound
		}
		return "", err
	}
	if batchExpired {
		return "", ErrBatchExpired
	}

	// If batch is not expired, proceed to select a code
	var code string
	err = tx.QueryRow(ctx, `
		SELECT code
		FROM codes
		WHERE batch_id = $1 AND client_id = $2 AND customer_id IS NULL
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`, req.BatchID, req.ClientID).Scan(&code)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", ErrNoCodeFound
		}
		return "", err
	}

	if time.Since(selectCodeTime) > 100*time.Millisecond {
		log.Printf("Queries for checking batch expiration and selecting code took too long (%v)ms", time.Since(selectCodeTime))
	}

	// Retrieve rules from cache or database
	rules, err := getRulesForBatch(ctx, tx, req.BatchID)
	if err != nil {
		return "", err
	}

	if !checkRules(rules, req.CustomerID) {
		return "", ErrConditionNotMet
	}

	updateCodesTime := time.Now()
	_, err = tx.Exec(ctx, "UPDATE codes SET customer_id=$1 WHERE code=$2", req.CustomerID, code)
	if err != nil {
		return "", err
	}
	if time.Now().Sub(updateCodesTime) > 100*time.Millisecond {
		log.Printf("Query for updating codes took long (%v)ms", time.Now().Sub(updateCodesTime))
	}

	insertCodeUsageTime := time.Now()
	_, err = tx.Exec(ctx, "INSERT INTO code_usage (code, batch_id, client_id, customer_id, used_at) VALUES ($1, $2, $3, $4, $5)", code, req.BatchID, req.ClientID, req.CustomerID, time.Now())
	if err != nil {
		return "", err
	}
	if time.Now().Sub(insertCodeUsageTime) > 100*time.Millisecond {
		log.Printf("Query for inserting codes took long (%v)ms", time.Now().Sub(insertCodeUsageTime))
	}

	if err = tx.Commit(ctx); err != nil {
		return "", err
	}

	return code, nil
}

func getRulesForBatch(ctx context.Context, tx pgx.Tx, batchID string) (Rules, error) {
	// Check cache first
	if cached, found := batchCache.Load(batchID); found {
		cachedRules := cached.(CachedRules)
		// Check if the cache is still valid
		if time.Since(cachedRules.CacheTime) < cacheExpiration {
			return cachedRules.Rules, nil
		}
		// Cache expired, delete it
		batchCache.Delete(batchID)
	}

	// If not in cache or cache expired, fetch from database
	var rules Rules
	err := tx.QueryRow(ctx, "SELECT rules FROM batches WHERE id=$1", batchID).Scan(&rules)
	if err != nil {
		return Rules{}, err
	}

	// Store the fetched rules in cache
	batchCache.Store(batchID, CachedRules{
		Rules:     rules,
		CacheTime: time.Now(),
	})

	return rules, nil
}

func getBatches(ctx context.Context) ([]Batch, error) {
	rows, err := db.Query(ctx, "SELECT id, name, rules, expired FROM batches WHERE expired = false")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var batches []Batch
	for rows.Next() {
		var batch Batch
		err := rows.Scan(&batch.ID, &batch.Name, &batch.Rules, &batch.Expired)
		if err != nil {
			return nil, err
		}
		batches = append(batches, batch)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return batches, nil
}

func createBatch(ctx context.Context, name string, rules string) (string, error) {
	// Generate a new UUID for the batch
	batchID := uuid.New().String()

	// Insert the new batch into the database
	_, err := db.Exec(ctx, "INSERT INTO batches (id, name, rules) VALUES ($1, $2, $3)", batchID, name, rules)
	if err != nil {
		return "", err
	}

	return batchID, nil
}

func uploadCodes(ctx context.Context, file io.Reader, batchID string) error {
	// Create a new CSV reader
	reader := csv.NewReader(file)

	// Read all CSV records
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("error reading CSV: %v", err)
	}

	// Start a transaction
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	// Prepare the bulk insert statement
	stmt := "INSERT INTO codes (client_id, batch_id, code) VALUES "
	var values []interface{}
	for i, record := range records[1:] { // Skip header row
		if len(record) != 3 {
			return fmt.Errorf("invalid record format at row %d", i+2)
		}
		stmt += fmt.Sprintf("($%d, $%d, $%d),", i*3+1, i*3+2, i*3+3)
		values = append(values, record[0], batchID, record[2])
	}
	stmt = stmt[:len(stmt)-1] // Remove the trailing comma

	// Execute the bulk insert
	_, err = tx.Exec(ctx, stmt, values...)
	if err != nil {
		return fmt.Errorf("error executing bulk insert: %v", err)
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	return nil
}

type Rule interface {
	Check(ctx context.Context, customerID string) bool
}

type NoRule struct{}

func (r NoRule) Check(ctx context.Context, customerID string) bool {
	return true
}

type MaxPerCustomerRule struct {
	MaxCount  int
	TimeLimit int // in days, 0 or null means no time limit
}

func (r MaxPerCustomerRule) Check(ctx context.Context, customerID string) bool {
	query := `
		SELECT COUNT(*)
		FROM code_usage
		WHERE customer_id = $1`
	args := []interface{}{customerID}

	if r.TimeLimit > 0 {
		query += ` AND used_at >= $2`
		args = append(args, time.Now().AddDate(0, 0, -r.TimeLimit))
	}

	var count int
	err := db.QueryRow(ctx, query, args...).Scan(&count)

	if err != nil {
		log.Printf("Error checking MaxPerCustomerRule: %v", err)
		return false
	}

	return count < r.MaxCount
}

func checkRules(rules Rules, customerID string) bool {
	ctx := context.Background()

	var ruleCheckers []Rule

	if rules.MaxPerCustomer > 0 {
		ruleCheckers = append(ruleCheckers, MaxPerCustomerRule{
			MaxCount:  rules.MaxPerCustomer,
			TimeLimit: rules.TimeLimit,
		})
	}

	if len(ruleCheckers) == 0 {
		ruleCheckers = append(ruleCheckers, NoRule{})
	}

	for _, rule := range ruleCheckers {
		if !rule.Check(ctx, customerID) {
			return false
		}
	}

	return true
}
