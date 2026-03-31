package command

import (
	"fmt"

	"bitbucket.org/xtracta/ws/internal/git"
	"bitbucket.org/xtracta/ws/internal/manifest"
	"bitbucket.org/xtracta/ws/internal/term"
)

// LL displays a dashboard of repo status: branch, dirty state, last commit.
func LL(m *manifest.Manifest, parentDir, filter string) error {
	repos := m.ResolveFilter(filter)
	if len(repos) == 0 {
		fmt.Println("No repos matched the filter.")
		return nil
	}

	statuses := git.StatusAll(parentDir, repos, 20)

	// Calculate column widths
	maxName, maxBranch := 0, 0
	for _, s := range statuses {
		if len(s.Name) > maxName {
			maxName = len(s.Name)
		}
		if len(s.Branch) > maxBranch {
			maxBranch = len(s.Branch)
		}
	}

	for _, s := range statuses {
		if s.Err != nil {
			fmt.Printf("%-*s  %s\n", maxName, s.Name,
				term.Colorize(term.Red, s.Err.Error()))
			continue
		}

		var color string
		switch {
		case s.Branch == "(detached)":
			color = term.Red
		case s.Dirty:
			color = term.Yellow
		default:
			color = term.Green
		}

		dirty := " "
		if s.Dirty {
			dirty = "*"
		}

		msg := s.CommitMsg
		if len(msg) > 70 {
			msg = msg[:67] + "..."
		}

		age := ""
		if s.CommitAge != "" {
			age = " " + term.Colorize(term.Dim, "("+s.CommitAge+")")
		}

		fmt.Printf("%s  %s%s\n",
			term.Colorize(color, fmt.Sprintf("%-*s  %-*s [%s]", maxName, s.Name, maxBranch, s.Branch, dirty)),
			msg, age)
	}
	return nil
}
