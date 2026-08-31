# Phase 1 - Input, config, JSON, and offline CLI

> **Intent:** Deliver exact input/config validation and a credential-free basic dry-run CLI with stable JSON results.
> **Shippable alone?** yes - users can validate raw single-chunk notifications offline; network sending and rich formatting remain explicitly unavailable.
> **Preconditions:** phase 0 is `DONE` and all phase-0 gates pass.

## State contract (mandatory)

1. Read [STATE.md](STATE.md) first. If section 1 is `OPEN`, finish or revert that unit using section 6. Run section-3 gates and reconcile sections 1, 7, and 11 with repository truth.
2. Before editing, open this sub-phase in section 1 and set section 6 to `claimed - nothing written yet`.
3. After applicable gates pass, append the ledger row, update files/verification/deviations/blockers/board, reset section 6, and point section 1 to the next unit with `Status: none`.
4. If interrupted, retain `OPEN`, describe every partial edit in section 6, and follow the WIP-commit setting.

## External facts used

- **R4:** `github.com/pelletier/go-toml/v2` `v2.4.3` supplies structured TOML decoding: https://github.com/pelletier/go-toml/releases/tag/v2.4.3
- **R3:** Cobra `v1.10.2` supplies exact flag-changed detection and argument parsing: https://github.com/spf13/cobra/releases/tag/v1.10.2

## Sub-phases

### 1.1 Implement bounded, exact input acquisition

- **Model:** `agent-2:sonnet`
- **Assignment:** `agent-2:sonnet` implements input precedence and validation as an isolated pure/testable boundary.
- **Files:** `internal/input/source.go:1` (new), `internal/input/source_test.go:1` (new), `internal/apperr/error.go:1` (only if a planned code constant is missing). Create `internal/input` because no existing package owns source acquisition.
- **Change:** Implement `Source{Message string, MessageSet bool, Stdin io.Reader, StdinIsTerminal bool, MaxBytes int64}` and `Read(Source) (string,error)`. Reject `MaxBytes<=0` as `invalid_flag`. Determine `-m` presence from Cobra's changed bit, not `Message != ""`. If `MessageSet`, inspect non-terminal stdin for one byte without blocking on a TTY; any byte, including whitespace/NUL, produces `conflicting_input`, exit 2. Then validate the message bytes. Without `-m`, reject terminal stdin immediately; otherwise read through `io.LimitReader(max+1)` (guard `MaxInt64` overflow explicitly), classify read errors, reject zero bytes, reject over-limit bytes, and reject `!utf8.Valid`. Never trim, normalize newlines, replace invalid bytes, or append a newline. Return the exact Go string.
- **Unit tests:** `TestReadMessageExact` covers spaces, tabs, leading/trailing and final newline; `TestReadStdinExact` covers the same bytes; `TestReadConflictOnSingleWhitespaceByte`; `TestReadMessageDoesNotReadTerminal`; `TestReadRejectsEmptyMessage`; `TestReadRejectsEmptyPipe`; `TestReadRejectsTerminal`; `TestReadRejectsInvalidUTF8`; `TestReadRejectsLimitPlusOne`; `TestReadAcceptsExactlyLimit`; `TestReadRejectsNonPositiveLimit`; `TestReadClassifiesReaderFailure`; `FuzzReadPreservesValidUTF8WithinLimit` asserts returned bytes equal input.
- **e2e tests:** `T-IN-01` piped spaces/newlines are preserved in dry-run; `T-IN-02` `-m` plus one stdin byte is exit 2/`conflicting_input`; `T-IN-03` empty pipe is exit 4/`input_empty`; `T-IN-04` limit+1 produces exit 4 and no stdout.
- **Done:** all D5/D6 branches have named tests; no unbounded `io.ReadAll` exists; `make verify` passes; unit closed in `STATE.md` with section 1 pointing to 1.2 and affected test rows updated.

### 1.2 Implement strict TOML plus environment merge

