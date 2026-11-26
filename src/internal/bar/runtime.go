package bar

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/MoeForge/moejs"
	"github.com/MoeForge/moejs/engine"
	"github.com/MoeForge/moejs/eventloop"
	"github.com/MoeForge/moejs/webapi"
)

// Options configures a running .bar Instance.
type Options struct {
	Client     *http.Client
	Inputs     map[string]string
	Lang       string
	Theme      string
	Vibrancy   bool
	Offline    bool
	DataDir    string
	Cookies    *CookieJar
	Storage    *KVStore
	OnUpdate   func(Snapshot)
	OnOpenURL  func(string)
	OnCopyText func(string)
}

// Instance runs a single parsed .bar component in its own moejs runtime and
// event loop. Its public methods are safe to call from any goroutine.
type Instance struct {
	spec       Spec
	mod        *moejs.Module
	rt         *moejs.Runtime
	loop       *eventloop.Loop
	web        *webapi.API
	cookies    *CookieJar
	storage    *KVStore
	remove     func()
	onUpdate   func(Snapshot)
	onOpenURL  func(string)
	onCopyText func(string)

	mu           sync.RWMutex
	inputs       map[string]string
	lang         string
	theme        string
	vibrancy     bool
	offline      bool
	snap         Snapshot
	asyncErr     string
	logs         []string
	closed       bool
	rendering    bool
	schedRunning bool
	schedWaiters []chan struct{}

	// Loop-goroutine-only references kept rooted via AddLiveRoots.
	defObj  moejs.Value
	actions map[string]*moejs.Object
}

func normalizeBarTheme(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "light":
		return "light"
	case "dark":
		return "dark"
	default:
		return "dark"
	}
}

// NewInstance compiles spec.Script, boots its event loop + webapi + persistent
// CookieJar/KVStore, performs an initial render, and kicks off scheduled(env).
func NewInstance(spec Spec, opts Options) (*Instance, error) {
	if strings.TrimSpace(spec.Script) == "" {
		return nil, errors.New("bar: empty script")
	}
	mod, err := moejs.Compile(spec.ID+".js", spec.Script)
	if err != nil {
		return nil, fmt.Errorf("bar compile %s: %w", spec.ID, err)
	}
	cookies := opts.Cookies
	if cookies == nil {
		var cookiePath string
		if opts.DataDir != "" {
			cookiePath = filepath.Join(opts.DataDir, "cookies.json")
		}
		if cookies, err = NewCookieJar(cookiePath); err != nil {
			return nil, fmt.Errorf("bar cookies %s: %w", spec.ID, err)
		}
	}
	storage := opts.Storage
	if storage == nil {
		var storagePath string
		if opts.DataDir != "" {
			storagePath = filepath.Join(opts.DataDir, "storage.json")
		}
		if storage, err = NewKVStore(storagePath); err != nil {
			return nil, fmt.Errorf("bar storage %s: %w", spec.ID, err)
		}
	}

	rt := moejs.NewRuntime(moejs.Options{MemoryLimit: 32 << 20})

	baseClient := opts.Client
	if baseClient == nil {
		baseClient = &http.Client{Timeout: 15 * time.Second}
	}
	clientCopy := *baseClient
	clientCopy.Jar = cookies

	inst := &Instance{
		spec:       spec,
		mod:        mod,
		rt:         rt,
		cookies:    cookies,
		storage:    storage,
		onUpdate:   opts.OnUpdate,
		onOpenURL:  opts.OnOpenURL,
		onCopyText: opts.OnCopyText,
		inputs:     MergeInputs(spec, opts.Inputs),
		lang:       opts.Lang,
		theme:      normalizeBarTheme(opts.Theme),
		vibrancy:   opts.Vibrancy,
		offline:    opts.Offline,
		actions:    map[string]*moejs.Object{},
	}

	loop, err := eventloop.New(rt, eventloop.Options{
		OnError: func(err error) {
			if err == nil {
				return
			}
			inst.recordError(err.Error())
		},
	})
	if err != nil {
		return nil, fmt.Errorf("bar eventloop %s: %w", spec.ID, err)
	}
	inst.loop = loop

	// Keep the default export object and active UI callback functions rooted.
	inst.remove = rt.AddLiveRoots(func(add func(moejs.Value)) {
		inst.mu.RLock()
		defer inst.mu.RUnlock()
		if !inst.defObj.IsUndefined() && !inst.defObj.IsNull() {
			add(inst.defObj)
		}
		for _, fn := range inst.actions {
			if fn != nil {
				add(engine.ObjectValue(fn))
			}
		}
	})

	api, err := webapi.Install(loop, webapi.Options{
		Client: &clientCopy,
		Allow: func(req *http.Request) error {
			if inst.isOffline() {
				return errors.New("offline mode is enabled")
			}
			return OriginAllowed(req.URL.String(), spec.Permissions)
		},
	})
	if err != nil {
		if inst.remove != nil {
			inst.remove()
		}
		return nil, fmt.Errorf("bar webapi %s: %w", spec.ID, err)
	}
	inst.web = api

	if err := inst.installGlobals(); err != nil {
		inst.Close()
		return nil, err
	}

	if err := loop.Start(); err != nil {
		inst.Close()
		return nil, fmt.Errorf("bar start %s: %w", spec.ID, err)
	}

	ready := make(chan error, 1)
	ok := loop.RunOnLoop(func(r *moejs.Runtime) {
		if err := r.Load(mod); err != nil {
			ready <- fmt.Errorf("bar load %s: %w", spec.ID, err)
			return
		}
		defVal, hasDef := r.Export("default")
		if !hasDef || defVal.IsUndefined() || defVal.IsNull() {
			ready <- fmt.Errorf("bar %s: default export missing", spec.ID)
			return
		}
		inst.mu.Lock()
		inst.defObj = defVal
		inst.mu.Unlock()

		inst.renderNow(r)
		ready <- nil
		inst.runScheduled(r)
	})
	if !ok {
		inst.Close()
		return nil, errors.New("bar loop terminated before startup")
	}
	if err := <-ready; err != nil {
		inst.Close()
		return nil, err
	}
	return inst, nil
}

