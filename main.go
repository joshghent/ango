package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
)

var db *pgxpool.Pool

func main() {
	var err error
	db, err = connectToDB()
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer db.Close()
	log.Println("Connected to the database successfully.")

	r := gin.Default()

	r.POST("/api/get-code", getCodeHandler)

	if err := r.Run(":3000"); err != nil {
		log.Fatalf("Unable to start server: %v\n", err)
	}
}

type Request struct {
	BatchID    string `json:"batchid"`
	ClientID   string `json:"clientid"`   // this is the client identifier that the codes are tied to
	CustomerID string `json:"customerid"` // this is the external systems customerId. Provided from your systems when making the request
}

type Code struct {
	Code string `json:"code"`
}

func connectToDB() (*pgxpool.Pool, error) {
	var db *pgxpool.Pool
	var err error
	maxRetries := 5
	databaseURL := os.Getenv("DATABASE_URL")

	for i := 0; i < maxRetries; i++ {
		db, err = pgxpool.Connect(context.Background(), databaseURL)
		if err == nil {
			// Test the connection by querying the "batches" table
			err = testDBConnection(db)
			if err == nil {
				break
			}
		}
		log.Printf("Error connecting to database (attempt %d/%d): %v\nDatabase URL: %s", i+1, maxRetries, err, databaseURL)
		time.Sleep(5 * time.Second)
	}

	return db, err
}

func testDBConnection(db *pgxpool.Pool) error {
	// Try to query the "batches" table to ensure the connection is working and permissions are sufficient
	var testResult int
	err := db.QueryRow(context.Background(), "SELECT 1 FROM batches LIMIT 1").Scan(&testResult)
	if err != nil {
		return err
	}
	return nil
}

func getCodeHandler(c *gin.Context) {
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "cannot parse json"})
		return
	}

	// Validate UUIDs immediately after parsing JSON
	if _, err := uuid.Parse(req.BatchID); err != nil {
		c.JSON(400, gin.H{"error": "invalid batch_id format"})
		return
	}
	if _, err := uuid.Parse(req.ClientID); err != nil {
		c.JSON(400, gin.H{"error": "invalid client_id format"})
		return
	}
	if _, err := uuid.Parse(req.CustomerID); err != nil {
		c.JSON(400, gin.H{"error": "invalid customer_id format"})
		return
	}

	code, err := getCodeWithTimeout(context.Background(), req)
	if err != nil {
		if err == ErrNoCodeFound {
			c.JSON(404, gin.H{"error": "no code found"})
		} else if err == ErrConditionNotMet {
			c.JSON(403, gin.H{"error": "rule conditions not met"})
		} else {
			fmt.Printf("Error: %e", err)
			c.JSON(500, gin.H{"error": "database error"})
		}
		return
	}

	c.JSON(200, Code{Code: code})
}

func checkRules(rules Rules, customerID string) bool {
	// Implement logic to check maxpercustomer and time limit
	return true
}
