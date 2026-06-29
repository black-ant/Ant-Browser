# macOS Publish Plan

## Purpose

This document defines the macOS packaging plan for Maka Browser.

The goal is to turn the current codebase into a macOS build that can:

- build on a native macOS machine
- launch from `/Applications`
- keep user-writable state outside the `.app` bundle
- bundle required proxy runtime binaries
- avoid breaking existing Windows and Linux packaging flows

## Current Entry Command

The initial internal-build script can be invoked on a real Mac with:

```bash
bash publish/mac/publish-mac.sh --arch arm64
```

For the first iteration, `arm64` is the recommended target.

This is a plan document only. It does not mean macOS packaging is already implemented.

## Current Status

The repository already has:

- Windows packaging flow
- Linux packaging flow
- partial Darwin runtime compatibility in backend code
- initial `publish/mac/publish-mac.sh` scaffold for internal test builds
- initial `publish/config.init.mac.yaml` template
- committed Darwin runtime binaries under `bin/darwin-amd64/` and `bin/darwin-arm64/`
- Darwin runtime entries in `publish/runtime-manifest.json`
- Darwin runtime source lock entries in `publish/runtime-sources.json`

The repository does not yet have:

- signing / notarization flow

The repository now includes the first macOS writable-state implementation for app bundle roots:

- when the app root is inside `.app/Contents/MacOS` or `.app/Contents/Resources`
- writable state is redirected to `~/Library/Application Support/ant-browser`
- `bin/` stays in the app bundle
- config, chrome, and data move to the user state root

## Current Implementation Note

The first Maka Browser bridge package intentionally keeps `Ant Browser.app` as
the app-update archive root so existing Ant Browser clients can install it.
Visible app metadata is Maka Browser. The scaffold places helper binaries and
seed files under:

- `Ant Browser.app/Contents/MacOS/bin`
- `Ant Browser.app/Contents/MacOS/config.yaml`
- `Ant Browser.app/Contents/MacOS/chrome/README.md`

This is not the prettiest final bundle layout, but it matches the current runtime path resolution and avoids a larger refactor in Phase 1.

After the first internal build is stable, the bundle layout can be reviewed and moved toward `Contents/Resources` if needed.

## Why macOS Looks More Complex

macOS is not difficult because of Wails alone. The real complexity comes from four areas:

1. Installed `.app` bundles under `/Applications` should be treated as read-only.
2. User data must not be written inside the `.app` bundle.
3. External helper binaries such as `xray` and `sing-box` must exist for Darwin and must be bundled correctly.
4. Public distribution usually requires code signing and notarization, otherwise Gatekeeper may block launch.

## Recommended Scope

### Phase 1: Internal Test Build

Target:

- `darwin/arm64` first
- output `.app` and `.zip`
- unsigned build is acceptable for internal testing

Why:

- Apple Silicon is the mainstream macOS target now
- it keeps the first version smaller and easier to verify
- it avoids spending time on Intel support before the runtime path is stable

### Phase 2: Public Distribution Build

Target:

- signed `.app`
- notarized `.zip` or `.dmg`
- optional `darwin/amd64` or universal build

Why:

- end users expect double-click install and normal launch
- unsigned apps and embedded helper binaries are more likely to be blocked

## Recommended Runtime Layout

### App Bundle

Recommended structure inside the built app:

- `Ant Browser.app/Contents/MacOS/ant-chrome`
- `Ant Browser.app/Contents/Resources/bin/xray`
- `Ant Browser.app/Contents/Resources/bin/sing-box`
- optional placeholder `chrome/README.md` if you want to keep behavior aligned with Linux

### User-Writable State

Recommended macOS state root:

- `~/Library/Application Support/ant-browser`

Recommended contents under the state root:

- `config.yaml`
- `proxies.yaml`
- `data/`
- `chrome/`

Rule:

- runtime binaries stay in the app bundle
- config, database, browser cores, logs, and profile data stay in the user state root

## Code Changes Required

### 1. Add macOS Writable State Handling

Current Linux detached-state logic only activates on Linux:

- `backend/internal/apppath/apppath.go`

Required change:

- extend path detection so installed macOS apps also use a detached writable state root
- recommended trigger: when `GOOS=darwin` and app root is not writable, or when running from an `.app` bundle

Expected result:

- app launch from `/Applications` does not try to write config/data into the bundle

### 2. Add Darwin Runtime Binaries

Current runtime manifest has Windows and Linux only:

- `publish/runtime-manifest.json`

Required additions:

- `bin/darwin-arm64/xray`
- `bin/darwin-arm64/sing-box`
- optional `bin/darwin-amd64/xray`
- optional `bin/darwin-amd64/sing-box`
- manifest hash entries for the new targets

