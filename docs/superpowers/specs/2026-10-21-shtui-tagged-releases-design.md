# Shtui Tagged Release Automation Design

**Status:** Awaiting user review  
**Date:** 2026-10-21

## Goal

Publish downloadable Shtui binaries and generated release notes when a `v*` version tag is pushed to GitHub. Test and vet the project before publishing. Keep local snapshot validation available without publishing.

## Scope

- Add a root `.goreleaser.yaml` for Shtui's tagged releases.
- Add `.github/workflows/release.yml`, triggered only by pushed `v*` tags.
- Ignore GoReleaser's `/dist/` output.
- Document release validation and tag-push steps in `docs/releasing.md`, link to GitHub Releases from `README.md`.
- Keep the existing Linux-only install script and desktop integration unchanged.
- Do not create a release tag or publish anything as part of this change.

## Release artifacts

Build `./cmd/shtui` with `CGO_ENABLED=0` for:

- Linux amd64 and arm64
- macOS (darwin) amd64 and arm64
- Windows amd64 and arm64

Package non-Windows builds as `.tar.gz`, Windows builds as `.zip`, and include `README.md` and `LICENSE` in archives. Publish a SHA-256 `checksums.txt` and GoReleaser-generated release notes. Actual artifact names follow GoReleaser's naming template.

The six target binaries cross-compiled successfully with the current source. This verifies compilation, not interactive runtime behavior on every operating system; user-facing instructions must not imply that the Linux installer supports macOS or Windows.

## Workflow and publishing

- Run on GitHub-hosted Ubuntu after a `v*` tag push.
- Check out full Git history so changelog generation can compare tags.
- Set up the Go version from `go.mod` (currently Go 1.26.0).
- Run `go test ./...`, then `go vet ./...`; any failure prevents publishing.
- Run GoReleaser v2 through `goreleaser/goreleaser-action@v7` with `actions/checkout@v6` and `actions/setup-go@v6`.
- Grant only `contents: write` to the release job and use GitHub's automatic `GITHUB_TOKEN`; no personal access token or additional secret.
- Generate a Git-based changelog, sorted ascending, excluding `docs:`, `test:`, and `chore:` commits.
- Mark releases as pre-releases initially, matching the project's early release stage. The configuration must make this setting easy to change before stable releases.

## Failure and safety boundaries

- Tests, vet, build, archive creation, and checksum generation must finish before GoReleaser publishes.
- The tag is the release trigger; branch pushes do not publish.
- Snapshot validation must not require a release token and must not contact GitHub to publish.
- Keep `/dist/` out of version control; do not stage build output.
- Workflow permissions remain scoped to `contents: write` for publishing.

## Acceptance checks

- `goreleaser check` accepts the configuration.
- `goreleaser release --snapshot --clean` builds all six target binaries and creates the intended archives and checksum file without publishing.
- Inspect the generated archives to confirm the platform-specific archive formats and inclusion of README and LICENSE.
- `go test ./...` and `go vet ./...` pass locally, matching the workflow gates.
- The workflow triggers on `v*` tags only and grants only `contents: write`.
- `docs/releasing.md` explains local validation and the tag-push release flow, and README links users to releases without claiming unsupported installation paths.
