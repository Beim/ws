package command

import (
	"fmt"

	"github.com/dtuit/ws/internal/git"
	"github.com/dtuit/ws/internal/manifest"
)

// Pull runs git pull --ff-only across repos with progress and per-repo output.
func Pull(m *manifest.Manifest, parentDir, filter string) error {
	repos := m.ResolveFilter(filter)
	if len(repos) == 0 {
		fmt.Println("No repos matched the filter.")
		return nil
	}

	workers := git.Workers(len(repos))
	failCount := git.RunAll(parentDir, repos, []string{"git", "pull", "--ff-only"}, workers, git.RunOpts{
		Verb:     "pulling",
		Summary:  "Pulled",
		Suppress: "Already up to date.",
	})
	if failCount > 0 {
		return fmt.Errorf("%d repo(s) failed", failCount)
	}
	return nil
}
