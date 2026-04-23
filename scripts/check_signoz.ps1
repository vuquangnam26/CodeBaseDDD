
$Endpoints = @(
    "http://localhost:4318/v1/logs",
    "http://localhost:4317",
    "http://signoz-otel-collector:4318/v1/logs"
)

foreach ($url in $Endpoints) {
    Write-Host "`nChecking $url ..." -ForegroundColor Cyan
    try {
        if ($url.Contains("4317")) {
            # OTLP gRPC - just check port
            $tcp = New-Object System.Net.Sockets.TcpClient
            $host_port = $url.Replace("http://", "").Split(":")
            $tcp.Connect($host_port[0], $host_port[1])
            if ($tcp.Connected) {
                Write-Host "Success: Port 4317 is open" -ForegroundColor Green
                $tcp.Close()
            }
        } else {
            # OTLP HTTP - check path
            $response = Invoke-WebRequest -Uri $url -Method POST -Body "{}" -ContentType "application/json" -ErrorAction SilentlyContinue
            Write-Host "Success: Managed to reach $url (Status: $($response.StatusCode))" -ForegroundColor Green
        }
    } catch {
        Write-Host "Failed to reach $url" -ForegroundColor Red
        Write-Host "Error: $($_.Exception.Message)"
    }
}
