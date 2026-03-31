package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"bitbucket.org/xtracta/ws/internal/manifest"
)

// RepoStatus holds the result of querying a single repo's git state.
type RepoStatus struct {
	Name      string
	Branch    string
	Dirty     bool
	CommitMsg string
	CommitAge string
	Err       error
}

// gitCmd runs a git command in the given directory and returns trimmed stdout.
func gitCmd(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(stderr.String()), err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Status queries the git status of a single repo.
func Status(repoDir, name string) RepoStatus {
	s := RepoStatus{Name: name}

	gitDir := filepath.Join(repoDir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		s.Err = fmt.Errorf("not cloned")
		return s
	}

	// Get branch
	branch, err := gitCmd(repoDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		s.Err = err
		return s
	}
	if branch == "HEAD" {
		s.Branch = "(detached)"
	} else {
		s.Branch = branch
	}

	// Get dirty status
	porcelain, _ := gitCmd(repoDir, "status", "--porcelain")
	s.Dirty = len(porcelain) > 0

	// Get last commit
	logOut, err := gitCmd(repoDir, "log", "-1", "--format=%s\x1f%ar")
	if err != nil {
		s.CommitMsg = "(no commits)"
		return s
	}
	parts := strings.SplitN(logOut, "\x1f", 2)
	if len(parts) == 2 {
		s.CommitMsg = parts[0]
		s.CommitAge = parts[1]
	}

	return s
}

// StatusAll queries git status for multiple repos in parallel.
func StatusAll(parentDir string, repos []manifest.RepoInfo, maxWorkers int) []RepoStatus {
	results := make([]RepoStatus, len(repos))
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for i, repo := range repos {
		wg.Add(1)
		go func(idx int, r manifest.RepoInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			repoDir := filepath.Join(parentDir, r.Name)
			results[idx] = Status(repoDir, r.Name)
		}(i, repo)
	}

	wg.Wait()
	return results
}

// Exec runs a command in each repo dir in parallel, printing prefixed output.
// Returns the number of repos that failed.
func Exec(parentDir string, repos []manifest.RepoInfo, cmdArgs []string, maxWorkers int) int {
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxWorkers)
	failCount := 0

	// Calculate max name length for alignment
	maxName := 0
	for _, r := range repos {
		if len(r.Name) > maxName {
			maxName = len(r.Name)
		}
	}

	for _, repo := range repos {
		wg.Add(1)
		go func(r manifest.RepoInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			repoDir := filepath.Join(parentDir, r.Name)
			if _, err := os.Stat(filepath.Join(repoDir, ".git")); err != nil {
				mu.Lock()
				fmt.Fprintf(os.Stderr, "%-*s | skipped (not cloned)\n", maxName, r.Name)
				mu.Unlock()
				return
			}

			cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
			cmd.Dir = repoDir
			output, err := cmd.CombinedOutput()

			mu.Lock()
			defer mu.Unlock()

			prefix := fmt.Sprintf("%-*s | ", maxName, r.Name)
			text := strings.TrimRight(string(output), "\n")
			if text != "" {
				for _, line := range strings.Split(text, "\n") {
					fmt.Println(prefix + line)
				}
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "%sfailed: %v\n", prefix, err)
				failCount++
			}
		}(repo)
	}

	wg.Wait()
	return failCount
}

// Clone clones a single repo.
func Clone(parentDir string, repo manifest.RepoInfo) error {
	repoDir := filepath.Join(parentDir, repo.Name)
	cmd := exec.Command("git", "clone", "-b", repo.Branch, "--single-branch", repo.URL, repoDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
