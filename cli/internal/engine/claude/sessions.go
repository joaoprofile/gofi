package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/engine"
)

// SetModel implements engine.ModelSwitcher. An idle process is ended, so the
// next turn starts on the new model and resumes the same conversation by id.
func (s *Session) SetModel(model string) {
	s.mu.Lock()
	s.opts.Model = model
	p := s.proc
	if p != nil && s.turn == nil {
		s.proc = nil // detached now, so the next turn cannot land on it
	} else {
		p = nil // mid-turn: the new model takes over from the next restart
	}
	s.mu.Unlock()
	if p != nil {
		s.kill(p)
	}
}

// Reading the engine's own store is best-effort: its format is the engine's
// business and can change between versions, so every reader degrades to "no
// conversations" rather than to an error.
const (
	headBytes = 64 << 10  // holds the opening message
	tailBytes = 256 << 10 // holds the latest title the engine wrote
)

var sessionID = regexp.MustCompile(`^[A-Za-z0-9-]{8,64}$`)

// ListSessions returns the conversations Claude Code kept for dir, newest
// first — the ones `claude --resume` offers, including those started outside
// gofi.
func ListSessions(dir string, limit int) []engine.Saved {
	store := storeDir(dir)
	if store == "" {
		return nil
	}
	entries, err := os.ReadDir(store)
	if err != nil {
		return nil
	}
	var out []engine.Saved
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".jsonl")
		if !ok || !sessionID.MatchString(id) {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() == 0 {
			continue
		}
		title := titleOf(filepath.Join(store, e.Name()), info.Size())
		if title == "" {
			continue // no user message: nothing worth resuming
		}
		out = append(out, engine.Saved{ID: id, Title: title, Updated: info.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated.After(out[j].Updated) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// storeDir is where the engine keeps dir's conversations: the path with its
// separators flattened. Two spellings are tried rather than one guessed at.
func storeDir(dir string) string {
	home, err := os.UserHomeDir()
	if err != nil || dir == "" {
		return ""
	}
	root := filepath.Join(home, ".claude", "projects")
	for _, name := range []string{
		regexp.MustCompile(`[^A-Za-z0-9]`).ReplaceAllString(dir, "-"),
		regexp.MustCompile(`[\\/.]`).ReplaceAllString(dir, "-"),
	} {
		p := filepath.Join(root, name)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	return ""
}

type record struct {
	Type        string `json:"type"`
	AITitle     string `json:"aiTitle"`
	IsSidechain bool   `json:"isSidechain"`
	Message     *struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// titleOf names a conversation the way the engine's own resume list does: its
// latest generated title, or else the question it opened with.
func titleOf(path string, size int64) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	from := max(size-tailBytes, 0)
	tail := readAt(f, from, min(size, tailBytes))
	recs := records(tail, from > 0)
	for i := len(recs) - 1; i >= 0; i-- {
		if recs[i].Type == "ai-title" && recs[i].AITitle != "" {
			return recs[i].AITitle
		}
	}
	for _, r := range records(readAt(f, 0, min(size, headBytes)), false) {
		if r.Type != "user" || r.IsSidechain || r.Message == nil {
			continue
		}
		if text := openingText(r.Message.Content); text != "" {
			line, _, _ := strings.Cut(text, "\n")
			if r := []rune(line); len(r) > 60 {
				line = string(r[:60]) + "…"
			}
			return line
		}
	}
	return ""
}

// openingText is the first thing the user wrote, skipping the context blocks
// an editor attaches (<ide_opened_file>, <system-reminder>, …).
func openingText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	for _, b := range blocks(raw) {
		t := strings.TrimSpace(b.Text)
		if b.Type == "text" && t != "" && !strings.HasPrefix(t, "<") {
			return t
		}
	}
	return ""
}

func readAt(f *os.File, from, n int64) []byte {
	buf := make([]byte, n)
	read, err := f.ReadAt(buf, from)
	if err != nil && err != io.EOF {
		return nil
	}
	return buf[:read]
}

// records splits a slice of the store into parsed lines, dropping the first
// one when the slice starts mid-file — half a JSON object is not a record.
func records(b []byte, partialStart bool) []record {
	var out []record
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	first := true
	for sc.Scan() {
		if first && partialStart {
			first = false
			continue
		}
		first = false
		var r record
		if json.Unmarshal(sc.Bytes(), &r) == nil {
			out = append(out, r)
		}
	}
	return out
}
