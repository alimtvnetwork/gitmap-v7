package completion

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alimtvnetwork/gitmap-v7/gitmap/constants"
)

// InstallCDFunction writes the gcd shell wrapper to the user's profile.
func InstallCDFunction(shell string) error {
	snippet := cdSnippet(shell)
	if len(snippet) == 0 {
		return fmt.Errorf(constants.ErrCompUnknownShell, shell)
	}

	return appendCDFunctions(snippet, cdProfilePaths(shell))
}

// cdSnippet returns the gcd function body for the given shell.
func cdSnippet(shell string) string {
	switch shell {
	case constants.ShellPowerShell:
		return constants.CDFuncPowerShell
	case constants.ShellBash:
		return constants.CDFuncBash
	case constants.ShellZsh:
		return constants.CDFuncZsh
	default:
		return ""
	}
}

// cdProfilePaths returns all profile paths to write the cd function to.
func cdProfilePaths(shell string) []string {
	switch shell {
	case constants.ShellPowerShell:
		return resolvePowerShellProfilePaths()
	case constants.ShellBash:
		home, _ := os.UserHomeDir()
		return []string{filepath.Join(home, ".bashrc")}
	default:
		home, _ := os.UserHomeDir()
		return []string{filepath.Join(home, ".zshrc")}
	}
}

// appendCDFunctions appends the managed wrapper to every resolved profile.
func appendCDFunctions(snippet string, profilePaths []string) error {
	for _, profilePath := range profilePaths {
		if err := appendCDFunction(snippet, profilePath); err != nil {
			return err
		}
	}

	return nil
}

// appendCDFunction installs or upgrades the gcd function in the profile.
//
// Behaviour:
//   - No managed block present → append a fresh block (Installed message).
//   - Current marker present → no-op (Already-installed message).
//   - Stale marker present (older version) → strip the previous block
//     and append the new one (Upgraded message).
//
// The managed block is delimited by `CDFuncMarker` (start) and
// `CDFuncEndMarker` (end). Any historical marker line beginning with
// `CDFuncMarkerPrefix` is treated as the start of a stale block.
func appendCDFunction(snippet, profilePath string) error {
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
		return fmt.Errorf(constants.ErrCompProfileWrite, profilePath, err)
	}

	existing, _ := os.ReadFile(profilePath)
	existingStr := string(existing)

	if strings.Contains(existingStr, constants.CDFuncMarker) {
		fmt.Fprintf(os.Stderr, constants.MsgCDFuncAlready)

		return nil
	}

	cleaned, hadStale := stripStaleCDBlock(existingStr)
	updated := cleaned + fmt.Sprintf("\n%s\n%s\n%s\n",
		constants.CDFuncMarker, snippet, constants.CDFuncEndMarker)

	if err := os.WriteFile(profilePath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf(constants.ErrCompProfileWrite, profilePath, err)
	}

	if hadStale {
		fmt.Fprintf(os.Stderr, constants.MsgCDFuncUpgraded)
	} else {
		fmt.Fprintf(os.Stderr, constants.MsgCDFuncInstalled)
	}

	return nil
}

// stripStaleCDBlock removes any previously-installed managed wrapper
// block from the profile. A block runs from a line starting with
// CDFuncMarkerPrefix down to the first CDFuncEndMarker line, or — for
// pre-end-marker installs — to the closing backtick of the snippet
// (matched by a line containing only `}` followed by a backtick).
//
// Returns the cleaned content and whether anything was removed.
func stripStaleCDBlock(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	removed := false
	skipping := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !skipping && strings.HasPrefix(trimmed, constants.CDFuncMarkerPrefix) {
			skipping = true
			removed = true

			continue
		}
		if skipping {
			if trimmed == constants.CDFuncEndMarker || trimmed == "}`" {
				skipping = false
			}

			continue
		}
		out = append(out, line)
	}

	return strings.TrimRight(strings.Join(out, "\n"), "\n"), removed
}
