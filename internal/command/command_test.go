package command

import (
	"encoding/json"
	"testing"

	"github.com/dtuit/ws/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildWorkspaceJSON(t *testing.T) {
	repos := []manifest.RepoInfo{
		{Name: "repo-a"},
		{Name: "repo-b"},
	}

	out, err := BuildWorkspaceJSON(repos, "..")
	require.NoError(t, err)

	var ws map[string]interface{}
	require.NoError(t, json.Unmarshal(out, &ws))

	// Has default settings
	settings, ok := ws["settings"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, settings, "files.exclude")

	// Folders: workspace + 2 repos
	folders, ok := ws["folders"].([]interface{})
	require.True(t, ok)
	assert.Len(t, folders, 3)

	// First folder is workspace root
	first := folders[0].(map[string]interface{})
	assert.Equal(t, "~ workspace", first["name"])
	assert.Equal(t, ".", first["path"])

	// Repo folders have correct paths
	second := folders[1].(map[string]interface{})
	assert.Equal(t, "repo-a", second["name"])
	assert.Equal(t, "../repo-a", second["path"])
}

func TestBuildWorkspaceJSON_EmptyRepos(t *testing.T) {
	out, err := BuildWorkspaceJSON(nil, "..")
	require.NoError(t, err)

	var ws map[string]interface{}
	require.NoError(t, json.Unmarshal(out, &ws))
	folders := ws["folders"].([]interface{})
	assert.Len(t, folders, 1) // just workspace root
}

func TestBuildWorkspaceJSON_CustomRoot(t *testing.T) {
	repos := []manifest.RepoInfo{{Name: "my-repo"}}

	out, err := BuildWorkspaceJSON(repos, "/abs/path/to/repos")
	require.NoError(t, err)

	var ws map[string]interface{}
	require.NoError(t, json.Unmarshal(out, &ws))
	folders := ws["folders"].([]interface{})
	second := folders[1].(map[string]interface{})
	assert.Equal(t, "/abs/path/to/repos/my-repo", second["path"])
}

func TestParseSuperArgs_WithGroup(t *testing.T) {
	m, err := manifest.Parse([]byte(`
remotes:
  default: git@example.com
groups:
  ai: [repo-a]
repos:
  repo-a:
`))
	require.NoError(t, err)

	filter, cmdArgs := ParseSuperArgs(m, []string{"ai", "git", "status"})
	assert.Equal(t, "ai", filter)
	assert.Equal(t, []string{"git", "status"}, cmdArgs)
}

func TestParseSuperArgs_WithoutGroup(t *testing.T) {
	m, err := manifest.Parse([]byte(`
remotes:
  default: git@example.com
repos:
  repo-a:
`))
	require.NoError(t, err)

	filter, cmdArgs := ParseSuperArgs(m, []string{"git", "status"})
	assert.Equal(t, "", filter)
	assert.Equal(t, []string{"git", "status"}, cmdArgs)
}

func TestParseSuperArgs_Empty(t *testing.T) {
	m, _ := manifest.Parse([]byte(`
remotes:
  default: git@example.com
repos:
  repo-a:
`))

	filter, cmdArgs := ParseSuperArgs(m, nil)
	assert.Equal(t, "", filter)
	assert.Nil(t, cmdArgs)
}
