#!/usr/bin/env bash
# Updates the version and checksums in teknikqa/macports-upkeep's Portfile
# for a new upkeep release, then commits and pushes. Run after a tag is
# pushed; MACPORTS_TAP_GITHUB_TOKEN must have contents:write on that repo.
set -euo pipefail

VERSION="${1:?usage: update-macports-portfile.sh <version, e.g. 0.11.1>}"
: "${MACPORTS_TAP_GITHUB_TOKEN:?MACPORTS_TAP_GITHUB_TOKEN must be set}"

TAP_REPO="teknikqa/macports-upkeep"
PORTFILE="sysutils/upkeep/Portfile"
SRC_URL="https://codeload.github.com/teknikqa/upkeep/legacy.tar.gz/v${VERSION}"

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT

archive="${workdir}/src.tar.gz"
curl -sL -o "$archive" "$SRC_URL"

sha256=$(shasum -a 256 "$archive" | cut -d' ' -f1)
rmd160=$(openssl dgst -rmd160 "$archive" | sed 's/^.*= //')
size=$(wc -c <"$archive" | tr -d ' ')

git clone --depth 1 "https://x-access-token:${MACPORTS_TAP_GITHUB_TOKEN}@github.com/${TAP_REPO}.git" "${workdir}/tap"
cd "${workdir}/tap"

sed -i '' \
  -e "s|^go.setup .*|go.setup            github.com/teknikqa/upkeep ${VERSION} v|" \
  "$PORTFILE"

# Rewrite the checksums block (three lines: rmd160, sha256, size).
awk -v rmd160="$rmd160" -v sha256="$sha256" -v size="$size" '
  /^checksums / { print "checksums           rmd160  " rmd160 " \\"; next }
  /^ *sha256  / { print "                    sha256  " sha256 " \\"; next }
  /^ *size    / { print "                    size    " size; next }
  { print }
' "$PORTFILE" >"${PORTFILE}.new"
mv "${PORTFILE}.new" "$PORTFILE"

if git diff --quiet -- "$PORTFILE"; then
  echo "Portfile already up to date for ${VERSION}"
  exit 0
fi

git config user.name "github-actions[bot]"
git config user.email "github-actions[bot]@users.noreply.github.com"
git add "$PORTFILE"
git commit -m "upkeep ${VERSION}"
git push
