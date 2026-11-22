package bar

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MoeForge/moejs"
	"github.com/MoeForge/moejs/engine"
	"github.com/MoeForge/moejs/eventloop"
	"github.com/MoeForge/moejs/webapi"
)

// Options configures a running .bar Instance.
type Options struct {
	Client    *http.Client
	Inputs    map[string]string
	Lang      string
	Offline   bool
	OnUpdate  func(Snapshot)
	OnOpenURL func(string)
}

// Instance runs a single parsed .bar component in its own moejs runtime and
// event loop. Its public methods are safe to call from any goroutine.
type Instance struct {
	spec      Spec
	mod       *moejs.Module
	rt        *moejs.Runtime
	loop      *eventloop.Loop
	web       *webapi.API
	remove    func()
	onUpdate  func(Snapshot)
	onOpenURL func(string)

	mu        sync.RWMutex
	inputs    map[string]string
	lang      string
	offline   bool
	snap      Snapshot
	asyncErr  string
	logs      []string
	closed    bool
	rendering bool

	// Loop-goroutine-only references kept rooted via AddLiveRoots.
	defObj  moejs.Value
	actions map[string]*moejs.Object
}

// NewInstance compiles spec.Script, boots its event loop + webapi, performs an
// initial render, and kicks off scheduled(env).
func NewInstance(spec Spec, opts Options) (*Instance, error) {
	if strings.TrimSpace(spec.Script) == "" {
		return nil, errors.New("bar: empty script")
	}
	mod, err := moejs.Compile(spec.ID+".js", spec.Script)
	if err != nil {
		return nil, fmt.Errorf("bar compile %s: %w", spec.ID, err)
	}
	rt := moejs.NewRuntime(moejs.Options{MemoryLimit: 32 << 20})

	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	inst := &Instance{
		spec:      spec,
		mod:       mod,
		rt:        rt,
		onUpdate:  opts.OnUpdate,
		onOpenURL: opts.OnOpenURL,
		inputs:    MergeInputs(spec, opts.Inputs),
		lang:      opts.Lang,
		offline:   opts.Offline,
		actions:   map[string]*moejs.Object{},
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
		Client: client,
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
	logFn := func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		parts := make([]string, 0, len(args))
		for i := range args {
			s, err := r.ToString(moejs.Arg(args, i))
			if err != nil {
				return moejs.Undefined(), err
			}
			parts = append(parts, s.GoString())
		}
		line := strings.Join(parts, " ")
		inst.mu.Lock()
		inst.logs = append(inst.logs, line)
		if len(inst.logs) > 64 {
			inst.logs = append([]string(nil), inst.logs[len(inst.logs)-64:]...)
		}
		inst.mu.Unlock()
		return moejs.Undefined(), nil
	}

	metrix := map[string]any{
		"invalidate": inst.rt.Function("invalidate", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
			if inst.rendering {
				return moejs.Undefined(), nil
			}
			inst.renderNow(inst.rt)
			inst.notifyUpdate()
			return moejs.Undefined(), nil
		}),
		"openURL": inst.rt.Function("openURL", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			s, err := r.ToString(moejs.Arg(args, 0))
			if err != nil {
				return moejs.Undefined(), err
			}
			raw := strings.TrimSpace(s.GoString())
			if u, err := url.Parse(raw); err == nil {
				scheme := strings.ToLower(u.Scheme)
				if (scheme == "http" || scheme == "https") && inst.onOpenURL != nil {
					go inst.onOpenURL(raw)
				}
			}
			return moejs.Undefined(), nil
		}),
		"log": inst.rt.Function("log", 0, logFn),
	}
	if err := inst.rt.SetGlobal("$metrix", metrix); err != nil {
		return err
	}

	console := map[string]any{
		"log":   inst.rt.Function("log", 0, logFn),
		"info":  inst.rt.Function("info", 0, logFn),
		"warn":  inst.rt.Function("warn", 0, logFn),
		"error": inst.rt.Function("error", 0, logFn),
	}
	if err := inst.rt.SetGlobal("console", console); err != nil {
		return err
	}
	return inst.rt.SetGlobal("log", inst.rt.Function("log", 0, logFn))
}

func (inst *Instance) isOffline() bool {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.offline
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

// InvokeAction executes a callback captured from the latest render(env)
// (such as button onClick, shortcut, or tray menu item).
func (inst *Instance) InvokeAction(actionID string) {
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
		if _, err := r.CallFunction(engine.ObjectValue(fn), moejs.Undefined()); err != nil {
			inst.recordError(err.Error())
		} else {
			inst.renderNow(r)
			inst.notifyUpdate()
		}
	})
}

// Close cancels in-flight fetches, terminates the event loop, and frees roots.
func (inst *Instance) Close() {
	inst.mu.Lock()
	if inst.closed {
		inst.mu.Unlock()
		return
	}
	inst.closed = true
	inst.mu.Unlock()

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

func (inst *Instance) envValue(r *moejs.Runtime) (moejs.Value, error) {
	inst.mu.RLock()
	rawInputs := make(map[string]string, len(inst.inputs))
	for k, v := range inst.inputs {
		rawInputs[k] = v
	}
	lang := inst.lang
	inst.mu.RUnlock()

	if lang == "" {
		lang = "en"
	}
	typedInputs := make(map[string]any, len(rawInputs))
	inputTypes := make(map[string]string, len(inst.spec.Inputs))
	for _, in := range inst.spec.Inputs {
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
		"inputs":   typedInputs,
		"lang":     lang,
		"platform": runtime.GOOS,
	})
	if err != nil {
		return moejs.Undefined(), err
	}
	return r.ParseJSONString(string(b))
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
		return
	}
	envVal, err := inst.envValue(r)
	if err != nil {
		inst.recordError(err.Error())
		return
	}
	inst.mu.Lock()
	inst.asyncErr = ""
	inst.mu.Unlock()

	ret, err := r.CallFunction(schedFn, defVal, envVal)
	if err != nil {
		inst.recordError(err.Error())
		return
	}
	// Attach rejection handler in case scheduled() returns a rejected promise
	// without catching inside JS.
	if ret.IsObject() {
		thenFn, err := r.Get(ret, "then")
		if err == nil && engine.IsCallable(thenFn) {
			noop := r.Function("noop", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				return moejs.Undefined(), nil
			})
			onReject := r.Function("onReject", 1, func(realm *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
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
				inst.recordError(msg)
				return moejs.Undefined(), nil
			})
			_, _ = r.CallFunction(thenFn, ret, noop, onReject)
		}
	}
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