- **Model:** `agent-2:sonnet`
- **Assignment:** `agent-2:sonnet` implements config parsing and merge; `agent-1:opus` reviews the boundary because it handles credentials.
- **Files:** `internal/config/config.go:1` (new), `internal/config/config_test.go:1` (new), `internal/testutil/fs.go:1` (new only if repeated safe temp-file setup warrants it). Create `internal/config` because no package owns secrets/configuration.
- **Change:** Define `Config{Token, ChatID string}` and `LoadOptions{Path string, Explicit bool, HomeDir func()(string,error), ReadFile func(string)([]byte,error), LookupEnv func(string)(string,bool)}`. Resolve default to `filepath.Join(home,".tgsend")`; never use repository cwd. Parse into `rawConfig{Token string, ChatID any}` with a `toml.Decoder` and strict unknown-field rejection. Accept `chat_id` only as TOML string or integer; normalize integer with base-10 formatting. Merge non-empty `TGSEND_TOKEN` and `TGSEND_CHAT_ID` independently after parsing. Rules: explicit missing/unreadable path always fails before environment merge; missing default behaves as an empty file so two environment values can satisfy it; malformed existing file always fails even if environment is complete; both merged values must be non-empty; `chat_id` must be either a nonzero signed decimal fitting int64 or `@` followed by ASCII letters/digits/underscore, with no whitespace. Never include token/file contents in errors. Do not read or test the repository `.tgsend`.
- **Unit tests:** `TestLoadTOMLStringChatID`; `TestLoadTOMLIntegerChatID`; `TestEnvironmentOverridesEachField`; `TestEnvironmentCompletesMissingDefault`; `TestExplicitMissingFailsWithCompleteEnvironment`; `TestMalformedExistingFileWinsOverEnvironment`; `TestUnknownKeyRejected`; `TestWrongTypesRejected`; `TestMissingTokenAndChatClassified`; `TestInvalidChatIDs` table; `TestReadErrorClassified`; `TestErrorsRedactToken` with token in file/env and simulated low-level error; `FuzzLoadNeverEchoesToken`.
- **e2e tests:** `T-CFG-01` temporary default `$HOME/.tgsend` is selected; `T-CFG-02` explicit `-c` wins; `T-CFG-03` env-only operation works when default is absent; `T-CFG-04` explicit missing path is exit 3 even with complete env; `T-CFG-05` unknown TOML key is exit 3 and token absent.
- **Done:** D4 is implemented exactly; temp files are owner-readable/writable in tests; no test opens repository `.tgsend`; `make verify` passes; unit closed in `STATE.md` with section 1 pointing to 1.3.

### 1.3 Freeze send/dry-run JSON schemas

- **Model:** `agent-2:sonnet`
- **Assignment:** `agent-2:sonnet` extends the presenter; `agent-1:opus` reviews the machine contract and fixtures.
- **Files:** `internal/presenter/presenter.go:1`, `internal/presenter/presenter_test.go:1`, `internal/presenter/testdata/send_success.json:1` (new), `internal/presenter/testdata/send_dry_run.json:1` (new), `internal/presenter/testdata/send_error.json:1` (new). Follow the established presenter package and add fixtures only because no existing fixture directory serves them.
- **Change:** Add `SendResult{DryRun bool, ChunksTotal int, ChunksSent int, MessageIDs []int64, Chunks []PreviewChunk}`. `PreviewChunk` is present only when dry-run and has `index` (1-based), `text`, `entities` (always array), and `disable_notification`; never include token or chat ID. A successful real send has `chunks_sent==chunks_total`, message IDs in order, and omits `chunks`; dry-run has `chunks_sent=0`, empty message IDs, and all preview chunks. Extend error envelope with optional `progress{chunks_total,chunks_sent,failed_chunk}` only after a plan exists; preserve required `schema_version:"1"`, `ok:false`, `command:"send"`, and `error{code,message,retryable}`. Use arrays rather than null. Build structs, never maps, to stabilize field order; tests compare decoded semantics and golden bytes including the trailing newline.
- **Unit tests:** `TestSendSuccessGolden`; `TestDryRunGolden`; `TestSendErrorWithoutProgressGolden`; `TestSendErrorWithProgressGolden`; `TestArraysAreNeverNull`; `TestRealSuccessOmitsMessageBodies`; `TestPreviewOmitsCredentials`; `TestExactlyOneResultOrError`; `TestAllKnownErrorCodesSerialize`.
- **e2e tests:** `T-JSON-01` dry-run emits one stdout document and empty stderr; `T-JSON-02` each validation category emits one stderr document, empty stdout, and its D21 exit code; `T-JSON-03` sentinel token/chat ID never appears in dry-run or error output.
- **Done:** schema examples and Go fixtures agree byte-for-byte; no body is echoed after real sends; D12/D21/I-5/I-7 hold; `make verify` passes; unit closed in `STATE.md` with section 1 pointing to 1.4.

### 1.4 Wire basic offline CLI and application service

