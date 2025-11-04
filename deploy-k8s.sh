#!/bin/bash

# One-Command Kubernetes Deployment Script
# Deploys PostgreSQL Load Test Client to Kubernetes

set -e

export DB_HOST="pg-ha-cluster.demo.svc.cluster.local"
export DB_PORT="5432"
export DB_USER="postgres"
export DB_PASSWORD="SVuwzFvw!HJf;vLm"
export DB_NAME="postgres"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color



# Check if kubectl can connect to cluster
echo -e "${YELLOW}Checking Kubernetes cluster connection...${NC}"
if ! kubectl cluster-info &> /dev/null; then
    echo -e "${RED}Error: Cannot connect to Kubernetes cluster.${NC}"
    echo "Please configure kubectl to connect to your cluster."
    exit 1
fi

echo -e "${GREEN}✓ Connected to Kubernetes cluster${NC}\n"


IMAGE_NAME="souravbiswassanto/pg-load-test:latest"

# if [ -f "Dockerfile" ]; then
#     docker build -t $IMAGE_NAME . || {
#         echo -e "${RED}Error: Docker build failed${NC}"
#         exit 1
#     }
#     echo -e "${GREEN}✓ Docker image built successfully${NC}\n"
#     docker push $IMAGE_NAME || {
#         echo -e "${YELLOW}Warning: Failed to push image to registry. Continuing anyway...${NC}"
#     }
# else
#     echo -e "${RED}Error: Dockerfile not found${NC}"
#     exit 1
# fi

# # Load image to kind if using kind
# if kubectl config current-context | grep -q "kind"; then
#     echo -e "${YELLOW}Detected kind cluster, loading image...${NC}"
#     kind load docker-image $IMAGE_NAME || {
#         echo -e "${YELLOW}Warning: Failed to load image to kind. Continuing anyway...${NC}"
#     }
#     echo -e "${GREEN}✓ Image loaded to kind${NC}\n"
# fi

# # Encode database credentials
# echo -e "${YELLOW}Configuring database credentials...${NC}"

# Check if .env file exists
if [ -f ".env" ]; then
    echo -e "${GREEN}✓ Found .env file${NC}"
    
    # Source .env file
    export $(cat .env | grep -v '^#' | xargs)
    
    # Encode credentials
    DB_HOST_B64=$(echo -n "$DB_HOST" | base64)
    DB_PORT_B64=$(echo -n "$DB_PORT" | base64)
    DB_USER_B64=$(echo -n "$DB_USER" | base64)
    DB_PASSWORD_B64=$(echo -n "$DB_PASSWORD" | base64)
    DB_NAME_B64=$(echo -n "$DB_NAME" | base64)
    
    # Update secret file
    sed -i.bak "s|DB_HOST:.*|DB_HOST: $DB_HOST_B64|g" k8s/02-secret.yaml
    sed -i.bak "s|DB_PORT:.*|DB_PORT: $DB_PORT_B64|g" k8s/02-secret.yaml
    sed -i.bak "s|DB_USER:.*|DB_USER: $DB_USER_B64|g" k8s/02-secret.yaml
    sed -i.bak "s|DB_PASSWORD:.*|DB_PASSWORD: $DB_PASSWORD_B64|g" k8s/02-secret.yaml
    sed -i.bak "s|DB_NAME:.*|DB_NAME: $DB_NAME_B64|g" k8s/02-secret.yaml
    
    echo -e "${GREEN}✓ Database credentials encoded${NC}\n"
else
    echo -e "${YELLOW}Warning: .env file not found. Using default credentials from k8s/02-secret.yaml${NC}"
    echo -e "${YELLOW}Please update k8s/02-secret.yaml with your database credentials${NC}\n"
fi

# Deploy to Kubernetes
echo -e "${YELLOW}Deploying to Kubernetes...${NC}"

kubectl apply -f k8s/ || {
    echo -e "${RED}Error: Kubernetes deployment failed${NC}"
    exit 1
}

echo -e "${GREEN}✓ Resources deployed successfully${NC}\n"

# Wait for job to start
echo -e "${YELLOW}Waiting for job to start...${NC}"
sleep 5

# Get pod name
POD_NAME=$(kubectl get pods -n demo -l app=pg-load-test --no-headers -o custom-columns=":metadata.name" 2>/dev/null | head -n 1)

if [ -z "$POD_NAME" ]; then
    echo -e "${RED}Error: Could not find pod${NC}"
    echo "Check deployment status with: kubectl get pods -n demo"
    exit 1
fi

echo -e "${GREEN}✓ Job started: $POD_NAME${NC}\n"

# Show instructions
echo -e "${BLUE}=================================================="
echo "Deployment Complete!"
echo -e "==================================================${NC}\n"

echo -e "${GREEN}Next steps:${NC}\n"
echo -e "1. Watch logs:"
echo -e "   ${YELLOW}kubectl logs -f -n demo $POD_NAME${NC}\n"

echo -e "2. Check job status:"
echo -e "   ${YELLOW}kubectl get jobs -n demo${NC}\n"

echo -e "3. Get all resources:"
echo -e "   ${YELLOW}kubectl get all -n demo -l app=pg-load-test${NC}\n"

echo -e "4. View final results (after completion):"
echo -e "   ${YELLOW}kubectl logs -n demo $POD_NAME${NC}\n"

echo -e "5. Cleanup:"
echo -e "   ${YELLOW}kubectl delete job,configmap,secret,pvc -n demo -l app=pg-load-test${NC}\n"

# Ask if user wants to follow logs
read -p "Would you like to follow the logs now? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}Following logs (Ctrl+C to exit)...${NC}\n"
    kubectl logs -f -n demo $POD_NAME
fi
