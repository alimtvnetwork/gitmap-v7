# Pending Issues

## 01 — Unit Test Coverage Gaps
- **Status**: Open since v2.49.0
- **Description**: Missing unit tests for `task`, `env`, and `install` command families
- **Impact**: Low — commands work but lack automated regression coverage
- **Blocked By**: Nothing — can be done anytime
- **Files Affected**: `cmd/task*.go`, `cmd/env*.go`, `cmd/install*.go`

## 02 — Install --check Missing "Not Found" Message
- **Status**: Open since v2.49.0
- **Description**: `gitmap install --check <tool>` doesn't print a distinct message when a tool is not installed; constant was added but wiring is incomplete
- **Impact**: Low — tool still works, just poor UX for missing tools
- **Files Affected**: `cmd/installtools.go`

## 03 — Docs Site Navigation Missing Pages
- **Status**: Open since v2.76.0
- **Description**: `version-history` and `clone` pages exist but are not linked from the sidebar or commands page navigation
- **Impact**: Low — pages exist at `/version-history` and users won't discover them organically
- **Files Affected**: Sidebar component, `src/data/commands.ts`

## 04 — Helptext/env.md Missing --shell Examples
- **Status**: Open since v2.49.0
- **Description**: The `--shell` flag was wired into env commands but the help text file doesn't demonstrate usage
- **Impact**: Low — flag works but users won't know about it from `gitmap help env`
- **Files Affected**: `helptext/env.md`

## 05 — Clone-Next Missing --dry-run Support
- **Status**: Open (feature gap)
- **Description**: The flatten spec (87-clone-next-flatten.md) mentions `--dry-run` for previewing clone-next actions but it's not implemented
- **Impact**: Medium — users can't preview destructive folder removal before it happens
- **Files Affected**: `cmd/clonenext.go`, `cmd/clonenextflags.go`, `constants/constants_clonenext.go`

## 06 — Multi-URL Clone: PowerShell Comma-Splitting Crash (FIXED v3.80.0)
- **Status**: Fixed in v3.80.0
- **Reported**: User ran `gitmap clone url1,url2,url3` in PowerShell on Windows; got `fatal: could not create leading directories of 'D:\...\https:\github.com\alimtvnetwork\email-reader-v3.gitmap-tmp-...': Invalid argument`
- **Root Cause**:
  1. PowerShell on Windows silently splits unquoted comma-separated arguments into multiple `argv` entries when invoking external executables. So `url1,url2,url3` arrived as three separate `os.Args` entries, not one string.
  2. `parseCloneFlags` only inspected the first two positional args: `Arg(0)` became the source URL, `Arg(1)` was treated as the **folder name**.
  3. `executeDirectClone` then called `filepath.Abs("https://github.com/.../email-reader-v3")`, producing the nonsense Windows path `D:\...\https:\github.com\alimtvnetwork\email-reader-v3` (illegal because `:` is reserved after the drive letter).
  4. The replace-strategy code then tried to `os.RemoveAll` and `git clone` into that path, both of which fail with "filename, directory name, or volume label syntax is incorrect" / "could not create leading directories".
  5. Spec `01-app/104-clone-multi.md` and `mem://features/clone-multi` had been **planned for v3.38.0 but never implemented** — the parser still assumed exactly one source.
- **Solution**:
  1. New `flattenURLArgs([]string) []string` (`gitmap/cmd/clonemulti.go`) — splits each positional arg on `,`, trims whitespace, drops empties, dedupes case-insensitively (normalising trailing `.git`), preserving first-seen order. Accepts both `a b c` and `a,b,c` and mixed `a,b c d,e`.
  2. `parseCloneFlags` now returns a `CloneFlags` struct exposing the **full positional slice** (not just `Arg(0)`/`Arg(1)`).
  3. `resolveCloneFolderName` defensively returns `""` when the second positional arg looks like a URL — so even single-URL invocations can't be misinterpreted as `<url> <folder=other-url>`.
  4. `runClone` detects multi-URL form (any positional contains `,`, or 2+ positionals where both Arg(0) and Arg(1) parse as URLs) and dispatches to the new `runCloneMulti` worker which calls a non-fatal `executeDirectCloneOne` per URL, continuing on failure.
  5. Exit codes per spec: `0` all OK, `1` partial failure, `3` all URLs invalid.
