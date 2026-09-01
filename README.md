# tgsend

## Overview

`tgsend` is a command-line tool for sending messages to Telegram. This phase
ships exact UTF-8 input handling, strict configuration validation, stable JSON
responses, and an offline dry-run preview with optional formatting and
deterministic chunking.

Live sending is not shipped yet; dry-run remains the credential-free way to
validate a message and inspect every chunk.

## Requirements

- Go 1.27.0
- Make

The first build may download the pinned Go modules and the Go toolchain
required by the local Go installation. Docker is an optional fallback when a
local Go 1.27 toolchain is unavailable.

## Build from source

From the repository root, run:

```sh
make build
```

The executable is written to `bin/tgsend`.

## Usage

```sh
./bin/tgsend [flags]
```

Input can come from `-m, --message` or piped standard input. The two sources
are mutually exclusive; input whitespace and newlines are preserved exactly.

Available flags:

```text
  -c, --config string         configuration file path
      --dry-run               validate and preview without credentials or network
  -h, --help                  help for tgsend
      --max-input-bytes int   maximum input size in bytes (default 1048576)
  -m, --message string        message text (mutually exclusive with stdin)
      --monospace             format each body chunk as preformatted text
      --title string          optional bold title
      --type string            optional type: INFO, WARNING, ERROR, CRITICAL
      --silent                disable Telegram notifications
      --version               print version information as JSON
```

Examples:

```sh
./bin/tgsend --dry-run -m 'Hello from tgsend'
./bin/tgsend --dry-run --type WARNING --title 'Deploy' --monospace \
  -m 'Release started: check the canary before promoting.'
printf 'first line\r\nsecond line\n' | ./bin/tgsend --dry-run
```

## Formatting

`--type` is case-insensitive and accepts exactly `INFO`, `WARNING`, `ERROR`,
or `CRITICAL`. The type is normalized to uppercase and displayed with its
fixed icon: `ℹ️`, `⚠️`, `❌`, or `🚨`, respectively. A title, when supplied,
is displayed on the next line in bold. The type and title header is followed
by one blank line before the body.

The header appears only in the first chunk. A title creates a `bold` entity;
`--monospace` creates a `pre` entity covering the body of each chunk. Chunk
labels or counters are never added, and the body text is otherwise preserved
exactly, including whitespace and line endings.

## Long messages

Messages are split into ordered chunks of at most 4096 UTF-16 code units, the
Telegram text limit. The first chunk includes the optional header and reserves
space for it; later chunks can use the full limit. When possible, a chunk ends
at the last complete newline that fits. No text is trimmed, reordered, or
annotated, and concatenating the body portions reconstructs the original body
byte-for-byte.

## Configuration

The configuration file is TOML with exactly these keys:

```toml
token = "REPLACE_WITH_TELEGRAM_BOT_TOKEN"
chat_id = "@example_channel"
```

The default path is `$HOME/.tgsend`; `--config PATH` selects an explicit file.
An explicitly selected file must exist and be readable. Unknown TOML keys and
wrong value types are rejected. `chat_id` accepts a nonzero signed decimal
integer fitting `int64`, or `@` followed by ASCII letters, digits, or `_`.

## Environment

`TGSEND_TOKEN` and `TGSEND_CHAT_ID` override their corresponding file values
independently when non-empty. If the default file is absent, both variables can
provide the complete configuration. Empty variables do not override a file
value.

The dry-run path intentionally does not read configuration, credentials, or
the network.

## JSON output

`--version` and `--dry-run` write exactly one JSON document followed by a
newline to standard output. Errors write exactly one JSON document to standard
error; standard output remains empty.

Version response:

```json
{"schema_version":"1","ok":true,"command":"version","result":{"version":"dev","commit":"none","date":"unknown"}}
```

Dry-run response:

```json
{"schema_version":"1","ok":true,"command":"send","result":{"dry_run":true,"chunks_total":1,"chunks_sent":0,"message_ids":[],"chunks":[{"index":1,"text":"Hello","entities":[],"disable_notification":false}]}}
```

Representative validation error:

```json
{"schema_version":"1","ok":false,"command":"send","error":{"code":"input_empty","message":"input is empty","retryable":false}}
```

Responses never include tokens or chat IDs.

## Dry run

Dry-run validates the selected input and previews the raw message without
requiring a config file, credentials, or network access. It accepts one raw
message up to the 1 MiB input byte cap, plans all formatted chunks, and returns
their exact text and entities arrays. The `--silent` flag is reflected in
`disable_notification`. Each chunk contains `index`, `text`, `entities`, and
`disable_notification`; entity objects contain `type`, `offset`, and `length`
in UTF-16 code units (and `language` when present).

Example preview for a formatted message (the token and chat ID are not needed
for dry-run and never appear in its output):

```json
{"schema_version":"1","ok":true,"command":"send","result":{"dry_run":true,"chunks_total":1,"chunks_sent":0,"message_ids":[],"chunks":[{"index":1,"text":"⚠️ WARNING\nDeploy\n\nRelease started","entities":[{"type":"bold","offset":11,"length":6},{"type":"pre","offset":19,"length":15}],"disable_notification":false}]}}
```

To validate the examples with an isolated home directory and a JSON parser:

```sh
tmp_home=$(mktemp -d)
trap 'rm -rf "$tmp_home"' EXIT
HOME="$tmp_home" ./bin/tgsend --dry-run -m 'Hello' | python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["ok"] and x["result"]["chunks"][0]["text"] == "Hello"'
HOME="$tmp_home" ./bin/tgsend --dry-run --type WARNING --title Deploy --monospace -m 'Release started' | python3 -c 'import json,sys; x=json.load(sys.stdin); c=x["result"]["chunks"][0]; assert c["text"] == "⚠️ WARNING\nDeploy\n\nRelease started" and [e["type"] for e in c["entities"]] == ["bold","pre"]'
printf 'first line\r\nsecond line\n' | HOME="$tmp_home" ./bin/tgsend --dry-run | python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["result"]["chunks"][0]["text"] == "first line\r\nsecond line\n"'
```

## Exit codes

- `0` — success
- `2` — usage or command-line argument error
- `3` — configuration error
- `4` — input error
- `5` — Telegram API rejection or protocol error
- `6` — transport unavailable or failed
- `7` — Telegram rate limit

## Limits

- Input is limited to 1 MiB by default; change it with `--max-input-bytes`.
- Every output chunk is limited to 4096 UTF-16 code units, including its
  formatting entities.
- The first chunk reserves space for its header and separator; a title that
  would make the header exceed the safe limit is rejected.
- Live sending and Telegram request retries are not available in this phase.

## Troubleshooting

- Use `--help` to check the exact flags and defaults.
- If standard input is a terminal and `-m` is not supplied, provide a message
  with `-m` or pipe input into the command.
- If both `-m` and non-empty standard input are supplied, the command exits
  with code `2` and reports `conflicting_input`.
- If `--type` is not one of the four accepted names, the command exits with
  code `2` and reports `invalid_flag`.
- If the title plus its header exceeds the first-chunk limit, the command
  exits with code `2` and reports `title_too_long`.
- For offline checks, always use `--dry-run`; it does not inspect
  `$HOME/.tgsend`, `--config`, or environment credentials.
