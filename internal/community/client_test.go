package community

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// TestCommunityFlow runs against a community Worker, for example one started
// with `wrangler dev --local`. It needs SB_COMMUNITY_URL and SB_ADMIN_KEY.
func TestCommunityFlow(t *testing.T) {
	base, key := os.Getenv("SB_COMMUNITY_URL"), os.Getenv("SB_ADMIN_KEY")
	if base == "" || key == "" {
		t.Skip("set SB_COMMUNITY_URL and SB_ADMIN_KEY to run against a community Worker")
	}
	ctx := context.Background()
	c := New(base)

	user := fmt.Sprintf("go_%d", time.Now().UnixNano()%1_000_000_000)
	token, err := c.Register(ctx, user, "correct-horse")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := c.Login(ctx, user, "wrong-password"); err == nil {
		t.Error("login with a wrong password succeeded")
	}
	if _, _, err := c.Upload(ctx, "", "anon", "", "", src()); err == nil {
		t.Error("upload without a login succeeded")
	}

	id, msg, err := c.Upload(ctx, token, "Go 测试脚本", "coins", "from the Go test", src())
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if id == "" || msg == "" {
		t.Fatalf("upload gave id %q, message %q", id, msg)
	}
	if _, err := c.Download(ctx, id); err == nil {
		t.Error("a pending script downloaded")
	}

	adminPost(t, base, key, "/api/admin/scripts/"+id+"/review", `{"action":"approve"}`)

	list, err := c.List(ctx, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, s := range list {
		found = found || s.ID == id
	}
	if !found {
		t.Errorf("approved script %s not in the list", id)
	}
	raw, err := c.Download(ctx, id)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	var doc struct {
		Format string `json:"format"`
		Name   string `json:"name"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil || doc.Format != "scriptboard/1" || doc.Name != "Go 测试脚本" {
		t.Errorf("download %q: %v", raw, err)
	}
	if err := c.Logout(ctx, token); err != nil {
		t.Errorf("logout: %v", err)
	}
}

func src() string {
	return `export function meta() { return { name: "Go 测试脚本", icon: "coins", sources: {} }; }
export function render(ctx) { return { title: "t", big: "1", sub: "s", lines: [], chart: null }; }`
}

func adminPost(t *testing.T, base, key, path, body string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, strings.TrimRight(base, "/")+path, bytes.NewBufferString(body))
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-admin-key", key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin %s: %s %s", path, resp.Status, b)
	}
}
