# Kubernetes starter manifests

These manifests are a local-cluster starter, not a production security configuration.

Build and load the images into your cluster, then apply:

```bash
docker build -t gitcastle-api:dev .
docker build -t gitcastle-frontend:dev ./frontend
kubectl apply -k deploy/k8s
```

Before production use, replace the development PostgreSQL secret, use a managed database or a reviewed StatefulSet strategy, configure an Ingress with TLS, and move repository storage to durable backup-tested storage.
