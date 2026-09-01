#!/bin/sh

set -eu

if ! command -v docker >/dev/null 2>&1; then
	printf '%s\n' 'test-container: docker is required' >&2
	exit 1
fi
if ! docker buildx version >/dev/null 2>&1; then
	printf '%s\n' 'test-container: docker buildx is required' >&2
	exit 1
fi

repo_root=$(CDPATH=; cd -- "$(dirname -- "$0")/../.." && pwd)
temp_root=$(mktemp -d "${TMPDIR:-/tmp}/tgsend-container.XXXXXX")
image="tgsend:test-$$"
cleaned=0

cleanup() {
	if [ "$cleaned" -eq 1 ]; then
		return
	fi
	cleaned=1
	docker image rm "$image" >/dev/null 2>&1 || true
	rm -rf "$temp_root"
}
trap cleanup 0 1 2 3 15

mkdir -p "$temp_root/linux/amd64"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$temp_root/linux/amd64/tgsend" ./cmd/tgsend
cp "$repo_root/Dockerfile" "$temp_root/Dockerfile"
cp "$repo_root/.dockerignore" "$temp_root/.dockerignore"

docker buildx build \
	--load \
	--platform linux/amd64 \
	--tag "$image" \
	"$temp_root"

image_user=$(docker image inspect --format '{{.Config.User}}' "$image")
if [ "$image_user" != "65532:65532" ]; then
	printf 'test-container: image user is %s, want 65532:65532\n' "$image_user" >&2
	exit 1
fi
image_platform=$(docker image inspect --format '{{.Os}}/{{.Architecture}}' "$image")
if [ "$image_platform" != "linux/amd64" ]; then
	printf 'test-container: image platform is %s, want linux/amd64\n' "$image_platform" >&2
	exit 1
fi

version_output=$(docker run --rm --platform linux/amd64 "$image" --version)
case "$version_output" in
	*'"schema_version":"1"'*'"ok":true'*'"command":"version"'*) ;;
	*)
		printf 'test-container: unexpected version output: %s\n' "$version_output" >&2
		exit 1
		;;
esac

body_file="$temp_root/body.txt"
awk 'BEGIN { for (i = 0; i < 2500; i++) printf "🚀" }' >"$body_file"
preview=$(docker run --rm --platform linux/amd64 -i "$image" \
	--dry-run --title "Smoke 😀" --type warning --monospace <"$body_file")
case "$preview" in
	*'"dry_run":true'*'"chunks_total":2'*'"chunks_sent":0'*) ;;
	*)
		printf 'test-container: unexpected dry-run output: %s\n' "$preview" >&2
		exit 1
		;;
esac

wrapper_home="$temp_root/wrapper-home"
mkdir -p "$wrapper_home"
native_preview=$(printf '%s' 'wrapper exact 😀' | docker run --rm --platform linux/amd64 -i "$image" --dry-run)
wrapper_preview=$(printf '%s' 'wrapper exact 😀' | HOME="$wrapper_home" TGSEND_IMAGE="$image" "$repo_root/tgsend.sh" --dry-run)
if [ "$wrapper_preview" != "$native_preview" ]; then
	printf '%s\n' 'test-container: wrapper dry-run differs from native image output' >&2
	exit 1
fi

printf '%s\n' 'token = "123:container-wrapper"' >"$wrapper_home/.tgsend"
default_wrapper_preview=$(printf '%s' 'wrapper config' | HOME="$wrapper_home" TGSEND_IMAGE="$image" "$repo_root/tgsend.sh" --dry-run)
if [ "$default_wrapper_preview" != "$(printf '%s' 'wrapper config' | docker run --rm --platform linux/amd64 -i "$image" --dry-run)" ]; then
	printf '%s\n' 'test-container: wrapper default-config path changed native dry-run' >&2
	exit 1
fi

history=$(docker history --no-trunc --format '{{.CreatedBy}}' "$image")
case "$history" in
	*'.tgsend'*|*'sentinel-token'*|*'go.mod'*)
		printf '%s\n' 'test-container: image history contains ignored source or secret material' >&2
		exit 1
		;;
esac

printf '%s\n' 'test-container: image, non-root user, version, dry-run, platform, and secret boundary checks passed'
