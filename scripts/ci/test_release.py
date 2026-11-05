import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

import release


class MetadataTests(unittest.TestCase):
    def setUp(self):
        self.env = {"GITHUB_REF": "refs/heads/master", "GITHUB_SHA": "a" * 40,
                    "GITHUB_RUN_NUMBER": "12"}

    def test_branch_build_is_immutable_prerelease(self):
        self.assertEqual(release.release_metadata(self.env, "1.9.0"),
                         {"version": "1.9.0", "tag": "ci-12-aaaaaaa", "prerelease": "true"})

    def test_manual_version_override(self):
        self.env["INPUT_VERSION"] = " 2.0.0 "
        self.assertEqual(release.release_metadata(self.env, "1.9.0")["version"], "2.0.0")

    def test_tag_is_authoritative(self):
        self.env["GITHUB_REF"] = "refs/tags/v2.1.0"
        self.assertEqual(release.release_metadata(self.env, "1.9.0"),
                         {"version": "2.1.0", "tag": "v2.1.0", "prerelease": "false"})

    def test_prerelease_tag(self):
        self.env["GITHUB_REF"] = "refs/tags/v2.1.0-beta.1"
        self.assertEqual(release.release_metadata(self.env, "1.9.0")["prerelease"], "true")

    def test_tag_override_mismatch_fails(self):
        self.env.update(GITHUB_REF="refs/tags/v2.0.0", INPUT_VERSION="3.0.0")
        with self.assertRaises(ValueError):
            release.release_metadata(self.env, "1.9.0")

    def test_missing_run_identity_fails(self):
        self.env.pop("GITHUB_SHA")
        with self.assertRaises(ValueError):
            release.release_metadata(self.env, "1.9.0")

    def test_unsafe_or_invalid_versions_fail(self):
        for version in ("", "v1.9.0", "1.9", "1.09.0", "1.9.0/../../x", "1.9.0\nx=y", "$(id)", "-1.9.0"):
            with self.subTest(version=version), self.assertRaises(ValueError):
                release.validate_version(version)

    def test_unsafe_tag_fails(self):
        self.env["GITHUB_REF"] = "refs/tags/v1.0.0\ninjected=true"
        with self.assertRaises(ValueError):
            release.release_metadata(self.env, "1.9.0")


class AssetTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        for name in release.expected_assets("1.9.0"):
            (self.directory / name).write_bytes(name.encode())

    def test_complete_set_and_checksums(self):
        lines = release.collect_assets(self.directory, "1.9.0")
        self.assertEqual(len(lines), 7)
        for line in lines:
            digest, name = line.rstrip().split("  ")
            self.assertEqual(digest, hashlib.sha256((self.directory / name).read_bytes()).hexdigest())
        self.assertEqual(release.collect_assets(self.directory, "1.9.0"), lines)

    def test_missing_platform_fails(self):
        (self.directory / "AntBrowser-1.9.0-macos-arm64.zip").unlink()
        with self.assertRaisesRegex(ValueError, "missing="):
            release.collect_assets(self.directory, "1.9.0")

    def test_frontend_artifact_is_rejected(self):
        (self.directory / "index.html").write_text("not a release package")
        with self.assertRaisesRegex(ValueError, "unexpected="):
            release.collect_assets(self.directory, "1.9.0")

    def test_empty_asset_fails(self):
        next(self.directory.iterdir()).write_bytes(b"")
        with self.assertRaisesRegex(ValueError, "non-empty regular file"):
            release.collect_assets(self.directory, "1.9.0")

    def test_symlink_asset_fails(self):
        asset = next(self.directory.iterdir())
        asset.unlink()
        asset.symlink_to(__file__)
        with self.assertRaisesRegex(ValueError, "non-empty regular file"):
            release.collect_assets(self.directory, "1.9.0")

    def test_arch_prerelease_name(self):
        self.assertIn("ant-browser-2.0.0_beta.1-1-x86_64.pkg.tar.zst", release.expected_assets("2.0.0-beta.1"))


