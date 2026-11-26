package cmdrun

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"
)

var (
	// ErrExecDisabled reports that host command execution has not been allowed in Settings.
	ErrExecDisabled = errors.New("本机命令执行未开启，请先在「设置 → 数据源」或「命令任务」中开启允许本机命令执行")
	// ErrDesktopOnly reports that host commands cannot run on mobile platforms.
	ErrDesktopOnly = errors.New("本机命令执行仅支持桌面端")
)

var (
	loginPathOnce sync.Once
	cachedPath    string
)

func loginPath() string {
	loginPathOnce.Do(func() {
		var parts []string
		if cur := os.Getenv("PATH"); cur != "" {
			parts = append(parts, strings.Split(cur, string(os.PathListSeparator))...)
		}
		if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
			if sh := strings.TrimSpace(os.Getenv("SHELL")); sh != "" {
				ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
				out, err := exec.CommandContext(ctx, sh, "-lc", `printf %s "$PATH"`).Output()
				cancel()
				if err == nil {
					if s := strings.TrimSpace(string(out)); s != "" {
						parts = append(parts, strings.Split(s, ":")...)
					}
				}
			}
			parts = append(parts, "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin")
		}
		seen := map[string]bool{}
		var dedup []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			dedup = append(dedup, p)
		}
		cachedPath = strings.Join(dedup, string(os.PathListSeparator))
	})
	return cachedPath
}

type limitedBuffer struct {
	buf bytes.Buffer
	max int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := b.max - b.buf.Len(); room > 0 {
		b.buf.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

func (b *limitedBuffer) Bytes() []byte { return b.buf.Bytes() }

func splitArgs(s string) ([]string, error) {
	var args []string
	var cur strings.Builder
	var quote rune
	inTok := false
	escaped := false

	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
			inTok = true
		case r == '\\' && quote != '\'':
			escaped = true
			inTok = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inTok = true
		case unicode.IsSpace(r):
			if inTok {
				args = append(args, cur.String())
				cur.Reset()
				inTok = false
			}
		default:
			cur.WriteRune(r)
			inTok = true
		}
	}
	if escaped || quote != 0 {
		return nil, errors.New("引号或转义符未闭合")
	}
	if inTok {
		args = append(args, cur.String())
	}
	return args, nil
}

// Execute runs spec once with params and extracts configured fields from stdout.
func Execute(ctx context.Context, spec Spec, params map[string]string, trigger string, now time.Time) Record {
	rec := Record{
		ID:        newRecordID(now),
		CommandID: spec.ID,
		T:         now.UnixMilli(),
		Trigger:   trigger,
		ExitCode:  -1,
	}
	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		rec.Error = ErrDesktopOnly.Error()
		return rec
	}
	if err := spec.Validate(); err != nil {
		rec.Error = err.Error()
		return rec
	}
	resolved, masked, ordered, err := ResolveParams(spec.Params, params)
	if err != nil {
		rec.Error = err.Error()
		return rec
	}
	rec.Params = masked
	if IsTrustedBuiltin(spec) {
		if err := ValidateBuiltinPingParams(resolved); err != nil {
			rec.Error = err.Error()
			return rec
		}
	}

	runCtx, cancel := context.WithTimeout(ctx, spec.TimeoutDuration())
	defer cancel()

	cmd, err := buildCmd(runCtx, spec, ordered)
	if err != nil {
		rec.Error = err.Error()
		return rec
	}
	if strings.TrimSpace(spec.WorkDir) != "" {
		cmd.Dir = spec.WorkDir
	}
	cmd.Env = buildEnv(spec.Env, resolved)

	stdout := limitedBuffer{max: MaxStdoutBytes}
	stderr := limitedBuffer{max: MaxStderrBytes}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	rec.DurationMs = time.Since(start).Milliseconds()

	outBytes := stdout.Bytes()
	errBytes := stderr.Bytes()
	rec.Stdout = strings.TrimSpace(string(outBytes))
	rec.Stderr = strings.TrimSpace(string(errBytes))

	parsedJSON, extracted, extErr := ExtractAll(outBytes, spec.Extract)
	rec.JSON = parsedJSON
	if len(extracted) > 0 {
		rec.Extracted = extracted
	}

	if runErr == nil {
		rec.OK = true
		rec.ExitCode = 0
		if extErr != nil {
			rec.Error = extErr.Error()
		}
		return rec
	}

	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		rec.Error = fmt.Sprintf("执行超时（>%ds）", int(spec.TimeoutDuration().Seconds()))
		return rec
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		rec.ExitCode = exitErr.ExitCode()
	}
	if rec.Stderr != "" {
		msg := rec.Stderr
		if len(msg) > 240 {
			msg = msg[len(msg)-240:]
		}
		rec.Error = msg
	} else {
		rec.Error = runErr.Error()
	}
	return rec
}

func buildCmd(ctx context.Context, spec Spec, ordered []string) (*exec.Cmd, error) {
	sh := strings.TrimSpace(spec.Shell)
	switch strings.ToLower(sh) {
	case "", "sh", "bash", "zsh":
		if sh == "" {
			sh = "sh"
		}
		argv := append([]string{"-c", spec.Command, "--"}, ordered...)
		return exec.CommandContext(ctx, sh, argv...), nil
	case "pwsh":
		return exec.CommandContext(ctx, "pwsh", "-NoProfile", "-NonInteractive", "-Command", spec.Command), nil
	case "exec":
		args, err := splitArgs(spec.Command)
		if err != nil {
			return nil, err
		}
		if len(args) == 0 {
			return nil, errors.New("exec 命令不能为空")
		}
		return exec.CommandContext(ctx, args[0], args[1:]...), nil
	default:
		argv := append([]string{"-c", spec.Command, "--"}, ordered...)
		return exec.CommandContext(ctx, sh, argv...), nil
	}
}

func buildEnv(staticEnv map[string]string, params map[string]string) []string {
	env := os.Environ()
	merged := make(map[string]string, len(env)+len(staticEnv)+2*len(params)+2)
	var order []string
	setKV := func(k, v string) {
		if _, exists := merged[k]; !exists {
			order = append(order, k)
		}
		merged[k] = v
	}
	for _, entry := range env {
		if i := strings.IndexByte(entry, '='); i > 0 {
			setKV(entry[:i], entry[i+1:])
		}
	}
	if lp := loginPath(); lp != "" {
		setKV("PATH", lp)
	}
	for k, v := range staticEnv {
		if strings.TrimSpace(k) != "" {
			setKV(k, v)
		}
	}
	for k, v := range params {
		sbKey := ParamEnvKey(k)
		setKV(sbKey, v)
		setKV(strings.TrimPrefix(sbKey, "SB_"), v)
	}
	out := make([]string, 0, len(order))
	for _, k := range order {
		out = append(out, k+"="+merged[k])
	}
	return out
}

func newRecordID(now time.Time) string {
	var raw [4]byte
	_, _ = rand.Read(raw[:])
	return fmt.Sprintf("%d-%s", now.UnixMilli(), hex.EncodeToString(raw[:]))
}
