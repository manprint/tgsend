# Phase 2 - Message composition and chunking

> **Intent:** Replace the basic planner with a Unicode-correct, fully validated request planner for titles, severities, monospace entities, and long bodies.
> **Shippable alone?** yes - dry-run exposes the exact final Telegram requests offline; native sending remains disabled until phase 3.
> **Preconditions:** phase 1 is `DONE`; phase-1 raw dry-run remains byte-identical for inputs fitting one chunk.

## State contract (mandatory)

1. Read [STATE.md](STATE.md) first. Resolve an `OPEN` unit from section 6, run section-3 gates, and correct any mismatch in sections 1, 7, or 11 before new work.
2. Open the selected sub-phase in section 1 before editing and set section 6 to `claimed - nothing written yet`.
3. Close only after all applicable gates pass and sections 4-11 are updated; section 1 must point to the next unit with `Status: none`.
4. On interruption, leave `OPEN`, record exact partial work in section 6, and follow the WIP-commit policy.

## External facts used

- **R1:** `sendMessage` accepts text up to 4096 characters after entity parsing: https://core.telegram.org/bots/api#sendmessage
- **R2:** `MessageEntity.offset` and `.length` are measured in UTF-16 code units: https://core.telegram.org/bots/api#messageentity

## Fixed composition contract

- A type line is `<configured code points><space><UPPERCASE TYPE>`.
- A title line is the exact `--title` string and receives one `bold` entity.
- If both exist, type line precedes title and they are joined by one LF.
- A non-empty header is followed by exactly two LF before the first body byte.
- Header and separator occur only in request 1. No chunk indicator is added.
- A `pre` entity covers only each request's body segment. Type/title and body entities never overlap.
- Header plus separator must use at most 4094 UTF-16 units, reserving two units so any valid first rune can fit; otherwise return `invalid_flag` with safe code `title_too_long` added to the known-code set.

## Sub-phases

### 2.1 Implement UTF-16 boundary primitives

- **Model:** `agent-2:sonnet`
- **Assignment:** `agent-2:sonnet` implements pure Unicode primitives with exhaustive table/fuzz tests.
- **Files:** `internal/message/utf16.go:1` (new), `internal/message/utf16_test.go:1` (new). Keep helpers unexported unless another internal package demonstrably needs them.
- **Change:** Implement `utf16Len(string) int` by decoding runes and counting one unit for BMP runes and two for code points above `U+FFFF`; input is already valid UTF-8, but helper tests must define invalid-input behavior as an error rather than replacement. Implement `prefixWithin(s,budget) (byteEnd,units,error)` returning the largest complete-rune byte prefix not exceeding a nonnegative UTF-16 budget. Implement `utf16Offset(s,byteIndex)` that rejects non-rune boundaries. Use checked integer addition in planner-facing helpers to avoid overflow. Do not split combining sequences specially: Telegram limits are code-unit based and I-1 forbids normalization; only rune boundaries are mandatory.
- **Unit tests:** `TestUTF16LenASCII`; `TestUTF16LenBMP`; `TestUTF16LenAstral`; `TestUTF16LenCombiningSequence`; `TestPrefixWithinNeverSplitsRune`; `TestPrefixWithinZero`; `TestOffsetRejectsMidRune`; `TestInvalidUTF8Rejected`; `TestCheckedAddOverflow`; `FuzzPrefixWithin` asserts valid UTF-8, `units<=budget`, maximality, and `prefix+suffix==input`.
- **e2e tests:** none (pure primitive; planner e2e follows).
- **Done:** helpers pass race/fuzz seed tests and have no byte/rune-count substitutions for UTF-16; `make verify` passes; unit closes with section 1 pointing to 2.2 and test rows updated.

### 2.2 Implement deterministic newline-preferred body splitting

- **Model:** `agent-2:sonnet`
- **Assignment:** `agent-2:sonnet` owns the correctness-sensitive splitter; `agent-1:opus` reviews algorithm and invariant tests.
- **Files:** `internal/message/split.go:1` (new), `internal/message/split_test.go:1` (new), `internal/message/testdata/split_cases.json:1` (new only for readable boundary fixtures).
- **Change:** Implement `splitBody(body string, firstBudget, laterBudget int) ([]string,error)`. Require non-empty valid UTF-8 body and positive budgets. For each segment, find the largest complete-rune prefix within the current budget. If more input remains, search that prefix for the last LF and cut immediately after it, including the LF in the current segment; ignore a zero-length candidate and otherwise prefer it even if substantially earlier than the hard limit. If no LF fits, use the maximal rune prefix. Use `firstBudget` once, `laterBudget` thereafter. Guarantee progress or return an internal error; return non-nil slices. Do not trim CRLF: the LF boundary includes both bytes when contiguous. Rejoin all segments in tests and compare bytes exactly.
- **Unit tests:** `TestSplitNoopWithinBudget`; `TestSplitAtLastFittingLF`; `TestSplitPreservesCRLF`; `TestSplitFallsBackAtRuneBoundary`; `TestSplitAstralBudget`; `TestSplitConsecutiveAndLeadingLF`; `TestSplitFinalLF`; `TestSplitUsesFirstBudgetOnce`; `TestSplitRejectsImpossibleBudget`; `TestSplitEveryChunkWithinBudget`; `T-MSG-07` table covering ASCII/BMP/astral/combining/CRLF asserts concatenation byte equality and each UTF-16 bound; `FuzzSplitBodyPreservesInputAndBounds` asserts I-1/I-2/progress.
- **e2e tests:** `T-MSG-01` dry-run of >4096 ASCII splits at last fitting LF; `T-MSG-02` no-newline astral input splits without invalid UTF-8; `T-MSG-03` no generated chunk labels occur.
- **Done:** splitter is deterministic, linear enough for the 1 MiB cap (no whole-remainder rescans from the start), and proves I-1/I-2; `make verify` passes; unit closes with section 1 pointing to 2.3.

