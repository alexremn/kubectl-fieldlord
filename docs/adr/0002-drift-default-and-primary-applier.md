# ADR-0002: Drift default is managedFields-native; primary-applier inference

- **Status:** Accepted
- **Date:** 2026-06-14

## Context

`drift` answers "which fields are owned by a manager I did not expect?" To do
that it needs a baseline: the manager that *should* own the object's fields.

Two baseline models exist:

1. **managedFields-native** — read the live object's `managedFields` and flag
   fields owned by a manager other than the expected one. No external input.
2. **manifest-vs-live** — the user supplies a desired manifest (`-f`); diff it
   against live state and attribute each difference.

v0.1 ships only the native model (manifest mode is scheduled for v0.2). The
native model still needs to know *which* manager is the expected one when the
user does not say.

## Decision

**Default to managedFields-native drift.** The expected manager is set by
`--expect-manager`; when it is absent, infer the **primary applier**:

> the `Operation == Apply`, main-resource (`Subresource == ""`) manager that owns
> the most leaf fields.

Tie-break, in order:

1. most owned leaf fields,
2. then most-recent `Time` (RFC3339 strings compare chronologically),
3. then lexicographically smallest manager name (deterministic final tiebreak).

If **no Apply-operation manager exists**, do not guess — return an error
(`cannot infer primary applier; pass --expect-manager`), which maps to exit code
1.

**Subresource scoping:** fields owned only via a subresource entry
(`status`, `scale`) are *not* compared against the main-resource applier, so a
controller that legitimately updates `/status` is never reported as drift from
the main applier.

## Consequences

- **Positive:** zero-config drift detection for the common case (one dominant
  GitOps applier plus controllers/autoscalers fighting over a few fields like
  `spec.replicas`).
- **Positive:** the inference is fully deterministic and independent of map
  iteration order, so output is stable and testable.
- **Positive:** refusing to guess when there is no Apply manager avoids
  confidently-wrong output on objects that predate Server-Side Apply.
- **Trade-off:** the primary-applier heuristic ("most leaves") can be wrong on
  unusual ownership shapes; `--expect-manager` is the explicit escape hatch.
- **Limitation:** attribution against pre-SSA objects or generic managers
  (`before-first-apply`, `kubectl-client-side-apply`) can be partial; such
  results are surfaced honestly rather than hidden.
