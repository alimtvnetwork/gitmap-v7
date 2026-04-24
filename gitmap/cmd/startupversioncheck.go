// Package cmd — startup version check (v3.90.0+).
//
// On every invocation (with the safe-list exceptions below) we print
// a single-line banner to STDERR showing the active gitmap.exe version
// and warn when the requested subcommand or flag was introduced in a
// version newer than the binary the user is actually running. This
// catches the "I just ran `gitmap pending clear` and nothing happened"
// class of bugs where the user is on PATH-old binary while the source
// repo / docs are on the newer version.
//
// Design rules:
//
//  1. STDERR only — never pollutes scriptable stdout (csv/json/etc.).
//  2. One line, not a report. The doctor command exists for full audits.
//  3. Suppressed by `--no-banner`, `--no-version-check`, GITMAP_QUIET=1,
//     and for the safe-list of commands the user needs in order to
//     RECOVER from a version mismatch (version, help, update, doctor,
//     self-install, self-uninstall, completion).
//  4. Pure local — no network, no exec of the deployed binary. The
//     active version is `constants.Version` (the running binary literally
//     IS this build); the source-of-truth version is also constants.Version.
//     The comparison that matters is "active vs minimum required by the
//     subcommand the user just typed", which is data-driven.
//  5. Min-version map (cmdMinVersions) is the single source of truth for
//     "this feature ships in version X" — additive, append-only.
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/alimtvnetwork/gitmap-v7/gitmap/constants"
)

// cmdMinVersions records the minimum gitmap version that introduced
// each subcommand or flag. Append-only. Used by the startup check to
// warn when an older active binary cannot satisfy the request.
//
// Keys are subcommand names (constants.Cmd*) OR `<cmd>:--<flag>` for
// per-flag introductions. Values are semver strings WITHOUT the `v`
// prefix so they parse via parseStartupSemver.
//
// IMPORTANT: only list features whose absence would silently misbehave
// on an older binary. Trivial cosmetic additions don't belong here.
var cmdMinVersions = map[string]string{
	// Multi-URL clone (mem://features/clone-multi).
	"clone:multi-url":             "3.80.0",
	"clone:--no-replace":          "3.55.0",
	"clone:semicolon-separator":   "3.89.0",
	"clone:smart-quote-strip":     "3.89.0",

	// Pending-task management.
	constants.CmdPending + " clear": "3.88.0",

	// Self-update diagnostics.
	"update:--debug-windows":   "3.86.0",
	"update:handoff-log":       "3.87.0",

	// Self-install / uninstall (separate from third-party install).
	constants.CmdSelfInstall:   "3.0.0",
	constants.CmdSelfUninstall: "3.0.0",
}

// startupCheckSafeCommands are the commands we MUST NOT block or warn
// on, because the user runs them precisely to fix a version mismatch
// (or to read help that explains the mismatch).
var startupCheckSafeCommands = map[string]struct{}{
	constants.CmdVersion:       {},
	constants.CmdVersionAlias:  {},
	constants.CmdHelp:          {},
	constants.CmdUpdate:        {},
	constants.CmdUpdateRunner:  {},
	constants.CmdUpdateCleanup: {},
	constants.CmdDoctor:        {},
	constants.CmdSelfInstall:   {},
	constants.CmdSelfUninstall: {},
}

// runStartupVersionCheck prints the active-version banner to stderr
// and emits warnings when the typed command/flags require a newer
// binary than the one currently running. Called from Run() before
// dispatch. Returns silently when suppressed.
func runStartupVersionCheck(command string, args []string) {
	if isStartupCheckSuppressed(command, args) {
		return
	}

	active := constants.Version
	fmt.Fprintf(os.Stderr, constants.MsgStartupCheckBanner, active)

	required, label := requiredVersionFor(command, args)
	if len(required) == 0 {
		return
	}

	if !startupVersionAtLeast(active, required) {
		fmt.Fprintf(os.Stderr, constants.MsgStartupCheckWarn,
			label, required, active)
	}
}

// isStartupCheckSuppressed centralises every reason the banner is
// hidden. Order matters only for readability — all checks are equal.
func isStartupCheckSuppressed(command string, args []string) bool {
	if os.Getenv(constants.EnvGitMapQuiet) == constants.EnvGitMapQuietTrue {
		return true
	}
	if _, safe := startupCheckSafeCommands[command]; safe {
		return true
	}
	for _, a := range args {
		if a == constants.FlagNoBanner ||
			a == constants.FlagNoVersionCheck {
			return true
		}
	}

	return false
}

