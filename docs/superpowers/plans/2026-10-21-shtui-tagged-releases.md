# Shtui Tagged Releases Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish tested Shtui binaries and release notes on pushed version tags, with checksums and local snapshot validation.

**Architecture:** GoReleaser owns cross-platform builds, archives, checksums, changelog generation, and GitHub publishing. A tag-only GitHub Actions workflow runs tests and vet before invoking GoReleaser with the repository's Go version and a least-privilege release token. Release instructions document snapshot checks and pushing a version tag.

**Tech Stack:** Go 1.26.0, GoReleaser v2, GitHub Actions, YAML, shell archive inspection tools.

**Spec:** `docs/superpowers/specs/2026-10-21-shtui-tagged-releases-design.md`

## Global Constraints

- Build `./cmd/shtui` with `CGO_ENABLED=0`.
- Targets: Linux, darwin, and Windows; each for amd64 and arm64.
- Use `.tar.gz` for non-Windows archives and `.zip` for Windows archives.
- Include `README.md` and `LICENSE` in every archive.
- Generate a SHA-256 `checksums.txt` and Git-based release notes.
- Trigger publishing only on pushed `v*` tags.
- Use Go from `go.mod` (Go 1.26.0) and fetch full Git history.
- Run `go test ./...` and `go vet ./...` before publishing.
- Grant the release job only `contents: write`; use automatic `GITHUB_TOKEN`.
- Mark releases as pre-releases initially; do not create or push a tag in this implementation.
- Keep `/dist/` ignored and leave Linux-only installation integration unchanged.

## Review Focus

- A target is omitted or gets the wrong OS/architecture archive format — snapshot output must contain all six builds, with ZIP for Windows and tar.gz for Linux/macOS.
- README or LICENSE is missing from a downloadable archive — list each generated archive and verify both files are present.
- A non-tag push can publish or permissions are broader than necessary — inspect the workflow trigger and permissions; validate workflow syntax with `actionlint`.
- A failed test or vet could still reach GoReleaser — inspect that both are sequential failing steps before the release action; validate workflow syntax with `actionlint`.
- Snapshot mode accidentally publishes or uses a stable release flag — run snapshot mode without `GITHUB_TOKEN`, verify local artifacts, and inspect the config's pre-release setting.

---

### Task 1: Configure GoReleaser artifacts

**Files:**
- Create: `.goreleaser.yaml`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: `./cmd/shtui`, `go.mod`, `README.md`, and `LICENSE`.
- Produces: GoReleaser v2 configuration defining six builds, archive formats, included files, SHA-256 checksums, changelog filters, and initial pre-release behavior; `/dist/` remains untracked.

- [ ] **Step 1: Add the GoReleaser configuration** using `version: 2`, `project_name: shtui`, `main: ./cmd/shtui`, `CGO_ENABLED=0`, the six OS/architecture pairs in Global Constraints, `.tar.gz` archives with a Windows `.zip` format override, `README.md` and `LICENSE` archive files, `checksums.txt`, Git changelog sorted ascending with `docs:`, `test:`, and `chore:` excluded, and `release.prerelease: true`.
- [ ] **Step 2: Ignore build output** by adding `/dist/` to the existing `.gitignore` without changing its other entries.
- [ ] **Step 3: Validate the config** with `goreleaser check`.
  - Run: `goreleaser check`
  - Expected: configuration passes without errors.
- [ ] **Step 4: Build a non-publishing snapshot** with `goreleaser release --snapshot --clean`.
  - Run: `goreleaser release --snapshot --clean`
  - Expected: command succeeds and writes release artifacts under `dist/` without publishing.
- [ ] **Step 5: Inspect generated outputs**. Confirm all six target executables appear in archives, Windows archives are `.zip`, Linux/macOS archives are `.tar.gz`, every archive contains `README.md` and `LICENSE`, and checksums validate by running `(cd dist && sha256sum -c checksums.txt)`.

### Task 2: Add tag-triggered GitHub Actions publishing

**Files:**
- Create: `.github/workflows/release.yml`

**Interfaces:**
- Consumes: `.goreleaser.yaml` and `/dist/`-ignored output from Task 1.
- Produces: a tag-only workflow that gates release publication on tests and vet.

- [ ] **Step 1: Add the release workflow** with `push.tags: ["v*"]`, job permissions `contents: write`, `actions/checkout@v6` with `fetch-depth: 0`, `actions/setup-go@v6` with `go-version-file: go.mod` and cache enabled, sequential `go test ./...` and `go vet ./...` steps, then `goreleaser/goreleaser-action@v7` using distribution `goreleaser`, version `~> v2`, and args `release --clean`; pass `${{ secrets.GITHUB_TOKEN }}` as `GITHUB_TOKEN` only to the GoReleaser step.
- [ ] **Step 2: Validate workflow syntax**.
  - Run: `actionlint .github/workflows/release.yml`
  - Expected: no syntax or action input errors.
- [ ] **Step 3: Re-run the workflow's Go gates locally**.
  - Run: `go test ./... && go vet ./...`
  - Expected: both commands pass; GitHub Actions is configured to stop before GoReleaser if either fails.

### Task 3: Document releases for maintainers and users

**Files:**
- Create: `docs/releasing.md`
- Modify: `README.md`

**Interfaces:**
- Consumes: the exact tags, workflow, and snapshot commands from Tasks 1–2.
- Produces: maintainer steps for validation and tag pushes, plus a user-facing link to GitHub Releases.

- [ ] **Step 1: Write `docs/releasing.md`** with the local `goreleaser check` and `goreleaser release --snapshot --clean` commands, generated artifact/checksum overview, `git tag -a v0.1.0 -m "Shtui v0.1.0"` and `git push origin v0.1.0` release steps, the pre-release setting and how to switch it for stable releases, and a note that the Linux installer remains Linux-only.
- [ ] **Step 2: Add a GitHub Releases link to the README install area** without claiming that the Linux installer supports macOS or Windows.
- [ ] **Step 3: Review documentation against configuration**. Confirm filenames, formats, target matrix, tag pattern, prerelease status, and commands match `.goreleaser.yaml` and `.github/workflows/release.yml`.
