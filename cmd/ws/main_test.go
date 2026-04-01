package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCodeArgs_Default(t *testing.T) {
	filter, worktrees, err := parseCodeArgs([]string{"backend"}, "")
	require.NoError(t, err)
	assert.Equal(t, "backend", filter)
	assert.False(t, worktrees.Set)
}

func TestParseCodeArgs_WorktreesFlags(t *testing.T) {
	tests := []struct {
		args     []string
		filter   string
		override bool
	}{
		{args: []string{"--worktrees", "backend"}, filter: "backend", override: true},
		{args: []string{"-W", "backend"}, filter: "backend", override: true},
		{args: []string{"-t", "backend"}, filter: "backend", override: true},
		{args: []string{"backend", "--worktrees"}, filter: "backend", override: true},
		{args: []string{"backend", "-t"}, filter: "backend", override: true},
		{args: []string{"backend", "-W"}, filter: "backend", override: true},
		{args: []string{"--no-worktrees", "backend"}, filter: "backend", override: false},
		{args: []string{"-t"}, filter: "ctx", override: true},
	}

	for _, tt := range tests {
		filter, worktrees, err := parseCodeArgs(tt.args, "ctx")
		require.NoError(t, err)
		assert.Equal(t, tt.filter, filter)
		assert.True(t, worktrees.Set)
		assert.Equal(t, tt.override, worktrees.Value)
	}
}

func TestParseCodeArgs_RejectsUnknownFlag(t *testing.T) {
	_, _, err := parseCodeArgs([]string{"--bogus"}, "")
	require.Error(t, err)
}

func TestParseCodeArgs_RejectsMultipleFilters(t *testing.T) {
	_, _, err := parseCodeArgs([]string{"backend", "frontend"}, "")
	require.Error(t, err)
}

func TestParseContextArgs_Show(t *testing.T) {
	action, filter, worktrees, err := parseContextArgs(nil)
	require.NoError(t, err)
	assert.Equal(t, "show", action)
	assert.Equal(t, "", filter)
	assert.False(t, worktrees.Set)
}

func TestParseContextArgs_Set(t *testing.T) {
	action, filter, worktrees, err := parseContextArgs([]string{"backend"})
	require.NoError(t, err)
	assert.Equal(t, "set", action)
	assert.Equal(t, "backend", filter)
	assert.False(t, worktrees.Set)
}

func TestParseContextArgs_Add(t *testing.T) {
	action, filter, worktrees, err := parseContextArgs([]string{"add", "backend", "repo-a"})
	require.NoError(t, err)
	assert.Equal(t, "add", action)
	assert.Equal(t, "backend,repo-a", filter)
	assert.False(t, worktrees.Set)
}

func TestParseContextArgs_SetWithWorktreesFlag(t *testing.T) {
	action, filter, worktrees, err := parseContextArgs([]string{"-t", "backend"})
	require.NoError(t, err)
	assert.Equal(t, "set", action)
	assert.Equal(t, "backend", filter)
	assert.True(t, worktrees.Set)
	assert.True(t, worktrees.Value)
}

func TestParseContextArgs_AddWithWorktreesFlag(t *testing.T) {
	action, filter, worktrees, err := parseContextArgs([]string{"add", "-t", "backend", "repo-a"})
	require.NoError(t, err)
	assert.Equal(t, "add", action)
	assert.Equal(t, "backend,repo-a", filter)
	assert.True(t, worktrees.Set)
	assert.True(t, worktrees.Value)
}

func TestParseContextArgs_SetWithNoWorktreesFlag(t *testing.T) {
	action, filter, worktrees, err := parseContextArgs([]string{"--no-worktrees", "backend"})
	require.NoError(t, err)
	assert.Equal(t, "set", action)
	assert.Equal(t, "backend", filter)
	assert.True(t, worktrees.Set)
	assert.False(t, worktrees.Value)
}

func TestParseContextArgs_AddRequiresFilter(t *testing.T) {
	_, _, _, err := parseContextArgs([]string{"add"})
	require.Error(t, err)
}

func TestParseContextArgs_RejectsUnknownFlag(t *testing.T) {
	_, _, _, err := parseContextArgs([]string{"--bogus"})
	require.Error(t, err)
}