- **Files Affected**:
  - `gitmap/cmd/clone.go` — new `runClone` dispatcher + `shouldUseMultiClone` + `runCloneMulti`
  - `gitmap/cmd/clonemulti.go` (new) — `flattenURLArgs`, `classifyURLs`, `executeDirectCloneOne`, `resolveCloneFolder`, `normaliseURLKey`
  - `gitmap/cmd/rootflags.go` — `CloneFlags` struct, `isLikelyURL` guard
  - `gitmap/constants/constants_clone.go` — `MsgCloneInvalidURLFmt`, `MsgCloneSummaryMultiFmt`, `MsgCloneRegisteredInline`, `MsgCloneMultiBegin`, `MsgCloneMultiItem`, `ErrCloneAllInvalid`, `ErrCloneMultiFailedFmt`, `ExitCloneMultiPartialFail`, `ExitCloneMultiAllInvalid`
  - `gitmap/constants/constants.go` — version bumped to `3.80.0`
- **PowerShell Note**: Even after this fix, users should prefer space-separated URLs in PowerShell to avoid relying on PS's implicit comma-splitting (which differs across PS 5.1 / 7.x). Both forms now work either way.

## 07 — URL Shortcut: `gitmap <url>` Should Auto-Clone (FIXED v3.81.0)
- **Status**: Fixed in v3.81.0
- **Reported**: User ran `gitmap https://github.com/...,https://...,https://...` (omitting the `clone` subcommand) and got `Unknown command: https://github.com/...`. Same with single-URL `gitmap <url>` and any GitHub/GitLab/SSH URL.
- **Root Cause**: `Run()` treated `os.Args[1]` strictly as a subcommand name and dispatched it through `dispatchCore`/`dispatchRelease`/etc. A bare URL has no matching subcommand, so it fell through to `ErrUnknownCommand`. There was no shortcut layer between argv and dispatch.
- **Solution**: In `gitmap/cmd/root.go` `Run()`, immediately after migration runs, check if `os.Args[1]` looks like a git URL via the existing `isLikelyURL` helper (matches `https://`, `http://`, `git@`). If yes, rewrite argv to `[binary, "clone", <original args...>]` so the existing multi-URL clone pipeline (issue 06) handles it. Single URL, comma-list, or space-separated URLs all work — `runCloneMulti`'s `flattenURLArgs` covers all forms.
- **Files Affected**:
  - `gitmap/cmd/root.go` — argv-rewrite shortcut before alias extraction and dispatch
  - `gitmap/constants/constants.go` — version bumped to `3.81.0`
- **UX Note**: The shortcut only fires for URLs (HTTPS/SSH git). Local file paths, shorthands (`json`/`csv`/`text`), and all existing subcommands keep their current behaviour.

## 08 — CI Lint Failures: errorlint / gocritic / unparam (FIXED v3.81.1)
- **Status**: Fixed in v3.81.1
- **Reported**: `golangci-lint run` failed in CI with 3 NEW findings vs main baseline:
  1. `cmd/reinstall.go:125` — `errorlint`: `err.(*exec.ExitError)` type assertion fails on wrapped errors
  2. `committransfer/env.go:6` — `gocritic` (unlambda): `func() []string { return os.Environ() }` should be `os.Environ`
  3. `committransfer/replay.go:126` — `unparam`: `shouldSkipPath` parameter `info os.FileInfo` is never read
