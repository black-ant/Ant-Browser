# macOS Packaging

## Current Status

The repository currently supports unsigned macOS internal packages:

- local builds for `darwin/amd64` and `darwin/arm64`
- `.app` and `.zip` output from `publish/mac/publish-mac.sh`
- pinned Darwin `xray` and `sing-box` runtimes with manifest verification
- writable application state outside the `.app` bundle
- automatic GitHub Actions builds and Release uploads for both architectures

Code signing, notarization, DMG generation, and public macOS distribution are not implemented.

## Local Build

Run on a native macOS host with the same architecture as the target. Required tools:

- `python3`
- `ditto`
- `wails`
- `go`, `node`, and `npm`

Build an Apple Silicon package:

```bash
bash publish/mac/publish-mac.sh --arch arm64
```

Build an Intel package:

```bash
bash publish/mac/publish-mac.sh --arch amd64
```

The version defaults to `wails.json`. Override it for a package-only build with:

```bash
bash publish/mac/publish-mac.sh --arch arm64 --version 1.9.0
```

Available options:

- `--skip-frontend`: reuse existing `frontend/dist` while still building the native app
- `--skip-build`: reuse the existing Wails app under `build/bin/`
- `--skip-runtime-verify`: skip runtime manifest verification
- `--keep-staging`: keep the assembled bundle under `publish/staging/mac/`

Output files:

```text
publish/output/AntBrowser-<version>-macos-<arch>.app
publish/output/AntBrowser-<version>-macos-<arch>.zip
```

## GitHub Actions

Use **Actions → Build and Release → Run workflow**, push to `master` / `main`, or push a `v*` release tag. The unified workflow builds:

- `amd64` on `macos-15-intel`
- `arm64` on `macos-15`

The optional `version` input overrides the package version on branch runs. CI validates the app executable, bundled proxy runtimes, `Info.plist`, and ZIP extraction. Once all platform builds pass, the ZIPs are uploaded to the shared GitHub Release alongside Linux/Arch packages and `SHA256SUMS`. Branch runs publish prereleases; stable version tags publish formal releases.

Only the ZIP is uploaded, preserving the `.app` executable permissions. Packages remain unsigned and not notarized. CI does not prove Finder launch, browser-core startup, or real proxy connectivity. See [the Actions workflow documentation](../../.github/README.md).

## Bundle and User State

The current assembly layout is intentionally simple:

```text
Ant Browser.app/Contents/MacOS/ant-chrome
Ant Browser.app/Contents/MacOS/bin/xray
Ant Browser.app/Contents/MacOS/bin/sing-box
Ant Browser.app/Contents/MacOS/config.yaml
Ant Browser.app/Contents/MacOS/chrome/README.md
```

When the application runs from an `.app` bundle, writable state is redirected to:

```text
~/Library/Application Support/ant-browser/
```

Configuration, database, logs, browser cores, and profile data belong in the user state root. Proxy runtime binaries remain in the app bundle.

## Running an Unsigned Build

For a package downloaded or copied from another machine, macOS may apply quarantine metadata. For an internal test package only:

```bash
xattr -dr com.apple.quarantine "AntBrowser-<version>-macos-<arch>.app"
open "AntBrowser-<version>-macos-<arch>.app"
```

Unsigned builds are for internal testing. Removing quarantine is not a substitute for signing or notarization.

## Validation Boundary

The repository and CI currently validate package assembly and file structure. A complete macOS release still requires testing on real hardware for:

- first launch from Finder and `/Applications`
- user-state creation and migration
- browser-core detection and instance startup
- `xray` and `sing-box` process startup
- clean window close and application quit
- signing, notarization, and Gatekeeper behavior
