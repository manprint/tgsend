# Phase 3 - Telegram transport and send orchestration

> **Intent:** Implement production `sendMessage`, bounded 429 handling, ordered multi-chunk sends, and complete native CLI behavior.
> **Shippable alone?** yes - this phase completes the native sender; Docker and release automation are separate delivery layers.
> **Preconditions:** phase 2 is `DONE`; the immutable message plan is the sole request source.

## State contract (mandatory)

1. Read [STATE.md](STATE.md) first. Finish/revert any `OPEN` unit from section 6, run section-3 gates, and reconcile sections 1, 7, and 11 with the repository.
2. Open the selected sub-phase in section 1 before any edit; set section 6 to `claimed - nothing written yet`.
3. Close after gates pass by updating sections 4-11, resetting section 6, and pointing section 1 to the next unit with `Status: none`.
4. If interrupted, retain `OPEN`, record exact partial state, and obey the WIP-commit setting.

## External facts used

- **R1:** Telegram `sendMessage` request/response fields and `ResponseParameters.retry_after`: https://core.telegram.org/bots/api#sendmessage
- **R2:** Entity offset/length units are UTF-16: https://core.telegram.org/bots/api#messageentity
- **D11:** Telegram offers no idempotency key for this operation; only explicit 429 responses are retried.

## Fixed transport contract

- POST JSON to `<base>/bot<token>/sendMessage`; production base is `https://api.telegram.org`.
- Request keys: `chat_id`, `text`, optional `entities`, `disable_notification`; never set `parse_mode`.
- A 10-second `http.Client.Timeout` applies to each attempt. The caller context may end sooner.
- Initial attempt plus at most two retries. Retry only when HTTP status or decoded API `error_code` is 429 and `parameters.retry_after` is a positive integer.
- Refuse a retry that would make cumulative requested sleep exceed 60 seconds. Context cancellation during sleep stops immediately.
- Never retry transport errors, timeouts, malformed responses, 5xx, or non-429 Telegram errors.

## Sub-phases

### 3.1 Implement one-attempt Bot API client

- **Model:** `agent-2:sonnet`
- **Assignment:** `agent-2:sonnet` implements and tests the transport boundary; `agent-1:opus` reviews protocol and redaction.
- **Files:** `internal/telegram/client.go:1` (new), `internal/telegram/client_test.go:1` (new), `internal/telegram/types.go:1` (new only if it keeps wire structs separate). Create `internal/telegram` because no package owns Telegram I/O.
- **Change:** Define minimal `Doer{Do(*http.Request)(*http.Response,error)}` and `Client` created by `NewClient(Options)` with token, validated base URL, `Doer`, and retry dependencies added in 3.2. Define private request structs and response `{OK bool, Result{MessageID int64}, ErrorCode int, Description string, Parameters{RetryAfter int}}`. Serialize a `message.Chunk` without modification; omit `entities` only when empty. Build the token-bearing URL locally but never retain it in returned errors. Use `http.NewRequestWithContext`, `Content-Type: application/json`, and a response-body limit of 1 MiB. Always close response bodies. Treat non-2xx, `ok:false`, missing result/message ID, trailing JSON, oversized body, or malformed JSON as typed safe errors. Map non-429 API rejection to `telegram_rejected`/5 and transport/protocol failures to exit 6. Descriptions may be exposed only after replacing any exact token occurrence; safer default is stable local text plus Telegram numeric code.
- **Unit tests:** `TestSendRequestExactJSON` checks method/path/content type/body and absence of parse mode; `TestSendPreservesTextAndEntities`; `TestSendSuccessMessageID`; `TestSendTelegramRejection`; `TestSendHTTP5xxNotSuccess`; `TestSendMalformedAndTrailingJSON`; `TestSendOversizedResponse`; `TestSendClosesBody`; `TestSendContextCancellation`; `TestTokenAbsentFromEveryError` uses token in URL/cause/description; `TestBaseURLValidation` rejects non-HTTP(S) and production credentials in query/userinfo.
- **e2e tests:** `T-TG-01` fake endpoint receives exact single-chunk request and success ID; `T-TG-02` API rejection yields exit 5 and one stderr JSON; `T-TG-03` malformed/5xx/connection failure each yields exit 6 and exactly one request.
- **Done:** wire JSON matches R1 and planner entities unchanged, response resource bounds hold, token redaction is proven, `make verify` passes, and the unit closes with section 1 pointing to 3.2.