### 2.3 Implement final composer/planner and formatting CLI

- **Model:** `agent-2:sonnet`
- **Assignment:** `agent-2:sonnet` replaces `BasicPlanner`; `agent-1:opus` reviews entity math, raw-path regression, and acceptance assertions.
- **Files:** `internal/message/planner.go:1` (new), `internal/message/planner_test.go:1` (new), `internal/message/basic.go:1` (delete after references migrate), `internal/message/basic_test.go:1` (migrate/delete), `internal/app/service.go:1`, `internal/app/service_test.go:1`, `internal/cli/root.go:1`, `internal/cli/root_test.go:1`, `internal/presenter/testdata/send_dry_run.json:1`, `test/e2e/message_test.go:1` (new), `internal/apperr/error.go:1` (add `title_too_long`). Follow established packages and do not create a second planner model.
- **Change:** Define `message.Options{Title,Type string; Monospace,Silent bool}` and `Planner.Plan(body string,Options) ([]Chunk,error)`. Normalize type case-insensitively to the four D8 values and map exact code-point sequences; reject all others before planning. Validate title/type strings as UTF-8. Build header and locate title byte range, then convert title offset/length to UTF-16. Enforce the 4094-unit header+separator maximum. Compute first body budget as `4096-utf16Len(headerWithSeparator)` and later budget 4096; call `splitBody`. For each body segment compose text, add title `bold` only in chunk 1 when non-empty, add body `pre` when `Monospace`, set all entity offsets/lengths in UTF-16, sort entities by offset then length, and validate every entity lies within text. Keep entities non-nil. Extend app options and Cobra with `--title`, `--monospace`, and `--type`. The phase-1 raw, no-option, <=4096 path must produce identical preview bytes. Dry-run serializes all exact chunks and remains config/network free.
- **Unit tests:** `TestHeaderShapes` covers title/type/both/neither exact LF layout; `TestSeverityCaseNormalizationAndCodePoints`; `TestUnknownSeverityRejected`; `TestTitleBoldUTF16Offset`; `TestPreCoversBodyOnly`; `TestEntitiesDoNotOverlap`; `TestHeaderOnlyFirstChunk`; `TestHeaderReservesBodyRune`; `TestTitleTooLongRejected`; `TestPlannerNoChunkLabels`; `TestPlannerRawPathMatchesPhase1Golden`; `TestPlannerPlansEntireInputBeforeReturn`; `TestEveryEntityInBounds`; fuzz `FuzzPlannerInvariants` asserts I-1/I-2 and deterministic repeat output.
- **e2e tests:** execute `T-MSG-01..03`; `T-MSG-04` exact title-only preview; `T-MSG-05` all severity inputs normalize to expected code points; `T-MSG-06` monospace body offsets for astral header/body; `T-MSG-08` title/type occur only in chunk 1; `T-MSG-09` invalid type/title length exits 2 before config access.
- **Done:** all fixed composition rules are reflected in golden previews, raw phase-1 output is unchanged, planner has no I/O, and all input is planned before any future sender sees it; `make verify` passes; unit closes with section 1 pointing to 2.4 and every `T-MSG-*` row reconciled.

### 2.4 Update README.md

- **Model:** `agent-3:haiku`
- **Assignment:** `agent-3:haiku` documents phase-2 formatting and long-message preview behavior only.
- **Files:** `README.md:1`.
- **Change:** Preserve existing sections and update `Usage`, `Flags`, `Formatting`, `Long messages`, `Dry run`, `Limits`, and `Troubleshooting`. Document `--title`, `--type`, `--monospace`, exact type names, first-chunk-only header, no chunk counters, newline-preferred splitting, 4096 UTF-16 safety, 1 MiB input cap, and dry-run preview entity fields. Use realistic commands and redacted JSON. State that live sending is still unavailable in this phase. Do not describe algorithms, helper names, package layout, or future work.
- **Unit tests:** none (documentation); execute examples with the compiled binary and parse JSON.
- **e2e tests:** none - examples map to `T-MSG-01`, `T-MSG-04`, `T-MSG-05`, and `T-MSG-06`.
- **Done:** a new user can predict visible header/body formatting and preview every chunk from README alone; only shipped behavior appears; `make verify` passes; unit closes with phase-2 docs row done and section 1 pointing to phase 3 sub-phase 3.1.

## Phase gates

- **Build:** `make build`
- **Fmt:** `make fmt-check`
- **Lint:** `make lint`
- **Test subset:** `make test && make test-e2e && make vuln`
- **Regression guard:** all prior IDs plus `T-MSG-01..09` and unit-level `T-MSG-07`
- **README:** exact formatting, chunk behavior, limits, and still-offline status.

## Phase done criterion

For every accepted body/options combination, dry-run returns the exact ordered requests the Telegram client will later send, every request and entity satisfies UTF-16 limits, and body reconstruction is byte-identical. `make verify` is green, README documents only this offline planner, and `STATE.md` section 11 marks phase 2 `DONE` with all units closed.
