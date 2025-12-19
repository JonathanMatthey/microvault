#!/bin/bash
set -euo pipefail

# Ensure we run from the directory containing this script (server/)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

SERVER=65.21.180.95

echo "=== Building SelfStack Backend ==="
# Build the full module (prod config now lives in prod.go/main.go)
GOOS=linux GOARCH=amd64 go build -o selfstack -ldflags "-s -w" .

echo "=== Building Next.js Frontend ==="
cd ..
# Ensure Node 20 is available (prefer nvm if present, otherwise use system node)
if command -v nvm >/dev/null 2>&1; then
	export NVM_DIR="$HOME/.nvm"
	[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
	nvm install 20 >/dev/null 2>&1 || true
	nvm use 20 || { echo "Failed to use Node 20 via nvm" >&2; exit 1; }
else
	if command -v node >/dev/null 2>&1; then
		NODE_MAJOR=$(node -p "process.versions.node.split('.')[0]" || echo 0)
		if [ "$NODE_MAJOR" -lt 20 ]; then
			echo "Node >= 20 is required. Found $(node -v)." >&2; exit 1
		fi
	else
		echo "Node 20 is required but not found." >&2; exit 1
	fi
fi

# Ensure pnpm is available (try corepack if missing)
if ! command -v pnpm >/dev/null 2>&1; then
	if command -v corepack >/dev/null 2>&1; then
		corepack enable || true
		corepack prepare pnpm@latest --activate || true
	fi
fi

if command -v pnpm >/dev/null 2>&1; then
	pnpm install
	# Temporarily rename .env.local to avoid it overriding production env
	[ -f .env.local ] && mv .env.local .env.local.bak
	pnpm run build
	# Restore .env.local after build
	[ -f .env.local.bak ] && mv .env.local .env.local
else
	echo "pnpm is required to build the frontend" >&2
	exit 1
fi
cd server

echo "=== Uploading Backend ==="
ssh root@$SERVER "mkdir -p /root/selfstack"
scp selfstack deploy/install.sh deploy/selfstack.service deploy/config.yaml root@$SERVER:/root/selfstack/

FRONT_VAULT_DIR=/var/www/vault.skatkis-tech.net
FRONT_SELFSTACK_DIR=/var/www/selfstack.skatkis-tech.net

echo "=== Uploading Client ==="
ssh root@$SERVER "mkdir -p $FRONT_VAULT_DIR $FRONT_SELFSTACK_DIR"
scp -r ../out/* root@$SERVER:$FRONT_SELFSTACK_DIR/

echo "=== Uploading Nginx Configs ==="
scp deploy/selfstack.nginx.conf root@$SERVER:/etc/nginx/sites-available/content.skatkis-tech.net.conf
scp deploy/selfstack-client.nginx.conf root@$SERVER:/etc/nginx/sites-available/selfstack.skatkis-tech.net.conf

echo "=== Setting Permissions ==="
ssh root@$SERVER "chown -R www-data:www-data $FRONT_SELFSTACK_DIR"
ssh root@$SERVER "chmod -R 755 $FRONT_SELFSTACK_DIR"
ssh root@$SERVER "find $FRONT_SELFSTACK_DIR -type f -exec chmod 644 {} \;"

echo "=== Enabling Nginx Sites ==="
ssh root@$SERVER "ln -sf /etc/nginx/sites-available/content.skatkis-tech.net.conf /etc/nginx/sites-enabled/"
ssh root@$SERVER "ln -sf /etc/nginx/sites-available/selfstack.skatkis-tech.net.conf /etc/nginx/sites-enabled/"

echo "=== Testing and Reloading Nginx ==="
ssh root@$SERVER "nginx -t && systemctl reload nginx"

echo "=== Installing and Starting SelfStack Service ==="
ssh root@$SERVER "cd /root/selfstack && chmod +x install.sh && ./install.sh"

echo ""
echo "=== Deployment Complete ==="
echo "Served through cloudflare"
echo "Backend: https://content.skatkis-tech.net"
echo "Client:  https://selfstack.skatkis-tech.net (Next.js)"
echo ""
echo "Configuration uses Google OAuth from /root/client.json"



