#!/bin/bash
# ============================================================
# Sample API Demo — Order Service (CQRS + Kafka)
#
# Prerequisites:
#   1. Start infrastructure: make docker-up-infra
#   2. Run migrations:       make migrate-up
#   3. Start app:            make dev-kafka  (or make dev for in-memory)
#
# This script exercises the full RESTful API flow.
# ============================================================

set -e
BASE_URL="${BASE_URL:-http://localhost:8080}"
CUSTOMER_ID="550e8400-e29b-41d4-a716-446655440000"

echo "=============================================="
echo " Order Service — RESTful API Sample Flow"
echo " Base URL: $BASE_URL"
echo " Event Bus: check EVENT_BUS_TYPE env var"
echo "=============================================="

# ---- Health Checks ----
echo ""
echo "▸ 1. Health Check (liveness)"
curl -s "$BASE_URL/health/live" | python -m json.tool 2>/dev/null || curl -s "$BASE_URL/health/live"

echo ""
echo "▸ 2. Health Check (readiness)"
curl -s "$BASE_URL/health/ready" | python -m json.tool 2>/dev/null || curl -s "$BASE_URL/health/ready"

# ---- Create Order ----
echo ""
echo "▸ 3. POST /orders — Create a new order"
CREATE_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/orders" \
  -H "Content-Type: application/json" \
  -d "{\"customer_id\": \"$CUSTOMER_ID\"}")

HTTP_CODE=$(echo "$CREATE_RESPONSE" | tail -n1)
BODY=$(echo "$CREATE_RESPONSE" | sed '$d')
echo "   Status: $HTTP_CODE"
echo "   Body: $BODY"

ORDER_ID=$(echo "$BODY" | grep -o '"order_id":"[^"]*"' | cut -d'"' -f4)
echo "   Extracted Order ID: $ORDER_ID"

if [ -z "$ORDER_ID" ]; then
  echo "ERROR: Failed to extract order ID. Exiting."
  exit 1
fi

# ---- Add Item 1 ----
echo ""
echo "▸ 4. POST /orders/$ORDER_ID/items — Add first item"
curl -s -X POST "$BASE_URL/orders/$ORDER_ID/items" \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": "PROD-001",
    "product_name": "Premium Widget",
    "quantity": 3,
    "unit_price": 2500
  }' | python -m json.tool 2>/dev/null || echo "(raw output above)"

# ---- Add Item 2 ----
echo ""
echo "▸ 5. POST /orders/$ORDER_ID/items — Add second item"
curl -s -X POST "$BASE_URL/orders/$ORDER_ID/items" \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": "PROD-002",
    "product_name": "Deluxe Gadget",
    "quantity": 1,
    "unit_price": 5000
  }' | python -m json.tool 2>/dev/null || echo "(raw output above)"

# ---- Add Item 3 ----
echo ""
echo "▸ 6. POST /orders/$ORDER_ID/items — Add third item"
curl -s -X POST "$BASE_URL/orders/$ORDER_ID/items" \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": "PROD-003",
    "product_name": "Ultra Sensor",
    "quantity": 10,
    "unit_price": 750
  }' | python -m json.tool 2>/dev/null || echo "(raw output above)"

# ---- Confirm Order ----
echo ""
echo "▸ 7. POST /orders/$ORDER_ID/confirm — Confirm the order"
curl -s -X POST "$BASE_URL/orders/$ORDER_ID/confirm" \
  -H "Content-Type: application/json" | python -m json.tool 2>/dev/null || echo "(raw output above)"

# ---- Wait for Projection ----
echo ""
echo "▸ 8. Waiting 3 seconds for outbox worker + projection..."
sleep 3

# ---- Get Order (Read Model) ----
echo ""
echo "▸ 9. GET /orders/$ORDER_ID — Read model (projected view)"
curl -s "$BASE_URL/orders/$ORDER_ID" | python -m json.tool 2>/dev/null || curl -s "$BASE_URL/orders/$ORDER_ID"

# ---- List Orders ----
echo ""
echo "▸ 10. GET /orders — List all orders"
curl -s "$BASE_URL/orders" | python -m json.tool 2>/dev/null || curl -s "$BASE_URL/orders"

# ---- List Orders with Filters ----
echo ""
echo "▸ 11. GET /orders?status=CONFIRMED — Filter by status"
curl -s "$BASE_URL/orders?status=CONFIRMED" | python -m json.tool 2>/dev/null || curl -s "$BASE_URL/orders?status=CONFIRMED"

echo ""
echo "▸ 12. GET /orders?customer_id=$CUSTOMER_ID — Filter by customer"
curl -s "$BASE_URL/orders?customer_id=$CUSTOMER_ID&page=1&size=10&sort=created_at&dir=desc" \
  | python -m json.tool 2>/dev/null || echo "(raw output above)"

# ---- Error Cases ----
echo ""
echo "▸ 13. POST /orders/$ORDER_ID/confirm — Try confirming again (expect error)"
curl -s -X POST "$BASE_URL/orders/$ORDER_ID/confirm" \
  -H "Content-Type: application/json" | python -m json.tool 2>/dev/null || echo "(raw output above)"

echo ""
echo "▸ 14. POST /orders/$ORDER_ID/items — Add item to confirmed order (expect error)"
curl -s -X POST "$BASE_URL/orders/$ORDER_ID/items" \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": "PROD-004",
    "product_name": "Late Addition",
    "quantity": 1,
    "unit_price": 999
  }' | python -m json.tool 2>/dev/null || echo "(raw output above)"

echo ""
echo "▸ 15. GET /orders/00000000-0000-0000-0000-000000000000 — Non-existent order"
curl -s "$BASE_URL/orders/00000000-0000-0000-0000-000000000000" | python -m json.tool 2>/dev/null || echo "(raw output above)"

echo ""
echo "▸ 16. POST /orders — Validation error (missing customer_id)"
curl -s -X POST "$BASE_URL/orders" \
  -H "Content-Type: application/json" \
  -d '{}' | python -m json.tool 2>/dev/null || echo "(raw output above)"

# ---- Admin ----
echo ""
echo "▸ 17. POST /admin/outbox/requeue — Requeue failed events"
curl -s -X POST "$BASE_URL/admin/outbox/requeue" | python -m json.tool 2>/dev/null || echo "(raw output above)"

# ---- Metrics ----
echo ""
echo "▸ 18. GET /metrics — Prometheus metrics (first 30 lines)"
curl -s "$BASE_URL/metrics" | head -30

echo ""
echo "=============================================="
echo " ✅  API Demo Complete!"
echo ""
echo " Summary:"
echo "   - Created order:    $ORDER_ID"
echo "   - Added 3 items:    PROD-001 (3x2500), PROD-002 (1x5000), PROD-003 (10x750)"
echo "   - Total:            20,000 cents = \$200.00"
echo "   - Confirmed order"
echo "   - Read model updated via outbox -> event bus -> projection"
echo "   - Tested error cases (double confirm, add to confirmed, not found)"
echo "=============================================="
