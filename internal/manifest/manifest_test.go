package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testManifest = `
remotes:
  default: git@bitbucket.org:xtracta
  github: git@github.com:some-org

branch: master

groups:
  ai:  [ai-data-api, ai-gateway, mmdoc]
  eng: [global-auth, xtracta-app]

repos:
  xtracta-app:
  global-auth:
  ai-data-api: { branch: main }
  ai-gateway: { branch: main }
  mmdoc:
  custom-repo: { url: git@custom:org/repo.git }
  github-repo: { remote: github, branch: develop }

exclude:
  - old-repo
  - dead-repo
`

func TestParse(t *testing.T) {
	m, err := Parse([]byte(testManifest))
	require.NoError(t, err)

	assert.Equal(t, "git@bitbucket.org:xtracta", m.Remotes["default"])
	assert.Equal(t, "git@github.com:some-org", m.Remotes["github"])
	assert.Equal(t, "master", m.DefaultBranch)
	assert.Len(t, m.Groups["ai"], 3)
	assert.Len(t, m.Groups["eng"], 2)
	assert.Len(t, m.Repos, 7)
	assert.Len(t, m.Exclude, 2)
}

func TestParse_BareRepoEntry(t *testing.T) {
	m, err := Parse([]byte(testManifest))
	require.NoError(t, err)

	cfg := m.Repos["xtracta-app"]
	assert.Empty(t, cfg.Branch)
	assert.Empty(t, cfg.Remote)
	assert.Empty(t, cfg.URL)
	assert.Equal(t, "master", m.ResolveBranch(cfg))
}

func TestParse_BranchOverride(t *testing.T) {
	m, err := Parse([]byte(testManifest))
	require.NoError(t, err)

	cfg := m.Repos["ai-data-api"]
	assert.Equal(t, "main", cfg.Branch)
	assert.Equal(t, "main", m.ResolveBranch(cfg))
}

func TestParse_BackwardCompat_SingularRemote(t *testing.T) {
	yaml := `
remote: git@bitbucket.org:xtracta
branch: master
repos:
  my-repo:
`
	m, err := Parse([]byte(yaml))
	require.NoError(t, err)

	assert.Equal(t, "git@bitbucket.org:xtracta", m.Remotes["default"])
	assert.Equal(t, "git@bitbucket.org:xtracta/my-repo.git", m.ResolveURL("my-repo", m.Repos["my-repo"]))
}

func TestResolveURL(t *testing.T) {
	m, err := Parse([]byte(testManifest))
	require.NoError(t, err)

	tests := []struct {
		name     string
		expected string
	}{
		{"xtracta-app", "git@bitbucket.org:xtracta/xtracta-app.git"},
		{"custom-repo", "git@custom:org/repo.git"},
		{"github-repo", "git@github.com:some-org/github-repo.git"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, m.ResolveURL(tt.name, m.Repos[tt.name]))
		})
	}
}

func TestResolveFilter_All(t *testing.T) {
	m, err := Parse([]byte(testManifest))
	require.NoError(t, err)

	repos := m.ResolveFilter("all")
	names := repoNames(repos)
	// Should contain all repos that are in at least one group
	assert.Contains(t, names, "ai-data-api")
	assert.Contains(t, names, "ai-gateway")
	assert.Contains(t, names, "mmdoc")
	assert.Contains(t, names, "global-auth")
	assert.Contains(t, names, "xtracta-app")
	// Should NOT contain repos not in any group
	assert.NotContains(t, names, "custom-repo")
	assert.NotContains(t, names, "github-repo")
}

func TestResolveFilter_GroupName(t *testing.T) {
	m, err := Parse([]byte(testManifest))
	require.NoError(t, err)

	repos := m.ResolveFilter("ai")
	names := repoNames(repos)
	assert.Equal(t, []string{"ai-data-api", "ai-gateway", "mmdoc"}, names)
}

func TestResolveFilter_CommaSeparated(t *testing.T) {
	m, err := Parse([]byte(testManifest))
	require.NoError(t, err)

	repos := m.ResolveFilter("ai,eng")
	names := repoNames(repos)
	assert.Contains(t, names, "ai-data-api")
	assert.Contains(t, names, "global-auth")
	assert.Len(t, names, 5) // 3 ai + 2 eng
}

