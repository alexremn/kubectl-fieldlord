# kubectl-fieldlord

Make Kubernetes Server-Side Apply field ownership legible.

`kubectl fieldlord` parses `managedFields` on any object and tells you who owns each field, whether ownership has drifted from who you expect to be in charge, and — with `predict` — exactly which fields a `--force-conflicts` apply would clobber before you run it. It plugs into your existing `kubectl` workflow.

---

## Safety & Privacy

- **`explain` and `drift` are read-only.** They issue only GET requests and never modify your cluster.
- **`predict` is a non-persisting dry-run.** It issues a Server-Side Apply with `DryRun:["All"]` and `Force:false` — no change is persisted. However, the dry-run does invoke server-side mutating and validating admission webhooks. Webhooks that declare `sideEffects: None` or `sideEffects: NoneOnDryRun` are safe. Webhooks lacking that declaration may have real side effects. Run `predict` against a non-production cluster first when your webhook posture is unknown.
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

Once v0.2.0 is published and the krew-index PR is accepted:

```bash
kubectl krew install fieldlord
kubectl fieldlord --version
```

See [docs/krew-submission.md](docs/krew-submission.md) for the owner runbook.

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

### `drift -f` — manifest-attributed drift (offline)

Compare a desired manifest against the live object and attribute every changed
field to its owner in `managedFields`.

```
kubectl fieldlord drift <resource> -f <manifest> [--expect-manager <name>] [--include-status]
```

`-f` accepts a file path or `-` for stdin. Exactly one resource may be
specified. The diff is computed entirely offline after the initial GET — no
dry-run is issued.

**Example:**

```bash
kubectl fieldlord drift deploy/api -f desired.yaml --expect-manager helm
```

Default table output:

```
FIELD                                                  EXPECTED   ACTUAL-MANAGER  OPERATION  CHANGE    ATTRIBUTED
.spec.replicas                                         helm       keda-operator   Apply      Modified  conflict
.spec.template.spec.containers[name="app"].image       helm       helm            Apply      Modified  self-change
.spec.template.spec.containers[name="app"].resources   helm       -               -          Added     addition
```

**JSON output:**

```bash
kubectl fieldlord drift deploy/api -f desired.yaml --expect-manager helm -o json
```

```json
{
  "schemaVersion": "v1",
  "command": "drift",
  "resource": {
    "group": "apps",
    "version": "v1",
    "kind": "Deployment",
    "namespace": "default",
    "name": "api"
  },
  "findings": [
    {
      "path": ".spec.replicas",
      "actualOwner": {
        "manager": "keda-operator",
        "operation": "Apply",
        "apiVersion": "apps/v1"
      },
      "change": "Modified",
      "conflict": true
    },
    {
      "path": ".spec.template.spec.containers[name=\"app\"].image",
      "actualOwner": {
        "manager": "helm",
        "operation": "Apply",
        "apiVersion": "apps/v1"
      },
      "change": "Modified",
      "conflict": false
    },
    {
      "path": ".spec.template.spec.containers[name=\"app\"].resources",
      "change": "Added"
    }
  ],
  "warnings": []
}
```

`change`, `conflict`, and `granularity` are `omitempty` — they are absent in
native drift output and only present in manifest mode.

**Exit codes for `drift -f`:**

| Code | Meaning |
|------|---------|
| `0`  | No conflicts. Self-changes, additions, and degraded findings do not gate exit 2. |
| `1`  | Decode error, not-exactly-one-resource, or other runtime error. |
| `2`  | At least one **conflict** (a Modified or Removed field owned by a manager other than `--expect-manager`). Only emitted when `--expect-manager` is named. |

**Empty `--expect-manager` is informational.** Without a named baseline, every
finding is classified as self-change, addition, or degraded — exit `2` is never
produced. Manifest mode does not infer a primary applier.

**`--include-status`** opts the `status` subresource into the diff. By default
status fields are excluded from both sides before diffing.

**Schema degradation (CRDs without a schema):** when the live cluster has no
OpenAPI v3 schema for the resource type, the diff falls back to a deduced
converter that treats lists as atomic. Affected paths are reported at
containing-list granularity and labeled `granularity: "degraded"` in JSON output,
with a warning. The paths are never wrong-keyed — they are coarser than ideal.

**Canonicalization residual:** a value the author writes that the apiserver
normalizes on admission (e.g. resource quantity `"1"` → `"1Gi"`) will appear as
a Modified field and may be flagged as a conflict even though the author's intent
is satisfied. This is a structural limitation of offline diffing. `predict` (which
issues an actual SSA dry-run) does not have this issue.