- **Root Cause**:
  1. **errorlint**: Direct type assertion on `error` only matches the outermost concrete type. If any wrapper (e.g. `fmt.Errorf("%w", err)`) sits between, the assertion silently fails and we'd report exit code `1` instead of the real script exit code. The project's `.golangci.yml` enables `errorlint` precisely to forbid this pattern (memory rule: "Use `errors.Is`" — same family applies for `errors.As`).
  2. **gocritic unlambda**: Wrapping a parameterless, same-signature function in another lambda is dead indirection — `os.Environ` already satisfies `func() []string`. The wrapper was a leftover from an earlier refactor that briefly took arguments.
  3. **unparam**: `shouldSkipPath` historically accepted `info os.FileInfo` to check `IsDir()`, but that check was lifted into both call sites (so the caller can return `filepath.SkipDir`). The parameter became dead weight; `unparam` correctly flagged it.
- **Solution**:
  1. `cmd/reinstall.go`: replaced the type assertion with `var exitErr *exec.ExitError; if errors.As(err, &exitErr) { ... }` and added `"errors"` to imports. Now correctly unwraps any future wrapping.
  2. `committransfer/env.go`: simplified to `var currentEnv = os.Environ` — same behaviour, no allocation, no indirection. Tests can still stub it (`currentEnv = func() []string { return ... }`).
  3. `committransfer/replay.go`: removed the unused `info os.FileInfo` parameter from `shouldSkipPath`; updated both call sites in `snapshotCopy` and `mirrorPrune`. Caller still has its own `info` in scope for the `IsDir()` branch after the skip check.
- **Files Affected**:
  - `gitmap/cmd/reinstall.go` — `errors.As` + import
  - `gitmap/committransfer/env.go` — direct method-value assignment
  - `gitmap/committransfer/replay.go` — signature + 2 call sites
  - `gitmap/constants/constants.go` — version bumped to `3.81.1`
- **Prevention**: All three rules (`errorlint`, `gocritic`, `unparam`) are already enabled in `.golangci.yml` — the issue was that they passed silently before the offending code was introduced. Going forward, run `golangci-lint run --path-prefix=gitmap` locally before pushing (or rely on the CI diff-vs-baseline gate which now catches this).

## 09 — Windows Update Cleanup Popup: `Windows cannot find '\\'` (FIXED v3.82.0)
- **Status**: Fixed in v3.82.0
- **Reported**: After a successful `gitmap update`, the terminal showed `→ Handing off cleanup to deployed binary: gitmap.exe update-cleanup`, then Windows displayed a popup: `Windows cannot find '\\'`. This repeated across multiple update attempts and the terminal showed no useful diagnostics.
- **Reproduction Context**:
  1. Run `gitmap update` on Windows from a deployed binary setup.
  2. Allow the update runner to finish build/deploy + migrations.
  3. At phase 3, the handoff copy tries to invoke the newly deployed binary with `update-cleanup`.
  4. Instead of cleanup running quietly, Windows Shell/`cmd` surfaces `Windows cannot find '\\'`.