Status:

- implemented for both `darwin-arm64` and `darwin-amd64`
- files are committed into the repository
- runtime manifest verification now works for both Darwin targets

Related scripts to extend:

- `tools/runtime/sync-runtime.py`
- `tools/runtime/update-runtime-manifest.py`
- `tools/runtime/verify-runtime.sh`

### 3. Add macOS Publish Script

New file to add:

- `publish/mac/publish-mac.sh`

Recommended responsibilities:

1. verify host is macOS
2. verify target arch (`arm64` first)
3. install frontend dependencies
4. build frontend
5. run `wails build -platform darwin/arm64`
6. place runtime binaries into the app bundle
7. optionally archive to `.zip`
8. optionally sign and notarize when environment variables are provided

Current scaffold status:

- implemented as an unsigned internal-build script
- outputs `.app` plus `.zip`
- requires a native macOS host
- intentionally does not attempt notarization yet

### 4. Add macOS Runtime Placement Logic

The app currently resolves most paths through shared runtime helpers, which is good.

Files likely involved:

- `backend/app.go`
- `backend/app_paths.go`
- `backend/app_utils.go`
- `backend/internal/browser/types.go`
- `backend/internal/proxy/xray.go`
- `backend/internal/proxy/singbox.go`
- `main.go`

Goal:

- all writable files go to the user state root
- helper binaries continue to resolve from the app bundle

### 5. Signing and Notarization

This is not required for a first internal test build, but is required for a serious public release.

Needed later:

- Apple Developer certificate
- `codesign`
- `notarytool`
- entitlements if runtime behavior requires them

Typical flow:

1. sign helper binaries
2. sign the `.app`
3. zip or build dmg
4. notarize
5. staple

## Recommended Implementation Order

1. Deliver `darwin/arm64` internal test build only.
2. Add macOS detached state root.
3. Add Darwin runtime binaries and manifest support.
4. Add `publish/mac/publish-mac.sh`.
5. Verify launch from `/Applications`.
6. Verify browser core placement under user state root.
7. Verify proxy runtime launch on macOS.
8. Add signing and notarization only after the unsigned build is stable.
9. Decide whether `darwin/amd64` is worth supporting.

## Validation Checklist

The macOS work should not be considered complete until all items below are verified on a real Mac.

### Packaging

- build completes on native macOS
- output `.app` exists
- output `.zip` or `.dmg` exists
- bundled helper binaries are executable

### First Launch

- app launches from Finder
- app launches after copying to `/Applications`
- first launch creates `~/Library/Application Support/ant-browser`
- `config.yaml` is seeded correctly
- database and `data/` are created under the user state root

### Browser Core

- manually placed browser core can be detected
- browser core path persists in config or database
- browser instance can actually start

### Proxy Runtime

- `xray` can be launched from the app bundle
- `sing-box` can be launched from the app bundle
- work directories are created under the user state root

### Exit Behavior

- window close works
- explicit quit works
- no stuck background process remains after quit

### Regression Safety

- Windows packaging still builds
- Linux packaging still builds
- Linux detached state behavior still works

## Difficulty Assessment

### Internal Test Build

Difficulty: medium

Main blockers:

- mac runtime binaries
- detached writable state
- mac packaging script

### Public Release Build

Difficulty: medium-high

Main blockers:

- signing
- notarization
- quarantine / Gatekeeper behavior
- helper binary signing order

## Suggested First Deliverable

The safest first milestone is:

- macOS `arm64`
- native build on a real Mac
- unsigned `.app`
- zipped artifact for internal testing
- detached writable state under `~/Library/Application Support/ant-browser`
- bundled `xray` and `sing-box`

Do not start with:

- universal binary
- dmg beautification
- public distribution
- Intel support

Those can come after the app is proven stable on one Mac target first.

## Files Expected To Be Added Or Updated

Likely new files:

- `publish/mac/publish-mac.sh`
- `publish/mac/README.md`

Likely updated files:

- `backend/internal/apppath/apppath.go`
- `backend/internal/apppath/apppath_test.go`
- `backend/runtime_paths.go`
- `backend/app.go`
- `main.go`
- `publish/runtime-manifest.json`
- `publish/runtime-sources.json`
- `tools/runtime/sync-runtime.py`
- `tools/runtime/update-runtime-manifest.py`
- `tools/runtime/verify-runtime.sh`

## Decision Record

Current recommendation:

- do macOS `arm64` first
- solve writable state before touching signing
- keep Windows and Linux publish flows unchanged unless shared runtime code needs extension
- treat public notarized distribution as Phase 2, not Phase 1
