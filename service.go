package main

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v4"
)

var (
	ErrNoCodeFound     = errors.New("no code found")
	ErrConditionNotMet = errors.New("rule conditions not met")
)

type Rules struct {
	MaxPerCustomer int `json:"maxpercustomer"`
	TimeLimit      int `json:"timelimit"`
}

func getCode(ctx context.Context, req Request) (string, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var code string
	err = tx.QueryRow(ctx, "SELECT code FROM codes WHERE batch_id=$1 AND client_id=$2 AND customer_id IS NULL FOR UPDATE SKIP LOCKED LIMIT 1", req.BatchID, req.ClientID).Scan(&code)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", ErrNoCodeFound
		}
		return "", err
	}

	var rules Rules
	err = tx.QueryRow(ctx, "SELECT rules FROM batches WHERE id=$1", req.BatchID).Scan(&rules)
	if err != nil {
		return "", err
	}

	if !checkRules(ctx, rules, req.CustomerID) {
		return "", ErrConditionNotMet
	}

	_, err = tx.Exec(ctx, "UPDATE codes SET customer_id=$1 WHERE code=$2", req.CustomerID, code)
	if err != nil {
		return "", err
	}

	_, err = tx.Exec(ctx, "INSERT INTO code_usage (code, batch_id, client_id, customer_id, used_at) VALUES ($1, $2, $3, $4, $5)", code, req.BatchID, req.ClientID, req.CustomerID, time.Now())
	if err != nil {
		return "", err
	}

	if err = tx.Commit(ctx); err != nil {
		return "", err
	}

	return code, nil
}

func checkRules(ctx context.Context, rules Rules, customerID string) bool {
	// Implement the actual logic to check maxpercustomer and timelimit
	// This is a placeholder implementation
	return true
}
