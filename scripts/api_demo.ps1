# ============================================================
# Sample API Demo — Order Service (CQRS + Kafka)
#
# Prerequisites:
#   1. Start infrastructure: make docker-up-infra
#   2. Run migrations:       make migrate-up
#   3. Start app:            make dev-kafka  (or make dev for in-memory)
# ============================================================

$BASE_URL = "http://localhost:8080"
$CUSTOMER_ID = "550e8400-e29b-41d4-a716-446655440000"

Write-Host "=============================================="
Write-Host " Order Service — RESTful API Sample Flow"
Write-Host " Base URL: $BASE_URL"
Write-Host "=============================================="

# ---- 1. Health Check ----
Write-Host "`n▸ 1. Health Check (liveness)"
Invoke-RestMethod -Uri "$BASE_URL/health/live" | ConvertTo-Json

Write-Host "`n▸ 2. Health Check (readiness)"
Invoke-RestMethod -Uri "$BASE_URL/health/ready" | ConvertTo-Json

# ---- 3. Create Order ----
Write-Host "`n▸ 3. POST /orders — Create a new order"
$createBody = @{ customer_id = $CUSTOMER_ID } | ConvertTo-Json
$createResult = Invoke-RestMethod -Uri "$BASE_URL/orders" -Method POST -ContentType "application/json" -Body $createBody
$createResult | ConvertTo-Json
$ORDER_ID = $createResult.order_id
Write-Host "   Order ID: $ORDER_ID"

# ---- 4. Add Item 1 ----
Write-Host "`n▸ 4. POST /orders/$ORDER_ID/items — Add first item (3x Premium Widget @ 2500)"
$item1 = @{
    product_id   = "PROD-001"
    product_name = "Premium Widget"
    quantity     = 3
    unit_price   = 2500
} | ConvertTo-Json
Invoke-RestMethod -Uri "$BASE_URL/orders/$ORDER_ID/items" -Method POST -ContentType "application/json" -Body $item1 | ConvertTo-Json

# ---- 5. Add Item 2 ----
Write-Host "`n▸ 5. POST /orders/$ORDER_ID/items — Add second item (1x Deluxe Gadget @ 5000)"
$item2 = @{
    product_id   = "PROD-002"
    product_name = "Deluxe Gadget"
    quantity     = 1
    unit_price   = 5000
} | ConvertTo-Json
Invoke-RestMethod -Uri "$BASE_URL/orders/$ORDER_ID/items" -Method POST -ContentType "application/json" -Body $item2 | ConvertTo-Json

# ---- 6. Add Item 3 ----
Write-Host "`n▸ 6. POST /orders/$ORDER_ID/items — Add third item (10x Ultra Sensor @ 750)"
$item3 = @{
    product_id   = "PROD-003"
    product_name = "Ultra Sensor"
    quantity     = 10
    unit_price   = 750
} | ConvertTo-Json
Invoke-RestMethod -Uri "$BASE_URL/orders/$ORDER_ID/items" -Method POST -ContentType "application/json" -Body $item3 | ConvertTo-Json

# ---- 7. Confirm Order ----
Write-Host "`n▸ 7. POST /orders/$ORDER_ID/confirm — Confirm the order"
Invoke-RestMethod -Uri "$BASE_URL/orders/$ORDER_ID/confirm" -Method POST -ContentType "application/json" | ConvertTo-Json

# ---- 8. Wait for Projection ----
Write-Host "`n▸ 8. Waiting 3 seconds for outbox worker + projection..."
Start-Sleep -Seconds 3

# ---- 9. Get Order (Read Model) ----
Write-Host "`n▸ 9. GET /orders/$ORDER_ID — Read model (projected view)"
Invoke-RestMethod -Uri "$BASE_URL/orders/$ORDER_ID" | ConvertTo-Json -Depth 5

