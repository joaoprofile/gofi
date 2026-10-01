package guard

import (
	"encoding/json"

	"github.com/joaoprofile/gofi/cli/internal/config"
)

// Decide applies one hook event to the session's state and returns what the
// hook prints: nil for no opinion, which leaves Claude Code's permission flow
// exactly as it was.
func Decide(in Input, root, mode string, store Store, msg func(search string, deny bool) string) []byte {
	if mode == config.GuardOff || in.SessionID == "" {
		return nil
	}
	kind, search := Classify(in, root)
	switch kind {
	case NewPrompt:
		_ = store.Save(in.SessionID, State{})
	case Query:
		st := store.Load(in.SessionID)
		if !st.Asked {
			st.Asked = true
			_ = store.Save(in.SessionID, st)
		}
	case Search:
		st := store.Load(in.SessionID)
		if st.Asked {
			return nil
		}
		if mode == config.GuardEnforce {
			return output(map[string]any{
				"hookEventName":            EventPreTool,
				"permissionDecision":       "deny",
				"permissionDecisionReason": msg(search, true),
			})
		}
		if st.Warned {
			return nil
		}
		st.Warned = true
		_ = store.Save(in.SessionID, st)
		// No permission decision: the call goes through the usual flow, and
		// the agent reads the reminder next to its result.
		return output(map[string]any{
			"hookEventName":     EventPreTool,
			"additionalContext": msg(search, false),
		})
	}
	return nil
}

func output(specific map[string]any) []byte {
	b, _ := json.Marshal(map[string]any{"hookSpecificOutput": specific})
	return b
}
