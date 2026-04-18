package command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const agentPinsFile = ".ws-agent-pins"

type agentPinsState struct {
	Pins []string `yaml:"pins,omitempty"`
}

func agentPinsPath(wsHome string) string {
	return filepath.Join(wsHome, agentPinsFile)
}

// loadAgentPins returns the set of pinned session IDs for the workspace.
func loadAgentPins(wsHome string) (map[string]bool, error) {
	data, err := os.ReadFile(agentPinsPath(wsHome))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	var state agentPinsState
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(state.Pins))
	for _, id := range state.Pins {
		if id = strings.TrimSpace(id); id != "" {
			set[id] = true
		}
	}
	return set, nil
}

func saveAgentPins(wsHome string, pins map[string]bool) error {
	ids := make([]string, 0, len(pins))
	for id := range pins {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	if len(ids) == 0 {
		if err := os.Remove(agentPinsPath(wsHome)); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	data, err := yaml.Marshal(agentPinsState{Pins: ids})
	if err != nil {
		return err
	}
	return os.WriteFile(agentPinsPath(wsHome), data, 0644)
}

// addAgentPin returns true if the ID was newly added (false if already pinned).
func addAgentPin(wsHome, id string) (bool, error) {
	pins, err := loadAgentPins(wsHome)
	if err != nil {
		return false, err
	}
	if pins[id] {
		return false, nil
	}
	pins[id] = true
	return true, saveAgentPins(wsHome, pins)
}

// removeAgentPin returns true if the ID was removed (false if wasn't pinned).
func removeAgentPin(wsHome, id string) (bool, error) {
	pins, err := loadAgentPins(wsHome)
	if err != nil {
		return false, err
	}
	if !pins[id] {
		return false, nil
	}
	delete(pins, id)
	return true, saveAgentPins(wsHome, pins)
}

// detectCurrentAgentSession walks up the process tree looking for a claude
// session whose recorded PID matches an ancestor. Returns the session ID and
// true if found.
func detectCurrentAgentSession() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}

	pidToSession := buildClaudePidIndex(filepath.Join(home, ".claude", "sessions"))
	if len(pidToSession) == 0 {
		return "", false
	}

	pid := os.Getppid()
	for i := 0; i < 20 && pid > 1; i++ {
		if sid, ok := pidToSession[pid]; ok {
			return sid, true
		}
		ppid, err := readParentPID(pid)
		if err != nil {
			return "", false
		}
		if ppid == pid {
			return "", false
		}
		pid = ppid
	}
	return "", false
}

func buildClaudePidIndex(sessionsDir string) map[int]string {
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil
	}
	index := make(map[int]string)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sessionsDir, entry.Name()))
		if err != nil {
			continue
		}
		var meta claudeSessionMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		if meta.PID <= 0 || meta.SessionID == "" {
			continue
		}
		index[meta.PID] = meta.SessionID
	}
	return index
}

func readParentPID(pid int) (int, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PPid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return strconv.Atoi(fields[1])
			}
		}
	}
	return 0, fmt.Errorf("no PPid entry for pid %d", pid)
}