func (inst *Instance) installGlobals() error {
	return installBarGlobals(
		inst.rt,
		inst.spec,
		inst.cookies,
		inst.storage,
		func(raw string) {
			if inst.onCopyText != nil {
				inst.onCopyText(raw)
			}
		},
		func(raw string) {
			if inst.onOpenURL != nil {
				inst.onOpenURL(raw)
			}
		},
		func() {
			if inst.rendering {
				return
			}
			inst.renderNow(inst.rt)
			inst.notifyUpdate()
		},
		func(line string) {
			inst.mu.Lock()
			inst.logs = append(inst.logs, line)
			if len(inst.logs) > 64 {
				inst.logs = append([]string(nil), inst.logs[len(inst.logs)-64:]...)
			}
			inst.mu.Unlock()
		},
	)
}

func installBarGlobals(
	rt *moejs.Runtime,
	spec Spec,
	cookies *CookieJar,
	storage *KVStore,
	onCopy func(string),
	onOpen func(string),
	onInvalidate func(),
	onLog func(string),
) error {
	logFn := func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		parts := make([]string, 0, len(args))
		for i := range args {
			s, err := r.ToString(moejs.Arg(args, i))
			if err != nil {
				return moejs.Undefined(), err
			}
			parts = append(parts, s.GoString())
		}
		if onLog != nil {
			onLog(strings.Join(parts, " "))
		}
		return moejs.Undefined(), nil
	}

	storageAPI := map[string]any{
		"get": rt.Function("get", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			if storage == nil {
				return moejs.Null(), nil
			}
			k, err := r.ToString(moejs.Arg(args, 0))
			if err != nil {
				return moejs.Undefined(), err
			}
			v, ok := storage.Get(k.GoString())
			if !ok {
				return moejs.Null(), nil
			}
			return moejs.String(v), nil
		}),
		"set": rt.Function("set", 2, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			if storage == nil {
				return moejs.Undefined(), nil
			}
			k, err := r.ToString(moejs.Arg(args, 0))
			if err != nil {
				return moejs.Undefined(), err
			}
			v, err := r.ToString(moejs.Arg(args, 1))
			if err != nil {
				return moejs.Undefined(), err
			}
			if err := storage.Set(k.GoString(), v.GoString()); err != nil {
				return moejs.Undefined(), err
			}
			return moejs.Undefined(), nil
		}),
		"delete": rt.Function("delete", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			if storage == nil {
				return moejs.Undefined(), nil
			}
			k, err := r.ToString(moejs.Arg(args, 0))
			if err != nil {
				return moejs.Undefined(), err
			}
			return moejs.Undefined(), storage.Delete(k.GoString())
		}),
		"clear": rt.Function("clear", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
			if storage == nil {
				return moejs.Undefined(), nil
			}
			return moejs.Undefined(), storage.Clear()
		}),
		"keys": rt.Function("keys", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
			if storage == nil {
				return rt.FromGo([]string{})
			}
			return rt.FromGo(storage.Keys())
		}),
	}

	cookiesAPI := map[string]any{
		"get": rt.Function("get", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			s, err := r.ToString(moejs.Arg(args, 0))
			if err != nil {
				return moejs.Undefined(), err
			}
			rawURL := strings.TrimSpace(s.GoString())
			if err := OriginAllowed(rawURL, spec.Permissions); err != nil {
				return moejs.Undefined(), err
			}
			out := map[string]string{}
			if cookies != nil {
				if u, err := url.Parse(rawURL); err == nil {
					for _, c := range cookies.Cookies(u) {
						out[c.Name] = c.Value
					}
				}
			}
			return rt.FromGo(out)
		}),
		"header": rt.Function("header", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			s, err := r.ToString(moejs.Arg(args, 0))
			if err != nil {
				return moejs.Undefined(), err
			}
			rawURL := strings.TrimSpace(s.GoString())
			if err := OriginAllowed(rawURL, spec.Permissions); err != nil {
				return moejs.Undefined(), err
			}
			if cookies == nil {
				return moejs.String(""), nil
			}
			return moejs.String(cookies.HeaderFor(rawURL)), nil
		}),
		"set": rt.Function("set", 3, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			if cookies == nil {
				return moejs.Undefined(), nil
			}
			us, err := r.ToString(moejs.Arg(args, 0))
			if err != nil {
				return moejs.Undefined(), err
			}
			rawURL := strings.TrimSpace(us.GoString())
			if err := OriginAllowed(rawURL, spec.Permissions); err != nil {
				return moejs.Undefined(), err
			}
			u, err := url.Parse(rawURL)
			if err != nil || u.Hostname() == "" {
				return moejs.Undefined(), errors.New("invalid url")
			}
			ns, err := r.ToString(moejs.Arg(args, 1))
			if err != nil {
				return moejs.Undefined(), err
			}
			vs, err := r.ToString(moejs.Arg(args, 2))
			if err != nil {
				return moejs.Undefined(), err
			}
			host := strings.ToLower(u.Hostname())
			sc := StoredCookie{
				Name:   strings.TrimSpace(ns.GoString()),
				Value:  vs.GoString(),
				Domain: host,
				Path:   "/",
				Secure: strings.EqualFold(u.Scheme, "https"),
			}
			if len(args) >= 4 && !moejs.Arg(args, 3).IsUndefined() && !moejs.Arg(args, 3).IsNull() {
				rawOpts, err := rt.ToGo(moejs.Arg(args, 3))
				if err != nil {
					return moejs.Undefined(), err
				}
				if om, ok := rawOpts.(map[string]any); ok {
					if p, ok := om["path"].(string); ok && strings.HasPrefix(p, "/") {
						sc.Path = p
					}
					if d, ok := om["domain"].(string); ok && strings.TrimSpace(d) != "" {
						dom := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(d), "."))
						if host != dom && (!strings.Contains(dom, ".") || !strings.HasSuffix(host, "."+dom)) {
							return moejs.Undefined(), fmt.Errorf("cookie domain %q does not match %q", d, host)
						}
						sc.Domain = "." + dom
					}
					if b, ok := om["secure"].(bool); ok {
						sc.Secure = b
					}
					if b, ok := om["httpOnly"].(bool); ok {
						sc.HttpOnly = b
					}
					if ma, ok := toFloat64(om["maxAge"]); ok {
						if ma <= 0 {
							_, err := cookies.Delete(sc.Domain, sc.Name, sc.Path)
							return moejs.Undefined(), err
						}
						sc.Expires = time.Now().Add(time.Duration(ma) * time.Second)
					}
				}
			}
			return moejs.Undefined(), cookies.Set(sc)
		}),
		"delete": rt.Function("delete", 2, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			if cookies == nil {
				return moejs.Undefined(), nil
			}
			us, err := r.ToString(moejs.Arg(args, 0))
			if err != nil {
				return moejs.Undefined(), err
			}
			rawURL := strings.TrimSpace(us.GoString())
			if err := OriginAllowed(rawURL, spec.Permissions); err != nil {
				return moejs.Undefined(), err
			}
			u, err := url.Parse(rawURL)
			if err != nil || u.Hostname() == "" {
				return moejs.Undefined(), errors.New("invalid url")
			}
			ns, err := r.ToString(moejs.Arg(args, 1))
			if err != nil {
				return moejs.Undefined(), err
			}
			_, err = cookies.DeleteForHost(u.Hostname(), ns.GoString())
			return moejs.Undefined(), err
		}),
		"clear": rt.Function("clear", 0, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			if cookies == nil {
				return moejs.Undefined(), nil
			}
			arg := moejs.Arg(args, 0)
			if arg.IsUndefined() || arg.IsNull() {
				return moejs.Undefined(), cookies.Clear("")
			}
			us, err := r.ToString(arg)
			if err != nil {
				return moejs.Undefined(), err
			}
			rawURL := strings.TrimSpace(us.GoString())
			if rawURL == "" {
				return moejs.Undefined(), cookies.Clear("")
			}
			if err := OriginAllowed(rawURL, spec.Permissions); err != nil {
				return moejs.Undefined(), err
			}
			u, err := url.Parse(rawURL)
			if err != nil || u.Hostname() == "" {
				return moejs.Undefined(), errors.New("invalid url")
			}
			_, err = cookies.DeleteForHost(u.Hostname(), "")
			return moejs.Undefined(), err
		}),
	}

	metrix := map[string]any{
		"invalidate": rt.Function("invalidate", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
			if onInvalidate != nil {
				onInvalidate()
			}
			return moejs.Undefined(), nil
		}),
		"openURL": rt.Function("openURL", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			s, err := r.ToString(moejs.Arg(args, 0))
			if err != nil {
				return moejs.Undefined(), err
			}
			raw := strings.TrimSpace(s.GoString())
			if u, err := url.Parse(raw); err == nil {
				scheme := strings.ToLower(u.Scheme)
				if (scheme == "http" || scheme == "https") && onOpen != nil {
					onOpen(raw)
				}
			}
			return moejs.Undefined(), nil
		}),
		"copy": rt.Function("copy", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			s, err := r.ToString(moejs.Arg(args, 0))
			if err != nil {
				return moejs.Undefined(), err
			}
			if onCopy != nil {
				onCopy(s.GoString())
			}
			return moejs.Undefined(), nil
		}),
		"log":     rt.Function("log", 0, logFn),
		"storage": storageAPI,
		"cookies": cookiesAPI,
	}
	if err := rt.SetGlobal("$metrix", metrix); err != nil {
		return err
	}

	realm := rt.Realm()
	lsObj := realm.NewObjectWithProto(realm.ObjectPrototype)
	const attrMethod = 1<<0 | 1<<1 | 1<<2
	const attrAccessor = 1<<1 | 1<<2
	defMethod := func(name string, length int, fn moejs.NativeFunc) {
		lsObj.DefineOwnDataFast(realm, engine.StringKey(realm.InternGoString(name)), rt.Function(name, length, fn), attrMethod)
	}
	defMethod("getItem", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		if storage == nil {
			return moejs.Null(), nil
		}
		k, err := r.ToString(moejs.Arg(args, 0))
		if err != nil {
			return moejs.Undefined(), err
		}
		v, ok := storage.Get(k.GoString())
		if !ok {
			return moejs.Null(), nil
		}
		return moejs.String(v), nil
	})
	defMethod("setItem", 2, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		if storage == nil {
			return moejs.Undefined(), nil
		}
		k, err := r.ToString(moejs.Arg(args, 0))
		if err != nil {
			return moejs.Undefined(), err
		}
		v, err := r.ToString(moejs.Arg(args, 1))
		if err != nil {
			return moejs.Undefined(), err
		}
		return moejs.Undefined(), storage.Set(k.GoString(), v.GoString())
	})
	defMethod("removeItem", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		if storage == nil {
			return moejs.Undefined(), nil
		}
		k, err := r.ToString(moejs.Arg(args, 0))
		if err != nil {
			return moejs.Undefined(), err
		}
		return moejs.Undefined(), storage.Delete(k.GoString())
	})
	defMethod("clear", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
		if storage == nil {
			return moejs.Undefined(), nil
		}
		return moejs.Undefined(), storage.Clear()
	})
	defMethod("key", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		if storage == nil {
			return moejs.Null(), nil
		}
		n, err := r.ToNumber(moejs.Arg(args, 0))
		if err != nil {
			return moejs.Undefined(), err
		}
		keys := storage.Keys()
		idx := int(n)
		if idx < 0 || idx >= len(keys) {
			return moejs.Null(), nil
		}
		return moejs.String(keys[idx]), nil
	})
	getLen := realm.NewNativeFunction(realm.InternGoString("get length"), 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
		if storage == nil {
			return moejs.Int(0), nil
		}
		return moejs.Int(int64(storage.Count())), nil
	})
	lsObj.DefineOwnAccessorFast(realm, engine.StringKey(realm.InternGoString("length")), getLen, nil, attrAccessor)
	if err := rt.SetGlobal("localStorage", engine.ObjectValue(lsObj)); err != nil {
		return err
	}

	console := map[string]any{
		"log":   rt.Function("log", 0, logFn),
		"info":  rt.Function("info", 0, logFn),
		"warn":  rt.Function("warn", 0, logFn),
		"error": rt.Function("error", 0, logFn),
	}
	if err := rt.SetGlobal("console", console); err != nil {
		return err
	}
	if err := rt.SetGlobal("log", rt.Function("log", 0, logFn)); err != nil {
		return err
	}
	return installBarUIKit(rt)
}

