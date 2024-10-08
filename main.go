package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"

	// "net/http"
	// _ "net/http/pprof" // Register pprof handlers
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
)

var db *pgxpool.Pool

func main() {
	log.SetOutput(os.Stdout)
	// go func() {
	// 	log.Println("Starting pprof on :6060")
	// 	http.ListenAndServe(":6060", nil)
	// }()

	var err error
	db, err = connectToDB()
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer db.Close()
	log.Println("Connected to the database successfully.")

	go monitorDBConnections(db)

	r := gin.Default()

	r.GET("/healthcheck", healthcheckHandler)
	r.POST("/api/v1/code/redeem", getCodeHandler)
	r.GET("/api/v1/batches", getBatchesHandler)
	r.POST("/api/v1/codes/upload", uploadCodesHandler)

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

type Batch struct {
	ID string `json:"id"`
	Name string `json:"name"`
	Rules Rules `json:"rules"`
	Expired bool `json:"expired"`
}

func connectToDB() (*pgxpool.Pool, error) {
	var db *pgxpool.Pool
	var err error
	maxRetries := 5
	databaseURL := os.Getenv("DATABASE_URL")

	for i := 0; i < maxRetries; i++ {
		config, _ := pgxpool.ParseConfig(databaseURL)
		config.MaxConns = 50
		config.MaxConnIdleTime = 30 * time.Second
		config.MaxConnLifetime = 1 * time.Hour
		config.HealthCheckPeriod = 1 * time.Minute
		config.ConnConfig.ConnectTimeout = 5 * time.Second

		db, err = pgxpool.ConnectConfig(context.Background(), config)
		if err == nil {
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

func monitorDBConnections(pool *pgxpool.Pool) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := pool.Stat()
		log.Printf("DB Pool Stats - Total: %d, Idle: %d, In Use: %d, Max: %d",
			stats.TotalConns(), stats.IdleConns(), stats.AcquiredConns(), stats.MaxConns())

		// Check and reset stalled connections with extended timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := pool.AcquireFunc(ctx, func(conn *pgxpool.Conn) error {
			_, err := conn.Exec(ctx, "SELECT 1")
			if err != nil {
				log.Printf("Resetting stalled connection: %v", err)
				conn.Hijack()
			}
			return nil
		})
		cancel()
		if err != nil {
			log.Printf("Error checking for stalled connections: %v", err)
		}

		// Terminate long-running idle connections with extended timeout
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		_, err = pool.Exec(ctx, `
			SELECT pg_terminate_backend(pid)
			FROM pg_stat_activity
			WHERE state = 'idle in transaction'
			  AND state_change < NOW() - INTERVAL '30 seconds'
			  AND query NOT LIKE '%pg_terminate_backend%'
		`)
		cancel()
		if err != nil {
			log.Printf("Error terminating long-running transactions: %v", err)
		}
	}
}


func testDBConnection(db *pgxpool.Pool) error {
	// Check if the required tables exist
	tables := []string{"batches", "codes"}
	for _, table := range tables {
		var exists bool
		err := db.QueryRow(context.Background(), "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = $1)", table).Scan(&exists)
		if err != nil {
			return fmt.Errorf("error checking if table %s exists: %v", table, err)
		}
		if !exists {
			return fmt.Errorf("required table %s does not exist", table)
		}
	}

	// Test the connection by querying the database
	var testResult int
	err := db.QueryRow(context.Background(), "SELECT 1").Scan(&testResult)
	if err != nil {
		return fmt.Errorf("error testing database connection: %v", err)
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

	code, err := getCode(context.Background(), req)
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

func getBatchesHandler(c *gin.Context) {
	batches, err := getBatches(context.Background())
	if err != nil {
		c.JSON(500, gin.H{"error": "database error"})
		return
	}
	c.JSON(200, batches)
}

func uploadCodesHandler(c *gin.Context) {
	// Get the CSV file from the request
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "No CSV file provided"})
		return
	}
	defer file.Close()

	// Check if the file is a CSV
	if !strings.HasSuffix(header.Filename, ".csv") {
		c.JSON(400, gin.H{"error": "File must be a CSV"})
		return
	}

	// Check if the CSV contains required columns
	csvReader := csv.NewReader(file)
	headers, err := csvReader.Read()
	if err != nil {
		c.JSON(400, gin.H{"error": "Failed to read CSV headers"})
		return
	}
	if !containsColumns(headers, []string{"code", "client_id"}) {
		c.JSON(400, gin.H{"error": "CSV must contain 'code' and 'client_id' columns"})
		return
	}

	// Reset file pointer to the beginning
	file.Seek(0, 0)

	// Get batch name from form data
	batchName := c.PostForm("batch_name")
	if batchName == "" {
		c.JSON(400, gin.H{"error": "Batch name is required"})
		return
	}

	// Get rules from form data (optional)
	rules := c.PostForm("rules")

	// Create a new batch with the given name and rules
	batchID, err := createBatch(c.Request.Context(), batchName, rules)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to create batch: " + err.Error()})
		return
	}

	// Call the service function to handle the upload
	err = uploadCodes(c.Request.Context(), file, batchID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to upload codes: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Codes uploaded successfully"})
}

// Used to check if the CSV contains the required columns
func containsColumns(headers []string, requiredColumns []string) bool {
	headerSet := make(map[string]bool)
	for _, h := range headers {
		headerSet[h] = true
	}
	for _, rc := range requiredColumns {
		if !headerSet[rc] {
			return false
		}
	}
	return true
}

// Add this new function at the end of the file
func healthcheckHandler(c *gin.Context) {
	err := db.Ping(context.Background())
	if err != nil {
		log.Printf("Healthcheck failed: %v", err)
		c.JSON(500, gin.H{"status": "unhealthy", "message": "Unable to connect to the database"})
		return
	}
	c.JSON(200, gin.H{"status": "healthy", "message": "System is operational"})
}
