package committransfer

import (
	"fmt"
	"io"
	"os"
)

// PrintPlan writes a human-readable preview of plan to w (spec §3).
// Returns the number of commits that will actually be replayed (handy
// for the y/N prompt threshold).
func PrintPlan(w io.Writer, plan ReplayPlan, prefix string) int {
	willReplay := 0
	for _, c := range plan.Commits {
		if c.SkipCause == "" {
			willReplay++
		}
	}
	fmt.Fprintf(w, "%s replaying %d commits onto target:\n", prefix, willReplay)
	for i, c := range plan.Commits {
		printPlanLine(w, prefix, i+1, len(plan.Commits), c)
	}

	return willReplay
}

// printPlanLine renders one commit's preview row.
func printPlanLine(w io.Writer, prefix string, i, n int, c SourceCommit) {
	if c.SkipCause != "" {
		fmt.Fprintf(w, "%s [%d/%d] %s → -        skipped: %s\n",
			prefix, i, n, c.ShortSHA, c.SkipCause)

		return
	}
	subject := firstLine(c.Cleaned)
	fmt.Fprintf(w, "%s [%d/%d] %s  %s\n", prefix, i, n, c.ShortSHA, subject)
}

// firstLine returns everything before the first newline.
func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}

	return s
}

// PrintSummary writes the final replay summary (spec §12).
func PrintSummary(w io.Writer, prefix string, res ReplayResult) {
	fmt.Fprintf(w, "%s done: replayed %d, skipped %d (drop=%d, already-replayed=%d, empty=%d)\n",
		prefix, res.Replayed,
		res.SkippedDrop+res.SkippedReplayed+res.SkippedEmpty,
		res.SkippedDrop, res.SkippedReplayed, res.SkippedEmpty)
	if res.Pushed && len(res.NewSHAs) > 0 {
		fmt.Fprintf(w, "%s pushed %d commits\n", prefix, len(res.NewSHAs))
	}
}

// Confirm reads "y" / "yes" from os.Stdin and returns true. Anything
// else (including EOF) returns false.
func Confirm(prefix string) bool {
	fmt.Fprintf(os.Stdout, "%s proceed? [y/N] ", prefix)
	var ans string
	if _, err := fmt.Scanln(&ans); err != nil {
		return false
	}
	switch ans {
	case "y", "Y", "yes", "YES", "Yes":
		return true
	}

	return false
}
