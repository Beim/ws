package command

import (
	"fmt"
	"os/exec"

	"github.com/dtuit/ws/internal/git"
	"github.com/dtuit/ws/internal/manifest"
)

// Super runs an arbitrary command in each repo directory.
func Super(m *manifest.Manifest, parentDir, filter string, cmdArgs []string) error {
	// Validate the command exists before fanning out
	if _, err := exec.LookPath(cmdArgs[0]); err != nil {
		return fmt.Errorf("command not found: %s", cmdArgs[0])
	}

	repos := m.ResolveFilter(filter)
	if len(repos) == 0 {
		fmt.Println("No repos matched the filter.")
		return nil
	}

	workers := git.Workers(len(repos))
	failCount := git.Exec(parentDir, repos, cmdArgs, workers)
	if failCount > 0 {
		return fmt.Errorf("%d repo(s) failed", failCount)
	}
	return nil
}

// ParseSuperArgs disambiguates the filter and command arguments.
// If the first arg matches a known group or repo, it's the filter.
func ParseSuperArgs(m *manifest.Manifest, args []string) (filter string, cmdArgs []string) {
	if len(args) == 0 {
		return "", nil
	}
	if m.IsGroupOrRepo(args[0]) {
		return args[0], args[1:]
	}
	return "", args
}
