package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dtuit/ws/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initTestRepo creates a git repo with one commit in a temp directory.
func initTestRepo(t *testing.T, dir, name string) string {
	t.Helper()
	repoDir := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial commit"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "cmd %v failed: %s", args, out)
	}
	return repoDir
}

func TestStatus_CleanRepo(t *testing.T) {
	dir := t.TempDir()
	repoDir := initTestRepo(t, dir, "clean-repo")

	s := Status(repoDir, "clean-repo")
	assert.NoError(t, s.Err)
	assert.Equal(t, "master", s.Branch)
	assert.False(t, s.Dirty)
	assert.Equal(t, "initial commit", s.CommitMsg)
	assert.NotEmpty(t, s.CommitAge)
}

func TestStatus_DirtyRepo(t *testing.T) {
	dir := t.TempDir()
	repoDir := initTestRepo(t, dir, "dirty-repo")

	// Create an untracked file
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "new.txt"), []byte("hello"), 0644))

	s := Status(repoDir, "dirty-repo")
	assert.NoError(t, s.Err)
	assert.True(t, s.Dirty)
}

func TestStatus_DetachedHead(t *testing.T) {
	dir := t.TempDir()
	repoDir := initTestRepo(t, dir, "detached-repo")

	// Get current commit hash and checkout detached
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	hash, err := cmd.Output()
	require.NoError(t, err)

	cmd = exec.Command("git", "checkout", "--detach", "HEAD")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	s := Status(repoDir, "detached-repo")
	assert.NoError(t, s.Err)
	assert.Equal(t, "(detached)", s.Branch)
	_ = hash
}

func TestStatus_MissingDir(t *testing.T) {
	s := Status("/nonexistent/path", "missing")
	assert.Error(t, s.Err)
	assert.Contains(t, s.Err.Error(), "not cloned")
}

func TestStatusAll_Parallel(t *testing.T) {
	dir := t.TempDir()

	repos := []manifest.RepoInfo{
		{Name: "repo-a"},
		{Name: "repo-b"},
		{Name: "repo-c"},
	}
	for _, r := range repos {
		initTestRepo(t, dir, r.Name)
	}

	results := StatusAll(dir, repos, 5)
	assert.Len(t, results, 3)
	for i, s := range results {
		assert.NoError(t, s.Err, "repo %s", repos[i].Name)
		assert.Equal(t, "master", s.Branch)
	}
}

func TestStatusAll_MixedState(t *testing.T) {
	dir := t.TempDir()

	// One real repo, one missing
	initTestRepo(t, dir, "exists")
	repos := []manifest.RepoInfo{
		{Name: "exists"},
		{Name: "missing"},
	}

	results := StatusAll(dir, repos, 5)
	assert.NoError(t, results[0].Err)
	assert.Error(t, results[1].Err)
}
