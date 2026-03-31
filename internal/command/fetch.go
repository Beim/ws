package command

import (
	"fmt"

	"github.com/dtuit/ws/internal/git"
	"github.com/dtuit/ws/internal/manifest"
)

// Fetch runs git fetch --prune across repos with progress and per-repo output.
func Fetch(m *manifest.Manifest, parentDir, filter string) error {
	repos := m.ResolveFilter(filter)
	if len(repos) == 0 {
		fmt.Println("No repos matched the filter.")
		return nil
	}

	workers := git.Workers(len(repos))
	failCount := git.RunAll(parentDir, repos, []string{"git", "fetch", "--prune"}, workers, git.RunOpts{
		Verb:    "fetching",
		Summary: "Fetched",
	})
	if failCount > 0 {
		return fmt.Errorf("%d repo(s) failed", failCount)
	}
	return nil
}
