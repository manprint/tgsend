# Phase 4 - Binary e2e, image, and Docker wrapper

> **Intent:** Harden process-level coverage, package the static binary in a minimal non-root image, and provide a POSIX Docker wrapper with native-equivalent argument/config behavior.
> **Shippable alone?** yes - users can run the complete sender either natively or through Docker using local builds.
> **Preconditions:** phase 3 is `DONE`; no test may use real Telegram or repository `.tgsend`.

## State contract (mandatory)

1. Read [STATE.md](STATE.md) first. Resolve any `OPEN` unit, run section-3 gates, and reconcile sections 1, 7, and 11 with repository truth.
2. Open the sub-phase in section 1 before editing and set section 6 to `claimed - nothing written yet`.
3. Close only after applicable gates pass and sections 4-11 are updated; reset section 6 and point section 1 to the next unit.
4. If interrupted, preserve `OPEN`, list exact partial edits in section 6, and follow the WIP-commit setting.

## External facts used

- **R6:** GoReleaser `dockers_v2` build contexts use `$TARGETPLATFORM/<binary>` and buildx: https://goreleaser.com/customization/package/dockers_v2/
- **R13:** Distroless static Debian 12 supplies a shell-free non-root runtime and CA certificates: https://github.com/GoogleContainerTools/distroless/tree/main/base
- **D14:** Wrapper is Docker-only and must preserve CLI stdin, args, env, config, JSON streams, and exit status.

## Sub-phases

### 4.1 Complete compiled-binary acceptance matrix

- **Model:** `agent-2:sonnet`
- **Assignment:** `agent-2:sonnet` expands e2e coverage; `agent-1:opus` reviews acceptance assertions and isolation.
- **Files:** `test/e2e/main_test.go:1`, `test/e2e/harness_test.go:1`, `test/e2e/cli_test.go:1`, `test/e2e/input_config_test.go:1`, `test/e2e/message_test.go:1`, `test/e2e/send_test.go:1`, `test/e2e/server_test.go:1`, `test/e2e/testdata/:1` (new only for bodies too large to keep readable inline).
- **Change:** Make the e2e package a table-driven black-box matrix covering every documented flag and source/config combination against a scripted local HTTP server. Build once per package with test endpoint enabled; give every process a temporary HOME/config, minimal environment, context timeout, and unique fake token. Assert exit code, exact stdout/stderr cardinality, request count/order/body/entities, and token absence. Ensure tests can run in parallel only when their server/environment is isolated; process tests sharing globals remain serial. Add Windows-aware executable suffix and avoid POSIX-only assumptions in Go harness.
- **Unit tests:** harness tests for environment replacement, Windows exit-code handling, server script exhaustion, request deep-copy, and timeout cleanup.
- **e2e tests:** `T-E2E-01` default config + stdin; `T-E2E-02` explicit config + `-m`; `T-E2E-03` env precedence; `T-E2E-04` `--silent`; `T-E2E-05` dry-run without credentials; `T-E2E-06` each exit category 2-7; `T-E2E-07` exact whitespace/CRLF preservation; `T-E2E-08` full reference scenario; `T-E2E-09` first/middle/final API failure progress; `T-E2E-10` version/help bypass input/config/network.
- **Done:** all seven SPECS examples map to tests, no process inherits developer HOME/proxy/credentials, repeated `make test-e2e` leaves no processes/files, `make verify` passes, and the unit closes with section 1 pointing to 4.2.

### 4.2 Add minimal production image and local smoke target

- **Model:** `agent-2:sonnet`
- **Assignment:** `agent-2:sonnet` implements image packaging; `agent-1:opus` reviews privilege, certificate, and secret boundaries.
- **Files:** `Dockerfile:1` (new), `.dockerignore:1` (new), `Makefile:1`, `test/container/smoke.sh:1` (new). Create `test/container` because no container harness exists; keep the production Dockerfile at root for GoReleaser.
- **Change:** Use a runtime-only `Dockerfile` compatible with GoReleaser's temporary context: `FROM gcr.io/distroless/static-debian12:nonroot`, `ARG TARGETPLATFORM`, copy `$TARGETPLATFORM/tgsend` to `/usr/local/bin/tgsend`, set `ENTRYPOINT ["/usr/local/bin/tgsend"]`, and do not compile in Docker. This runtime supplies CA certificates, no shell/package manager, and numeric non-root UID/GID 65532. Set OCI source/license/title labels and include no config/token. `.dockerignore` excludes `.git`, `.tgsend`, plan/docs/tests, local binaries, coverage, and dist. `make test-container` builds a static Linux test binary into a temporary `linux/amd64/` context, invokes `docker buildx build --load --platform linux/amd64`, verifies image user is nonzero, runs `--version`, pipes a multi-chunk dry-run, verifies CA operation against a controlled TLS endpoint only if the hidden test build supports it, then removes image/context. The smoke script uses `set -eu`, traps cleanup, and fails clearly when Docker/buildx is unavailable; it must not silently pass.
- **Unit tests:** none for Dockerfile; add a Go/text assertion only if needed to ensure `.dockerignore` contains `.tgsend` and Dockerfile contains no token/config copy or build command.
- **e2e tests:** `T-CTR-01` image `--version` JSON; `T-CTR-02` image dry-run with stdin and no config/network; `T-CTR-03` image inspect reports non-root user and Linux/amd64; `T-CTR-04` long Unicode title/type/monospace planning through image preserves `T-E2E-08` request-preview invariants; `T-CTR-05` build context/image history contains no `.tgsend` or sentinel token.
- **Done:** image runs static binary as non-root with CA support, build contains no source/secret/compiler, local smoke cleans up on success/failure, `make verify` and `make test-container` pass, and the unit closes with section 1 pointing to 4.3.

