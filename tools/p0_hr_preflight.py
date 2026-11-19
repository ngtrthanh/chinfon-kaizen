#!/usr/bin/env python3
"""Strict HR source preflight for the OrgAxis -> Kaizen P0 real-data rollout.

No network, no credentials, no mutations to application DB. Validation-only by
default; --output writes a fresh CSV with all roles set to WORKER/employee.
This is NOT an OrgAxis structural importer and does not prove position slots.
"""
from __future__ import annotations
import argparse
import csv
import hashlib
import io
import json
import os
from pathlib import Path
import re
import sys

FIELDS = ("emp_id", "name", "job_title", "dept_code", "dept_name",
          "report_to", "e_mail", "mobile_num", "seed_pin", "role", "group")
REQUIRED = ("emp_id", "name", "dept_code")
VALID_CODE = re.compile(r"^[A-Z0-9][A-Z0-9_.-]{0,49}$")
FORBIDDEN_FORMULA_PREFIX = ("=", "+", "-", "@", "\t", "\r")


def validate(data: bytes) -> tuple[list[dict[str, str]], list[str], dict]:
    """Return normalized rows, redacted errors and non-PII statistics."""
    digest = hashlib.sha256(data).hexdigest()
    try:
        decoded = data.decode("utf-8-sig")
    except UnicodeDecodeError:
        return [], ["CSV must be UTF-8"], {"sha256": digest}
    try:
        rows = list(csv.DictReader(io.StringIO(decoded, newline="")))
    except csv.Error:
        return [], ["CSV parsing error"], {"sha256": digest}
    if not rows:
        return [], ["Empty CSV; at least one HR-approved record is required"], {"sha256": digest}
    reader = csv.DictReader(io.StringIO(decoded, newline=""))
    columns = reader.fieldnames or []
    if len(set(columns)) != len(columns):
        return [], ["Duplicate CSV header"], {"sha256": digest}
    missing = [key for key in REQUIRED if key not in columns]
    if missing:
        return [], ["Missing required columns: " + ",".join(missing)], {"sha256": digest}
    if len(rows) > 5000:
        return [], ["Staff import exceeds Kaizen's 5000-row limit"], {"sha256": digest}
    errors = []
    seen = set()
    normalized = []
    departments = {}
    for lineno, original in enumerate(rows, 2):
        if None in original:
            errors.append(f"row {lineno}: extra unheaded columns")
            continue
        row = {k: str(original.get(k) or "").strip() for k in FIELDS}
        row["emp_id"] = row["emp_id"].upper()
        row["dept_code"] = row["dept_code"].upper()
        row["report_to"] = row["report_to"].upper()
        code = row["emp_id"]
        if not VALID_CODE.fullmatch(code):
            errors.append(f"row {lineno}: invalid emp_id")
            continue
        if code in seen:
            errors.append(f"row {lineno}: duplicate emp_id")
            continue
        seen.add(code)
        if not row["name"] or len(row["name"]) > 120:
            errors.append(f"row {lineno}: invalid name")
        if not VALID_CODE.fullmatch(row["dept_code"]) or len(row["dept_code"]) > 20:
            errors.append(f"row {lineno}: invalid dept_code")
        if row["report_to"] and not VALID_CODE.fullmatch(row["report_to"]):
            errors.append(f"row {lineno}: invalid report_to")
        if row["report_to"] == code:
            errors.append(f"row {lineno}: self-reporting is forbidden")
        for k in ("name", "job_title", "dept_name", "group"):
            if row[k].startswith(FORBIDDEN_FORMULA_PREFIX):
                errors.append(f"row {lineno}: spreadsheet formula prefix in {k}")
        if row["seed_pin"]:
            errors.append(f"row {lineno}: seed_pin is prohibited; invite-only onboarding required")
        if row["role"].lower() not in ("", "employee", "worker"):
            errors.append(f"row {lineno}: privileged role assignment requires independent approval; import workers only")
        if row["mobile_num"]:
            errors.append(f"row {lineno}: mobile_num not required; do not export private phone numbers to this P0 feed")
        if row["e_mail"] and ("@" not in row["e_mail"] or " " in row["e_mail"]):
            errors.append(f"row {lineno}: invalid optional email")
        if row["dept_name"]:
            known = departments.get(row["dept_code"])
            if known and known != row["dept_name"]:
                errors.append(f"row {lineno}: contradictory dept_name for same dept_code")
            departments[row["dept_code"]] = row["dept_name"]
        row["role"] = "employee"
        row["mobile_num"] = ""
        row["seed_pin"] = ""
        normalized.append(row)

    employees = {r["emp_id"]: r for r in normalized}
    if len(employees) == len(normalized):
        for row in normalized:
            manager = row["report_to"]
            if manager and manager not in employees:
                errors.append(f"employee {row['emp_id']}: report_to not in import dataset")
        colors = {}
        def visit(key: str) -> None:
            if colors.get(key) == 1:
                errors.append(f"employee {key}: reporting cycle")
                return
            if colors.get(key) == 2:
                return
            colors[key] = 1
            m = employees[key]["report_to"]
            if m and m in employees:
                visit(m)
            colors[key] = 2
        for key in employees:
            if not colors.get(key):
                visit(key)
    stats = {
        "sha256": digest, "rows": len(rows), "valid_rows": len(normalized),
        "departments": len({r["dept_code"] for r in normalized}),
        "with_manager": sum(bool(r["report_to"]) for r in normalized),
        "without_email": sum(not r["e_mail"] for r in normalized),
        "proposed_role": "WORKER_ONLY", "errors": len(errors),
    }
    return normalized, errors, stats


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--input", required=True, type=Path)
    ap.add_argument("--output", type=Path, help="Write a new Kaizen-compatible CSV ONLY if no errors")
    ap.add_argument("--report", type=Path, help="Write a redacted JSON summary")
    args = ap.parse_args(argv)
    try:
        content = args.input.read_bytes()
    except OSError as exc:
        print(f"Cannot read input: {type(exc).__name__}", file=sys.stderr)
        return 2
    rows, errors, stats = validate(content)
    summary = dict(stats, accepted=(len(errors) == 0), error_list=errors)
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    if args.report:
        fd = os.open(str(args.report), os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, "w", encoding="utf-8") as report:
            json.dump(summary, report, ensure_ascii=False, indent=2)
    if errors:
        return 1
    if args.output:
        fd = os.open(str(args.output), os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, "w", encoding="utf-8", newline="") as dest:
            writer = csv.DictWriter(dest, fieldnames=FIELDS, lineterminator="\n")
            writer.writeheader()
            writer.writerows(rows)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