- **Model:** `agent-2:sonnet`
- **Assignment:** `agent-2:sonnet` implements orchestration and compiled acceptance; `agent-1:opus` reviews acceptance behavior and dependency direction.
- **Files:** `internal/message/types.go:1` (new), `internal/message/basic.go:1` (new), `internal/message/basic_test.go:1` (new), `internal/app/service.go:1` (new), `internal/app/service_test.go:1` (new), `internal/cli/root.go:1`, `internal/cli/root_test.go:1`, `cmd/tgsend/main.go:1`, `test/e2e/input_config_test.go:1` (new). Create `message` and `app` packages because no existing location owns request plans or use-case ordering.
- **Change:** Define shared immutable `message.Entity{Type string,Offset,Length int}` and `message.Chunk{Text string,Entities []Entity,DisableNotification bool}`. Add a temporary phase-1 `BasicPlanner.Plan(body string,silent bool)` that accepts raw text only when UTF-16 length <=4096 and returns one chunk with a non-nil empty entity slice; phase 2 replaces this implementation without changing types or raw-message bytes. Define `app.Options{Message,MessageSet,ConfigPath,ConfigExplicit,Silent,DryRun,MaxInputBytes}` and `Service` dependencies for input, config, planner, and sender. In this phase, support only `--dry-run`; a non-dry invocation returns safe `telegram_transport`/exit 6 message `sending is not available in this build phase` without attempting network. Execution order is input validation, basic plan, dry-run return; dry-run must not call config. Register `-m/--message`, `-c/--config`, `--silent`, `--dry-run`, `--max-input-bytes` with exact defaults. Use Cobra flag `Changed` for source/config explicitness. Make `Execute` the only stream writer through presenter.
- **Unit tests:** `TestBasicPlannerPreservesRawText`; `TestBasicPlannerCountsAstralUTF16`; `TestDryRunSkipsConfig`; `TestDryRunCallsPlannerOnce`; `TestApplicationValidationOrder`; `TestNonDryDoesNotAttemptNetwork`; `TestCLIFlagDefaults`; `TestCLIChangedBits`; `TestAppErrorsReachCorrectStreamAndExit`.
- **e2e tests:** execute `T-IN-01..04`, `T-CFG-01..05`, and `T-JSON-01..03` against the compiled binary; use dry-run for config-independent cases and injectable service tests for config until real send exists. Add `T-DRY-01`: with nonexistent HOME, invalid explicit config, unset credentials, and a network sentinel, `--dry-run -m Hello` succeeds and returns exactly one preview chunk.
- **Done:** basic dry-run is usable without credentials/network, all registered flags have exact help/defaults, no real request is possible, raw <=4096 behavior satisfies I-1/I-6/I-7, `make verify` passes, and the unit is closed in `STATE.md` with section 1 pointing to 1.5.

### 1.5 Update README.md

- **Model:** `agent-3:haiku`
- **Assignment:** `agent-3:haiku` updates user documentation for phase-1 shipped behavior only.
- **Files:** `README.md:1`.
- **Change:** Preserve the existing structure. Update `Overview`, `Usage`, `Configuration`, `Environment`, `JSON output`, `Dry run`, `Exit codes`, `Limits`, and `Troubleshooting`. Document only basic raw single-chunk dry-run plus source/config validation; show stdin and `-m`, conflict behavior, TOML schema with placeholder token, env precedence, default path, 1 MiB limit, and representative success/error JSON without real values. State clearly that live sending and formatting are not yet shipped in this phase. Do not mention internal symbols/files/algorithms or future implementation details.
- **Unit tests:** none (documentation); run every dry-run example in an isolated temporary HOME and validate JSON with a parser when available.
- **e2e tests:** none - examples map to `T-DRY-01`, `T-IN-01`, `T-CFG-03`, and `T-JSON-02`.
- **Done:** a new user can configure and exercise offline validation from README alone, no secret is realistic/reusable, known limitations are accurate, `make verify` passes, and the unit closes with phase-1 docs row done and section 1 pointing to phase 2 sub-phase 2.1.

## Phase gates

- **Build:** `make build`
- **Fmt:** `make fmt-check`
- **Lint:** `make lint`
- **Test subset:** `make test && make test-e2e && make vuln`
- **Regression guard:** all `T-CLI-*`, `T-IN-*`, `T-CFG-*`, `T-JSON-*`, `T-DRY-01`
- **README:** source/config/JSON/dry-run behavior and current no-send/no-format limitations are exact.

## Phase done criterion

The compiled binary validates exact input and strict merged config, emits stable one-document JSON, and can preview one raw <=4096-unit request offline without reading config. All failures use the specified stream/code, all phase tests pass under `make verify`, README documents only shipped behavior, and `STATE.md` section 11 shows phase 1 `DONE` with every unit closed.
