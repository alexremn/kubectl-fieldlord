# Examples

> **Warning:** Run these examples only on a **disposable** cluster (kind or k3d).
> Never apply them to a shared or production cluster.

## hpa-replicas-fight: seeing the ownership conflict

This example demonstrates the classic field-ownership fight where an HPA controller
continuously overwrites `spec.replicas` on a Deployment, conflicting with any other
field manager (Helm, kubectl apply, Argo CD, etc.) that also sets that field.

### Prerequisites

- A throwaway cluster: `kind create cluster` or `k3d cluster create demo`
- `kubectl-fieldlord` installed (see the root README)
- Note the kubectl and Kubernetes server versions you are using:
  ```
  kubectl version
  ```

### Steps

**1. Apply the manifest with a known field manager**

```bash
kubectl apply --server-side \
  --field-manager=demo \
  -f examples/hpa-replicas-fight.yaml
```

**2. Wait for the HPA to run its first reconcile**

```bash
kubectl get hpa replicas-fight --watch
```

Once the HPA reports `TARGETS` and has adjusted `REPLICAS`, the HPA controller
has written `spec.replicas` using its own field manager.

**3. Inspect field ownership with kubectl-fieldlord**

Show every field and its owner:

```bash
kubectl fieldlord explain deploy/replicas-fight
```

Check whether the `demo` manager still owns `spec.replicas`, or whether the HPA
controller has taken it over:

```bash
kubectl fieldlord drift deploy/replicas-fight --expect-manager demo
```

If `spec.replicas` is listed as a drift — owned by a manager other than `demo` —
you have reproduced the fight.

### What to look for

- `kubectl fieldlord explain` will show `spec.replicas` owned by
  `horizontal-pod-autoscaler` (the HPA controller's field manager name).
- `kubectl fieldlord drift --expect-manager demo` will flag `spec.replicas`
  because the `demo` manager no longer owns it.

### Cleanup

```bash
kind delete cluster   # or: k3d cluster delete demo
```
