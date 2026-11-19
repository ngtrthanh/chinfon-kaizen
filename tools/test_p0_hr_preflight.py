#!/usr/bin/env python3
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from p0_hr_preflight import validate


class HRPreflightTests(unittest.TestCase):
    def test_valid_hierarchy_and_phone_free(self):
        source = ("emp_id,name,dept_code,dept_name,report_to,role\n"
                  "C001,General Director,BOD,Board,,employee\n"
                  "C002,Warehouse Manager,P81,Warehouse,C001,worker\n"
                  "C003,Storekeeper,P81,Warehouse,C002,employee\n")
        rows, errors, report = validate(source.encode("utf-8"))
        self.assertEqual(errors, [])
        self.assertEqual(report["valid_rows"], 3)
        self.assertEqual(report["without_email"], 3)
        self.assertEqual(report["proposed_role"], "WORKER_ONLY")
        self.assertTrue(all(r["role"] == "employee" and not r["mobile_num"] for r in rows))

    def test_unresolved_manager_is_hard_failure(self):
        data = b"emp_id,name,dept_code,report_to\nC001,Worker,P81,MISSING\n"
        _, errors, report = validate(data)
        self.assertTrue(any("report_to not in" in e for e in errors))
        self.assertGreater(report["errors"], 0)

    def test_reporting_cycle_is_hard_failure(self):
        data = b"emp_id,name,dept_code,report_to\nC001,A,P81,C002\nC002,B,P81,C001\n"
        _, errors, _ = validate(data)
        self.assertTrue(any("cycle" in e for e in errors))

    def test_privileged_import_blocked(self):
        data = b"emp_id,name,dept_code,role\nC001,Admin,P81,admin\n"
        _, errors, _ = validate(data)
        self.assertTrue(any("privileged" in e for e in errors))

    def test_duplicate_casefold_and_seed_pin_rejected(self):
        data = b"emp_id,name,dept_code,seed_pin\nc001,A,P81,12345\nC001,B,P81,\n"
        _, errors, _ = validate(data)
        self.assertTrue(any("duplicate" in e for e in errors))
        self.assertTrue(any("seed_pin" in e for e in errors))

    def test_spreadsheet_formula_rejected(self):
        data = b"emp_id,name,dept_code\nC001,=cmd(),P81\n"
        _, errors, _ = validate(data)
        self.assertTrue(any("formula" in e for e in errors))

    def test_empty_data_rejected(self):
        _, errors, _ = validate(b"emp_id,name,dept_code\n")
        self.assertTrue(errors)


if __name__ == "__main__":
    unittest.main()
