package cmd

import (
	"flag"
	"strings"

	"github.com/alimtvnetwork/gitmap-v7/gitmap/constants"
)

// parseScanFlags parses flags for the scan command.
func parseScanFlags(args []string) (dir, configPath, mode, output, outFile, outputPath string, ghDesktop, openFolder, quiet, noVSCodeSync, noAutoTags bool, workers int) {
	fs := flag.NewFlagSet(constants.CmdScan, flag.ExitOnError)
	cfgFlag := fs.String("config", constants.DefaultConfigPath, constants.FlagDescConfig)
	modeFlag := fs.String("mode", "", constants.FlagDescMode)
	outputFlag := fs.String("output", "", constants.FlagDescOutput)
	outFileFlag := fs.String("out-file", "", constants.FlagDescOutFile)
	outputPathFlag := fs.String("output-path", "", constants.FlagDescOutputPath)
	ghDesktopFlag, openFlag, quietFlag := registerScanBoolFlags(fs)
	noVSCodeSyncFlag := fs.Bool(constants.FlagNoVSCodeSync, false, constants.FlagDescNoVSCodeSync)
	noAutoTagsFlag := fs.Bool(constants.FlagNoAutoTags, false, constants.FlagDescNoAutoTags)
	workersFlag := fs.Int(constants.FlagScanWorkers, constants.DefaultScanWorkers, constants.FlagDescScanWorkers)
	fs.Parse(args)

	dir = resolveScanDir(fs)

	return dir, *cfgFlag, *modeFlag, *outputFlag, *outFileFlag, *outputPathFlag, *ghDesktopFlag, *openFlag, *quietFlag, *noVSCodeSyncFlag, *noAutoTagsFlag, *workersFlag
}

// registerScanBoolFlags registers boolean flags for the scan command.
func registerScanBoolFlags(fs *flag.FlagSet) (*bool, *bool, *bool) {
	ghDesktopFlag := fs.Bool("github-desktop", false, constants.FlagDescGHDesktop)
	openFlag := fs.Bool("open", false, constants.FlagDescOpen)
	quietFlag := fs.Bool("quiet", false, constants.FlagDescQuiet)

	return ghDesktopFlag, openFlag, quietFlag
}

// resolveScanDir returns the scan directory from positional args or default.
func resolveScanDir(fs *flag.FlagSet) string {
	if fs.NArg() > 0 {
		return fs.Arg(0)
	}

	return constants.DefaultDir
}

// CloneFlags holds all parsed clone-command flags and positional args.
// Exposing the full positional slice (Positional) lets runClone detect
// the multi-URL invocation form documented in spec/01-app/104-clone-multi.md.
type CloneFlags struct {
	Source      string
	FolderName  string
	TargetDir   string
	SSHKeyName  string
	Positional  []string
	SafePull    bool
	GHDesktop   bool
	NoReplace   bool
	Verbose     bool
}

// parseCloneFlags parses flags for the clone command.
func parseCloneFlags(args []string) CloneFlags {
	fs := flag.NewFlagSet(constants.CmdClone, flag.ExitOnError)
	targetFlag := fs.String("target-dir", constants.DefaultDir, constants.FlagDescTargetDir)
	safePullFlag := fs.Bool("safe-pull", false, constants.FlagDescSafePull)
	ghDesktopFlag := fs.Bool("github-desktop", false, constants.FlagDescGHDesktop)
	verboseFlag := fs.Bool("verbose", false, constants.FlagDescVerbose)
	noReplaceFlag := fs.Bool("no-replace", false, constants.FlagDescCloneNoReplace)
	sshKeyFlag := fs.String("ssh-key", "", "SSH key name for clone")
	fs.StringVar(sshKeyFlag, "K", "", "SSH key name (short)")
	fs.Parse(args)

	return CloneFlags{
		Source:     resolveCloneSource(fs),
		FolderName: resolveCloneFolderName(fs),
		TargetDir:  *targetFlag,
		SSHKeyName: *sshKeyFlag,
		Positional: fs.Args(),
		SafePull:   *safePullFlag,
		GHDesktop:  *ghDesktopFlag,
		NoReplace:  *noReplaceFlag,
		Verbose:    *verboseFlag,
	}
}

// resolveCloneSource returns the clone source from positional args.
func resolveCloneSource(fs *flag.FlagSet) string {
	if fs.NArg() > 0 {
		return fs.Arg(0)
	}

	return ""
}

// resolveCloneFolderName returns the optional folder name (second positional arg).
// When the second positional looks like a URL, it's NOT a folder name —
// callers must treat the full positional list as a multi-URL batch instead.
func resolveCloneFolderName(fs *flag.FlagSet) string {
	if fs.NArg() > 1 {
		second := fs.Arg(1)
		if isLikelyURL(second) {
			return ""
		}

		return second
	}

	return ""
}

// isLikelyURL is a cheap prefix check used to disambiguate
// "folder name" vs "second URL" without importing the clone package.
func isLikelyURL(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))

	return strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "git@")
}
