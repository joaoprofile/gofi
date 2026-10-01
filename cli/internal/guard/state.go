package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// State is what the guard remembers of one session between hook runs: whether
// the current question has been asked of the index, and whether the agent was
// already warned about it — once per question is a reminder, every call is
// noise that costs tokens.
type State struct {
	Asked  bool `json:"asked"`
	Warned bool `json:"warned"`
}

// Store keeps session states in a directory of the user's cache: they belong
// to a machine and a session, never to the project or its git history.
type Store struct{ Dir string }

// staleAfter is how long a session's state is kept after it was last touched.
const staleAfter = 48 * time.Hour

// DefaultStore is the store under the user cache directory.
func DefaultStore() Store {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return Store{Dir: filepath.Join(dir, "gofi", "guard")}
}

var reSession = regexp.MustCompile(`[^A-Za-z0-9._-]`)

func (s Store) path(session string) string {
	return filepath.Join(s.Dir, reSession.ReplaceAllString(session, "_")+".json")
}

// Load returns a session's state; an unknown session starts unasked.
func (s Store) Load(session string) State {
	var st State
	if b, err := os.ReadFile(s.path(session)); err == nil {
		_ = json.Unmarshal(b, &st)
	}
	return st
}

// Save records a session's state, and sweeps the states of sessions long gone.
func (s Store) Save(session string, st State) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	s.sweep()
	b, _ := json.Marshal(st)
	return os.WriteFile(s.path(session), b, 0o600)
}

func (s Store) sweep() {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > staleAfter {
			_ = os.Remove(filepath.Join(s.Dir, e.Name()))
		}
	}
}
