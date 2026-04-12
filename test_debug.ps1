sleep 2

$body = @{"customer_id"="550e8400-e29b-41d4-a716-446655440000"} | ConvertTo-Json
Invoke-WebRequest -Uri http://localhost:8080/orders -Method POST -Headers @{"Content-Type"="application/json"} -Body $body -UseBasicParsing | Out-Null

sleep 1
