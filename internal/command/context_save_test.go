package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dtuit/ws/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveContextGroup_WritesManifest(t *testing.T) {
	wsHome := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wsHome, "manifest.yml"), []byte(`
root: repos
remotes:
  origin: git@example.com:org
groups:
  existing: [repo-c]
repos:
  repo-a:
  repo-b:
  repo-c:
`), 0644))

	m, err := manifest.Load(filepath.Join(wsHome, "manifest.yml"))
	require.NoError(t, err)
	require.NoError(t, saveStoredContextState(wsHome, "repo-a,repo-b", []manifest.RepoInfo{
		{Name: "repo-a"},
		{Name: "repo-b"},
	}, nil))

	require.NoError(t, SaveContextGroup(m, wsHome, "focus", false))

	saved, err := manifest.Load(filepath.Join(wsHome, "manifest.yml"))
	require.NoError(t, err)
	assert.Equal(t, []string{"repo-a", "repo-b"}, saved.Groups["focus"])
	assert.Equal(t, []string{"repo-c"}, saved.Groups["existing"])
}

func TestSaveContextGroup_LocalCreatesManifestLocal(t *testing.T) {
	wsHome := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wsHome, "manifest.yml"), []byte(`
root: repos
remotes:
  origin: git@example.com:org
repos:
  repo-a:
  repo-b:
`), 0644))

	m, err := manifest.Load(filepath.Join(wsHome, "manifest.yml"))
	require.NoError(t, err)
	require.NoError(t, saveStoredContextState(wsHome, "repo-a,repo-b", []manifest.RepoInfo{
		{Name: "repo-a"},
		{Name: "repo-b"},
	}, nil))

	require.NoError(t, SaveContextGroup(m, wsHome, "focus", true))

	merged, err := manifest.LoadWithLocal(wsHome)
	require.NoError(t, err)
	assert.Equal(t, []string{"repo-a", "repo-b"}, merged.Groups["focus"])
}

func TestSaveContextGroup_PreservesManifestWhitespace(t *testing.T) {
	wsHome := t.TempDir()
	manifestPath := filepath.Join(wsHome, "manifest.yml")
	original := `# Header

root: repos

remotes:
  origin: git@example.com:org

groups:
  existing: [repo-c]

# Separator comment
repos:
  repo-a:
  repo-b:
  repo-c:
`
	require.NoError(t, os.WriteFile(manifestPath, []byte(original), 0644))

	m, err := manifest.Load(manifestPath)
	require.NoError(t, err)
	require.NoError(t, saveStoredContextState(wsHome, "repo-a,repo-b", []manifest.RepoInfo{
		{Name: "repo-a"},
		{Name: "repo-b"},
	}, nil))

	require.NoError(t, SaveContextGroup(m, wsHome, "focus", false))

	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	assert.Equal(t, `# Header

root: repos

remotes:
  origin: git@example.com:org

groups:
  existing: [repo-c]
  focus:
    - repo-a
    - repo-b

# Separator comment
repos:
  repo-a:
  repo-b:
  repo-c:
`, string(data))
}

func TestSaveContextGroup_CollapsesWorktreesToBaseRepos(t *testing.T) {
	wsHome := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wsHome, "manifest.yml"), []byte(`
root: repos
remotes:
  origin: git@example.com:org
repos:
  repo:
`), 0644))

	m, err := manifest.Load(filepath.Join(wsHome, "manifest.yml"))
	require.NoError(t, err)
	require.NoError(t, saveStoredContextState(wsHome, "repo@repo-feature,repo", []manifest.RepoInfo{
		{Name: "repo@repo-feature", Worktree: "repo-feature"},
		{Name: "repo"},
	}, nil))

	require.NoError(t, SaveContextGroup(m, wsHome, "focus", false))

	saved, err := manifest.Load(filepath.Join(wsHome, "manifest.yml"))
	require.NoError(t, err)
	assert.Equal(t, []string{"repo"}, saved.Groups["focus"])
}

func TestSaveContextGroup_LocalPreservesWorktreeRefs(t *testing.T) {
	wsHome := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wsHome, "manifest.yml"), []byte(`
root: repos
remotes:
  origin: git@example.com:org
repos:
  repo:
`), 0644))

	m, err := manifest.Load(filepath.Join(wsHome, "manifest.yml"))
	require.NoError(t, err)
	require.NoError(t, saveStoredContextState(wsHome, "repo@feature,repo", []manifest.RepoInfo{
		{Name: "repo@feature", Worktree: "feature"},
		{Name: "repo"},
	}, nil))

	require.NoError(t, SaveContextGroup(m, wsHome, "focus", true))

	merged, err := manifest.LoadWithLocal(wsHome)
	require.NoError(t, err)
	assert.Equal(t, []string{"repo@feature", "repo"}, merged.Groups["focus"])
}

func TestSaveContextGroup_RejectsLocalOnlyReposInSharedManifest(t *testing.T) {
	wsHome := t.TempDir()
	m := loadManifestWithLocal(t, wsHome, `
root: repos
remotes:
  origin: git@example.com:org
repos:
  repo-a:
`, `
repos:
  local-repo:
`)
	require.NoError(t, saveStoredContextState(wsHome, "local-repo", []manifest.RepoInfo{
		{Name: "local-repo"},
	}, nil))

	err := SaveContextGroup(m, wsHome, "focus", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "local-repo")
	assert.Contains(t, err.Error(), "--local")
	assert.Contains(t, err.Error(), "manifest.local.yml")
}

