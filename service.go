package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
)

var (
	ErrNoCodeFound     = errors.New("no code found")
	ErrConditionNotMet = errors.New("rule conditions not met")
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

func getCodeWithTimeout(ctx context.Context, req Request) (string, error) {
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

	// Create a new context with a timeout
	// ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	// defer cancel()

	tx, err := db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var code string
	selectCodeTime := time.Now()
	err = tx.QueryRow(ctx, "SELECT code FROM codes WHERE batch_id=$1 AND client_id=$2 AND customer_id IS NULL FOR UPDATE SKIP LOCKED LIMIT 1", req.BatchID, req.ClientID).Scan(&code)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", ErrNoCodeFound
		}
		return "", err
	}
	if time.Now().Sub(selectCodeTime) > 100*time.Millisecond {
		fmt.Printf("Query for selecting codes took too long (%v)ms", time.Now().Sub(selectCodeTime))
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
		fmt.Printf("Query for updating codes took long (%v)ms", time.Now().Sub(updateCodesTime))
	}

	insertCodeUsageTime := time.Now()
	_, err = tx.Exec(ctx, "INSERT INTO code_usage (code, batch_id, client_id, customer_id, used_at) VALUES ($1, $2, $3, $4, $5)", code, req.BatchID, req.ClientID, req.CustomerID, time.Now())
	if err != nil {
		return "", err
	}
	if time.Now().Sub(insertCodeUsageTime) > 100*time.Millisecond {
		fmt.Printf("Query for inserting codes took long (%v)ms", time.Now().Sub(insertCodeUsageTime))
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
