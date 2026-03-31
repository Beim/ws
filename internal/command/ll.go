package command

import (
	"fmt"

	"github.com/dtuit/ws/internal/git"
	"github.com/dtuit/ws/internal/manifest"
	"github.com/dtuit/ws/internal/term"
)

// LL displays a dashboard of repo status: branch, dirty state, last commit.
func LL(m *manifest.Manifest, parentDir, filter string) error {
	repos := m.ResolveFilter(filter)
	if len(repos) == 0 {
		fmt.Println("No repos matched the filter.")
		return nil
	}

	workers := git.Workers(len(repos))
	statuses := git.StatusAll(parentDir, repos, workers)

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

		// Status symbols: +staged *unstaged ?untracked $stashed
		symbols := s.Symbols()
		sync := s.SyncSymbol()

		// Color based on state
		var color string
		switch {
		case s.Branch == "(detached)":
			color = term.Red
		case s.Ahead > 0 && s.Behind > 0:
			color = term.Red
		case s.Ahead > 0:
			color = term.Magenta
		case s.Behind > 0:
			color = term.Yellow
		case s.IsDirty():
			color = term.Yellow
		case s.NoRemote:
			color = term.Cyan
		default:
			color = term.Green
		}

		// Pad symbols to fixed width for alignment
		symbolStr := fmt.Sprintf("%-4s", symbols)
		syncStr := fmt.Sprintf("%-4s", sync)

		msg := s.CommitMsg
		if len(msg) > 60 {
			msg = msg[:57] + "..."
		}

		age := ""
		if s.CommitAge != "" {
			age = " " + term.Colorize(term.Dim, "("+s.CommitAge+")")
		}

		fmt.Printf("%s %s  %s%s\n",
			term.Colorize(color, fmt.Sprintf("%-*s  %-*s %s[%s]", maxName, s.Name, maxBranch, s.Branch, syncStr, symbolStr)),
			"",
			msg, age)
	}
	return nil
}