func TestSaveContextGroup_RejectsWhenNoContextIsSet(t *testing.T) {
	wsHome := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wsHome, "manifest.yml"), []byte(`
root: repos
remotes:
  origin: git@example.com:org
repos:
  repo-a:
`), 0644))

	m, err := manifest.Load(filepath.Join(wsHome, "manifest.yml"))
	require.NoError(t, err)

	err = SaveContextGroup(m, wsHome, "focus", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no context set")
}

func TestValidateContextGroupName_RejectsReservedActivityFilters(t *testing.T) {
	m, err := parseManifestYAML(`
remotes:
  origin: git@example.com:org
repos:
  repo-a:
`)
	require.NoError(t, err)

	for _, group := range []string{activeFilterToken, dirtyFilterToken, mineFilterToken, "mine:1d", "active:1d"} {
		err = validateContextGroupName(m, group)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reserved filter name")
	}
}

func TestGroupWithWorktreeMembers_ResolvesCorrectly(t *testing.T) {
	wsHome := t.TempDir()
	repoRoot := filepath.Join(wsHome, "repos")

	// Create a git repo with a worktree
	initCheckout(t, filepath.Join(repoRoot, "repo"))
	runGit(t, filepath.Join(repoRoot, "repo"), "worktree", "add", "-b", "feature", filepath.Join(repoRoot, "repo-feature"))

	require.NoError(t, os.WriteFile(filepath.Join(wsHome, "manifest.yml"), []byte(`
root: repos
remotes:
  origin: git@example.com:org
groups:
  feature-work: [repo@repo-feature]
repos:
  repo:
`), 0644))

	m, err := manifest.LoadWithLocal(wsHome)
	require.NoError(t, err)

	// Resolve the group — should find the worktree
	repos, err := resolveCommandRepos(m, wsHome, "feature-work", false)
	require.NoError(t, err)
	require.Len(t, repos, 1)
	assert.Equal(t, "repo@repo-feature", repos[0].Name)
	assert.Equal(t, filepath.Join(repoRoot, "repo-feature"), repos[0].Path)
}

func TestGroupWithWorktreeMembers_SkipsMissingWorktree(t *testing.T) {
	wsHome := t.TempDir()
	repoRoot := filepath.Join(wsHome, "repos")

	initCheckout(t, filepath.Join(repoRoot, "repo"))
	// No worktree created — group member should be skipped with warning

	require.NoError(t, os.WriteFile(filepath.Join(wsHome, "manifest.yml"), []byte(`
root: repos
remotes:
  origin: git@example.com:org
groups:
  stale: [repo@nonexistent]
repos:
  repo:
`), 0644))

	m, err := manifest.LoadWithLocal(wsHome)
	require.NoError(t, err)

	repos, err := resolveCommandRepos(m, wsHome, "stale", false)
	require.NoError(t, err)
	assert.Empty(t, repos)
}

func TestGroupWithMixedMembers_ResolvesAll(t *testing.T) {
	wsHome := t.TempDir()
	repoRoot := filepath.Join(wsHome, "repos")

	initCheckout(t, filepath.Join(repoRoot, "api"))
	initCheckout(t, filepath.Join(repoRoot, "web"))
	runGit(t, filepath.Join(repoRoot, "api"), "worktree", "add", "-b", "feature", filepath.Join(repoRoot, "api-feature"))

	require.NoError(t, os.WriteFile(filepath.Join(wsHome, "manifest.yml"), []byte(`
root: repos
remotes:
  origin: git@example.com:org
groups:
  feature: [api@api-feature, web]
repos:
  api:
  web:
`), 0644))

	m, err := manifest.LoadWithLocal(wsHome)
	require.NoError(t, err)

	repos, err := resolveCommandRepos(m, wsHome, "feature", false)
	require.NoError(t, err)
	require.Len(t, repos, 2)
	assert.Equal(t, "api@api-feature", repos[0].Name)
	assert.Equal(t, "web", repos[1].Name)
}

func TestPreserveContextMembers_KeepsWorktreeRefs(t *testing.T) {
	active := map[string]manifest.RepoConfig{"repo": {}}
	members, invalid := preserveContextMembers(
		[]string{"repo@feature", "repo"},
		active,
	)
	assert.Equal(t, []string{"repo@feature", "repo"}, members)
	assert.Empty(t, invalid)
}

func TestPreserveContextMembers_RejectsUnknownRepos(t *testing.T) {
	active := map[string]manifest.RepoConfig{"repo": {}}
	_, invalid := preserveContextMembers(
		[]string{"unknown@feature"},
		active,
	)
	assert.Equal(t, []string{"unknown@feature"}, invalid)
}

func TestCompleteContextSuggestsSave(t *testing.T) {
	m, err := parseManifestYAML(`
remotes:
  origin: git@example.com:org
repos:
  repo-a:
`)
	require.NoError(t, err)

	result := Complete(m, []string{"context", ""}, 1)
	assert.Contains(t, result.Values, "save")
}

func TestCompleteContextSaveSuggestsLocal(t *testing.T) {
	m, err := parseManifestYAML(`
remotes:
  origin: git@example.com:org
repos:
  repo-a:
`)
	require.NoError(t, err)

	result := Complete(m, []string{"context", "save", ""}, 2)
	assert.Equal(t, []string{"--local"}, result.Values)
}
