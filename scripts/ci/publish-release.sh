#!/usr/bin/env bash
set -euo pipefail

: "${GH_REPO:?GH_REPO is required}"
: "${GITHUB_SHA:?GITHUB_SHA is required}"
: "${RELEASE_TAG:?RELEASE_TAG is required}"
: "${RELEASE_VERSION:?RELEASE_VERSION is required}"
: "${RELEASE_PRERELEASE:?RELEASE_PRERELEASE is required}"

# Verify checksums again immediately before the network upload.
(cd release-artifacts && sha256sum --check SHA256SUMS)

notes="$(mktemp)"
trap 'rm -f "$notes"' EXIT
cat > "$notes" <<EOF_NOTES
Built from commit ${GITHUB_SHA} by [GitHub Actions](https://github.com/${GH_REPO}/actions/runs/${GITHUB_RUN_ID}).

Downloads: Linux amd64/arm64 (.deb and .tar.gz), Arch Linux x86_64 (.pkg.tar.zst), macOS Intel/Apple Silicon (.zip).
macOS packages are unsigned and not notarized. Browser cores are downloaded separately by the application.
Verify downloads using SHA256SUMS. Package validation does not replace desktop launch and proxy-connectivity testing.
EOF_NOTES

release_exists=false
if gh release view "$RELEASE_TAG" >/dev/null 2>&1; then
  release_exists=true
fi

# A rerun may refresh assets, but never attach a different commit to an old tag.
if tag_sha="$(gh api "repos/${GH_REPO}/commits/${RELEASE_TAG}" --jq .sha 2>/dev/null)"; then
  if [[ "$tag_sha" != "$GITHUB_SHA" ]]; then
    echo "Release tag points to $tag_sha, not $GITHUB_SHA" >&2
    exit 1
  fi
elif [[ "$release_exists" == true ]]; then
  echo 'Existing release tag could not be verified' >&2
  exit 1
fi

flags=()
if [[ "$RELEASE_PRERELEASE" == true ]]; then
  flags+=(--prerelease --latest=false)
fi
if [[ "$release_exists" == false ]]; then
  # Keep incomplete uploads out of the public release list.
  gh release create "$RELEASE_TAG" --target "$GITHUB_SHA" \
    --title "Ant Browser ${RELEASE_VERSION} (${RELEASE_TAG})" \
    --draft --generate-notes --notes-file "$notes" "${flags[@]}"
fi
gh release upload "$RELEASE_TAG" release-artifacts/* --clobber
if [[ "$RELEASE_PRERELEASE" == true ]]; then
  gh release edit "$RELEASE_TAG" --draft=false --prerelease --latest=false
else
  gh release edit "$RELEASE_TAG" --draft=false --prerelease=false --latest
fi
url="$(gh release view "$RELEASE_TAG" --json url --jq .url)"
printf '### Published release\n\n%s\n' "$url" >> "$GITHUB_STEP_SUMMARY"
echo "$url"