class VersionTests(unittest.TestCase):
    def test_all_version_fields_are_updated(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for name in ("wails.json", "frontend/package.json", "frontend/package-lock.json", "packaging/arch/PKGBUILD"):
                (root / name).parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(release.ROOT / name, root / name)
            release.apply_version(root, "2.0.0-beta.1")
            self.assertEqual(json.loads((root / "wails.json").read_text())["info"]["productVersion"], "2.0.0-beta.1")
            self.assertEqual(json.loads((root / "frontend/package.json").read_text())["version"], "2.0.0-beta.1")
            lock = json.loads((root / "frontend/package-lock.json").read_text())
            self.assertEqual(lock["version"], "2.0.0-beta.1")
            self.assertEqual(lock["packages"][""]["version"], "2.0.0-beta.1")
            self.assertIn("pkgver=2.0.0_beta.1\n", (root / "packaging/arch/PKGBUILD").read_text())

    def test_lock_only_uses_official_registry(self):
        lock = json.loads((release.ROOT / "frontend/package-lock.json").read_text())
        for name, package in lock["packages"].items():
            if "resolved" in package:
                with self.subTest(package=name):
                    self.assertTrue(package["resolved"].startswith("https://registry.npmjs.org/"))


class PublishTests(unittest.TestCase):
    """Exercise the real release shell script without network access or credentials."""

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="ant-release test ")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.assets = self.root / "release-artifacts"
        self.assets.mkdir()
        for name in release.expected_assets("1.9.0"):
            (self.assets / name).write_bytes(name.encode())
        release.collect_assets(self.assets, "1.9.0")
        self.log = self.root / "calls.jsonl"
        mock = self.root / "gh"
        mock.write_text('''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
args = sys.argv[1:]
with open(os.environ['MOCK_LOG'], 'a') as handle:
    handle.write(json.dumps(args) + '\\n')
if args[:2] == ['release', 'view']:
    if '--json' in args:
        print('https://github.com/OWNER/REPO/releases/tag/ci-12-aaaaaaa')
    elif os.environ.get('RELEASE_EXISTS') != 'true':
        sys.exit(1)
elif args[0] == 'api':
    if os.environ.get('RELEASE_EXISTS') != 'true' and os.environ.get('TAG_EXISTS') != 'true':
        sys.exit(1)
    print(os.environ.get('TAG_SHA', os.environ['GITHUB_SHA']))
elif args[:2] == ['release', 'upload'] and os.environ.get('FAIL_UPLOAD') == 'true':
    sys.exit(1)
''')
        mock.chmod(0o755)
        self.env = dict(os.environ, PATH=f"{self.root}:{os.environ['PATH']}",
                        MOCK_LOG=str(self.log), GH_REPO="OWNER/REPO", GITHUB_SHA="a" * 40,
                        RELEASE_TAG="ci-12-aaaaaaa", RELEASE_VERSION="1.9.0", RELEASE_PRERELEASE="true",
                        GITHUB_RUN_ID="123", GITHUB_STEP_SUMMARY=str(self.root / "summary"))

    def run_publish(self):
        result = subprocess.run(["bash", str(release.ROOT / "scripts/ci/publish-release.sh")],
                                cwd=self.root, env=self.env, text=True, capture_output=True)
        calls = [json.loads(line) for line in self.log.read_text().splitlines()] if self.log.exists() else []
        return result, calls

    def test_snapshot_creates_draft_uploads_then_publishes(self):
        result, calls = self.run_publish()
        self.assertEqual(result.returncode, 0, result.stderr)
        create = next(c for c in calls if c[:2] == ["release", "create"])
        self.assertIn("--draft", create)
        self.assertIn("--prerelease", create)
        self.assertIn("--latest=false", create)
        upload_index = next(i for i, c in enumerate(calls) if c[:2] == ["release", "upload"])
        edit_index = next(i for i, c in enumerate(calls) if c[:2] == ["release", "edit"])
        self.assertLess(upload_index, edit_index)
        self.assertEqual(len(calls[upload_index][3:-1]), 8)  # seven packages + SHA256SUMS

    def test_stable_release_is_latest(self):
        self.env.update(RELEASE_PRERELEASE="false", RELEASE_TAG="v1.9.0", TAG_EXISTS="true")
        result, calls = self.run_publish()
        self.assertEqual(result.returncode, 0, result.stderr)
        edit = next(c for c in calls if c[:2] == ["release", "edit"])
        self.assertIn("--latest", edit)
        self.assertIn("--prerelease=false", edit)

    def test_rerun_does_not_recreate_release(self):
        self.env["RELEASE_EXISTS"] = "true"
        result, calls = self.run_publish()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(any(c[:2] == ["release", "create"] for c in calls))

    def test_existing_tag_other_commit_fails_before_upload(self):
        self.env.update(RELEASE_EXISTS="true", TAG_SHA="b" * 40)
        result, calls = self.run_publish()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("not " + "a" * 40, result.stderr)
        self.assertFalse(any(c[:2] == ["release", "upload"] for c in calls))

    def test_failed_upload_leaves_draft(self):
        self.env["FAIL_UPLOAD"] = "true"
        result, calls = self.run_publish()
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(any(c[:2] == ["release", "edit"] for c in calls))

    def test_corrupted_asset_fails_before_network(self):
        next(p for p in self.assets.iterdir() if p.name != "SHA256SUMS").write_bytes(b"corrupted")
        result, calls = self.run_publish()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(calls, [])


if __name__ == "__main__":
    unittest.main()
