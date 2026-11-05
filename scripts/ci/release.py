#!/usr/bin/env python3
"""Shared, dependency-free version and release-asset rules for GitHub Actions."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[2]
VERSION_RE = re.compile(r"(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z]+(?:\.[0-9A-Za-z]+)*)?")


def validate_version(version):
    if not VERSION_RE.fullmatch(version):
        raise ValueError(f"Invalid version {version!r}; expected 1.9.0 or 1.9.0-beta.1")
    return version


def arch_version(version):
    # makepkg pkgver must not contain a hyphen (it separates pkgrel).
    return validate_version(version).replace("-", "_")


def release_metadata(env, product_version):
    version = validate_version(env.get("INPUT_VERSION", "").strip() or product_version)
    ref = env.get("GITHUB_REF", "")
    if ref.startswith("refs/tags/"):
        tag = ref.removeprefix("refs/tags/")
        if not tag.startswith("v"):
            raise ValueError("Release tags must start with v")
        tag_version = validate_version(tag[1:])
        if env.get("INPUT_VERSION", "").strip() and version != tag_version:
            raise ValueError("Version override does not match release tag")
        version = tag_version
        prerelease = "-" in version
    else:
        run_number = env.get("GITHUB_RUN_NUMBER", "")
        sha = env.get("GITHUB_SHA", "")
        if not re.fullmatch(r"[0-9]+", run_number) or not re.fullmatch(r"[0-9a-f]{40}", sha):
            raise ValueError("A workflow run number and full commit SHA are required")
        tag = f"ci-{run_number}-{sha[:7]}"
        prerelease = True
    return {"version": version, "tag": tag, "prerelease": str(prerelease).lower()}


def apply_version(root, version):
    validate_version(version)
    for name in ("wails.json", "frontend/package.json", "frontend/package-lock.json"):
        path = root / name
        data = json.loads(path.read_text(encoding="utf-8"))
        if name == "wails.json":
            data["info"]["productVersion"] = version
        else:
            data["version"] = version
            if name.endswith("package-lock.json"):
                data["packages"][""]["version"] = version
        path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    path = root / "packaging/arch/PKGBUILD"
    text, count = re.subn(r"^pkgver=.*$", f"pkgver={arch_version(version)}", path.read_text(), count=1, flags=re.M)
    if count != 1:
        raise ValueError("PKGBUILD pkgver missing")
    path.write_text(text)


def expected_assets(version):
    validate_version(version)
    names = {f"ant-browser-{arch_version(version)}-1-x86_64.pkg.tar.zst"}
    for arch in ("amd64", "arm64"):
        names.update((f"ant-browser_{version}_{arch}.deb",
                      f"AntBrowser-{version}-linux-{arch}.tar.gz",
                      f"AntBrowser-{version}-macos-{arch}.zip"))
    return names


def collect_assets(directory, version):
    expected = expected_assets(version)
    actual = {path.name for path in directory.iterdir() if path.name != "SHA256SUMS"}
    if actual != expected:
        raise ValueError(f"Asset set mismatch; missing={sorted(expected - actual)}, unexpected={sorted(actual - expected)}")
    lines = []
    for name in sorted(expected):
        path = directory / name
        if path.is_symlink() or not path.is_file() or path.stat().st_size == 0:
            raise ValueError(f"Asset is not a non-empty regular file: {name}")
        with path.open("rb") as handle:
            digest = hashlib.file_digest(handle, "sha256").hexdigest()
        lines.append(f"{digest}  {name}\n")
    (directory / "SHA256SUMS").write_text("".join(lines), encoding="utf-8")
    return lines


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("metadata")
    commands.add_parser("version").add_argument("version")
    assets = commands.add_parser("assets")
    assets.add_argument("directory", type=Path)
    assets.add_argument("--version", required=True)
    args = parser.parse_args()
    if args.command == "metadata":
        version = json.loads((ROOT / "wails.json").read_text())["info"]["productVersion"]
        metadata = release_metadata(os.environ, version)
        apply_version(ROOT, metadata["version"])
        output = "".join(f"{key}={value}\n" for key, value in metadata.items())
        if os.environ.get("GITHUB_OUTPUT"):
            with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as handle:
                handle.write(output)
        print(output, end="")
    elif args.command == "version":
        apply_version(ROOT, args.version)
    else:
        print("".join(collect_assets(args.directory, args.version)), end="")


if __name__ == "__main__":
    main()
