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
	Expired   bool
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

    ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
    defer cancel()

    // Check batch expiration from cache
    rules, batchExpired, err := getRulesForBatch(ctx, req.BatchID)
    if err != nil {
        if err == pgx.ErrNoRows {
            return "", ErrNoBatchFound
        }
        return "", err
    }
    if batchExpired {
        return "", ErrBatchExpired
    }

    // Begin transaction after initial check
    tx, err := db.BeginTx(ctx, pgx.TxOptions{})
    if err != nil {
        return "", err
    }
    defer func() {
        if tx != nil {
            tx.Rollback(ctx) // Ensure rollback if not committed
        }
    }()

    selectCodeTime := time.Now()

    // Attempt to acquire a code
    var code string
    err = tx.QueryRow(ctx, `
        SELECT code
        FROM codes
        WHERE batch_id = $1 AND client_id = $2 AND customer_id IS NULL
        FOR NO KEY UPDATE SKIP LOCKED
        LIMIT 1
    `, req.BatchID, req.ClientID).Scan(&code)
    if err != nil {
        if err == pgx.ErrNoRows {
            return "", ErrNoCodeFound
        }
        return "", err
    }

    if time.Since(selectCodeTime) > 100*time.Millisecond {
        log.Printf("Queries for selecting code took too long (%v)ms", time.Since(selectCodeTime))
    }

    if !checkRules(rules, req.CustomerID) {
        return "", ErrConditionNotMet
    }

    // Code usage updates
    updateCodesTime := time.Now()
    _, err = tx.Exec(ctx, "UPDATE codes SET customer_id=$1 WHERE code=$2", req.CustomerID, code)
    if err != nil {
        return "", err
    }
    if time.Since(updateCodesTime) > 100*time.Millisecond {
        log.Printf("Query for updating codes took too long (%v)ms", time.Since(updateCodesTime))
    }

		// Remove code usage because it's not needed, will queue for later.
    // insertCodeUsageTime := time.Now()
    // _, err = tx.Exec(ctx, "INSERT INTO code_usage (code, batch_id, client_id, customer_id, used_at) VALUES ($1, $2, $3, $4, $5)", code, req.BatchID, req.ClientID, req.CustomerID, time.Now())
    // if err != nil {
    //     return "", err
    // }
    // if time.Since(insertCodeUsageTime) > 100*time.Millisecond {
    //     log.Printf("Query for inserting code usage took too long (%v)ms", time.Since(insertCodeUsageTime))
    // }

    if err = tx.Commit(ctx); err != nil {
        return "", err
    }
    tx = nil // Avoid rollback

    return code, nil
}


func getRulesForBatch(ctx context.Context, batchID string) (Rules, bool, error) {
	// Check cache first
	if cached, found := batchCache.Load(batchID); found {
		cachedRules := cached.(CachedRules)
		// Check if the cache is still valid
		if time.Since(cachedRules.CacheTime) < cacheExpiration {
			return cachedRules.Rules, cachedRules.Expired, nil
		}
		// Cache expired, delete it
		batchCache.Delete(batchID)
	}

	// If not in cache or cache expired, fetch from database
	var rules Rules
	var expired bool
	err := db.QueryRow(ctx, "SELECT rules, expired FROM batches WHERE id=$1", batchID).Scan(&rules, &expired)
	if err != nil {
		return Rules{}, false, err
	}

	// Store the fetched rules in cache
	batchCache.Store(batchID, CachedRules{
		Rules:     rules,
		Expired:   expired,
		CacheTime: time.Now(),
	})

	return rules, expired, nil
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
		values = append(values, record[0], batchID, record[1])
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
