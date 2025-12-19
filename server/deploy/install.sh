#!/bin/bash
set -e

echo "=== SelfStack Installation Script ==="

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    echo "Error: This script must be run as root"
    exit 1
fi

# Create selfstack user if it doesn't exist
if ! id -u selfstack >/dev/null 2>&1; then
    echo "Creating selfstack user..."
    useradd -r -s /bin/false selfstack
else
    echo "User selfstack already exists"
fi

# Create necessary directories
echo "Creating directories..."
mkdir -p /var/lib/selfstack/data
mkdir -p /var/lib/selfstack/uploads
mkdir -p /etc/selfstack

# Set ownership
echo "Setting ownership..."
chown -R selfstack:selfstack /var/lib/selfstack

# Copy binary to /usr/local/bin
echo "Installing binary..."
# Stop service first to release the binary
if systemctl is-active --quiet selfstack.service; then
    echo "Stopping selfstack service..."
    systemctl stop selfstack.service

    # Wait for service to fully stop
    sleep 2

    # Verify it's stopped
    for i in {1..10}; do
        if ! systemctl is-active --quiet selfstack.service; then
            echo "Service stopped successfully"
            break
        fi
        echo "Waiting for service to stop... ($i/10)"
        sleep 1
    done

    if systemctl is-active --quiet selfstack.service; then
        echo "Warning: Service did not stop cleanly, forcing kill"
        systemctl kill -s SIGKILL selfstack.service
        sleep 1
    fi
fi

cp selfstack /usr/local/bin/selfstack
chmod 755 /usr/local/bin/selfstack

# Copy configuration file
echo "Installing configuration..."
cp config.yaml /etc/selfstack/config.yaml
chown selfstack:selfstack /etc/selfstack/config.yaml
chmod 644 /etc/selfstack/config.yaml

# Copy Google OAuth credentials if they exist in /root
if [ -f /root/client.json ]; then
    echo "Copying Google OAuth credentials..."
    cp /root/client.json /etc/selfstack/client.json
    chown selfstack:selfstack /etc/selfstack/client.json
    chmod 600 /etc/selfstack/client.json
fi

# Copy systemd service file
echo "Installing systemd service..."
cp selfstack.service /etc/systemd/system/selfstack.service
chmod 644 /etc/systemd/system/selfstack.service

# Reload systemd
echo "Reloading systemd..."
systemctl daemon-reload

# Enable service to start on boot
echo "Enabling selfstack service..."
systemctl enable selfstack.service

# Restart service
echo "Restarting selfstack service..."
systemctl restart selfstack.service

# Wait a moment for service to start
sleep 2

# Check service status
echo ""
echo "=== Service Status ==="
systemctl status selfstack.service --no-pager || true

echo ""
echo "=== Installation Complete ==="
echo "Service is running and enabled to start on boot"
echo "View logs with: journalctl -u selfstack -f"

