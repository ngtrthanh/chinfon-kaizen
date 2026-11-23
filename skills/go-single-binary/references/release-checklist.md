# Release acceptance and extraction record

## Verified source extraction

The original `ngtrthanh/chinfon-kaizen/build-win.ps1` runs:

```powershell
go build -trimpath -ldflags "-s -w" -o bin/kaizen.exe ./cmd/kaizen
```

Kaizen's `internal/app/app.go` embeds web templates and migrations using `//go:embed` and is built with a pure-Go SQLite driver. Reusable pattern: Go source + embed + `go build`. Project-specific: Kaizen routes, employee/payout data, Go module versions, port, SQLite paths and Windows runtime scripts. The new skill adds cleanliness, tests/vet, checksums, manifest, format checks and ZIP integrity; these are **new controls**, not claimed features of the original build script.

## Before packaging

- [ ] Explicitly select the Go `main` package. Do not assume every Go repository uses `./cmd/app`.
- [ ] Confirm `go.mod` and Go toolchain on build host; pin dependencies and review `go.sum`.
- [ ] Confirm any `go:embed` patterns include the intended UI/assets/migrations; test startup from a different working directory.
- [ ] Decide whether `CGO_ENABLED=0` works. If CGO is required, build natively with a C compiler and inspect external dynamic libraries independently.
- [ ] Test DB migration on a **copy**; the release ZIP does not include live data or backups.
- [ ] No secrets, credentials, customer data, PII, `.env`, personal keys or runtime upload directories in archive.
- [ ] Clean Git commit, approved version and changelog, CI of owning repository green.

## After packaging

- [ ] `package_go.py verify RELEASE.zip` returns zero and validates binary format, architecture and SHA-256.
- [ ] `manifest.json` includes Git commit, dirty flag, target, CGO flag, actual Go toolchain, executed checks, binary length and SHA-256.
- [ ] Archive holds exactly executable, manifest, checksums; zip SHA-256 recorded separately.
- [ ] Windows native smoke test on clean host, no developer Go installation required for running a pure-Go binary.
- [ ] Service installation, account permissions, firewall/TLS, Windows Defender/signing, monitoring and restart tested **by the consumer project**.
- [ ] Runtime data path, DB migration, uploads, MFA/decryption keys, backup and restore validated separately.
- [ ] Release evidence, business owner and rollback commit recorded. No automatic deployment or acceptance implied.

## Verification semantics

`go test ./...` and `go vet ./...` demonstrate source checks. `go mod verify` verifies downloaded module archives against local checksums; it is not an advisory/vulnerability scan. File-format and SHA-256 checks validate package contents, not runtime dependencies. The archive's timestamps mean this is not a deterministic/reproducible-build attestation. For CGO, "one .exe" does **not** imply absence of external native libraries.
