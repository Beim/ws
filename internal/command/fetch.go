package command

import (
	"fmt"
	"os"

	"github.com/dtuit/ws/internal/git"
	"github.com/dtuit/ws/internal/manifest"
)

// Fetch runs git fetch --prune across repos with progress and per-repo output.
// With no remotes specified, fetches every configured remote per repo
// (`git fetch --all --prune`). With one or more remotes specified, fetches
// each named remote independently and skips repos that don't declare it.
func Fetch(m *manifest.Manifest, wsHome, filter string, remotes []string) error {
	repos, err := resolveCommandRepos(m, wsHome, filter, false)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Println("No repos matched the filter.")
		return nil
	}

	workers := git.Workers(len(repos))

	if len(remotes) == 0 {
		failCount := git.RunAll(repos, []string{"git", "fetch", "--all", "--prune"}, workers, git.RunOpts{
			Verb:    "fetching",
			Summary: "Fetched",
		})
		if failCount > 0 {
			return fmt.Errorf("%d repo(s) failed", failCount)
		}
		return nil
	}

	totalFail := 0
	for _, remote := range remotes {
		var matched []manifest.RepoInfo
		for _, r := range repos {
			if _, ok := r.Remotes[remote]; ok {
				matched = append(matched, r)
			} else {
				fmt.Fprintf(os.Stderr, "  %s: no remote %q configured, skipping\n", r.Name, remote)
			}
		}
		if len(matched) == 0 {
			continue
		}
		totalFail += git.RunAll(matched, []string{"git", "fetch", "--prune", remote}, git.Workers(len(matched)), git.RunOpts{
			Verb:    "fetching " + remote,
			Summary: "Fetched " + remote,
		})
	}
	if totalFail > 0 {
		return fmt.Errorf("%d repo(s) failed", totalFail)
	}
	return nil
}