func TestResolveFilter_SingleRepo(t *testing.T) {
	m, err := Parse([]byte(testManifest))
	require.NoError(t, err)

	repos := m.ResolveFilter("mmdoc")
	assert.Len(t, repos, 1)
	assert.Equal(t, "mmdoc", repos[0].Name)
}

func TestResolveFilter_Empty(t *testing.T) {
	m, err := Parse([]byte(testManifest))
	require.NoError(t, err)

	repos := m.ResolveFilter("")
	// Same as "all" — returns grouped repos
	assert.Len(t, repos, 5)
}

func TestAllRepos(t *testing.T) {
	m, err := Parse([]byte(testManifest))
	require.NoError(t, err)

	repos := m.AllRepos()
	assert.Len(t, repos, 7) // all 7 active repos
	// Should be sorted
	assert.Equal(t, "ai-data-api", repos[0].Name)
}

func TestRepoGroups(t *testing.T) {
	m, err := Parse([]byte(testManifest))
	require.NoError(t, err)

	rg := m.RepoGroups()
	assert.Contains(t, rg["ai-data-api"], "ai")
	assert.Contains(t, rg["xtracta-app"], "eng")
	assert.Empty(t, rg["custom-repo"])
}

func TestIsGroupOrRepo(t *testing.T) {
	m, err := Parse([]byte(testManifest))
	require.NoError(t, err)

	assert.True(t, m.IsGroupOrRepo("ai"))
	assert.True(t, m.IsGroupOrRepo("mmdoc"))
	assert.True(t, m.IsGroupOrRepo("ai,eng"))
	assert.False(t, m.IsGroupOrRepo("nonexistent"))
	assert.False(t, m.IsGroupOrRepo("git"))
}

func TestMergeLocal(t *testing.T) {
	dir := t.TempDir()

	// Write main manifest
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.yml"), []byte(testManifest), 0644))

	// Write local override
	localYAML := `
remotes:
  my-fork: git@github.com:darren

repos:
  old-repo:
  my-experiment: { remote: my-fork, branch: dev }

exclude:
  - xtracta-app

groups:
  my-group: [old-repo, my-experiment]
  ai: [ai-data-api]
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.local.yml"), []byte(localYAML), 0644))

	m, err := LoadWithLocal(dir)
	require.NoError(t, err)

	// Remotes merged
	assert.Equal(t, "git@github.com:darren", m.Remotes["my-fork"])
	assert.Equal(t, "git@bitbucket.org:xtracta", m.Remotes["default"]) // preserved

	// Repos merged: old-repo un-excluded, my-experiment added
	assert.Contains(t, m.Repos, "old-repo")
	assert.Contains(t, m.Repos, "my-experiment")
	assert.Equal(t, "dev", m.Repos["my-experiment"].Branch)

	// Exclude merged: xtracta-app added to exclude list
	assert.Contains(t, m.Exclude, "xtracta-app")
	assert.Contains(t, m.Exclude, "dead-repo") // original preserved

	// Groups: ai overridden, my-group added
	assert.Equal(t, []string{"ai-data-api"}, m.Groups["ai"])
	assert.Equal(t, []string{"old-repo", "my-experiment"}, m.Groups["my-group"])
	assert.Equal(t, []string{"global-auth", "xtracta-app"}, m.Groups["eng"]) // preserved

	// old-repo is in both repos and exclude — repos wins, it's active
	active := m.ActiveRepos()
	assert.Contains(t, active, "old-repo")
}

func TestMergeLocal_NoLocalFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.yml"), []byte(testManifest), 0644))

	m, err := LoadWithLocal(dir)
	require.NoError(t, err)
	assert.Len(t, m.Repos, 7) // unchanged
}

func TestParse_DefaultBranch(t *testing.T) {
	yaml := `
remotes:
  default: git@example.com
repos:
  my-repo:
`
	m, err := Parse([]byte(yaml))
	require.NoError(t, err)
	assert.Equal(t, "master", m.DefaultBranch) // default when not specified
}

func repoNames(repos []RepoInfo) []string {
	names := make([]string, len(repos))
	for i, r := range repos {
		names[i] = r.Name
	}
	return names
}
