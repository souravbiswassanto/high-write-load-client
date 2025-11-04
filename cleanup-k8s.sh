#!/bin/bash

# cleanup-k8s.sh - Clean up Kubernetes resources created by deploy-k8s.sh
# This script removes all resources in the pg-load-test namespace

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}==================================================================${NC}"
echo -e "${BLUE}PostgreSQL Load Test - Kubernetes Cleanup${NC}"
echo -e "${BLUE}==================================================================${NC}"
echo ""

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}Error: kubectl is not installed${NC}"
    echo "Please install kubectl: https://kubernetes.io/docs/tasks/tools/"
    exit 1
fi

# Check if cluster is accessible
if ! kubectl cluster-info &> /dev/null; then
    echo -e "${RED}Error: Cannot connect to Kubernetes cluster${NC}"
    echo "Please check your kubeconfig and cluster status"
    exit 1
fi

echo -e "${YELLOW}This will delete the following resources in demo namespace:${NC}"
echo "  - Load test Jobs, ConfigMaps, Secrets, PVCs (with label app=pg-load-test)"
echo "  - Note: The demo namespace and PostgreSQL resources will NOT be deleted"
echo ""

# Check if namespace exists
if ! kubectl get namespace demo &> /dev/null; then
    echo -e "${YELLOW}Namespace 'demo' does not exist. Nothing to clean up.${NC}"
    exit 0
fi

# Show current resources
echo -e "${BLUE}Current load test resources in demo namespace:${NC}"
kubectl get all,configmap,secret,pvc -n demo -l app=pg-load-test 2>/dev/null || echo "No load test resources found"
echo ""

# Ask for confirmation
read -p "$(echo -e ${YELLOW}Are you sure you want to delete load test resources in demo namespace? [y/N]: ${NC})" -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}Cleanup cancelled${NC}"
    exit 0
fi

echo ""
echo -e "${BLUE}Deleting load test Kubernetes resources...${NC}"
echo ""

# Delete namespaced resources with label selector (in order)
echo -e "${YELLOW}Deleting Jobs...${NC}"
kubectl delete jobs -n demo -l app=pg-load-test --ignore-not-found=true

echo -e "${YELLOW}Deleting Pods...${NC}"
kubectl delete pods -n demo -l app=pg-load-test --ignore-not-found=true

echo -e "${YELLOW}Deleting ConfigMaps...${NC}"
kubectl delete configmaps -n demo -l app=pg-load-test --ignore-not-found=true

echo -e "${YELLOW}Deleting Secrets...${NC}"
kubectl delete secrets -n demo -l app=pg-load-test --ignore-not-found=true

echo -e "${YELLOW}Deleting PVCs...${NC}"
kubectl delete pvc -n demo -l app=pg-load-test --ignore-not-found=true

echo ""
echo -e "${GREEN}==================================================================${NC}"
echo -e "${GREEN}✅ Cleanup completed successfully!${NC}"
echo -e "${GREEN}==================================================================${NC}"
echo ""
echo -e "${BLUE}Load test resources have been removed:${NC}"
echo "  ✓ Jobs deleted"
echo "  ✓ Pods deleted"
echo "  ✓ ConfigMaps deleted"
echo "  ✓ Secrets deleted"
echo "  ✓ PVCs deleted"
echo ""
echo -e "${GREEN}Note: demo namespace and PostgreSQL cluster were preserved${NC}"
echo ""
