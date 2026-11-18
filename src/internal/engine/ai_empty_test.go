package engine

import (
	"context"
	"testing"
	"time"
)

// A machine with no usage logs reports empty lists, which must not crash the
// bundled Claude Code script in any view.
func TestClaudeScriptWithEmptyUsage(t *testing.T) {
	s := builtin(t, "ai-claude")
	empty := map[string]any{
		"windows": map[string]any{"today": 0, "week": 0, "month": 0, "block": 0},
		"models":  nil,
		"daily":   nil,
	}
	for _, view := range []float64{0, 1, 2} {
		for _, win := range []float64{0, 1, 2} {
			_, err := s.Run(context.Background(), Input{
				Now:   time.Now(),
				View:  map[string]float64{"view": view, "window": win},
				Usage: map[string]any{"claude-code": empty},
			})
			if err != nil {
				t.Errorf("view %v window %v: %v", view, win, err)
			}
		}
	}
}