func (inst *Instance) isOffline() bool {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.offline
}

// Spec returns the parsed .bar component specification.
func (inst *Instance) Spec() Spec { return inst.spec }

// Cookies returns the component's CookieJar.
func (inst *Instance) Cookies() *CookieJar { return inst.cookies }

// Storage returns the component's KVStore.
func (inst *Instance) Storage() *KVStore { return inst.storage }

// ReloadState reloads persisted cookies and storage from disk.
func (inst *Instance) ReloadState() error {
	if inst == nil {
		return nil
	}
	var err1, err2 error
	if inst.cookies != nil {
		err1 = inst.cookies.Reload()
	}
	if inst.storage != nil {
		err2 = inst.storage.Reload()
	}
	return errors.Join(err1, err2)
}

// Refreshing reports whether scheduled(env) is currently in flight.
func (inst *Instance) Refreshing() bool {
	if inst == nil {
		return false
	}
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.schedRunning
}

// Snapshot returns the latest UI + tray snapshot.
func (inst *Instance) Snapshot() Snapshot {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.snap
}

// Refresh asks scheduled(env) to fetch fresh data on the event loop.
func (inst *Instance) Refresh() {
	inst.mu.RLock()
	closed := inst.closed
	inst.mu.RUnlock()
	if closed || inst.loop == nil {
		return
	}
	inst.loop.RunOnLoop(func(r *moejs.Runtime) {
		inst.runScheduled(r)
	})
}

