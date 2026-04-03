# Namespace Change Summary

All Kubernetes deployment scripts and manifests have been updated to deploy in the **demo** namespace instead of creating a separate **pg-load-test** namespace.

## Changes Made

### Kubernetes Manifests (k8s/ directory):

1. **00-namespace.yaml** - Commented out (demo namespace already exists)
2. **01-configmap.yaml** - Updated to use `namespace: demo`
3. **02-secret.yaml** - Updated to use `namespace: demo`
4. **03-job.yaml** - Updated to use `namespace: demo`
5. **04-pvc.yaml** - Updated to use `namespace: demo`
6. **debug-pod.yaml** - Updated to use `namespace: demo`

### Scripts:

1. **deploy-k8s.sh** - Updated all references from `pg-load-test` to `demo`
2. **cleanup-k8s.sh** - Updated to clean only load test resources in demo namespace (preserves PostgreSQL)
3. **debug-k8s-connectivity.sh** - Updated namespace to `demo`
4. **quick-fix-use-service-ip.sh** - Updated namespace to `demo`

## Why This Fixes the Issue

The error was happening because:
- The load test pod was in a separate `pg-load-test` namespace
- The PostgreSQL cluster is in the `demo` namespace
- Cross-namespace service discovery can have issues with DNS resolution
- Network policies might block cross-namespace traffic

By deploying in the same `demo` namespace:
- ✅ Direct service-to-service communication within the same namespace
- ✅ No DNS resolution issues
- ✅ No network policy conflicts
- ✅ Simplified networking

## How to Deploy

```bash
./deploy-k8s.sh
```

This will:
1. Build the Docker image
2. Deploy ConfigMap, Secret, Job, and PVC to the `demo` namespace
3. Start the load test in the same namespace as PostgreSQL

## Monitoring

```bash
# Watch logs
kubectl logs -f -n demo <pod-name>

# Check job status
kubectl get jobs -n demo -l app=pg-load-test

# Get all load test resources
kubectl get all,configmap,secret,pvc -n demo -l app=pg-load-test
```

## Cleanup

```bash
./cleanup-k8s.sh
```

This will:
- Delete ONLY the load test resources (Job, ConfigMap, Secret, PVC)
- Preserve the demo namespace and PostgreSQL cluster

Or manually:
```bash
kubectl delete job,configmap,secret,pvc -n demo -l app=pg-load-test
```

## Benefits

1. **Same Namespace**: Load test and PostgreSQL in the same namespace
2. **Simplified DNS**: Use `pg-ha-cluster` or `pg-ha-cluster.demo.svc.cluster.local`
3. **No Network Issues**: No cross-namespace networking complexities
4. **Cleaner Setup**: No need to create/manage additional namespace
