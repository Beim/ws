package version

import (
	"fmt"
	"runtime/debug"
	"strings"
)

const fallbackVersion = "dev"

var (
	version = ""
	commit  = ""
	date    = ""

	readBuildInfo = debug.ReadBuildInfo
)

// Info describes the build version embedded in the binary.
type Info struct {
	Version string
	Commit  string
	Date    string
	Dirty   bool
}

// Current returns the best available version metadata for this binary.
func Current() Info {
	info := Info{
		Version: strings.TrimSpace(version),
		Commit:  strings.TrimSpace(commit),
		Date:    strings.TrimSpace(date),
	}

	if bi, ok := readBuildInfo(); ok {
		applyBuildInfo(&info, bi)
	}

	if info.Version == "" || info.Version == "(devel)" {
		info.Version = fallbackVersion
	}

	return info
}

// String returns a human-readable version string for CLI output.
func String() string {
	return Current().String()
}

// String formats version metadata for CLI output.
func (i Info) String() string {
	base := "ws " + i.Version

	var meta []string
	short := shortCommit(i.Commit)
	if short != "" && !strings.Contains(i.Version, short) {
		meta = append(meta, short)
	}
	if i.Date != "" {
		meta = append(meta, i.Date)
	}
	if i.Dirty && !strings.Contains(i.Version, "dirty") {
		meta = append(meta, "dirty")
	}
	if len(meta) == 0 {
		return base
	}

	return fmt.Sprintf("%s (%s)", base, strings.Join(meta, ", "))
}

func applyBuildInfo(info *Info, bi *debug.BuildInfo) {
	if info.Version == "" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		info.Version = bi.Main.Version
	}

	settings := make(map[string]string, len(bi.Settings))
	for _, setting := range bi.Settings {
		settings[setting.Key] = setting.Value
	}

	if info.Commit == "" {
		info.Commit = settings["vcs.revision"]
	}
	if info.Date == "" {
		info.Date = settings["vcs.time"]
	}
	info.Dirty = settings["vcs.modified"] == "true"
}

func shortCommit(commit string) string {
	if len(commit) <= 7 {
		return commit
	}
	return commit[:7]
}
