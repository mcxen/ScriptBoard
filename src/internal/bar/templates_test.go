package bar

import (
	"testing"
	"time"
)

func TestBuiltinTemplatesCompileBootAndInteract(t *testing.T) {
	tpls := Templates()
	if len(tpls) < 6 {
		t.Fatalf("expected at least 6 built-in templates, got %d", len(tpls))
	}

	seen := map[string]bool{}
	for _, tpl := range tpls {
		if tpl.ID == "" || tpl.Name == "" || tpl.Style == "" || tpl.Source == "" {
			t.Fatalf("template missing required metadata: %+v", tpl)
		}
		if seen[tpl.ID] {
			t.Fatalf("duplicate template id %q", tpl.ID)
		}
		seen[tpl.ID] = true

		gotTpl, ok := TemplateByID(tpl.ID)
		if !ok || gotTpl.ID != tpl.ID {
			t.Fatalf("TemplateByID(%q) failed", tpl.ID)
		}

		sp, warnings, err := CompileCheck(tpl.Source)
		if err != nil {
			t.Fatalf("CompileCheck(%s) failed: %v", tpl.ID, err)
		}
		if len(warnings) > 0 {
			t.Errorf("template %s produced warnings: %v", tpl.ID, warnings)
		}
		if sp.ID != tpl.ID {
			t.Errorf("template %s parsed spec id = %q", tpl.ID, sp.ID)
		}

		inst, err := NewInstance(sp, Options{
			DataDir: t.TempDir(),
			Offline: true,
			Theme:   "dark",
		})
		if err != nil {
			t.Fatalf("NewInstance(%s): %v", tpl.ID, err)
		}
		if !inst.WaitIdle(5 * time.Second) {
			inst.Close()
			t.Fatalf("WaitIdle(%s) timed out", tpl.ID)
		}
		snap := inst.Snapshot()
		inst.Close()

		if snap.Error != "" {
			t.Errorf("template %s runtime error: %s", tpl.ID, snap.Error)
		}
		if snap.Tray == nil || snap.Tray.Title == "" {
			t.Errorf("template %s missing tray title: %+v", tpl.ID, snap.Tray)
		}
		if snap.Panel == nil {
			t.Errorf("template %s missing panel", tpl.ID)
		}
		if acts := CollectActions(snap); len(acts) == 0 {
			t.Errorf("template %s expected interactive actions", tpl.ID)
		}
	}
}

