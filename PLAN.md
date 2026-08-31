# Tgsend Implementation Plan

This is the repository entry point requested for the implementation plan. The canonical, executable plan is split across [docs/plans/001_plan-Tgsend/](docs/plans/001_plan-Tgsend/) so every phase is self-contained and execution can resume safely.

**Before implementing anything, read [STATE.md](docs/plans/001_plan-Tgsend/STATE.md).** It is the only file allowed to record progress, current work, verification results, deviations, and blockers. Open one sub-phase there before editing and close it only after its gates pass.

## Target outcome

Implement Go 1.27 CLI `tgsend` for sending exact UTF-8 stdin or `-m` content to one Telegram chat. It must support strict TOML/environment configuration, optional title/severity/monospace/silent formatting, deterministic Unicode-safe splitting, 429-only retries, stable JSON, offline dry-run, static cross-platform binaries, a minimal GHCR image, a Docker-only POSIX wrapper, checksum-verifying installers, and tag-driven releases.

The final acceptance scenario sends a long Unicode body with WARNING, title, and monospace options to a fake Telegram endpoint. Requests must be ordered and <=4096 UTF-16 units; only request 1 has the header; entities must be valid; reconstructed body bytes must equal input; stdout must contain one schema-v1 success document.

## Fixed contracts

- Module: `github.com/manprint/tgsend`; Go `1.27.0`; Cobra `v1.10.2`; go-toml/v2 `v2.4.3`.
- Input: exactly one non-empty source (`-m` or stdin), valid UTF-8, no trimming, 1 MiB default maximum.
- Config: strict TOML keys `token`, `chat_id`; environment overrides each field; explicit missing `-c` always fails.
- Formatting: explicit Telegram entities, no parse mode; title bold; monospace body `pre`; fixed severity code points; header only chunk 1.
- Chunking: newline-preferred, then complete-rune fallback; all limits/entity offsets measured in UTF-16; no chunk counters.
- Transport: `sendMessage`, 10s per attempt, serial chunks; retry only explicit 429, 2 retries, 60s cumulative wait.
- Output: send/version JSON with `schema_version:"1"`; success stdout, failure stderr; textual help; exit codes 0/2/3/4/5/6/7.
- Dry-run: no config read, DNS, HTTP, or sleep; exact planned chunks returned without token/chat ID.
- Delivery: seven native targets; GHCR Linux amd64/arm64/armv7; checksums, SBOMs, MIT license (`manprint`).
- Security: repository `.tgsend` is never read, logged, tested, copied, packaged, or committed.

Full decisions, interface fields, invariants, risks, and external sources are in [overview.md](docs/plans/001_plan-Tgsend/overview.md).

## Phase 0 - Foundation and contracts

Detailed file: [phase_01.md](docs/plans/001_plan-Tgsend/phase_01.md)

- **0.1 Initialize module, entrypoint, and build metadata:** create module, thin main, injectable build info, Make gates, and ignore rules.
- **0.2 Define typed application errors and exit taxonomy:** freeze safe symbolic errors, progress shape, and exit codes.
- **0.3 Implement Cobra root, JSON version output, and stream discipline:** textual help, JSON version/errors, single-document output.
- **0.4 Add compiled-binary e2e harness and acceptance fixtures:** build a temporary executable under isolated HOME/environment.
- **0.5 Update README.md:** document only build/help/version and the current no-send limitation.

**Gate:** `make verify`; compiled `--version`, textual help, and invalid-argument JSON tests pass without reading real config.

## Phase 1 - Input, config, JSON, and offline CLI

Detailed file: [phase_02.md](docs/plans/001_plan-Tgsend/phase_02.md)

- **1.1 Implement bounded, exact input acquisition:** precedence, terminal handling, UTF-8, empty/size/read errors, byte preservation.
- **1.2 Implement strict TOML plus environment merge:** integer/string chat ID, strict keys/types, explicit/default path semantics, redaction.
- **1.3 Freeze send/dry-run JSON schemas:** stable result/error/progress/preview structs and golden fixtures.
- **1.4 Wire basic offline CLI and application service:** register core flags and provide credential-free one-chunk raw dry-run.
- **1.5 Update README.md:** document input/config/env/JSON/dry-run and exact intermediate limitations.

