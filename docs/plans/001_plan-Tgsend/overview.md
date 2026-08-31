# Tgsend - Plan Overview

> **Status:** planning | **Authored:** 2026-09-01 by `agent-1:opus`
> **Folder:** `docs/plans/001_plan-Tgsend/`
> **Executing this plan? Read [STATE.md](STATE.md) FIRST.** It is the only execution-state file. Open a unit there before editing and close it only after its gates pass.

## Goal

Build `tgsend`, a Go CLI that reads an exact UTF-8 message from either `-m` or stdin, composes optional Telegram formatting, splits it safely, and sends it to one configured chat. Ship deterministic JSON, exhaustive tests, static cross-platform binaries, a minimal multi-platform GHCR image, a POSIX Docker wrapper, verified installers, and tag-driven releases.

```text
Given valid environment credentials and a UTF-8 body longer than one Telegram message,
when the compiled CLI receives it on stdin with --type WARNING --title "Deploy" --monospace,
then the fake Telegram endpoint receives ordered <=4096-UTF-16-unit sendMessage requests,
only request 1 has the header, every body segment has a correct pre entity, concatenating
body segments reproduces the input byte-for-byte, and stdout is one schema_version=1 success JSON document.
```

## Design decisions

| # | Decision | Consequence |
|---|----------|-------------|
| **D1** | Module `github.com/manprint/tgsend`, Go `1.27.0`. | `go.mod`, CI, and release builds use the same toolchain. |
| **D2 (user, Q1)** | Use Cobra `v1.10.2`. | `cmd/tgsend` stays thin; Cobra parses flags/help while the application service owns behavior. |
| **D3 (user, Q2)** | Use `github.com/pelletier/go-toml/v2` `v2.4.3`. | Strict TOML decoding rejects unknown keys and type mismatches. |
| **D4** | Config keys are `token` and `chat_id`; `TGSEND_TOKEN` and `TGSEND_CHAT_ID` override each key independently. | A missing default `~/.tgsend` is allowed only if the merged environment is complete; an explicitly named missing `-c` file always fails. |
| **D5** | Exactly one non-empty source: `-m` or stdin. No positional arguments. | Non-empty stdin plus `-m` is a usage error; empty/TTY stdin without `-m` is an input error; no content is trimmed. |
| **D6** | Require valid UTF-8 and enforce `--max-input-bytes` before composition or any network call. | Default is 1,048,576 bytes; values <=0, overflow, empty input, invalid UTF-8, and oversized input fail before sending. |
| **D7** | Use explicit Telegram `MessageEntity` values, never parse modes. | The original body needs no HTML/Markdown escaping; title uses `bold`, each monospace body segment uses `pre`, and offsets/lengths are UTF-16 units. |
| **D8** | Types are case-insensitive `INFO`, `WARNING`, `ERROR`, `CRITICAL`, normalized uppercase. | Prefixes are respectively `U+2139 U+FE0F`, `U+26A0 U+FE0F`, `U+274C`, `U+1F6A8`; any other value fails locally. |
| **D9** | Header is first chunk only; split after the last fitting newline, otherwise at the largest valid rune boundary. | No content or newline is discarded, reordered, duplicated, or synthesized within the body. First-chunk body capacity accounts for header UTF-16 units. |
| **D10** | Classic Bot API `sendMessage`; 10-second HTTP timeout. | Requests use JSON and include `chat_id`, `text`, optional `entities`, and `disable_notification`; tokens never enter errors, logs, or output. |
| **D11 (user, Q3)** | Retry only HTTP/API 429, at most 2 retries and 60 seconds cumulative waiting. | Honor positive `retry_after`; do not retry transport, timeout, 5xx, malformed, or other API failures because Telegram has no idempotency key. |
| **D12 (user, Q4)** | JSON for send and version; Cobra help remains text. | Exactly one newline-terminated JSON document goes to stdout on success or stderr on failure; no usage text contaminates errors. |
| **D13 (user, Q5)** | Do not add chunk counters to Telegram text. | Sent text contains only the specified header/separator and original body. |
| **D14 (user, Q6)** | The shell wrapper is Docker-only. | `tgsend.sh` forwards args/stdin/env, mounts only the chosen config read-only, preserves exit status, and never falls back to a host binary. |
| **D15 (user, Q7)** | Install latest stable by default; support `TGSEND_VERSION`. | Binary and wrapper installers resolve/pin a release, download matching assets, and verify SHA-256 before installation. |
| **D16 (user, Q8)** | CI never sends to real Telegram. | All automated API tests use `httptest`; a credential-gated manual smoke procedure is documented but not scheduled. |
| **D17 (user, Q9)** | Gates are gofmt, go vet, golangci-lint, govulncheck, tests, and release checks. | Tool versions are pinned; `make verify` is the deterministic non-Docker aggregate gate. |
| **D18 (user, Q10)** | MIT license, copyright holder `manprint`. | Release archives, image metadata, and repository contain consistent license data. |
| **D19** | Use internal packages for config, input, message planning, Telegram transport, JSON presentation, and orchestration. | Side effects are injected; package APIs remain small and cannot become external compatibility commitments. |
| **D20** | `--dry-run` is fully offline and skips config loading. | It works without token/chat ID and returns the exact planned request chunks without exposing credentials. |
| **D21** | Exit codes: 0 success; 2 CLI/flag conflict; 3 config; 4 input; 5 Telegram rejection; 6 transport/protocol; 7 rate-limit exhausted. | Every error has one stable symbolic JSON code and one stable process category; partial send counts are reported. |
| **D22** | Release targets: Linux amd64/arm64/armv7, macOS amd64/arm64, Windows amd64/arm64; GHCR Linux amd64/arm64/armv7. | GoReleaser builds static binaries, checksums and SBOMs; `dockers_v2` builds one multi-platform image. |