// UpdateInputs updates input values and language, re-renders immediately, and
// triggers scheduled(env).
func (inst *Instance) UpdateInputs(inputs map[string]string, lang string) {
	inst.mu.Lock()
	if inst.closed {
		inst.mu.Unlock()
		return
	}
	inst.inputs = MergeInputs(inst.spec, inputs)
	if lang != "" {
		inst.lang = lang
	}
	inst.asyncErr = ""
	inst.mu.Unlock()

	inst.loop.RunOnLoop(func(r *moejs.Runtime) {
		inst.renderNow(r)
		inst.notifyUpdate()
		inst.runScheduled(r)
	})
}

// SetOffline updates whether network requests are blocked.
func (inst *Instance) SetOffline(offline bool) {
	inst.mu.Lock()
	inst.offline = offline
	inst.mu.Unlock()
}

// SetAppearance updates the effective host theme ("dark"|"light") and vibrancy
// flag and re-renders the component if either changed.
func (inst *Instance) SetAppearance(theme string, vibrancy bool) {
	if inst == nil || inst.loop == nil {
		return
	}
	theme = normalizeBarTheme(theme)
	inst.mu.Lock()
	if inst.closed || (inst.theme == theme && inst.vibrancy == vibrancy) {
		inst.mu.Unlock()
		return
	}
	inst.theme = theme
	inst.vibrancy = vibrancy
	inst.mu.Unlock()

	inst.loop.RunOnLoop(func(r *moejs.Runtime) {
		inst.renderNow(r)
		inst.notifyUpdate()
	})
}

