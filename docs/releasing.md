# Releasing Shtui

Pushing a version tag that starts with `v` (for example, `v0.1.0`) triggers the GitHub Actions release workflow. The workflow runs `go test ./...` and `go vet ./...`; GoReleaser publishes only if both pass.

## Validate locally

Use the Go version declared in `go.mod` for Shtui. Install the GoReleaser v2 binary separately using its [official installation instructions](https://goreleaser.com/getting-started/install/oss/); prefer a prebuilt binary or package if your Go toolchain is older than GoReleaser's own requirement. From the repository root, run:

```sh
goreleaser check
goreleaser release --snapshot --clean
(cd dist && sha256sum -c checksums.txt)
```

Snapshot mode builds locally and does not publish a GitHub release. Inspect the generated archives in `dist/` before tagging: Linux and macOS use `.tar.gz`, Windows uses `.zip`, and each archive contains the executable, `README.md`, and `LICENSE`.

## Publish a release

Create and push an annotated version tag from the commit to release:

```sh
git tag -a v0.1.0 -m "Shtui v0.1.0"
git push origin v0.1.0
```

The `v*` tag push starts the workflow. It checks out full Git history for release notes and uses GitHub's automatic `GITHUB_TOKEN` with `contents: write`; no personal access token is needed. GoReleaser creates the release notes, six platform archives (Linux, macOS, and Windows on amd64 and arm64), and `checksums.txt`.

Releases are marked as pre-releases initially. Before publishing a stable release, change `release.prerelease` in `.goreleaser.yaml` to `false` and commit that change before creating the stable version tag.

## Platform notes

The six release targets cross-compile successfully, but interactive runtime behavior has not been verified on every target operating system. The repository's installer and desktop-file integration are Linux-only; macOS and Windows builds are distributed as archives, not through the Linux installer.
