# Security Policy

## Supported versions

| Version | Supported |
|---------|-----------|
| 0.2.x   | ✅ |
| 0.1.x   | ✅ |

kubectl-fieldlord is pre-1.0; only the latest released minor receives fixes.

## Reporting a vulnerability

Please report security issues **privately** via a GitHub Security Advisory:

➡️ https://github.com/alexremn/kubectl-fieldlord/security/advisories/new

Do **not** open a public issue for a suspected vulnerability.

We aim to acknowledge reports within **7 days** and to provide a remediation
plan or fix timeline shortly after triage.

## Scope

### `explain` and `drift` (v0.1+) — read-only

`explain` and `drift` issue only `GET`/`LIST` requests against the apiserver
named in your kubeconfig. They run no background services, open no listening
ports, and make no network calls other than to that apiserver. They collect no
telemetry.

### `predict` (v0.2+) — non-persisting SSA dry-run

`predict` issues a **Server-Side Apply dry-run** (`DryRun:["All"]`,
`Force:false`). The request does **not** persist any change to the cluster.
However, a dry-run apply does invoke server-side mutating and validating
admission webhooks:

- Webhooks that declare `sideEffects: None` or `sideEffects: NoneOnDryRun`
  honour the dry-run flag and produce no real side effects.
- Webhooks that do **not** declare one of those values may perform real side
  effects (external API calls, audit events, etc.) even though the apply
  itself is not persisted.

**Recommendation:** run `predict` against a non-production cluster first when
the `sideEffects` posture of the admission webhooks in that cluster is unknown.

**RBAC for `predict`:** the principal (user or service account) running
`predict` must have permission to `patch` (Server-Side Apply) the target
resource. A dry-run apply uses the same authorization path as a real apply.

### Highest-value reports (all versions)

- Any code path where the tool could **mutate** a cluster beyond the
  intentional dry-run in `predict`.
- Any path where the tool sends data **anywhere other than** the configured
  apiserver (no telemetry, no external calls).
- Leakage of credentials, tokens, or kubeconfig contents into output, logs,
  or error messages.
- Crashes or panics triggered by hostile/malformed `managedFields`,
  `FieldsV1` blobs, or API responses.
- RBAC privilege-escalation: `predict` must not acquire more privilege than
  the calling principal already holds.
