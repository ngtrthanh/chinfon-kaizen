#!/usr/bin/env python3
"""Offline SSOT-backed ASVS Level 2 audit ledger and MIS-friendly CSV export."""
import csv
import json
import pathlib
import sys
ROOT = pathlib.Path(__file__).resolve().parents[1]
ASVS = ROOT / "security/asvs"
VALID = {"PASS","FAIL","NOT_TESTED","NOT_APPLICABLE","REMEDIATED_UNVERIFIED"}
def main():
    c = json.loads((ASVS / "criteria-5.0.0-L2.json").read_text(encoding="utf8"))
    a = json.loads((ASVS / "assessment.json").read_text(encoding="utf8"))
    rows = c["requirements"]
    ids = [x["id"] for x in rows]
    assert len(rows) == 253 and len(set(ids)) == 253, "baseline mismatch"
    assert sum(x["level"] == 1 for x in rows) == 70
    assert sum(x["level"] == 2 for x in rows) == 183
    assert not set(a["controls"]) - set(ids), "unknown requirement id"
    out = []
    for req in rows:
        val = a["controls"].get(req["id"], {})
        status = val.get("status", "NOT_TESTED")
        if status not in VALID: raise ValueError(f"{req['id']}: invalid status")
        if status in ("PASS","NOT_APPLICABLE") and not val.get("evidence"):
            raise ValueError(f"{req['id']}: claimed {status} without evidence")
        if status == "NOT_APPLICABLE" and not val.get("rationale"):
            raise ValueError(f"{req['id']}: N/A requires rationale")
        out.append([req["id"],req["level"],req["chapter"],req["text"],status,
                    "; ".join(val.get("evidence",[])),val.get("reviewer") or "",
                    val.get("rationale") or "",val.get("note") or ""])
    from collections import Counter
    cnt=Counter(row[4] for row in out)
    print(f"ASVS 5.0.0 L2: {len(out)} required (L1=70, L2=183); statuses: {dict(cnt)}")
    print("Compliance NOT CERTIFIED unless all applicable requirements are independently verified PASS.")
    if len(sys.argv)>1 and sys.argv[1]=="--csv":
        path = pathlib.Path(sys.argv[2] if len(sys.argv)>2 else "asvs-review.csv")
        with path.open("w", encoding="utf-8-sig",newline="") as f:
            writer=csv.writer(f)
            writer.writerow(["ID","Level","Chapter","Requirement","Status","Evidence","Reviewer","N/A rationale","Note"])
            writer.writerows(out)
        print("CSV:",path)
if __name__=="__main__":main()
