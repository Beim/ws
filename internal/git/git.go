package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/dtuit/ws/internal/manifest"
)

// RepoStatus holds the result of querying a single repo's git state.
type RepoStatus struct {
	Name      string
	Branch    string
	Staged    bool // indexed changes ready to commit
	Unstaged  bool // working tree changes
	Untracked bool // untracked files
	Stashed   bool // stash entries exist
	Ahead     int  // commits ahead of upstream
	Behind    int  // commits behind upstream
	NoRemote  bool // no upstream tracking branch
	CommitMsg string
	CommitAge string
	Err       error
}

// Symbols returns a compact status string like gita: +*?$ for dirty indicators.
func (s RepoStatus) Symbols() string {
	var b strings.Builder
	if s.Staged {
		b.WriteByte('+')
	}
	if s.Unstaged {
		b.WriteByte('*')
	}
	if s.Untracked {
		b.WriteByte('?')
	}
	if s.Stashed {
		b.WriteByte('$')
	}
	return b.String()
}

// SyncSymbol returns the remote sync indicator.
func (s RepoStatus) SyncSymbol() string {
	switch {
	case s.NoRemote:
		return "~"
	case s.Ahead > 0 && s.Behind > 0:
		return fmt.Sprintf("%d⇕%d", s.Ahead, s.Behind)
	case s.Ahead > 0:
		return fmt.Sprintf("↑%d", s.Ahead)
	case s.Behind > 0:
		return fmt.Sprintf("↓%d", s.Behind)
	default:
		return "="
	}
}

// IsDirty returns true if the working tree has any local changes.
func (s RepoStatus) IsDirty() bool {
	return s.Staged || s.Unstaged || s.Untracked
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

	// Detect staged changes: index differs from HEAD
	if _, err := gitCmd(repoDir, "diff", "--cached", "--quiet"); err != nil {
		s.Staged = true
	}

	// Detect unstaged changes: working tree differs from index
	if _, err := gitCmd(repoDir, "diff", "--quiet"); err != nil {
		s.Unstaged = true
	}

	// Detect untracked files
	untracked, _ := gitCmd(repoDir, "ls-files", "--others", "--exclude-standard")
	if untracked != "" {
		s.Untracked = true
	}

	// Check for stash
	stashFile := filepath.Join(repoDir, ".git", "logs", "refs", "stash")
	if _, err := os.Stat(stashFile); err == nil {
		s.Stashed = true
	}

	// Get ahead/behind upstream
	counts, err := gitCmd(repoDir, "rev-list", "--left-right", "--count", "@{u}...HEAD")
	if err != nil {
		s.NoRemote = true
	} else {
		parts := strings.Fields(counts)
		if len(parts) == 2 {
			s.Behind, _ = strconv.Atoi(parts[0])
			s.Ahead, _ = strconv.Atoi(parts[1])
		}
	}

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

// Workers returns the effective worker count: WS_WORKERS env, or min(cpus, repoCount).
func Workers(repoCount int) int {
	if env := os.Getenv("WS_WORKERS"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			return n
		}
	}
	cpus := runtime.NumCPU()
	if repoCount < cpus {
		return repoCount
	}
	return cpus
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

// ExecResult holds the outcome of running a command in one repo.
type ExecResult struct {
	Name   string
	Output string
	Err    error
}

// Exec runs a command in each repo dir in parallel, printing prefixed output.
// Failed commands are retried synchronously (handles TTY/stdin issues).
// Returns the number of repos that failed.
func Exec(parentDir string, repos []manifest.RepoInfo, cmdArgs []string, maxWorkers int) int {
	// Calculate max name length for alignment
	maxName := 0
	for _, r := range repos {
		if len(r.Name) > maxName {
			maxName = len(r.Name)
		}
	}

	// Phase 1: parallel execution
	results := make([]ExecResult, len(repos))
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for i, repo := range repos {
		wg.Add(1)
		go func(idx int, r manifest.RepoInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			repoDir := filepath.Join(parentDir, r.Name)
			if _, err := os.Stat(filepath.Join(repoDir, ".git")); err != nil {
				results[idx] = ExecResult{Name: r.Name, Err: fmt.Errorf("not cloned")}
				return
			}

			cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
			cmd.Dir = repoDir
			output, err := cmd.CombinedOutput()
			results[idx] = ExecResult{Name: r.Name, Output: string(output), Err: err}
		}(i, repo)
	}
	wg.Wait()

	// Phase 2: print results and retry failures synchronously
	failCount := 0
	for i, res := range results {
		prefix := fmt.Sprintf("%-*s | ", maxName, res.Name)

		if res.Err != nil && res.Err.Error() == "not cloned" {
			fmt.Fprintf(os.Stderr, "%sskipped (not cloned)\n", prefix)
			continue
		}

		// Retry failed commands synchronously (handles TTY/pipe issues)
		if res.Err != nil {
			repoDir := filepath.Join(parentDir, repos[i].Name)
			cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
			cmd.Dir = repoDir
			output, err := cmd.CombinedOutput()
			res = ExecResult{Name: res.Name, Output: string(output), Err: err}
			results[i] = res
		}

		text := strings.TrimRight(res.Output, "\n")
		if text != "" {
			for _, line := range strings.Split(text, "\n") {
				fmt.Println(prefix + line)
			}
		}
		if res.Err != nil {
			fmt.Fprintf(os.Stderr, "%sfailed: %v\n", prefix, res.Err)
			failCount++
		}
	}

	return failCount
}

// Clone clones a single repo.
func Clone(parentDir string, repo manifest.RepoInfo) error {
	repoDir := filepath.Join(parentDir, repo.Name)
	cmd := exec.Command("git", "clone", "-b", repo.Branch, "--single-branch", "--", repo.URL, repoDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