// InvokeAction executes a callback captured from the latest render(env)
// (such as button onClick, shortcut, or tray menu item).
func (inst *Instance) InvokeAction(actionID string) {
	inst.InvokeActionWithArg(actionID, nil)
}

// InvokeActionWithArg executes an action callback (onClick, onChange, onSubmit)
// passing arg when non-nil.
func (inst *Instance) InvokeActionWithArg(actionID string, arg any) {
	if actionID == "" {
		return
	}
	inst.mu.RLock()
	closed := inst.closed
	inst.mu.RUnlock()
	if closed || inst.loop == nil {
		return
	}
	inst.loop.RunOnLoop(func(r *moejs.Runtime) {
		inst.mu.RLock()
		fn := inst.actions[actionID]
		inst.mu.RUnlock()
		if fn == nil {
			return
		}
		var callArgs []moejs.Value
		if arg != nil {
			if jsArg, err := r.FromGo(arg); err == nil {
				callArgs = []moejs.Value{jsArg}
			}
		}
		if _, err := r.CallFunction(engine.ObjectValue(fn), moejs.Undefined(), callArgs...); err != nil {
			inst.recordError(err.Error())
		} else {
			inst.renderNow(r)
			inst.notifyUpdate()
		}
	})
}

// InvokeActionSync runs actionID on the loop, waits for a returned Promise (if
// any) up to timeout, re-renders, and returns the updated Snapshot.
func (inst *Instance) InvokeActionSync(actionID string, timeout time.Duration) (Snapshot, bool) {
	return inst.InvokeActionSyncWithArg(actionID, nil, timeout)
}

