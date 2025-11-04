#!/bin/bash

# quick-fix-use-service-ip.sh - Update secret to use PostgreSQL service IP instead of hostname

set -e

# Colors
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}Getting PostgreSQL service IP...${NC}"

# Get the ClusterIP of the PostgreSQL service
SERVICE_IP=$(kubectl get svc pg-ha-cluster -n demo -o jsonpath='{.spec.clusterIP}')

if [ -z "$SERVICE_IP" ]; then
    echo -e "${YELLOW}Could not find PostgreSQL service in demo namespace${NC}"
    echo "Available services:"
    kubectl get svc -n demo
    exit 1
fi

echo -e "${GREEN}Found PostgreSQL service IP: $SERVICE_IP${NC}"
echo ""

echo -e "${BLUE}Updating Kubernetes secret to use IP address...${NC}"

# Encode the IP
ENCODED_IP=$(echo -n "$SERVICE_IP" | base64)

# Update the secret
kubectl patch secret pg-load-test-secret -n demo --type='json' -p="[{'op': 'replace', 'path': '/data/DB_HOST', 'value':'$ENCODED_IP'}]"

echo -e "${GREEN}✓ Secret updated successfully${NC}"
echo ""

echo -e "${BLUE}New configuration:${NC}"
echo "  DB_HOST: $SERVICE_IP"
echo "  DB_PORT: 5432"
echo ""

echo -e "${YELLOW}Now delete the failed job and redeploy:${NC}"
echo "  kubectl delete job pg-load-test-job -n demo"
echo "  kubectl apply -f k8s/03-job.yaml"
echo ""
