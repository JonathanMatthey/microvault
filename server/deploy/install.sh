#!/bin/bash
set -e

echo "=== MicroVault Installation Script ==="

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    echo "Error: This script must be run as root"
    exit 1
fi

# Create microvault user if it doesn't exist
if ! id -u microvault >/dev/null 2>&1; then
    echo "Creating microvault user..."
    useradd -r -s /bin/false microvault
else
    echo "User microvault already exists"
fi

# Create necessary directories
echo "Creating directories..."
mkdir -p /var/lib/microvault/data
mkdir -p /var/lib/microvault/uploads
mkdir -p /etc/microvault

# Set ownership
echo "Setting ownership..."
chown -R microvault:microvault /var/lib/microvault

# Copy binary to /usr/local/bin
echo "Installing binary..."
# Stop service first to release the binary
if systemctl is-active --quiet microvault.service; then
    echo "Stopping microvault service..."
    systemctl stop microvault.service
    
    # Wait for service to fully stop
    sleep 2
    
    # Verify it's stopped
    for i in {1..10}; do
        if ! systemctl is-active --quiet microvault.service; then
            echo "Service stopped successfully"
            break
        fi
        echo "Waiting for service to stop... ($i/10)"
        sleep 1
    done
    
    if systemctl is-active --quiet microvault.service; then
        echo "Warning: Service did not stop cleanly, forcing kill"
        systemctl kill -s SIGKILL microvault.service
        sleep 1
    fi
fi

cp microvault /usr/local/bin/microvault
chmod 755 /usr/local/bin/microvault

# Copy configuration file
echo "Installing configuration..."
cp config.yaml /etc/microvault/config.yaml
chown microvault:microvault /etc/microvault/config.yaml
chmod 644 /etc/microvault/config.yaml

# Copy Google OAuth credentials if they exist in /root
if [ -f /root/client.json ]; then
    echo "Copying Google OAuth credentials..."
    cp /root/client.json /etc/microvault/client.json
    chown microvault:microvault /etc/microvault/client.json
    chmod 600 /etc/microvault/client.json
fi

# Copy systemd service file
echo "Installing systemd service..."
cp microvault.service /etc/systemd/system/microvault.service
chmod 644 /etc/systemd/system/microvault.service

# Reload systemd
echo "Reloading systemd..."
systemctl daemon-reload

# Enable service to start on boot
echo "Enabling microvault service..."
systemctl enable microvault.service

# Restart service
echo "Restarting microvault service..."
systemctl restart microvault.service

# Wait a moment for service to start
sleep 2

# Check service status
echo ""
echo "=== Service Status ==="
systemctl status microvault.service --no-pager || true

echo ""
echo "=== Installation Complete ==="
echo "Service is running and enabled to start on boot"
echo "View logs with: journalctl -u microvault -f"