## Open questions

None - all clarification questions were resolved before plan files were written.

## Architecture summary

Cobra maps CLI syntax into `app.Options`; `app.Run` acquires and validates input, skips or resolves config, asks `message.Planner` for immutable chunks, and either previews them or sends them serially through `telegram.Client`. `presenter` alone emits the versioned JSON envelope. All I/O, HTTP endpoint/client, sleeper, and build metadata are injectable.

## Interface

| Surface | Name | Type / values | Default | Notes |
|---------|------|---------------|---------|-------|
| Flag | `-m`, `--message` | string | unset | Conflicts with non-empty stdin; exact bytes after CLI decoding. |
| Flag | `-c`, `--config` | path | `~/.tgsend` | Explicit missing/unreadable path is config error; ignored by dry-run. |
| Flag | `--title` | valid UTF-8 string | unset | Empty is equivalent to unset; bold line before body. |
| Flag | `--monospace` | bool | `false` | Adds one `pre` entity around each chunk's body only. |
| Flag | `--type` | INFO/WARNING/ERROR/CRITICAL, case-insensitive | unset | Adds normalized type line only to chunk 1. |
| Flag | `--silent` | bool | `false` | Maps to Telegram `disable_notification`. |
| Flag | `--dry-run` | bool | `false` | No config or network; JSON includes exact chunk requests. |
| Flag | `--max-input-bytes` | positive int64 | `1048576` | Limits source bytes before send planning. |
| Flag | `--version` | bool | `false` | Emits JSON build metadata and exits without config/input. |
| Config | `token` | non-empty string | none | Required after merge unless dry-run/version/help. |
| Config | `chat_id` | integer or string | none | Normalize to JSON string; accept signed decimal ID or `@username`. |
| Environment | `TGSEND_TOKEN` | string | unset | Non-empty value overrides file token. |
| Environment | `TGSEND_CHAT_ID` | string | unset | Non-empty value overrides file chat ID. |

## Protocol and data-structure changes

