#!/bin/sh
set -eu

tmp_dir=
install_tmp=

cleanup() {
  if [ -n "${install_tmp-}" ] && [ -e "$install_tmp" ]; then
    rm -f "$install_tmp"
  fi
  if [ -n "${tmp_dir-}" ] && [ -d "$tmp_dir" ]; then
    rm -rf "$tmp_dir"
  fi
}
trap cleanup EXIT
trap 'exit 130' HUP INT TERM

die() {
  echo "tgsend installer: $1" >&2
  exit 1
}

normalize_version() {
  raw_version=${TGSEND_VERSION-latest}
  if [ "$raw_version" = latest ]; then
    release_path=latest/download
    expected_version=
    return
  fi

  case "$raw_version" in
    v*) version_without_v=${raw_version#v} ;;
    *) version_without_v=$raw_version ;;
  esac
  case "$version_without_v" in
    ''|*[!0-9.]*|.*|*.) die "TGSEND_VERSION must be latest or semantic version X.Y.Z" ;;
  esac

  saved_ifs=$IFS
  IFS=.
  # shellcheck disable=SC2086
  set -- $version_without_v
  IFS=$saved_ifs
  [ "$#" -eq 3 ] || die "TGSEND_VERSION must be latest or semantic version X.Y.Z"
  for component in "$@"; do
    case "$component" in
      0|[1-9]|[1-9][0-9]*) ;;
      *) die "TGSEND_VERSION must be latest or semantic version X.Y.Z" ;;
    esac
  done

  release_path=v$version_without_v
  expected_version=$version_without_v
}

select_base_url() {
  if [ "${TGSEND_INSTALL_TEST-}" = 1 ]; then
    base_url=${TGSEND_INSTALL_BASE_URL-}
    [ -n "$base_url" ] || die "TGSEND_INSTALL_BASE_URL is required in test mode"
  else
    base_url=https://github.com/manprint/tgsend/releases
  fi

  case "$base_url" in
    https://*) ;;
    http://*) [ "${TGSEND_INSTALL_TEST-}" = 1 ] || die "release URL must use HTTPS" ;;
    *) die "release URL must use HTTPS" ;;
  esac
  base_url=${base_url%/}
}

download() {
  url=$1
  download_destination=$2
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --retry 2 --connect-timeout 10 --output "$download_destination" "$url" || die "download failed"
  elif command -v wget >/dev/null 2>&1; then
    wget -q --timeout=10 -O "$download_destination" "$url" || die "download failed"
  else
    die "curl or wget is required"
  fi
}

select_platform() {
	if [ "${TGSEND_INSTALL_TEST-}" = 1 ] && [ "${FAKE_UNAME_S+x}" = x ]; then
		os_name=$FAKE_UNAME_S
	else
		os_name=$(uname -s 2>/dev/null) || die "cannot determine operating system"
	fi
	if [ "${TGSEND_INSTALL_TEST-}" = 1 ] && [ "${FAKE_UNAME_M+x}" = x ]; then
		machine=$FAKE_UNAME_M
	else
		machine=$(uname -m 2>/dev/null) || die "cannot determine architecture"
	fi
	case "$os_name:$machine" in
    Linux:x86_64|Linux:amd64) artifact_suffix=linux_amd64 ;;
    Linux:aarch64|Linux:arm64) artifact_suffix=linux_arm64 ;;
    Linux:armv7l|Linux:armv7|Linux:armhf) artifact_suffix=linux_armv7 ;;
    Darwin:x86_64|Darwin:amd64) artifact_suffix=darwin_amd64 ;;
    Darwin:arm64|Darwin:aarch64) artifact_suffix=darwin_arm64 ;;
    *) die "unsupported platform: $os_name/$machine" ;;
  esac
}

verify_checksum() {
  checksum_file=$1
  asset_name=$2
  checksum=$(awk -v name="$asset_name" '$2 == name { if (found++) exit 2; value=$1 } END { if (found != 1) exit 1; print value }' "$checksum_file") || die "checksum entry missing or duplicated"
  case "$checksum" in
    ''|*[!0-9a-fA-F]*) die "invalid checksum" ;;
  esac
  [ "${#checksum}" -eq 64 ] || die "invalid checksum"
  printf '%s  %s\n' "$checksum" "$asset_name" > "$tmp_dir/selected.checksum"
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$tmp_dir" && sha256sum -c selected.checksum >/dev/null) || die "checksum verification failed"
  elif command -v shasum >/dev/null 2>&1; then
    (cd "$tmp_dir" && shasum -a 256 -c selected.checksum >/dev/null) || die "checksum verification failed"
  else
    die "sha256sum or shasum is required"
  fi
}

validate_binary() {
  binary=$1
  [ -x "$binary" ] || die "release archive does not contain an executable tgsend"
  version_json=$("$binary" --version 2>/dev/null) || die "downloaded binary failed version validation"
  case "$version_json" in
    \{*\}) ;;
    *) die "downloaded binary returned invalid version JSON" ;;
  esac
  printf '%s\n' "$version_json" | grep -F '"ok":true' >/dev/null || die "downloaded binary returned an unsuccessful version response"
  if [ -n "$expected_version" ]; then
    printf '%s\n' "$version_json" | grep -F "\"version\":\"$expected_version\"" >/dev/null || die "downloaded binary version does not match requested version"
  fi
}

normalize_version
select_base_url
select_platform

asset_name=tgsend_${artifact_suffix}.tar.gz
archive_url=$base_url/$release_path/$asset_name
checksums_url=$base_url/$release_path/checksums.txt
install_dir=${TGSEND_INSTALL_DIR:-/usr/local/bin}
destination=$install_dir/tgsend

tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/tgsend-install.XXXXXX") || die "cannot create temporary directory"
mkdir -p "$install_dir" || die "cannot create installation directory"
archive_path=$tmp_dir/$asset_name
download "$archive_url" "$archive_path"
download "$checksums_url" "$tmp_dir/checksums.txt"
verify_checksum "$tmp_dir/checksums.txt" "$asset_name"

extract_dir=$tmp_dir/extract
mkdir "$extract_dir"
tar -tzf "$archive_path" | awk '$0 == "tgsend" { found++ } END { exit(found == 1 ? 0 : 1) }' >/dev/null 2>&1 || die "release archive has unexpected or missing members"
tar -xzf "$archive_path" -C "$extract_dir" tgsend >/dev/null 2>&1 || die "cannot extract release binary"
validate_binary "$extract_dir/tgsend"

install_tmp=$(mktemp "$install_dir/.tgsend.tmp.XXXXXX") || die "cannot create atomic install file"
cp "$extract_dir/tgsend" "$install_tmp" || die "cannot stage binary"
chmod 0755 "$install_tmp" || die "cannot set executable mode"
mv -f "$install_tmp" "$destination" || die "cannot install binary"
install_tmp=
echo "tgsend installed to $destination"