// requiredVersionFor returns the highest min-version requested by the
// command + its flags, plus a human-readable label describing what
// triggered the requirement (used in the warning message).
//
// We pick the HIGHEST so a single warning surfaces the most demanding
// feature on the line — e.g. `clone --no-replace url1;url2` reports
// the semicolon (3.89.0), not --no-replace (3.55.0).
func requiredVersionFor(command string, args []string) (string, string) {
	highest := ""
	highestLabel := ""

	for key, version := range collectMinVersionMatches(command, args) {
		if startupVersionAtLeast(version, highest) && version != highest {
			highest = version
			highestLabel = key
		}
	}

	return highest, highestLabel
}

// collectMinVersionMatches returns every (label → version) entry in
// cmdMinVersions that the current invocation triggers. Split out so
// requiredVersionFor stays under 15 lines per the code-style memory.
func collectMinVersionMatches(command string,
	args []string) map[string]string {
	hits := make(map[string]string, 4)

	if version, ok := cmdMinVersions[command]; ok {
		hits[command] = version
	}
	for sub := range startupSubcommandKeys(command, args) {
		if version, ok := cmdMinVersions[sub]; ok {
			hits[sub] = version
		}
	}
	for _, flag := range args {
		key := command + ":" + flag
		if version, ok := cmdMinVersions[key]; ok {
			hits[key] = version
		}
	}
	for _, key := range startupBehaviourKeys(command, args) {
		if version, ok := cmdMinVersions[key]; ok {
			hits[key] = version
		}
	}

	return hits
}

// startupSubcommandKeys yields keys of the form "<cmd> <subcmd>" for
// commands that take a positional sub-verb (e.g. `pending clear`).
func startupSubcommandKeys(command string,
	args []string) map[string]struct{} {
	out := make(map[string]struct{}, 1)
	if len(args) == 0 {
		return out
	}
	first := args[0]
	if strings.HasPrefix(first, "-") {
		return out
	}
	out[command+" "+first] = struct{}{}

	return out
}

// startupBehaviourKeys detects argument SHAPES that imply a feature,
// not a literal flag — e.g. `;` in a clone arg means the user is
// relying on the v3.89.0 semicolon-separator support.
func startupBehaviourKeys(command string, args []string) []string {
	out := make([]string, 0, 2)
	if command != constants.CmdClone {
		return out
	}
	urlCount := 0
	for _, a := range args {
		if strings.ContainsAny(a, ",;") {
			if strings.Contains(a, ";") {
				out = append(out, "clone:semicolon-separator")
			}
			out = append(out, "clone:multi-url")
		}
		if strings.ContainsAny(a, "\u201C\u201D\u2018\u2019\uFEFF") {
			out = append(out, "clone:smart-quote-strip")
		}
		if isLikelyURL(a) {
			urlCount++
		}
	}
	if urlCount >= 2 {
		out = append(out, "clone:multi-url")
	}

	return out
}

// startupVersionAtLeast reports whether `have` is >= `want` semver.
// Empty `want` is satisfied by any `have` (so the "no requirement"
// fast-path in requiredVersionFor works without special-casing).
func startupVersionAtLeast(have, want string) bool {
	if len(want) == 0 {
		return true
	}
	hMaj, hMin, hPatch := parseStartupSemver(have)
	wMaj, wMin, wPatch := parseStartupSemver(want)

	if hMaj != wMaj {
		return hMaj > wMaj
	}
	if hMin != wMin {
		return hMin > wMin
	}

	return hPatch >= wPatch
}

// parseStartupSemver extracts (major, minor, patch) from "X.Y.Z" or
// "vX.Y.Z". Missing or non-numeric parts collapse to 0 — only ever
// used for ordering, never for display.
func parseStartupSemver(s string) (int, int, int) {
	clean := strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.SplitN(clean, ".", 3)
	maj := atoiSafeStartup(parts, 0)
	min := atoiSafeStartup(parts, 1)
	patch := atoiSafeStartup(parts, 2)

	return maj, min, patch
}

// atoiSafeStartup returns parts[idx] as int, defaulting to 0 on miss.
// Stops at the first non-digit so "0-rc1" → 0.
func atoiSafeStartup(parts []string, idx int) int {
	if idx >= len(parts) {
		return 0
	}
	out := 0
	for _, r := range parts[idx] {
		if r < '0' || r > '9' {
			break
		}
		out = out*10 + int(r-'0')
	}

	return out
}