| Change | Shape | Backward-compat strategy |
|--------|-------|--------------------------|
| TOML config | Object with only `token`, `chat_id`; `chat_id` accepts TOML integer or string. | First release; strict errors prevent silently ignored configuration. |
| JSON envelope | `schema_version:"1"`, `ok`, `command`, and exactly one of `result` or `error`; optional send progress in errors. | Field names and exit categories are frozen for v1; additions must be optional. |
| Dry-run chunk | `index`, `text`, `entities[{type,offset,length}]`, `disable_notification`; no token/chat ID. | Same planner object feeds preview and transport, preventing preview/send drift. |
| Telegram request | Bot API JSON `chat_id`, `text`, optional `entities`, `disable_notification`. | No parse mode; classic `sendMessage` maximizes Bot API compatibility. |

## Phases

| Phase | File | Primary assignment | Shippable alone? |
|-------|------|--------------------|------------------|
| 0 - Foundation and contracts | [phase_01.md](phase_01.md) | `agent-3:haiku` | yes, buildable CLI skeleton |
| 1 - Input, config, JSON, and offline CLI | [phase_02.md](phase_02.md) | `agent-2:sonnet` | yes, validated dry-run CLI |
| 2 - Message composition and chunking | [phase_03.md](phase_03.md) | `agent-2:sonnet` | yes, exact request planner |
| 3 - Telegram transport and send orchestration | [phase_04.md](phase_04.md) | `agent-2:sonnet` | yes, complete native sender |
| 4 - Binary e2e, image, and Docker wrapper | [phase_05.md](phase_05.md) | `agent-2:sonnet` | yes, native and container UX |
| 5 - Release automation, installers, and final docs | [phase_06.md](phase_06.md) | `agent-2:sonnet` | yes, releasable product |

Live status is only in `STATE.md` section 11.

## Reuse map (top candidates)

| Need | Reuse | Location |
|------|-------|----------|
| Product scenarios | Existing seven CLI examples | `SPECS.md:5` |
| Quality requirements | Existing exhaustive-test/modularity requirements | `SPECS.md:25` |
| Delivery contract | Existing workflow/binary/image/wrapper requirements | `SPECS.md:31` |
| Documentation contract | Existing README, agentic, curl, and remote requirements | `SPECS.md:38` |
| Secret exclusion | Existing ignored local config | `.gitignore:1` |
| Existing implementation | none - repository has no production code or test harness | `SPECS.md:1` |

## References

| # | What it settled | Source | Version / date |
|---|-----------------|--------|----------------|
| R1 | `sendMessage` fields, 4096-character limit, response envelope, and `retry_after`. | https://core.telegram.org/bots/api#sendmessage | Bot API, checked 2026-09-01 |
| R2 | Entity offsets and lengths are UTF-16 code units. | https://core.telegram.org/bots/api#messageentity | Bot API, checked 2026-09-01 |
| R3 | Cobra dependency version. | https://github.com/spf13/cobra/releases/tag/v1.10.2 | v1.10.2 |
| R4 | TOML dependency version. | https://github.com/pelletier/go-toml/releases/tag/v2.4.3 | v2.4.3 |
| R5 | GoReleaser build targets and current release. | https://goreleaser.com/customization/builds/go/ | v2.18.0, checked 2026-09-01 |
| R6 | `dockers_v2` reuses binaries, uses buildx, and builds/pushes multi-platform images. | https://goreleaser.com/customization/package/dockers_v2/ | v2.18.0 docs |
| R7 | GHCR workflow permissions and `GITHUB_TOKEN` authentication. | https://docs.github.com/actions/publishing-packages/publishing-docker-images | checked 2026-09-01 |
| R8 | golangci-lint tool version. | https://github.com/golangci/golangci-lint/releases/tag/v2.13.2 | v2.13.2 |
| R9 | govulncheck tool version. | https://github.com/golang/vuln/releases/tag/v1.1.4 | v1.1.4 |
| R10 | Go toolchain release contract. | https://go.dev/doc/devel/release | Go 1.27.0, checked 2026-09-01 |
| R11 | Syft version used to generate release SBOMs. | https://github.com/anchore/syft/releases/tag/v1.51.1 | v1.51.1 |
| R12 | Exact CI action release tags. | https://github.com/actions/checkout/releases/tag/v7.0.1 | checkout v7.0.1; companion action releases checked 2026-09-01 |
| R13 | Static non-root runtime image with CA certificates. | https://github.com/GoogleContainerTools/distroless/tree/main/base | `gcr.io/distroless/static-debian12:nonroot`, checked 2026-09-01 |

