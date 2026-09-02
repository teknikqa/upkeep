#!/usr/bin/env bash
# Updates the version and checksums in teknikqa/macports-upkeep's Portfile
# for a new upkeep release, then commits and pushes. Run after a tag is
# pushed; MACPORTS_TAP_GITHUB_TOKEN must have contents:write on that repo.
set -euo pipefail

VERSION="${1:?usage: update-macports-portfile.sh <version, e.g. 0.11.1>}"
: "${MACPORTS_TAP_GITHUB_TOKEN:?MACPORTS_TAP_GITHUB_TOKEN must be set}"

TAP_REPO="teknikqa/macports-upkeep"
PORTFILE="sysutils/upkeep/Portfile"
RELEASE_BASE="https://github.com/teknikqa/upkeep/releases/download/v${VERSION}"

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT

checksum_for() {
  local arch="$1" archive="${workdir}/${2}"
  curl -sL -o "$archive" "${RELEASE_BASE}/upkeep_${VERSION}_darwin_${arch}.tar.gz"
  local sha256 rmd160 size
  sha256=$(shasum -a 256 "$archive" | cut -d' ' -f1)
  rmd160=$(openssl dgst -rmd160 "$archive" | sed 's/^.*= //')
  size=$(wc -c <"$archive" | tr -d ' ')
  echo "${rmd160} ${sha256} ${size}"
}

read -r arm_rmd160 arm_sha256 arm_size < <(checksum_for arm64 arm64.tar.gz)
read -r amd_rmd160 amd_sha256 amd_size < <(checksum_for amd64 amd64.tar.gz)

git clone --depth 1 "https://x-access-token:${MACPORTS_TAP_GITHUB_TOKEN}@github.com/${TAP_REPO}.git" "${workdir}/tap"
cd "${workdir}/tap"

sed -i '' -e "s|^version             .*|version             ${VERSION}|" "$PORTFILE"

awk \
  -v arm_rmd160="$arm_rmd160" -v arm_sha256="$arm_sha256" -v arm_size="$arm_size" \
  -v amd_rmd160="$amd_rmd160" -v amd_sha256="$amd_sha256" -v amd_size="$amd_size" '
  index($0, "eq {arm64}")  { arch = "arm"; print; next }
  index($0, "eq {x86_64}") { arch = "amd"; print; next }
  arch == "arm" && index($0, "rmd160") { print "                    rmd160  " arm_rmd160 " \\"; next }
  arch == "arm" && index($0, "sha256") { print "                    sha256  " arm_sha256 " \\"; next }
  arch == "arm" && index($0, "size")   { print "                    size    " arm_size; arch = ""; next }
  arch == "amd" && index($0, "rmd160") { print "                    rmd160  " amd_rmd160 " \\"; next }
  arch == "amd" && index($0, "sha256") { print "                    sha256  " amd_sha256 " \\"; next }
  arch == "amd" && index($0, "size")   { print "                    size    " amd_size; arch = ""; next }
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
