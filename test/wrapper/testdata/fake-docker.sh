#!/bin/sh

set -eu

: "${FAKE_DOCKER_ARGS:?FAKE_DOCKER_ARGS is required}"
: "${FAKE_DOCKER_STDIN:?FAKE_DOCKER_STDIN is required}"
: >"$FAKE_DOCKER_ARGS"
for argument do
	printf '%s\0' "$argument" >>"$FAKE_DOCKER_ARGS"
done
cat >"$FAKE_DOCKER_STDIN"
exit "${FAKE_DOCKER_EXIT:-0}"
