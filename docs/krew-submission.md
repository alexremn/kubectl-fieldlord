# Krew Submission Runbook

This document describes how to submit `kubectl fieldlord` to the
[krew-index](https://github.com/kubernetes-sigs/krew-index) once v0.2.0 is
published. It is a manual owner action — nothing here happens automatically.

> **When to run this:** after the `v0.2.0` tag has been pushed, the GitHub
> Actions release workflow has completed, and the GitHub Release is public.

---

## How the manifest is generated (single-sourced)

There is **no hand-written `plugins/fieldlord.yaml`** in this repository.
The krew manifest is generated automatically by GoReleaser during the release
workflow, driven by the `krews:` block in `.goreleaser.yaml`:

```yaml
krews:
  - name: fieldlord
    ids: [archives]
    homepage: https://github.com/alexremn/kubectl-fieldlord
    short_description: Explain Server-Side Apply field ownership and drift
    description: |
      Parse managedFields to explain per-field ownership, attribute drift to a
      fieldManager, and predict what --force-conflicts would clobber.
    repository:
      owner: alexremn
      name: krew-index
      branch: kubectl-fieldlord-{{ .Version }}
      token: '{{ .Env.KREW_INDEX_TOKEN }}'
      pull_request:
        enabled: true
        base:
          owner: kubernetes-sigs
          name: krew-index
          branch: master
    skip_upload: auto
```

`metadata.name` is `fieldlord`; users invoke it as `kubectl fieldlord`.

---

## Prerequisites (one-time setup)

Complete these before pushing any release tag. The release workflow will fail
silently or partially if these are missing.

### 1. Fork `krew-index`

The krew PR workflow pushes a branch to *your* fork, then opens a PR from
that fork to `kubernetes-sigs/krew-index`. You need a fork at
`alexremn/krew-index`.

```bash
gh repo fork kubernetes-sigs/krew-index --org alexremn --clone=false
```

Verify: https://github.com/alexremn/krew-index must exist and be a fork of
`kubernetes-sigs/krew-index`.

### 2. Create a PAT and add the `KREW_INDEX_TOKEN` secret

The GoReleaser `krews:` block uses `KREW_INDEX_TOKEN` to push a branch to
`alexremn/krew-index` and open the cross-repo PR.

Required PAT scopes for the `alexremn/krew-index` fork:
- **Contents** — read + write (to push the branch)
- **Pull requests** — read + write (to open the PR)

Create the PAT at https://github.com/settings/tokens (fine-grained, scoped to
`alexremn/krew-index`), then add it as a repository secret:

```bash
gh secret set KREW_INDEX_TOKEN \
  --repo alexremn/kubectl-fieldlord \
  --body "<paste-token-here>"
```

### 3. Ensure the repository is public

krew-index maintainers review submissions from public repositories. The repo
must be public before the PR is opened, or reviewers cannot inspect the source.

```bash
gh repo edit alexremn/kubectl-fieldlord --visibility public
```

### 4. Developer Certificate of Origin (DCO) + Kubernetes CLA

krew-index requires:

- **DCO**: all commits in this repo must be signed off (`git commit -s`).
  The krew-index CI bot checks commit history of the PR.
- **Signed Kubernetes CLA**: the submitting GitHub account must have signed the
  Kubernetes CLA at https://cla.k8s.io. This gate appears on the PR opened by
  GoReleaser.

---

## Local pre-tag validation

Before pushing the release tag, validate the release locally. This catches
manifest generation errors and bad archive paths before they reach CI.

### Step 1 — snapshot build

```bash
go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean
```

This produces `dist/` with archives and a generated krew manifest (e.g.
`dist/fieldlord.yaml`). No tag is needed; no artifacts are published.

### Step 2 — install the snapshot locally

```bash
# Find the generated manifest and a local archive
MANIFEST=$(ls dist/fieldlord*.yaml | head -1)
ARCHIVE=$(ls dist/kubectl-fieldlord_*_linux_amd64.tar.gz | head -1)  # adjust OS/arch

kubectl krew install --manifest="$MANIFEST" --archive="$ARCHIVE"
kubectl fieldlord --version
kubectl fieldlord explain deploy/<any-deploy-in-your-cluster>
```

Confirm the binary runs and the output is correct.

### Step 3 — validate the manifest

Pin the validator to a known-good version:

```bash
go install sigs.k8s.io/krew/cmd/validate-krew-manifest@v0.4.5
validate-krew-manifest -manifest "$MANIFEST"
```

The command must exit `0` with no errors before proceeding.

---

## Releasing (tag → PR)

Once prerequisites are satisfied and local validation passes:

```bash
git tag v0.2.0
git push origin v0.2.0
```

The `.github/workflows/release.yml` workflow triggers on the `v0.2.0` tag:

1. GoReleaser builds archives for all platforms (linux/darwin/windows ×
   amd64/arm64).
2. Generates the krew manifest from `.goreleaser.yaml`'s `krews:` block.
3. Cosign signs `checksums.txt` (keyless, Sigstore bundle).
4. Syft generates SPDX + CycloneDX SBOMs per archive.
5. GoReleaser pushes branch `kubectl-fieldlord-v0.2.0` to
   `alexremn/krew-index` and opens a PR against
   `kubernetes-sigs/krew-index` (`master`).
6. The GitHub Release is created (tagged as prerelease until manually
   promoted, per `release.prerelease: auto`).

---

## krew-index PR review

After the workflow completes:

1. Verify the PR appeared at https://github.com/kubernetes-sigs/krew-index/pulls.
2. The PR description is auto-populated by GoReleaser. Add context about the
   plugin if requested by a reviewer.
3. krew-index maintainers perform a human review — typical turnaround is days
   to a few weeks. Watch the PR for review comments and respond promptly.
4. Once merged and the krew-index build propagates, users can install with:

```bash
kubectl krew install fieldlord
```

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| `KREW_INDEX_TOKEN` error in CI | Secret missing or expired PAT | Re-create PAT, update secret |
| GoReleaser fails to push branch | Fork `alexremn/krew-index` does not exist | Create the fork (Step 1) |
| `validate-krew-manifest` errors | Archive name template mismatch | Check `archives.name_template` in `.goreleaser.yaml` |
| CLA bot blocks PR | CLA not signed | Sign at https://cla.k8s.io |
| DCO bot blocks PR | Commits lack `Signed-off-by` | Rewrite commits with `git rebase --signoff` |
| PR not opened | `skip_upload: auto` + non-tag build | Only tag builds trigger the upload; snapshot skips it by design |

---

## References

- krew plugin submission guide: https://krew.sigs.k8s.io/docs/developer-guide/release/
- krew manifest format: https://krew.sigs.k8s.io/docs/developer-guide/plugin-manifest/
- validate-krew-manifest: https://github.com/kubernetes-sigs/krew/tree/master/cmd/validate-krew-manifest
- GoReleaser krew publisher docs: https://goreleaser.com/customization/krew/
