# Phase 0 - Foundation and contracts

> **Intent:** Establish a compiling Go module, stable error/build contracts, deterministic tool gates, and a compiled-binary test harness without implementing Telegram sends.
> **Shippable alone?** yes - it produces a tested CLI skeleton with help and JSON version output; no network behavior is claimed.
> **Preconditions:** none

## State contract (mandatory)

1. Before touching anything, read [STATE.md](STATE.md). If section 1 `Status` is `OPEN`, finish or revert that unit first using section 6. Run all applicable commands in section 3 and reconcile sections 1, 7, and 11 with the repository.
2. Open this sub-phase in `STATE.md` section 1 before editing: set type, ID, `Status: OPEN`, intent, next action, assignment, and section 6 to `claimed - nothing written yet`.
3. Close only after gates are green: append section 4, update sections 5 and 7-11, reset section 6 to `none - tree consistent`, and point section 1 to the next unit with `Status: none`. Commit only if section 3 enables WIP commits.
4. If interrupted, leave the unit `OPEN` and record exact completed/pending edits in section 6; create the prescribed `wip(<id>)` commit only when WIP commits are enabled.

## External facts used

- **R3:** Cobra `v1.10.2` is the pinned CLI framework: https://github.com/spf13/cobra/releases/tag/v1.10.2
- **R8:** golangci-lint `v2.13.2` is the pinned lint tool: https://github.com/golangci/golangci-lint/releases/tag/v2.13.2
- **R9:** govulncheck is installed from `golang.org/x/vuln` `v1.1.4`: https://github.com/golang/vuln/releases/tag/v1.1.4
- **R10:** The repository toolchain is Go `1.27.0`: https://go.dev/doc/devel/release

## Sub-phases

### 0.1 Initialize module, entrypoint, and build metadata

- **Model:** `agent-3:haiku`
- **Assignment:** `agent-3:haiku` creates the mechanical scaffold; `agent-1:opus` reviews package boundaries and public contracts before close.
- **Files:** `go.mod:1` (new), `go.sum:1` (generated), `cmd/tgsend/main.go:1` (new), `internal/buildinfo/buildinfo.go:1` (new), `internal/buildinfo/buildinfo_test.go:1` (new), `Makefile:1` (new), `.gitignore:1` (extend without removing `.tgsend`). Follow the repository's root/internal/cmd conventions; create these directories because no existing implementation directory serves them.
- **Change:** Declare module `github.com/manprint/tgsend`, `go 1.27.0`, direct Cobra `v1.10.2`, and later-needed TOML `v2.4.3`; run `go mod tidy`. Implement `buildinfo.Info{Version, Commit, Date string}` and `Current() Info` backed by package string variables defaulting to `dev`, `none`, `unknown` so GoReleaser can set them with `-ldflags -X`. Keep `main()` limited to constructing default dependencies, calling `cli.Execute`, and `os.Exit`; until 0.3 exists it may call a minimal compile-safe function in the same file, which must be removed in 0.3. Add Make targets exactly: `build`, `fmt`, `fmt-check`, `lint`, `test`, `test-e2e`, `vuln`, `verify`; use `go build ./cmd/tgsend`, a failing `gofmt -l` check, `go vet ./...`, pinned golangci-lint, `go test -race ./...`, `go test -tags=e2e ./...`, pinned govulncheck, and aggregate non-Docker checks. Ignore `/bin/`, `/dist/`, `/coverage.out` while preserving `.tgsend`.
- **Unit tests:** `TestCurrentDefaults` asserts exactly `dev/none/unknown`; `TestCurrentReflectsLinkVariables` saves/restores package variables and proves all three fields map correctly.
- **e2e tests:** none (entrypoint is not behavior-complete yet).
- **Done:** `go mod tidy` leaves no diff; `make build`, `make fmt-check`, `make lint`, `make test`, `make test-e2e`, and `make vuln` pass; `.tgsend` remains ignored; package dependency direction is `cmd -> internal` only; unit closed in `STATE.md` with section 1 pointing to 0.2, ledger/files/gates updated, section 6 reset, and board updated.

