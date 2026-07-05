<#
.SYNOPSIS
    使用 SSH 上传文件到远程服务器
#>

param(
    [string]$Apikey
)

if (!$Apikey) {
    Write-Host "请提供 ClinePass API Key"
    Write-Host "用法: .\upload.ps1 <api-key>"
    exit 1
}

$ErrorActionPreference = "Stop"

# 服务器配置
$Host = "154.219.110.138"
$User = "root"
$Password = "Zc2002119"

# 本地文件
$LocalBinary = "C:\Users\hayou\Desktop\OH-WorkSpace\_tools\clinepass-proxy\bin\clinepass-proxy"
$LocalDeploy = "C:\Users\hayou\Desktop\OH-WorkSpace\_tools\clinepass-proxy\deploy\deploy.sh"

# 远程目录
$RemoteDir = "/tmp/clinepass-deploy"

Write-Host "=== ClinePass Proxy 文件上传脚本 ===" -ForegroundColor Cyan
Write-Host ""
Write-Host "目标服务器: ${User}@${Host}"
Write-Host "API Key: $($Apikey.Substring(0, [Math]::Min(8, $Apikey.Length)))..."
Write-Host ""

# 创建远程目录
Write-Host "1. 创建远程目录..." -ForegroundColor Yellow
ssh -o StrictHostKeyChecking=no "${User}@${Host}" "mkdir -p $RemoteDir"
if ($LASTEXITCODE -ne 0) {
    Write-Host "✗ 创建目录失败" -ForegroundColor Red
    exit 1
}
Write-Host "✓ 目录创建成功" -ForegroundColor Green
Write-Host ""

# 上传二进制文件
Write-Host "2. 上传二进制文件..." -ForegroundColor Yellow
scp -o StrictHostKeyChecking=no $LocalBinary "${User}@${Host}:${RemoteDir}/clinepass-proxy"
if ($LASTEXITCODE -ne 0) {
    Write-Host "✗ 上传二进制文件失败" -ForegroundColor Red
    exit 1
}
Write-Host "✓ 二进制文件上传成功" -ForegroundColor Green
Write-Host ""

# 上传部署脚本
Write-Host "3. 上传部署脚本..." -ForegroundColor Yellow
scp -o StrictHostKeyChecking=no $LocalDeploy "${User}@${Host}:${RemoteDir}/deploy.sh"
if ($LASTEXITCODE -ne 0) {
    Write-Host "✗ 上传部署脚本失败" -ForegroundColor Red
    exit 1
}
Write-Host "✓ 部署脚本上传成功" -ForegroundColor Green
Write-Host ""

# 设置执行权限
Write-Host "4. 设置执行权限..." -ForegroundColor Yellow
ssh -o StrictHostKeyChecking=no "${User}@${Host}" "chmod +x ${RemoteDir}/clinepass-proxy ${RemoteDir}/deploy.sh"
if ($LASTEXITCODE -ne 0) {
    Write-Host "✗ 设置权限失败" -ForegroundColor Red
    exit 1
}
Write-Host "✓ 权限设置成功" -ForegroundColor Green
Write-Host ""

# 执行部署脚本
Write-Host "5. 执行部署脚本..." -ForegroundColor Yellow
ssh -o StrictHostKeyChecking=no "${User}@${Host}" "cd $RemoteDir && ./deploy.sh $Apikey"
if ($LASTEXITCODE -ne 0) {
    Write-Host "✗ 部署失败" -ForegroundColor Red
    exit 1
}
Write-Host ""
Write-Host "✓ 部署完成!" -ForegroundColor Green
Write-Host ""
Write-Host "API 端点:" -ForegroundColor Cyan
Write-Host "  - http://${Host}:55991/v1/chat/completions"
Write-Host "  - http://${Host}:55991/v1/models"
Write-Host "  - http://${Host}:55991/health"
Write-Host ""
Write-Host "CherryStudio 配置:" -ForegroundColor Cyan
Write-Host "  API 地址: http://${Host}:55991/v1"
Write-Host "  API Key:  任意值"
Write-Host "  模型:     glm-5.2 / kimi-k2.7-code / deepseek-v4-pro 等"
