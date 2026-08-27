#!/usr/bin/env bash
set -euo pipefail

# ./hack/release.sh — Calculate and create the next release tag
#
# This script looks up the latest semantic version tag (vX.Y.Z)
# from the upstream repo Fyve-Labs/fyve-cli, calculates the
# next version according to the requested bump (patch|minor|major),
# updates the .deb install instructions in README.md to the next version,
# then optionally tags the current repository and pushes the tag to origin.
#
# Usage:
#   ./release.sh [patch|minor|major] [-y] [--no-push] [--dry-run]
#
# Defaults:
#   bump = patch
#   confirm before tagging/pushing unless -y is given
#
# Examples:
#   ./release.sh            # bump patch from upstream latest
#   ./release.sh minor -y   # bump minor and push without confirmation
#   ./release.sh major --dry-run   # show what would happen
#

UPSTREAM_REPO_URL="https://github.com/Fyve-Labs/fyve-cli.git"
BUMP="patch"
ASSUME_YES="false"
NO_PUSH="false"
DRY_RUN="false"

while (( "$#" )); do
  case "$1" in
    patch|minor|major)
      BUMP="$1";
      shift ;;
    -y|--yes)
      ASSUME_YES="true"; shift ;;
    --no-push)
      NO_PUSH="true"; shift ;;
    -n|--dry-run)
      DRY_RUN="true"; shift ;;
    -h|--help)
      sed -n '1,80p' "$0"; exit 0 ;;
    *)
      echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

# Fetch latest tag from upstream repo
# List all vX.Y.Z tags, normalize, version-sort and pick the latest.
LATEST_TAG_NAME=$(
  git ls-remote --tags "$UPSTREAM_REPO_URL" 'v[0-9]*.[0-9]*.[0-9]*' 2>/dev/null \
    | awk '{print $2}' \
    | sed -E 's#\^\{\}$##; s#^refs/tags/##' \
    | awk '{orig=$0; gsub(/^v/, "", $0); split($0,a,/[.]/); printf "%03d.%03d.%03d %s\n", a[1]+0, a[2]+0, a[3]+0, orig}' \
    | sort \
    | tail -n1 \
    | awk '{print $2}'
) || true

if [[ -z "${LATEST_TAG_NAME}" ]]; then
  echo "No existing tags found in upstream. Defaulting to v0.0.0" >&2
  LATEST_TAG_NAME="v0.0.0"
fi

# Strip leading v and split into components
VERSION=${LATEST_TAG_NAME#v}
IFS='.' read -r MAJOR MINOR PATCH <<<"$VERSION"

# Fallbacks if parse failed
MAJOR=${MAJOR:-0}
MINOR=${MINOR:-0}
PATCH=${PATCH:-0}

# Remember current version (pre-bump)
CURRENT_VERSION="${MAJOR}.${MINOR}.${PATCH}"

case "$BUMP" in
  patch)
    PATCH=$((PATCH + 1)) ;;
  minor)
    MINOR=$((MINOR + 1)); PATCH=0 ;;
  major)
    MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
  *)
    echo "Invalid bump type: $BUMP" >&2; exit 1 ;;
esac

NEXT_VERSION="${MAJOR}.${MINOR}.${PATCH}"
NEXT_TAG="v${NEXT_VERSION}"

echo "Upstream latest tag: ${LATEST_TAG_NAME}"
echo "Requested bump:      ${BUMP}"
echo "Next tag to release: ${NEXT_TAG}"

if [[ "$DRY_RUN" == "true" ]]; then
  echo "Will update README.md install instructions:"
  echo "  /releases/download/v${CURRENT_VERSION}/ → /releases/download/v${NEXT_VERSION}/"

  echo "Dry run enabled. No tag will be created."
  exit 0
fi

# Verify we are on a clean working tree to avoid accidental tagging
if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "Working tree has uncommitted changes. Please commit or stash before releasing." >&2
  exit 1
fi

# Create annotated tag
if git rev-parse "$NEXT_TAG" >/dev/null 2>&1; then
  echo "Tag $NEXT_TAG already exists locally." >&2
else
  echo "Creating tag $NEXT_TAG ..."
  git tag -a "$NEXT_TAG" -m "Release $NEXT_TAG"
fi

# Push tag
if [[ "$NO_PUSH" == "true" ]]; then
  echo "--no-push specified. Not pushing tag."
  exit 0
fi

echo "Pushing tag $NEXT_TAG to origin ..."
git push origin "$NEXT_TAG"

echo "Done."