### 0.2 Define typed application errors and exit taxonomy

- **Model:** `agent-3:haiku`
- **Assignment:** `agent-3:haiku` implements the closed taxonomy exactly; `agent-1:opus` reviews because it is a cross-package protocol.
- **Files:** `internal/apperr/error.go:1` (new), `internal/apperr/error_test.go:1` (new). Create `internal/apperr` because no existing package owns process-level failures.
- **Change:** Define `Kind` constants for `usage`, `config`, `input`, `telegram`, `transport`, and `rate_limit`; map them to exit codes 2-7 respectively. Define stable symbolic `Code` constants: `invalid_arguments`, `conflicting_input`, `invalid_flag`, `config_not_found`, `config_unreadable`, `config_invalid`, `config_incomplete`, `input_empty`, `input_unreadable`, `input_too_large`, `input_invalid_utf8`, `telegram_rejected`, `telegram_transport`, `telegram_protocol`, and `telegram_rate_limited`. Implement `Error` with unexported cause plus exported safe `Code`, `Kind`, `Message`, and optional `Progress{ChunksTotal, ChunksSent, FailedChunk}`; `Error()` returns only the safe message, `Unwrap()` returns the cause, and `ExitCode(error)` returns 1 for unknown errors. Add constructors that require callers to supply a safe message; never derive user output from a URL/request containing a token.
- **Unit tests:** `TestExitCodeByKind` covers every kind and unknown/nil; `TestErrorUnwrapsCause` proves `errors.Is`; `TestErrorStringUsesSafeMessage` places a sentinel token in the cause and asserts it is absent from `Error()`; `TestProgressUsesOneBasedFailedChunk` verifies validation rejects negative counts, sent > total, and failed indexes outside `1..total`.
- **e2e tests:** none (internal contract only).
- **Done:** all constants and numeric exit codes match D21; no constructor formats a wrapped cause into the safe message; `make verify` passes; unit closed in `STATE.md` with section 1 pointing to 0.3 and all close fields updated.

### 0.3 Implement Cobra root, JSON version output, and stream discipline

- **Model:** `agent-3:haiku`
- **Assignment:** `agent-3:haiku` implements the small CLI/presenter surface; `agent-1:opus` reviews the JSON contract.
- **Files:** `internal/presenter/presenter.go:1` (new), `internal/presenter/presenter_test.go:1` (new), `internal/cli/root.go:1` (new), `internal/cli/root_test.go:1` (new), `cmd/tgsend/main.go:1` (replace temporary scaffold). Create packages because no current package owns output or CLI parsing.
- **Change:** Define envelope fields in this order for deterministic fixtures: `schema_version`, `ok`, `command`, then `result` or `error`. Implement `VersionResult{Version,Commit,Date}` and `ErrorBody{Code,Message,Retryable}`; omit no required field and terminate every document with one newline using one `json.Encoder.Encode` call. Implement `cli.Dependencies{Stdin,Stdout,Stderr,BuildInfo}` and `NewRoot`; configure Cobra with `SilenceErrors=true`, `SilenceUsage=true`, no positional args, textual help to stdout, and no direct process exits. Implement `--version` as JSON command `version`; it must bypass input/config. Implement `Execute(ctx,deps,args) int` to write one safe JSON error to stderr and return `apperr.ExitCode`; unknown errors become `internal_error`, exit 1. Main passes `os.Args[1:]` and real streams. R3 applies; do not let Cobra print an additional error/usage line.
- **Unit tests:** `TestVersionEnvelope` compares decoded JSON and asserts stdout only plus trailing newline; `TestVersionLinkMetadata` injects non-default values; `TestHelpIsText` asserts usage on stdout and empty stderr; `TestUnknownFlagIsSingleJSONError` asserts exit 2, empty stdout, one decodable stderr object, and no `Usage:`; `TestPositionalArgumentRejected` asserts `invalid_arguments`/2; `TestPresenterNeverSerializesCause` embeds a token in a cause and searches both streams.
- **e2e tests:** `T-CLI-01` - `go run ./cmd/tgsend --version` exits 0 and emits exactly one version envelope; `T-CLI-02` - `go run ./cmd/tgsend --help` exits 0 with textual help and no JSON requirement.
- **Done:** JSON key/stream/exit behavior matches D12 and D21; no package except `presenter` encodes user-facing JSON; `make verify` passes; unit closed in `STATE.md` with section 1 pointing to 0.4 and board/tests updated.

