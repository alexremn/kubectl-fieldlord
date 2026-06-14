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

## v0.2 — Public launch

- **`predict`** — clobber predictor: a server-side apply with `DryRun:["All"]`,
  `Force:false` to report exactly which fields a `--force-conflicts` apply would
  overwrite, and which manager currently owns them. The headline feature.
- **Manifest-mode drift** — `drift <resource> -f desired.yaml`: diff a desired
  manifest against live state and attribute each differing field to its owner.
- **krew distribution** + GoReleaser release automation.
- **Windows** binaries; macOS/Linux on amd64/arm64.
- **Supply-chain**: checksums, cosign keyless signing, SBOMs, SLSA provenance.
- OpenAPI-schema-aware atomic/list-type labeling.
- Public repository launch.

## v0.3 — Hardening

- Performance and bounded iteration for very large objects (e.g. objects with
  many `managedFields` entries or large `FieldsV1` blobs).
- Per-entry-vs-served APIVersion mismatch surfacing.

## Later

- Open-core: an Argo/Flux-aware fleet dashboard or CI admission gate built on the
  same ownership/drift/predict engine.

## Explicitly deferred from v0.1

`predict`, manifest-mode drift (`-f`), krew/GoReleaser distribution,
supply-chain signing, Windows builds, and OpenAPI-schema atomic labeling are all
**not present in v0.1** and are scheduled as above.
