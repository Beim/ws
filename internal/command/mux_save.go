package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dtuit/ws/internal/manifest"
)

// MuxSave captures the current multiplexer session layout and persists it
// to the manifest. By default it writes to manifest.yml; with local=true
// it writes to manifest.local.yml instead.
func MuxSave(m *manifest.Manifest, wsHome, sessionName string, local bool) error {
	mux, err := resolveMuxBackend(m)
	if err != nil {
		return err
	}

	_, session, err := m.Mux.ResolveSession(sessionName, wsHome)
	if err != nil {
		return err
	}

	exists, err := mux.HasSession(session)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%s session %q does not exist", mux.Name(), session)
	}

	specs, err := mux.ListWindows(session)
	if err != nil {
		return err
	}
	if len(specs) == 0 {
		return fmt.Errorf("session %q has no windows", session)
	}

	windows := specsToMuxWindows(m, wsHome, specs)

	targetPath := filepath.Join(wsHome, manifestFile)
	targetLabel := manifestFile
	if local {
		targetPath = filepath.Join(wsHome, localManifestFile)
		targetLabel = localManifestFile
	}

	// Preserve existing mux config, update the relevant session.
	// Copy the Sessions map to avoid mutating the loaded manifest.
	cfg := m.Mux
	cfg.Backend = mux.Name()
	if len(cfg.Sessions) > 0 {
		sessions := make(map[string]manifest.MuxSession, len(cfg.Sessions))
		for k, v := range cfg.Sessions {
			sessions[k] = v
		}
		cfg.Sessions = sessions
		target := sessionName
		if target == "" {
			// Single session — find its key
			for k := range cfg.Sessions {
				target = k
				break
			}
		}
		s := cfg.Sessions[target]
		s.Windows = windows
		cfg.Sessions[target] = s
	} else {
		// Legacy format
		cfg.Windows = windows
	}

	if err := upsertManifestMux(targetPath, cfg, local); err != nil {
		return err
	}

	fmt.Printf("Saved mux layout to %s (%d windows)\n", targetLabel, len(windows))
	return nil
}

// specsToMuxWindows converts resolved MuxWindowSpecs back to manifest MuxWindows
// by reversing absolute paths to repo names or workspace-relative paths.
func specsToMuxWindows(m *manifest.Manifest, wsHome string, specs []MuxWindowSpec) []manifest.MuxWindow {
	// Build a reverse lookup: absolute path → repo name
	repoByPath := make(map[string]string)
	for name, cfg := range m.ActiveRepos() {
		repoByPath[m.ResolvePath(wsHome, name, cfg)] = name
	}

	var windows []manifest.MuxWindow
	for _, spec := range specs {
		w := specToMuxWindow(wsHome, repoByPath, spec)
		windows = append(windows, w)
	}
	return windows
}

func specToMuxWindow(wsHome string, repoByPath map[string]string, spec MuxWindowSpec) manifest.MuxWindow {
	if len(spec.Panes) == 0 {
		return manifest.MuxWindow{Name: spec.Name}
	}

	// Resolve each pane dir to a repo name or relative path.
	type resolvedPane struct {
		repo string // non-empty if this pane dir is a known repo
		dir  string // relative or repo-name representation
	}
	resolved := make([]resolvedPane, len(spec.Panes))
	for i, pane := range spec.Panes {
		if repo, ok := repoByPath[pane.Dir]; ok {
			resolved[i] = resolvedPane{repo: repo, dir: repo}
		} else {
			resolved[i] = resolvedPane{dir: relativeMuxDir(wsHome, pane.Dir)}
		}
	}

	// Check if all panes share the same directory.
	allSame := true
	for _, r := range resolved[1:] {
		if r.dir != resolved[0].dir {
			allSame = false
			break
		}
	}

	sizes := collectPaneSizes(spec.Panes)
	cmds := collectPaneCmds(spec.Panes)

	if allSame {
		w := manifest.MuxWindow{
			Name:   spec.Name,
			Dir:    resolved[0].dir,
			Layout: spec.Layout,
			Sizes:  sizes,
			Cmd:    cmds,
		}
		if len(spec.Panes) > 1 {
			w.Panes = len(spec.Panes)
		}
		return w
	}

	// Multiple panes with different dirs — check if all are repos.
	allRepos := true
	var repoNames []string
	for _, r := range resolved {
		if r.repo == "" {
			allRepos = false
			break
		}
		repoNames = append(repoNames, r.repo)
	}

	if allRepos {
		return manifest.MuxWindow{
			Name:   spec.Name,
			Filter: strings.Join(repoNames, ","),
			Split:  true,
			Layout: spec.Layout,
			Sizes:  sizes,
			Cmd:    cmds,
		}
	}

	// Mixed: use the first pane's dir and note the pane count.
	// This is a best-effort fallback.
	return manifest.MuxWindow{
		Name:   spec.Name,
		Dir:    resolved[0].dir,
		Panes:  len(spec.Panes),
		Layout: spec.Layout,
		Sizes:  sizes,
		Cmd:    cmds,
	}
}

// collectPaneSizes returns the size percentages from pane specs, or nil if all are zero.
func collectPaneSizes(panes []MuxPaneSpec) []int {
	hasSize := false
	for _, p := range panes {
		if p.Size > 0 {
			hasSize = true
			break
		}
	}
	if !hasSize {
		return nil
	}
	sizes := make([]int, len(panes))
	for i, p := range panes {
		sizes[i] = p.Size
	}
	return sizes
}

