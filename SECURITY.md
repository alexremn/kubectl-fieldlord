# Security Policy

## Supported versions

| Version | Supported |
|---------|-----------|
| 0.1.x   | ✅ |

kubectl-fieldlord is pre-1.0; only the latest released minor receives fixes.

## Reporting a vulnerability

Please report security issues **privately** via a GitHub Security Advisory:

➡️ https://github.com/alexremn/kubectl-fieldlord/security/advisories/new

Do **not** open a public issue for a suspected vulnerability.

We aim to acknowledge reports within **7 days** and to provide a remediation
plan or fix timeline shortly after triage.

## Scope

kubectl-fieldlord v0.1 is **read-only**: `explain` and `drift` issue only
`GET`/`LIST` requests against the apiserver named in your kubeconfig. It runs no
background services, opens no listening ports, and makes no network calls other
than to that apiserver. It collects no telemetry.

The highest-value reports are therefore:

- Any code path where the tool could **mutate** a cluster (it must not in v0.1).
- Any path where it could send data **anywhere other than** the configured apiserver.
- Leakage of credentials, tokens, or kubeconfig contents into output, logs, or errors.
- Crashes or panics triggered by hostile/malformed `managedFields` or API responses.

When the `predict` feature lands in v0.2 it will issue a server-side apply with
`DryRun: ["All"]` (non-persisting); its security model and webhook side-effect
disclosure will be documented here at that time.
