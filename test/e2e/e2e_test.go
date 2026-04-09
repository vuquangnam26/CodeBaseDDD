//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startPostgresContainer(t *testing.T) (string, func()) {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "orderdb",
			"POSTGRES_USER":     "order",
			"POSTGRES_PASSWORD": "order123",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "5432")

	dsn := fmt.Sprintf("host=%s port=%s user=order password=order123 dbname=orderdb sslmode=disable",
		host, port.Port())

	return dsn, func() { container.Terminate(ctx) }
}

func TestE2E_OrderFlow(t *testing.T) {
	// This test requires the application to be running.
	// Skip if we can't connect.
	baseURL := "http://localhost:8080"

	// Health check
	resp, err := http.Get(baseURL + "/health/live")
	if err != nil {
		t.Skip("Application not running, skipping E2E test")
	}
	defer resp.Body.Close()

	customerID := uuid.New().String()

	// 1. Create Order
	createBody, _ := json.Marshal(map[string]string{"customer_id": customerID})
	resp, err = http.Post(baseURL+"/orders", "application/json", bytes.NewReader(createBody))
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var createResult map[string]string
	json.NewDecoder(resp.Body).Decode(&createResult)
	resp.Body.Close()
	orderID := createResult["order_id"]
	require.NotEmpty(t, orderID)

	// 2. Add Item
	addBody, _ := json.Marshal(map[string]interface{}{
		"product_id":   "PROD-001",
		"product_name": "Premium Widget",
		"quantity":     3,
		"unit_price":   2500,
	})
	resp, err = http.Post(baseURL+"/orders/"+orderID+"/items", "application/json", bytes.NewReader(addBody))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// 3. Add another item
	addBody2, _ := json.Marshal(map[string]interface{}{
		"product_id":   "PROD-002",
		"product_name": "Deluxe Gadget",
		"quantity":     1,
		"unit_price":   5000,
	})
	resp, err = http.Post(baseURL+"/orders/"+orderID+"/items", "application/json", bytes.NewReader(addBody2))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// 4. Confirm Order
	resp, err = http.Post(baseURL+"/orders/"+orderID+"/confirm", "application/json", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// 5. Wait for projection (outbox worker polls every 1s)
	time.Sleep(3 * time.Second)

	// 6. Get Order from read model
	resp, err = http.Get(baseURL + "/orders/" + orderID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var orderResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&orderResp)
	resp.Body.Close()

	assert.Equal(t, "CONFIRMED", orderResp["status"])
	assert.Equal(t, float64(12500), orderResp["total_amount"]) // 3*2500 + 1*5000

	// 7. List Orders
	resp, err = http.Get(baseURL + "/orders?customer_id=" + customerID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	t.Log("E2E test passed: create -> add items -> confirm -> read model updated")
}