### 0.4 Add compiled-binary e2e harness and acceptance fixtures

- **Model:** `agent-2:sonnet`
- **Assignment:** `agent-2:sonnet` owns process-level harness correctness; `agent-1:opus` reviews acceptance assertions and secret isolation.
- **Files:** `test/e2e/main_test.go:1` (new), `test/e2e/harness_test.go:1` (new), `test/e2e/cli_test.go:1` (new), `internal/testutil/json.go:1` (new). Create `test/e2e` because no compiled-binary harness exists; keep general helpers internal to tests.
- **Change:** Under build tag `e2e`, use `TestMain` to build one fresh temporary `tgsend` binary with `go build -o <temp> ./cmd/tgsend`, then remove it. Provide a `run` helper with explicit args, stdin bytes, environment, timeout, captured stdout/stderr, and exit code; start from a minimal controlled environment with temporary `HOME`, never the developer home and never repository `.tgsend`. Provide JSON decoders that require exactly one document plus EOF and preserve raw bytes for newline assertions. Port `T-CLI-01/02` to the compiled binary; do not use shell parsing in tests.
- **Unit tests:** `TestDecodeOneJSONRejectsConcatenatedDocuments` and `TestDecodeOneJSONRequiresTrailingNewline`; harness helper tests cover exit-code extraction and deadline cancellation.
- **e2e tests:** `T-CLI-01` compiled `--version` contract; `T-CLI-02` compiled textual help; `T-CLI-03` unknown flag yields only stderr JSON, exit 2, and no token-like environment value appears.
- **Done:** `make test-e2e` always builds a fresh temporary executable, passes from any working directory, cannot discover the real `.tgsend`, and leaves no artifact; `make verify` passes; unit closed in `STATE.md` with section 1 pointing to 0.5 and all test rows reconciled.

### 0.5 Update README.md

- **Model:** `agent-3:haiku`
- **Assignment:** `agent-3:haiku` creates the initial user guide for shipped phase-0 behavior only.
- **Files:** `README.md:1` (new). Create at repository root because no README exists.
- **Change:** Add only `Overview`, `Requirements`, `Build from source`, `Version`, `Help`, and `Current limitations`. State that this phase provides the CLI skeleton and does not send messages yet. Show `make build`, `./bin/tgsend --version`, and help with representative output shape. Do not document future flags, Telegram internals, package layout, algorithms, phases, or roadmap. Use formal language and preserve this structure for later incremental edits.
- **Unit tests:** none (documentation); execute every command block that is valid in this phase.
- **e2e tests:** none - `T-CLI-01` and `T-CLI-02` already execute the documented behavior.
- **Done:** a new contributor can build and inspect the current CLI from README alone, limitations are accurate, links/commands work, no unshipped behavior appears, `make verify` passes, and the unit is closed in `STATE.md` with phase-0 docs and phase rows updated and section 1 pointing to phase 1 sub-phase 1.1.

## Phase gates

- **Build:** `make build`
- **Fmt:** `make fmt-check`
- **Lint:** `make lint`
- **Test subset:** `make test && make test-e2e && make vuln`
- **Regression guard:** `T-CLI-01`, `T-CLI-02`, `T-CLI-03`
- **README:** documents only the buildable skeleton, help, version JSON, and explicit no-send limitation.

## Phase done criterion

A clean checkout can run `make verify`, build `tgsend`, obtain textual help and the exact JSON version envelope from a compiled binary, and cannot accidentally read the developer's config during tests. README reflects only this shipped behavior, and `STATE.md` section 11 shows phase 0 `DONE` with every sub-phase closed.