// InvokeActionSyncWithArg runs actionID with an optional argument on the loop,
// waits for any returned Promise up to timeout, re-renders, and returns Snapshot.
func (inst *Instance) InvokeActionSyncWithArg(actionID string, arg any, timeout time.Duration) (Snapshot, bool) {
	if actionID == "" {
		return Snapshot{}, false
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	inst.mu.RLock()
	closed := inst.closed
	inst.mu.RUnlock()
	if closed || inst.loop == nil {
		return Snapshot{}, false
	}

	type res struct {
		snap Snapshot
		ok   bool
	}
	ch := make(chan res, 1)
	var once sync.Once
	finish := func(s Snapshot, ok bool) {
		once.Do(func() { ch <- res{snap: s, ok: ok} })
	}

	posted := inst.loop.RunOnLoop(func(r *moejs.Runtime) {
		inst.mu.RLock()
		fn := inst.actions[actionID]
		inst.mu.RUnlock()
		if fn == nil {
			finish(Snapshot{}, false)
			return
		}
		var callArgs []moejs.Value
		if arg != nil {
			if jsArg, err := r.FromGo(arg); err == nil {
				callArgs = []moejs.Value{jsArg}
			}
		}
		ret, err := r.CallFunction(engine.ObjectValue(fn), moejs.Undefined(), callArgs...)
		if err != nil {
			inst.recordError(err.Error())
			finish(inst.Snapshot(), true)
			return
		}
		if ret.IsObject() {
			if thenFn, err := r.Get(ret, "then"); err == nil && engine.IsCallable(thenFn) {
				onResolve := r.Function("onResolve", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
					inst.renderNow(r)
					inst.notifyUpdate()
					finish(inst.Snapshot(), true)
					return moejs.Undefined(), nil
				})
				onReject := r.Function("onReject", 1, func(realm *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
					msg := extractJSError(r, realm, args)
					inst.recordError(msg)
					finish(inst.Snapshot(), true)
					return moejs.Undefined(), nil
				})
				if _, callErr := r.CallFunction(thenFn, ret, onResolve, onReject); callErr != nil {
					inst.recordError(callErr.Error())
					finish(inst.Snapshot(), true)
				}
				return
			}
		}
		inst.renderNow(r)
		inst.notifyUpdate()
		finish(inst.Snapshot(), true)
	})
	if !posted {
		return Snapshot{}, false
	}
	select {
	case out := <-ch:
		return out.snap, out.ok
	case <-time.After(timeout):
		return inst.Snapshot(), false
	}
}

// WaitIdle waits until any pending scheduled(env) run completes and the latest
// snapshot is committed, up to timeout.
func (inst *Instance) WaitIdle(timeout time.Duration) bool {
	if inst == nil || inst.loop == nil {
		return false
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	deadline := time.Now().Add(timeout)

	barrier := func() bool {
		rem := time.Until(deadline)
		if rem <= 0 {
			return false
		}
		done := make(chan struct{})
		if !inst.loop.RunOnLoop(func(_ *moejs.Runtime) { close(done) }) {
			return false
		}
		select {
		case <-done:
			return true
		case <-time.After(rem):
			return false
		}
	}

	// Ensure startup/queued RunOnLoop callbacks have entered runScheduled.
	if !barrier() {
		return false
	}

	inst.mu.Lock()
	if inst.closed {
		inst.mu.Unlock()
		return false
	}
	if inst.schedRunning {
		ch := make(chan struct{})
		inst.schedWaiters = append(inst.schedWaiters, ch)
		inst.mu.Unlock()
		rem := time.Until(deadline)
		if rem <= 0 {
			return false
		}
		select {
		case <-ch:
		case <-time.After(rem):
			return false
		}
	} else {
		inst.mu.Unlock()
	}

	return barrier()
}

// Close cancels in-flight fetches, terminates the event loop, and frees roots.
func (inst *Instance) Close() {
	inst.mu.Lock()
	if inst.closed {
		inst.mu.Unlock()
		return
	}
	inst.closed = true
	inst.schedRunning = false
	waiters := inst.schedWaiters
	inst.schedWaiters = nil
	inst.mu.Unlock()
	for _, ch := range waiters {
		close(ch)
	}

	if inst.web != nil {
		inst.web.Close()
	}
	if inst.loop != nil {
		inst.loop.Terminate()
	}
	if inst.remove != nil {
		inst.remove()
	}
}

func (inst *Instance) startScheduled(r *moejs.Runtime) {
	inst.mu.Lock()
	inst.schedRunning = true
	inst.snap.Refreshing = true
	inst.mu.Unlock()
	if r != nil {
		inst.renderNow(r)
	}
	inst.notifyUpdate()
}

func (inst *Instance) finishScheduled(r *moejs.Runtime, asyncErr string) {
	inst.mu.Lock()
	if asyncErr != "" {
		inst.asyncErr = asyncErr
	}
	inst.schedRunning = false
	inst.snap.Refreshing = false
	waiters := inst.schedWaiters
	inst.schedWaiters = nil
	inst.mu.Unlock()
	if r != nil {
		inst.renderNow(r)
	}
	inst.notifyUpdate()
	for _, ch := range waiters {
		close(ch)
	}
}

func envJSONValue(r *moejs.Runtime, spec Spec, inputs map[string]string, lang string) (moejs.Value, error) {
	return envJSONValueWithAppearance(r, spec, inputs, lang, "dark", false, false)
}

func envJSONValueWithAppearance(r *moejs.Runtime, spec Spec, inputs map[string]string, lang, theme string, vibrancy, refreshing bool) (moejs.Value, error) {
	if lang == "" {
		lang = "en"
	}
	rawInputs := MergeInputs(spec, inputs)
	typedInputs := make(map[string]any, len(rawInputs))
	inputTypes := make(map[string]string, len(spec.Inputs))
	for _, in := range spec.Inputs {
		inputTypes[in.Name] = in.Type
	}
	for k, v := range rawInputs {
		switch inputTypes[k] {
		case "number":
			if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				typedInputs[k] = f
			} else {
				typedInputs[k] = v
			}
		case "boolean":
			typedInputs[k] = strings.EqualFold(strings.TrimSpace(v), "true") || strings.TrimSpace(v) == "1"
		default:
			typedInputs[k] = v
		}
	}
	b, err := json.Marshal(map[string]any{
		"inputs":     typedInputs,
		"lang":       lang,
		"theme":      normalizeBarTheme(theme),
		"vibrancy":   vibrancy,
		"refreshing": refreshing,
		"platform":   runtime.GOOS,
	})
	if err != nil {
		return moejs.Undefined(), err
	}
	return r.ParseJSONString(string(b))
}

