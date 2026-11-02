package loop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// AgentsDir holds, in a run's folder, each agent's state (<id>.json) and
// transcript (<id>.ndjson), id being <node>-<iter>.
const AgentsDir = "agents"

// The states of an agent (0002, D22). An agent is a node of kind agent at
// one iteration: the pane of a run in gofi watch.
const (
	AgentWorking = "trabalhando"
	AgentDone    = "concluido"
	AgentFailed  = "falhou"
	AgentBlocked = "bloqueado" // stopped on a decision; the run waits at its gate
)

// AgentState is one agent of a run, as the runtime records it.
type AgentState struct {
	ID      string    `json:"id"`
	Node    string    `json:"node"`
	Role    string    `json:"role"`
	Iter    int       `json:"iter"`
	Status  string    `json:"status"`
	Started time.Time `json:"started"`
	Ended   time.Time `json:"ended,omitzero"`
	CostUSD float64   `json:"cost_usd"`
	Seconds float64   `json:"seconds"`
	Error   string    `json:"error,omitempty"`
}

// AgentID names the agent of a node at an iteration.
func AgentID(node string, iter int) string { return fmt.Sprintf("%s-%d", node, iter) }

// Transcript is where an agent's turn is recorded.
func Transcript(root, run, id string) string {
	return filepath.Join(Dir(root, run), AgentsDir, id+".ndjson")
}

func saveAgent(root, run string, a *AgentState) error {
	dir := filepath.Join(Dir(root, run), AgentsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return writeJSON(filepath.Join(dir, a.ID+".json"), a)
}

// Agents reads the agents of a run, in the order they started.
func Agents(root, run string) ([]AgentState, error) {
	dir := filepath.Join(Dir(root, run), AgentsDir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []AgentState
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var a AgentState
		if json.Unmarshal(data, &a) == nil && a.ID != "" {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Started.Before(out[j].Started) })
	return out, nil
}
