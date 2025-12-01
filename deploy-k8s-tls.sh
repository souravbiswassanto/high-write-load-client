#!/bin/bash

# PostgreSQL Load Test - TLS Deployment Script (Full Certificate Authentication)
# This script deploys the load testing job with full certificate authentication

set -e

NAMESPACE="demo"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "🚀 Deploying PostgreSQL Load Test with TLS (Full Certificate Auth)"
echo "=================================================="

# Check if namespace exists
if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
    echo "❌ Error: Namespace '$NAMESPACE' does not exist!"
    echo "Please create it first: kubectl create namespace $NAMESPACE"
    exit 1
fi

# Check if PostgreSQL cluster secret exists
if ! kubectl get secret pg-ha-cluster-client-cert -n "$NAMESPACE" &> /dev/null; then
    echo "❌ Error: Secret 'pg-ha-cluster-client-cert' not found in namespace '$NAMESPACE'"
    echo "Please ensure your PostgreSQL cluster is running and has generated client certificates"
    exit 1
fi

echo "✅ Namespace '$NAMESPACE' exists"
echo "✅ TLS certificates found"

# Clean up any existing deployment
echo ""
echo "🧹 Cleaning up any existing deployment..."
kubectl delete job pg-load-test-tls -n "$NAMESPACE" --ignore-not-found=true
kubectl delete configmap pg-load-test-config-tls -n "$NAMESPACE" --ignore-not-found=true
kubectl delete secret pg-load-test-secret-tls -n "$NAMESPACE" --ignore-not-found=true
sleep 2

# Apply Kubernetes manifests
echo ""
echo "📋 Applying Kubernetes manifests..."
kubectl apply -f "$SCRIPT_DIR/k8s-tls/01-configmap.yaml"
kubectl apply -f "$SCRIPT_DIR/k8s-tls/02-secret.yaml"
kubectl apply -f "$SCRIPT_DIR/k8s-tls/03-job.yaml"

echo ""
echo "✅ Deployment complete!"
echo ""
echo "📊 Monitor the job with:"
echo "   kubectl get jobs -n $NAMESPACE -w"
echo ""
echo "📋 View job status:"
echo "   kubectl describe job pg-load-test-tls -n $NAMESPACE"
echo ""
echo "📜 View logs:"
echo "   kubectl logs -f job/pg-load-test-tls -n $NAMESPACE"
echo ""
echo "🔍 Debug pod (if needed):"
echo "   kubectl get pods -n $NAMESPACE -l app=pg-load-test-tls"
echo ""
echo "=================================================="
