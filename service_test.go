package main

import (
	"bytes"
	"context"
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
	validBatchID := "11111111-1111-1111-1111-111111111111"
	validClientID := "217be7c8-679c-4e08-bffc-db3451bdcdbf"
	validCustomerID := uuid.New().String()

	req := Request{
		BatchID:    validBatchID,
		ClientID:   validClientID,
		CustomerID: validCustomerID,
	}

	t.Run("Successful Code Assignment", func(t *testing.T) {
		code, err := getCodeWithTimeout(context.Background(), req)
		assert.Nil(t, err, "Expected no error")
		assert.NotEmpty(t, code, "Expected code to be returned")
	})

	t.Run("No Code Found", func(t *testing.T) {
		req := Request{
			BatchID:    uuid.New().String(),
			ClientID:   validClientID,
			CustomerID: validCustomerID,
		}
		code, err := getCodeWithTimeout(context.Background(), req)
		assert.Equal(t, ErrNoCodeFound, err, "Expected no code found error")
		assert.Empty(t, code, "Expected no code to be returned")
	})

	t.Run("Invalid BatchID", func(t *testing.T) {
		req := Request{
			BatchID:    "invalid-uuid",
			ClientID:   validClientID,
			CustomerID: validCustomerID,
		}
		code, err := getCodeWithTimeout(context.Background(), req)
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
		code, err := getCodeWithTimeout(context.Background(), req)
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
		code, err := getCodeWithTimeout(context.Background(), req)
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
			req := httptest.NewRequest("POST", "/api/get-code", bytes.NewBuffer([]byte(tt.request)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router := gin.Default()
			router.POST("/api/get-code", getCodeHandler)
			router.ServeHTTP(w, req)

			assert.Equal(t, 400, w.Code)
			assert.Contains(t, w.Body.String(), tt.errorMsg)
		})
	}
}
