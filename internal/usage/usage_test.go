package usage

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func line(at time.Time, id, req, model string, in, out, cc, cr int) string {
	return fmt.Sprintf(`{"timestamp":%q,"requestId":%q,"message":{"id":%q,"model":%q,"usage":{"input_tokens":%d,"output_tokens":%d,"cache_creation_input_tokens":%d,"cache_read_input_tokens":%d}}}`,
		at.UTC().Format(time.RFC3339Nano), req, id, model, in, out, cc, cr)
}

// Logs of two projects: replies counted once however many lines they stream
// on, by window, by model, by day, and the five-hour block.
func TestClaudeCodeFromLogs(t *testing.T) {
	now := time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	a := []string{
		line(now.Add(-1*time.Hour), "m1", "r1", "claude-a", 10, 90, 0, 0),          // 100 today, in the block
		line(now.Add(-1*time.Hour), "m1", "r1", "claude-a", 10, 90, 0, 0),          // the same reply again
		line(now.Add(-30*time.Minute), "m2", "r2", "claude-b", 100, 100, 300, 500), // 1000
		"not json at all",
		`{"timestamp":"2026-11-18T10:00:00Z","type":"user"}`, // no usage
	}
	b := []string{
		line(now.AddDate(0, 0, -3), "m3", "r3", "claude-a", 0, 2000, 0, 0),  // this week, this month
		line(now.AddDate(0, 0, -40), "m4", "r4", "claude-a", 0, 9999, 0, 0), // too old
	}
	write := func(name string, lines []string) {
		p := filepath.Join(dir, name, "s.jsonl")
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("proj-a", a)
	write("proj-b", b)

	r, err := Fetch(context.Background(), "claude-code", Options{Dir: dir, Now: now, Budget: 10000})
	if err != nil {
		t.Fatal(err)
	}
	if r.Windows["today"] != 1100 {
		t.Errorf("today %v, want 1100 (the repeated reply counted once)", r.Windows["today"])
	}
	if r.Windows["week"] != 3100 || r.Windows["month"] != 3100 {
		t.Errorf("week %v month %v, want 3100", r.Windows["week"], r.Windows["month"])
	}
	if r.Windows["block"] != 1100 {
		t.Errorf("block %v, want 1100", r.Windows["block"])
	}
	if r.Used != 3100 || r.Limit != 10000 {
		t.Errorf("used %v limit %v", r.Used, r.Limit)
	}
	if len(r.Models) != 2 || r.Models[0].Name != "claude-a" || r.Models[0].Value != 2100 {
		t.Errorf("models %+v", r.Models)
	}
	if len(r.Daily) != 14 || r.Daily[13].Name != "11-18" || r.Daily[13].Value != 1100 {
		t.Errorf("daily %+v", r.Daily)
	}
	if r.Provider != "claude-code" || r.Unit != "tokens" {
		t.Errorf("report %+v", r)
	}
}

func TestBlockEndsAfterFiveHours(t *testing.T) {
	now := time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC)
	es := []claudeEntry{{at: now.Add(-7 * time.Hour), total: 50}}
	if got := activeBlock(es, now); got != 0 {
		t.Errorf("block %v after it ended, want 0", got)
	}
}

func TestBalanceProviders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/user/balance":
			fmt.Fprint(w, `{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":"110.50"}]}`)
		case "/api/v1/credits":
			fmt.Fprint(w, `{"data":{"total_credits":20,"total_usage":7.5}}`)
		case "/v1/users/me/balance":
			fmt.Fprint(w, `{"data":{"available_balance":49.9}}`)
		case "/v1/user/info":
			fmt.Fprint(w, `{"data":{"totalBalance":"8.80"}}`)
		}
	}))
	defer srv.Close()
	o := Options{Key: "good", BaseURL: srv.URL, Budget: 200, HTTP: srv.Client()}

	r, err := Fetch(context.Background(), "deepseek-balance", o)
	if err != nil || r.Balance != 110.5 || r.Unit != "CNY" || r.Limit != 200 || r.Used != 89.5 {
		t.Errorf("deepseek %+v %v", r, err)
	}
	r, err = Fetch(context.Background(), "openrouter-credits", o)
	if err != nil || r.Balance != 12.5 || r.Limit != 20 || r.Used != 7.5 {
		t.Errorf("openrouter %+v %v", r, err)
	}
	if r, err = Fetch(context.Background(), "moonshot-balance", o); err != nil || r.Balance != 49.9 {
		t.Errorf("moonshot %+v %v", r, err)
	}
	if r, err = Fetch(context.Background(), "siliconflow-balance", o); err != nil || r.Balance != 8.8 {
		t.Errorf("siliconflow %+v %v", r, err)
	}

	bad := o
	bad.Key = "bad"
	if _, err := Fetch(context.Background(), "deepseek-balance", bad); err == nil || !strings.Contains(err.Error(), "密钥") {
		t.Errorf("a wrong key should say so: %v", err)
	}
	if _, err := Fetch(context.Background(), "deepseek-balance", Options{}); err == nil {
		t.Error("a missing key should be refused before any request")
	}
	if _, err := Fetch(context.Background(), "nope", o); err == nil {
		t.Error("an unknown provider should be refused")
	}
}

type fake struct{}

func (fake) ID() string     { return "fake" }
func (fake) Name() string   { return "Fake" }
func (fake) NeedsKey() bool { return false }
func (fake) Fetch(ctx context.Context, o Options) (Report, error) {
	return Report{Unit: "x", Used: 1, Limit: 4}, nil
}

// A provider registered by another package is found like the built-in ones.
func TestRegisterExtendsProviders(t *testing.T) {
	Register(fake{})
	r, err := Fetch(context.Background(), "fake", Options{})
	if err != nil || r.Provider != "fake" || r.Limit != 4 {
		t.Fatalf("%+v %v", r, err)
	}
	found := false
	for _, p := range List() {
		found = found || p.ID() == "fake"
	}
	if !found {
		t.Error("registered provider not listed")
	}
}