func (inst *Instance) envValue(r *moejs.Runtime) (moejs.Value, error) {
	inst.mu.RLock()
	rawInputs := make(map[string]string, len(inst.inputs))
	for k, v := range inst.inputs {
		rawInputs[k] = v
	}
	lang := inst.lang
	theme := inst.theme
	vibrancy := inst.vibrancy
	refreshing := inst.schedRunning
	inst.mu.RUnlock()
	return envJSONValueWithAppearance(r, inst.spec, rawInputs, lang, theme, vibrancy, refreshing)
}

func (inst *Instance) renderNow(r *moejs.Runtime) {
	if inst.rendering {
		return
	}
	inst.rendering = true
	defer func() { inst.rendering = false }()

	inst.mu.RLock()
	defVal := inst.defObj
	inst.mu.RUnlock()
	if defVal.IsUndefined() || defVal.IsNull() {
		return
	}

	renderFn, err := r.Get(defVal, "render")
	if err != nil || !engine.IsCallable(renderFn) {
		inst.setErrorSnapshot("render() function not exported")
		return
	}
	envVal, err := inst.envValue(r)
	if err != nil {
		inst.setErrorSnapshot(err.Error())
		return
	}
	res, err := r.CallFunction(renderFn, defVal, envVal)
	if err != nil {
		inst.setErrorSnapshot(err.Error())
		return
	}
	snap, actions, err := extractSnapshot(r, res)
	if err != nil {
		inst.setErrorSnapshot(err.Error())
		return
	}
	inst.mu.Lock()
	snap.UpdatedAt = time.Now()
	snap.Refreshing = inst.schedRunning
	if snap.Error == "" && snap.Tray != nil && snap.Tray.Error != "" {
		snap.Error = snap.Tray.Error
	}
	if snap.Error == "" && inst.asyncErr != "" {
		snap.Error = inst.asyncErr
	}
	if len(inst.logs) > 0 {
		snap.Logs = append([]string(nil), inst.logs...)
	}
	inst.snap = snap
	inst.actions = actions
	inst.mu.Unlock()
}

func (inst *Instance) runScheduled(r *moejs.Runtime) {
	inst.mu.RLock()
	defVal := inst.defObj
	inst.mu.RUnlock()
	if defVal.IsUndefined() || defVal.IsNull() {
		return
	}
	schedFn, err := r.Get(defVal, "scheduled")
	if err != nil || !engine.IsCallable(schedFn) {
		inst.startScheduled(nil)
		inst.finishScheduled(r, "")
		return
	}
	inst.mu.Lock()
	inst.asyncErr = ""
	inst.mu.Unlock()

	inst.startScheduled(r)
	envVal, err := inst.envValue(r)
	if err != nil {
		inst.finishScheduled(r, err.Error())
		return
	}
	ret, err := r.CallFunction(schedFn, defVal, envVal)
	if err != nil {
		inst.finishScheduled(r, err.Error())
		return
	}
	if ret.IsObject() {
		thenFn, err := r.Get(ret, "then")
		if err == nil && engine.IsCallable(thenFn) {
			onResolve := r.Function("onResolve", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				inst.finishScheduled(r, "")
				return moejs.Undefined(), nil
			})
			onReject := r.Function("onReject", 1, func(realm *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
				inst.finishScheduled(r, extractJSError(r, realm, args))
				return moejs.Undefined(), nil
			})
			if _, callErr := r.CallFunction(thenFn, ret, onResolve, onReject); callErr != nil {
				inst.finishScheduled(r, callErr.Error())
			}
			return
		}
	}
	inst.finishScheduled(r, "")
}

func extractJSError(r *moejs.Runtime, realm *moejs.Realm, args []moejs.Value) string {
	msg := "scheduled() failed"
	if len(args) > 0 {
		if mVal, err := r.Get(args[0], "message"); err == nil && !mVal.IsUndefined() {
			if s, err := realm.ToString(mVal); err == nil {
				msg = s.GoString()
			}
		} else if s, err := realm.ToString(args[0]); err == nil {
			msg = s.GoString()
		}
	}
	return msg
}

