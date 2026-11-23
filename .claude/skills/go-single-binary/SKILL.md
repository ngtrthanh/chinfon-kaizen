---
name: go-single-binary
description: Package the current Go application's main package as a verified versioned Windows/Linux binary, checksums, release manifest and ZIP. Use for preparing production release artifacts, not deploying them.
---

# Kaizen adapter for shared Go binary packaging

Follow the complete local skill at `skills/go-single-binary/SKILL.md`, including its safety and verification requirements. Do not improvise or remove the clean-Git, Go tests/vet, binary format, or checksum gates.

For this repository the main package is `./cmd/kaizen`, `CGO_ENABLED=0`, target `windows/amd64`. Use `./build-release.ps1 -Version <approved-version>` or the local portable `package_go.py` script. Never package actual data, staff CSV, MFA keys, uploads or environment secrets. The older `build-win.ps1` remains a fast developer build, not a release archive.

Read the consumer's pin at `skills/go-single-binary/CONSUMER.md` before updating any shared skill. Edit the private JF canonical source first and synchronize a reviewed commit.
