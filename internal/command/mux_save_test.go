package command

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dtuit/ws/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpecToMuxWindow_SinglePaneSameDir(t *testing.T) {
	wsHome := "/ws"
	repoByPath := map[string]string{"/ws/repos/api": "api"}

	spec := MuxWindowSpec{
		Name:  "editor",
		Panes: []MuxPaneSpec{{Dir: "/ws/repos/api"}},
	}

	w := specToMuxWindow(wsHome, repoByPath, spec)
	assert.Equal(t, "editor", w.Name)
	assert.Equal(t, "api", w.Dir)
	assert.Equal(t, 0, w.Panes) // single pane, no count needed
	assert.Empty(t, w.Filter)
}

func TestSpecToMuxWindow_MultiplePanesSameDir(t *testing.T) {
	wsHome := "/ws"
	repoByPath := map[string]string{"/ws/repos/api": "api"}

	spec := MuxWindowSpec{
		Name: "servers",
		Panes: []MuxPaneSpec{
			{Dir: "/ws/repos/api"},
			{Dir: "/ws/repos/api"},
			{Dir: "/ws/repos/api"},
		},
	}

	w := specToMuxWindow(wsHome, repoByPath, spec)
	assert.Equal(t, "servers", w.Name)
	assert.Equal(t, "api", w.Dir)
	assert.Equal(t, 3, w.Panes)
	assert.Empty(t, w.Filter)
}

func TestSpecToMuxWindow_MultiplePanesDifferentRepos(t *testing.T) {
	wsHome := "/ws"
	repoByPath := map[string]string{
		"/ws/repos/api": "api",
		"/ws/repos/web": "web",
	}

	spec := MuxWindowSpec{
		Name: "services",
		Panes: []MuxPaneSpec{
			{Dir: "/ws/repos/api"},
			{Dir: "/ws/repos/web"},
		},
	}

	w := specToMuxWindow(wsHome, repoByPath, spec)
	assert.Equal(t, "services", w.Name)
	assert.Equal(t, "api,web", w.Filter)
	assert.True(t, w.Split)
	assert.Empty(t, w.Dir)
}

func TestSpecToMuxWindow_WorkspaceRoot(t *testing.T) {
	wsHome := "/ws"
	repoByPath := map[string]string{}

	spec := MuxWindowSpec{
		Name:  "workspace",
		Panes: []MuxPaneSpec{{Dir: "/ws"}},
	}

	w := specToMuxWindow(wsHome, repoByPath, spec)
	assert.Equal(t, "workspace", w.Name)
	assert.Empty(t, w.Dir) // "." becomes ""
}

func TestSpecToMuxWindow_RelativeSubdir(t *testing.T) {
	wsHome := "/ws"
	repoByPath := map[string]string{}

	spec := MuxWindowSpec{
		Name:  "docs",
		Panes: []MuxPaneSpec{{Dir: "/ws/docs/guide"}},
	}

	w := specToMuxWindow(wsHome, repoByPath, spec)
	assert.Equal(t, "docs", w.Name)
	assert.Equal(t, "docs/guide", w.Dir)
}

func TestSpecToMuxWindow_MixedPanesFallback(t *testing.T) {
	wsHome := "/ws"
	repoByPath := map[string]string{
		"/ws/repos/api": "api",
	}

	spec := MuxWindowSpec{
		Name: "mixed",
		Panes: []MuxPaneSpec{
			{Dir: "/ws/repos/api"},
			{Dir: "/tmp/scratch"},
		},
	}

	w := specToMuxWindow(wsHome, repoByPath, spec)
	assert.Equal(t, "mixed", w.Name)
	assert.Equal(t, "api", w.Dir)
	assert.Equal(t, 2, w.Panes)
}

func TestUpsertManifestMuxText_NewSection(t *testing.T) {
	content := `root: repos

repos:
  api:
  web:
`
	cfg := manifest.MuxConfig{
		Backend: "tmux",
		Windows: []manifest.MuxWindow{
			{Name: "editor", Dir: "api"},
		},
	}

	result, err := upsertManifestMuxText(content, cfg)
	require.NoError(t, err)

	assert.Equal(t, `root: repos

repos:
  api:
  web:

mux:
  backend: tmux
  windows:
    - {name: editor, dir: api}
`, result)
}