### 4.3 Implement and test POSIX Docker wrapper

- **Model:** `agent-2:sonnet`
- **Assignment:** `agent-2:sonnet` implements shell portability and fake/real runtime tests; `agent-1:opus` reviews native-equivalence assertions.
- **Files:** `tgsend.sh:1` (new), `test/wrapper/wrapper_test.go:1` (new), `test/wrapper/testdata/fake-docker.sh:1` (new), `test/container/smoke.sh:1`, `Makefile:1`. Create `test/wrapper` because wrapper command construction needs isolated tests.
- **Change:** Write POSIX `sh` with `set -eu`, no arrays/eval/bashisms. Require `docker` in PATH. Select `${TGSEND_IMAGE:-ghcr.io/manprint/tgsend:latest}`; this variable is for local testing/pinning and is documented. Scan original `"$@"` in a function (without mutation) for `-c PATH`, `--config PATH`, `--config=PATH`, and attached `-cPATH`; reject missing value exactly as CLI usage JSON/exit 2. Determine config source: explicit path, else `$HOME/.tgsend` only if it exists. Resolve an existing file to an absolute physical path, reject non-regular/unreadable explicit config with one schema-v1 config error/exit 3, and mount it read-only at the identical absolute path. Run with `--rm -i`, `--user "$(id -u):$(id -g)"`, `--workdir "$PWD"`, `--env HOME="$HOME"`, and name-only forwarding for set `TGSEND_TOKEN`/`TGSEND_CHAT_ID` so secret values do not appear in process args. Forward every original CLI argument byte-for-byte and preserve Docker/container exit status via `exec`. Never mount cwd, Docker socket, or config parent. No host-binary fallback. If no config file exists, mount none so env-only config can work. Fake Docker writes one argument per NUL-delimited record for unambiguous assertions.
- **Unit tests:** `TestWrapperForwardsArgumentsIncludingSpacesAndEmpty`; `TestWrapperForwardsStdin`; `TestWrapperDefaultConfigMountReadOnly`; `TestWrapperExplicitConfigForms`; `TestWrapperEnvOnlyNoMount`; `TestWrapperForwardsEnvNamesNotValues`; `TestWrapperImageOverride`; `TestWrapperNoDockerExit`; `TestWrapperMissingConfigJSONExit3`; `TestWrapperPreservesContainerExit`; `TestWrapperNoBashisms` runs `dash -n` and ShellCheck `v0.10.0` with `-s sh`; CI installs that exact version.
- **e2e tests:** `T-WRP-01` fake Docker exact args/stdin/env; `T-WRP-02` real wrapper dry-run equals native decoded JSON except nondeterministic build metadata; `T-WRP-03` real wrapper reads default and each explicit config form against fake endpoint; `T-WRP-04` container exit 2-7 is preserved; `T-WRP-05` sentinel token absent from wrapper stderr/fake Docker argv; `T-WRP-06` Linux and macOS-compatible POSIX syntax verified (macOS in CI or documented local command if runner unavailable).
- **Done:** wrapper behavior matches binary for all seven SPECS scenarios, explicit errors remain one JSON document, no secret value enters argv, `make verify` and `make test-container` pass, and the unit closes with section 1 pointing to 4.4.

### 4.4 Update README.md

- **Model:** `agent-3:haiku`
- **Assignment:** `agent-3:haiku` documents local Docker and wrapper use for shipped behavior only.
- **Files:** `README.md:1`.
- **Change:** Preserve structure; update `Requirements`, `Run with Docker`, `Docker wrapper`, `Configuration`, `Environment`, `Image selection`, `Examples`, `Security`, `Limits`, and `Troubleshooting`. Show building image locally, invoking with stdin/flags, wrapper installation from repository checkout, default/explicit config behavior, `TGSEND_IMAGE`, non-root execution, and Docker permission/common mount failures. State Linux/macOS wrapper support and no Windows/PowerShell wrapper. Do not mention test-only endpoint, internal layout, or release/install URLs not shipped until phase 5.
- **Unit tests:** none (documentation); execute native/image/wrapper examples using dry-run or fake endpoint.
- **e2e tests:** none - examples map to `T-CTR-01`, `T-CTR-04`, `T-WRP-02`, and `T-WRP-03`.
- **Done:** a new user can build and run both image and wrapper from README alone, platform/security constraints are clear, `make verify` and `make test-container` pass, and the unit closes with phase-4 docs row done and section 1 pointing to phase 5 sub-phase 5.1.

## Phase gates

- **Build:** `make build`
- **Fmt:** `make fmt-check`
- **Lint:** `make lint`
- **Test subset:** `make test && make test-e2e && make vuln`
- **Container:** `make test-container`
- **Regression guard:** all prior IDs plus `T-E2E-01..10`, `T-CTR-01..05`, `T-WRP-01..06`
- **README:** local native/Docker/wrapper installation and usage, exact platform/security limitations.

## Phase done criterion

The exhaustive compiled-binary matrix passes, the local image runs as non-root without secrets or build tools, and `tgsend.sh` preserves native CLI behavior, streams, config, env, and exit status on Linux/macOS. `make verify` and `make test-container` are green, README covers both delivery modes, and `STATE.md` section 11 marks phase 4 `DONE` with every unit closed.
