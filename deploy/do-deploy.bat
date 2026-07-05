@echo off
REM ClinePass Proxy v2 Deploy Script
"%~dp0\..\..\..\..\Windows\System32\OpenSSH\ssh.exe" -o StrictHostKeyChecking=no root@154.219.110.138 "mkdir -p /opt/clinepass-proxy"