**Gate:** `make verify`; all `T-IN-*`, `T-CFG-*`, `T-JSON-*`, and `T-DRY-01` pass.

## Phase 2 - Message composition and chunking

Detailed file: [phase_03.md](docs/plans/001_plan-Tgsend/phase_03.md)

- **2.1 Implement UTF-16 boundary primitives:** lengths, byte-prefix limits, offsets, overflow/invalid-input behavior.
- **2.2 Implement deterministic newline-preferred body splitting:** preserve every byte and guarantee bounded valid UTF-8 chunks.
- **2.3 Implement final composer/planner and formatting CLI:** exact header, severity/title/pre entities, all chunks planned before send.
- **2.4 Update README.md:** document formatting, splitting, preview entities, and limits.

**Gate:** `make verify`; fuzz/table invariants plus `T-MSG-01..09` prove reconstruction, bounds, and entity correctness.

## Phase 3 - Telegram transport and send orchestration

Detailed file: [phase_04.md](docs/plans/001_plan-Tgsend/phase_04.md)

- **3.1 Implement one-attempt Bot API client:** exact request/response protocol, bounded body, resource cleanup, token redaction.
- **3.2 Add bounded 429 retry policy:** exact sleep/attempt counts; every ambiguous failure explicitly tested as no-retry.
- **3.3 Complete application send loop and process behavior:** serial order, stop-on-failure, progress, message IDs, guarded test endpoint.
- **3.4 Update README.md:** full native usage, retry/partial-failure semantics, security, and manual live smoke.

**Gate:** `make verify`; `T-TG-01..09` and `T-E2E-08` prove exact send, retry, ordering, redaction, and partial completion.

## Phase 4 - Binary e2e, image, and Docker wrapper

Detailed file: [phase_05.md](docs/plans/001_plan-Tgsend/phase_05.md)

- **4.1 Complete compiled-binary acceptance matrix:** every documented source/config/flag/error path against isolated fake Telegram.
- **4.2 Add minimal production image and local smoke target:** distroless static non-root image, CA support, no source/secrets/build tools.
- **4.3 Implement and test POSIX Docker wrapper:** safe argument/stdin/env/config forwarding, read-only mount, exact exit preservation.
- **4.4 Update README.md:** local image/wrapper installation, use, platform limits, and troubleshooting.

**Gate:** `make verify && make test-container`; all `T-E2E-*`, `T-CTR-*`, and `T-WRP-*` pass on their declared platforms.

## Phase 5 - Release automation, installers, and final docs

Detailed file: [phase_06.md](docs/plans/001_plan-Tgsend/phase_06.md)

- **5.1 Configure GoReleaser, licenses, archives, SBOMs, and GHCR image:** exact target matrix and modern `dockers_v2` manifest.
- **5.2 Add CI and tag-driven publish workflows:** non-live quality matrix, least permissions, pinned actions/tools, gated tag release.
- **5.3 Implement checksum-verifying POSIX installers:** latest/pinned binary and wrapper install with atomic fail-closed replacement.
- **5.4 Add release artifact and manifest acceptance checks:** reject missing/extra/corrupt/unsafe artifacts and wrong image platforms.
- **5.5 Update README.md and perform final documentation review:** complete user and agentic guide with tested examples/installers.

**Gate:** `make verify && make release-check && make test-container`; all release, CI, installer, documentation, and post-publish manifest checks pass.

## Shared quality gates

| Gate | Command | Purpose |
|------|---------|---------|
| Build | `make build` | Build native CLI |
| Format | `make fmt-check` | Reject non-gofmt Go files |
| Lint | `make lint` | go vet plus pinned golangci-lint |
| Unit/integration | `make test` | Race-enabled package tests |
| Binary e2e | `make test-e2e` | Fresh compiled-process tests |
| Vulnerability | `make vuln` | Pinned govulncheck |
| Aggregate | `make verify` | All deterministic non-Docker gates |
| Container | `make test-container` | Local image/wrapper smoke |
| Release | `make release-check` | GoReleaser snapshot/artifact validation |

Assignments use `agent-1:opus` for architecture/review, `agent-2:sonnet` for non-trivial implementation/testing, and `agent-3:haiku` for scaffolding/mechanical documentation. Exact assignment and done criteria appear in every sub-phase file.
