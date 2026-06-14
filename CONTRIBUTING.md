# Contributing to kubectl-fieldlord

Thank you for your interest in contributing. This document covers the mechanics of getting a change from idea to merged pull request.

---

## Developer Certificate of Origin

All commits must be signed off under the [Developer Certificate of Origin](https://developercertificate.org/). Add the sign-off to every commit:

```bash
git commit -s -m "feat: short description of change"
```

Pull requests that contain unsigned commits will not be merged.

---

## Getting started

### Prerequisites

- Go 1.26 or later (required by the pinned k8s.io v0.36 dependencies)
- [golangci-lint](https://golangci-lint.run/) v2 (`brew install golangci-lint` or see upstream install docs)
- `setup-envtest` for the integration test lane (see below)

### Build

```bash
make build          # produces bin/kubectl-fieldlord
```

### Run the test suite

```bash
make test           # go test -race ./...
```

All unit tests must pass with the race detector enabled before submitting a PR.

### Run the linter

```bash
make lint           # golangci-lint run
```

The project uses `.golangci.yml` at the repo root. Fix all lint findings before submitting.

### Coverage

```bash
make cover          # go test -race -coverprofile=coverage.txt ./...
```

The project targets **80% statement coverage** across all packages. PRs that significantly reduce coverage without a clear justification will be asked to add tests.

---

## Golden-file workflow

The `pkg/ownership` package uses golden files to lock in the decoded output of known `managedFields` fixtures. To regenerate golden files after a deliberate change to the decoder:

```bash
go test ./pkg/ownership -run Golden -update
```

Review the diff in `internal/testdata/fieldsv1/` carefully before committing — golden-file changes are a signal that the output contract has changed.

---

## Integration tests (envtest)

The `test/integration/` lane runs against a real Kubernetes API server managed by `controller-runtime/tools/setup-envtest`. To run it:

1. Install `setup-envtest`:

   ```bash
   go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
   ```

2. Download the envtest binaries for the target version:

   ```bash
   setup-envtest use 1.34.1
   ```

   Note the assets path printed by the command (it looks like `~/.local/share/kubebuilder-envtest/k8s/1.34.1-<os>-<arch>`).

3. Run the integration tests:

   ```bash
   KUBEBUILDER_ASSETS=$(setup-envtest use 1.34.1 -p path) \
     go test -tags integration ./test/integration/
   ```

The integration tests require the `integration` build tag and are not run by `make test`. Run them before submitting changes that touch `pkg/kube`, `pkg/ownership`, or `pkg/cmd`.

---

## Commit style

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>: <short description>

<optional body>
```

Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `perf`, `ci`.

Examples:

```
feat: add --expect-manager flag to drift command
fix: handle empty FieldsV1 without panic
test: golden fixture for multi-owner deployment
docs: add envtest setup instructions to CONTRIBUTING
```

Keep the subject line under 72 characters. Use the body to explain *why*, not *what*.

---

## Pull request checklist

- [ ] `make test` passes (race detector on)
- [ ] `make lint` passes
- [ ] Coverage has not dropped significantly (run `make cover`)
- [ ] Integration tests pass for changes touching the Kubernetes client layer
- [ ] Golden files updated and reviewed (if output contract changed)
- [ ] All commits are signed off (`git commit -s`)
- [ ] PR description explains the motivation and links any relevant issues

---

## Code of conduct

Participation is governed by the [Code of Conduct](CODE_OF_CONDUCT.md).
