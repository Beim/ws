package command

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dtuit/ws/internal/git"
	"github.com/dtuit/ws/internal/manifest"
	"github.com/dtuit/ws/internal/term"
)

// Pull runs git pull across repos with progress and per-repo output.
func Pull(m *manifest.Manifest, parentDir, filter string) error {
	repos := m.ResolveFilter(filter)
	if len(repos) == 0 {
		fmt.Println("No repos matched the filter.")
		return nil
	}

	workers := git.Workers(len(repos))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	failCount := 0
	done := 0
	total := len(repos)

	maxName := 0
	for _, r := range repos {
		if len(r.Name) > maxName {
			maxName = len(r.Name)
		}
	}

	noPromptEnv := append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_SSH_COMMAND=ssh -o BatchMode=yes",
	)

	fi, _ := os.Stdout.Stat()
	isTTY := fi != nil && fi.Mode()&os.ModeCharDevice != 0

	for _, repo := range repos {
		wg.Add(1)
		go func(r manifest.RepoInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			repoDir := filepath.Join(parentDir, r.Name)
			prefix := fmt.Sprintf("%-*s | ", maxName, r.Name)

			if _, err := os.Stat(filepath.Join(repoDir, ".git")); err != nil {
				mu.Lock()
				done++
				fmt.Fprintf(os.Stderr, "%sskipped (not cloned)\n", prefix)
				mu.Unlock()
				return
			}

			cmd := exec.Command("git", "pull", "--ff-only")
			cmd.Dir = repoDir
			cmd.Stdin = nil
			cmd.Env = noPromptEnv
			output, err := cmd.CombinedOutput()

			mu.Lock()
			defer mu.Unlock()

			done++
			pullOutput := strings.TrimSpace(string(output))

			if isTTY {
				fmt.Fprint(os.Stderr, "\r\033[K")
			}

			if err != nil {
				fmt.Fprintf(os.Stderr, "%s%s\n", prefix, term.Colorize(term.Red, "failed: "+err.Error()))
				if pullOutput != "" {
					for _, line := range strings.Split(pullOutput, "\n") {
						fmt.Fprintf(os.Stderr, "%s%s\n", prefix, line)
					}
				}
				failCount++
			} else if pullOutput != "" && pullOutput != "Already up to date." {
				for _, line := range strings.Split(pullOutput, "\n") {
					fmt.Println(prefix + line)
				}
			}

			if isTTY && done < total {
				fmt.Fprintf(os.Stderr, "\r\033[Kpulling... %d/%d", done, total)
			}
		}(repo)
	}

	wg.Wait()
	if isTTY {
		fmt.Fprint(os.Stderr, "\r\033[K")
	}
	fmt.Printf("Pulled %d repo(s).\n", total-failCount)

	if failCount > 0 {
		return fmt.Errorf("%d repo(s) failed", failCount)
	}
	return nil
}
