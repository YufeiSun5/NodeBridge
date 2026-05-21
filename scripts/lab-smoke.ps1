param(
    [switch]$SkipDockerUp,
    [switch]$SkipMigrate
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$compose = Join-Path $root "deploy/docker-compose.dev.yml"
$syncAgent = "go run ./cmd/sync-agent"

$env:PATH = "$root\.vfox\sdks\golang\bin;$root\.vfox\sdks\golang\packages\bin;$root\.vfox\sdks\nodejs;$env:PATH"
$env:PATH = "C:\Program Files\Docker\Docker\resources\bin;$env:PATH"

function Invoke-NodeBridge {
    param([string]$Command)
    Push-Location $root
    try {
        Invoke-Expression $Command
        if ($LASTEXITCODE -ne 0) {
            throw "command failed with exit code ${LASTEXITCODE}: $Command"
        }
    } finally {
        Pop-Location
    }
}

function Invoke-NodeBridgeRetry {
    param(
        [string]$Command,
        [int]$Attempts = 10
    )
    for ($i = 1; $i -le $Attempts; $i++) {
        try {
            Invoke-NodeBridge $Command
            return
        } catch {
            if ($i -eq $Attempts) {
                throw
            }
            Start-Sleep -Seconds 3
        }
    }
}

function Wait-RabbitMQ {
    param([string]$Container)
    for ($i = 0; $i -lt 30; $i++) {
        docker exec $Container rabbitmq-diagnostics -q ping | Out-Null
        if ($LASTEXITCODE -eq 0) {
            return
        }
        Start-Sleep -Seconds 2
    }
    throw "RabbitMQ container not ready: $Container"
}

if (-not $SkipDockerUp) {
    $docker = Get-Command docker -ErrorAction SilentlyContinue
    if (-not $docker) {
        throw "docker command not found; start your existing MySQL/RabbitMQ containers manually or install Docker CLI."
    }
    docker compose -f $compose up -d --remove-orphans
    Wait-RabbitMQ "nodebridge-rabbitmq-edge-a"
    Wait-RabbitMQ "nodebridge-rabbitmq-edge-b"
    Wait-RabbitMQ "nodebridge-rabbitmq-server"
    Start-Sleep -Seconds 5
}

Invoke-NodeBridge "$syncAgent -config configs/lab/edge-a.local.yaml"
Invoke-NodeBridge "$syncAgent -config configs/lab/edge-b.local.yaml"
Invoke-NodeBridge "$syncAgent -config configs/lab/server.local.yaml"

if (-not $SkipMigrate) {
    Invoke-NodeBridge "$syncAgent migrate -config configs/lab/edge-a.local.yaml -scope edge"
    Invoke-NodeBridge "$syncAgent migrate -config configs/lab/edge-b.local.yaml -scope edge"
    Invoke-NodeBridge "$syncAgent migrate -config configs/lab/server.local.yaml -scope server"
}

Invoke-NodeBridgeRetry "$syncAgent init-rabbitmq -mode edge -amqp-url amqp://sync:sync_password@127.0.0.1:5673/edge-a-sync"
Invoke-NodeBridgeRetry "$syncAgent init-rabbitmq -mode edge -amqp-url amqp://sync:sync_password@127.0.0.1:5674/edge-b-sync"
Invoke-NodeBridgeRetry "$syncAgent init-rabbitmq -mode server -edges edge-001,edge-002 -amqp-url amqp://sync:sync_password@127.0.0.1:5675/server-sync"

Write-Host "single-pc lab is ready"
Write-Host "Edge A RabbitMQ UI: http://127.0.0.1:15673  user=sync password=sync_password"
Write-Host "Edge B RabbitMQ UI: http://127.0.0.1:15674  user=sync password=sync_password"
Write-Host "Server RabbitMQ UI: http://127.0.0.1:15675  user=sync password=sync_password"
