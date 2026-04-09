Write-Host ">>> Checking Swagger JSON Endpoint <<<"
Invoke-RestMethod -Uri "http://localhost:8080/swagger/doc.json" | Select-Object -Property swagger, info, host, basePath | ConvertTo-Json

Write-Host "`n>>> Creating Order... <<<"
$ORDER = Invoke-RestMethod -Uri "http://localhost:8080/orders" -Method POST -ContentType "application/json" -Body '{"customer_id": "550e8400-e29b-41d4-a716-446655440000"}'
$ORDER_ID = $ORDER.order_id
Write-Host "Created Order: $ORDER_ID"

Write-Host "`n>>> Adding an item to the order... <<<"
Invoke-RestMethod -Uri "http://localhost:8080/orders/$ORDER_ID/items" -Method POST -ContentType "application/json" -Body '{"product_id": "KAFKA-TEST", "product_name": "Event Driven T-Shirt", "quantity": 1, "unit_price": 4200}' | ConvertTo-Json

Write-Host "`n>>> Confirming the order... <<<"
Invoke-RestMethod -Uri "http://localhost:8080/orders/$ORDER_ID/confirm" -Method POST -ContentType "application/json" | ConvertTo-Json

Write-Host "`nWaiting for Kafka to route the message to the projection worker..."
Start-Sleep -Seconds 3

Write-Host "`n>>> Fetching Read Model for the Order... (Projection populated via Kafka outbox worker) <<<"
Invoke-RestMethod -Uri "http://localhost:8080/orders/$ORDER_ID" | ConvertTo-Json -Depth 5
