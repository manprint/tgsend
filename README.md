# tgsend

`tgsend` is a command-line tool that sends exact UTF-8 messages to a Telegram
chat or channel. It reads a message from `--message` or standard input, checks
the complete message locally, formats it when requested, splits long messages
at Telegram's UTF-16 limit, and sends the resulting chunks in order.

## Requirements

- Go 1.27.0
- Make
- Docker with Buildx (only for the container and wrapper modes)

The first Go command may download modules and the Go 1.27 toolchain selected by
the module. If Go 1.27 is not installed locally, use a Go toolchain that can
download it automatically or run the build in a Go 1.27 Docker image.

## Installation from source

From the repository root:

```sh
make build
```

The executable is written to `bin/tgsend`.

## Run with Docker

The production image contains only the static `tgsend` binary and its CA
certificates. It runs as UID/GID `65532:65532`, has no shell or package
manager, and does not contain a configuration file or credentials.

Build a local image from source for the host platform shown below:

```sh
image_context=$(mktemp -d)
mkdir -p "$image_context/linux/amd64"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$image_context/linux/amd64/tgsend" ./cmd/tgsend
cp Dockerfile .dockerignore "$image_context/"
docker buildx build --load --platform linux/amd64 -t tgsend:local "$image_context"
rm -rf "$image_context"
```

Run with the default configuration file. The mount is read-only and uses the
same absolute path inside the container:

```sh
printf 'Hello\n' | docker run --rm -i \
  --user "$(id -u):$(id -g)" \
  --env HOME="$HOME" \
  --mount "type=bind,src=$HOME/.tgsend,dst=$HOME/.tgsend,readonly" \
  tgsend:local
```

For environment-only configuration, export both variables and pass their
names to Docker so their values are not written in the command arguments:

```sh
export TGSEND_TOKEN='REPLACE_WITH_TELEGRAM_BOT_TOKEN'
export TGSEND_CHAT_ID='@example_channel'
printf 'Hello\n' | docker run --rm -i \
  --user "$(id -u):$(id -g)" \
  --env HOME="$HOME" --env TGSEND_TOKEN --env TGSEND_CHAT_ID \
  tgsend:local
```

The image entrypoint is already `tgsend`, so flags follow the image name:

```sh
docker run --rm -i --env HOME="$HOME" tgsend:local --dry-run -m 'Hello'
```

The Dockerfile expects a prebuilt platform directory because compilation is
intentionally performed outside the image. `make test-container` runs an
isolated build and smoke test for the image and cleans up its temporary image.

## Docker wrapper

`tgsend.sh` is a Docker-only POSIX launcher for Linux and macOS. Install it
from a repository checkout and select the image with `TGSEND_IMAGE`:

```sh
mkdir -p "$HOME/.local/bin"
cp tgsend.sh "$HOME/.local/bin/tgsend"
chmod 755 "$HOME/.local/bin/tgsend"
printf 'Hello\n' | env TGSEND_IMAGE=tgsend:local "$HOME/.local/bin/tgsend"
```

The wrapper forwards every original flag and stdin byte-for-byte, runs Docker
with `--rm -i`, the caller's UID/GID and working directory, and mounts only an
existing default or explicitly selected config file read-only. It supports
`-c PATH`, `-cPATH`, `--config PATH`, and `--config=PATH`. With no config file,
set `TGSEND_TOKEN` and `TGSEND_CHAT_ID`; the wrapper passes only those variable
names to Docker. Container exit codes are returned unchanged.

The wrapper requires Docker and a readable config file when one is selected.
It does not mount the current directory, the Docker socket, or a config
parent directory. It is not a Windows/PowerShell wrapper; use the native
binary or invoke Docker directly on Windows.

## Usage

```sh
./bin/tgsend [flags]
```

Input can come from `-m, --message` or piped standard input. The sources are
mutually exclusive, and input whitespace and line endings are preserved.

The seven basic workflows are:

1. Use the default `$HOME/.tgsend` configuration with standard input:

   ```sh
   printf 'Hello\n' | ./bin/tgsend
   ```

2. Select a configuration file explicitly:

   ```sh
   printf 'Hello\n' | ./bin/tgsend --config /path/to/tgsend.toml
   ```

3. Send a log file:

   ```sh
   cat log.txt | ./bin/tgsend
   ```

4. Send a message supplied as a flag:

   ```sh
   ./bin/tgsend --config /path/to/tgsend.toml --message 'Hello'
   ```

5. Send message body chunks as preformatted monospace text:

   ```sh
   ./bin/tgsend --config /path/to/tgsend.toml --message 'Hello' --monospace
   ```

6. Add a severity type (`INFO`, `WARNING`, `ERROR`, or `CRITICAL`):

   ```sh
   printf 'Hello\n' | ./bin/tgsend --config /path/to/tgsend.toml --type WARNING
   ```

7. Add a bold title:

   ```sh
   printf 'Hello\n' | ./bin/tgsend --title 'My message title'
   ```

