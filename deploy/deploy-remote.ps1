<#
.SYNOPSIS
    Deploy ClinePass Proxy to HK server
#>

param(
    [string]$Apikey
)

if (!$Apikey) {
    Write-Host "Please provide ClinePass API Key"
    Write-Host "Usage: .\deploy-remote.ps1 <api-key>"
    exit 1
}

$ErrorActionPreference = "Stop"

# Server config
$ServerHost = "154.219.110.138"
$ServerUser = "root"
$ServerPass = "Zc2002119"

# Local files
$BinFile = "C:\Users\hayou\Desktop\OH-WorkSpace\_tools\clinepass-proxy\bin\clinepass-proxy"
$ScriptFile = "C:\Users\hayou\Desktop\OH-WorkSpace\_tools\clinepass-proxy\deploy\deploy.sh"

# Remote directory
$DeployDir = "/tmp/clinepass-deploy"

Write-Host "=== ClinePass Proxy Deploy ===" -ForegroundColor Cyan
Write-Host ""
Write-Host "Server: $ServerUser@$ServerHost"
Write-Host "API Key: $($Apikey.Substring(0, [Math]::Min(8, $Apikey.Length)))..."
Write-Host ""

# Helper function using cmd.exe
function Run-Remote {
    param(
        [string]$Command,
        [int]$Delay = 1000
    )
    
    $p = New-Object System.Diagnostics.Process
    $p.StartInfo.FileName = "cmd.exe"
    $p.StartInfo.Arguments = "/c $Command"
    $p.StartInfo.UseShellExecute = $false
    $p.StartInfo.RedirectStandardInput = $true
    $p.StartInfo.RedirectStandardOutput = $true
    $p.StartInfo.RedirectStandardError = $true
    $p.StartInfo.CreateNoWindow = $true
    $p.Start() | Out-Null
    
    # Wait for password prompt
    Start-Sleep -Milliseconds $Delay
    $p.StandardInput.WriteLine($ServerPass)
    $p.StandardInput.Close()
    $p.WaitForExit()
    
    return @{
        Code = $p.ExitCode
        Out = $p.StandardOutput.ReadToEnd()
        Err = $p.StandardError.ReadToEnd()
    }
}

# Step 1: Create directory
Write-Host "1. Create directory..." -ForegroundColor Yellow
$cmd = "ssh -o StrictHostKeyChecking=no $ServerUser@$ServerHost `"mkdir -p $DeployDir`""
$r = Run-Remote -Command $cmd
if ($r.Code -ne 0) {
    Write-Host "FAIL: $($r.Err)" -ForegroundColor Red
    exit 1
}
Write-Host "OK" -ForegroundColor Green

# Step 2: Upload binary
Write-Host "2. Upload binary..." -ForegroundColor Yellow
$cmd = "scp -o StrictHostKeyChecking=no `"$BinFile`" $ServerUser@$ServerHost`:$DeployDir/clinepass-proxy"
$r = Run-Remote -Command $cmd
if ($r.Code -ne 0) {
    Write-Host "FAIL: $($r.Err)" -ForegroundColor Red
    exit 1
}
Write-Host "OK" -ForegroundColor Green

# Step 3: Upload script
Write-Host "3. Upload script..." -ForegroundColor Yellow
$cmd = "scp -o StrictHostKeyChecking=no `"$ScriptFile`" $ServerUser@$ServerHost`:$DeployDir/deploy.sh"
$r = Run-Remote -Command $cmd
if ($r.Code -ne 0) {
    Write-Host "FAIL: $($r.Err)" -ForegroundColor Red
    exit 1
}
Write-Host "OK" -ForegroundColor Green

# Step 4: Set permissions
Write-Host "4. Set permissions..." -ForegroundColor Yellow
$cmd = "ssh -o StrictHostKeyChecking=no $ServerUser@$ServerHost `"chmod +x $DeployDir/clinepass-proxy $DeployDir/deploy.sh`""
$r = Run-Remote -Command $cmd
if ($r.Code -ne 0) {
    Write-Host "FAIL: $($r.Err)" -ForegroundColor Red
    exit 1
}
Write-Host "OK" -ForegroundColor Green

# Step 5: Run deploy
Write-Host "5. Deploy..." -ForegroundColor Yellow
$cmd = "ssh -o StrictHostKeyChecking=no $ServerUser@$ServerHost `"cd $DeployDir && ./deploy.sh $Apikey`""
$r = Run-Remote -Command $cmd
Write-Host $r.Out
if ($r.Code -ne 0) {
    Write-Host "FAIL: $($r.Err)" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "=== Deploy Done ===" -ForegroundColor Green
Write-Host ""
Write-Host "API Endpoints:" -ForegroundColor Cyan
Write-Host "  http://$ServerHost`:55991/v1/chat/completions"
Write-Host "  http://$ServerHost`:55991/v1/models"
Write-Host "  http://$ServerHost`:55991/health"
Write-Host ""
Write-Host "CherryStudio Config:" -ForegroundColor Cyan
Write-Host "  URL: http://$ServerHost`:55991/v1"
Write-Host "  Key: any"
Write-Host "  Models: glm-5.2 / kimi-k2.7-code / deepseek-v4-pro"