- **Root Cause**:
  1. The bug was **not** inside `runUpdateCleanup()` itself. The failure happened *before cleanup started*, in `gitmap/cmd/updatehandoff_phase3.go` during the Windows phase-3 launch.
  2. `spawnDeployedCleanupWindows` originally built one flat shell command string and passed it to `cmd.exe /C`: `ping 127.0.0.1 -n 3 >nul & start "" /B "<deployed>" update-cleanup`.
  3. That pattern depended on fragile `cmd.exe` quoting semantics (`start` treats the first quoted token as a window title, and Go's Windows argument escaping adds another parsing layer). External Go/Windows reports match this exact failure mode, including the popup `Windows cannot find '\\'`.
  4. The handoff also discarded stdout/stderr and ignored the returned `Start()` error, so the CLI emitted **no useful diagnostics** even when the detached launch failed.
- **Solution**:
  1. Removed the fragile `cmd.exe /C ... start ...` handoff from `gitmap/cmd/updatehandoff_phase3.go`.
  2. Windows now launches the deployed binary directly with `exec.Command(deployed, constants.CmdUpdateCleanup)` instead of routing through `cmd/start`.
  3. Added a Windows-only hidden-process helper (`gitmap/cmd/processattr_windows.go`) so the cleanup process stays unobtrusive without embedding Windows-only fields in shared code.
  4. Added `GITMAP_UPDATE_CLEANUP_DELAY_MS=1500` plus `delayUpdateCleanupIfNeeded()` so the cleanup process waits briefly before deleting temp `.exe` / `.old` files.
  5. Cleanup handoff now prints the resolved target path and reports launch failures to `os.Stderr` instead of failing silently.
- **Files Affected**:
  - `gitmap/cmd/updatehandoff_phase3.go`
  - `gitmap/cmd/updatecleanup.go`
  - `gitmap/cmd/processattr_windows.go`
  - `gitmap/cmd/processattr_other.go`
  - `gitmap/cmd/selfuninstallhandoff.go`
  - `gitmap/constants/constants_update.go`
  - `gitmap/constants/constants.go`
- **Prevention**:
  1. Avoid string-built `cmd.exe /C start ...` launchers for internal handoffs.
  2. Never silence detached-launch failures in update-critical paths.
  3. Keep Windows-only process attributes in `_windows.go` files so non-Windows builds stay clean.

## 10 — Windows Update Cleanup Repeats After "Fix": PATH-First Handoff Targets Wrong Binary (FIXED v3.83.0)
- **Status**: Fixed in v3.83.0
- **Reported**: User kept seeing the update-cleanup failure repeatedly even after the earlier `cmd.exe` popup fix. The update appeared to complete, but cleanup still did not reliably run, and the console still lacked enough evidence to show *which binary* actually received `update-cleanup`.
- **Root Cause**:
  1. The earlier fix removed the fragile `cmd.exe /C start ...` launcher, but `resolveDeployedBinaryPath()` still resolved the cleanup target via `exec.LookPath("gitmap")` **before** checking the config-declared deployed location.
  2. On Windows machines with duplicate or stale `gitmap.exe` installs on `PATH`, Phase 3 could hand `update-cleanup` to the **wrong binary** — not the freshly deployed one that had just been updated.
  3. When the wrong binary was launched hidden/detached, the user saw the same cleanup problem repeat, but the terminal did not clearly reveal the resolution source (`PATH` vs config vs sibling) or the child PID, so the failure looked mysterious and "random".
  4. The embedded PowerShell comment in `constants_update.go` had also become stale: it still described the old `cmd.exe`-based handoff, which made future debugging and reasoning about the real runtime path harder.
- **Why Logs Still Felt Missing**:
  1. The handoff printed the target path, but **not** how that target was chosen.
  2. It did not print a started PID for the detached cleanup child.
  3. Invalid cleanup delay values were silently ignored.
  4. Without `--verbose`, there was no durable handoff trace explaining whether the cleanup ran inline, from config, from a sibling binary, or from a PATH fallback.
- **Solution**:
  1. Reordered deployed-binary resolution in `gitmap/cmd/updatehandoff_phase3.go` to prefer the config-declared deployed binary (`powershell.json` / deploy path) first, then sibling binary next to the handoff copy, and only use `PATH` as a last resort.
  2. Added explicit console output for the **resolution source** and the resolved cleanup target path.
  3. Added explicit console output for the detached cleanup child **PID** after a successful `Start()`.
  4. Added verbose-log entries for target resolution, inline cleanup, child start success, start failure, and missing-target cases.
  5. Added an explicit stderr error when no deployed cleanup target can be resolved at all.
  6. Added an explicit stderr + verbose warning when `GITMAP_UPDATE_CLEANUP_DELAY_MS` contains an invalid value instead of silently ignoring it.
  7. Corrected the stale embedded cleanup comment in `gitmap/constants/constants_update.go` so docs now match the real implementation.
- **Files Affected**:
  - `gitmap/cmd/updatehandoff_phase3.go` — prefer config/sibling over PATH; print source/path/pid; log handoff lifecycle
  - `gitmap/cmd/updatecleanup.go` — log cleanup start/finish and invalid delay values
  - `gitmap/constants/constants_update.go` — new handoff/log strings + corrected embedded update cleanup note
  - `gitmap/constants/constants.go` — version bumped to `3.83.0`
- **Console Evidence Added**:
  1. `→ Cleanup target resolved via: config|sibling|PATH`
  2. `→ Cleanup target path: ...`
  3. `→ Cleanup process started (pid=...)`
  4. `→ Cleanup binary: ...` inside `update-cleanup`
  5. explicit stderr message when the handoff target cannot be resolved or the delay env is invalid
- **Prevention**:
  1. In self-update flows, never trust `PATH` first when a config-declared deployed binary exists.
  2. Every detached handoff must log **which target** was selected and **why**.
  3. Embedded script comments/docs must be updated together with orchestration changes so future debugging is based on reality, not stale notes.
  4. Best-effort cleanup may stay non-fatal, but target-resolution failures must always be visible in the console and verbose log.

## 13 — CI Lint Failures: errorlint / gocritic / unparam (FIXED v3.85.1)
- **Status**: Fixed in v3.85.1
- **Reported**: golangci-lint v1.64.8 reported 3 NEW findings on a CI run:
  1. `cmd/reinstall.go:125` — `errorlint`: `err.(*exec.ExitError)` will fail on wrapped errors.
  2. `committransfer/env.go:6` — `gocritic/unlambda`: `func() []string { return os.Environ() }` should be `os.Environ`.
  3. `committransfer/replay.go:126` — `unparam`: `shouldSkipPath` parameter `info` is unused.
- **Root Cause**: Three independent style/correctness regressions slipped in across separate refactors:
  1. Direct type assertion on `error` instead of `errors.As` — predates the project's errorlint adoption; missed in earlier sweep.
  2. A wrapping lambda around `os.Environ` left over from an earlier "test-stubbable" pattern that was simplified later.
  3. `shouldSkipPath` originally inspected `info.IsDir()`, but the dir-skip logic moved into the callers (`filepath.Walk` callbacks already had `info` in scope), leaving `info` dead in the helper signature.
- **Solution** (already in source as of this entry):
  1. `cmd/reinstall.go` now uses `var exitErr *exec.ExitError; if errors.As(err, &exitErr) { exitCode = exitErr.ExitCode() }`.
  2. `committransfer/env.go` declares `var currentEnv = os.Environ` (no lambda).
  3. `committransfer/replay.go` `shouldSkipPath(rel string, opts Options) bool` — `info` removed; both call sites updated to `shouldSkipPath(rel, opts)`.
- **Files Affected**:
  - `gitmap/cmd/reinstall.go`
  - `gitmap/committransfer/env.go`
  - `gitmap/committransfer/replay.go`
- **Why It Repeated**: The CI pipeline ran on a stale commit before these fixes were pushed. The auto-summary generator (`.github/scripts/lint-issue-summary.py`) does NOT yet have a per-finding fingerprint, so re-runs against the same stale commit kept producing the same NEW set without flipping prior FIXED entries. This is now mitigated by the auto-resolve pass added in the previous task — once the next CI run on a fresh commit reports 0 NEW for these fingerprints, any open CI-lint entry will be auto-flipped to FIXED.
- **Prevention**:
  1. Run `golangci-lint run ./...` locally before pushing — the same `.golangci.yml` is used in CI.
  2. Never type-assert `error` directly; always go through `errors.As` / `errors.Is`. Enforced by `errorlint`.
  3. Don't wrap stdlib funcs in zero-arg lambdas. If a test seam is needed, assign the function value directly (`var x = pkg.Func`).
  4. When refactoring helper internals, re-check the helper's signature for now-dead parameters. `unparam` catches this if run locally.
  5. The auto-resolve pass will close stale CI-lint entries automatically on the next clean run — no manual sweep required.