### 3.2 Add bounded 429 retry policy

- **Model:** `agent-2:sonnet`
- **Assignment:** `agent-2:sonnet` implements retry lifecycle; `agent-1:opus` reviews because duplicate notifications are a high-impact failure.
- **Files:** `internal/telegram/retry.go:1` (new), `internal/telegram/retry_test.go:1` (new), `internal/telegram/client.go:1`, `internal/telegram/client_test.go:1`.
- **Change:** Define `Sleeper{Sleep(context.Context,time.Duration) error}` with production implementation using a timer/select, and options `MaxRetries=2`, `MaxCumulativeWait=60s`. Make one private `sendAttempt`; public `Send` loops only on a structured 429 result. Accept 429 signaled by HTTP 429 or decodable Telegram `error_code:429`. Require positive `retry_after`; convert seconds with overflow checks. Before sleeping, fail as `telegram_rate_limited`/7 with `Retryable:false` when retry count or cumulative budget would be exceeded; otherwise sleep exactly the requested duration and retry the same chunk once. If context ends while sleeping, return `telegram_transport`/6 and do not issue another request. Mark rate-limit errors `Retryable:false` in final CLI JSON because the process has exhausted its own policy; intermediate errors are not emitted.
- **Unit tests:** `Test429ThenSuccessSleepsAndRetries`; `TestTwoRetriesThenSuccess`; `TestThird429StopsAtThreeAttempts`; `TestRetryAfterExceedingBudgetDoesNotSleep`; `TestCumulativeWaitExceeding60Stops`; `TestMissingOrZeroRetryAfterDoesNotRetry`; `TestHTTP429AndAPI429Equivalent`; `Test5xxNeverRetries`; `TestTransportNeverRetries`; `TestTimeoutNeverRetries`; `TestMalformedNeverRetries`; `TestContextCancelledDuringSleepDoesNotRetry`; `TestRetryResendsOnlyCurrentChunk`; assert exact attempt/sleep counts in every case.
- **e2e tests:** `T-TG-04` fake server returns 429/1 then success: two requests and injected sleeper receives 1s; `T-TG-05` three 429 responses: exit 7, three requests, two sleeps; `T-TG-06` 500 and dropped connection each produce one request/no retry.
- **Done:** only explicit, bounded 429 paths can loop; every no-retry class has an attempt-count assertion; I-4 holds; `make verify` passes; unit closes with section 1 pointing to 3.3.

### 3.3 Complete application send loop and process-level behavior

