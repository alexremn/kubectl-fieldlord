# Distribution Runbook — krew & Homebrew

`kubectl fieldlord` ships through two channels, both wired into the release
pipeline. Neither needs a hand-maintained manifest.

| Channel | Source of truth | Published by | Auth needed |
|---------|-----------------|--------------|-------------|
| **krew** | `.krew.yaml` | `.github/workflows/krew.yml` (`rajatjindal/krew-release-bot`) on release publish | none |
| **Homebrew** | `homebrew_casks:` in `.goreleaser.yaml` | GoReleaser during `release.yml` | `HOMEBREW_TAP_GITHUB_TOKEN` secret |

`metadata.name` is `fieldlord`; users invoke it as `kubectl fieldlord`. The binary
is `kubectl-fieldlord`.

---

## How krew publishing works

There is **no fork and no `KREW_INDEX_TOKEN`**. On a *published* GitHub Release,
`krew.yml` runs `rajatjindal/krew-release-bot`, which:

1. reads `.krew.yaml`,
2. fills `{{ .TagName }}` and each archive's `sha256` from the release assets,
3. opens (or updates) a PR to `kubernetes-sigs/krew-index` via the bot's own
   hosted backend — no token or fork on our side.

The archive filenames in `.krew.yaml` are **version-less**
(`kubectl-fieldlord_<os>_<arch>.tar.gz`, windows `.zip`); the tag lives in the
release path. This must stay in sync with `archives.name_template` in
`.goreleaser.yaml`.

## How Homebrew publishing works

GoReleaser's `homebrew_casks:` block generates `Casks/kubectl-fieldlord.rb` and
pushes it to `alexremn/homebrew-tap` on every tagged release, using the
`HOMEBREW_TAP_GITHUB_TOKEN` secret. Users then:

```sh
brew install alexremn/tap/kubectl-fieldlord
```

The cask installs the `kubectl-fieldlord` binary on `PATH` and strips the macOS
quarantine xattr so it runs without a Gatekeeper prompt.

---

## One-time prerequisites

1. **Repository public** — krew-index review and the bot both need a public repo:
   `gh repo edit alexremn/kubectl-fieldlord --visibility public`.
2. **`HOMEBREW_TAP_GITHUB_TOKEN` secret** — a PAT with **Contents: read+write** on
   `alexremn/homebrew-tap`:
   `gh secret set HOMEBREW_TAP_GITHUB_TOKEN --repo alexremn/kubectl-fieldlord --body "<token>"`.
   (The `alexremn/homebrew-tap` repo already exists.)
3. **Kubernetes CLA** — the krew-index PR runs the CNCF EasyCLA check. Sign at
   <https://cla.k8s.io> if prompted on the PR.

---

## Local validation (before tagging)

```sh
make third-party-licenses    # the release archives include this file
go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean --skip=publish,sign,sbom
ls dist/*.tar.gz dist/*.zip  # names must match the URLs in .krew.yaml
```

Confirm the generated cask renders at `dist/homebrew/Casks/kubectl-fieldlord.rb`.
Optionally lint the krew manifest's static shape (note: `validate-krew-manifest`
does not evaluate the `krew-release-bot` template directives, so validate a
rendered copy or rely on the bot's PR CI):

```sh
go install sigs.k8s.io/krew/cmd/validate-krew-manifest@v0.4.5
```

---

## Release flow (tag → publish → PRs)

```sh
git tag -a v0.1.0 -m "kubectl-fieldlord v0.1.0"
git push origin v0.1.0
```

1. `release.yml` runs the reusable `ci` workflow (build/test/lint) as a gate.
2. GoReleaser builds all platforms, signs `checksums.txt` (cosign keyless),
   generates SBOMs, pushes the Homebrew cask to `alexremn/homebrew-tap`, and
   creates a **draft** GitHub Release (`release.draft: true`).
3. SLSA build provenance is attested over `checksums.txt`.
4. **You review the draft release notes and click *Publish*.**
5. Publishing fires `krew.yml` → `krew-release-bot` opens the krew-index PR.
6. A krew maintainer reviews; respond to EasyCLA / review comments.
7. After merge: `kubectl krew update && kubectl krew install fieldlord`.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| Homebrew cask not pushed | `HOMEBREW_TAP_GITHUB_TOKEN` missing/expired | Re-create PAT, update the secret |
| krew PR never opens | Release left as draft | Publish the GitHub Release (step 4) |
| `validate-krew-manifest` errors on `.krew.yaml` | It can't read `krew-release-bot` template directives | Validate a rendered manifest, or trust the bot's PR CI |
| krew archive 404 | `.krew.yaml` filename ≠ `archives.name_template` | Keep both version-less and in sync |
| CLA bot blocks the krew PR | Kubernetes CLA unsigned | Sign at <https://cla.k8s.io> |

---

## References

- krew developer guide: <https://krew.sigs.k8s.io/docs/developer-guide/release/>
- krew-release-bot: <https://github.com/rajatjindal/krew-release-bot>
- GoReleaser Homebrew casks: <https://goreleaser.com/customization/homebrew_casks/>
