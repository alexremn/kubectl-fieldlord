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
pkg/drift/                  # native drift detection + primary-applier inference
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

## FieldsV1 decoding

`managedFields[].fieldsV1` is decoded with the upstream
`sigs.k8s.io/structured-merge-diff/v6/fieldpath` library
(`Set.FromJSON` → `Leaves().Iterate` → `Path.String()`). The encoding is a
closed, documented grammar; decoding is deterministic and requires no LLM. See
[ADR-0001](adr/0001-use-structured-merge-diff.md).
