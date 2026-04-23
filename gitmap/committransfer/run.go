package committransfer

import (
	"fmt"
	"os"
)

// RunRight is the public entry point for `commit-right`. The caller
// has already resolved both endpoints and built the Options struct.
//
// Phases 2 and 3 (commit-left, commit-both) will add RunLeft and
// RunBoth on top of the same Plan/Replay primitives.
func RunRight(sourceDir, targetDir string, opts Options) error {
	plan, err := BuildPlan(sourceDir, targetDir, opts)
	if err != nil {
		return fmt.Errorf("build plan: %w", err)
	}
	willReplay := PrintPlan(os.Stdout, plan, opts.LogPrefix)
	if willReplay == 0 {
		fmt.Fprintf(os.Stdout, "%s nothing to replay.\n", opts.LogPrefix)

		return nil
	}
	if !opts.DryRun && !opts.Yes && !Confirm(opts.LogPrefix) {
		fmt.Fprintf(os.Stderr, "%s aborted by user.\n", opts.LogPrefix)

		return nil
	}
	res, replayErr := Replay(plan, opts)
	if replayErr != nil {
		PrintSummary(os.Stderr, opts.LogPrefix, res)

		return replayErr
	}
	res.Pushed = maybePush(targetDir, opts, len(res.NewSHAs))
	PrintSummary(os.Stdout, opts.LogPrefix, res)

	return nil
}

// maybePush runs `git push` unless --no-push is set, the target is not
// a git repo, or there are no new commits. Returns true on success.
func maybePush(targetDir string, opts Options, newCount int) bool {
	if opts.NoPush || opts.NoCommit || newCount == 0 || opts.DryRun {
		return false
	}
	if _, err := pushHEAD(targetDir); err != nil {
		fmt.Fprintf(os.Stderr, "%s push failed: %v\n", opts.LogPrefix, err)

		return false
	}

	return true
}
