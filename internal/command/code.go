package command

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dtuit/ws/internal/manifest"
)

// Code generates a VS Code workspace file and opens it.
func Code(m *manifest.Manifest, parentDir, wsHome, filter string) error {
	repos := m.ResolveFilter(filter)

	wsFile := filepath.Join(wsHome, m.Workspace)

	// Compute relative path from workspace file to repo root
	relRoot, err := filepath.Rel(wsHome, parentDir)
	if err != nil {
		relRoot = parentDir
	}

	ws := buildWorkspace(repos, relRoot)

	out, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write
	tmp := wsFile + ".tmp"
	if err := os.WriteFile(tmp, append(out, '\n'), 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, wsFile); err != nil {
		return err
	}

	fmt.Printf("Generated %s (%d repos)\n", m.Workspace, len(repos))

	// Open in VS Code if available
	if codeBin, err := exec.LookPath("code"); err == nil {
		cmd := exec.Command(codeBin, wsFile)
		cmd.Start()
	}

	return nil
}

// buildWorkspace creates the VS Code workspace JSON structure.
func buildWorkspace(repos []manifest.RepoInfo, relRoot string) map[string]interface{} {
	folders := []interface{}{
		map[string]interface{}{"name": "~ workspace", "path": "."},
	}
	for _, repo := range repos {
		folders = append(folders, map[string]interface{}{
			"name": repo.Name,
			"path": filepath.Join(relRoot, repo.Name),
		})
	}

	return map[string]interface{}{
		"folders": folders,
		"settings": map[string]interface{}{
			"files.exclude": map[string]interface{}{
				"**/.git": true,
			},
		},
	}
}

// BuildWorkspaceJSON is exported for testing.
func BuildWorkspaceJSON(repos []manifest.RepoInfo, relRoot string) ([]byte, error) {
	ws := buildWorkspace(repos, relRoot)
	out, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
