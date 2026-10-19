#!/usr/bin/env bash
# Updates the version and checksums in teknikqa/macports-ports's Portfile
# for a new upkeep release, committing through the GitHub contents API. Run
# after a tag is pushed; MACPORTS_TAP_GITHUB_TOKEN must have contents:write on
# that repo.
set -euo pipefail

VERSION="${1:?usage: update-macports-portfile.sh <version, e.g. 0.11.1>}"
: "${MACPORTS_TAP_GITHUB_TOKEN:?MACPORTS_TAP_GITHUB_TOKEN must be set}"
export GH_TOKEN="$MACPORTS_TAP_GITHUB_TOKEN"

TAP_REPO="teknikqa/macports-ports"
PORTFILE="sysutils/upkeep/Portfile"
CONTENTS_API="repos/${TAP_REPO}/contents/${PORTFILE}"
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

current="${workdir}/Portfile.current"
updated="${workdir}/Portfile"
# The blob sha is required to update the file.
blob_sha=$(gh api "$CONTENTS_API" --jq .sha)
gh api "$CONTENTS_API" --jq .content | base64 --decode >"$current"

sed -e "s|^version             .*|version             ${VERSION}|" "$current" |
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
' >"$updated"

if cmp -s "$current" "$updated"; then
  echo "Portfile already up to date for ${VERSION}"
  exit 0
fi

gh api -X PUT "$CONTENTS_API" \
  -f message="upkeep ${VERSION}" \
  -f sha="$blob_sha" \
  -f content="$(base64 <"$updated" | tr -d '\n')" \
  -f 'committer[name]=github-actions[bot]' \
  -f 'committer[email]=github-actions[bot]@users.noreply.github.com' \
  --jq '"Committed \(.commit.sha) to \(.commit.html_url)"'