func (inst *Instance) setErrorSnapshot(msg string) {
	inst.mu.Lock()
	inst.snap.Error = msg
	inst.snap.UpdatedAt = time.Now()
	inst.mu.Unlock()
}

func (inst *Instance) recordError(msg string) {
	inst.mu.Lock()
	inst.asyncErr = msg
	inst.mu.Unlock()
	inst.renderNow(inst.rt)
	inst.notifyUpdate()
}

func (inst *Instance) notifyUpdate() {
	if inst.onUpdate == nil {
		return
	}
	inst.onUpdate(inst.Snapshot())
}

func toFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int64:
		return float64(x), true
	case int:
		return float64(x), true
	default:
		return 0, false
	}
}

// CompileCheck parses a .bar XML document, validates permissions/inputs,
// compiles its ES module, and verifies that export default.render(env) runs.
func CompileCheck(source string) (Spec, []string, error) {
	spec, _, warnings, err := CheckRender(source, "zh")
	return spec, warnings, err
}

// CheckRender runs CompileCheck and also returns the initial offline Snapshot
// produced by default.render(env).
func CheckRender(source, lang string) (Spec, Snapshot, []string, error) {
	sp, err := Parse(source)
	if err != nil {
		return Spec{}, Snapshot{}, nil, err
	}
	var warnings []string

	if strings.ContainsAny(sp.ID, `/\`) || strings.Contains(sp.ID, "..") {
		return sp, Snapshot{}, nil, fmt.Errorf(".bar id %q contains illegal path characters", sp.ID)
	}
	for _, orig := range sp.Permissions {
		u, err := url.Parse(orig)
		if err != nil || (strings.ToLower(u.Scheme) != "http" && strings.ToLower(u.Scheme) != "https") || strings.TrimSpace(u.Hostname()) == "" {
			warnings = append(warnings, fmt.Sprintf("invalid network permission origin %q: must be http:// or https:// with host", orig))
		}
	}
	seenInputs := map[string]bool{}
	for _, in := range sp.Inputs {
		if seenInputs[in.Name] {
			warnings = append(warnings, fmt.Sprintf("duplicate <input name=%q>", in.Name))
		}
		seenInputs[in.Name] = true
		switch in.Type {
		case "string", "select", "number", "boolean", "password":
		default:
			warnings = append(warnings, fmt.Sprintf("unknown input type %q on %q (supported: string, select, number, boolean, password)", in.Type, in.Name))
		}
		if in.Type == "select" && len(in.Options) == 0 {
			warnings = append(warnings, fmt.Sprintf("select input %q has no <option> entries", in.Name))
		}
	}

	mod, err := moejs.Compile(sp.ID+".js", sp.Script)
	if err != nil {
		return sp, Snapshot{}, warnings, fmt.Errorf("bar script compile error: %w", err)
	}

	rt := moejs.NewRuntime(moejs.Options{MemoryLimit: 32 << 20})
	cookies, _ := NewCookieJar("")
	storage, _ := NewKVStore("")
	var logs []string
	if err := installBarGlobals(rt, sp, cookies, storage, nil, nil, nil, func(line string) {
		logs = append(logs, line)
	}); err != nil {
		return sp, Snapshot{}, warnings, err
	}
	if err := rt.Load(mod); err != nil {
		return sp, Snapshot{}, warnings, fmt.Errorf("bar script load error: %w", err)
	}
	defVal, hasDef := rt.Export("default")
	if !hasDef || defVal.IsUndefined() || defVal.IsNull() {
		return sp, Snapshot{}, warnings, errors.New("bar script must export default { render(env) { ... } }")
	}
	renderFn, err := rt.Get(defVal, "render")
	if err != nil || !engine.IsCallable(renderFn) {
		return sp, Snapshot{}, warnings, errors.New("bar default export must provide a callable render(env) method")
	}
	envVal, err := envJSONValue(rt, sp, nil, lang)
	if err != nil {
		return sp, Snapshot{}, warnings, err
	}
	resVal, err := rt.CallFunction(renderFn, defVal, envVal)
	if err != nil {
		return sp, Snapshot{}, warnings, fmt.Errorf("initial render(env) failed: %w", err)
	}
	snap, _, err := extractSnapshot(rt, resVal)
	if err != nil {
		return sp, Snapshot{}, warnings, fmt.Errorf("invalid render(env) return value: %w", err)
	}
	snap.Logs = logs
	if snap.Tray != nil && utf8.RuneCountInString(snap.Tray.Title) > 12 {
		warnings = append(warnings, fmt.Sprintf("tray.title %q exceeds 12 characters and will be truncated in the menu bar", snap.Tray.Title))
	}
	if snap.Tray == nil && snap.Panel == nil {
		warnings = append(warnings, "render(env) returned neither tray nor panel")
	}
	return sp, snap, warnings, nil
}