func TestUpsertManifestMuxText_ReplaceExisting(t *testing.T) {
	content := `root: repos

mux:
  backend: tmux
  windows:
    - {name: old, dir: old-repo}

repos:
  api:
`
	cfg := manifest.MuxConfig{
		Backend: "tmux",
		Session: "my-ws",
		Windows: []manifest.MuxWindow{
			{Name: "new", Dir: "api"},
			{Name: "split", Filter: "api,web", Split: true},
		},
	}

	result, err := upsertManifestMuxText(content, cfg)
	require.NoError(t, err)

	assert.Equal(t, `root: repos

mux:
  backend: tmux
  session: my-ws
  windows:
    - {name: new, dir: api}
    - {name: split, filter: "api,web", split: true}

repos:
  api:
`, result)
}

func TestUpsertManifestMuxText_EmptyFile(t *testing.T) {
	cfg := manifest.MuxConfig{
		Backend: "tmux",
		Windows: []manifest.MuxWindow{
			{Name: "ws", Dir: "api"},
		},
	}

	result, err := upsertManifestMuxText("", cfg)
	require.NoError(t, err)

	assert.Equal(t, `mux:
  backend: tmux
  windows:
    - {name: ws, dir: api}
`, result)
}

func TestUpsertManifestMuxText_MultiplePanes(t *testing.T) {
	content := `root: repos
repos:
  api:
`
	cfg := manifest.MuxConfig{
		Backend: "tmux",
		Windows: []manifest.MuxWindow{
			{Name: "dev", Dir: "api", Panes: 3},
		},
	}

	result, err := upsertManifestMuxText(content, cfg)
	require.NoError(t, err)

	assert.Contains(t, result, "- {name: dev, dir: api, panes: 3}")
}

func TestUpsertManifestMuxText_PreservesWindowLayout(t *testing.T) {
	content := `root: repos
repos:
  api:
`
	cfg := manifest.MuxConfig{
		Backend: "tmux",
		Windows: []manifest.MuxWindow{
			{Name: "dev", Dir: "api", Layout: "even-horizontal"},
		},
	}

	result, err := upsertManifestMuxText(content, cfg)
	require.NoError(t, err)

	assert.Contains(t, result, `layout: even-horizontal`)
}

func TestUpsertManifestMuxText_OmitsTiledLayout(t *testing.T) {
	content := `root: repos
repos:
  api:
`
	cfg := manifest.MuxConfig{
		Backend: "tmux",
		Windows: []manifest.MuxWindow{
			{Name: "dev", Dir: "api", Layout: "tiled"},
		},
	}

	result, err := upsertManifestMuxText(content, cfg)
	require.NoError(t, err)

	assert.NotContains(t, result, "layout:")
}

func TestRenderMuxBlock_NoWindowsNoSession(t *testing.T) {
	cfg := manifest.MuxConfig{Backend: "tmux"}
	lines := renderMuxBlock(cfg)
	assert.Equal(t, []string{"mux:", "  backend: tmux"}, lines)
}

func TestUpsertManifestMux_WritesFile(t *testing.T) {
	wsHome := t.TempDir()
	manifestPath := filepath.Join(wsHome, "manifest.yml")
	require.NoError(t, os.WriteFile(manifestPath, []byte(`root: repos
repos:
  api:
`), 0644))

	cfg := manifest.MuxConfig{
		Backend: "tmux",
		Windows: []manifest.MuxWindow{
			{Name: "editor", Dir: "api"},
		},
	}

	require.NoError(t, upsertManifestMux(manifestPath, cfg, false))

	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "mux:")
	assert.Contains(t, string(data), "backend: tmux")
}

func TestUpsertManifestMux_CreatesLocalFile(t *testing.T) {
	wsHome := t.TempDir()
	localPath := filepath.Join(wsHome, "manifest.local.yml")

	cfg := manifest.MuxConfig{
		Backend: "tmux",
		Windows: []manifest.MuxWindow{
			{Name: "editor", Dir: "api"},
		},
	}

	require.NoError(t, upsertManifestMux(localPath, cfg, true))

	data, err := os.ReadFile(localPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "mux:")
}

