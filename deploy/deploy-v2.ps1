param(
    [string]$Action = "deploy",
    [string]$ApiKey = "sk_b1a9318b433fbbe02eef29711721201252b0d37d8075c20155bf69e2b74e5275"
)

$hostAddr = "154.219.110.138"
$user = "root"
$pass = "Zc2002119"

function Send-Command {
    param($command)
    $si = [System.Diagnostics.ProcessStartInfo]::new("ssh", "-o StrictHostKeyChecking=no $user@$hostAddr `"$command`"")
    $si.UseShellExecute = $false
    $si.RedirectStandardInput = $true
    $si.RedirectStandardOutput = $true
    $si.RedirectStandardError = $true
    $si.CreateNoWindow = $true
    $p = [System.Diagnostics.Process]::new()
    $p.StartInfo = $si
    $p.Start() | Out-Null
    Start-Sleep 1
    $p.StandardInput.WriteLine($pass)
    $p.StandardInput.Close()
    return $p.StandardOutput.ReadToEnd()
}

function Send-File {
    param($local, $remote)
    $si = [System.Diagnostics.ProcessStartInfo]::new("scp", "-o StrictHostKeyChecking=no `"$local`" $user@$hostAddr`:$remote")
    $si.UseShellExecute = $false
    $si.RedirectStandardInput = $true
    $si.RedirectStandardOutput = $true
    $si.RedirectStandardError = $true
    $si.CreateNoWindow = $true
    $p = [System.Diagnostics.Process]::new()
    $p.StartInfo = $si
    $p.Start() | Out-Null
    Start-Sleep 2
    $p.StandardInput.WriteLine($pass)
    $p.StandardInput.Close()
    $p.WaitForExit(60000)
    return $p.ExitCode
}

Write-Host "=== Deploy v2 ===" -ForegroundColor Cyan

# Upload binary
Write-Host "Uploading binary..." -ForegroundColor Yellow
$code = Send-File -local "C:\Users\hayou\Desktop\OH-WorkSpace\_tools\clinepass-proxy\bin\clinepass-proxy" -remote "/tmp/clinepass-proxy"
if ($code -ne 0) { Write-Host "FAILED" -ForegroundColor Red; exit 1 }
Write-Host "OK" -ForegroundColor Green

# Replace & restart
Write-Host "Deploying..." -ForegroundColor Yellow
$result = Send-Command "mv /tmp/clinepass-proxy /opt/clinepass-proxy/clinepass-proxy && chmod +x /opt/clinepass-proxy/clinepass-proxy && systemctl restart clinepass-proxy && sleep 1 && systemctl is-active clinepass-proxy"
Write-Host $result

# Verify
$status = Send-Command "curl -s http://127.0.0.1:55991/health"
Write-Host "Health: $status" -ForegroundColor Cyan

Write-Host "=== Done ===" -ForegroundColor Green
