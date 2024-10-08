package main

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestGetCodeWithTimeout(t *testing.T) {
	// Setup database connection for tests
	var err error
	db, err = connectToDB() // Ensuring db is set globally as it might be used elsewhere
	if err != nil {
		t.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer db.Close()

	// Generate valid UUIDs for testing
	// This is based on the seed data
	validBatchID := "11111111-1111-1111-1111-111111111111"
	validClientID := "217be7c8-679c-4e08-bffc-db3451bdcdbf"
	validCustomerID := uuid.New().String()

	req := Request{
		BatchID:    validBatchID,
		ClientID:   validClientID,
		CustomerID: validCustomerID,
	}

	t.Run("Successful Code Assignment", func(t *testing.T) {
		code, err := getCode(context.Background(), req)
		assert.Nil(t, err, "Expected no error")
		assert.NotEmpty(t, code, "Expected code to be returned")
	})

	t.Run("No Batch Found", func(t *testing.T) {
		req := Request{
			BatchID:    uuid.New().String(),
			ClientID:   validClientID,
			CustomerID: validCustomerID,
		}
		code, err := getCode(context.Background(), req)
		assert.Equal(t, ErrNoBatchFound, err, "Expected no batch was found error")
		assert.Empty(t, code, "Expected no code to be returned")
	})

	t.Run("No Code Found when all are used in the batch", func(t *testing.T) {
		req := Request{
			BatchID:    "33333333-3333-3333-3333-333333333333",
			ClientID:   validClientID,
			CustomerID: validCustomerID,
		}
		code, err := getCode(context.Background(), req)
		assert.Equal(t, ErrNoCodeFound, err, "Expected no code was found error")
		assert.Empty(t, code, "Expected no code to be returned")
	})

	t.Run("Responds when the batch is expired", func(t *testing.T) {
		req := Request{
			BatchID:    "44444444-4444-4444-4444-444444444444",
			ClientID:   validClientID,
			CustomerID: validCustomerID,
		}
		code, err := getCode(context.Background(), req)
		assert.Equal(t, ErrBatchExpired, err, "Expected batch expired error")
		assert.Empty(t, code, "Expected no code to be returned")
	})

	t.Run("Invalid BatchID", func(t *testing.T) {
		req := Request{
			BatchID:    "invalid-uuid",
			ClientID:   validClientID,
			CustomerID: validCustomerID,
		}
		code, err := getCode(context.Background(), req)
		assert.NotNil(t, err, "Expected error for invalid BatchID")
		assert.Contains(t, err.Error(), "invalid batch_id format")
		assert.Empty(t, code, "Expected no code to be returned")
	})

	t.Run("Invalid ClientID", func(t *testing.T) {
		req := Request{
			BatchID:    validBatchID,
			ClientID:   "invalid-uuid",
			CustomerID: validCustomerID,
		}
		code, err := getCode(context.Background(), req)
		assert.NotNil(t, err, "Expected error for invalid ClientID")
		assert.Contains(t, err.Error(), "invalid client_id format")
		assert.Empty(t, code, "Expected no code to be returned")
	})

	t.Run("Invalid CustomerID", func(t *testing.T) {
		req := Request{
			BatchID:    validBatchID,
			ClientID:   validClientID,
			CustomerID: "invalid-uuid",
		}
		code, err := getCode(context.Background(), req)
		assert.NotNil(t, err, "Expected error for invalid CustomerID")
		assert.Contains(t, err.Error(), "invalid customer_id format")
		assert.Empty(t, code, "Expected no code to be returned")
	})
}

func TestGetCodeHandler_InvalidJSON(t *testing.T) {
	tests := []struct {
		name     string
		request  string
		errorMsg string
	}{
		{
			name: "Invalid BatchID",
			request: `{
                "batchid": "invalid-batch-id",
                "clientid": "217be7c8-679c-4e08-bffc-db3451bdcdbf",
                "customerid": "fba9230a-a521-430e-aaf8-8aefbf588071"
            }`,
			errorMsg: "invalid batch_id format",
		},
		{
			name: "Invalid ClientID",
			request: `{
                "batchid": "c6ffca2e-603b-4b14-a39d-9e37b6f1d63b",
                "clientid": "invalid-client-id",
                "customerid": "fba9230a-a521-430e-aaf8-8aefbf588071"
            }`,
			errorMsg: "invalid client_id format",
		},
		{
			name: "Invalid CustomerID",
			request: `{
                "batchid": "c6ffca2e-603b-4b14-a39d-9e37b6f1d63b",
                "clientid": "217be7c8-679c-4e08-bffc-db3451bdcdbf",
                "customerid": "invalid-customer-id"
            }`,
			errorMsg: "invalid customer_id format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/code/redeem", bytes.NewBuffer([]byte(tt.request)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router := gin.Default()
			router.POST("/api/v1/code/redeem", getCodeHandler)
			router.ServeHTTP(w, req)

			assert.Equal(t, 400, w.Code)
			assert.Contains(t, w.Body.String(), tt.errorMsg)
		})
	}
}

func TestGetBatchesHandler(t *testing.T) {
	// Setup database connection for tests
	var err error
	db, err = connectToDB() // Ensuring db is set globally as it might be used elsewhere
	if err != nil {
		t.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer db.Close()

	// Create a new gin router
	router := gin.Default()
	router.GET("/api/v1/batches", getBatchesHandler)

	t.Run("Fetch batches successfully", func(t *testing.T) {
		// Create a new request
		req, _ := http.NewRequest("GET", "/api/v1/batches", nil)
		w := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(w, req)

		// Assert the response code
		assert.Equal(t, 200, w.Code)

		// Parse the response body
		var response []Batch
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		// Assert that we got some batches
		assert.NotEmpty(t, response)

		// Check the structure of the first batch
		firstBatch := response[0]
		assert.NotEmpty(t, firstBatch.ID)
		assert.NotEmpty(t, firstBatch.Name)
		assert.NotNil(t, firstBatch.Rules)
		assert.NotNil(t, firstBatch.Expired)
	})
}

func TestUploadCodesHandler(t *testing.T) {
	// Setup database connection for tests
	var err error
	db, err = connectToDB() // Ensuring db is set globally as it might be used elsewhere
	if err != nil {
		t.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer db.Close()

	// Create a new gin router
	router := gin.Default()
	router.POST("/api/v1/codes/upload", uploadCodesHandler)

	t.Run("Successful upload", func(t *testing.T) {
		// Create a new multipart writer
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Add the batch name and rules
		_ = writer.WriteField("batch_name", "Test Batch")
		_ = writer.WriteField("rules", `{"maxpercustomer": 2, "timelimit": 7}`)

		// Create the file part
		part, _ := writer.CreateFormFile("file", "test.csv")
		_, _ = part.Write([]byte("client_id,batch_id,code\n217be7c8-679c-4e08-bffc-db3451bdcdbf,11111111-1111-1111-1111-111111111111,TESTCODE123"))

		writer.Close()

		// Create a new request
		req, _ := http.NewRequest("POST", "/api/v1/codes/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(w, req)

		// Assert the response code
		assert.Equal(t, 200, w.Code)

		// Parse the response body
		var response map[string]string
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		// Check the response message
		assert.Equal(t, "Codes uploaded successfully", response["message"])

		// Check if the record is in the database
		var count int
		err = db.QueryRow(context.Background(), "SELECT COUNT(*) FROM codes WHERE code = $1", "TESTCODE123").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("Incorrectly formatted CSV", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("batch_name", "Test Batch")
		part, _ := writer.CreateFormFile("file", "test.csv")
		_, _ = part.Write([]byte("invalid,csv,format"))
		writer.Close()

		req, _ := http.NewRequest("POST", "/api/v1/codes/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, 500, w.Code)
		assert.Contains(t, w.Body.String(), "Failed to upload codes")
	})

	t.Run("CSV missing required columns", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("batch_name", "Test Batch")
		part, _ := writer.CreateFormFile("file", "test.csv")
		_, _ = part.Write([]byte("client_id,code\n217be7c8-679c-4e08-bffc-db3451bdcdbf,TESTCODE123"))
		writer.Close()

		req, _ := http.NewRequest("POST", "/api/v1/codes/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, 500, w.Code)
		assert.Contains(t, w.Body.String(), "Failed to upload codes")
	})

	t.Run("No batch name provided", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "test.csv")
		_, _ = part.Write([]byte("client_id,batch_id,code\n217be7c8-679c-4e08-bffc-db3451bdcdbf,11111111-1111-1111-1111-111111111111,TESTCODE123"))
		writer.Close()

		req, _ := http.NewRequest("POST", "/api/v1/codes/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, 400, w.Code)
		assert.Contains(t, w.Body.String(), "Batch name is required")
	})

	t.Run("No rules provided", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("batch_name", "Test Batch")
		part, _ := writer.CreateFormFile("file", "test.csv")
		_, _ = part.Write([]byte("client_id,batch_id,code\n217be7c8-679c-4e08-bffc-db3451bdcdbf,11111111-1111-1111-1111-111111111111,TESTCODE123"))
		writer.Close()

		req, _ := http.NewRequest("POST", "/api/v1/codes/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		assert.Contains(t, w.Body.String(), "Codes uploaded successfully")
	})

	t.Run("No CSV provided", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("batch_name", "Test Batch")
		writer.Close()

		req, _ := http.NewRequest("POST", "/api/v1/codes/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, 400, w.Code)
		assert.Contains(t, w.Body.String(), "No CSV file provided")
	})

	t.Run("File is not a CSV", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("batch_name", "Test Batch")
		part, _ := writer.CreateFormFile("file", "test.txt")
		_, _ = part.Write([]byte("This is not a CSV file"))
		writer.Close()

		req, _ := http.NewRequest("POST", "/api/v1/codes/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, 400, w.Code)
		assert.Contains(t, w.Body.String(), "File must be a CSV")
	})
}
