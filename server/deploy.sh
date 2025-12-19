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
	[ -f .env.local.bak ] && mv .env.local.bak .env.local
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

echo "=== Verifying Certbot Certificates ==="
ssh root@$SERVER '
	set -euo pipefail
	domains=("selfstack.skatkis-tech.net" "content.skatkis-tech.net")
	need_issue=()
	echo "Checking certbot allocations for: ${domains[*]}"

	if command -v certbot >/dev/null 2>&1; then
		certbot --version || true
	else
		echo "Error: certbot not found on server PATH" >&2
		exit 1
	fi

	for d in "${domains[@]}"; do
		echo "--- $d ---"
		live="/etc/letsencrypt/live/$d"
		if [ -d "$live" ]; then
			ls -l "$live" || true
			if [ ! -f "$live/fullchain.pem" ] || [ ! -f "$live/privkey.pem" ]; then
				echo "Missing fullchain.pem or privkey.pem in $live" >&2
				need_issue+=("$d")
			else
				if command -v openssl >/dev/null 2>&1; then
					echo "Certificate metadata:"
					openssl x509 -in "$live/fullchain.pem" -noout -issuer -subject -dates || true
				fi
				echo "certbot certificates for $d:" && certbot certificates --domain "$d" || true
			fi
			[ -f "/etc/letsencrypt/renewal/$d.conf" ] || { echo "Missing renewal config: /etc/letsencrypt/renewal/$d.conf" >&2; need_issue+=("$d"); }
		else
			echo "Missing live directory: $live" >&2
			need_issue+=("$d")
		fi
	done

	if [ "${#need_issue[@]}" -gt 0 ]; then
		echo "Attempting to issue certificates for: ${need_issue[*]}"
		# Ensure nginx is running and will serve challenges
		systemctl status nginx >/dev/null 2>&1 || systemctl start nginx || true
		for d in "${need_issue[@]}"; do
			if certbot certificates --domain "$d" >/dev/null 2>&1; then
				echo "certbot reports an entry for $d but live files missing; attempting reinstall"
			fi
			if [ -n "${CERTBOT_EMAIL:-}" ]; then
				emailArgs=(--email "$CERTBOT_EMAIL")
			else
				emailArgs=(--register-unsafely-without-email)
			fi
			# Prefer nginx plugin to handle challenge config automatically
			certbot --nginx --non-interactive --agree-tos --redirect "${emailArgs[@]}" -d "$d" || {
				echo "Failed to issue certificate for $d via nginx plugin" >&2
				exit 1
			}
		done
	fi

	# Show certbot timers if available
	if command -v systemctl >/dev/null 2>&1; then
		systemctl list-timers --all | grep -E "certbot|letsencrypt" || true
		systemctl status certbot.timer 2>/dev/null || true
	fi

	# Final verification
	for d in "${domains[@]}"; do
		live="/etc/letsencrypt/live/$d"
		if [ ! -f "$live/fullchain.pem" ] || [ ! -f "$live/privkey.pem" ]; then
			echo "Certificate for $d is still missing after issuance attempt." >&2
			exit 1
		fi
	done
'

echo "=== Testing and Reloading Nginx ==="
ssh root@$SERVER '
  # Capture nginx test output
  nginx_output=$(nginx -t 2>&1)
  
  # Filter out selfstack certificate errors
  filtered_output=$(echo "$nginx_output" | grep -v "cannot load certificate.*selfstack")
  
  # Check if there are any real errors (test failed) after filtering
  if echo "$filtered_output" | grep -q "test failed"; then
    echo "$filtered_output"
    exit 1
  fi
  
  # If we get here, either test passed or only had selfstack cert warnings
  echo "$filtered_output"
  systemctl reload nginx
'

echo "=== Installing and Starting SelfStack Service ==="
ssh root@$SERVER "cd /root/selfstack && chmod +x install.sh && ./install.sh"

echo ""
echo "=== Deployment Complete ==="
echo "Served through cloudflare"
echo "Backend: https://content.skatkis-tech.net"
echo "Client:  https://selfstack.skatkis-tech.net (Next.js)"
echo ""
echo "Configuration uses Google OAuth from /root/client.json"



