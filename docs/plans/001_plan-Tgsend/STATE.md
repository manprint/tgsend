# Tgsend - Implementation State

> **READ THIS FILE FIRST at the start of every session, before any other plan file. OPEN a unit in section 1 before touching code; CLOSE it after the gates pass.**
> **Last updated:** 2026-09-01 01:30 UTC | **By:** `agent-2:sonnet` | **Session:** 2

## 0. Protocol

This is the only execution-state file. Position, progress, ledger, verification, deviations, and blockers all live here. A unit is one sub-phase, task, bug, verify audit, or correction.

**Resume (cold start):**

1. Read this file end to end.
2. Read section 1. If `Status: OPEN`, read section 6 and finish or revert that unit before starting another. With WIP commits on, an optional `wip:` HEAD commit is the authoritative partial diff. If `Status: none`, open the unit named by `Next action`.
3. Run the commands in section 3 and compare reality with sections 1, 7, and 11. The repository wins; correct this file when it disagrees.
4. Open only the phase file/sub-phase named in section 1. Read `overview.md` only when section 2 lacks a needed design decision.

**Open a unit before editing:** set section 1 type, ID, `Status: OPEN`, intent, phase, next action, and assignment; set section 6 to `claimed - nothing written yet`; update the header timestamp. Only then edit repository files.

**Close a unit after gates are green:** append one section-4 row; update sections 5, 7, 8, 9, 10, and 11; reset section 6 to `none - tree consistent`; set section 1 to the next unit with `Status: none`; update the timestamp. If WIP commits are on, commit only this unit's files and plan/state files, never `git add -A`, and record the SHA. A unit is not done until state is closed.

**Interrupted mid-unit:** leave `Status: OPEN` and write exact files completed, edits pending, temporary code to remove, and stopping reason in section 6. With WIP commits on, also create `wip(<id>): <what remains>`; never push it.

## 1. Current unit

- **Type:** `sub-phase`
- **ID:** `4.1`
- **Status:** `none`
- **Intent:** Harden compiled process coverage and cover every documented native behavior before packaging.
- **Phase:** 4 - Binary e2e, image, and Docker wrapper (`phase_05.md`)
- **Next action:** Read sub-phase 4.1 requirements, open the unit, then extend the black-box e2e matrix for all config, flag, exit, whitespace, and failure-position combinations.
- **Assigned:** `agent-2:sonnet`
- **Repo state:** branch `main` | working tree dirty with closed-unit state plus unrelated untracked `.serena/` | last implementation commit `ae6cc9b`

## 2. Feature context (self-contained recap)

Build Go 1.27 CLI `tgsend` to send exact UTF-8 stdin or `-m` content to one Telegram chat configured by strict TOML/environment. Optional title/type/monospace formatting uses explicit UTF-16 entities; long input splits deterministically and sends serially. Output is stable JSON except textual help; dry-run is credential/network free. Ship tested native binaries, minimal GHCR image, Docker-only POSIX wrapper, verified installers, CI, checksums, SBOMs, and MIT license.

**Reference scenario:** compiled CLI receives >4096-unit Unicode stdin with WARNING/title/monospace; fake Telegram receives ordered bounded requests, header only first, valid entities, exact reconstructed body, and stdout one schema-v1 success document.

**Hard constraints:** never read repository `.tgsend`; validate complete input/plan before send; retry only explicit 429 (2 retries, 60s cumulative); no live Telegram in CI; no token in any output/artifact.

**Key decisions in force:** D2 Cobra v1.10.2; D3 go-toml v2.4.3; D4 config/env precedence; D5 exactly one input; D7 explicit entities; D9 newline-preferred UTF-16 split; D11 bounded 429-only retry; D12 JSON send/version; D14 Docker-only wrapper; D22 fixed release matrix.

## 3. Environment and commands

These commands are authoritative and must remain identical to overview/phase gates.

