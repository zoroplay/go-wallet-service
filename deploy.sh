#!/usr/bin/env bash

set -e  # Exit on any error

# Change to correct directory
cd ~/bonus-service

# Load environment variables..
if [ -f .env ]; then
    set -a  # automatically export all variables
    source .env
    set +a  # stop automatically exporting
else
    echo "Warning: .env file not found"
fi

# Create network if it doesn't exist
docker network create sbenet || true

# Stop existing containers
docker compose down || true

# Pull latest images
docker compose pull

# Start services
docker compose up -d

# Show running containers......
docker ps

echo "Deployment completed successfully"

     