func TestRelativeMuxDir(t *testing.T) {
	tests := []struct {
		name   string
		wsHome string
		dir    string
		want   string
	}{
		{"repo subpath", "/ws", "/ws/repos/api", "repos/api"},
		{"workspace root", "/ws", "/ws", ""},
		{"outside workspace", "/ws", "/tmp/other", "/tmp/other"},
		{"nested subdir", "/ws", "/ws/docs/guide", "docs/guide"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := relativeMuxDir(tt.wsHome, tt.dir)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRenderMuxBlock_IncludesBarsWhenTrue(t *testing.T) {
	cfg := manifest.MuxConfig{Backend: "tmux", Bars: true}
	lines := renderMuxBlock(cfg)
	assert.Contains(t, lines, "  bars: true")
}

func TestRenderMuxBlock_OmitsBarsWhenFalse(t *testing.T) {
	cfg := manifest.MuxConfig{Backend: "tmux", Bars: false}
	lines := renderMuxBlock(cfg)
	for _, line := range lines {
		assert.NotContains(t, line, "bars")
	}
}

func TestGenerateKDLLayout_WithBars(t *testing.T) {
	windows := []MuxWindowSpec{
		{Name: "editor", Panes: []MuxPaneSpec{{Dir: "/home/user/code"}}},
	}
	kdl := generateKDLLayout(windows, true)

	assert.Contains(t, kdl, "default_tab_template")
	assert.Contains(t, kdl, `plugin location="tab-bar"`)
	assert.Contains(t, kdl, `plugin location="status-bar"`)
	assert.Contains(t, kdl, `tab name="editor"`)
}

func TestGenerateKDLLayout_WithoutBars(t *testing.T) {
	windows := []MuxWindowSpec{
		{Name: "editor", Panes: []MuxPaneSpec{{Dir: "/home/user/code"}}},
	}
	kdl := generateKDLLayout(windows, false)

	assert.NotContains(t, kdl, "default_tab_template")
	assert.NotContains(t, kdl, "tab-bar")
	assert.NotContains(t, kdl, "status-bar")
	assert.Contains(t, kdl, `tab name="editor"`)
}

func TestSpecToMuxWindow_PreservesSizes(t *testing.T) {
	wsHome := "/ws"
	repoByPath := map[string]string{"/ws/repos/api": "api"}
	spec := MuxWindowSpec{
		Name:  "dev",
		Panes: []MuxPaneSpec{{Dir: "/ws/repos/api", Size: 70}, {Dir: "/ws/repos/api", Size: 30}},
	}
	w := specToMuxWindow(wsHome, repoByPath, spec)
	assert.Equal(t, []int{70, 30}, w.Sizes)
}

func TestSpecToMuxWindow_OmitsSizesWhenEqual(t *testing.T) {
	wsHome := "/ws"
	repoByPath := map[string]string{"/ws/repos/api": "api"}
	spec := MuxWindowSpec{
		Name:  "dev",
		Panes: []MuxPaneSpec{{Dir: "/ws/repos/api"}, {Dir: "/ws/repos/api"}},
	}
	w := specToMuxWindow(wsHome, repoByPath, spec)
	assert.Nil(t, w.Sizes)
}

func TestComputePaneSizePercents_UnequalPanes(t *testing.T) {
	panes := []MuxPaneSpec{{Dir: "/a"}, {Dir: "/b"}}
	computePaneSizePercents(panes, []int{60, 40})
	assert.Equal(t, 60, panes[0].Size)
	assert.Equal(t, 40, panes[1].Size)
}

func TestComputePaneSizePercents_EqualPanesSkipped(t *testing.T) {
	panes := []MuxPaneSpec{{Dir: "/a"}, {Dir: "/b"}}
	computePaneSizePercents(panes, []int{50, 50})
	assert.Equal(t, 0, panes[0].Size)
	assert.Equal(t, 0, panes[1].Size)
}

func TestComputePaneSizePercents_SumsTo100(t *testing.T) {
	panes := []MuxPaneSpec{{Dir: "/a"}, {Dir: "/b"}, {Dir: "/c"}}
	computePaneSizePercents(panes, []int{100, 50, 50})
	total := panes[0].Size + panes[1].Size + panes[2].Size
	assert.Equal(t, 100, total)
}

func TestRenderMuxWindowLines_WithSizes(t *testing.T) {
	w := manifest.MuxWindow{Name: "dev", Dir: "api", Panes: 2, Sizes: []int{70, 30}}
	lines := renderMuxWindowLines(w)
	assert.Contains(t, lines[0], "sizes: [70, 30]")
}

func TestGenerateKDLLayout_WithPaneSizes(t *testing.T) {
	windows := []MuxWindowSpec{{
		Name:   "dev",
		Panes:  []MuxPaneSpec{{Dir: "/a", Size: 70}, {Dir: "/b", Size: 30}},
		Layout: "even-horizontal",
	}}
	kdl := generateKDLLayout(windows, false)
	assert.Contains(t, kdl, `pane size="70%" cwd="/a"`)
	assert.Contains(t, kdl, `pane size="30%" cwd="/b"`)
}

func TestGenerateKDLLayout_NoPaneSizesWhenZero(t *testing.T) {
	windows := []MuxWindowSpec{{
		Name:  "dev",
		Panes: []MuxPaneSpec{{Dir: "/a"}, {Dir: "/b"}},
	}}
	kdl := generateKDLLayout(windows, false)
	assert.NotContains(t, kdl, `size=`)
}

func TestPaneCmd_Broadcast(t *testing.T) {
	cmd := []string{"cc"}
	assert.Equal(t, "cc", paneCmd(cmd, 0))
	assert.Equal(t, "cc", paneCmd(cmd, 1))
	assert.Equal(t, "cc", paneCmd(cmd, 99))
}

func TestPaneCmd_PerPane(t *testing.T) {
	cmd := []string{"cc", ""}
	assert.Equal(t, "cc", paneCmd(cmd, 0))
	assert.Equal(t, "", paneCmd(cmd, 1))
	assert.Equal(t, "", paneCmd(cmd, 2)) // out of range
}

func TestPaneCmd_Empty(t *testing.T) {
	assert.Equal(t, "", paneCmd(nil, 0))
	assert.Equal(t, "", paneCmd([]string{}, 0))
}

func TestCollectPaneCmds_AllSame(t *testing.T) {
	panes := []MuxPaneSpec{{Cmd: "cc"}, {Cmd: "cc"}}
	assert.Equal(t, []string{"cc"}, collectPaneCmds(panes))
}

func TestCollectPaneCmds_Different(t *testing.T) {
	panes := []MuxPaneSpec{{Cmd: "cc"}, {Cmd: ""}}
	assert.Equal(t, []string{"cc", ""}, collectPaneCmds(panes))
}

func TestCollectPaneCmds_NoneSet(t *testing.T) {
	panes := []MuxPaneSpec{{Cmd: ""}, {Cmd: ""}}
	assert.Nil(t, collectPaneCmds(panes))
}

func TestRenderMuxWindowLines_CmdScalar(t *testing.T) {
	w := manifest.MuxWindow{Name: "dev", Dir: "api", Cmd: []string{"cc"}}
	lines := renderMuxWindowLines(w)
	assert.Contains(t, lines[0], `cmd: "cc"`)
	assert.NotContains(t, lines[0], "[")
}

func TestRenderMuxWindowLines_CmdList(t *testing.T) {
	w := manifest.MuxWindow{Name: "dev", Dir: "api", Panes: 2, Cmd: []string{"cc", ""}}
	lines := renderMuxWindowLines(w)
	assert.Contains(t, lines[0], `cmd: ["cc", ""]`)
}

func TestSpecToMuxWindow_PreservesLayout(t *testing.T) {
	wsHome := "/ws"
	repoByPath := map[string]string{"/ws/repos/api": "api"}

	spec := MuxWindowSpec{
		Name:   "dev",
		Panes:  []MuxPaneSpec{{Dir: "/ws/repos/api"}, {Dir: "/ws/repos/api"}},
		Layout: "even-vertical",
	}

	w := specToMuxWindow(wsHome, repoByPath, spec)
	assert.Equal(t, "even-vertical", w.Layout)
}

func TestSpecToMuxWindow_PreservesLayoutForFilter(t *testing.T) {
	wsHome := "/ws"
	repoByPath := map[string]string{
		"/ws/repos/api": "api",
		"/ws/repos/web": "web",
	}

	spec := MuxWindowSpec{
		Name:   "services",
		Panes:  []MuxPaneSpec{{Dir: "/ws/repos/api"}, {Dir: "/ws/repos/web"}},
		Layout: "even-horizontal",
	}

	w := specToMuxWindow(wsHome, repoByPath, spec)
	assert.Equal(t, "even-horizontal", w.Layout)
	assert.True(t, w.Split)
}

func TestInferTmuxLayout_Horizontal(t *testing.T) {
	// { } brackets indicate horizontal arrangement (side by side)
	layout := inferTmuxLayout("de9a,115x37,0,0{56x37,0,0,0,58x37,57,0,1}", 2)
	assert.Equal(t, "even-horizontal", layout)
}

func TestInferTmuxLayout_Vertical(t *testing.T) {
	// [ ] brackets indicate vertical arrangement (stacked)
	layout := inferTmuxLayout("a1b2,80x48,0,0[80x24,0,0,0,80x23,0,25,1]", 2)
	assert.Equal(t, "even-vertical", layout)
}

func TestInferTmuxLayout_SinglePane(t *testing.T) {
	layout := inferTmuxLayout("c3d4,80x48,0,0,0", 1)
	assert.Empty(t, layout)
}

func TestInferLayoutFromPositions_SideBySide(t *testing.T) {
	panes := []zellijPanePos{
		{x: 0, y: 0},
		{x: 40, y: 0},
	}
	assert.Equal(t, "even-horizontal", inferLayoutFromPositions(panes))
}

func TestInferLayoutFromPositions_Stacked(t *testing.T) {
	panes := []zellijPanePos{
		{x: 0, y: 0},
		{x: 0, y: 25},
	}
	assert.Equal(t, "even-vertical", inferLayoutFromPositions(panes))
}

func TestInferLayoutFromPositions_Mixed(t *testing.T) {
	panes := []zellijPanePos{
		{x: 0, y: 0},
		{x: 40, y: 0},
		{x: 0, y: 25},
	}
	assert.Equal(t, "tiled", inferLayoutFromPositions(panes))
}

func TestInferLayoutFromPositions_SinglePane(t *testing.T) {
	panes := []zellijPanePos{{x: 0, y: 0}}
	assert.Empty(t, inferLayoutFromPositions(panes))
}

func TestGenerateKDLLayout_SplitDirectionFromLayout(t *testing.T) {
	tests := []struct {
		name      string
		layout    string
		wantSplit string
	}{
		{"even-horizontal maps to vertical", "even-horizontal", `split_direction="vertical"`},
		{"even-vertical maps to horizontal", "even-vertical", `split_direction="horizontal"`},
		{"tiled maps to vertical", "tiled", `split_direction="vertical"`},
		{"empty maps to vertical", "", `split_direction="vertical"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			windows := []MuxWindowSpec{
				{Name: "test", Panes: []MuxPaneSpec{{Dir: "/a"}, {Dir: "/b"}}, Layout: tt.layout},
			}
			kdl := generateKDLLayout(windows, false)
			assert.Contains(t, kdl, tt.wantSplit)
		})
	}
}

func TestGenerateKDLLayout_SinglePaneNoSplitDirection(t *testing.T) {
	windows := []MuxWindowSpec{
		{Name: "test", Panes: []MuxPaneSpec{{Dir: "/a"}}, Layout: "even-vertical"},
	}
	kdl := generateKDLLayout(windows, false)
	assert.NotContains(t, kdl, "split_direction")
}

func TestGenerateKDLLayout_CommandWrappedInShell(t *testing.T) {
	windows := []MuxWindowSpec{{
		Name:  "dev",
		Panes: []MuxPaneSpec{{Dir: "/home/user/code", Cmd: "IS_SANDBOX=1 claude"}},
	}}
	kdl := generateKDLLayout(windows, false)
	shell := userShell()
	assert.Contains(t, kdl, fmt.Sprintf("command=%q", shell))
	assert.Contains(t, kdl, `args "-ic"`)
	assert.Contains(t, kdl, "IS_SANDBOX=1 claude; exec "+shell)
	assert.NotContains(t, kdl, `command="IS_SANDBOX=1"`)
}

func TestGenerateKDLLayout_NoCmdNoBashWrap(t *testing.T) {
	windows := []MuxWindowSpec{{
		Name:  "dev",
		Panes: []MuxPaneSpec{{Dir: "/home/user/code"}},
	}}
	kdl := generateKDLLayout(windows, false)
	assert.NotContains(t, kdl, "command=")
	assert.Contains(t, kdl, `pane cwd="/home/user/code"`)
}

func TestCompleteMuxSuggestsSave(t *testing.T) {
	m, err := parseManifestYAML(`
remotes:
  origin: git@example.com:org
repos:
  repo-a:
`)
	require.NoError(t, err)

	result := Complete(m, []string{"mux", ""}, 1)
	assert.Contains(t, result.Values, "save")
}
