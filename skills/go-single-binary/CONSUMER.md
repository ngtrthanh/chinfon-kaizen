# Kaizen consumer adapter for go-single-binary

Canonical source: `ngtrthanh/jf/harness/skills/go-single-binary/` (private).
Pinned JF source commit: `063ed99c4d8c4dbd9ff4a90bc0f31595b0fc08c7`.
Local snapshot: `skills/go-single-binary/` in this repository.
Origin of reusable build recipe: `build-win.ps1` Git blob `d0d9305ca4c5c28910ea36db9a98356644df4fe1`.

This directory is a verbatim consumer copy of the five canonical files. When updating the shared skill, update JF first, run tests, then copy the reviewed exact versions here and verify Git blob SHA parity. The JF commit above is a fixed provenance snapshot, not a run-time download target.

## Project-specific invocation

```powershell
.\build-release.ps1 -Version 0.3.0
python skills/go-single-binary/scripts/package_go.py verify dist/kaizen_0.3.0_windows_amd64.zip
```

Requirements: Python >=3.10, matching Go toolchain, clean committed Git tree. Release artifacts are in ignored `dist/`. This does **not** install or deploy Kaizen. The legacy `build-win.ps1` continues producing `bin/kaizen.exe` for local use.

CI runs the shared skill fixture tests, not a full Windows runtime UAT. Kaizen's own security, migrations, approval, backup and production gates remain separate and mandatory. Never package `data/`, uploads, staff CSV, `mfa.key`, passwords or `.env` into the binary ZIP.
