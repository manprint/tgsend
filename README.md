# tgsend

## Overview

`tgsend` is a command-line tool for sending messages to Telegram. The current
release provides the command-line skeleton, build metadata, machine-readable
version output, and textual help.

## Requirements

- Go 1.27.0
- Make

The first build may download the pinned Go modules and toolchain required by
the local Go installation.

## Build from source

From the repository root, run:

```sh
make build
```

The executable is written to `bin/tgsend`.

## Version

Run:

```sh
./bin/tgsend --version
```

The command writes one JSON document to standard output:

```json
{"schema_version":"1","ok":true,"command":"version","result":{"version":"dev","commit":"none","date":"unknown"}}
```

## Help

Run:

```sh
./bin/tgsend --help
```

Help is textual and is written to standard output.

## Current limitations

This release does not send messages and does not read Telegram credentials or
configuration. Message input, formatting, configuration, network transport,
and container usage are not available yet.
