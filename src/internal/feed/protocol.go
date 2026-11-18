package feed

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// A Protocol is a way of reading a response: it names its scheme, checks a
// request written for it, and fetches one. http, https, file, tcp and exec
// ship with the package; Register adds another (a websocket, a serial port, a
// message queue) without touching the rest. A script never sees the protocol:
// it asks for numbers, and the host reads them however the source says.
//
//	http://, https://   an HTTP request (what a curl command becomes)
//	file:///path        a local file, read whole each time (a log, a status file)
//	tcp://host:port     connect, send Body if any, read until the peer closes
//	exec:cmd arg …      run a program, read its output (no shell; off until the user allows it)
type Protocol interface {
	Fetcher
	// Scheme is the URL scheme, in lower case: "tcp".
	Scheme() string
	// Validate checks a request that uses the scheme.
	Validate(r Request) error
}

var (
	protoMu sync.RWMutex
	protos  = map[string]Protocol{}
)

// Register adds a protocol, or replaces the one with the same scheme.
func Register(p Protocol) {
	protoMu.Lock()
	defer protoMu.Unlock()
	protos[strings.ToLower(p.Scheme())] = p
}

func protocolFor(scheme string) (Protocol, bool) {
	protoMu.RLock()
	defer protoMu.RUnlock()
	p, ok := protos[strings.ToLower(scheme)]
	return p, ok
}

