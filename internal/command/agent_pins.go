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

const (
	// Legacy flat path; new state lives at .ws/agent-pins.yml.
	legacyAgentPinsFile = ".ws-agent-pins"

	agentPinsStateFile = "agent-pins.yml"
)

type agentPinsState struct {
	Pins []string `yaml:"pins,omitempty"`
}

// loadAgentPins returns the set of pinned session IDs for the workspace.
func loadAgentPins(wsHome string) (map[string]bool, error) {
	data, err := os.ReadFile(stateReadPath(wsHome, agentPinsStateFile, legacyAgentPinsFile))
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
		if id != "" {
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

	path, err := stateWritePath(wsHome, agentPinsStateFile, legacyAgentPinsFile)
	if err != nil {
		return err
	}

	if len(ids) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	data, err := yaml.Marshal(agentPinsState{Pins: ids})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
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
		ppid, ok := readParentPID(pid)
		if !ok || ppid == pid {
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

func readParentPID(pid int) (int, bool) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PPid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				ppid, err := strconv.Atoi(fields[1])
				return ppid, err == nil
			}
		}
	}
	return 0, false
}