## Invariants

- **I-1:** The concatenation of planned body slices is byte-for-byte identical to input.
- **I-2:** Every request text is valid UTF-8 and at most 4096 UTF-16 code units; every entity is in bounds and on UTF-16 boundaries.
- **I-3:** Validation completes for the entire input and plan before request 1 is sent.
- **I-4:** At most one chunk is in flight; successful chunks are never resent after a later permanent failure.
- **I-5:** Token values never appear in JSON, logs, test failures, command lines, image layers, release assets, or error strings.
- **I-6:** Dry-run performs no config file read, DNS lookup, HTTP call, or sleep.
- **I-7:** Success is stdout-only; failure is stderr-only; each stream receives at most one JSON document except textual help.

## Risk register

| Risk | Mitigation |
|------|------------|
| Unicode count differs from bytes/runes. | Shared UTF-16 helpers plus astral/combining/newline tests in phase 2. |
| Retry duplicates notifications. | Only explicit 429 is retried; transport/5xx fail immediately in phase 3 tests. |
| Partial send is mistaken for total failure. | Error envelope reports `chunks_total`, `chunks_sent`, and failed 1-based index. |
| Dry-run and real requests diverge. | One immutable plan is serialized by both preview and Telegram client. |
| Local config leaks. | Never read repository `.tgsend`; ignore remains tested; installers/images contain no config. |
| Release matrix drifts. | GoReleaser snapshot artifact assertions and container manifest checks in phase 5. |

## Verification summary

| Gate | Command | Where it runs |
|------|---------|---------------|
| Build | `make build` | every phase after 0.1 |
| Format | `make fmt-check` | every phase |
| Vet and lint | `make lint` | every phase |
| Unit/integration | `make test` | every phase |
| Compiled-binary e2e | `make test-e2e` | phases 1-5 |
| Vulnerabilities | `make vuln` | every phase after dependencies exist |
| Release config | `make release-check` | phase 5 |
| Local container smoke | `make test-container` | phases 4-5; Docker/buildx required |
| Aggregate non-Docker | `make verify` | every phase close and CI |

**Acceptance:** `T-E2E-08` proves the reference scenario end to end; `T-MSG-07` proves exact Unicode/body reconstruction; `T-TG-09` proves serial partial-failure accounting. `T-CTR-04` repeats the principal scenario through the Docker wrapper.

**Run caveats:** e2e tests compile a fresh temporary binary; API tests use an injected `httptest` URL; container smoke is separate from `make verify`, requires Docker with buildx, and never uses real Telegram credentials.

## Model-assignment summary

| Phase | Sub-phases by assignment | Primary | `agent-1` review gates |
|-------|--------------------------|---------|------------------------|
| 0 | 0.1-0.3 `agent-3:haiku`; 0.4 `agent-2:sonnet`; 0.5 `agent-3:haiku` | `agent-3:haiku` | 0.1 contracts, 0.4 acceptance harness |
| 1 | 1.1-1.4 `agent-2:sonnet`; 1.5 `agent-3:haiku` | `agent-2:sonnet` | 1.4 CLI/JSON acceptance |
| 2 | 2.1-2.3 `agent-2:sonnet`; 2.4 `agent-3:haiku` | `agent-2:sonnet` | 2.2 Unicode algorithm, 2.3 planner acceptance |
| 3 | 3.1-3.3 `agent-2:sonnet`; 3.4 `agent-3:haiku` | `agent-2:sonnet` | 3.2 retry lifecycle, 3.3 partial-send acceptance |
| 4 | 4.1-4.3 `agent-2:sonnet`; 4.4 `agent-3:haiku` | `agent-2:sonnet` | 4.1/4.3 acceptance, 4.2 container boundary |
| 5 | 5.1-5.3 `agent-2:sonnet`; 5.4-5.5 `agent-3:haiku` | `agent-2:sonnet` | 5.1 supply chain, 5.3 release acceptance, 5.5 final docs read |
