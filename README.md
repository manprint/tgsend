# tgsend

## Overview

`tgsend` is a command-line tool for sending messages to Telegram. This phase
ships exact UTF-8 input handling, strict configuration validation, stable JSON
responses, and an offline dry-run preview for one raw message.

Live sending and message formatting are not shipped yet.

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
      --silent                disable Telegram notifications
      --version               print version information as JSON
```

Examples:

```sh
./bin/tgsend --dry-run -m 'Hello from tgsend'
printf 'first line\r\nsecond line\n' | ./bin/tgsend --dry-run
```

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
message up to 4096 UTF-16 code units and returns its text and entities array.
The `--silent` flag is reflected in `disable_notification`.

To validate the examples with an isolated home directory and a JSON parser:

```sh
tmp_home=$(mktemp -d)
HOME="$tmp_home" ./bin/tgsend --dry-run -m 'Hello' | python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["ok"] and x["result"]["chunks"][0]["text"] == "Hello"'
printf 'first line\r\nsecond line\n' | HOME="$tmp_home" ./bin/tgsend --dry-run | python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["result"]["chunks"][0]["text"] == "first line\r\nsecond line\n"'
rm -r "$tmp_home"
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
- A raw planned message must fit within 4096 UTF-16 code units.
- Formatting, splitting, live sending, and Telegram request retries are not
  available in this phase.

## Troubleshooting

- Use `--help` to check the exact flags and defaults.
- If standard input is a terminal and `-m` is not supplied, provide a message
  with `-m` or pipe input into the command.
- If both `-m` and non-empty standard input are supplied, the command exits
  with code `2` and reports `conflicting_input`.
- For offline checks, always use `--dry-run`; it does not inspect
  `$HOME/.tgsend`, `--config`, or environment credentials.
