package command

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dtuit/ws/internal/git"
	"github.com/dtuit/ws/internal/manifest"
)

// Setup clones missing repos and configures the shell.
func Setup(m *manifest.Manifest, parentDir, wsHome, filter string) error {
	repos := m.ResolveFilter(filter)
	if len(repos) == 0 {
		fmt.Println("No repos matched the filter.")
		return nil
	}

	cloned := 0
	for _, repo := range repos {
		repoDir := filepath.Join(parentDir, repo.Name)
		if _, err := os.Stat(repoDir); err == nil {
			continue
		}
		if err := manifest.ValidateURL(repo.URL); err != nil {
			fmt.Fprintf(os.Stderr, "  Skipping %s: %v\n", repo.Name, err)
			continue
		}
		fmt.Printf("  Cloning %s (%s)...\n", repo.Name, repo.Branch)
		if err := git.Clone(parentDir, repo); err != nil {
			fmt.Fprintf(os.Stderr, "  FAILED: %v\n", err)
			continue
		}
		cloned++
	}

	// Count total cloned
	total := 0
	for _, repo := range m.AllRepos() {
		gitDir := filepath.Join(parentDir, repo.Name, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			total++
		}
	}

	if cloned > 0 {
		fmt.Printf("Cloned %d repo(s).\n", cloned)
	}
	fmt.Printf("Setup complete: %d repo(s) on disk.\n", total)

	// Configure shell
	if err := installShellInit(wsHome); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not update shell config: %v\n", err)
	}

	return nil
}

const shellMarkerBegin = "# BEGIN ws"
const shellMarkerEnd = "# END ws"

func shellBlock(wsHome string) string {
	return fmt.Sprintf(`%s
export WS_HOME=%q
ws() {
  case "$1" in
    cd)
      local dir
      dir="$(command ws cd "${@:2}")" && cd "$dir"
      ;;
    *)
      command ws "$@"
      ;;
  esac
}
%s`, shellMarkerBegin, wsHome, shellMarkerEnd)
}

func installShellInit(wsHome string) error {
	rcFile := shellRCPath()
	if rcFile == "" {
		return nil
	}

	block := shellBlock(wsHome)

	content := ""
	if data, err := os.ReadFile(rcFile); err == nil {
		content = string(data)
	}

	// Check if block already exists and is current
	re := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(shellMarkerBegin) + `.*?` + regexp.QuoteMeta(shellMarkerEnd))
	if re.MatchString(content) {
		existing := re.FindString(content)
		if existing == block {
			return nil // already up to date
		}
		// Replace existing block
		content = re.ReplaceAllString(content, block)
		fmt.Printf("Updated shell config in %s\n", rcFile)
	} else {
		// Append new block
		if !strings.HasSuffix(content, "\n") && content != "" {
			content += "\n"
		}
		content += "\n" + block + "\n"
		fmt.Printf("Added shell config to %s\n", rcFile)
	}

	return os.WriteFile(rcFile, []byte(content), 0644)
}

func shellRCPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	// Prefer zshrc if zsh is the shell, otherwise bashrc
	shell := os.Getenv("SHELL")
	if strings.Contains(shell, "zsh") {
		return filepath.Join(home, ".zshrc")
	}
	return filepath.Join(home, ".bashrc")
}