**Pointer to `predict`:** for the server-authoritative clobber set (exactly which
fields a `--force-conflicts` apply would overwrite), use `predict`. `drift -f`
and `predict` answer different questions and can legitimately return different
results on the same resource — this is expected, not a bug. See
[Three-way exit semantics](#three-way-exit-semantics).

---

### Three-way exit semantics

Native `drift`, `drift -f`, and `predict` answer three distinct questions. They
can legitimately differ on exit code for the same resource:

| Tool | Question answered | Conflict definition | Schema needed |
|------|------------------|---------------------|---------------|
| `drift` (native) | Which live fields are owned by a manager I didn't expect? | Any field owned by a non-expected manager | No |
| `drift -f` | Which desired-vs-live changes are owned by someone else? | Desired field Modified/Removed by a non-expected manager | Yes (degrades without) |
| `predict` | Which fields would `--force-conflicts` overwrite? | apiserver-reported SSA conflict set | Yes (server-side) |

A resource may show exit `2` from `predict` (a field your manager would
force-take from another) while `drift -f` shows exit `0` (the desired and live
values already match for that field, so no diff to attribute). Conversely,
`drift -f` may show a conflict for a field the apiserver would not report in a
dry-run, due to the canonicalization residual. Treat each tool's output as the
answer to its own specific question.

---

### `predict` — clobber predictor (SSA dry-run)

Show which fields a `--force-conflicts` apply would clobber and who currently owns them.

```
kubectl fieldlord predict <resource> -f <manifest> --as-manager <name>
```

Both `-f` (file path or `-` for stdin) and `--as-manager` are required. Exactly one resource may be specified per invocation.

`predict` issues a Server-Side Apply dry-run (`DryRun:["All"]`, `Force:false`) against the live object. The conflict set returned — the fields a `--force-conflicts` would overwrite — is the output. No change is persisted. See the [Safety & Privacy](#safety--privacy) section for webhook considerations.

`predict` requires Kubernetes >= 1.22 (Server-Side Apply GA). It fast-fails with an error on older servers.

**Example:**

```bash
kubectl fieldlord predict deploy/api -f desired.yaml --as-manager argocd-controller
```

Default table output:

```
FIELD                             CLOBBERS-MANAGER       OPERATION
.spec.replicas                    helm-controller        Apply
.spec.template.spec.containers    another-manager        Apply
```

**JSON output:**

```bash
kubectl fieldlord predict deploy/api -f desired.yaml --as-manager argocd-controller -o json
```

```json
{
  "schemaVersion": "v1",
  "command": "predict",
  "resource": {
    "group": "apps",
    "version": "v1",
    "kind": "Deployment",
    "namespace": "default",
    "name": "api"
  },
  "findings": [
    {
      "path": ".spec.replicas",
      "lowConfidence": false,
      "currentOwner": {
        "manager": "helm-controller",
        "operation": "Apply",
        "apiVersion": "apps/v1"
      }
    }
  ]
}
```

`lowConfidence: true` is set on servers affected by kubernetes/kubernetes#119141
(~1.27 range) where the conflict set may be incomplete.

**Warnings:**

- If `--as-manager` owns no Apply-operation fields on the live object, `predict`
  prints a warning (the result may still be useful).

**Exit codes for `predict`:**

| Code | Meaning |
|------|---------|
| `0`  | No conflicts — the apply would succeed without `--force-conflicts`. |
| `1`  | Runtime error; OR the server returned a non-409 response (e.g. webhook dry-run failure, "could not predict"); OR Kubernetes < 1.22. |
| `2`  | Non-empty clobber set — at least one field would be force-taken. Use this as your CI gate. |

---

## Exit codes

| Code | Command | Meaning |
|------|---------|---------|
| `0`  | all | Success. No attributed drift (`drift`/`drift -f`), no clobber conflicts (`predict`), or field ownership table (`explain`, always 0). |
| `1`  | all | Runtime error: authentication failure, resource not found, decode error, cardinality error, unsupported server version, or "could not predict" (non-409 dry-run failure). |
| `2`  | `drift`, `drift -f`, `predict` | Diagnostic signal: attributed drift found (`drift` native), conflict owned by another manager when `--expect-manager` is named (`drift -f`), or non-empty clobber set (`predict`). Use as CI gate. |

`drift -f` without a named `--expect-manager` never produces exit `2` — the mode
is informational. See [Three-way exit semantics](#three-way-exit-semantics) for
when the three tools can legitimately disagree on the same resource.

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