# ---- 10. List Orders ----
Write-Host "`n▸ 10. GET /orders — List all orders"
Invoke-RestMethod -Uri "$BASE_URL/orders" | ConvertTo-Json -Depth 5

# ---- 11. Filter by status ----
Write-Host "`n▸ 11. GET /orders?status=CONFIRMED — Filter by status"
Invoke-RestMethod -Uri "$BASE_URL/orders?status=CONFIRMED" | ConvertTo-Json -Depth 5

# ---- 12. Filter by customer + pagination ----
Write-Host "`n▸ 12. GET /orders?customer_id=...&page=1&size=10 — Filter + paginate"
Invoke-RestMethod -Uri "$BASE_URL/orders?customer_id=$CUSTOMER_ID&page=1&size=10&sort=created_at&dir=desc" | ConvertTo-Json -Depth 5

# ---- Error Cases ----

Write-Host "`n▸ 13. POST /orders/$ORDER_ID/confirm — Double confirm (expect 409 Conflict)"
try {
    Invoke-RestMethod -Uri "$BASE_URL/orders/$ORDER_ID/confirm" -Method POST -ContentType "application/json"
} catch {
    $_.ErrorDetails.Message | Write-Host
    $streamReader = [System.IO.StreamReader]::new($_.Exception.Response.GetResponseStream())
    $streamReader.ReadToEnd() | Write-Host
    $streamReader.Close()
}

Write-Host "`n▸ 14. POST /orders/$ORDER_ID/items — Add to confirmed order (expect 409 Conflict)"
try {
    $badItem = @{ product_id="X"; product_name="Fail"; quantity=1; unit_price=100 } | ConvertTo-Json
    Invoke-RestMethod -Uri "$BASE_URL/orders/$ORDER_ID/items" -Method POST -ContentType "application/json" -Body $badItem
} catch {
    $streamReader = [System.IO.StreamReader]::new($_.Exception.Response.GetResponseStream())
    $streamReader.ReadToEnd() | Write-Host
    $streamReader.Close()
}

Write-Host "`n▸ 15. GET /orders/00000000-... — Non-existent order (expect 404)"
try {
    Invoke-RestMethod -Uri "$BASE_URL/orders/00000000-0000-0000-0000-000000000000"
} catch {
    $streamReader = [System.IO.StreamReader]::new($_.Exception.Response.GetResponseStream())
    $streamReader.ReadToEnd() | Write-Host
    $streamReader.Close()
}

Write-Host "`n▸ 16. POST /orders {} — Missing customer_id (expect 400 Validation Error)"
try {
    Invoke-RestMethod -Uri "$BASE_URL/orders" -Method POST -ContentType "application/json" -Body '{}'
} catch {
    $streamReader = [System.IO.StreamReader]::new($_.Exception.Response.GetResponseStream())
    $streamReader.ReadToEnd() | Write-Host
    $streamReader.Close()
}

# ---- Admin ----
Write-Host "`n▸ 17. POST /admin/outbox/requeue — Requeue failed outbox events"
Invoke-RestMethod -Uri "$BASE_URL/admin/outbox/requeue" -Method POST | ConvertTo-Json

# ---- Metrics ----
Write-Host "`n▸ 18. GET /metrics — Prometheus metrics (first 20 lines)"
(Invoke-WebRequest -Uri "$BASE_URL/metrics").Content.Split("`n") | Select-Object -First 20

Write-Host "`n=============================================="
Write-Host " ✅  API Demo Complete!"
Write-Host ""
Write-Host " Summary:"
Write-Host "   Order ID:    $ORDER_ID"
Write-Host "   Items:       PROD-001 (3x2500), PROD-002 (1x5000), PROD-003 (10x750)"
Write-Host "   Total:       20,000 cents = `$200.00"
Write-Host "   Status:      CONFIRMED"
Write-Host "   Flow:        REST -> Command -> UoW(write+outbox) -> Worker -> EventBus -> Projection -> ReadModel"
Write-Host "=============================================="