Disable Telegram notifications with `--silent`:

```sh
./bin/tgsend --config /path/to/tgsend.toml --message 'Quiet update' --silent
```

Validate input and inspect the exact planned chunks without reading
configuration or contacting Telegram with `--dry-run`:

```sh
./bin/tgsend --dry-run --type INFO --title 'Planned update' --monospace -m 'Hello'
```

Use `--version` for machine-readable build metadata and `--help` for the full
flag list.

## Flags

```text
  -c, --config string         configuration file path
      --dry-run               validate and preview without credentials or network
  -h, --help                  help for tgsend
      --max-input-bytes int   maximum input size in bytes (default 1048576)
  -m, --message string        message text (mutually exclusive with stdin)
      --monospace             format each body chunk as preformatted text
      --silent                disable Telegram notifications
      --title string           optional bold title
      --type string            optional type: INFO, WARNING, ERROR, or CRITICAL
      --version               print version information as JSON
```

## Configuration

The configuration file is TOML with exactly these keys:

```toml
token = "REPLACE_WITH_TELEGRAM_BOT_TOKEN"
chat_id = "@example_channel"
```

The default path is `$HOME/.tgsend`; `--config PATH` selects an explicit file.
An explicit file must exist and be readable. Unknown TOML keys and wrong value
types are rejected. `chat_id` accepts a nonzero signed decimal integer fitting
`int64`, or `@` followed by ASCII letters, digits, or `_`.

## Environment

`TGSEND_TOKEN` and `TGSEND_CHAT_ID` override their corresponding file values
independently when non-empty. If the default file is absent, both variables can
provide the complete configuration. Empty variables do not override a file
value.

When using `tgsend.sh`, `TGSEND_IMAGE` selects the Docker image and defaults to
`ghcr.io/manprint/tgsend:latest`. `TGSEND_TOKEN` and `TGSEND_CHAT_ID` are
forwarded to the container by name; their values are never added to Docker's
argument list.

Keep credentials in the environment or configuration file, never in command
arguments. A configuration file should be readable only by its owner:

```sh
chmod 600 "$HOME/.tgsend"
```

## Image selection

Direct Docker use defaults to the image tag you provide after the build. The
wrapper defaults to `ghcr.io/manprint/tgsend:latest`; pin it to a local or
approved tag with `TGSEND_IMAGE`:

```sh
env TGSEND_IMAGE=tgsend:local "$HOME/.local/bin/tgsend" --dry-run -m 'Hello'
```

The wrapper does not download or compile a host binary. Docker must be able to
pull or already have the selected image, and the invoking user must have
permission to access the Docker daemon.

## Formatting

`--type` is case-insensitive and accepts exactly `INFO`, `WARNING`, `ERROR`, or
`CRITICAL`. The normalized header uses the fixed icons `ℹ️`, `⚠️`, `❌`, and
`🚨`. A title is placed on the next line in bold, followed by one blank line.
The header appears only in the first chunk.

`--monospace` adds a `pre` entity covering only the body of every chunk. Title
and body entity offsets and lengths are UTF-16 code units, as required by
Telegram. No chunk labels or counters are added.

## Long messages

Messages are split into ordered chunks of at most 4096 UTF-16 code units. The
first chunk reserves space for its optional header; later chunks can use the
full limit. When possible, a chunk ends after the last complete newline that
fits. No text is trimmed, reordered, or annotated, and concatenating the body
portions reconstructs the original input byte-for-byte.

## JSON output

Every successful send, dry-run, version response, and error is exactly one JSON
document followed by a newline. Successful output goes to standard output;
errors go to standard error and standard output remains empty.

Successful send:

```json
{"schema_version":"1","ok":true,"command":"send","result":{"dry_run":false,"chunks_total":1,"chunks_sent":1,"message_ids":[42]}}
```

Dry-run preview:

```json
{"schema_version":"1","ok":true,"command":"send","result":{"dry_run":true,"chunks_total":1,"chunks_sent":0,"message_ids":[],"chunks":[{"index":1,"text":"Hello","entities":[],"disable_notification":false}]}}
```

Partial failure:

```json
{"schema_version":"1","ok":false,"command":"send","error":{"code":"telegram_rejected","message":"Telegram API rejected request (code 400)","retryable":false,"progress":{"chunks_total":2,"chunks_sent":1,"failed_chunk":2}}}
```

`--version` returns `version`, `commit`, and `date`. Responses never include
the bot token or chat ID, and successful sends do not echo message bodies.

## Exit codes

- `0` — success
- `2` — usage or command-line argument error
- `3` — configuration error
- `4` — input error
- `5` — Telegram API rejection
- `6` — transport or Telegram protocol failure
- `7` — Telegram rate limit after the retry policy is exhausted

## Retries and failures

