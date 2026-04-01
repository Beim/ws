package version

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCurrentUsesExplicitValues(t *testing.T) {
	restore := stubBuildInfo(func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{
				Version: "v9.9.9",
			},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abcdef0123456789"},
				{Key: "vcs.time", Value: "2026-04-01T00:00:00Z"},
			},
		}, true
	})
	defer restore()

	restoreVars := stubInjectedValues("v1.2.3", "1234567890abcdef", "2026-05-01T00:00:00Z")
	defer restoreVars()

	info := Current()

	assert.Equal(t, Info{
		Version: "v1.2.3",
		Commit:  "1234567890abcdef",
		Date:    "2026-05-01T00:00:00Z",
	}, info)
	assert.Equal(t, "ws v1.2.3 (1234567, 2026-05-01T00:00:00Z)", info.String())
}

func TestCurrentUsesBuildInfoVersion(t *testing.T) {
	restore := stubBuildInfo(func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{
				Version: "v0.2.0",
			},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abcdef0123456789"},
				{Key: "vcs.time", Value: "2026-04-01T03:58:28Z"},
			},
		}, true
	})
	defer restore()

	restoreVars := stubInjectedValues("", "", "")
	defer restoreVars()

	assert.Equal(t, Info{
		Version: "v0.2.0",
		Commit:  "abcdef0123456789",
		Date:    "2026-04-01T03:58:28Z",
	}, Current())
}

func TestCurrentFallsBackToDevWithVCSMetadata(t *testing.T) {
	restore := stubBuildInfo(func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{
				Version: "(devel)",
			},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abcdef0123456789"},
				{Key: "vcs.time", Value: "2026-04-01T03:58:28Z"},
				{Key: "vcs.modified", Value: "true"},
			},
		}, true
	})
	defer restore()

	restoreVars := stubInjectedValues("", "", "")
	defer restoreVars()

	info := Current()

	assert.Equal(t, Info{
		Version: "dev",
		Commit:  "abcdef0123456789",
		Date:    "2026-04-01T03:58:28Z",
		Dirty:   true,
	}, info)
	assert.Equal(t, "ws dev (abcdef0, 2026-04-01T03:58:28Z, dirty)", info.String())
}

func TestCurrentFallsBackToDevWithoutBuildInfo(t *testing.T) {
	restore := stubBuildInfo(func() (*debug.BuildInfo, bool) {
		return nil, false
	})
	defer restore()

	restoreVars := stubInjectedValues("", "", "")
	defer restoreVars()

	assert.Equal(t, Info{Version: "dev"}, Current())
	assert.Equal(t, "ws dev", String())
}

func TestStringOmitsDuplicateCommitAndDirtyMarkers(t *testing.T) {
	info := Info{
		Version: "5c46783-dirty",
		Commit:  "5c467830e05fc214c7bbd62eaeed8ab931a2e507",
		Date:    "2026-04-01T04:25:53Z",
		Dirty:   true,
	}

	assert.Equal(t, "ws 5c46783-dirty (2026-04-01T04:25:53Z)", info.String())
}

func stubBuildInfo(fn func() (*debug.BuildInfo, bool)) func() {
	previous := readBuildInfo
	readBuildInfo = fn
	return func() {
		readBuildInfo = previous
	}
}

func stubInjectedValues(v, c, d string) func() {
	prevVersion := version
	prevCommit := commit
	prevDate := date

	version = v
	commit = c
	date = d

	return func() {
		version = prevVersion
		commit = prevCommit
		date = prevDate
	}
}
