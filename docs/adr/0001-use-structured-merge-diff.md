# ADR-0001: Decode FieldsV1 with structured-merge-diff, not a hand-rolled parser

- **Status:** Accepted
- **Date:** 2026-06-14

## Context

The core job of kubectl-fieldlord is turning a Kubernetes object's
`managedFields[].fieldsV1` blob into human-readable field paths. `FieldsV1` is a
trie-shaped JSON encoding with four element prefixes (`f:` struct field / map
key, `v:` set member, `i:` atomic-list index, `k:` associative-list key) plus a
`.` self-marker. Associative-list keys can be compound and contain arbitrary
UTF-8.

Two options were considered:

1. Hand-roll a `FieldsV1` parser.
2. Reuse the upstream `sigs.k8s.io/structured-merge-diff/v6/fieldpath` package
   (`Set.FromJSON`, `Set.Leaves`, `Set.Iterate`, `Path.String`).

## Decision

Use the upstream `structured-merge-diff` library. The original tech spec flagged
"mapping `FieldsV1` set-paths back to human field paths" as the project's main
adoption risk; in fact the encoding is a closed, documented grammar that the
library already decodes exactly and renders via `Path.String()`. This is the
same code the apiserver uses, so it is correct by construction and maintained
upstream.

The library is pinned to the same major version as the vendored apimachinery
(`v6`), matching `k8s.io/* v0.36.2`.

## Consequences

- **Positive:** decoding is deterministic, exact, and requires no LLM. We avoid
  an entire class of parser bugs (compound keys, UTF-8 values, unknown future
  prefixes — which the library skips for forward-compatibility).
- **Positive:** the real risk shifts to *semantic* interpretation (atomic lists,
  version skew, ownership transfer), which is where we focus instead.
- **Neutral:** we take a direct dependency on `structured-merge-diff`, already a
  transitive dependency of client-go.
- **Constraint:** the `v6` import path must track the vendored apimachinery
  major version (the wire format is stable across versions; the Go import path is
  not).
