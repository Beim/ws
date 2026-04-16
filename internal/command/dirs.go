package command

import (
	"fmt"

	"github.com/dtuit/ws/internal/git"
	"github.com/dtuit/ws/internal/manifest"
)

// Dirs prints repo name and absolute path pairs, one per line.
// Output is tab-separated for easy parsing by scripts and AI agents.
func Dirs(m *manifest.Manifest, wsHome, filter string, includeWorktrees bool, root bool) error {
	if root {
		fmt.Println(wsHome)
		return nil
	}

	repos, err := resolveCommandRepos(m, wsHome, filter, includeWorktrees)
	if err != nil {
		return err
	}

	for _, repo := range repos {
		if !git.IsCheckout(repo.Path) {
			continue
		}
		fmt.Printf("%s\t%s\n", repo.Name, repo.Path)
	}
	return nil
}
