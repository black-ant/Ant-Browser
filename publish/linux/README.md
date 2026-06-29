# Linux Publish

## Targets

- `linux/amd64`
- `linux/arm64`

Output artifacts:

- `publish/output/MakaBrowser-<version>-linux-<arch>.tar.gz`
- `publish/output/ant-browser_<version>_<arch>.deb`

## Runtime policy

Linux publish uses repository-pinned runtime files and hash verification.

Pinned upstream source lock file:

- `publish/runtime-sources.json`

Required files:

- `bin/linux-amd64/xray`
- `bin/linux-amd64/sing-box`
- `bin/linux-arm64/xray`
- `bin/linux-arm64/sing-box`

Hashes are validated by:

- `tools/runtime/verify-runtime.sh`
- `publish/runtime-manifest.json`

Recommended way to refresh runtime files (download + archive checksum + extract + manifest update):

```bash
python3 tools/runtime/sync-runtime.py
python3 tools/runtime/sync-runtime.py --target linux-amd64
python3 tools/runtime/sync-runtime.py --target linux-arm64
```

If you replace runtime files manually, update manifest hashes:

```bash
python3 tools/runtime/update-runtime-manifest.py --target linux-amd64
python3 tools/runtime/update-runtime-manifest.py --target linux-arm64
```

If runtime file or hash is missing/mismatched, publish will fail.

## Commands

Single architecture:

```bash
bash publish/linux/publish-linux.sh --arch amd64
bash publish/linux/publish-linux.sh --arch arm64
```

From Windows wrapper (`bat\publish.bat L`), Linux publish is executed through Docker Desktop using `publish/linux/linux-builder.Dockerfile`.

Batch call:

```bash
bash publish/linux/publish-linux-all.sh
```

## Notes

- Linux packages do **not** include browser cores (`chrome/` is not bundled).
- Build on native architecture runner for stability.
- `.deb` installs app files under `/opt/ant-browser`.
- `.deb` bundles `xray` and `sing-box` under `/opt/ant-browser/bin`.
- Linux packages keep an empty `chrome/` placeholder with `README.md`, but do **not** bundle browser core binaries.
- `.deb` registers an application launcher at `/usr/share/applications/ant-browser.desktop`.
- `.deb` installs standard Linux desktop icons under `/usr/share/icons/hicolor/*/apps/maka-browser.png` and `/usr/share/pixmaps/maka-browser.png`, so menus and launchers are more likely to pick up the app icon correctly.
- `.deb` bundles AppStream metadata under `/usr/share/metainfo/ant-browser.metainfo.xml`, which improves recognition in software centers and GUI `.deb` installers.
- On Debian/Ubuntu desktop environments that already support local `.deb` GUI installers, the package can usually be installed by double-clicking it; if the host has no GUI installer association, use `sudo apt install ./ant-browser_<version>_<arch>.deb`.
- Linux packages currently register the app in the desktop launcher/menu; they do not force-create a shortcut file on each user's desktop.
