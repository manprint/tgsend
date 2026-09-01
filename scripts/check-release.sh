#!/bin/sh
set -eu

die() {
  echo "release checker: $1" >&2
  exit 1
}

dist=${1:-dist}
project_root=${2:-}
if [ -z "$project_root" ]; then
  project_root=$(CDPATH=; cd -- "$(dirname "$0")/.." && pwd -P) || die "cannot locate project root"
else
  project_root=$(CDPATH=; cd -- "$project_root" && pwd -P) || die "cannot locate project root"
fi
dist=$(CDPATH=; cd -- "$dist" && pwd -P) || die "distribution directory not found"

tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/tgsend-release-check.XXXXXX") || die "cannot create temporary directory"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

expected_archives='tgsend_darwin_amd64.tar.gz
tgsend_darwin_arm64.tar.gz
tgsend_linux_amd64.tar.gz
tgsend_linux_arm64.tar.gz
tgsend_linux_armv7.tar.gz
tgsend_windows_amd64.zip
tgsend_windows_arm64.zip'

actual_archives=$(
  for path in "$dist"/tgsend_*.tar.gz "$dist"/tgsend_*.zip; do
    if [ -f "$path" ]; then
      basename "$path"
    fi
  done | sort
)
if [ "$actual_archives" != "$(printf '%s\n' "$expected_archives" | sort)" ]; then
  die "archive set does not match the seven supported targets"
fi

for archive in $expected_archives; do
  [ -f "$dist/$archive" ] || die "missing archive: $archive"
done

checksums=$dist/checksums.txt
[ -f "$checksums" ] || die "missing checksums.txt"
awk '
  NF != 2 || length($1) != 64 || $1 !~ /^[0-9A-Fa-f]+$/ || $2 == "" { exit 1 }
  seen[$2]++ { exit 2 }
  { print $2 }
' "$checksums" > "$tmp_dir/checksum-assets" || die "malformed or duplicate checksum entry"
require_one() {
  pattern=$1
  label=$2
  count=0
  for path in $pattern; do
    if [ -f "$path" ]; then
      count=$((count + 1))
    fi
  done
  [ "$count" -eq 1 ] || die "$label SBOM count is $count, expected one"
}

for archive in $expected_archives; do
  [ -f "$dist/$archive.sbom.json" ] || die "missing archive SBOM: $archive.sbom.json"
done
for binary_suffix in darwin_amd64 darwin_arm64 linux_amd64 linux_arm64 linux_arm windows_amd64 windows_arm64; do
  require_one "$dist"/tgsend_*_"$binary_suffix".sbom.json "binary $binary_suffix"
done
sbom_count=$(for path in "$dist"/*.sbom.json; do [ -f "$path" ] && echo "$path"; done | wc -l | tr -d ' ')
[ "$sbom_count" -eq 14 ] || die "expected 14 SBOM files, found $sbom_count"

checksum_count=$(wc -l < "$tmp_dir/checksum-assets" | tr -d ' ')
[ "$checksum_count" -eq 21 ] || die "expected 21 checksum entries, found $checksum_count"
while IFS= read -r asset; do
  case "$asset" in
    */*|..*) die "checksum references unsafe asset: $asset" ;;
  esac
  [ -f "$dist/$asset" ] || die "checksum references missing asset: $asset"
done < "$tmp_dir/checksum-assets"
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$dist" && sha256sum -c checksums.txt >/dev/null) || die "checksum verification failed"
elif command -v shasum >/dev/null 2>&1; then
  (cd "$dist" && shasum -a 256 -c checksums.txt >/dev/null) || die "checksum verification failed"
else
  die "sha256sum or shasum is required"
fi

check_members() {
  archive=$1
  members=$tmp_dir/$(basename "$archive").members
  case "$archive" in
    *.tar.gz)
      expected_members='LICENSE
README.md
tgsend'
      tar -tzf "$archive" > "$members" || die "cannot list archive: $(basename "$archive")"
      ;;
    *.zip)
      expected_members='LICENSE
README.md
tgsend.exe'
      unzip -Z1 "$archive" > "$members" || die "cannot list archive: $(basename "$archive")"
      ;;
    *) die "unsupported archive format: $archive" ;;
  esac
  awk '
    /^\// || /(^|\/)\.\.($|\/)/ { exit 1 }
    seen[$0]++ { exit 2 }
  ' "$members" || die "archive contains unsafe or duplicate members: $(basename "$archive")"
  actual=$(sort "$members")
  if [ "$actual" != "$(printf '%s\n' "$expected_members" | sort)" ]; then
    die "archive members do not match the expected contract: $(basename "$archive")"
  fi
}

for archive in $expected_archives; do
  check_members "$dist/$archive"
done

native_dir=$tmp_dir/native
mkdir "$native_dir"
tar -xzf "$dist/tgsend_linux_amd64.tar.gz" -C "$native_dir" tgsend || die "cannot extract native archive"
chmod 0755 "$native_dir/tgsend"
version_json=$("$native_dir/tgsend" --version 2>/dev/null) || die "native release binary failed --version"
printf '%s\n' "$version_json" | grep -F '"ok":true' >/dev/null || die "native release binary returned invalid version JSON"

installer=$project_root/scripts/install.sh
[ -f "$installer" ] || die "binary installer is missing"
asset_pattern="asset_name=tgsend_\${artifact_suffix}.tar.gz"
grep -F "$asset_pattern" "$installer" >/dev/null || die "installer asset naming is not tied to release names"
for mapping in \
  'Linux:x86_64|Linux:amd64) artifact_suffix=linux_amd64' \
  'Linux:aarch64|Linux:arm64) artifact_suffix=linux_arm64' \
  'Linux:armv7l|Linux:armv7|Linux:armhf) artifact_suffix=linux_armv7' \
  'Darwin:x86_64|Darwin:amd64) artifact_suffix=darwin_amd64' \
  'Darwin:arm64|Darwin:aarch64) artifact_suffix=darwin_arm64'; do
  grep -F "$mapping" "$installer" >/dev/null || die "installer platform mapping is incomplete: $mapping"
done

wrapper=$project_root/tgsend.sh
wrapper_checksum=$project_root/tgsend.sh.sha256
[ -f "$wrapper" ] || die "wrapper asset is missing"
[ -f "$wrapper_checksum" ] || die "generated wrapper checksum is missing"
awk 'NF != 2 || length($1) != 64 || $1 !~ /^[0-9A-Fa-f]+$/ || $2 != "tgsend.sh" || seen++ { exit 1 } END { exit(seen == 1 ? 0 : 1) }' "$wrapper_checksum" || die "wrapper checksum manifest is invalid"
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$project_root" && sha256sum -c tgsend.sh.sha256 >/dev/null) || die "wrapper checksum verification failed"
else
  (cd "$project_root" && shasum -a 256 -c tgsend.sh.sha256 >/dev/null) || die "wrapper checksum verification failed"
fi

scan_release_asset() {
  path=$1
  if grep -a -F -e '.tgsend' -e 'fixture-secret-token' "$path" >/dev/null 2>&1; then
    die "release asset contains forbidden local configuration or secret sentinel: $(basename "$path")"
  fi
}
for archive in $expected_archives; do
  scan_release_asset "$dist/$archive"
done
for sbom in "$dist"/*.sbom.json; do
  [ -f "$sbom" ] && scan_release_asset "$sbom"
done

echo "release artifacts valid: seven archives, 21 checksums, 14 SBOMs, safe members, native version, and installer names"
