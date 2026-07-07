# Contributing

Thank you for helping improve xqua-go. This document covers day-to-day development and how we cut releases.

## Development

### Prerequisites

- Go 1.26.4 or newer
- Docker (optional, for integration tests and the `showcase` example)

### Common tasks

```bash
make help          # list available targets
make check         # format check, go vet, and unit tests (pre-commit gate)
make test-all      # unit + integration tests (starts dev Postgres)
make fmt           # format Go source
make tidy          # sync go.mod / go.sum
```

Run a single example:

```bash
go run ./examples/hello
make dev-up && go run ./examples/showcase
```

Before opening a pull request, run `make check`. If your change touches Postgres-backed code, also run `make test-integration`.

### Changelog entries

User-facing changes should be noted under `## [Unreleased]` in [`CHANGELOG.md`](CHANGELOG.md). Use [Keep a Changelog](https://keepachangelog.com/) categories where they fit:

- **Added** — new features
- **Changed** — behavior changes in existing functionality
- **Deprecated** — features marked for removal
- **Removed** — removed features
- **Fixed** — bug fixes
- **Security** — vulnerability fixes

## Releases

We follow [Semantic Versioning](https://semver.org/). Go module tags use a `v` prefix (`v0.1.0`, `v0.2.0`, …). Consumers pin with:

```bash
go get github.com/InTacht/xqua-go@v0.2.0
```

### Pre-release checklist

1. All intended changes are merged on `main`.
2. `make check` passes.
3. Integration-sensitive changes have been exercised with `make test-integration`.
4. `CHANGELOG.md` has a dated section for the new version (not just `[Unreleased]`).
5. The compare link at the bottom of `CHANGELOG.md` points at the previous tag.

### Cutting a release

1. **Prepare the changelog.** Move items from `[Unreleased]` into a new section:

   ```markdown
   ## [Unreleased]

   ## [0.2.0] - 2026-08-01

   ### Added
   - ...
   ```

   Update the footer links:

   ```markdown
   [Unreleased]: https://github.com/InTacht/xqua-go/compare/v0.2.0...HEAD
   [0.2.0]: https://github.com/InTacht/xqua-go/releases/tag/v0.2.0
   ```

2. **Commit on `main`.**

   ```bash
   git add CHANGELOG.md
   git commit -m "Prepare v0.2.0 release notes."
   git push origin main
   ```

3. **Run the release target.** From a clean working tree:

   ```bash
   make release VERSION=v0.2.0
   ```

   This runs, in order:

   | Step | Target | What it does |
   |------|--------|--------------|
   | 1 | `release-check` | `make check`, validates `VERSION`, requires a clean tree, ensures the tag does not already exist, and confirms a matching `CHANGELOG.md` section |
   | 2 | `release-tag` | Creates an annotated git tag; notes are extracted from the changelog section |
   | 3 | `release-push` | Pushes the tag to `origin` |
   | 4 | `release-github` | Creates a GitHub release with `gh` when installed |

4. **Verify on GitHub.** Confirm the tag, release notes, and compare link look correct.

### Release targets

Run individual steps when you need finer control:

```bash
make release-check VERSION=v0.2.0   # validate only
make release-tag   VERSION=v0.2.0   # tag locally
make release-push  VERSION=v0.2.0   # push tag
make release-github VERSION=v0.2.0  # publish GitHub release
```

Override release notes with a file instead of changelog extraction:

```bash
make release VERSION=v0.2.0 NOTES=notes.txt
```

### Manual fallback

If `gh` is unavailable or push auth fails locally:

```bash
make release-check VERSION=v0.2.0
make release-tag   VERSION=v0.2.0
git push origin v0.2.0
gh release create v0.2.0 --title v0.2.0 --notes-file notes.txt
```

`release-github` skips quietly when `gh` is not installed; the tag push is still enough for `go get` to resolve the module.

### First release note (v0.1.0)

`v0.1.0` was tagged before this workflow landed. Its notes live in `CHANGELOG.md`; push the tag and create the GitHub release manually if they are not on the remote yet:

```bash
git push origin v0.1.0
gh release create v0.1.0 --title v0.1.0 --notes-file <(awk -v ver="0.1.0" \
  '$0 ~ "^## \\[" ver "\\] " { found=1; next } found && /^## \[/ { exit } found { print }' CHANGELOG.md)
```