- **Model:** `agent-2:sonnet`
- **Assignment:** `agent-2:sonnet` replaces the phase-1 no-send branch and owns process acceptance; `agent-1:opus` reviews ordering, partial failure, and hidden test endpoint controls.
- **Files:** `internal/app/service.go:1`, `internal/app/service_test.go:1`, `internal/cli/root.go:1`, `internal/cli/root_test.go:1`, `internal/buildinfo/buildinfo.go:1`, `cmd/tgsend/main.go:1`, `test/e2e/server_test.go:1` (new), `test/e2e/send_test.go:1` (new), `test/e2e/main_test.go:1`, `internal/presenter/presenter.go:1`.
- **Change:** Define app interfaces `ConfigLoader`, `Planner`, and `Sender{Send(ctx,config,chunk)(messageID,error)}`. Final execution order: parse flags; acquire/validate complete input; build/validate every chunk; if dry-run return preview; load/merge config; send chunks serially by index; collect IDs. On failure stop immediately and attach `Progress{total,sent,index+1}` without altering underlying kind/code. Successful send result has ordered IDs and no body preview. Remove the phase-1 unavailable-sender branch. Main constructs `http.Client{Timeout:10s}`, production sleeper, loader/planner/client/service. Add build string `TestEndpointEnabled="false"`; e2e builds set it to `true` with `-ldflags -X` and may then supply `TGSEND_API_BASE_URL` pointing to loopback HTTP. Production/default binaries reject that environment variable as `invalid_arguments`; never document it as user functionality. E2E server records decoded requests and scripts responses without real credentials.
- **Unit tests:** `TestServiceValidationBeforeConfig`; `TestServicePlanBeforeConfig`; `TestDryRunSkipsConfigAndSender`; `TestPlanFailureSendsNothing`; `TestSequentialSendOrder`; `TestStopAtFirstFailure`; `TestPartialProgressZeroFirstMiddleLast`; `TestSuccessIDsInOrder`; `TestConfigNotLoadedUntilPlanComplete`; `TestProductionRejectsTestBaseURL`; `TestTestBuildAcceptsLoopbackOnly`; `TestNoConcurrentSend` uses a blocking sender to prove max in-flight one.
- **e2e tests:** `T-TG-07` env-only credentials send one request successfully; `T-TG-08` multi-chunk sends serially and outputs ordered IDs; `T-TG-09` chunk 2 permanent failure stops at two requests and reports total/sent=1/failed=2; `T-E2E-08` reference scenario validates long Unicode title/type/monospace request sequence, first-only header, UTF-16 entities, exact body reconstruction, and one stdout success JSON; repeat `T-TG-02..06` through process where sleep duration is zeroed only by injected test sleeper in app-level tests, not production binary.
- **Done:** live native send behavior replaces only the explicit phase-1 limitation; all validation is complete before request 1, sends are serial, partial progress is exact, default build cannot redirect credentials, `make verify` passes, and the unit closes with section 1 pointing to 3.4.

### 3.4 Update README.md

- **Model:** `agent-3:haiku`
- **Assignment:** `agent-3:haiku` documents the now-complete native sender; no implementation internals.
- **Files:** `README.md:1`.
- **Change:** Preserve structure and update `Overview`, `Installation from source`, `Usage`, `Flags`, `Configuration`, `Environment`, `Formatting`, `Long messages`, `JSON output`, `Exit codes`, `Retries and failures`, `Security`, `Limits`, and `Troubleshooting`. Remove the no-send limitation. Include all seven original scenarios in executable form, plus `--silent` and `--dry-run`. Explain 10s per-attempt timeout, 429-only retries (2, 60s cumulative), stop-on-first-failure, partial-progress JSON, and no retry for ambiguous failures. Show placeholders only. Add a manual real-Telegram smoke procedure requiring the user to set env locally; never suggest CI secrets or print commands that echo tokens.
- **Unit tests:** none (documentation); execute non-live examples against the compiled fake endpoint harness where applicable.
- **e2e tests:** none - examples map to `T-TG-07`, `T-TG-08`, `T-TG-09`, and `T-E2E-08`; manual live smoke remains explicitly non-CI.
- **Done:** a new user can configure and send safely from README alone, all failure semantics/limits are discoverable, no hidden test endpoint or internal detail is mentioned, `make verify` passes, and the unit closes with phase-3 docs row done and section 1 pointing to phase 4 sub-phase 4.1.

## Phase gates

- **Build:** `make build`
- **Fmt:** `make fmt-check`
- **Lint:** `make lint`
- **Test subset:** `make test && make test-e2e && make vuln`
- **Regression guard:** every prior ID plus `T-TG-01..09` and `T-E2E-08`
- **README:** full native CLI usage, retry/partial-send behavior, security, limits, and manual smoke.

## Phase done criterion

A compiled production binary sends exact planned chunks serially to Telegram, retries only bounded explicit 429 responses, reports ordered success or precise partial failure, and cannot leak or redirect credentials through the test seam. `make verify` passes, README fully describes native behavior, and `STATE.md` section 11 marks phase 3 `DONE` with every unit closed.
