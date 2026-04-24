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
