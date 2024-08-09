package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
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
	for i := 0; i < maxRetries; i++ {
		db, err = pgxpool.Connect(context.Background(), os.Getenv("DATABASE_URL"))
		if err == nil {
			break
		}
		log.Printf("Unable to connect to database: %v\n", err)
		time.Sleep(5 * time.Second)
	}
	return db, err
}

func getCodeHandler(c *gin.Context) {
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "cannot parse json"})
		return
	}

	code, err := getCodeWithTimeout(context.Background(), req)
	if err != nil {
		if err == ErrNoCodeFound {
			c.JSON(404, gin.H{"error": "no code found"})
		} else if err == ErrConditionNotMet {
			c.JSON(403, gin.H{"error": "rule conditions not met"})
		} else {
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
