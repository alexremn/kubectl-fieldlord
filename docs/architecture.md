# Architecture

kubectl-fieldlord is a standard kubectl plugin (the `sample-cli-plugin` shape).
It is organized into small, focused packages that depend **downward only**, so
each layer can be understood and unit-tested in isolation.

## Package layout

```
cmd/kubectl-fieldlord/      # package main: entrypoint + exit-code mapping (run())
internal/buildinfo/         # ldflag-injected Version/Commit/Date
pkg/cmd/                    # cobra wiring: root, global flags, explain, drift, version
  root.go                   #   NewRootCmd: ConfigFlags + globalOptions as persistent flags
  options.go                #   cmdOptions (incl. injectable resolveFunc), validateOutput
  probe.go                  #   capability-probe helper (warn, never block)
  explain.go / drift.go     #   command construction + orchestration
  exit.go                   #   ExitError{Code, Err}
pkg/kube/                   # thin client layer: Resolve (resource.Builder), DetectTier
pkg/ownership/              # FieldsV1 decode (structured-merge-diff) + ownership model
  model.go                  #   Owner, OwnedPath, Model
  decode.go                 #   decodeEntry: one managedFields entry -> leaf paths
  build.go                  #   Build: aggregate entries -> sorted Model + inverted index
pkg/drift/                  # drift detection: native mode + manifest mode
  typeconv.go               #   TypeConverter: OpenAPI v3 (schema-aware, merge-key paths)
  #                         #     degrades to deduced converter when no schema available
  manifest.go               #   manifest-attributed diff: typed.Compare, attribution,
  #                         #     conflict/self-change/addition/degraded classification,
  #                         #     server-defaulting suppression (scrub + Removed intersection)
pkg/render/                 # table / json / yaml renderers + color gating
test/integration/           # envtest end-to-end (build tag: integration)
```

## Dependency direction

```
cmd/kubectl-fieldlord  ->  pkg/cmd
pkg/cmd                ->  pkg/kube, pkg/ownership, pkg/drift, pkg/render
pkg/drift              ->  pkg/ownership
pkg/render             ->  pkg/ownership
```

No package imports a package above it. `pkg/ownership`, `pkg/drift`, and
`pkg/render` have no dependency on `pkg/cmd` or the Kubernetes client.

## Testability: injected resolver

`cmdOptions` carries a `resolve resolveFunc` field whose production value is
`kube.Resolve`. Commands call `o.resolve(...)` rather than `kube.Resolve`
directly, so `runExplain`/`runDrift` orchestration is unit-tested by injecting a
fake resolver that returns canned objects — no live cluster required. Combined
with `--skip-version-check` (which short-circuits the capability probe), the full
command flow is exercised in unit tests; only the genuine cluster glue
(`Resolve`, discovery, kubeconfig namespace resolution) is left to the envtest
integration test.

## Output envelope (json/yaml)

```jsonc
{
  "schemaVersion": "v1",
  "command": "explain" | "drift",
  "resource": { "group": "", "version": "", "kind": "", "namespace": "", "name": "" },
  "serverVersion": "v1.34.2",     // omitted when --skip-version-check
  "supportTier": "full",          // full | best-effort | unsupported | unknown
  "findings": [ /* []OwnedPath for explain, []drift.Finding for drift */ ],
  "warnings": [ "string", ... ]
}
```

When more than one resource is requested with `-o json|yaml`, the output is a
single top-level **array** of envelopes (not concatenated documents), so it is
parseable by `jq` and `encoding/json`.

## Manifest-attributed drift (`drift -f`)

`pkg/drift/manifest.go` drives the offline desired-vs-live diff:

1. Live object and desired manifest are both decoded into
   `typed.TypedValue` using the `TypeConverter` from `typeconv.go`.
2. `live.Compare(desired)` (from `sigs.k8s.io/structured-merge-diff/v6/typed`)
   produces Added / Modified / Removed path sets with merge-key-aware paths
   (e.g. `.spec.template.spec.containers[name="app"].image`).
3. Server-managed metadata and (by default) status fields are scrubbed from
   both sides before the compare.
4. The Removed set is intersected with the fields owned by `--expect-manager`
   to suppress apiserver-defaulted fields. Added and Modified are left
   untouched (filtering Modified would hide real conflicts).
5. Each surviving path is attributed to its owner via the inverted index from
   `pkg/ownership`. Classification: **conflict** (Modified/Removed owned by
   another manager when `--expect-manager` named), **self-change** (owned by
   expected manager, including co-ownership), **addition** (in desired, not
   live), **degraded** (no schema → containing-list granularity).

`pkg/drift/typeconv.go` builds a `typed.TypeConverter` from the OpenAPI v3
schema fetched at startup. When no schema is available for the type (CRDs
without a published schema), it falls back to a deduced converter; affected
paths are flagged `granularity: "list"` in output.

**Key dependency:** `sigs.k8s.io/structured-merge-diff/v6/typed` — the `typed`
sub-package (not just `fieldpath`) is now used directly for diff computation.
This is the same library the apiserver uses for its own merge operations; the
import is already a transitive dependency of client-go. See
[ADR-0003](adr/0003-local-diff-is-schema-dependent.md) for the rationale and
known limitations.

The `drift -f` JSON envelope extends the base envelope with per-finding fields:
`change` (`modified`/`added`/`removed`), `conflict` (bool), `actualOwner`
`{manager, operation, apiVersion, time, subresource}`, and `granularity`
(`"list"` when degraded) — all `omitempty`, absent in native drift output.

## FieldsV1 decoding

`managedFields[].fieldsV1` is decoded with the upstream
`sigs.k8s.io/structured-merge-diff/v6/fieldpath` library
(`Set.FromJSON` → `Leaves().Iterate` → `Path.String()`). The encoding is a
closed, documented grammar; decoding is deterministic and requires no LLM. See
[ADR-0001](adr/0001-use-structured-merge-diff.md).
