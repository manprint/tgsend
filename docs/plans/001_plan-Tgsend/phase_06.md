# Phase 5 - Release automation, installers, and final documentation

> **Intent:** Produce reproducible multi-platform release assets, GHCR publication, verified installers, CI quality gates, and complete user/agent documentation.
> **Shippable alone?** yes - a `v*` tag can publish the tested product without manual asset assembly.
> **Preconditions:** phase 4 is `DONE`; native, container, and wrapper tests are green locally.

## State contract (mandatory)

1. Read [STATE.md](STATE.md) first. Resolve any `OPEN` unit from section 6, run section-3 gates, and reconcile sections 1, 7, and 11 with repository truth.
2. Open this sub-phase in section 1 before editing; set section 6 to `claimed - nothing written yet`.
3. Close only after all applicable gates pass and sections 4-11 are updated; reset section 6 and point section 1 to the next unit or final verification.
4. If interrupted, retain `OPEN`, record exact partial edits, and follow the WIP-commit setting.

## External facts used

- **R5:** GoReleaser `v2.18.0` supports target filtering, static Go builds, archives, checksums, and release metadata: https://goreleaser.com/customization/builds/go/
- **R6:** `dockers_v2` builds and pushes one multi-platform buildx manifest from prebuilt binaries: https://goreleaser.com/customization/package/dockers_v2/
- **R7:** GHCR publishing uses `GITHUB_TOKEN` with `packages:write`: https://docs.github.com/actions/publishing-packages/publishing-docker-images
- **R8/R9:** quality tools are golangci-lint `v2.13.2` and govulncheck `v1.1.4`.
- **R11:** Syft `v1.51.1` is the pinned SBOM generator: https://github.com/anchore/syft/releases/tag/v1.51.1
- **Pinned actions checked 2026-09-01:** `actions/checkout@v7.0.1`, `actions/setup-go@v7.0.0`, `goreleaser/goreleaser-action@v7.2.3`, `docker/login-action@v4.6.0`, `docker/setup-buildx-action@v4.3.0`, `anchore/sbom-action/download-syft@v0.24.2`.

## Sub-phases

### 5.1 Configure GoReleaser, licenses, archives, SBOMs, and GHCR image

- **Model:** `agent-2:sonnet`
- **Assignment:** `agent-2:sonnet` implements release configuration; `agent-1:opus` reviews target/supply-chain contracts before close.
- **Files:** `.goreleaser.yaml:1` (new), `LICENSE:1` (new), `Dockerfile:1`, `Makefile:1`, `.gitignore:1`.
- **Change:** Write GoReleaser schema `version: 2`, project `tgsend`, build ID `tgsend`, main `./cmd/tgsend`, binary `tgsend`, `CGO_ENABLED=0`, trimpath, reproducible flags, and ldflags setting buildinfo version/commit/date. Include exactly Linux amd64/arm64/arm with GOARM=7, Darwin amd64/arm64, Windows amd64/arm64; use ignore entries so no other GOOS/GOARCH pair appears. Name archives deterministically `tgsend_<os>_<arch>[v7]`, use `tar.gz` except Windows `zip`, include README/LICENSE, and produce SHA-256 `checksums.txt`. Configure Syft SBOMs for archives/binaries and retain them as release assets. Configure `dockers_v2` (not deprecated `dockers`/`docker_manifests`) with ID/build ID `tgsend`, image `ghcr.io/manprint/tgsend`, platforms `linux/amd64`, `linux/arm64`, `linux/arm/v7`, version tag and `latest` only for stable non-prerelease releases, SBOM enabled, and OCI title/source/revision/version/license annotations on index and manifests. Add MIT license text with holder `manprint`. Add `make release-check` running GoReleaser config validation and non-publishing snapshot; add `make release-snapshot`; ensure `/dist/` ignored.
- **Unit tests:** none for declarative config; `scripts/check-release.sh` in 5.4 supplies executable assertions.
- **e2e tests:** `T-REL-01` `goreleaser check` passes with v2.18.0; `T-REL-02` snapshot produces exactly seven binaries in expected archive formats plus checksums/SBOMs; `T-REL-03` each archive contains only binary, README, LICENSE and version JSON reports snapshot metadata; `T-REL-04` Docker config declares exactly three Linux platforms and no deprecated keys.
- **Done:** all D22 targets and only those targets are encoded, checksums use SHA-256, SBOM generation is pinned, license/OCI metadata agree, `make release-check` passes, and the unit closes with section 1 pointing to 5.2.

### 5.2 Add CI and tag-driven publish workflows

