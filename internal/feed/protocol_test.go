package feed

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestFileProtocol(t *testing.T) {
	p := filepath.Join(t.TempDir(), "n.txt")
	os.WriteFile(p, []byte("price 12.5\n"), 0o644)
	m := Mux{}
	body, err := m.Fetch(context.Background(), Request{URL: "file://" + filepath.ToSlash(p)})
	if err != nil || string(body) != "price 12.5\n" {
		t.Fatalf("got %q, %v", body, err)
	}
	if err := (Request{URL: "file://host/etc/x"}).Validate(); err == nil {
		t.Error("file address with a remote host accepted")
	}
	if err := (Request{URL: "file:relative"}).Validate(); err == nil {
		t.Error("relative file address accepted")
	}
	src := Source{ID: "f1", Name: "file", Every: 60, Request: Request{URL: "file://" + filepath.ToSlash(p)},
		Extract: Extract{Kind: KindRegex, Pattern: `price ([0-9.]+)`}}
	r := NewRunner(m, NewStore(t.TempDir()))
	smp, err := r.Poll(context.Background(), src)
	if err != nil || len(smp.Values) != 1 || smp.Values[0] != 12.5 {
		t.Fatalf("poll: %v, %v", smp, err)
	}
}

func TestTCPProtocol(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("no loopback:", err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				buf := make([]byte, 64)
				n, _ := c.Read(buf)
				fmt.Fprintf(c, "echo:%s", buf[:n])
				time.Sleep(3 * time.Second) // keeps the line open
			}()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := time.Now()
	body, err := Mux{}.Fetch(ctx, Request{URL: "tcp://" + ln.Addr().String(), Body: "ping"})
	if err != nil || string(body) != "echo:ping" {
		t.Fatalf("got %q, %v", body, err)
	}
	if time.Since(start) > 2800*time.Millisecond {
		t.Errorf("waited %v for a peer that had answered", time.Since(start))
	}
	if err := (Request{URL: "tcp://nohost"}).Validate(); err == nil {
		t.Error("tcp address without a port accepted")
	}
}

func TestExecProtocolNeedsPermission(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	req := Request{URL: "exec:echo 7.25"}
	if err := req.Validate(); err != nil {
		t.Fatal(err)
	}
	_, err := Mux{}.Fetch(context.Background(), req)
	if !errors.Is(err, ErrDisabled) {
		t.Fatalf("exec ran without permission: %v", err)
	}
	if IsRetryable(err) {
		t.Error("a disabled protocol should not be retried")
	}
	allowed := Mux{AllowExec: func() bool { return true }}
	body, err := allowed.Fetch(context.Background(), req)
	if err != nil || string(body) != "7.25\n" {
		t.Fatalf("got %q, %v", body, err)
	}
	// No shell: the operators are plain arguments.
	body, _ = allowed.Fetch(context.Background(), Request{URL: "exec:echo 1 ; echo 2"})
	if string(body) != "1 ; echo 2\n" {
		t.Errorf("shell syntax was interpreted: %q", body)
	}
	if _, err := allowed.Fetch(context.Background(), Request{URL: "exec:false"}); err == nil {
		t.Error("a failing command succeeded")
	}
}

type memProto struct{ body string }

func (memProto) Scheme() string           { return "mem" }
func (memProto) Validate(r Request) error { return nil }
func (p memProto) Fetch(ctx context.Context, r Request) ([]byte, error) {
	return []byte(p.body), nil
}

func TestRegisterProtocol(t *testing.T) {
	Register(memProto{body: `{"v": 3}`})
	r := Request{URL: "mem:thing"}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	body, err := Mux{}.Fetch(context.Background(), r)
	if err != nil || string(body) != `{"v": 3}` {
		t.Fatalf("got %q, %v", body, err)
	}
	if _, err := (Mux{}).Fetch(context.Background(), Request{URL: "gopher://x"}); err == nil {
		t.Error("unknown scheme fetched")
	}
	found := false
	for _, s := range Schemes() {
		found = found || s == "mem"
	}
	if !found {
		t.Error("registered scheme missing from Schemes")
	}
}

func TestParseRequest(t *testing.T) {
	r, err := ParseRequest("curl 'https://a.example/x' -H 'A: b'")
	if err != nil || r.URL != "https://a.example/x" || r.Headers["A"] != "b" {
		t.Fatalf("curl: %+v %v", r, err)
	}
	r, err = ParseRequest("https://a.example/y")
	if err != nil || r.URL != "https://a.example/y" || r.Method != "" {
		t.Fatalf("url: %+v %v", r, err)
	}
	r, err = ParseRequest("tcp://10.0.0.1:9000\nSTATUS\n")
	if err != nil || r.URL != "tcp://10.0.0.1:9000" || r.Body != "STATUS\n" {
		t.Fatalf("tcp: %+v %v", r, err)
	}
	r, err = ParseRequest("exec:cat /proc/loadavg")
	if err != nil || r.URL != "exec:cat /proc/loadavg" {
		t.Fatalf("exec: %+v %v", r, err)
	}
	for _, bad := range []string{"", "ftp://x", "hello"} {
		if _, err := ParseRequest(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

var _ = atomic.Int32{}