The sender performs one request at a time and stops at the first failed chunk.
Each HTTP attempt has a 10-second timeout. Only an explicit HTTP 429 or a
Telegram response with `error_code: 429` and a positive `retry_after` is
retried. There are at most two retries after the initial request, and the
cumulative requested wait is limited to 60 seconds.

Transport errors, timeouts, HTTP 5xx responses, malformed responses, and other
Telegram API errors are not retried because the outcome is not safely known.
When a chunk fails, the error JSON reports the total planned chunks, completed
chunks, and one-based failed chunk; later chunks are not sent.

## Dry run

`--dry-run` validates the selected input, plans every formatted chunk, and
returns exact text and entities. It does not require a configuration file,
credentials, or network access. The input byte limit is still enforced.

For an isolated local check:

```sh
tmp_home=$(mktemp -d)
trap 'rm -rf "$tmp_home"' EXIT
HOME="$tmp_home" ./bin/tgsend --dry-run -m 'Hello' \
  | python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["ok"] and x["result"]["chunks"][0]["text"] == "Hello"'
HOME="$tmp_home" ./bin/tgsend --dry-run --type WARNING --title Deploy --monospace -m 'Release started' \
  | python3 -c 'import json,sys; x=json.load(sys.stdin); c=x["result"]["chunks"][0]; assert c["text"] == "⚠️ WARNING\nDeploy\n\nRelease started" and [e["type"] for e in c["entities"]] == ["bold","pre"]'
printf 'first line\r\nsecond line\n' | HOME="$tmp_home" ./bin/tgsend --dry-run \
  | python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["result"]["chunks"][0]["text"] == "first line\r\nsecond line\n"'
```

The same offline check through the local image is:

```sh
printf 'Hello' | env TGSEND_IMAGE=tgsend:local "$HOME/.local/bin/tgsend" --dry-run \
  | python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["ok"] and x["result"]["chunks"][0]["text"] == "Hello"'
```

## Security

Do not pass the bot token as a command-line argument. Keep the configuration
file private, avoid shell tracing while credentials are set, and do not paste
credentials into issue reports or logs. `tgsend` does not print credentials or
chat IDs in JSON responses and never sends a dry-run request.

The image contains no credentials, config, source tree, compiler, shell, or
package manager. The wrapper passes secret values through Docker's environment
handling rather than its argument vector and mounts only the selected config
file read-only. Do not use `-v "$PWD:/..."` or mount the Docker socket.

No real Telegram credentials are needed by the automated test suite. A live
smoke test is optional and must be performed manually by the operator.

## Limits

- Input is limited to 1 MiB by default; change it with `--max-input-bytes`.
- Every sent chunk is limited to 4096 UTF-16 code units, including its header.
- The first chunk reserves space for the header and separator.
- A title/header that cannot fit is rejected before any request is made.
- Only complete, valid UTF-8 input is accepted.
- Docker mode additionally requires a Linux/amd64 image build for the example
  above; build another platform directory when targeting a different platform.

## Manual Telegram smoke test

Build the binary, set credentials only in the current shell without printing
them, run a dry-run first, then send a short message. Do not use this procedure
in CI:

```sh
read -r -s -p 'Telegram bot token: ' TGSEND_TOKEN; printf '\n'
read -r -p 'Telegram chat ID: ' TGSEND_CHAT_ID
export TGSEND_TOKEN TGSEND_CHAT_ID
./bin/tgsend --dry-run -m 'tgsend smoke test'
./bin/tgsend -m 'tgsend smoke test'
unset TGSEND_TOKEN TGSEND_CHAT_ID
```

Use a test chat or channel and revoke the token if it was exposed. The command
must produce one success JSON document on standard output; no token should
appear in either stream.

## Troubleshooting

- If standard input is a terminal and `--message` is not supplied, provide a
  message with `-m` or pipe input into the command.
- If `-m` and non-empty standard input are both supplied, the command exits 2
  with `conflicting_input`.
- If configuration is incomplete or an explicit file is missing, inspect the
  selected path and the two environment variables; the token itself is never
  included in the error.
- If Docker reports a missing source path during image build, ensure the build
  context contains `linux/amd64/tgsend` and that the command uses the root
  `Dockerfile`.
- If the wrapper says Docker is unavailable, install Docker/Buildx or grant
  the current user access to the Docker daemon. The wrapper has no host-binary
  fallback.
- If a container cannot read configuration, check that the host path is a
  regular readable file and that the path is mounted read-only at the same
  absolute path inside the container.
- On Windows, use the native executable or direct Docker commands; the POSIX
  wrapper is supported on Linux and macOS only.
- If `--type` is not one of the four accepted names, the command exits 2 with
  `invalid_flag`.
- If the title/header exceeds the first-chunk limit, the command exits 2 with
  `title_too_long`.
- For offline validation, use `--dry-run`; it does not inspect the config file
  or credentials.
- For an HTTP 429, inspect the final `telegram_rate_limited` error and its
  retryable status. Other failures are intentionally not retried.
