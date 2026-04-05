#!/bin/bash

# Obsideo Storage Provider Deployment Script
# This script installs the provider as a systemd service.

set -e

# Configuration
INSTALL_DIR="/opt/obsideo-provider"
SERVICE_NAME="obsideo-provider"
USER_NAME="obsideo"

# Ensure script is run as root
if [ "$EUID" -ne 0 ]; then
  echo "Please run as root (use sudo)"
  exit 1
fi

echo "--- Starting deployment of Obsideo Storage Provider ---"

# 1. Build the binary (if go is installed)
if command -v go &> /dev/null; then
    echo "Building provider binary..."
    go build -o provider .
else
    if [ ! -f "./provider" ]; then
        echo "Error: 'go' is not installed and no 'provider' binary found in current directory."
        echo "Please build the binary first or install Go."
        exit 1
    fi
    echo "Using existing 'provider' binary."
fi

# 2. Check for required configuration files
if [ ! -f "config.yaml" ]; then
    echo "Warning: config.yaml not found. Copying from example..."
    cp config.example.yaml config.yaml
fi

if [ ! -f "coordinator_pub.pem" ]; then
    echo "Warning: coordinator_pub.pem not found."
    echo "The provider will fail to start without the coordinator's public key."
    echo "Please ensure it is placed in $INSTALL_DIR/ after this script completes."
fi

# 3. Create dedicated user if it doesn't exist
if ! id -u "$USER_NAME" &>/dev/null; then
    echo "Creating system user '$USER_NAME'..."
    useradd -r -s /sbin/nologin "$USER_NAME"
fi

# 4. Prepare installation directory
echo "Installing files to $INSTALL_DIR..."
install -d -o "$USER_NAME" "$INSTALL_DIR"
cp provider config.yaml "$INSTALL_DIR/"

# Copy coordinator_pub.pem if it exists
if [ -f "coordinator_pub.pem" ]; then
    cp coordinator_pub.pem "$INSTALL_DIR/"
fi

chown -R "$USER_NAME":"$USER_NAME" "$INSTALL_DIR"

# 5. Setup systemd service
echo "Configuring systemd service..."
if [ -f "obsideo-provider.service" ]; then
    cp obsideo-provider.service /etc/systemd/system/
else
    echo "Creating systemd service file..."
    cat <<EOF > /etc/systemd/system/$SERVICE_NAME.service
[Unit]
Description=Obsideo storage provider
After=network.target

[Service]
Type=simple
User=$USER_NAME
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/provider start --config config.yaml
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
fi

# 6. Enable and start service
echo "Starting service..."
systemctl daemon-reload
systemctl enable "$SERVICE_NAME"
systemctl restart "$SERVICE_NAME"

echo "--- Deployment Complete ---"
echo "Service status:"
systemctl status "$SERVICE_NAME" --no-pager
echo ""
echo "To view logs, run: journalctl -u $SERVICE_NAME -f"