func TestInvokeActionWithArgAndSetAppearance(t *testing.T) {
	tpl, ok := TemplateByID("utility-switchboard")
	if !ok {
		t.Fatal("missing utility-switchboard template")
	}
	sp, err := Parse(tpl.Source)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := NewInstance(sp, Options{
		DataDir: t.TempDir(),
		Theme:   "dark",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close()
	if !inst.WaitIdle(5 * time.Second) {
		t.Fatal("WaitIdle timed out")
	}

	snap := inst.Snapshot()
	var switchID, segID string
	for _, act := range CollectActions(snap) {
		switch act.Type {
		case "Switch.onChange":
			if switchID == "" {
				switchID = act.ID
			}
		case "Segmented.onChange":
			segID = act.ID
		}
	}
	if switchID == "" || segID == "" {
		t.Fatalf("expected Switch.onChange and Segmented.onChange actions, got %+v", CollectActions(snap))
	}

	// Toggle first Switch to true -> persists stayAwake="1" and updates tray count to "3 ON".
	if _, ok := inst.InvokeActionSyncWithArg(switchID, true, 3*time.Second); !ok {
		t.Fatal("InvokeActionSyncWithArg(switch) failed")
	}
	if v, ok := inst.Storage().Get("stayAwake"); !ok || v != "1" {
		t.Fatalf("stayAwake storage = %q (%v), want 1", v, ok)
	}
	if title := inst.Snapshot().Tray.Title; title != "3 ON" {
		t.Fatalf("tray title after toggling switch = %q, want 3 ON", title)
	}

	// Switch Segmented tab to "actions" -> ListRow entries appear.
	snap2, ok := inst.InvokeActionSyncWithArg(segID, "actions", 3*time.Second)
	if !ok {
		t.Fatal("InvokeActionSyncWithArg(segmented) failed")
	}
	foundListRow := false
	for _, act := range CollectActions(snap2) {
		if act.Type == "ListRow" {
			foundListRow = true
			break
		}
	}
	if !foundListRow {
		t.Fatalf("expected ListRow actions on actions tab, got %+v", CollectActions(snap2))
	}

	// SetAppearance updates env.theme on a theme-aware component.
	themeXML := `<bar id="theme-env" title="Theme">
  <script><![CDATA[
    export default {
      render(env) {
        return {
          tray: { title: env.theme },
          panel: h("Column", { preferredWidth: 260, padding: 12 },
            $ui.kv("Theme", env.theme),
            $ui.kv("Vib", String(env.vibrancy))
          )
        };
      }
    };
  ]]></script>
</bar>`
	spTheme, err := Parse(themeXML)
	if err != nil {
		t.Fatal(err)
	}
	instTheme, err := NewInstance(spTheme, Options{Theme: "dark"})
	if err != nil {
		t.Fatal(err)
	}
	defer instTheme.Close()
	instTheme.WaitIdle(3 * time.Second)
	if got := instTheme.Snapshot().Tray.Title; got != "dark" {
		t.Fatalf("initial env.theme = %q, want dark", got)
	}
	instTheme.SetAppearance("light", true)
	instTheme.WaitIdle(3 * time.Second)
	if got := instTheme.Snapshot().Tray.Title; got != "light" {
		t.Fatalf("after SetAppearance env.theme = %q, want light", got)
	}
}

func TestWeatherBarUIKitPrimitives(t *testing.T) {
	xml := `<bar id="uikit-weather-test" title="UIKit Test">
  <script><![CDATA[
    let loading = true;
    export default {
      render() {
        const refresh = () => { loading = !loading; $metrix.invalidate(); };
        return {
          tray: { title: "24ms" },
          panel: h("Column", { preferredWidth: 300, padding: [10, 16, 16, 16], gap: 14, safeArea: true },
            $ui.topBar({
              icon: "map-pin",
              title: "Hangzhou",
              subtitle: "Zhejiang",
              loading: loading,
              externalURL: "https://open-meteo.com/",
              onRefresh: refresh
            }),
            $ui.staleError("Timeout using cached data"),
            $ui.hero({
              value: "24",
              unit: "ms",
              label: "Optimal",
              secondary: "12 / 28",
              icon: "activity",
              iconColor: "accent",
              metrics: [
                { icon: "wind", value: "12 km/h" },
                { icon: "droplets", value: "65%" }
              ]
            }),
            $ui.rangeRow({
              key: "day-0",
              label: "Today",
              icon: "sun",
              tag: "40%",
              min: 10,
              max: 30,
              low: 14,
              high: 26,
              lowLabel: "14°",
              highLabel: "26°",
              color: "accent",
              toColor: "cyan",
              active: true
            }),
            $ui.skeleton("hero")
          )
        };
      }
    };
  ]]></script>
</bar>`
	sp, err := Parse(xml)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := NewInstance(sp, Options{Theme: "dark"})
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close()
	if !inst.WaitIdle(3 * time.Second) {
		t.Fatal("WaitIdle timed out")
	}
	snap := inst.Snapshot()
	if snap.Error != "" {
		t.Fatalf("unexpected runtime error: %s", snap.Error)
	}
	texts := CollectTexts(snap.Panel)
	joined := ""
	for _, s := range texts {
		joined += s + "|"
	}
	for _, want := range []string{"Hangzhou", "Zhejiang", "Timeout using cached data", "24", "ms", "Optimal", "12 km/h", "65%", "Today", "40%", "14°", "26°"} {
		found := false
		for _, s := range texts {
			if s == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing expected UIKit text %q in %v", want, joined)
		}
	}
	acts := CollectActions(snap)
	if len(acts) < 2 {
		t.Fatalf("expected externalURL and refresh button actions, got %+v", acts)
	}
}