// collectPaneCmds returns the commands from pane specs.
// Returns nil if no pane has a command. If all panes share the same command,
// returns a single-element slice (broadcast form).
func collectPaneCmds(panes []MuxPaneSpec) []string {
	hasCmd := false
	for _, p := range panes {
		if p.Cmd != "" {
			hasCmd = true
			break
		}
	}
	if !hasCmd {
		return nil
	}
	allSame := true
	for _, p := range panes[1:] {
		if p.Cmd != panes[0].Cmd {
			allSame = false
			break
		}
	}
	if allSame {
		return []string{panes[0].Cmd}
	}
	cmds := make([]string, len(panes))
	for i, p := range panes {
		cmds[i] = p.Cmd
	}
	return cmds
}

// relativeMuxDir converts an absolute path to a workspace-relative representation.
func relativeMuxDir(wsHome, dir string) string {
	rel, err := filepath.Rel(wsHome, dir)
	if err != nil {
		return dir
	}
	// Don't use ".." paths that escape the workspace — keep absolute.
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return dir
	}
	if rel == "." {
		return ""
	}
	return rel
}

// upsertManifestMux writes the mux config into the manifest YAML file,
// replacing any existing mux: section or appending a new one.
func upsertManifestMux(path string, cfg manifest.MuxConfig, createIfMissing bool) error {
	content, err := readManifestText(path, createIfMissing)
	if err != nil {
		return err
	}

	updated, err := upsertManifestMuxText(content, cfg)
	if err != nil {
		return fmt.Errorf("updating %s: %w", filepath.Base(path), err)
	}

	return os.WriteFile(path, []byte(updated), 0644)
}

func upsertManifestMuxText(content string, cfg manifest.MuxConfig) (string, error) {
	lineEnding := detectLineEnding(content)
	lines, hasTrailingNewline := splitManifestLines(content)
	muxBlock := renderMuxBlock(cfg)

	if len(lines) == 0 {
		return joinManifestLines(muxBlock, lineEnding, true), nil
	}

	muxIndex, ok := findTopLevelKeyLine(lines, "mux")
	if !ok {
		// Append mux section
		lines = appendTopLevelSection(lines, muxBlock[0], muxBlock[1:])
		return joinManifestLines(lines, lineEnding, hasTrailingNewline), nil
	}

	// Find the end of the mux section (next top-level key or EOF)
	muxEnd := len(lines)
	for i := muxIndex + 1; i < len(lines); i++ {
		if isTopLevelContentLine(lines[i]) {
			muxEnd = i
			break
		}
	}

	// Trim trailing blank lines / comments that belong between sections
	for muxEnd > muxIndex+1 {
		trimmed := strings.TrimSpace(lines[muxEnd-1])
		if trimmed == "" || isTopLevelComment(lines[muxEnd-1]) {
			muxEnd--
			continue
		}
		break
	}

	lines = spliceLines(lines, muxIndex, muxEnd, muxBlock)
	return joinManifestLines(lines, lineEnding, hasTrailingNewline), nil
}

// renderMuxBlock produces the YAML lines for a mux config section.
func renderMuxBlock(cfg manifest.MuxConfig) []string {
	lines := []string{"mux:"}

	if cfg.Backend != "" {
		lines = append(lines, "  backend: "+cfg.Backend)
	}
	if cfg.Session != "" && len(cfg.Sessions) == 0 {
		lines = append(lines, "  session: "+cfg.Session)
	}
	if cfg.Bars {
		lines = append(lines, "  bars: true")
	}

	if len(cfg.Sessions) > 0 {
		lines = append(lines, "  sessions:")
		for _, name := range cfg.SessionNames() {
			s := cfg.Sessions[name]
			lines = append(lines, "    "+name+":")
			if s.Session != "" {
				lines = append(lines, "      session: "+s.Session)
			}
			if len(s.Windows) > 0 {
				lines = append(lines, "      windows:")
				for _, w := range s.Windows {
					for _, wl := range renderMuxWindowLines(w) {
						lines = append(lines, "    "+wl)
					}
				}
			}
		}
	} else if len(cfg.Windows) > 0 {
		lines = append(lines, "  windows:")
		for _, w := range cfg.Windows {
			lines = append(lines, renderMuxWindowLines(w)...)
		}
	}

	return lines
}

func renderMuxWindowLines(w manifest.MuxWindow) []string {
	// Build the flow-mapping on a single line for compact windows,
	// or use block style for windows with multiple fields.
	var fields []string

	fields = append(fields, fmt.Sprintf("name: %s", w.Name))

	if w.Dir != "" {
		fields = append(fields, fmt.Sprintf("dir: %s", w.Dir))
	}
	if w.Filter != "" {
		fields = append(fields, fmt.Sprintf("filter: %q", w.Filter))
	}
	if w.Split {
		fields = append(fields, "split: true")
	}
	if w.Panes > 1 {
		fields = append(fields, fmt.Sprintf("panes: %d", w.Panes))
	}
	if len(w.Cmd) == 1 && w.Cmd[0] != "" {
		fields = append(fields, fmt.Sprintf("cmd: %q", w.Cmd[0]))
	} else if len(w.Cmd) > 1 {
		quoted := make([]string, len(w.Cmd))
		for i, c := range w.Cmd {
			quoted[i] = fmt.Sprintf("%q", c)
		}
		fields = append(fields, fmt.Sprintf("cmd: [%s]", strings.Join(quoted, ", ")))
	}
	if w.Layout != "" && w.Layout != "tiled" {
		fields = append(fields, fmt.Sprintf("layout: %s", w.Layout))
	}
	if len(w.Sizes) > 0 {
		parts := make([]string, len(w.Sizes))
		for i, v := range w.Sizes {
			parts[i] = fmt.Sprintf("%d", v)
		}
		fields = append(fields, fmt.Sprintf("sizes: [%s]", strings.Join(parts, ", ")))
	}

	line := "    - {" + strings.Join(fields, ", ") + "}"
	return []string{line}
}
