sleep 3
$body = @{"customer_id"="550e8400-e29b-41d4-a716-446655440000"} | ConvertTo-Json
Write-Output "Making API call..."
$response = Invoke-WebRequest -Uri http://localhost:8080/orders -Method POST -Headers @{"Content-Type"="application/json"} -Body $body -UseBasicParsing
Write-Output "API call done"
Write-Output $response.Content | ConvertFrom-Json | ConvertTo-Json
sleep 2
