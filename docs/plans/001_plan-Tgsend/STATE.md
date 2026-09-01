# Tgsend - Implementation State

> **READ THIS FILE FIRST at the start of every session, before any other plan file. OPEN a unit in section 1 before touching code; CLOSE it after the gates pass.**
> **Last updated:** 2026-09-01 00:08 UTC | **By:** `agent-1:opus` | **Session:** 2

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
- **ID:** `1.1`
- **Status:** `none`
- **Intent:** Implement bounded, exact input acquisition.
- **Phase:** 1 - Input, config, JSON, and offline CLI (`phase_02.md`)
- **Next action:** Open and implement phase 1 sub-phase 1.1 as specified in `phase_02.md`.
- **Assigned:** `agent-2:sonnet`
- **Repo state:** branch `main` | working tree dirty with unrelated untracked `.serena/` only | last commit `fa791e4`

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
| 1 | sub-phase | 0.1 | agent-3:haiku | Initialized Go module, buildinfo, scaffold entrypoint, Make gates, and ignore rules | go.mod, go.sum, cmd/tgsend/main.go, internal/buildinfo/*, internal/tools/tools.go, Makefile, .gitignore | build, fmt-check, lint, test, test-e2e, vuln, verify all pass | uncommitted |
| 2 | sub-phase | 0.2 | agent-3:haiku | Added typed application errors, stable codes, exit taxonomy, safe causes, and progress validation | internal/apperr/error.go, internal/apperr/error_test.go | build, fmt-check, lint, test, test-e2e, vuln, verify all pass | uncommitted |
| 3 | sub-phase | 0.3 | agent-3:haiku | Added Cobra root, deterministic JSON presenter, version/help behavior, and safe error stream discipline | internal/presenter/*, internal/cli/*, cmd/tgsend/main.go | build, fmt-check, lint, test, test-e2e, vuln, verify all pass | uncommitted |
| 4 | sub-phase | 0.4 | agent-2:sonnet | Added fresh compiled-binary e2e harness, strict JSON decoder, and CLI acceptance tests | test/e2e/*, internal/testutil/json.go | build, fmt-check, lint, test, test-e2e, vuln, verify all pass | uncommitted |
| 5 | sub-phase | 0.5 | agent-3:haiku | Added phase-0 README for build, version, help, and current no-send limitation | README.md | README commands and build, fmt-check, lint, test, test-e2e, vuln, verify all pass | uncommitted |

## 5. Files touched

| Path | What was done | Unit |
|------|---------------|------|
| go.mod | Go 1.27 module with pinned Cobra and TOML dependencies | 0.1 |
| go.sum | Tidy dependency checksums | 0.1 |
| cmd/tgsend/main.go | Compile-safe entrypoint placeholder | 0.1 |
| internal/buildinfo/buildinfo.go | Linkable build metadata API | 0.1 |
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

## 6. In-flight work

none - tree consistent

## 7. Verification state

| Gate / test | Command | Last result | When |
|-------------|---------|-------------|------|
| Go toolchain | `go version` | PASS: local go1.26.1; module commands auto-selected go1.27.0 | 2026-08-31 |
| Build | `make build` | PASS | 2026-08-31 |
| Format | `make fmt-check` | PASS | 2026-08-31 |
| Lint | `make lint` | PASS: go vet and golangci-lint v2.13.2 | 2026-08-31 |
| Unit | `make test` | PASS: go test -race ./... including CLI/presenter tests | 2026-09-01 |
| E2E | `make test-e2e` | PASS: fresh compiled binary harness and CLI acceptance tests | 2026-09-01 |
| Vulnerability | `make vuln` | PASS: govulncheck v1.1.4 | 2026-08-31 |
| Release | `make release-check` | not-run | - |
| Container | `make test-container` | not-run | - |
| Aggregate | `make verify` | PASS | 2026-08-31 |

**Failing output (verbatim, trimmed to the error):**

```text
none
```

## 8. Runtime deviations from the plan

| # | Plan said | What was done | Why | Impact on later phases |
|---|-----------|---------------|-----|------------------------|
| 1 | Run pinned govulncheck directly against the Go 1.27 module | Scan a temporary copy with only the `go` directive lowered to 1.26.1; the pinned binary is built with local Go 1.26.1 | govulncheck v1.1.4 panics or rejects Go 1.27 package analysis; source compatibility is unchanged | Make vuln remains pinned and excludes `.tgsend`; later phases inherit the same scan workaround |

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
| 1 - Input, config, JSON, and offline CLI | phase_02.md | `TODO` | - |
| 2 - Message composition and chunking | phase_03.md | `TODO` | - |
| 3 - Telegram transport and send orchestration | phase_04.md | `TODO` | - |
| 4 - Binary e2e, image, and Docker wrapper | phase_05.md | `TODO` | - |
| 5 - Release automation, installers, and final docs | phase_06.md | `TODO` | - |

### Tests

| ID | Type | Status | Notes |
|----|------|--------|-------|
| T-CLI-01 | e2e | `DONE` | Version JSON from compiled binary |
| T-CLI-02 | e2e | `DONE` | Textual help from compiled binary |
| T-CLI-03 | e2e | `DONE` | Unknown flag produces one stderr JSON |
| T-IN-01 | e2e | `TODO` | Exact whitespace/newline stdin |
| T-IN-02 | e2e | `TODO` | Message/stdin conflict |
| T-IN-03 | e2e | `TODO` | Empty pipe error |
| T-IN-04 | e2e | `TODO` | Input limit rejection |
| T-CFG-01 | e2e | `TODO` | Default HOME config |
| T-CFG-02 | e2e | `TODO` | Explicit config selection |
| T-CFG-03 | e2e | `TODO` | Environment-only config |
| T-CFG-04 | e2e | `TODO` | Explicit missing config error |
| T-CFG-05 | e2e | `TODO` | Strict TOML/redaction |
| T-JSON-01 | e2e | `TODO` | Dry-run stdout-only JSON |
| T-JSON-02 | e2e | `TODO` | Validation errors/exit taxonomy |
| T-JSON-03 | e2e | `TODO` | Credential omission |
| T-DRY-01 | e2e | `TODO` | Offline no-config dry-run |
| T-MSG-01 | e2e | `TODO` | Newline-preferred split |
| T-MSG-02 | e2e | `TODO` | Astral fallback split |
| T-MSG-03 | e2e | `TODO` | No chunk labels |
| T-MSG-04 | e2e | `TODO` | Exact title preview |
| T-MSG-05 | e2e | `TODO` | Severity normalization/code points |
| T-MSG-06 | e2e | `TODO` | Monospace UTF-16 offsets |
| T-MSG-07 | unit | `TODO` | Unicode reconstruction/bounds table |
| T-MSG-08 | e2e | `TODO` | Header only first chunk |
| T-MSG-09 | e2e | `TODO` | Invalid type/title rejected locally |
| T-TG-01 | integration | `TODO` | Exact successful request |
| T-TG-02 | integration | `TODO` | API rejection mapping |
| T-TG-03 | integration | `TODO` | Protocol/transport failure mapping |
| T-TG-04 | integration | `TODO` | One 429 retry then success |
| T-TG-05 | integration | `TODO` | Retry exhaustion |
| T-TG-06 | integration | `TODO` | Ambiguous failure never retried |
| T-TG-07 | e2e | `TODO` | Env credentials real send path |
| T-TG-08 | e2e | `TODO` | Ordered multi-chunk IDs |
| T-TG-09 | e2e | `TODO` | Partial failure progress/stop |
| T-E2E-01 | e2e | `TODO` | Default config plus stdin |
| T-E2E-02 | e2e | `TODO` | Explicit config plus message flag |
| T-E2E-03 | e2e | `TODO` | Environment precedence |
| T-E2E-04 | e2e | `TODO` | Silent request field |
| T-E2E-05 | e2e | `TODO` | Dry-run no credentials |
| T-E2E-06 | e2e | `TODO` | Exit categories 2-7 |
| T-E2E-07 | e2e | `TODO` | Whitespace/CRLF preservation |
| T-E2E-08 | e2e | `TODO` | Full reference scenario |
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
| T-DOC-01 | docs | `TODO` | Dry-run examples execute |
| T-DOC-02 | docs | `TODO` | Send examples map to fake endpoint |
| T-DOC-03 | docs | `TODO` | Install examples execute |
| T-DOC-04 | docs | `TODO` | Original seven scenarios covered |
| T-DOC-05 | docs | `TODO` | README flags/defaults equal help |
| T-DOC-06 | docs | `TODO` | No secret/hidden switch in README |

### Docs

| Doc | Phase | Status | Notes |
|-----|-------|--------|-------|
| README.md | 0 | `DONE` | Build/help/version and no-send limitation |
| README.md | 1 | `TODO` | Input/config/JSON/basic dry-run |
| README.md | 2 | `TODO` | Formatting/chunking preview |
| README.md | 3 | `TODO` | Complete native send/retry/failure behavior |
| README.md | 4 | `TODO` | Docker/wrapper local use |
| README.md | 5 | `TODO` | Release/install/agentic/final guide |
| LICENSE | 5 | `TODO` | MIT, holder manprint |

### Audits

| Report | Date | Verdict | Open findings |
|--------|------|---------|---------------|
| none yet | - | - | - |
