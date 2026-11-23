---
name: go-single-binary
description: Build and verify a self-contained Go main-package executable for Windows or Linux, with a release ZIP, checksums, metadata and clean-source gates. Use when packaging Go services, CLI tools, Windows workers or embedded-web Go apps for reuse across repositories.
---

# Go single-binary release packaging

Use this skill when a Go project's source contains a runnable `main` package and the requested deliverable is a versioned executable rather than a source archive, container, installer or live deployment.

## Procedure

1. Inspect `go.mod`, the main package, `//go:embed` files and runtime data/config requirements. Do **not** confuse bundled templates/migrations with runtime SQLite databases, credentials, uploaded files, signing keys or operating-system services.
2. Require a clean, committed Git tree (ignore `dist/` in the consumer `.gitignore`). Confirm the intended package, version, target OS/architecture, and CGO requirement. Default to `windows/amd64`, `CGO_ENABLED=0`. Apps using CGO require native builds or a reviewed C cross toolchain; this skill intentionally refuses cross-target CGO.
3. Run Go module verification, all Go tests and vet. Do not reduce checks merely to meet a deadline. For security-sensitive projects, run the repository's own race, vulnerability, integration and UAT gates **in addition** to this script.
4. Build with `go build -mod=readonly -trimpath -buildvcs=true -ldflags='-s -w'`. Verify Windows PE / Linux ELF and CPU architecture. Do not inject linker variables that the app has not defined.
5. Package exactly one executable plus `manifest.json` and `SHA256SUMS`; verify the release directory and ZIP. The ZIP records its own SHA-256 on stdout. Do not describe a package as byte-for-byte reproducible across toolchains, machines or build timestamps.
6. Record the source commit and checksum alongside the deployment approval. A successful package is **not** a deployment, signed executable, independent security approval or proof that dynamic CGO dependencies are absent.

## Run

Requires Python 3.10+ (stdlib only), Go matching `go.mod`, and Git. Run from the consumer repository; use the local copied skill path:

```powershell
python skills/go-single-binary/scripts/package_go.py build --project-root . --package ./cmd/app --name myapp --version 0.1.0 --target windows/amd64 --cgo 0
python skills/go-single-binary/scripts/package_go.py verify dist/myapp_0.1.0_windows_amd64.zip
```

Linux target (pure Go):

```bash
python3 skills/go-single-binary/scripts/package_go.py build --project-root . --package ./cmd/app --name myapp --version 0.1.0 --target linux/amd64 --cgo 0
```

A Go project with CGO must be built on the target-native machine (or in a separately reviewed cross-compilation environment):

```powershell
python skills/go-single-binary/scripts/package_go.py build --project-root . --package . --name myapp --version 0.1.0 --target windows/amd64 --cgo 1
```

Use `--allow-dirty` **only** for an unshipped local diagnostic; its manifest explicitly reports `git_dirty=true`. A release path that already exists is not overwritten.

## Output

- `dist/<name>_<version>_<os>_<arch>/<name>[.exe]`
- `dist/<name>_<version>_<os>_<arch>/manifest.json`
- `dist/<name>_<version>_<os>_<arch>/SHA256SUMS`
- `dist/<name>_<version>_<os>_<arch>.zip`

The source's existing `build-win.ps1`, development runner, runtime environment, deployment configuration, SQLite data, uploads, MFA keys and enterprise acceptance procedures remain project-owned. Do not commit or ship these in the portable skill.

See `references/release-checklist.md` for acceptance checks and `templates/github-actions.yml` as a consumer workflow **template**, not as an active CI configuration.

## Ownership and distribution

Canonical private source: `ngtrthanh/jf/harness/skills/go-single-binary/`. Any consumer copy is a pinned snapshot; edit the canonical skill first, run its tests, then update consumers through reviewed PRs. A consumer should reference the exact JF commit and verify file parity before removing an older local version. Never fetch and execute a remote branch's latest script during production packaging.
