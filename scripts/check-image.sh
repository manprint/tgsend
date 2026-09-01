#!/bin/sh
set -eu

die() {
  echo "image checker: $1" >&2
  exit 1
}

tag=${1-}
[ -n "$tag" ] || die "usage: check-image.sh IMAGE:TAG"
command -v docker >/dev/null 2>&1 || die "docker is required"
command -v jq >/dev/null 2>&1 || die "jq is required"

raw=$(docker buildx imagetools inspect --raw "$tag") || die "cannot inspect image: $tag"
printf '%s\n' "$raw" | jq -e '
  def platform:
    if .os == "linux" and .architecture == "arm" and .variant == "v7" then "linux/arm/v7"
    else (.os + "/" + .architecture)
    end;
  ([.manifests[] | select(.platform != null and .platform.os == "linux") | .platform | platform] | sort) == ["linux/amd64", "linux/arm/v7", "linux/arm64"] and
  .annotations["org.opencontainers.image.licenses"] == "MIT" and
  .annotations["org.opencontainers.image.source"] == "https://github.com/manprint/tgsend" and
  (.annotations["org.opencontainers.image.version"] | type == "string" and length > 0) and
  (.annotations["org.opencontainers.image.revision"] | type == "string" and length > 0)
' >/dev/null || die "image index is missing the exact three platforms or OCI metadata"

echo "image manifest valid: linux/amd64, linux/arm64, linux/arm/v7 with OCI metadata"