- **Model:** `agent-2:sonnet`
- **Assignment:** `agent-2:sonnet` implements GitHub Actions; `agent-1:opus` reviews permissions, secret exposure, and release gating.
- **Files:** `.github/workflows/ci.yml:1` (new), `.github/workflows/release.yml:1` (new), `.github/dependabot.yml:1` (new if action update automation is desired), `Makefile:1`.
- **Change:** `ci.yml` triggers on pull requests and pushes to main with least `contents:read`. Matrix Go `1.27.0` across Ubuntu, macOS, Windows runs build/unit/compiled e2e using OS-native commands; one Ubuntu job runs `make fmt-check`, pinned golangci-lint, govulncheck, and `make release-check`; Ubuntu runs container/wrapper smoke, and macOS runs wrapper fake-runtime tests. No job reads `.tgsend` or Telegram secrets. `release.yml` triggers only `v[0-9]+.[0-9]+.[0-9]+` tags; first reruns the complete non-live quality suite, then uses `needs` so publish cannot start on failure. Set job permissions explicitly: `contents:write`, `packages:write`; login to `ghcr.io` as `${{ github.actor }}` using `${{ secrets.GITHUB_TOKEN }}`, configure buildx, install Syft, and run `goreleaser release --clean` with GoReleaser `v2.18.0`. Use the exact action tags listed above, no `main`, `master`, or floating `latest`. Add concurrency per workflow/ref and cancel pull-request CI but never an in-progress release. Upload no logs/environment artifacts. Dependabot updates Go modules and GitHub Actions weekly.
- **Unit tests:** YAML parse plus a workflow policy test (Go or shell) asserting triggers, permissions, exact action versions, Go version, `needs` gate, no Telegram secret names, and no `pull_request_target`.
- **e2e tests:** `T-CI-01` workflow policy test passes; `T-CI-02` local commands invoked by every CI job are runnable and match Make targets; `T-CI-03` release workflow contains one GoReleaser publish and GHCR login with repository token; `T-CI-04` no live Telegram endpoint/credential appears; `T-CI-05` a documented test tag in a fork/dry-run validates workflow syntax without publishing to production (manual acceptance, not required in normal CI).
- **Done:** PR/main CI gates all code without real Telegram, tags cannot publish before tests, permissions are minimal to release/GHCR needs, every action/tool is pinned, policy tests pass under `make verify`, and the unit closes with section 1 pointing to 5.3.

### 5.3 Implement checksum-verifying POSIX installers

- **Model:** `agent-2:sonnet`
- **Assignment:** `agent-2:sonnet` implements security-sensitive installers and adversarial tests; `agent-1:opus` reviews URL/version/checksum/install boundaries.
- **Files:** `scripts/install.sh:1` (new), `scripts/install-wrapper.sh:1` (new), `test/installer/installer_test.go:1` (new), `test/installer/testdata/:1` (new minimal fake release fixtures), `.goreleaser.yaml:1`, `Makefile:1`. Create `scripts` and `test/installer` because no installer location/harness exists.
- **Change:** Both installers are POSIX `sh`, `set -eu`, create a private `mktemp -d`, trap cleanup, require HTTPS URLs, and never use `eval`. Default release selector is `latest`; non-empty `TGSEND_VERSION` accepts `1.2.3` or `v1.2.3`, normalizes to `v1.2.3`, and rejects all other characters. `TGSEND_INSTALL_DIR` defaults `/usr/local/bin`. Binary installer maps `uname -s/-m` to only supported Linux/macOS assets (`linux_amd64`, `linux_arm64`, `linux_armv7`, `darwin_amd64`, `darwin_arm64`), downloads archive and `checksums.txt` from GitHub release, selects the exact filename with fixed-string matching, verifies using `sha256sum -c` or `shasum -a 256`, extracts only expected file, validates `tgsend --version` JSON, then installs mode 0755 atomically via a temp file in destination plus rename. Wrapper installer downloads `tgsend.sh` and `tgsend.sh.sha256`, verifies it, syntax-checks with `sh -n`, then atomically installs as `tgsend`. Any download/checksum/platform/install failure exits nonzero before replacing an existing file. Configure release extra files for wrapper and its generated checksum. Tests use a local HTTP fixture server via an installer-only injectable base URL accepted only when `TGSEND_INSTALL_TEST=1`; production default remains fixed GitHub HTTPS.
- **Unit tests:** `TestInstallerOSArchMap`; `TestVersionNormalizationAndRejection`; `TestLatestAndPinnedURLs`; `TestChecksumMismatchLeavesExistingBinary`; `TestTruncatedDownloadFails`; `TestMissingChecksumEntryFails`; `TestUnsupportedPlatformFails`; `TestAtomicReplacement`; `TestCleanupOnSignalAndFailure`; `TestWrapperSyntaxFailureDoesNotInstall`; `TestNoCurlPipeInsideInstaller`; `TestInstallMode0755`.
- **e2e tests:** `T-INS-01` latest binary fixture installs and version runs; `T-INS-02` `TGSEND_VERSION=v1.2.3` requests only pinned URLs; `T-INS-03` corrupted asset is rejected and prior executable hash unchanged; `T-INS-04` wrapper installs, invokes fake Docker, and preserves args; `T-INS-05` Linux amd64/arm64/armv7 and Darwin mappings select exact release names; `T-INS-06` installer output never contains fixture token/config content.
- **Done:** both installers fail closed before replacement, support latest/pinned releases and only declared platforms, verify exact assets, clean temporary data, tests pass under `make verify`, and the unit closes with section 1 pointing to 5.4.

