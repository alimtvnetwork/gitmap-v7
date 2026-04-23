package committransfer

import (
	"fmt"
)

// BuildPlan computes the replay set for one direction. It does NOT
// mutate either repo — it only reads source history and the target's
// recent log (for the idempotence check).
func BuildPlan(sourceDir, targetDir string, opts Options) (ReplayPlan, error) {
	sourceHead, err := currentRefName(sourceDir)
	if err != nil {
		return ReplayPlan{}, fmt.Errorf("read source HEAD ref: %w", err)
	}
	base, err := resolveBase(sourceDir, targetDir, opts.Since)
	if err != nil {
		return ReplayPlan{}, err
	}
	shas, err := revListReverse(sourceDir, base, "HEAD", opts.IncludeMerges)
	if err != nil {
		return ReplayPlan{}, fmt.Errorf("rev-list source: %w", err)
	}
	if opts.Limit > 0 && len(shas) > opts.Limit {
		shas = shas[:opts.Limit]
	}
	recentTargetLog, _ := recentLogSubjectsAndBodies(targetDir, 200)

	return assemblePlan(sourceDir, targetDir, sourceHead, base, shas, recentTargetLog, opts)
}

// resolveBase honors --since when set; otherwise asks git for the
// merge-base. An unrelated history yields "" (use full source history).
func resolveBase(sourceDir, targetDir, since string) (string, error) {
	if since != "" {
		return since, nil
	}
	targetHead, err := gitOut(targetDir, "rev-parse", "HEAD")
	if err != nil {
		// Empty target repo — no base, replay full source history.
		return "", nil
	}

	return mergeBase(sourceDir, "HEAD", targetHead)
}

// assemblePlan turns raw SHAs into hydrated SourceCommit entries with
// the message pipeline + idempotence check applied.
func assemblePlan(sourceDir, targetDir, sourceHead, base string,
	shas []string, recentTargetLog string, opts Options,
) (ReplayPlan, error) {
	plan := ReplayPlan{
		SourceDir: sourceDir, TargetDir: targetDir,
		SourceHEAD: sourceHead, BaseSHA: base,
	}
	for _, sha := range shas {
		entry, err := hydrateCommit(sourceDir, sha, recentTargetLog, opts)
		if err != nil {
			return plan, err
		}
		if entry.SkipCause == "drop-pattern" || isDropSkip(entry.SkipCause) {
			plan.SkippedDrop++
		}
		plan.Commits = append(plan.Commits, entry)
	}

	return plan, nil
}

// hydrateCommit reads one source commit, runs the message pipeline, and
// flags it as skipped when the pipeline says so or when the target
// already carries its provenance footer.
func hydrateCommit(sourceDir, sha, recentTargetLog string, opts Options) (SourceCommit, error) {
	subject, body, author, shortSHA, when, err := readCommit(sourceDir, sha)
	if err != nil {
		return SourceCommit{}, fmt.Errorf("read commit %s: %w", sha, err)
	}
	entry := SourceCommit{
		SHA: sha, ShortSHA: shortSHA, Subject: subject, Body: body,
		Author: author, AuthorAt: when,
	}
	if !opts.ForceReplay && opts.Message.Provenance &&
		AlreadyReplayed(recentTargetLog, opts.Message.SourceDisplayName, shortSHA) {
		entry.SkipCause = "already-replayed"

		return entry, nil
	}
	cleaned := CleanMessage(subject, body, opts.Message, shortSHA, when)
	if cleaned.Skipped != "" {
		entry.SkipCause = cleaned.Skipped

		return entry, nil
	}
	entry.Cleaned = cleaned.Final

	return entry, nil
}

// isDropSkip reports whether a SkipCause originated from the drop filter.
func isDropSkip(cause string) bool {
	return len(cause) >= len("drop-pattern") && cause[:len("drop-pattern")] == "drop-pattern"
}
