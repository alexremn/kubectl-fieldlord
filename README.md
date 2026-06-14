# kubectl-fieldlord

Make Kubernetes Server-Side Apply field ownership legible.

> **Early release — v0.1.** This release ships `explain` and `drift`. The `predict` clobber-predictor (SSA dry-run) is coming in v0.2.

`kubectl fieldlord` parses `managedFields` on any object and tells you who owns each field, and whether ownership has drifted from who you expect to be in charge. It is a read-only diagnostic tool that plugs into your existing `kubectl` workflow.

---

## Safety & Privacy

- **Read-only.** `explain` and `drift` only issue GET requests. They never modify your cluster.
- **Talks only to your configured apiserver.** No data leaves your network. No telemetry, no analytics, no phone-home, no LLM.
- **Respects your existing kubeconfig.** All standard kubectl config flags (`--context`, `--kubeconfig`, `-n/--namespace`, etc.) are honored.

---

## Install

### From source (v0.1)

Requires Go 1.26 or later (the pinned `k8s.io` v0.36 libraries require it).

```bash
go install github.com/alexremn/kubectl-fieldlord/cmd/kubectl-fieldlord@latest
```

`go install` builds and places the binary as `kubectl-fieldlord` in `$GOPATH/bin` (or `$GOBIN`). Ensure that directory is on your `PATH`, then `kubectl` will find it as a plugin:

```bash
kubectl fieldlord --version
```

### krew

Krew distribution is planned for v0.2.

---

## Usage

### `explain` — per-field ownership table

Show who owns each field of a resource, decoded from `managedFields`.

```
kubectl fieldlord explain <resource>...
```

Aliases: `own`, `who`

**Example:**

```bash
kubectl fieldlord explain deploy/api
```

Default table output:

```
# Deployment/api (server v1.29.2, tier full)
FIELD                                    MANAGER        OPERATION   APIVERSION   SUBRESOURCE   TIME                   MULTI
.metadata.annotations[kubectl.k...]      kubectl        Update      v1                         2024-03-10T08:00:00Z
.spec.replicas                           helm           Apply       apps/v1                    2024-03-10T09:00:00Z
.spec.template.spec.containers[{api}]…  helm           Apply       apps/v1                    2024-03-10T09:00:00Z
.status.availableReplicas                kube-ctrl-mgr  Update      apps/v1      status        2024-03-10T09:01:00Z
```

The `MULTI` column shows `*` when more than one distinct manager claims the same field.

**JSON output:**

```bash
kubectl fieldlord explain deploy/api -o json
```

```json
{
  "schemaVersion": "v1",
  "command": "explain",
  "resource": {
    "group": "apps",
    "version": "v1",
    "kind": "Deployment",
    "namespace": "default",
    "name": "api"
  },
  "serverVersion": "v1.29.2",
  "supportTier": "full",
  "findings": [
    {
      "path": ".spec.replicas",
      "atomic": false,
      "multiOwner": false,
      "owners": [
        {
          "manager": "helm",
          "operation": "Apply",
          "apiVersion": "apps/v1",
          "time": "2024-03-10T09:00:00Z"
        }
      ]
    }
  ],
  "warnings": []
}
```

When querying multiple resources with `-o json`, the output is a top-level JSON array of envelopes.

### `drift` — attribute ownership drift

Flag fields owned by a manager other than the expected one.

```
kubectl fieldlord drift <resource>... [--expect-manager <name>]
```

Without `--expect-manager`, `drift` infers the primary applier: the `Apply`-operation manager that owns the most leaf fields (tie-break: most leaves → most-recent time → alphabetical name). Subresource fields (e.g. `status`, `scale`) are excluded from comparison against the main applier.

**Example — inferred primary:**

```bash
kubectl fieldlord drift deploy/api
```

**Example — explicit manager:**

```bash
kubectl fieldlord drift deploy/api --expect-manager helm
```

**CI gate pattern:**

```bash
kubectl fieldlord drift deploy/api --expect-manager helm -o json
echo "Exit code: $?"
# Exit code 2 if attributed drift is found; 0 if clean; 1 on error.
```

---

## Exit codes

| Code | Meaning |
|------|---------|
| `0`  | Success, no attributed drift (or `explain`, which always exits 0). |
| `1`  | Runtime error: authentication failure, resource not found, cannot infer primary applier (no Apply-operation manager exists and `--expect-manager` was not passed), etc. |
| `2`  | Attributed drift found (`drift` only). At least one field is owned by a manager other than the expected one. This is the CI-gate signal. |

---

## Global flags

| Flag | Default | Description |
|------|---------|-------------|
| `-o`, `--output` | `table` | Output format: `table`, `json`, or `yaml`. |
| `--no-color` | `false` | Disable colored table output. Also honors the `NO_COLOR` environment variable. |
| `--skip-version-check` | `false` | Skip the server-version capability probe at startup. |
| `-n`, `--namespace` | (from kubeconfig) | Kubernetes namespace. |
| `--context` | (from kubeconfig) | Kubeconfig context to use. |
| `--kubeconfig` | `~/.kube/config` | Path to kubeconfig file. |

All standard `kubectl` config flags from `k8s.io/cli-runtime` are supported.

---

## Kubernetes compatibility

| Kubernetes version | Support tier | Notes |
|-------------------|-------------|-------|
| >= 1.22 | **Full** | SSA is GA; all features work as documented. |
| 1.18 – 1.21 | **Best-effort** | `explain` and `drift` work; SSA was beta in this range. |
| < 1.18 | **Unsupported** | A warning is printed at startup. |

A capability probe runs at startup and prints a tier warning when the server is below full support. Suppress it with `--skip-version-check`.

---

## Shell completion

```bash
kubectl fieldlord completion bash   >> ~/.bash_completion
kubectl fieldlord completion zsh    >> ~/.zshrc
kubectl fieldlord completion fish   > ~/.config/fish/completions/kubectl_fieldlord.fish
kubectl fieldlord completion powershell
```

---

## Prior art & acknowledgements

kubectl-fieldlord would not exist without the pioneering work of:

- [ahmetb/kubectl-fields](https://github.com/ahmetb/kubectl-fields) — field path exploration for Kubernetes objects.
- [tt-kuma/kubectl-colorize-managed-fields](https://github.com/tt-kuma/kubectl-colorize-managed-fields) — colorized rendering of `managedFields`.

---

## License

Apache-2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
