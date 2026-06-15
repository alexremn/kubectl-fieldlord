# ADR-0003: Local manifest diff is schema-dependent and degrades; predict is server-authoritative

- **Status:** Accepted
- **Date:** 2026-06-15

## Context

`drift -f` computes a desired-vs-live field diff entirely offline: it reads the
live object from the apiserver, decodes the user-supplied manifest, and produces
a set of Added / Modified / Removed field paths — then attributes each to the
manager that owns it in `managedFields`.

Three fundamental problems make a pure positional diff unacceptable.

### 1. Positional indices are wrong for associative lists

Kubernetes uses two list flavors. **Atomic lists** (e.g. `spec.hostAliases`) are
replaced wholesale; a positional index `[0]` is the only meaningful key.
**Associative lists** (e.g. `spec.template.spec.containers`, keyed by `name`)
are merged entry-by-entry; the correct path is
`.spec.template.spec.containers[name="app"]`, not `[0]`. Using positional indices
on an associative list produces paths that cannot be matched against `managedFields`
entries — the result would be systematically wrong or unattributed.

The only correct way to know which flavor applies is the OpenAPI v3 schema
(`x-kubernetes-list-type: map` / `atomic`). A positional diff without the schema
is always potentially wrong for list fields, and that error cannot be detected
locally.

### 2. Server-defaulting false-positives

The apiserver sets many fields the author never writes: `creationTimestamp`,
`uid`, `resourceVersion`, `generation`, defaulted `terminationMessagePolicy`,
and the entire `status` subresource. A naive desired-vs-live diff would report
every one of these as a Removed or Modified field, flooding the output with
noise and generating spurious conflicts.

The suppression strategy must also be correct: for Removed fields (fields in live
but not in desired), the fix is to intersect with the set of fields the
`--expect-manager` actually owns — if the apiserver owns a defaulted field, it is
not in the manager's ownership set, so it drops out naturally. For Added and
Modified fields (fields in desired that differ from live), no intersection is
applied; filtering Modified would hide real conflicts where the manager's
previously-applied value has been overwritten by another actor.

### 3. Apiserver canonicalization

The apiserver normalizes values on admission: resource quantities (`1` → `1Gi`),
enum casing (`Always` vs `always`), and similar transformations. A local diff
compares the author's literal string against the apiserver's canonical form and
sees a Modified field even when the author's intent is already satisfied. This
is structurally undetectable offline — the local tool has no access to the
admission path.

## Decision

### Typed SMD diff with OpenAPI v3 TypeConverter

Use `sigs.k8s.io/structured-merge-diff/v6/typed.Compare` (the same library the
apiserver uses for its own merge operations) with a `TypeConverter` built from
the OpenAPI v3 schema fetched from the live cluster at command startup. This
gives merge-key-aware paths
(`.spec.template.spec.containers[name="app"].image`) rather than positional
indices, matching the format used in `managedFields`.

When no schema is available for a type (CRDs that do not publish a schema, or
off-line use), fall back to a deduced converter. The deduced converter treats all
lists as atomic — no merge keys — so paths are coarser (the containing list, not
the individual entry), but they are never wrong-keyed. This degradation is
labeled explicitly in output with `granularity: "list"` and a warning.

### live.Compare(desired) direction

The diff is computed as `live.Compare(desired)` — the direction that produces
the set of changes needed to bring live state to the desired state. Removed =
fields in live not in desired; Modified = fields that differ; Added = fields in
desired not in live.

### Intersection only on Removed

To suppress server-defaulting false-positives:

- **Removed**: intersect with the set of fields owned by `--expect-manager`.
  Fields the manager never owned were defaulted by the apiserver and are not
  the author's responsibility.
- **Added / Modified**: no intersection. Filtering Modified would conceal
  conflicts where another manager has overwritten a field the author cares
  about. Filtering Added would hide new fields the author intends to apply.

Server-managed metadata (`.metadata.creationTimestamp`, `.metadata.uid`,
`.metadata.resourceVersion`, `.metadata.generation`) and the `status`
subresource are scrubbed from both sides before diffing (unless `--include-status`
is passed).

### Conflict classification and exit-2 gate

Each diff path is attributed to the manager that owns it in `managedFields`.
Classification:

| Change | Owner | Classification |
|--------|-------|----------------|
| Modified or Removed | a manager OTHER than `--expect-manager` | **conflict** |
| Modified or Removed | `--expect-manager` (including co-ownership) | **self-change** |
| Added | any (field not in live) | **addition** (informational) |
| any | no schema → coarse list path | **degraded** (informational) |

Exit code `2` (the CI gate) is emitted only when `--expect-manager` is named AND
at least one **conflict** is found. With an empty `--expect-manager`, the mode
is informational: every finding is self-change, addition, or degraded — exit `2`
is never produced. Manifest mode does not infer a primary applier; omitting
`--expect-manager` is always valid and always informational.

### Synthetic positional indices are never emitted

If the TypeConverter cannot produce a merge-key path, the path is reported at
the granularity of the containing list (degraded). A path of the form `[i]`
(positional index into an associative list) is never surfaced; it would be
meaningless against `managedFields` and misleading to the user.

## Consequences

### Positive

- Associative-list paths match the format used in `managedFields`, making
  attribution reliable for types with a published schema.
- Server-defaulting noise is suppressed without hiding real conflicts (the
  asymmetric intersection rule).
- Degradation is explicit and honest: CRD fields without a schema produce
  coarse paths labeled `granularity: "list"`, never silently wrong paths.

### Known residual: apiserver canonicalization

A value the author writes that the apiserver normalizes on admission (resource
quantity `"1"` → `"1Gi"`, enum casing, defaulted sub-fields) will appear as a
Modified field and may be classified as a conflict even though the author's
intent is met. This is a structural limitation of offline diffing — the local
tool cannot reproduce the admission path. `predict` (which performs an actual
SSA dry-run) does not have this issue.

### Schema degradation for CRDs

CRDs that do not publish an OpenAPI schema (or schemas published without
`x-kubernetes-list-map-keys` annotations) cause the TypeConverter to fall back
to the deduced converter. The diff paths are then at containing-list granularity
for list fields — the finding may cover more fields than actually changed, and
attribution will be unresolved. This is labeled in output and in a warning.

### Relationship to `predict`

`drift -f` and `predict` answer different questions and can legitimately disagree
on the same resource:

- **`predict`** issues a real SSA dry-run against the apiserver. It reports
  only the fields a `--force-conflicts` apply would actively overwrite (the
  conflict set). It is cluster-authoritative, schema-exact, and unaffected by
  canonicalization or defaulting noise. It does not show self-changes or
  additions.
- **`drift -f`** is fully offline after the initial GET. It reports ALL changed
  fields — conflicts, self-changes, additions, and degraded — with per-field
  attribution. It is richer in scope but subject to the canonicalization residual
  and schema-degradation limitations above.

Use `predict` when you need the server-authoritative clobber set before a
`--force-conflicts` apply. Use `drift -f` when you need a full attributed
desired-vs-live field inventory offline.
