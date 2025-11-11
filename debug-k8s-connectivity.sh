#!/bin/bash

# debug-k8s-connectivity.sh - Debug PostgreSQL connectivity issues in Kubernetes

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}==================================================================${NC}"
echo -e "${BLUE}PostgreSQL Load Test - Kubernetes Connectivity Debug${NC}"
echo -e "${BLUE}==================================================================${NC}"
echo ""

# Configuration
NAMESPACE="demo"
PG_HOST="pg-ha-cluster.demo.svc.cluster.local"
PG_PORT="5432"
PG_USER="postgres"
PG_PASSWORD="AZ1XyTmDXh.Hh.77"
PG_DATABASE="postgres"

echo -e "${YELLOW}Step 1: Checking if namespace exists...${NC}"
if kubectl get namespace $NAMESPACE &> /dev/null; then
    echo -e "${GREEN}✓ Namespace $NAMESPACE exists${NC}"
else
    echo -e "${RED}✗ Namespace $NAMESPACE does not exist${NC}"
    exit 1
fi
echo ""

echo -e "${YELLOW}Step 2: Checking if PostgreSQL service exists in demo namespace...${NC}"
if kubectl get svc pg-ha-cluster -n demo &> /dev/null; then
    echo -e "${GREEN}✓ PostgreSQL service exists in demo namespace${NC}"
    kubectl get svc pg-ha-cluster -n demo
else
    echo -e "${RED}✗ PostgreSQL service not found in demo namespace${NC}"
    echo "Available services in demo namespace:"
    kubectl get svc -n demo
fi
echo ""

echo -e "${YELLOW}Step 3: Checking PostgreSQL endpoints...${NC}"
if kubectl get endpoints pg-ha-cluster -n demo &> /dev/null; then
    echo -e "${GREEN}✓ PostgreSQL endpoints exist${NC}"
    kubectl get endpoints pg-ha-cluster -n demo
else
    echo -e "${RED}✗ PostgreSQL endpoints not found${NC}"
fi
echo ""

echo -e "${YELLOW}Step 4: Deploying debug pod...${NC}"
kubectl apply -f k8s/debug-pod.yaml
echo "Waiting for debug pod to be ready..."
kubectl wait --for=condition=ready pod/pg-debug -n $NAMESPACE --timeout=60s
echo -e "${GREEN}✓ Debug pod is ready${NC}"
echo ""

echo -e "${YELLOW}Step 5: Testing DNS resolution...${NC}"
echo "Resolving: $PG_HOST"
kubectl exec -n $NAMESPACE pg-debug -- nslookup $PG_HOST || echo -e "${RED}✗ DNS resolution failed${NC}"
echo ""

echo -e "${YELLOW}Step 6: Testing network connectivity (ping)...${NC}"
kubectl exec -n $NAMESPACE pg-debug -- ping -c 3 pg-ha-cluster.demo.svc.cluster.local || echo -e "${YELLOW}⚠ Ping failed (this is normal if ICMP is blocked)${NC}"
echo ""

echo -e "${YELLOW}Step 7: Testing TCP connectivity to PostgreSQL port...${NC}"
kubectl exec -n $NAMESPACE pg-debug -- timeout 5 bash -c "cat < /dev/null > /dev/tcp/$PG_HOST/$PG_PORT" && \
    echo -e "${GREEN}✓ TCP connection to $PG_HOST:$PG_PORT successful${NC}" || \
    echo -e "${RED}✗ TCP connection to $PG_HOST:$PG_PORT failed${NC}"
echo ""

echo -e "${YELLOW}Step 8: Testing PostgreSQL connection with psql...${NC}"
kubectl exec -n $NAMESPACE pg-debug -- psql -h $PG_HOST -p $PG_PORT -U $PG_USER -d $PG_DATABASE -c "SELECT version();" && \
    echo -e "${GREEN}✓ PostgreSQL connection successful${NC}" || \
    echo -e "${RED}✗ PostgreSQL connection failed${NC}"
echo ""

echo -e "${YELLOW}Step 9: Checking network policies...${NC}"
if kubectl get networkpolicies -n $NAMESPACE &> /dev/null; then
    echo "Network policies in $NAMESPACE:"
    kubectl get networkpolicies -n $NAMESPACE
else
    echo -e "${GREEN}✓ No network policies restricting traffic${NC}"
fi
echo ""

if kubectl get networkpolicies -n demo &> /dev/null; then
    echo "Network policies in demo namespace:"
    kubectl get networkpolicies -n demo
else
    echo -e "${GREEN}✓ No network policies in demo namespace${NC}"
fi
echo ""

echo -e "${YELLOW}Step 10: Checking if PostgreSQL pods are running...${NC}"
kubectl get pods -n demo -l app=pg-ha-cluster || echo "No PostgreSQL pods found with label app=pg-ha-cluster"
echo ""

echo -e "${BLUE}==================================================================${NC}"
echo -e "${BLUE}Debug Summary${NC}"
echo -e "${BLUE}==================================================================${NC}"
echo ""
echo -e "${YELLOW}If all tests passed:${NC}"
echo "  The issue might be with:"
echo "  - Connection timeout (already set to 30s)"
echo "  - Application code"
echo "  - Docker image not rebuilt with latest changes"
echo ""
echo -e "${YELLOW}If DNS resolution failed:${NC}"
echo "  Check if the service name is correct"
echo "  Try using the service IP directly"
echo "  Service IP: \$(kubectl get svc pg-ha-cluster -n demo -o jsonpath='{.spec.clusterIP}')"
echo ""
echo -e "${YELLOW}If TCP connection failed:${NC}"
echo "  Check network policies blocking traffic"
echo "  Check if PostgreSQL is listening on the correct port"
echo "  Verify PostgreSQL pods are running"
echo ""
echo -e "${YELLOW}If PostgreSQL connection failed:${NC}"
echo "  Check credentials"
echo "  Check pg_hba.conf settings"
echo "  Check PostgreSQL logs"
echo ""
echo -e "${YELLOW}Cleanup debug pod:${NC}"
echo "  kubectl delete pod pg-debug -n $NAMESPACE"
echo ""
