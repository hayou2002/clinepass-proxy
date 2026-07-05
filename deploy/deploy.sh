#!/bin/bash
set -e

INSTALL_DIR="/opt/clinepass-proxy"
SERVICE_NAME="clinepass-proxy"
PORT=55991

if [ -z "$1" ]; then
    echo "Usage: $0 <api-key>"
    echo "Example: $0 sk_xxx..."
    exit 1
fi

API_KEY="$1"

if [ "$EUID" -ne 0 ]; then
    echo "Please run as root"
    exit 1
fi

echo "=== ClinePass Proxy v2 Deploy ==="
echo ""

mkdir -p "$INSTALL_DIR"
cp clinepass-proxy "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/clinepass-proxy"

cat > /etc/systemd/system/$SERVICE_NAME.service << EOF
[Unit]
Description=ClinePass Proxy v2 - OpenAI compatible API proxy for ClinePass
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/clinepass-proxy -api-key "$API_KEY" -host 0.0.0.0 -port $PORT
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable $SERVICE_NAME
systemctl restart $SERVICE_NAME

sleep 2
if systemctl is-active --quiet $SERVICE_NAME; then
    echo "OK - Deployed successfully"
    echo "API: http://<IP>:$PORT/v1"
    echo "Models: cline-pass/glm-5.2, cline-pass/deepseek-v4-pro, etc."
    echo ""
    echo "Manage: systemctl status $SERVICE_NAME"
    echo "Logs:   journalctl -u $SERVICE_NAME -f"
else
    echo "FAILED - check: journalctl -u $SERVICE_NAME -n 50"
    exit 1
fi