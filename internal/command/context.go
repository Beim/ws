package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dtuit/ws/internal/manifest"
)

const contextFile = ".ws-context"
const scopeDir = ".scope"

// SetContext sets the default filter, regenerates the VS Code workspace,
// and updates the repos/ symlink directory to match.
func SetContext(m *manifest.Manifest, wsHome, filter string) error {
	path := filepath.Join(wsHome, contextFile)

	if filter == "" || filter == "none" || filter == "reset" {
		os.Remove(path)
		fmt.Println("Context cleared.")
		if err := syncReposDir(m, wsHome, ""); err != nil {
			return err
		}
		return Code(m, wsHome, "")
	}

	repos := m.ResolveFilter(filter, wsHome)
	if len(repos) == 0 {
		return fmt.Errorf("filter %q matched no repos", filter)
	}

	if err := os.WriteFile(path, []byte(filter+"\n"), 0644); err != nil {
		return err
	}
	fmt.Printf("Context set to %q (%d repos)\n", filter, len(repos))

	if err := syncReposDir(m, wsHome, filter); err != nil {
		return err
	}
	return Code(m, wsHome, filter)
}

// GetContext reads the current context filter, or "" if none is set.
func GetContext(wsHome string) string {
	data, err := os.ReadFile(filepath.Join(wsHome, contextFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// ShowContext displays the current context.
func ShowContext(m *manifest.Manifest, wsHome string) {
	ctx := GetContext(wsHome)
	if ctx == "" {
		fmt.Println("No context set (using all grouped repos)")
		return
	}
	repos := m.ResolveFilter(ctx, wsHome)
	fmt.Printf("Context: %s (%d repos)\n", ctx, len(repos))
}

// syncReposDir creates/updates a repos/ directory with symlinks to the scoped repos.
// This constrains what filesystem-based agents (CLI tools, Claude Code) can see.
func syncReposDir(m *manifest.Manifest, wsHome, filter string) error {
	repos := m.ResolveFilter(filter, wsHome)
	dir := filepath.Join(wsHome, scopeDir)

	// Remove existing symlinks (but not non-symlink entries, for safety)
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			p := filepath.Join(dir, e.Name())
			if e.Type()&os.ModeSymlink != 0 {
				os.Remove(p)
			}
		}
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating repos dir: %w", err)
	}

	// Create symlinks for scoped repos
	for _, repo := range repos {
		target := repo.Path
		link := filepath.Join(dir, repo.Name)

		// Use relative path for the symlink
		relTarget, err := filepath.Rel(dir, target)
		if err != nil {
			relTarget = target
		}

		if err := os.Symlink(relTarget, link); err != nil && !os.IsExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: could not symlink %s: %v\n", repo.Name, err)
		}
	}

	fmt.Printf(".scope/ updated (%d symlinks)\n", len(repos))
	return nil
}
