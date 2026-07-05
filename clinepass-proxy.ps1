<#
.SYNOPSIS
    ClinePass Proxy 管理脚本
.DESCRIPTION
    管理 ClinePass Proxy 服务的启停、配置和日志查看
#>

param(
    [Parameter(Position=0)]
    [ValidateSet("start", "stop", "status", "restart", "config", "logs", "help")]
    [string]$Action = "help"
)

$ErrorActionPreference = "Stop"

# Paths
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$BinPath = Join-Path $ScriptDir "bin\clinepass-proxy.exe"
$ConfigPath = Join-Path $ScriptDir "proxy-config.json"
$PidPath = Join-Path $ScriptDir "proxy.pid"
$LogPath = Join-Path $ScriptDir "logs\proxy.log"

# Ensure log directory exists
$LogDir = Join-Path $ScriptDir "logs"
if (!(Test-Path $LogDir)) {
    New-Item -ItemType Directory -Path $LogDir | Out-Null
}

function Read-Config {
    if (Test-Path $ConfigPath) {
        return Get-Content $ConfigPath -Raw | ConvertFrom-Json
    }
    return @{
        api_key = ""
        host = "127.0.0.1"
        port = 55991
        debug = $false
    }
}

function Write-Config($config) {
    $config | ConvertTo-Json -Depth 10 | Set-Content $ConfigPath -Encoding UTF8
}

function Get-ProcessId {
    if (Test-Path $PidPath) {
        $pid = Get-Content $PidPath -Raw
        if ($pid) { return [int]$pid.Trim() }
    }
    return $null
}

function Test-Running {
    $pid = Get-ProcessId
    if ($pid) {
        $proc = Get-Process -Id $pid -ErrorAction SilentlyContinue
        if ($proc -and $proc.ProcessName -match "clinepass-proxy") {
            return $true
        }
    }
    return $false
}

function Start-Proxy {
    if (Test-Running) {
        Write-Host "[INFO] Proxy is already running (PID: $(Get-ProcessId))" -ForegroundColor Yellow
        return
    }

    $config = Read-Config
    $args = @()

    if ($config.api_key) {
        $args += "-api-key", $config.api_key
    }
    if ($config.host) {
        $args += "-host", $config.host
    }
    if ($config.port) {
        $args += "-port", $config.port
    }
    if ($config.debug) {
        $args += "-debug"
    }

    Write-Host "[INFO] Starting ClinePass Proxy..." -ForegroundColor Green
    Write-Host "[INFO] Endpoint: http://$($config.host):$($config.port)/v1" -ForegroundColor Cyan

    $process = Start-Process -FilePath $BinPath -ArgumentList $args `
        -RedirectStandardOutput (Join-Path $LogDir "stdout.log") `
        -RedirectStandardError (Join-Path $LogDir "stderr.log") `
        -NoNewWindow -PassThru

    $process.Id | Set-Content $PidPath
    Write-Host "[INFO] Proxy started (PID: $($process.Id))" -ForegroundColor Green
    Write-Host "[INFO] Logs: $LogPath" -ForegroundColor Gray
}

function Stop-Proxy {
    $pid = Get-ProcessId
    if ($pid) {
        $proc = Get-Process -Id $pid -ErrorAction SilentlyContinue
        if ($proc) {
            Write-Host "[INFO] Stopping proxy (PID: $pid)..." -ForegroundColor Yellow
            Stop-Process -Id $pid -Force
            Write-Host "[INFO] Proxy stopped" -ForegroundColor Green
        }
        Remove-Item $PidPath -ErrorAction SilentlyContinue
    } else {
        Write-Host "[INFO] Proxy is not running" -ForegroundColor Gray
    }
}

function Show-Status {
    if (Test-Running) {
        $pid = Get-ProcessId
        $config = Read-Config
        Write-Host "[STATUS] Proxy is running (PID: $pid)" -ForegroundColor Green
        Write-Host "[STATUS] Endpoint: http://$($config.host):$($config.port)/v1" -ForegroundColor Cyan
    } else {
        Write-Host "[STATUS] Proxy is not running" -ForegroundColor Red
    }
}

function Set-Config {
    $config = Read-Config

    Write-Host "=== ClinePass Proxy Configuration ===" -ForegroundColor Cyan
    Write-Host ""

    $newKey = Read-Host "API Key (current: $(if($config.api_key){$config.api_key.Substring(0, [Math]::Min(8, $config.api_key.Length)) + '...'}else{'not set'}))"
    if ($newKey) { $config.api_key = $newKey }

    $newHost = Read-Host "Host (current: $($config.host))"
    if ($newHost) { $config.host = $newHost }

    $newPort = Read-Host "Port (current: $($config.port))"
    if ($newPort) { $config.port = [int]$newPort }

    $debugInput = Read-Host "Debug mode? (current: $($config.debug)) [y/N]"
    if ($debugInput -eq 'y' -or $debugInput -eq 'Y') {
        $config.debug = $true
    } elseif ($debugInput -eq 'n' -or $debugInput -eq 'N') {
        $config.debug = $false
    }

    Write-Config $config
    Write-Host "[INFO] Configuration saved" -ForegroundColor Green
    Write-Host "[INFO] Restart the proxy for changes to take effect" -ForegroundColor Yellow
}

function Show-Logs {
    if (Test-Path $LogPath) {
        Get-Content $LogPath -Tail 50
    } else {
        Write-Host "[INFO] No log file found" -ForegroundColor Gray
    }
}

function Show-Help {
    Write-Host ""
    Write-Host "ClinePass Proxy Manager" -ForegroundColor Cyan
    Write-Host "======================" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Usage: .\clinepass-proxy.ps1 <command>" -ForegroundColor White
    Write-Host ""
    Write-Host "Commands:" -ForegroundColor Yellow
    Write-Host "  start     Start the proxy service"
    Write-Host "  stop      Stop the proxy service"
    Write-Host "  restart   Restart the proxy service"
    Write-Host "  status    Show proxy status"
    Write-Host "  config    Configure API key and settings"
    Write-Host "  logs      Show recent logs"
    Write-Host "  help      Show this help"
    Write-Host ""
    Write-Host "Quick Start:" -ForegroundColor Yellow
    Write-Host "  1. .\clinepass-proxy.ps1 config   # Set your ClinePass API key"
    Write-Host "  2. .\clinepass-proxy.ps1 start    # Start the proxy"
    Write-Host "  3. Configure CherryStudio to use http://127.0.0.1:55991/v1"
    Write-Host ""
}

# Main
switch ($Action) {
    "start"   { Start-Proxy }
    "stop"    { Stop-Proxy }
    "restart" { Stop-Proxy; Start-Sleep -Seconds 1; Start-Proxy }
    "status"  { Show-Status }
    "config"  { Set-Config }
    "logs"    { Show-Logs }
    "help"    { Show-Help }
    default   { Show-Help }
}
