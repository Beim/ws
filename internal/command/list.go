package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bitbucket.org/xtracta/ws/internal/manifest"
)

// List prints a table of repos from the manifest with their status.
func List(m *manifest.Manifest, parentDir string, showAll bool) error {
	repos := m.AllRepos()
	repoGroups := m.RepoGroups()

	fmt.Printf("%-42s %-16s %-10s %s\n", "REPO", "BRANCH", "GROUPS", "CLONED")
	fmt.Println(strings.Repeat("-", 78))

	for _, r := range repos {
		groups := strings.Join(repoGroups[r.Name], ",")
		if groups == "" {
			groups = "-"
		}
		cloned := "-"
		gitDir := filepath.Join(parentDir, r.Name, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			cloned = "yes"
		}
		fmt.Printf("%-42s %-16s %-10s %s\n", r.Name, r.Branch, groups, cloned)
	}

	if showAll && len(m.Exclude) > 0 {
		fmt.Printf("\n%-42s %s\n", "EXCLUDED", "")
		fmt.Println(strings.Repeat("-", 78))
		for _, name := range m.Exclude {
			fmt.Printf("%-42s (see manifest.yml)\n", name)
		}
	}

	return nil
}
