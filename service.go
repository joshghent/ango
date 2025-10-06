package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
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

    selectCodeTime := time.Now()

    // Try pre-allocated codes first for high-volume batches
    var code string
    var codeID int
    var gotPreAllocated bool

    if preAllocator != nil {
        codeID, code, gotPreAllocated = preAllocator.TryGetPreAllocatedCode(ctx, req.BatchID, req.ClientID)
        if gotPreAllocated {
            // Begin transaction to claim the pre-allocated code
            tx, err := db.BeginTx(ctx, pgx.TxOptions{})
            if err != nil {
                return "", err
            }
            defer func() {
                if tx != nil {
                    tx.Rollback(ctx)
                }
            }()

            // Update the pre-allocated code
            result, err := tx.Exec(ctx, `
                UPDATE codes
                SET customer_id = $1, used_at = NOW()
                WHERE id = $2 AND customer_id IS NULL
            `, req.CustomerID, codeID)
            if err != nil {
                return "", err
            }

            // Check if we actually updated a row (code might have been taken by another request)
            if result.RowsAffected() == 0 {
                // Code was already taken, fall back to normal allocation
                gotPreAllocated = false
            } else {
                // Successfully claimed pre-allocated code
                if err = tx.Commit(ctx); err != nil {
                    return "", err
                }
                tx = nil
                log.Printf("Used pre-allocated code %s for batch %s", code, req.BatchID)
            }
        }
    }

    // Fall back to normal atomic allocation if pre-allocation failed
    if !gotPreAllocated {
        tx, err := db.BeginTx(ctx, pgx.TxOptions{})
        if err != nil {
            return "", err
        }
        defer func() {
            if tx != nil {
                tx.Rollback(ctx) // Ensure rollback if not committed
            }
        }()

        // Attempt to acquire and update a code atomically
        err = tx.QueryRow(ctx, `
            UPDATE codes
            SET customer_id = $3, used_at = NOW()
            WHERE id = (
                SELECT id FROM codes
                WHERE batch_id = $1 AND client_id = $2 AND customer_id IS NULL
                LIMIT 1
                FOR UPDATE SKIP LOCKED
            )
            RETURNING code, id
        `, req.BatchID, req.ClientID, req.CustomerID).Scan(&code, &codeID)
        if err != nil {
            if err == pgx.ErrNoRows {
                return "", ErrNoCodeFound
            }
            return "", err
        }

        if err = tx.Commit(ctx); err != nil {
            return "", err
        }
        tx = nil
    }

    if time.Since(selectCodeTime) > 50*time.Millisecond {
        log.Printf("Code acquisition took too long (%v)ms", time.Since(selectCodeTime))
    }

    // For immediate response, do basic rule checking synchronously
    // More complex validation will be done asynchronously
    if rules.MaxPerCustomer > 0 {
        // Quick check - only fail if customer has obviously exceeded limits
        count, err := getCustomerCodeCount(ctx, req.CustomerID, rules.TimeLimit)
        if err != nil {
            log.Printf("Error doing quick rule check: %v", err)
        } else {
            log.Printf("Debug: Customer %s has %d used codes, max allowed: %d", req.CustomerID, count, rules.MaxPerCustomer)
            if count >= rules.MaxPerCustomer {
                // Clear violation, rollback the code allocation
                _, rollbackErr := db.Exec(ctx, "UPDATE codes SET customer_id = NULL, used_at = NULL WHERE id = $1", codeID)
                if rollbackErr != nil {
                    log.Printf("Failed to rollback code allocation: %v", rollbackErr)
                }
                return "", ErrConditionNotMet
            }
        }
    }

    // Queue detailed async validation
    if asyncValidator != nil {
        validationJob := ValidationJob{
            CodeID:     codeID,
            Code:       code,
            BatchID:    req.BatchID,
            ClientID:   req.ClientID,
            CustomerID: req.CustomerID,
            Rules:      rules,
            Timestamp:  time.Now(),
        }
        if err := asyncValidator.QueueValidation(validationJob); err != nil {
            log.Printf("Failed to queue async validation for code %s: %v", code, err)
        }
    }

    return code, nil
}


func getRulesForBatch(ctx context.Context, batchID string) (Rules, bool, error) {
	// Try Redis cache first
	if redisClient != nil {
		cacheKey := fmt.Sprintf("batch_rules:%s", batchID)
		cached, err := redisClient.Get(ctx, cacheKey).Result()
		if err == nil {
			var cachedRules CachedRules
			if json.Unmarshal([]byte(cached), &cachedRules) == nil {
				return cachedRules.Rules, cachedRules.Expired, nil
			}
		}
	}

	// Fallback to in-memory cache
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

	cachedRules := CachedRules{
		Rules:     rules,
		Expired:   expired,
		CacheTime: time.Now(),
	}

	// Store in both Redis and in-memory cache
	if redisClient != nil {
		cacheKey := fmt.Sprintf("batch_rules:%s", batchID)
		if rulesBytes, err := json.Marshal(cachedRules); err == nil {
			redisClient.SetEx(ctx, cacheKey, rulesBytes, cacheExpiration)
		}
	}

	batchCache.Store(batchID, cachedRules)

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

func getCustomerCodeCount(ctx context.Context, customerID string, timeLimit int) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM codes
		WHERE customer_id = $1 AND used_at IS NOT NULL`
	args := []interface{}{customerID}

	if timeLimit > 0 {
		query += ` AND used_at >= $2`
		args = append(args, time.Now().AddDate(0, 0, -timeLimit))
	}

	var count int
	err := db.QueryRow(ctx, query, args...).Scan(&count)
	return count, err
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