- **Repo root:** `/mnt/fabio/dati/Git/SperimentazioniAI/telegram-sender`
- **Build:** `make build` | **Fmt:** `make fmt-check` | **Lint:** `make lint`
- **Unit tests:** `make test` | **E2E:** `make test-e2e`
- **Vulnerability:** `make vuln` | **Release:** `make release-check` | **Container:** `make test-container`
- **Aggregate:** `make verify` (build, format, lint, unit, e2e, vulnerability; intentionally excludes Docker and publishing)
- **Setup / caveats:** Go 1.27.0 and Make required. Phase 0.1 creates Make targets; before that, record them `not-run`, verify `go version`, and do not claim green. `make test-e2e` builds a fresh temp binary. Release check needs GoReleaser v2.18.0 and Syft v1.51.1. Container check needs Docker/buildx. No command may use repository `.tgsend` or real Telegram credentials.
- **WIP commits:** `on` (user requested local commits for every completed phase; explicit user instruction for this execution)

## 4. Work ledger

| # | Type | ID | Agent | What changed | Files | Gates | Commit |
|---|------|----|-------|--------------|-------|-------|--------|
| 1 | sub-phase | 0.1 | agent-3:haiku | Initialized Go module, buildinfo, scaffold entrypoint, Make gates, and ignore rules | go.mod, go.sum, cmd/tgsend/main.go, internal/buildinfo/*, internal/tools/tools.go, Makefile, .gitignore | build, fmt-check, lint, test, test-e2e, vuln, verify all pass | eed7e02 |
| 2 | sub-phase | 0.2 | agent-3:haiku | Added typed application errors, stable codes, exit taxonomy, safe causes, and progress validation | internal/apperr/error.go, internal/apperr/error_test.go | build, fmt-check, lint, test, test-e2e, vuln, verify all pass | 572ef9c |
| 3 | sub-phase | 0.3 | agent-3:haiku | Added Cobra root, deterministic JSON presenter, version/help behavior, and safe error stream discipline | internal/presenter/*, internal/cli/*, cmd/tgsend/main.go | build, fmt-check, lint, test, test-e2e, vuln, verify all pass | 5ead4ea |
| 4 | sub-phase | 0.4 | agent-2:sonnet | Added fresh compiled-binary e2e harness, strict JSON decoder, and CLI acceptance tests | test/e2e/*, internal/testutil/json.go | build, fmt-check, lint, test, test-e2e, vuln, verify all pass | fa791e4 |
| 5 | sub-phase | 0.5 | agent-3:haiku | Added phase-0 README for build, version, help, and current no-send limitation | README.md | README commands and build, fmt-check, lint, test, test-e2e, vuln, verify all pass | 325dc4c |
| 6 | sub-phase | 1.1 | agent-2:sonnet | Added bounded exact input acquisition with stdin/message precedence, terminal handling, limits, UTF-8 validation, typed errors, and fuzz coverage | internal/input/source.go, internal/input/source_test.go | build, fmt-check, lint, test, test-e2e, vuln, verify all pass | 979cca9 |
| 7 | sub-phase | 1.2 | agent-2:sonnet | Added strict TOML/environment configuration loading with path precedence, ChatID normalization, validation, and secret-safe typed errors | internal/config/config.go, internal/config/config_test.go | build, fmt-check, lint, test, test-e2e, vuln, verify all pass | f42d329 |
| 8 | sub-phase | 1.3 | agent-3:haiku | Added stable send/dry-run JSON schemas, non-null arrays, preview entities, optional progress, and golden fixtures | internal/presenter/presenter.go, internal/presenter/presenter_test.go, internal/presenter/testdata/* | build, fmt-check, lint, test, test-e2e, vuln, verify all pass | 473b273 |
| 9 | sub-phase | 1.4 | agent-2:sonnet | Added shared message types, UTF-16 basic planner, offline service, CLI flags, compiled input/dry-run acceptance tests, and credential-free behavior | internal/message/*, internal/app/*, internal/cli/*, internal/presenter/presenter.go, cmd/tgsend/main.go, test/e2e/input_config_test.go | build, fmt-check, lint, test, test-e2e, vuln, verify all pass | 5d35466 |
| 10 | sub-phase | 1.5 | agent-3:haiku | Updated README with phase-one usage, configuration, environment, JSON, dry-run, limits, exit codes, troubleshooting, and verified examples | README.md | README examples in isolated HOME plus build, fmt-check, lint, test, test-e2e, vuln, verify all pass | aec27ac |
| 11 | sub-phase | 2.1 | agent-2:sonnet | Added checked UTF-16 length, prefix, and byte-offset primitives with invalid UTF-8 and fuzz coverage | internal/message/utf16.go, internal/message/utf16_test.go | build, fmt-check, lint, test, test-e2e, vuln, verify, fuzz all pass | b85a3c3 |
| 12 | sub-phase | 2.2 | agent-2:sonnet | Added deterministic UTF-16-bounded body splitting with newline preference, CRLF preservation, progress checks, reconstruction tests, and fuzz coverage | internal/message/split.go, internal/message/split_test.go | build, fmt-check, lint, test, test-e2e, vuln, verify, fuzz all pass | beeec45 |
| 13 | sub-phase | 2.3 | agent-2:sonnet | Replaced BasicPlanner with the validated title/type/monospace composer, UTF-16 entities, long-body integration, CLI flags, and compiled message acceptance tests | internal/message/planner.go, internal/message/planner_test.go, internal/message/basic.go, internal/message/basic_test.go, internal/app/service.go, internal/app/service_test.go, internal/cli/root.go, internal/cli/root_test.go, internal/apperr/error.go, test/e2e/message_test.go | build, fmt-check, lint, test, test-e2e, vuln, verify, planner fuzz all pass; T-MSG-01..09 pass | 04ae677 |
| 14 | sub-phase | 2.4 | agent-3:haiku | Updated README with phase-two formatting, UTF-16 chunking, limits, dry-run entities, examples, and offline status | README.md | README examples parsed successfully; build, fmt-check, lint, test, test-e2e, vuln, verify all pass | 38dbb59 |
| 15 | sub-phase | 3.1 | agent-2:sonnet | Added one-attempt Telegram `sendMessage` JSON client with bounded response parsing, secret-safe typed errors, response closing, and base URL validation; stabilized vuln gate on patched Go 1.26.6 scanner runtime | internal/telegram/client.go, internal/telegram/client_test.go, Makefile | build, fmt-check, lint, race unit, e2e, vulnerability, verify all pass; protocol/request/redaction tests pass | 80062ec |
| 16 | sub-phase | 3.2 | agent-2:sonnet | Added bounded explicit-429 retry policy with production context-aware sleeper, overflow/budget checks, and exhaustive no-retry/attempt-count tests | internal/telegram/retry.go, internal/telegram/retry_test.go, internal/telegram/client.go, internal/telegram/client_test.go | build, fmt-check, lint, race unit, e2e, vulnerability, verify all pass; retry policy tests pass | 107c438 |
| 17 | sub-phase | 3.3 | agent-2:sonnet | Replaced the offline no-send branch with configuration-aware serial native sends, ordered message IDs, exact partial progress, a 10-second production HTTP client, loopback-only e2e endpoint controls, and the reference Unicode process test | internal/app/service.go, internal/app/service_test.go, internal/cli/root.go, internal/buildinfo/buildinfo.go, cmd/tgsend/main.go, cmd/tgsend/main_test.go, internal/telegram/retry.go, test/e2e/main_test.go, test/e2e/server_test.go, test/e2e/send_test.go | build, fmt-check, lint, race unit, e2e, vulnerability, verify all pass; T-TG-07..09 and T-E2E-08 pass | 5bbbe13 |
| 18 | sub-phase | 3.4 | agent-2:sonnet | Rewrote the operator README for native sending, all seven workflows, configuration/env precedence, JSON, limits, retries, partial progress, security, and manual smoke testing; verified offline examples | README.md | README offline examples pass; build, fmt-check, lint, race unit, e2e, vulnerability, verify all pass | ae6cc9b |

## 5. Files touched

| Path | What was done | Unit |
|------|---------------|------|
| go.mod | Go 1.27 module with pinned Cobra and TOML dependencies | 0.1 |
| go.sum | Tidy dependency checksums | 0.1 |
| cmd/tgsend/main.go | Compile-safe entrypoint placeholder | 0.1 |
| internal/buildinfo/buildinfo.go | Linkable build metadata API and test-endpoint build gate | 0.1, 3.3 |
| internal/buildinfo/buildinfo_test.go | Default and linker-variable tests | 0.1 |
| internal/tools/tools.go | Build-tagged retention of planned direct dependencies | 0.1 |
| Makefile | Build, format, lint, test, e2e, vulnerability, and verify gates | 0.1 |
| .gitignore | Preserved `.tgsend`; added build/release/coverage outputs | 0.1 |
| internal/apperr/error.go | Typed safe application errors, codes, kinds, exit codes, and progress | 0.2 |
| internal/apperr/error_test.go | Taxonomy, unwrap, redaction, and progress tests | 0.2 |
| internal/presenter/presenter.go | Stable JSON envelope, version response, and safe error conversion | 0.3 |
| internal/presenter/presenter_test.go | Envelope ordering, metadata, and cause isolation tests | 0.3 |
| internal/cli/root.go | Cobra root, flag handling, stream setup, and exit mapping | 0.3 |
| internal/cli/root_test.go | Version, help, unknown flag, positional, and safety tests | 0.3 |
| cmd/tgsend/main.go | Real stream dependency construction and process entrypoint | 0.3 |
| internal/testutil/json.go | Exactly-one-document JSON test decoder with newline requirement | 0.4 |
| test/e2e/main_test.go | Fresh temporary compiled-binary TestMain and repository-root resolution | 0.4 |
| test/e2e/harness_test.go | Controlled process runner, exit extraction, timeout, and decoder tests | 0.4 |
| test/e2e/cli_test.go | Compiled version, help, and unknown-flag acceptance tests | 0.4 |
| README.md | Phase-0 user guide for build, version, help, and no-send limitation | 0.5 |
| internal/input/source.go | Exact message/stdin acquisition with bounded reads and UTF-8 validation | 1.1 |
| internal/input/source_test.go | Input precedence, terminal, limit, error, and fuzz tests | 1.1 |
| internal/config/config.go | Strict TOML/environment configuration loader with path and validation rules | 1.2 |
| internal/config/config_test.go | Configuration precedence, type, validation, and redaction tests | 1.2 |
| internal/presenter/presenter.go | Stable send/dry-run result, preview, and progress JSON schema types | 1.3 |
| internal/presenter/presenter_test.go | Golden, omission, array, credential, and error-code serialization tests | 1.3 |
| internal/presenter/testdata/send_success.json | Golden real-send JSON response | 1.3 |
| internal/presenter/testdata/send_dry_run.json | Golden dry-run JSON response | 1.3 |
| internal/presenter/testdata/send_error.json | Golden error JSON response | 1.3 |
| internal/message/types.go | Shared message entity and chunk types | 1.4 |
| internal/message/basic.go | Phase-one UTF-16 bounded raw-text planner | 1.4 |
| internal/message/basic_test.go | Planner raw-text and UTF-16 boundary tests | 1.4 |
| internal/app/service.go | Input, planning, dry-run, and phase-one transport boundary service | 1.4 |
| internal/app/service_test.go | Service ordering, dry-run, and network isolation tests | 1.4 |
| internal/cli/root.go | Registered phase-one message/config/silent/dry-run/input-limit flags and runner wiring; removed obsolete no-send wording | 1.4, 3.3 |
| internal/cli/root_test.go | CLI defaults, Changed bits, and application stream/exit tests | 1.4 |
| cmd/tgsend/main.go | Constructed production native sender, 10-second HTTP client, retry sleeper, and endpoint gate | 1.4, 3.3 |
| test/e2e/input_config_test.go | Compiled input, dry-run, JSON, conflict, empty, and limit acceptance tests | 1.4 |
| README.md | Phase-one user guide for usage, configuration, JSON, dry-run, limits, exit codes, and troubleshooting | 1.5 |
| internal/message/utf16.go | Checked UTF-16 code-unit length, prefix, offset, and addition primitives | 2.1 |
| internal/message/utf16_test.go | UTF-16 table, boundary, invalid-input, overflow, and fuzz tests | 2.1 |
| internal/message/split.go | Deterministic newline-preferred UTF-16-bounded body splitter | 2.2 |
| internal/message/split_test.go | Split boundary, Unicode, reconstruction, bounds, and fuzz tests | 2.2 |
| internal/message/planner.go | Final title/type/monospace composer and entity validator | 2.3 |
| internal/message/planner_test.go | Header, severity, entity, limit, raw-path, and planner fuzz tests | 2.3 |
| internal/message/basic.go | Removed superseded phase-one planner | 2.3 |
| internal/message/basic_test.go | Removed superseded phase-one planner tests | 2.3 |
| internal/app/service.go | Forwarded formatting options to the final planner and added config/send orchestration with progress | 2.3, 3.3 |
| internal/app/service_test.go | Migrated service planner seam and added send ordering, validation, progress, dry-run, and serialization tests | 2.3, 3.3 |
| internal/cli/root.go | Added title, type, and monospace flags | 2.3 |
| internal/cli/root_test.go | Added formatting flag forwarding and default assertions | 2.3 |
| internal/apperr/error.go | Added safe title-too-long usage code | 2.3 |
| test/e2e/message_test.go | Compiled binary message splitting and formatting acceptance tests | 2.3 |
| internal/telegram/client.go | Telegram Bot API request/response client, bounded retry integration, safe errors, and bounded response reads | 3.1, 3.2 |
| internal/telegram/client_test.go | Telegram request, response, retry integration, redaction, closure, cancellation, and endpoint validation tests | 3.1, 3.2 |
| internal/telegram/retry.go | Context-aware bounded retry policy for explicit HTTP/API 429 responses and production sleeper factory | 3.2, 3.3 |
| internal/telegram/retry_test.go | Retry delay, cumulative budget, overflow, cancellation, attempt-count, and no-retry tests | 3.2 |
| Makefile | Vulnerability gate uses patched Go 1.26.6 temporary snapshot runtime | 3.1 |
| cmd/tgsend/main_test.go | Production/test endpoint policy and loopback validation tests | 3.3 |
| test/e2e/main_test.go | Fresh compiled e2e binary with test endpoint linker gate | 0.4, 3.3 |
| test/e2e/server_test.go | Loopback fake Telegram server with decoded request recording and scripted responses | 3.3 |
| test/e2e/send_test.go | Native send, ordered multi-chunk, partial failure, and full Unicode reference acceptance tests | 3.3 |
| README.md | Complete native sender guide with seven workflows, retry/failure semantics, security, limits, and manual smoke procedure | 3.4 |

## 6. In-flight work

claimed - nothing written yet; `.serena/` remains unrelated and untracked

## 7. Verification state

| Gate / test | Command | Last result | When |
|-------------|---------|-------------|------|
| Go toolchain | `go version` | PASS: local go1.26.1; module commands auto-selected go1.27.0 | 2026-09-01 |
| Build | `make build` | PASS | 2026-09-01 |
| Format | `make fmt-check` | PASS | 2026-09-01 |
| Lint | `make lint` | PASS: go vet and golangci-lint v2.13.2 | 2026-09-01 |
| Unit | `make test` | PASS: go test -race ./... including input, config, JSON schema, final planner, service send orchestration, CLI, endpoint policy, UTF-16 primitives, splitter, and planner tests | 2026-09-01 |
| E2E | `make test-e2e` | PASS: fresh compiled test-endpoint binary with input/config, dry-run/JSON, splitting, formatting, native Telegram sends, retries, partial progress, entities, and validation acceptance tests | 2026-09-01 |
| Vulnerability | `make vuln` | PASS: govulncheck v1.1.4 with temporary Go 1.26.6 snapshot; `.tgsend` excluded | 2026-09-01 |
| Release | `make release-check` | not-run | - |
| Container | `make test-container` | not-run | - |
| Aggregate | `make verify` | PASS: build, format, lint, race unit, e2e, vulnerability, Telegram protocol/retry, native send orchestration, and endpoint-policy gates; planner fuzz target passed 3s; README examples parsed | 2026-09-01 |

**Failing output (verbatim, trimmed to the error):**

```text
none
```

## 8. Runtime deviations from the plan

| # | Plan said | What was done | Why | Impact on later phases |
|---|-----------|---------------|-----|------------------------|
| 1 | Run pinned govulncheck directly against the Go 1.27 module | Scan a temporary copy with only the `go` directive lowered to Go 1.26.6; the pinned binary is built and run with the downloaded Go 1.26.6 toolchain | govulncheck v1.1.4 panics in x/tools SSA while analyzing Go 1.27 packages; source compatibility is unchanged | Make vuln remains pinned and excludes `.tgsend`; later phases inherit the same scan workaround |

## 9. Blockers and open questions

- none

## 10. Do-not-repeat

- Never open, print, copy, package, or use repository `.tgsend`.
- Do not retry transport, timeout, 5xx, malformed response, or non-429 API failures.
- Do not use deprecated GoReleaser `dockers` or `docker_manifests`; use `dockers_v2`.
- Do not run govulncheck v1.1.4 via `go run` under the auto-selected Go 1.27 toolchain; it panics in x/tools SSA. The Make target's temporary compatibility snapshot is the working path.

## 11. Progress board

### Phases

| Phase | File | Status | Notes |
|-------|------|--------|-------|
| 0 - Foundation and contracts | phase_01.md | `DONE` | 0.1-0.5 closed; build, test, e2e, lint, vulnerability, and README gates pass |
| 1 - Input, config, JSON, and offline CLI | phase_02.md | `DONE` | 1.1-1.5 closed; README examples and all phase gates pass |
| 2 - Message composition and chunking | phase_03.md | `DONE` | 2.1-2.4 closed; formatting/chunking and README gates pass |
| 3 - Telegram transport and send orchestration | phase_04.md | `DONE` | 3.1-3.4 closed; Telegram protocol/retry, native serial sends, partial progress, endpoint policy, e2e reference scenario, and README gates pass |
| 4 - Binary e2e, image, and Docker wrapper | phase_05.md | `TODO` | - |
| 5 - Release automation, installers, and final docs | phase_06.md | `TODO` | - |

### Tests

| ID | Type | Status | Notes |
|----|------|--------|-------|
| T-CLI-01 | e2e | `DONE` | Version JSON from compiled binary |
| T-CLI-02 | e2e | `DONE` | Textual help from compiled binary |
| T-CLI-03 | e2e | `DONE` | Unknown flag produces one stderr JSON |
| T-IN-01 | e2e | `DONE` | Exact whitespace/newline stdin |
| T-IN-02 | e2e | `DONE` | Message/stdin conflict |
| T-IN-03 | e2e | `DONE` | Empty pipe error |
| T-IN-04 | e2e | `DONE` | Input limit rejection |
| T-CFG-01 | e2e | `TODO` | Default HOME config |
| T-CFG-02 | e2e | `TODO` | Explicit config selection |
| T-CFG-03 | e2e | `TODO` | Environment-only config |
| T-CFG-04 | e2e | `TODO` | Explicit missing config error |
| T-CFG-05 | e2e | `TODO` | Strict TOML/redaction |
| T-JSON-01 | e2e | `DONE` | Dry-run stdout-only JSON |
| T-JSON-02 | e2e | `DONE` | Validation errors/exit taxonomy |
| T-JSON-03 | e2e | `DONE` | Credential omission |
| T-DRY-01 | e2e | `DONE` | Offline no-config dry-run |
| T-MSG-01 | e2e | `DONE` | Newline-preferred split |
| T-MSG-02 | e2e | `DONE` | Astral fallback split |
| T-MSG-03 | e2e | `DONE` | No chunk labels |
| T-MSG-04 | e2e | `DONE` | Exact title preview |
| T-MSG-05 | e2e | `DONE` | Severity normalization/code points |
| T-MSG-06 | e2e | `DONE` | Monospace UTF-16 offsets |
| T-MSG-07 | unit | `DONE` | Unicode reconstruction/bounds table |
| T-MSG-08 | e2e | `DONE` | Header only first chunk |
| T-MSG-09 | e2e | `DONE` | Invalid type/title rejected locally |
| T-TG-01 | integration | `DONE` | Exact successful request |
| T-TG-02 | integration | `DONE` | API rejection mapping |
| T-TG-03 | integration | `DONE` | Protocol/transport failure mapping |
| T-TG-04 | integration | `DONE` | One 429 retry then success |
| T-TG-05 | integration | `DONE` | Retry exhaustion |
| T-TG-06 | integration | `DONE` | Ambiguous failure never retried |
| T-TG-07 | e2e | `DONE` | Env credentials native send path against loopback fake server |
| T-TG-08 | e2e | `DONE` | Ordered multi-chunk IDs |
| T-TG-09 | e2e | `DONE` | Partial failure progress/stop |
| T-E2E-01 | e2e | `TODO` | Default config plus stdin |
| T-E2E-02 | e2e | `TODO` | Explicit config plus message flag |
| T-E2E-03 | e2e | `TODO` | Environment precedence |
| T-E2E-04 | e2e | `TODO` | Silent request field |
| T-E2E-05 | e2e | `TODO` | Dry-run no credentials |
| T-E2E-06 | e2e | `TODO` | Exit categories 2-7 |
| T-E2E-07 | e2e | `TODO` | Whitespace/CRLF preservation |
| T-E2E-08 | e2e | `DONE` | Full reference scenario: Unicode body, first-only header, UTF-16 entities, exact reconstruction, one success JSON |
| T-E2E-09 | e2e | `TODO` | Failure position variants |
| T-E2E-10 | e2e | `TODO` | Help/version bypass side effects |
| T-CTR-01 | container | `TODO` | Image version JSON |
| T-CTR-02 | container | `TODO` | Image offline stdin dry-run |
| T-CTR-03 | container | `TODO` | Non-root/platform inspect |
| T-CTR-04 | container | `TODO` | Reference planning through image |
| T-CTR-05 | container | `TODO` | No config/secret in image |
| T-WRP-01 | wrapper | `TODO` | Fake Docker args/stdin/env |
| T-WRP-02 | wrapper | `TODO` | Wrapper/native dry-run equivalence |
| T-WRP-03 | wrapper | `TODO` | Wrapper config forms |
| T-WRP-04 | wrapper | `TODO` | Exit status preservation |
| T-WRP-05 | wrapper | `TODO` | Secret absent from argv/stderr |
| T-WRP-06 | wrapper | `TODO` | Linux/macOS POSIX syntax |
| T-REL-01 | release | `TODO` | GoReleaser config check |
| T-REL-02 | release | `TODO` | Seven target artifacts plus metadata |
| T-REL-03 | release | `TODO` | Archive content/build metadata |
| T-REL-04 | release | `TODO` | Docker v2 three-platform config |
| T-REL-05 | release | `TODO` | Snapshot artifact checker |
| T-REL-06 | release | `TODO` | Missing artifact negative fixture |
| T-REL-07 | release | `TODO` | Installer/generated-name agreement |
| T-REL-08 | release | `TODO` | Published manifest platforms/OCI |
| T-REL-09 | release | `TODO` | No config/secret in release artifacts |
| T-CI-01 | workflow | `TODO` | Workflow policy |
| T-CI-02 | workflow | `TODO` | CI/local command parity |
| T-CI-03 | workflow | `TODO` | Release/GHCR wiring |
| T-CI-04 | workflow | `TODO` | No live Telegram CI |
| T-CI-05 | manual | `TODO` | Fork/test-tag syntax acceptance |
| T-INS-01 | installer | `TODO` | Latest binary install |
| T-INS-02 | installer | `TODO` | Pinned version URLs |
| T-INS-03 | installer | `TODO` | Checksum fail-closed behavior |
| T-INS-04 | installer | `TODO` | Wrapper installation/use |
| T-INS-05 | installer | `TODO` | OS/architecture mapping |
| T-INS-06 | installer | `TODO` | Installer redaction |
| T-DOC-01 | docs | `DONE` | Dry-run examples execute in isolated HOME with JSON assertions |
| T-DOC-02 | docs | `DONE` | Send examples map to compiled loopback fake-endpoint e2e coverage |
| T-DOC-03 | docs | `TODO` | Install examples execute |
| T-DOC-04 | docs | `DONE` | Original seven scenarios are executable command examples |
| T-DOC-05 | docs | `DONE` | README flag list/defaults match compiled help |
| T-DOC-06 | docs | `DONE` | README contains placeholders only and no hidden test switch |

### Docs

| Doc | Phase | Status | Notes |
|-----|-------|--------|-------|
| README.md | 0 | `DONE` | Build/help/version and no-send limitation |
| README.md | 1 | `DONE` | Input/config/JSON/basic dry-run |
| README.md | 2 | `DONE` | Formatting/chunking preview and offline dry-run behavior |
| README.md | 3 | `DONE` | Complete native send/retry/failure behavior and seven original workflows |
| README.md | 4 | `TODO` | Docker/wrapper local use |
| README.md | 5 | `TODO` | Release/install/agentic/final guide |
| LICENSE | 5 | `TODO` | MIT, holder manprint |

### Audits

| Report | Date | Verdict | Open findings |
|--------|------|---------|---------------|
| none yet | - | - | - |