### 5.4 Add release artifact and manifest acceptance checks

- **Model:** `agent-3:haiku`
- **Assignment:** `agent-3:haiku` implements mechanical artifact inspection from the exact contract; `agent-1:opus` reviews acceptance coverage.
- **Files:** `scripts/check-release.sh:1` (new), `scripts/check-image.sh:1` (new), `test/release/release_test.go:1` (new if shell assertions become unsafe), `Makefile:1`.
- **Change:** After a clean snapshot, enumerate expected archives exactly and reject missing/extra target artifacts. Verify `checksums.txt` against every release asset it lists, require an SBOM for every configured archive/binary, inspect archive members for path traversal/unexpected files, run native executable version, and cross-inspect non-native formats without execution. `check-image.sh <tag>` uses `docker buildx imagetools inspect` to require linux/amd64, linux/arm64, linux/arm/v7, OCI license/source/version annotations, and no extra platform. Add `make release-check` to run config plus snapshot/artifact checks; published-image inspection is a post-publish release job and failure marks the workflow failed without deleting evidence. Verify installer URLs against actual asset names. Never inspect or package `.tgsend`.
- **Unit tests:** parser tests for duplicate/missing/extra artifact, malformed checksum, path traversal archive entry, missing SBOM, and extra image platform.
- **e2e tests:** `T-REL-05` full snapshot artifact checker passes; `T-REL-06` intentionally removed asset makes checker fail; `T-REL-07` installer URL matrix matches generated names; `T-REL-08` published GHCR manifest has exactly three platforms and OCI metadata (post-publish); `T-REL-09` recursive search of archives/SBOM/image context finds no `.tgsend` or sentinel secret.
- **Done:** one command proves local release completeness and post-publish job proves manifest completeness; negative fixtures fail for the intended reason; `make release-check`, `make verify`, and `make test-container` pass; unit closes with section 1 pointing to 5.5.

### 5.5 Update README.md

- **Model:** `agent-3:haiku`
- **Assignment:** `agent-3:haiku` completes the user guide; `agent-1:opus` performs the final read and rejects internal, stale, unsafe, or untested instructions.
- **Files:** `README.md:1`, `SPECS.md:1` (read-only source of original scenarios; modify only to correct a proven contradiction and record a deviation), repository release links referenced by README.
- **Change:** Preserve and refine the existing guide. Ensure final sections cover: purpose/audience; binary, Docker, wrapper, and source requirements/install; inspect-before-run and one-line curl installers; Windows manual binary install; all commands/flags with defaults; TOML/env precedence; formatting/type/silent examples; long-message/UTF-16/input limits; stable JSON success/error/version examples; exit codes; retry/partial completion; security and secret handling; troubleshooting; release assets/checksums/SBOM; support link. Add `Agentic usage` with non-secret examples for start/progress/success/failure notifications, dry-run validation, JSON exit checking, and guidance to avoid duplicate progress sends after ambiguous failure. One-liners use `https://raw.githubusercontent.com/manprint/tgsend/main/scripts/install*.sh`; immediately provide safer download, inspect, then `sudo sh` alternatives. Document `TGSEND_VERSION` and `TGSEND_INSTALL_DIR`. Mention manual live smoke is optional and never CI. Exclude internal symbols/files/algorithms, plan/phase references, and unshipped roadmap.
- **Unit tests:** none (documentation); automated link/example command checker validates all non-live commands, flags, JSON examples, asset URLs, and headings.
- **e2e tests:** `T-DOC-01` every README dry-run example executes and JSON parses; `T-DOC-02` config/send examples map to compiled fake-endpoint tests; `T-DOC-03` install examples succeed against fixture releases; `T-DOC-04` all seven original SPECS scenarios are present and behavior-accurate; `T-DOC-05` documented flags/defaults equal `tgsend --help`; `T-DOC-06` no token-like literal or hidden test switch appears.
- **Done:** a new user or low-capability agent can install, configure, send, interpret output/failure, and troubleshoot using README alone; every example is tested or explicitly manual; `agent-1:opus` signs off; all gates pass; unit closes in `STATE.md` with phase/docs/tests done and next action set to final plan verification.

## Phase gates

- **Build:** `make build`
- **Fmt:** `make fmt-check`
- **Lint:** `make lint`
- **Test subset:** `make test && make test-e2e && make vuln`
- **Release:** `make release-check`
- **Container:** `make test-container`
- **Regression guard:** every prior ID plus `T-REL-01..09`, `T-CI-01..05`, `T-INS-01..06`, `T-DOC-01..06`
- **README:** complete user/agent guide, tested installers/examples, no internal detail or unshipped behavior.

## Phase done criterion

A clean `vX.Y.Z` tag gates on non-live tests, publishes exactly seven checksum/SBOM-backed binary archives and one three-platform GHCR image, exposes two fail-closed installers, and leaves a complete tested README. All shared gates and post-publish checks are green, `STATE.md` section 11 marks every phase/test/doc `DONE`, every unit is closed, and final verify reports no open blocker.
