param(
    [switch]$SkipDockerUp,
    [switch]$SkipMigrate
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
. (Join-Path $root "scripts/lib/lab11.ps1") -RepoRoot $root

$labDir = Join-Path $root ".cache/lab11"
$configDir = Join-Path $root "configs/lab11"
$compose = Join-Path $labDir "docker-compose.yml"

function Write-Lab11Compose {
    New-Item -ItemType Directory -Force -Path $labDir | Out-Null
    $lines = New-Object System.Collections.Generic.List[string]
    $lines.Add("services:")
    for ($i = 1; $i -le 10; $i++) {
        $edgeID = Get-Lab11EdgeID $i
        $rabbitPort = Get-Lab11EdgeRabbitPort $i
        $rabbitUIPort = Get-Lab11EdgeRabbitUIPort $i
        $mysqlPort = Get-Lab11EdgeMySQLPort $i
        $serverID = 100 + $i
        $lines.Add("  rabbitmq-$edgeID`:")
        $lines.Add("    image: rabbitmq:3-management")
        $lines.Add("    container_name: nodebridge-lab11-rabbitmq-$edgeID")
        $lines.Add("    ports:")
        $lines.Add("      - `"$rabbitPort`:5672`"")
        $lines.Add("      - `"$rabbitUIPort`:15672`"")
        $lines.Add("    volumes:")
        $lines.Add("      - ../../deploy/rabbitmq/rabbitmq.dev.conf:/etc/rabbitmq/rabbitmq.conf:ro")
        $lines.Add("      - ../../deploy/rabbitmq/definitions.dev.json:/etc/rabbitmq/definitions.json:ro")
        $lines.Add("")
        $lines.Add("  mysql-$edgeID`:")
        $lines.Add("    image: mysql:8.4")
        $lines.Add("    container_name: nodebridge-lab11-mysql-$edgeID")
        $lines.Add("    environment:")
        $lines.Add("      MYSQL_ROOT_PASSWORD: root_password")
        $lines.Add("      MYSQL_DATABASE: scada_edge")
        $lines.Add("      MYSQL_USER: sync_user")
        $lines.Add("      MYSQL_PASSWORD: sync_password")
        $lines.Add("    ports:")
        $lines.Add("      - `"$mysqlPort`:3306`"")
        $lines.Add("    command: [`"--server-id=$serverID`", `"--log-bin=mysql-bin`", `"--binlog-format=ROW`"]")
        $lines.Add("")
    }
    $lines.Add("  rabbitmq-server:")
    $lines.Add("    image: rabbitmq:3-management")
    $lines.Add("    container_name: nodebridge-lab11-rabbitmq-server")
    $lines.Add("    ports:")
    $lines.Add("      - `"5699`:5672`"")
    $lines.Add("      - `"15699`:15672`"")
    $lines.Add("    volumes:")
    $lines.Add("      - ../../deploy/rabbitmq/rabbitmq.dev.conf:/etc/rabbitmq/rabbitmq.conf:ro")
    $lines.Add("      - ../../deploy/rabbitmq/definitions.dev.json:/etc/rabbitmq/definitions.json:ro")
    $lines.Add("")
    $lines.Add("  mysql-server:")
    $lines.Add("    image: mysql:8.4")
    $lines.Add("    container_name: nodebridge-lab11-mysql-server")
    $lines.Add("    environment:")
    $lines.Add("      MYSQL_ROOT_PASSWORD: root_password")
    $lines.Add("      MYSQL_DATABASE: scada_center")
    $lines.Add("      MYSQL_USER: sync_user")
    $lines.Add("      MYSQL_PASSWORD: sync_password")
    $lines.Add("    ports:")
    $lines.Add("      - `"3330`:3306`"")
    $lines.Add("    command: [`"--server-id=299`", `"--log-bin=mysql-bin`", `"--binlog-format=ROW`"]")
    [System.IO.File]::WriteAllText($compose, ($lines -join [Environment]::NewLine), [System.Text.UTF8Encoding]::new($false))
}

function Write-Lab11Configs {
    New-Item -ItemType Directory -Force -Path $configDir | Out-Null
    $serverConfig = @"
mode: server

node:
  id: server-001
  name: Lab11 Server
  location: single-pc-lab11

mysql:
  host: 127.0.0.1
  port: 3330
  username: sync_user
  password: sync_password
  database: scada_center

rabbitmq:
  mode: external
  install: false
  server_url: amqp://sync:sync_password@127.0.0.1:5699/server-sync
  management_url: http://127.0.0.1:15699
  username: sync
  password: sync_password
  vhost: server-sync

sync:
  upload_batch_size: 50
  dispatch_batch_size: 50
  flush_interval_millis: 500
  retry_interval_seconds: 2
  node_timeout_seconds: 60

log_web:
  enable: false
  bind: 127.0.0.1
  port: 18180
  token: lab_token

security:
  admin_password: admin-pass
  exit_password: exit-pass
"@
    [System.IO.File]::WriteAllText((Join-Path $configDir "server.local.yaml"), $serverConfig, [System.Text.UTF8Encoding]::new($false))

    for ($i = 1; $i -le 10; $i++) {
        $edgeID = Get-Lab11EdgeID $i
        $mysqlPort = Get-Lab11EdgeMySQLPort $i
        $rabbitPort = Get-Lab11EdgeRabbitPort $i
        $rabbitUIPort = Get-Lab11EdgeRabbitUIPort $i
        $logPort = 18180 + $i
        $edgeConfig = @"
mode: edge

node:
  id: $edgeID
  name: Lab11 $edgeID
  location: single-pc-lab11

mysql:
  host: 127.0.0.1
  port: $mysqlPort
  username: sync_user
  password: sync_password
  database: scada_edge

rabbitmq:
  mode: external
  install: false
  local_url: amqp://sync:sync_password@127.0.0.1:$rabbitPort/$edgeID-sync
  server_url: amqp://sync:sync_password@127.0.0.1:5699/server-sync
  management_url: http://127.0.0.1:$rabbitUIPort
  username: sync
  password: sync_password
  vhost: $edgeID-sync

cdc:
  type: stub
  reader_name: $edgeID
  canal_addr: 127.0.0.1:11111
  destination: $edgeID
  username: ""
  password: ""
  filter: scada_edge\..*
  batch_size: 1000
  use_gtid: false

sync:
  upload_batch_size: 50
  dispatch_batch_size: 50
  flush_interval_millis: 500
  retry_interval_seconds: 2
  heartbeat_interval_seconds: 15

log_web:
  enable: false
  bind: 127.0.0.1
  port: $logPort
  token: lab_token

security:
  admin_password: admin-pass
  exit_password: exit-pass
"@
        [System.IO.File]::WriteAllText((Join-Path $configDir "$edgeID.local.yaml"), $edgeConfig, [System.Text.UTF8Encoding]::new($false))
    }
}

function Wait-RabbitMQ {
    param([string]$Container)
    for ($i = 0; $i -lt 45; $i++) {
        docker exec $Container rabbitmq-diagnostics -q ping | Out-Null
        if ($LASTEXITCODE -eq 0) {
            docker exec $Container rabbitmqctl await_startup | Out-Null
            return
        }
        Start-Sleep -Seconds 2
    }
    throw "RabbitMQ container not ready: $Container"
}

function Ensure-RabbitVHost {
    param(
        [string]$Container,
        [string]$VHost
    )
    for ($i = 0; $i -lt 30; $i++) {
        docker exec $Container rabbitmqctl await_startup | Out-Null
        docker exec $Container rabbitmqctl add_vhost $VHost | Out-Null
        docker exec $Container rabbitmqctl set_permissions -p $VHost sync ".*" ".*" ".*" | Out-Null
        if ($LASTEXITCODE -eq 0) {
            return
        }
        Start-Sleep -Seconds 2
    }
    throw "RabbitMQ vhost not ready: $Container/$VHost"
}

function Wait-MySQL {
    param([string]$Container)
    for ($i = 0; $i -lt 45; $i++) {
        docker exec $Container mysqladmin ping -usync_user -psync_password --silent | Out-Null
        if ($LASTEXITCODE -eq 0) {
            return
        }
        Start-Sleep -Seconds 2
    }
    throw "MySQL container not ready: $Container"
}

Write-Lab11Compose
Write-Lab11Configs

if (-not $SkipDockerUp) {
    docker compose -f $compose up -d --remove-orphans
    Assert-LastExit "docker compose lab11 up"
    for ($i = 1; $i -le 10; $i++) {
        Wait-RabbitMQ (Get-Lab11EdgeRabbitContainer $i)
        Wait-MySQL (Get-Lab11EdgeMySQLContainer $i)
    }
    Wait-RabbitMQ "nodebridge-lab11-rabbitmq-server"
    Wait-MySQL "nodebridge-lab11-mysql-server"
}

for ($i = 1; $i -le 10; $i++) {
    $edgeID = Get-Lab11EdgeID $i
    $rabbit = Get-Lab11EdgeRabbitContainer $i
    Ensure-RabbitVHost $rabbit "$edgeID-sync"
}

if (-not $SkipMigrate) {
    for ($i = 1; $i -le 10; $i++) {
        Invoke-NodeBridge @("migrate", "-config", (Get-Lab11EdgeConfig $i), "-scope", "edge")
    }
    Invoke-NodeBridge @("migrate", "-config", $script:Lab11ServerConfig, "-scope", "server")
}

for ($i = 1; $i -le 10; $i++) {
    $edgeID = Get-Lab11EdgeID $i
    Invoke-NodeBridge @("register-node", "-config", $script:Lab11ServerConfig, "-node-id", $edgeID, "-node-name", "Lab11 $edgeID", "-location", "single-pc-lab11", "-version", "0.24.0")
}

for ($i = 1; $i -le 10; $i++) {
    $edgeID = Get-Lab11EdgeID $i
    $rabbitPort = Get-Lab11EdgeRabbitPort $i
    Invoke-NodeBridge @("init-rabbitmq", "-mode", "edge", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:$rabbitPort/$edgeID-sync")
}
Invoke-NodeBridge @("init-rabbitmq", "-mode", "server", "-config", $script:Lab11ServerConfig, "-amqp-url", $script:Lab11ServerRabbitURL)

Write-Host "lab11 ready: 10 edge nodes + server"
Write-Host "Server RabbitMQ UI: http://127.0.0.1:15699 user=sync password=sync_password"
