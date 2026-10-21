#!/bin/sh

set -eu

: "${FAKE_DOCKER_ARGS:?FAKE_DOCKER_ARGS is required}"
: "${FAKE_DOCKER_STDIN:?FAKE_DOCKER_STDIN is required}"
: >"$FAKE_DOCKER_ARGS"
for argument do
	printf '%s\0' "$argument" >>"$FAKE_DOCKER_ARGS"
done
# Like Docker, read stdin only when it is attached with -i (always the third
# argument of the wrapper's "run --rm -i" invocation).
if [ "${3-}" = -i ]; then
	cat >"$FAKE_DOCKER_STDIN"
fi
exit "${FAKE_DOCKER_EXIT:-0}"
