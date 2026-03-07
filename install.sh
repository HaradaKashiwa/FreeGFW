#!/bin/bash

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting FreeGFW installation...${NC}"

# Check for root privileges
if [ "$EUID" -ne 0 ]; then 
  echo -e "${RED}Please run as root (use sudo)${NC}"
  exit 1
fi

# Install Docker if not installed
if ! command -v docker &> /dev/null; then
    echo -e "${YELLOW}Docker not found. Installing Docker...${NC}"
    curl -fsSL https://get.docker.com | bash -s docker
    if [ $? -ne 0 ]; then
        echo -e "${RED}Failed to install Docker. Please install it manually.${NC}"
        exit 1
    fi
    systemctl enable docker
    systemctl start docker
    echo -e "${GREEN}Docker installed successfully.${NC}"
else
    echo -e "${GREEN}Docker is already installed.${NC}"
fi

# Deploy FreeGFW using Docker
echo -e "${YELLOW}Deploying FreeGFW...${NC}"
# Remove existing container if it exists
if docker ps -a --format '{{.Names}}' | grep -q "^freegfw$"; then
    echo -e "${YELLOW}Removing existing freegfw container...${NC}"
    docker rm -f freegfw
fi

docker run -d --name freegfw --network=host \
  --restart unless-stopped \
  -v "freegfw:/data" \
  ghcr.io/haradakashiwa/freegfw
  
if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to deploy FreeGFW. Check the Docker error above.${NC}"
    exit 1
fi

# Get the server IP address
GET_IP=$(curl -s -4 https://api.ipify.org || curl -s -4 https://ifconfig.me || hostname -I | awk '{print $1}')
if [ -z "$GET_IP" ]; then
    GET_IP="<your_server_ip>"
fi

echo -e ""
echo -e "${GREEN}================================================================${NC}"
echo -e "${GREEN}FreeGFW deployed successfully!${NC}"
echo -e "${GREEN}================================================================${NC}"
echo -e ""
echo -e "You can now access the FreeGFW dashboard at:"
echo -e "${YELLOW}http://${GET_IP}:8080${NC}"
echo -e ""
echo -e "${RED}Please access this site as soon as possible and set a password to ensure security.${NC}"
echo -e "Enjoy!"