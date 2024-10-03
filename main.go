package main

import (
	"context"
	"log"
	"net/http"
	_ "net/http/pprof" // Register pprof handlers
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
)

var db *pgxpool.Pool

func main() {
	log.SetOutput(os.Stdout)
	// Start pprof for profiling in a separate goroutine
	go func() {
		log.Println("Starting pprof on :6060")
		http.ListenAndServe(":6060", nil)
	}()

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
		config, _ := pgxpool.ParseConfig(databaseURL)
		config.MaxConns = 20 // Adjust based on expected workload
		config.MaxConnIdleTime = 30 * time.Minute
		config.MaxConnLifetime = 2 * time.Hour

		db, err = pgxpool.ConnectConfig(context.Background(), config)
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
			log.Printf("Error: %v", err)
			c.JSON(500, gin.H{"error": "database error"})
		}
		return
	}

	c.JSON(200, Code{Code: code})
}

func checkRules(rules Rules, customerID string) bool {
	ctx := context.Background()

	// Calculate the start date for the time limit
	startDate := time.Now().AddDate(0, 0, -rules.TimeLimit)

	// Query to count the number of codes used by the customer within the time limit
	var count int
	err := db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM code_usage
		WHERE customer_id = $1 AND used_at >= $2
	`, customerID, startDate).Scan(&count)

	if err != nil {
		log.Printf("Error checking rules: %v", err)
		return false
	}

	// Check if the count is less than the maximum allowed per customer
	return count < rules.MaxPerCustomer
}
