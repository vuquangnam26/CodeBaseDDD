sleep 5

$body = @{"customer_id"="550e8400-e29b-41d4-a716-446655440000"} | ConvertTo-Json
$response = Invoke-WebRequest -Uri http://localhost:8080/orders -Method POST -Headers @{"Content-Type"="application/json"} -Body $body -UseBasicParsing

Write-Output "API Response:"
Write-Output $response.Content | ConvertFrom-Json | ConvertTo-Json -Depth 3

sleep 2

Write-Output "Checking database..."
docker exec order-postgres psql -U order -d orderdb -c "SELECT COUNT(*) as log_count FROM logs;"