// Schemes lists the schemes that can be used, sorted.
func Schemes() []string {
	protoMu.RLock()
	defer protoMu.RUnlock()
	out := make([]string, 0, len(protos))
	for s := range protos {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// schemeOf is the scheme of a request address: "http" for "http://x", "exec"
// for "exec:ls".
func schemeOf(addr string) string {
	i := strings.IndexByte(addr, ':')
	if i <= 0 {
		return ""
	}
	return strings.ToLower(addr[:i])
}

// ErrDisabled is returned for a protocol that the user has not allowed.
var ErrDisabled = errors.New("这个协议没有开启")

// Mux is the Fetcher that sends each request to the protocol of its scheme.
type Mux struct {
	// HTTP is the Fetcher for http and https; HTTPFetcher{} when nil.
	HTTP Fetcher
	// AllowExec says whether the exec protocol may run. It is asked each time,
	// so a change of the setting takes effect at once. Nil means no.
	AllowExec func() bool
}

// Fetch sends r by its protocol.
func (m Mux) Fetch(ctx context.Context, r Request) ([]byte, error) {
	scheme := schemeOf(r.URL)
	p, ok := protocolFor(scheme)
	if !ok {
		return nil, permanent(fmt.Errorf("不支持的协议 %q", scheme))
	}
	if scheme == "exec" && (m.AllowExec == nil || !m.AllowExec()) {
		return nil, permanent(fmt.Errorf("%w：exec 会运行本机程序，请先在「设置」里允许", ErrDisabled))
	}
	if scheme == "http" || scheme == "https" {
		if m.HTTP != nil {
			if err := r.Validate(); err != nil {
				return nil, permanent(err)
			}
			return m.HTTP.Fetch(ctx, r)
		}
	}
	return p.Fetch(ctx, r)
}

func init() {
	Register(httpProtocol{"http"})
	Register(httpProtocol{"https"})
	Register(fileProtocol{})
	Register(tcpProtocol{})
	Register(execProtocol{})
}

// ---- http, https ----

type httpProtocol struct{ scheme string }

func (p httpProtocol) Scheme() string { return p.scheme }

func (p httpProtocol) Validate(r Request) error {
	u, err := url.Parse(r.URL)
	if err != nil || u.Scheme != p.scheme || u.Host == "" {
		return errors.New("地址必须是完整的 http 或 https 地址")
	}
	if m := r.method(); !validMethods[m] {
		return fmt.Errorf("不支持的请求方法 %s", m)
	}
	return nil
}

func (p httpProtocol) Fetch(ctx context.Context, r Request) ([]byte, error) {
	return HTTPFetcher{}.Fetch(ctx, r)
}

// ---- file ----

type fileProtocol struct{}

func (fileProtocol) Scheme() string { return "file" }

func filePath(addr string) (string, error) {
	u, err := url.Parse(addr)
	if err != nil || u.Scheme != "file" || (u.Host != "" && u.Host != "localhost") || u.Path == "" {
		return "", errors.New("文件地址应写成 file:///绝对路径")
	}
	p := filepath.FromSlash(u.Path)
	if !filepath.IsAbs(p) {
		return "", errors.New("文件地址应写成 file:///绝对路径")
	}
	return p, nil
}

func (fileProtocol) Validate(r Request) error {
	_, err := filePath(r.URL)
	return err
}

func (fileProtocol) Fetch(ctx context.Context, r Request) ([]byte, error) {
	p, err := filePath(r.URL)
	if err != nil {
		return nil, permanent(err)
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readLimited(f)
}

func readLimited(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxBody))
	if err != nil {
		return nil, err
	}
	return b, nil
}

// ---- tcp ----

type tcpProtocol struct{}

func (tcpProtocol) Scheme() string { return "tcp" }

func tcpAddr(addr string) (string, error) {
	u, err := url.Parse(addr)
	if err != nil || u.Scheme != "tcp" || u.Hostname() == "" || u.Port() == "" {
		return "", errors.New("TCP 地址应写成 tcp://主机:端口")
	}
	return net.JoinHostPort(u.Hostname(), u.Port()), nil
}

func (tcpProtocol) Validate(r Request) error {
	_, err := tcpAddr(r.URL)
	return err
}

// tcpReadWait is how long a tcp read waits for more bytes once some have
// arrived, for a peer that answers and keeps the connection open.
const tcpReadWait = 2 * time.Second

func (tcpProtocol) Fetch(ctx context.Context, r Request) ([]byte, error) {
	addr, err := tcpAddr(r.URL)
	if err != nil {
		return nil, permanent(err)
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	if r.Body != "" {
		if _, err := io.WriteString(conn, r.Body); err != nil {
			return nil, err
		}
	}
	var buf bytes.Buffer
	chunk := make([]byte, 4096)
	for buf.Len() < maxBody {
		wait := 5 * time.Second
		if buf.Len() > 0 {
			wait = tcpReadWait
		}
		if dl, ok := ctx.Deadline(); ok && time.Until(dl) < wait {
			wait = time.Until(dl)
		}
		_ = conn.SetReadDeadline(time.Now().Add(wait))
		n, err := conn.Read(chunk)
		buf.Write(chunk[:n])
		if err != nil {
			var ne net.Error
			switch {
			case errors.Is(err, io.EOF):
				return buf.Bytes(), nil
			case errors.As(err, &ne) && ne.Timeout() && buf.Len() > 0:
				return buf.Bytes(), nil // answered, kept the line open
			}
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// ---- exec ----

type execProtocol struct{}

func (execProtocol) Scheme() string { return "exec" }

func execArgs(addr string) ([]string, error) {
	if !strings.HasPrefix(strings.ToLower(addr), "exec:") {
		return nil, errors.New("exec 地址应写成 exec:程序 参数…")
	}
	return splitShell(strings.TrimSpace(addr[len("exec:"):]))
}

func (execProtocol) Validate(r Request) error {
	args, err := execArgs(r.URL)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return errors.New("exec 地址应写成 exec:程序 参数…")
	}
	return nil
}

func (execProtocol) Fetch(ctx context.Context, r Request) ([]byte, error) {
	args, err := execArgs(r.URL)
	if err != nil || len(args) == 0 {
		return nil, permanent(errors.New("exec 地址应写成 exec:程序 参数…"))
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if r.Body != "" {
		cmd.Stdin = strings.NewReader(r.Body)
	}
	var out, errb limitedBuffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		msg := strings.TrimSpace(errb.String())
		if len(msg) > 200 {
			msg = msg[len(msg)-200:]
		}
		return nil, permanent(fmt.Errorf("命令失败：%w %s", err, msg))
	}
	return out.Bytes(), nil
}

// limitedBuffer keeps at most maxBody bytes and discards the rest.
type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := maxBody - b.Len(); room > 0 {
		b.Buffer.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}
