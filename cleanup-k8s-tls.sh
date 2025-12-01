#!/bin/bash

# PostgreSQL Load Test - TLS Cleanup Script

set -e

NAMESPACE="demo"

echo "🧹 Cleaning up PostgreSQL Load Test TLS deployment"
echo "=================================================="

echo "Deleting Job..."
kubectl delete job pg-load-test-tls -n "$NAMESPACE" --ignore-not-found=true

echo "Deleting ConfigMap..."
kubectl delete configmap pg-load-test-config-tls -n "$NAMESPACE" --ignore-not-found=true

echo "Deleting Secret..."
kubectl delete secret pg-load-test-secret-tls -n "$NAMESPACE" --ignore-not-found=true

echo ""
echo "✅ Cleanup complete!"
echo ""
echo "Note: TLS certificates (pg-ha-cluster-client-cert) are preserved"
echo "      as they are managed by the PostgreSQL cluster"
echo ""
