"""Offline smoke tests: temp Go module, Windows cross-compile, integrity, dirty gate."""
from __future__ import annotations
import importlib.util
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SKILL = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("package_go", SKILL / "scripts" / "package_go.py")
package_go = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(package_go)


class PackageGoTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / "go.mod").write_text("module example.com/releasecheck\n\ngo 1.23\n")
        (self.root / "main.go").write_text('package main\nimport _ "embed"\n//go:embed web.txt\nvar text string\nfunc main(){if text==""{panic("embed failed")}}\n')
        (self.root / "web.txt").write_text("bundled content\n")
        (self.root / ".gitignore").write_text("dist/\n")
        for args in (["git", "init", "-q"], ["git", "add", "."], ["git", "-c", "user.email=test@example.test", "-c", "user.name=test", "commit", "-qm", "fixture"]):
            subprocess.run(args, cwd=self.root, check=True)

    def test_windows_release_and_corruption(self):
        result = subprocess.run([sys.executable, str(SKILL / "scripts" / "package_go.py"), "build", "--project-root", str(self.root), "--package", ".", "--name", "fixture", "--version", "0.1.0", "--target", "windows/amd64"], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        zip_path = self.root / "dist/fixture_0.1.0_windows_amd64.zip"
        manifest = package_go.verify_release(zip_path)
        self.assertTrue(manifest["binary"]["filename"].endswith(".exe"))
        self.assertFalse(manifest["git_dirty"])
        folder = zip_path.with_suffix("")
        self.assertNotIn(b"\r\n", (folder / "SHA256SUMS").read_bytes())
        exe = folder / "fixture.exe"
        exe.write_bytes(exe.read_bytes() + b"corruption")
        with self.assertRaises(ValueError):
            package_go.verify_release(folder)

    def test_linux_release_verifies(self):
        result = subprocess.run([sys.executable, str(SKILL / "scripts" / "package_go.py"), "build", "--project-root", str(self.root), "--package", ".", "--name", "fixture", "--version", "0.1.1", "--target", "linux/amd64"], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        archive = self.root / "dist/fixture_0.1.1_linux_amd64.zip"
        self.assertEqual(package_go.verify_release(archive)["target"], "linux/amd64")

    def test_dirty_tree_denied(self):
        (self.root / "web.txt").write_text("modified\n")
        cmd = [sys.executable, str(SKILL / "scripts" / "package_go.py"), "build", "--project-root", str(self.root), "--package", ".", "--name", "fixture", "--version", "0.2.0"]
        result = subprocess.run(cmd, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("dirty/unversioned", result.stderr)

    def test_invalid_release_name_denied(self):
        self.assertRaises(ValueError, lambda: package_go.build(type("A", (), {"name":"../bad", "version":"v1"})()))


if __name__ == "__main__":
    unittest.main()
