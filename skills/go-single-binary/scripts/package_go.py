#!/usr/bin/env python3
"""Release a verified Go CLI/web-service main package as one versioned binary.

Python >=3.10; only standard library, Go and Git. Does NOT deploy/sign/bundle
runtime secrets, SQLite DBs, system service definitions or external DLLs.
"""
from __future__ import annotations

import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import struct
import subprocess
import sys
import tempfile
import zipfile

VERSION = "0.1.0"
TARGETS = {"windows/amd64", "windows/arm64", "linux/amd64", "linux/arm64"}
TOKEN = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$")


def sha(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def run(args: list[str], cwd: Path, env: dict[str, str] | None = None, capture: bool = False) -> str:
    print("+", " ".join(args), flush=True)
    proc = subprocess.run(args, cwd=cwd, env=env, text=True, capture_output=capture, check=False)
    if proc.returncode:
        if capture:
            print((proc.stdout or "") + (proc.stderr or ""), file=sys.stderr)
        raise ValueError(f"command failed (exit {proc.returncode}): {args[0]}")
    return (proc.stdout or "").strip() if capture else ""


def git_state(root: Path) -> tuple[str | None, bool]:
    try:
        commit = subprocess.run(["git", "rev-parse", "HEAD"], cwd=root, capture_output=True, text=True)
        if commit.returncode:
            return None, True
        status = subprocess.run(["git", "status", "--porcelain", "--untracked-files=all"], cwd=root, capture_output=True, text=True)
        if status.returncode:
            raise ValueError("cannot check Git working tree")
        return commit.stdout.strip(), bool(status.stdout.strip())
    except FileNotFoundError:
        return None, True


def binary_format(data: bytes, target: str) -> None:
    os_name, arch = target.split("/")
    if os_name == "windows":
        if len(data) < 0x44 or data[:2] != b"MZ":
            raise ValueError("not a Windows PE binary")
        off = struct.unpack_from("<I", data, 0x3c)[0]
        if off + 6 > len(data) or data[off:off + 4] != b"PE\x00\x00":
            raise ValueError("invalid Windows PE header")
        machine = struct.unpack_from("<H", data, off + 4)[0]
        if machine != {"amd64": 0x8664, "arm64": 0xAA64}[arch]:
            raise ValueError("PE architecture does not match target")
    elif os_name == "linux":
        if len(data) < 20 or data[:4] != b"\x7fELF" or data[5] != 1:
            raise ValueError("not a little-endian Linux ELF binary")
        machine = struct.unpack_from("<H", data, 18)[0]
        if machine != {"amd64": 62, "arm64": 183}[arch]:
            raise ValueError("ELF architecture does not match target")
    else:
        raise ValueError("unsupported target")


def verify(read_file, filenames: list[str]) -> dict:
    if "manifest.json" not in filenames or "SHA256SUMS" not in filenames:
        raise ValueError("manifest.json or SHA256SUMS missing")
    manifest = json.loads(read_file("manifest.json").decode("utf-8"))
    name = manifest.get("binary", {}).get("filename", "")
    if not name or "/" in name or "\\" in name or name not in filenames:
        raise ValueError("manifest has invalid/missing binary filename")
    payload = read_file(name)
    binary_format(payload, manifest["target"])
    digest = sha(payload)
    if digest != manifest["binary"]["sha256"] or len(payload) != manifest["binary"]["bytes"]:
        raise ValueError("binary hash/size mismatch")
    expected = f"{digest}  {name}\n{sha(read_file('manifest.json'))}  manifest.json\n"
    if read_file("SHA256SUMS").decode("utf-8") != expected:
        raise ValueError("SHA256SUMS mismatch")
    return manifest


def verify_release(path: Path) -> dict:
    if path.is_dir():
        files = [p.name for p in path.iterdir() if p.is_file()]
        if set(files) != {"manifest.json", "SHA256SUMS", next((n for n in files if n not in {"manifest.json", "SHA256SUMS"}), "")}:
            raise ValueError("unexpected or missing release files")
        return verify(lambda name: (path / name).read_bytes(), files)
    with zipfile.ZipFile(path) as archive:
        members = archive.namelist()
        if len(members) != 3 or len(set(members)) != 3:
            raise ValueError("archive must contain exactly three files")
        if any("/" not in n or n.startswith("/") or ".." in n.split("/") or "\\" in n for n in members):
            raise ValueError("unsafe zip paths")
        prefix = members[0].split("/", 1)[0] + "/"
        if any(not n.startswith(prefix) or "/" in n[len(prefix):] for n in members):
            raise ValueError("inconsistent release archive root")
        return verify(lambda name: archive.read(prefix + name), [n[len(prefix):] for n in members])


def build(args) -> dict:
    for label, token in (("name", args.name), ("version", args.version)):
        if not TOKEN.fullmatch(token):
            raise ValueError(f"invalid {label}: use letters, digits, dot, underscore and dash")
    root = args.project_root.resolve()
    if not (root / "go.mod").is_file():
        raise ValueError(f"go.mod not found in project root: {root}")
    if args.target not in TARGETS:
        raise ValueError("unsupported GOOS/GOARCH")
    commit, dirty = git_state(root)
    if dirty and not args.allow_dirty:
        raise ValueError("dirty/unversioned Git tree; commit files first (or use --allow-dirty for local trials)")
    host = run(["go", "env", "GOOS", "GOARCH"], root, capture=True).splitlines()
    if args.cgo == 1 and args.target != "/".join(host):
        raise ValueError("CGO cross build needs a configured cross C toolchain; package natively for this skill")
    env = dict(os.environ, GOOS=args.target.split("/")[0], GOARCH=args.target.split("/")[1], CGO_ENABLED=str(args.cgo))
    checks = []
    run(["go", "mod", "verify"], root, env); checks.append("go mod verify")
    run(["go", "test", "-mod=readonly", "./..."], root, dict(os.environ, CGO_ENABLED=str(args.cgo))); checks.append("go test ./...")
    run(["go", "vet", "-mod=readonly", "./..."], root, dict(os.environ, CGO_ENABLED=str(args.cgo))); checks.append("go vet ./...")
    main_pkg = run(["go", "list", "-mod=readonly", "-f", "{{.Name}}", args.package], root, env, capture=True)
    if main_pkg != "main":
        raise ValueError("package must be a Go main package")
    toolchain = run(["go", "version"], root, capture=True)
    stem = f"{args.name}_{args.version}_{args.target.replace('/', '_')}"
    out = (root / args.out_dir).resolve() if not args.out_dir.is_absolute() else args.out_dir.resolve()
    dest = out / stem
    archive_path = out / (stem + ".zip")
    if dest.exists() or archive_path.exists():
        raise ValueError("output release already exists; never overwrite an existing release")
    out.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="go-package-", dir=out) as tmp:
        tmp_dir = Path(tmp)
        exe_name = args.name + (".exe" if args.target.startswith("windows/") else "")
        binary_path = tmp_dir / exe_name
        build_cmd = ["go", "build", "-mod=readonly", "-trimpath", "-buildvcs=true" if commit else "-buildvcs=false", "-ldflags=-s -w", "-o", str(binary_path), args.package]
        run(build_cmd, root, env)
        binary = binary_path.read_bytes()
        binary_format(binary, args.target)
        build_info = run(["go", "version", "-m", str(binary_path)], root, capture=True)
        os_name, arch = args.target.split("/")
        if (f"build\tGOOS={os_name}" not in build_info
                or f"build\tGOARCH={arch}" not in build_info):
            raise ValueError("Go build metadata does not match target")
        manifest = {
            "schema_version": 1, "skill_version": VERSION,
            "name": args.name, "version": args.version, "package": args.package,
            "target": args.target, "cgo_enabled": bool(args.cgo), "go_toolchain": toolchain,
            "git_commit": commit, "git_dirty": dirty,
            "built_utc": datetime.now(timezone.utc).isoformat(timespec="seconds"),
            "checks": checks,
            "binary": {"filename": exe_name, "sha256": sha(binary), "bytes": len(binary)},
            "notes": "Single Go executable; runtime data/configuration and any external CGO libraries are not included.",
        }
        manifest_bytes = (json.dumps(manifest, indent=2, ensure_ascii=False) + "\n").encode("utf-8")
        (tmp_dir / "manifest.json").write_bytes(manifest_bytes)
        sums = f"{sha(binary)}  {exe_name}\n{sha(manifest_bytes)}  manifest.json\n"
        (tmp_dir / "SHA256SUMS").write_bytes(sums.encode("utf-8"))
        verify_release(tmp_dir)
        dest.mkdir()
        for filename in (exe_name, "manifest.json", "SHA256SUMS"):
            shutil.copy2(tmp_dir / filename, dest / filename)
    try:
        with zipfile.ZipFile(archive_path, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=6) as z:
            for name in (exe_name, "manifest.json", "SHA256SUMS"):
                z.write(dest / name, f"{stem}/{name}")
        verify_release(archive_path)
    except Exception:
        archive_path.unlink(missing_ok=True)
        raise
    print(f"RELEASE: {archive_path}\nSHA256 ZIP: {sha(archive_path.read_bytes())}", flush=True)
    return manifest


def main(argv=None) -> int:
    p = argparse.ArgumentParser(description=__doc__)
    sub = p.add_subparsers(dest="command", required=True)
    b = sub.add_parser("build", help="go test + vet + package + verify")
    b.add_argument("--project-root", type=Path, default=Path("."))
    b.add_argument("--package", required=True, help="Go main package, e.g. ./cmd/kaizen")
    b.add_argument("--name", required=True)
    b.add_argument("--version", required=True)
    b.add_argument("--target", default="windows/amd64", choices=sorted(TARGETS))
    b.add_argument("--cgo", type=int, default=0, choices=[0, 1])
    b.add_argument("--out-dir", type=Path, default=Path("dist"))
    b.add_argument("--allow-dirty", action="store_true", help="local experiments only; manifest records dirty=true")
    v = sub.add_parser("verify", help="verify ZIP or unpacked release without running it")
    v.add_argument("release", type=Path)
    args = p.parse_args(argv)
    try:
        result = build(args) if args.command == "build" else verify_release(args.release.resolve())
    except (OSError, ValueError, KeyError, json.JSONDecodeError, zipfile.BadZipFile) as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
    print(json.dumps({"verified": True, "name": result["name"], "version": result["version"], "target": result["target"]}, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
