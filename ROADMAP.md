# Roadmap

kubectl-fieldlord is built in phases. Each phase produces working, testable
software on its own.

## v0.1 — Foundation (this release)

- `explain` — per-field ownership table decoded from `managedFields`, keyed on
  `(manager, operation, apiVersion, subresource)`.
- `drift` (native mode) — attribute ownership drift to a `fieldManager`, with an
  inferred-primary-applier default and `--expect-manager` override.
- `version`, shell `completion`.
- `table` / `json` / `yaml` output with a versioned envelope; CI-gate exit codes.
- Server-version capability probe (warns, never blocks).
- Full unit + golden-file coverage; envtest integration test.

## v0.2 — Public launch ✅ DELIVERED

- **`predict`** — clobber predictor: a Server-Side Apply with `DryRun:["All"]`,
  `Force:false` to report exactly which fields a `--force-conflicts` apply would
  overwrite, and which manager currently owns them. The headline feature.
  - Requires Kubernetes >= 1.22 (SSA GA); fast-fails on older servers.
  - Exit codes: `0` clean / `2` non-empty clobber set (CI gate) / `1` error or
    "could not predict" (non-409 dry-run failure or server below floor).
  - `lowConfidence` flag on ~1.27 servers affected by kubernetes/kubernetes#119141.
- **Distribution**: GoReleaser v2 → GitHub Releases for linux/darwin/windows ×
  amd64/arm64.
- **Supply-chain**: cosign keyless signing (Sigstore bundle), SPDX + CycloneDX
  SBOMs, SLSA provenance.
- **krew**: manifest single-sourced by GoReleaser's `krews:` block; PR to
  `kubernetes-sigs/krew-index` opened automatically on tag push. See
  [docs/krew-submission.md](docs/krew-submission.md) for the owner runbook.
- **CodeQL** static analysis workflow.
- **Public repository launch** (post-merge owner action: flip repo public, then
  push `v0.2.0` tag → triggers release workflow + krew PR).

**Design-spec reconciliation (supersedes TECH_SPEC.md §14 table):**

- *Manifest-mode drift* (`drift -f`): this item was listed in the v0.2 spec
  table. It has **converged into `predict`** — `predict` delivers the
  server-authoritative clobber set (the fields a `--force-conflicts` apply
  would overwrite), which is the actionable output manifest-mode drift was
  intended to surface. A full desired-vs-live attributed diff remains a
  possible v0.3 item.
- *OpenAPI-schema-aware atomic/list-type labeling*: **moved to v0.3**.
  Not present in v0.2.

## v0.3 — Hardening

- OpenAPI-schema-aware atomic/list-type labeling (moved from v0.2).
- Performance and bounded iteration for very large objects (e.g. objects with
  many `managedFields` entries or large `FieldsV1` blobs).
- Per-entry-vs-served APIVersion mismatch surfacing.
- Full desired-vs-live attributed diff (building on `predict`'s conflict set).

## Later

- Open-core: an Argo/Flux-aware fleet dashboard or CI admission gate built on the
  same ownership/drift/predict engine.

## Explicitly deferred from v0.1

`predict`, krew/GoReleaser distribution, supply-chain signing, Windows builds,
and OpenAPI-schema atomic labeling were **not present in v0.1** and were
scheduled as above.

Note: `drift -f` (manifest-mode drift) was listed as a v0.2 item in the design
spec but did not ship as a separate flag — its purpose converged into `predict`.
See the v0.2 design-spec reconciliation note above.
