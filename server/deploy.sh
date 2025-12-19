#!/bin/bash
set -euo pipefail

SERVER=65.21.180.95

echo "=== Building MicroVault Backend ==="
# Build the full module (prod config now lives in prod.go/main.go)
GOOS=linux GOARCH=amd64 go build -o microvault -ldflags "-s -w" .

echo "=== Building Next.js Frontend ==="
cd ..
# Load nvm and use Node 20
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
nvm use 20 || { echo "Node 20 not found. Install with: nvm install 20" >&2; exit 1; }

if command -v pnpm >/dev/null 2>&1; then
	pnpm install
	# Temporarily rename .env.local to avoid it overriding production env
	[ -f .env.local ] && mv .env.local .env.local.bak
	pnpm run build
	# Restore .env.local after build
	[ -f .env.local.bak ] && mv .env.local.bak .env.local
else
	echo "pnpm is required to build the frontend" >&2
	exit 1
fi
cd server

echo "=== Uploading Backend ==="
ssh root@$SERVER "mkdir -p /root/microvault"
scp microvault deploy/install.sh deploy/microvault.service deploy/config.yaml root@$SERVER:/root/microvault/

FRONT_VAULT_DIR=/var/www/vault.skatkis-tech.net
FRONT_MICRO_DIR=/var/www/microvault.skatkis-tech.net

echo "=== Uploading Client ==="
ssh root@$SERVER "mkdir -p $FRONT_VAULT_DIR $FRONT_MICRO_DIR"
scp -r ../out/* root@$SERVER:$FRONT_MICRO_DIR/

echo "=== Uploading Nginx Configs ==="
scp deploy/microvault.nginx.conf root@$SERVER:/etc/nginx/sites-available/content.skatkis-tech.net.conf
scp deploy/microvault-client.nginx.conf root@$SERVER:/etc/nginx/sites-available/microvault.skatkis-tech.net.conf

echo "=== Setting Permissions ==="
ssh root@$SERVER "chown -R www-data:www-data $FRONT_MICRO_DIR"
ssh root@$SERVER "chmod -R 755 $FRONT_MICRO_DIR"
ssh root@$SERVER "find $FRONT_MICRO_DIR -type f -exec chmod 644 {} \;"

echo "=== Enabling Nginx Sites ==="
ssh root@$SERVER "ln -sf /etc/nginx/sites-available/content.skatkis-tech.net.conf /etc/nginx/sites-enabled/"
ssh root@$SERVER "ln -sf /etc/nginx/sites-available/microvault.skatkis-tech.net.conf /etc/nginx/sites-enabled/"

echo "=== Testing and Reloading Nginx ==="
ssh root@$SERVER "nginx -t && systemctl reload nginx"

echo "=== Installing and Starting MicroVault Service ==="
ssh root@$SERVER "cd /root/microvault && chmod +x install.sh && ./install.sh"

echo ""
echo "=== Deployment Complete ==="
echo "Served through cloudflare"
echo "Backend: https://content.skatkis-tech.net"
echo "Client:  https://microvault.skatkis-tech.net (Next.js)"
echo ""
echo "Configuration uses Google OAuth from /root/client.json"



