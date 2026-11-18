package usage

import (
	"bufio"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// claudeCode reads the logs Claude Code writes under ~/.claude/projects: one
// JSON line per event, and each reply carries its token counts. This is what
// ccusage reads too. The same reply can appear on several lines as it
// streams, so each is counted once, by message id and request id.
type claudeCode struct{}

func (claudeCode) ID() string     { return "claude-code" }
func (claudeCode) Name() string   { return "Claude Code（本机日志）" }
func (claudeCode) NeedsKey() bool { return false }

const blockLength = 5 * time.Hour

type claudeLine struct {
	Timestamp string `json:"timestamp"`
	RequestID string `json:"requestId"`
	Message   struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			Input         float64 `json:"input_tokens"`
			Output        float64 `json:"output_tokens"`
			CacheCreation float64 `json:"cache_creation_input_tokens"`
			CacheRead     float64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

type claudeEntry struct {
	at    time.Time
	model string
	total float64
}

func (claudeCode) Fetch(ctx context.Context, o Options) (Report, error) {
	dir := o.Dir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Report{}, err
		}
		dir = filepath.Join(home, ".claude", "projects")
	}
	since := o.Now.AddDate(0, 0, -35) // older logs are not read
	var entries []claudeEntry
	seen := map[string]bool{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		if info, err := d.Info(); err == nil && info.ModTime().Before(since) {
			return nil
		}
		entries = readClaudeFile(path, since, seen, entries)
		return nil
	})
	if err != nil {
		return Report{}, err
	}
	return buildClaudeReport(entries, o), nil
}

func readClaudeFile(path string, since time.Time, seen map[string]bool, out []claudeEntry) []claudeEntry {
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 256*1024), 16<<20)
	for sc.Scan() {
		var l claudeLine
		if json.Unmarshal(sc.Bytes(), &l) != nil || l.Message.Usage == nil {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, l.Timestamp)
		if err != nil || at.Before(since) {
			continue
		}
		if l.Message.ID != "" && l.RequestID != "" {
			key := l.Message.ID + ":" + l.RequestID
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		u := l.Message.Usage
		out = append(out, claudeEntry{at: at, model: l.Message.Model, total: u.Input + u.Output + u.CacheCreation + u.CacheRead})
	}
	return out
}

// buildClaudeReport sums the entries into the windows, models and days.
func buildClaudeReport(entries []claudeEntry, o Options) Report {
	now := o.Now
	loc := now.Location()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	weekStart := dayStart.AddDate(0, 0, -6)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)

	r := Report{Unit: "tokens", Windows: map[string]float64{}}
	models := map[string]float64{}
	days := map[string]float64{}
	for _, e := range entries {
		at := e.at.In(loc)
		if !at.Before(dayStart) {
			r.Windows["today"] += e.total
		}
		if !at.Before(weekStart) {
			r.Windows["week"] += e.total
		}
		if !at.Before(monthStart) {
			r.Windows["month"] += e.total
			models[e.model] += e.total
		}
		if !at.Before(dayStart.AddDate(0, 0, -13)) {
			days[at.Format("01-02")] += e.total
		}
	}
	r.Windows["block"] = activeBlock(entries, now)

	for name, v := range models {
		r.Models = append(r.Models, Item{Name: name, Value: v})
	}
	sort.Slice(r.Models, func(i, j int) bool { return r.Models[i].Value > r.Models[j].Value })
	for i := 13; i >= 0; i-- {
		d := dayStart.AddDate(0, 0, -i).Format("01-02")
		r.Daily = append(r.Daily, Item{Name: d, Value: days[d]})
	}
	r.Used, r.Limit, r.BlockLimit = r.Windows["month"], o.Budget, o.BlockBudget
	if len(entries) == 0 {
		r.Note = "没有找到 Claude Code 的使用记录"
	}
	return r
}

// activeBlock is the tokens of the current five-hour block, as ccusage counts
// them: a block starts at the hour of the first message after the last one
// ended and lasts five hours. It is 0 when the last block is over.
func activeBlock(entries []claudeEntry, now time.Time) float64 {
	if len(entries) == 0 {
		return 0
	}
	sorted := append([]claudeEntry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].at.Before(sorted[j].at) })
	var start time.Time
	var total float64
	for _, e := range sorted {
		if start.IsZero() || !e.at.Before(start.Add(blockLength)) {
			start = e.at.Truncate(time.Hour)
			total = 0
		}
		total += e.total
	}
	if !now.Before(start.Add(blockLength)) {
		return 0
	}
	return total
}
