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
    } finally {
        Pop-Location
    }
}

if (-not $SkipDockerUp) {
    $docker = Get-Command docker -ErrorAction SilentlyContinue
    if (-not $docker) {
        throw "docker command not found; start your existing MySQL/RabbitMQ containers manually or install Docker CLI."
    }
    docker compose -f $compose up -d
}

Invoke-NodeBridge "$syncAgent -config configs/lab/edge-a.local.yaml"
Invoke-NodeBridge "$syncAgent -config configs/lab/edge-b.local.yaml"
Invoke-NodeBridge "$syncAgent -config configs/lab/server.local.yaml"

if (-not $SkipMigrate) {
    Invoke-NodeBridge "$syncAgent migrate -config configs/lab/edge-a.local.yaml -scope edge"
    Invoke-NodeBridge "$syncAgent migrate -config configs/lab/edge-b.local.yaml -scope edge"
    Invoke-NodeBridge "$syncAgent migrate -config configs/lab/server.local.yaml -scope server"
}

Invoke-NodeBridge "$syncAgent init-rabbitmq -mode edge -amqp-url amqp://sync:sync_password@127.0.0.1:5672/edge-a-sync"
Invoke-NodeBridge "$syncAgent init-rabbitmq -mode edge -amqp-url amqp://sync:sync_password@127.0.0.1:5672/edge-b-sync"
Invoke-NodeBridge "$syncAgent init-rabbitmq -mode server -edges edge-001,edge-002 -amqp-url amqp://sync:sync_password@127.0.0.1:5672/server-sync"

Write-Host "single-pc lab is ready"
Write-Host "RabbitMQ UI: http://127.0.0.1:15672  user=sync password=sync_password"
