#!/bin/bash

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting FreeGFW Systemd Service installation...${NC}"

# Check for root privileges
if [ "$EUID" -ne 0 ]; then 
  echo -e "${RED}Please run as root (use sudo)${NC}"
  exit 1
fi

# Detect architecture
ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then
    RELEASE_ARCH="amd64"
elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
    RELEASE_ARCH="arm64"
else
    echo -e "${RED}Unsupported architecture: $ARCH${NC}"
    exit 1
fi

echo -e "${YELLOW}Detected architecture: ${RELEASE_ARCH}${NC}"

# Stop and remove existing Docker container if it exists to avoid port conflicts
if command -v docker &> /dev/null && docker ps -a --format '{{.Names}}' | grep -q "^freegfw$"; then
    echo -e "${YELLOW}Stopping existing FreeGFW Docker container to avoid port conflicts...${NC}"
    docker stop freegfw
    docker rm freegfw
    echo -e "${YELLOW}Note: Data from the Docker container is NOT automatically migrated to /var/lib/freegfw.${NC}"
fi

# Fetch the latest release tag
echo -e "${YELLOW}Fetching latest release version...${NC}"
LATEST_TAG=$(curl -s https://api.github.com/repos/HaradaKashiwa/FreeGFW/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_TAG" ]; then
    echo -e "${RED}Failed to get the latest release version. Please check your network or GitHub API limits.${NC}"
    exit 1
fi

echo -e "${GREEN}Latest version: ${LATEST_TAG}${NC}"

# Download binary
DOWNLOAD_URL="https://github.com/HaradaKashiwa/FreeGFW/releases/download/${LATEST_TAG}/freegfw-linux-${RELEASE_ARCH}.tar.gz"
echo -e "${YELLOW}Downloading FreeGFW from ${DOWNLOAD_URL}...${NC}"

curl -L -o /tmp/freegfw.tar.gz "$DOWNLOAD_URL"
if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to download the binary.${NC}"
    exit 1
fi

# Extract and install
echo -e "${YELLOW}Installing binary to /usr/local/bin/freegfw...${NC}"
tar -xzf /tmp/freegfw.tar.gz -C /tmp
if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to extract the archive.${NC}"
    exit 1
fi

mv /tmp/freegfw-linux-${RELEASE_ARCH} /usr/local/bin/freegfw
chmod +x /usr/local/bin/freegfw
rm -f /tmp/freegfw.tar.gz

# Setup data directory
echo -e "${YELLOW}Setting up working directory at /var/lib/freegfw...${NC}"
mkdir -p /var/lib/freegfw

# Create systemd service
echo -e "${YELLOW}Creating systemd service...${NC}"
cat <<EOF > /etc/systemd/system/freegfw.service
[Unit]
Description=FreeGFW Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/var/lib/freegfw
ExecStart=/usr/local/bin/freegfw
Restart=always
RestartSec=5
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF

# Enable and start service
echo -e "${YELLOW}Starting FreeGFW service...${NC}"
systemctl daemon-reload
systemctl enable freegfw
systemctl restart freegfw

if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to start FreeGFW service. Check logs with: journalctl -u freegfw${NC}"
    exit 1
fi

# Get the server IP address
GET_IP=$(curl -s -4 https://api.ipify.org || curl -s -4 https://ifconfig.me || hostname -I | awk '{print $1}')
if [ -z "$GET_IP" ]; then
    GET_IP="<your_server_ip>"
fi

echo -e ""
echo -e "${GREEN}================================================================${NC}"
echo -e "${GREEN}FreeGFW deployed successfully as a Systemd service!${NC}"
echo -e "${GREEN}================================================================${NC}"
echo -e ""
echo -e "You can now access the FreeGFW dashboard at:"
echo -e "${YELLOW}http://${GET_IP}:8080${NC}"
echo -e ""
echo -e "${RED}Please access this site as soon as possible and set a password to ensure security.${NC}"
echo -e "To view logs, run: ${YELLOW}journalctl -fu freegfw${NC}"
echo -e "Enjoy!"
