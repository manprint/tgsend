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
  echo "tgsend wrapper installer: $1" >&2
  exit 1
}

normalize_version() {
  raw_version=${TGSEND_VERSION-latest}
  if [ "$raw_version" = latest ]; then
    release_path=latest/download
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

verify_checksum() {
  checksum_file=$1
  checksum=$(awk '$2 == "tgsend.sh" { if (found++) exit 2; value=$1 } END { if (found != 1) exit 1; print value }' "$checksum_file") || die "wrapper checksum entry missing or duplicated"
  case "$checksum" in
    ''|*[!0-9a-fA-F]*) die "invalid wrapper checksum" ;;
  esac
  [ "${#checksum}" -eq 64 ] || die "invalid wrapper checksum"
  printf '%s  tgsend.sh\n' "$checksum" > "$tmp_dir/selected.checksum"
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$tmp_dir" && sha256sum -c selected.checksum >/dev/null) || die "wrapper checksum verification failed"
  elif command -v shasum >/dev/null 2>&1; then
    (cd "$tmp_dir" && shasum -a 256 -c selected.checksum >/dev/null) || die "wrapper checksum verification failed"
  else
    die "sha256sum or shasum is required"
  fi
}

normalize_version
select_base_url
install_dir=${TGSEND_INSTALL_DIR:-/usr/local/bin}
destination=$install_dir/tgsend
tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/tgsend-wrapper-install.XXXXXX") || die "cannot create temporary directory"
mkdir -p "$install_dir" || die "cannot create installation directory"
download "$base_url/$release_path/tgsend.sh" "$tmp_dir/tgsend.sh"
download "$base_url/$release_path/tgsend.sh.sha256" "$tmp_dir/tgsend.sh.sha256"
verify_checksum "$tmp_dir/tgsend.sh.sha256"
sh -n "$tmp_dir/tgsend.sh" || die "downloaded wrapper has invalid shell syntax"

install_tmp=$(mktemp "$install_dir/.tgsend-wrapper.tmp.XXXXXX") || die "cannot create atomic install file"
cp "$tmp_dir/tgsend.sh" "$install_tmp" || die "cannot stage wrapper"
chmod 0755 "$install_tmp" || die "cannot set executable mode"
mv -f "$install_tmp" "$destination" || die "cannot install wrapper"
install_tmp=
echo "tgsend wrapper installed to $destination"
