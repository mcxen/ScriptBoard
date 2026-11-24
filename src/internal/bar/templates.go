package bar

import "sort"

// Template is a curated, design-polished reference .bar menubar component that
// developers or AI assistants can inspect via MCP (bar_templates /
// bar_template_get) or create from in the desktop UI.
type Template struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Style            string   `json:"style"`
	Category         string   `json:"category"`
	Icon             string   `json:"icon"`
	Description      string   `json:"description"`
	RecommendedWidth int      `json:"recommendedWidth"`
	Tags             []string `json:"tags"`
	Features         []string `json:"features"`
	DesignNotes      string   `json:"designNotes"`
	Source           string   `json:"source,omitempty"`
}

// Templates returns all built-in menubar reference style templates sorted in
// curated showcase order.
func Templates() []Template {
	out := make([]Template, len(builtinTemplates))
	copy(out, builtinTemplates)
	return out
}

// TemplateIDs returns the ordered slice of built-in template IDs.
func TemplateIDs() []string {
	out := make([]string, len(builtinTemplates))
	for i, t := range builtinTemplates {
		out[i] = t.ID
	}
	sort.Strings(out)
	return out
}

// TemplateByID returns a built-in template by its ID.
func TemplateByID(id string) (Template, bool) {
	for _, t := range builtinTemplates {
		if t.ID == id {
			return t, true
		}
	}
	return Template{}, false
}

var builtinTemplates = []Template{
	{
		ID:               "native-glass",
		Name:             "Apple Native Glass",
		Style:            "Weather-Bar Translucent Minimal",
		Category:         "system",
		Icon:             "gauge",
		Description:      "Frameless Apple-native glass layout modeled on Weather Bar: 26px top bar, light 48px tnum Hero, 1px dividers, hourly curve, and 4px distribution range bars.",
		RecommendedWidth: 300,
		Tags:             []string{"native", "weather", "glass", "minimal", "range"},
		Features:         []string{"material=\"glass\"", "$ui.topBar", "$ui.hero", "$ui.rangeRow", "Chart(area)", "safeArea"},
		DesignNotes:      "Zero nested bordered cards. Relies on padding [10,16,16,16], gap 14, 1px $ui.divider(), fontWeight 300 oversized numerals with tnum + contentTransition, and gradient 4px range bars directly on VibrancyGlass.",
		Source: `<bar protocol-version="0"
     version="1.0.0"
     id="native-glass"
     title="Edge Health"
     category="system"
     icon="gauge"
     description="Frameless Apple-native glass monitor modeled on Weather Bar with 48px light Hero typography, monotone curve, and 4px range distribution bars."
     refresh="1m"
     material="glass">
  <inputs>
    <input name="region" type="string" label="Primary Region" default="Tokyo-01" />
  </inputs>
  <script><![CDATA[
    let sample = 0;
    let lastError = "";

    const REGIONS = [
      { id: "NRT", label: "Tokyo", icon: "zap", color: "accent", lo: 12, hi: 26, tag: "0%" },
      { id: "SGP", label: "SGP", icon: "signal", color: "accent", lo: 18, hi: 34, tag: "" },
      { id: "FRA", label: "FRA", icon: "activity", color: "accent", lo: 24, hi: 48, tag: "1%" },
      { id: "IAD", label: "IAD", icon: "globe", color: "accent", lo: 28, hi: 52, tag: "" },
      { id: "SJC", label: "SJC", icon: "gauge", color: "accent", lo: 19, hi: 38, tag: "" }
    ];

    const HOURLY = [
      { label: "Now", icon: "zap", ms: 24 },
      { label: "09", icon: "signal", ms: 22 },
      { label: "11", icon: "activity", ms: 25 },
      { label: "13", icon: "activity", ms: 29 },
      { label: "15", icon: "gauge", ms: 26 },
      { label: "17", icon: "signal", ms: 21 },
      { label: "19", icon: "zap", ms: 23 }
    ];

    export default {
      async scheduled() {
        sample++;
        $metrix.invalidate();
      },
      render(env) {
        const name = (env.inputs && env.inputs.region) || "Tokyo-01";
        const delta = sample % 3 === 0 ? 0 : (sample % 3 === 1 ? -2 : 3);
        const curMs = 24 + delta;
        const refresh = () => { sample++; lastError = ""; $metrix.invalidate(); };

        return {
          tray: {
            title: curMs + "ms",
            icon: "lucide:gauge",
            tooltip: name + " · Optimal (" + curMs + "ms)",
            menu: [
              { label: "Refresh", onClick: refresh },
              { label: "Copy Summary", onClick: () => $metrix.copy(name + " P50 14ms / cur " + curMs + "ms") }
            ]
          },
          panel: h("Column", {
            preferredWidth: 300,
            padding: [10, 16, 16, 16],
            gap: 14,
            safeArea: true,
            shortcuts: { "CmdOrCtrl+R": refresh }
          },
            $ui.topBar({
              icon: "map-pin",
              title: name,
              subtitle: "Global Edge",
              loading: false,
              externalURL: "https://example.com",
              onRefresh: refresh
            }),
            $ui.staleError(lastError),
            $ui.hero({
              value: String(curMs),
              unit: "ms",
              label: "Optimal",
              secondary: "P50 14ms / P99 42ms",
              icon: "activity",
              iconColor: "accent",
              metrics: [
                { icon: "shield-check", value: "0.0%", tooltip: "Packet loss" },
                { icon: "radio", value: "2 ms", tooltip: "Jitter" },
                { icon: "globe", value: "9 ms", tooltip: "DNS lookup" },
                { icon: "lock", value: "18 ms", tooltip: "TLS handshake" }
              ]
            }),
            $ui.divider(),
            h("Column", { fillWidth: true, gap: 2 },
              h("Row", { fillWidth: true },
                HOURLY.map(slot =>
                  h("Column", { grow: 1, basis: 0, alignItems: "center", gap: 5 },
                    h("Text", { fontSize: 11, textColor: "textMuted", fontFeatures: "tnum", singleLine: true }, slot.label),
                    h("Icon", { name: slot.icon, fontSize: 16, width: 16, height: 16, shrink: 0, textColor: "accent" }),
                    h("Text", { fontSize: 13, fontWeight: 600, fontFeatures: "tnum", singleLine: true }, String(slot.ms + delta))
                  )
                )
              ),
              h("Chart", {
                type: "area",
                height: 30,
                fillWidth: true,
                curve: "monotone",
                strokeWidth: 2,
                xDomain: [-0.5, 6.5],
                series: [{
                  name: "Latency",
                  color: "accent",
                  fill: { from: 0.16, to: 0 },
                  points: HOURLY.map((s, idx) => ({ x: idx, y: s.ms + delta }))
                }]
              })
            ),
            $ui.divider(),
            h("Column", { fillWidth: true, gap: 2 },
              REGIONS.map((r, i) =>
                $ui.rangeRow({
                  key: r.id,
                  label: r.label,
                  labelWidth: 46,
                  active: i === 0,
                  icon: r.icon,
                  iconColor: i === 0 ? "accent" : "textMuted",
                  low: r.lo,
                  high: r.hi,
                  min: 10,
                  max: 60,
                  lowLabel: r.lo + "ms",
                  highLabel: r.hi + "ms",
                  valWidth: 36,
                  color: "accent",
                  toColor: "cyan"
                })
              )
            )
          )
        };
      }
    };
  ]]></script>
</bar>`,
	},
	{
		ID:               "bento-glass",
		Name:             "Bento Glass Dashboard",
		Style:            "Frameless Glass Bento",
		Category:         "system",
		Icon:             "activity",
		Description:      "Edge telemetry with Segmented range switcher, 44px light Hero numeral, monotone reference trend, and borderless 2x2 KPI tiles.",
		RecommendedWidth: 300,
		Tags:             []string{"glass", "bento", "chart", "segmented", "metrics"},
		Features:         []string{"material=\"glass\"", "$ui.topBar", "$ui.hero", "Segmented", "$ui.statGrid", "Chart(area)"},
		DesignNotes:      "Uses frameless translucent layering on VibrancyGlass: light 44px Hero paired with a smooth area curve above a soft borderless 2x2 Bento statGrid.",
		Source: `<bar protocol-version="0"
     version="1.0.0"
     id="bento-glass"
     title="Edge Pulse"
     category="system"
     icon="activity"
     description="Frameless glass telemetry with segmented range tabs, 44px light Hero metric, smooth trend curve, and 2x2 stat grid."
     refresh="1m"
     material="glass">
  <inputs>
    <input name="nodeName" type="string" label="Cluster" default="Tokyo-Edge" />
  </inputs>
  <script><![CDATA[
    let range = $metrix.storage.get("range") || "1H";
    let tick = 0;

    const SERIES = {
      "1H": { p95: "24", unit: "ms", delta: "-6% faster", tone: "success", uptime: "99.98%", req: "1,420/s", errRate: "0.02%", cache: "94.1%", pts: [29, 27, 28, 25, 24, 26, 23, 24, 22, 24] },
      "24H": { p95: "31", unit: "ms", delta: "+2% load", tone: "accent", uptime: "99.95%", req: "1,180/s", errRate: "0.04%", cache: "91.8%", pts: [28, 30, 34, 33, 29, 27, 32, 35, 30, 31] },
      "7D": { p95: "28", unit: "ms", delta: "Stable", tone: "success", uptime: "99.97%", req: "1,090/s", errRate: "0.03%", cache: "92.6%", pts: [31, 29, 28, 30, 27, 26, 29, 28, 27, 28] }
    };

    export default {
      async scheduled() {
        tick++;
        $metrix.invalidate();
      },
      render(env) {
        const cur = SERIES[range] || SERIES["1H"];
        const cluster = (env.inputs && env.inputs.nodeName) || "Tokyo-Edge";
        const refresh = () => { tick++; $metrix.invalidate(); };

        return {
          tray: {
            title: cur.p95 + "ms",
            icon: "lucide:activity",
            tooltip: cluster + " P95 " + cur.p95 + "ms",
            menu: [
              { label: "Refresh Now", onClick: refresh },
              { label: "Copy Summary", onClick: () => $metrix.copy(cluster + " P95: " + cur.p95 + "ms (" + cur.uptime + ")") }
            ]
          },
          panel: h("Column", {
            preferredWidth: 300,
            padding: [10, 16, 16, 16],
            gap: 12,
            safeArea: true,
            shortcuts: { "CmdOrCtrl+R": refresh }
          },
            $ui.topBar({
              icon: "activity",
              iconColor: "accent",
              title: cluster,
              subtitle: range,
              badge: "Healthy",
              badgeTone: "success",
              onRefresh: refresh
            }),
            h("Segmented", {
              value: range,
              options: [
                { value: "1H", label: "1H" },
                { value: "24H", label: "24H" },
                { value: "7D", label: "7D" }
              ],
              onChange: v => {
                range = v;
                $metrix.storage.set("range", v);
                $metrix.invalidate();
              }
            }),
            $ui.hero({
              fontSize: 44,
              letterSpacing: -1.5,
              value: cur.p95,
              unit: cur.unit,
              label: "P95 Latency",
              secondary: "#" + tick,
              right: $ui.badge(cur.delta, cur.tone, { dot: true })
            }),
            h("Chart", {
              type: "area",
              height: 40,
              curve: "monotone",
              strokeWidth: 2,
              referenceLine: { value: 30, color: "warning" },
              series: [{
                name: "Latency",
                color: "accent",
                fill: { from: 0.20, to: 0 },
                points: cur.pts.map((y, x) => ({ x, y }))
              }]
            }),
            $ui.divider(),
            $ui.statGrid([
              { label: "Uptime", value: cur.uptime, tone: "success", dot: "success" },
              { label: "Throughput", value: cur.req, tone: "accent" },
              { label: "Error Rate", value: cur.errRate, tone: "text" },
              { label: "Cache Hit", value: cur.cache, tone: "cyan" }
            ], 2),
            $ui.footer("Live glass telemetry",
              h("Button", { variant: "ghost", onClick: () => $metrix.copy(cluster + " " + cur.p95 + "ms") }, "Copy")
            )
          )
        };
      }
    };
  ]]></script>
</bar>`,
	},
	{
		ID:               "quota-ring",
		Name:             "Cookie Quota Ring",
		Style:            "Frameless Session Ring",
		Category:         "developer",
		Icon:             "key",
		Description:      "CookieJar-authenticated account quota panel with frameless 76px gradient Ring, crisp kv breakdown, and stale-while-error banner.",
		RecommendedWidth: 300,
		Tags:             []string{"cookie", "ring", "quota", "storage", "session"},
		Features:         []string{"CookieJar", "$metrix.cookies", "$metrix.storage", "Ring(gradient)", "Progress(gradient)"},
		DesignNotes:      "Frameless glass grouping puts full focus on the 76px gradient Ring and tabular usage rows, preserving cached data behind $ui.staleError on refresh failure.",
		Source: `<bar protocol-version="0"
     version="1.0.0"
     id="quota-ring"
     title="Account Quota"
     category="developer"
     icon="key"
     description="Cookie-session quota monitor rendered directly on glass with circular gradient ring and stale-while-error protection."
     refresh="5m"
     material="glass">
  <permissions>
    <network origin="https://api.example.com" />
  </permissions>
  <inputs>
    <input name="endpoint" type="string" label="Quota API" default="https://api.example.com/v1/quota" />
  </inputs>
  <script><![CDATA[
    let quota = null;
    let errMsg = "";
    const cachedRaw = $metrix.storage.get("lastQuota");
    if (cachedRaw) {
      try { quota = JSON.parse(cachedRaw); } catch (_) {}
    }

    async function load(env) {
      const endpoint = (env.inputs && env.inputs.endpoint) || "https://api.example.com/v1/quota";
      const cookies = $metrix.cookies.get(endpoint);
      if (Object.keys(cookies).length === 0) {
        errMsg = "";
        $metrix.invalidate();
        return;
      }
      try {
        const resp = await fetch(endpoint);
        if (!resp.ok) throw new Error("HTTP " + resp.status);
        quota = await resp.json();
        errMsg = "";
        $metrix.storage.set("lastQuota", JSON.stringify(quota));
      } catch (e) {
        errMsg = String((e && e.message) || e);
      }
      $metrix.invalidate();
    }

    export default {
      async scheduled(env) { await load(env); },
      render(env) {
        const endpoint = (env.inputs && env.inputs.endpoint) || "https://api.example.com/v1/quota";
        const cookieCount = Object.keys($metrix.cookies.get(endpoint)).length;
        const live = cookieCount > 0 && quota !== null;
        const d = quota || { plan: "Pro Session", used: 68, limit: 100, fastUsed: 312, fastLimit: 500, resetIn: "6d 14h" };
        const pct = Math.min(100, Math.max(0, Math.round((d.used / Math.max(d.limit || 1, 1)) * 100)));
        const tone = pct >= 90 ? "danger" : (pct >= 75 ? "warning" : "accent");
        const statusText = errMsg ? "Error" : (live ? "Live" : (cookieCount > 0 ? "Ready" : "Preview"));
        const statusTone = errMsg ? "danger" : (live ? "success" : "warning");
        const refresh = () => { void load(env); };

        return {
          tray: {
            title: pct + "%",
            icon: "lucide:key",
            tooltip: "Account Quota: " + pct + "% (" + statusText + ")",
            menu: [
              { label: "Refresh", onClick: refresh },
              { label: "Copy Usage", onClick: () => $metrix.copy("Quota " + pct + "% (" + d.used + "/" + d.limit + ")") }
            ]
          },
          panel: h("Column", {
            preferredWidth: 300,
            padding: [10, 16, 16, 16],
            gap: 12,
            safeArea: true,
            shortcuts: { "CmdOrCtrl+R": refresh }
          },
            $ui.topBar({
              icon: "key",
              iconColor: "accent",
              title: "Account Quota",
              badge: live ? "" : statusText,
              badgeTone: statusTone,
              onRefresh: refresh
            }),
            $ui.staleError(errMsg),
            h("Row", { fillWidth: true, alignItems: "center", gap: 14 },
              h("Ring", {
                value: pct,
                max: 100,
                size: 76,
                thickness: 7,
                style: "gradient",
                color: tone,
                toColor: "cyan",
                label: pct + "%",
                sublabel: "USED"
              }),
              h("Column", { grow: 1, minWidth: 0, gap: 6 },
                $ui.kv("Monthly Credits", d.used + " / " + d.limit),
                $ui.kv("Fast Requests", (d.fastUsed || 0) + " / " + (d.fastLimit || 500)),
                $ui.kv("Resets In", d.resetIn || "--", { valueColor: "accent" })
              )
            ),
            $ui.divider(),
            h("Column", { fillWidth: true, gap: 6 },
              h("Row", { fillWidth: true, alignItems: "center" },
                h("Text", { fontSize: 11, textColor: "textMuted", grow: 1 }, "Fast tier allocation"),
                h("Text", { fontSize: 11, fontWeight: 600, fontFeatures: "tnum" }, (d.fastUsed || 0) + " / " + (d.fastLimit || 500))
              ),
              h("Progress", {
                value: d.fastUsed || pct,
                max: d.fastLimit || 100,
                height: 5,
                style: "gradient",
                color: "accent",
                toColor: "cyan"
              })
            ),
            cookieCount === 0 && !errMsg
              ? h("Text", { fontSize: 11, textColor: "textMuted", wrap: true }, "Import cookies in Manage Cookies or via MCP bar_cookies_set to fetch live data.")
              : null,
            $ui.footer((d.plan || "Pro Session") + (live ? "" : " · Preview"),
              h("Button", { variant: "ghost", onClick: () => $metrix.copy("Quota " + pct + "%") }, "Copy"),
              h("Button", { variant: "soft", onClick: refresh }, "Refresh")
            )
          )
        };
      }
    };
  ]]></script>
</bar>`,
	},
	{
		ID:               "utility-switchboard",
		Name:             "Utility Switchboard",
		Style:            "Frameless Control Center",
		Category:         "productivity",
		Icon:             "sliders",
		Description:      "First-party macOS Control Center feel: frameless Switch rows separated by 1px dividers, Segmented view tabs, and plain-icon ListRow actions.",
		RecommendedWidth: 300,
		Tags:             []string{"switch", "listrow", "segmented", "utility", "shortcuts"},
		Features:         []string{"Switch", "ListRow(iconPlain)", "Segmented", "shortcuts", "$metrix.storage"},
		DesignNotes:      "Eliminates card borders so native Switch toggles and plain-icon ListRows float directly on popover translucency with crisp 1px hairline dividers.",
		Source: `<bar protocol-version="0"
     version="1.0.0"
     id="utility-switchboard"
     title="Quick Controls"
     category="productivity"
     icon="sliders"
     description="Frameless Control Center style popover with persistent Switch toggles, Segmented tabs, and plain-icon ListRow actions."
     refresh="15m"
     material="popover">
  <script><![CDATA[
    let tab = "controls";
    let statusMsg = "All idle";
    const readBool = (k, defVal) => {
      const v = $metrix.storage.get(k);
      return v === null ? defVal : v === "1";
    };
    let stayAwake = readBool("stayAwake", false);
    let focusDND = readBool("focusDND", true);
    let autoProxy = readBool("autoProxy", true);

    function setFlag(key, val, label) {
      $metrix.storage.set(key, val ? "1" : "0");
      statusMsg = label + (val ? " on" : " off");
      $metrix.invalidate();
    }

    export default {
      render() {
        const activeCount = (stayAwake ? 1 : 0) + (focusDND ? 1 : 0) + (autoProxy ? 1 : 0);
        const resetAll = () => {
          stayAwake = false;
          focusDND = false;
          autoProxy = false;
          setFlag("stayAwake", false, "Stay Awake");
          setFlag("focusDND", false, "Focus");
          setFlag("autoProxy", false, "Proxy");
          statusMsg = "All reset";
          $metrix.invalidate();
        };

        return {
          tray: {
            title: activeCount + " ON",
            icon: "lucide:sliders",
            tooltip: "Quick Controls (" + activeCount + " active)",
            menu: [
              { label: stayAwake ? "Disable Stay Awake" : "Enable Stay Awake", onClick: () => { stayAwake = !stayAwake; setFlag("stayAwake", stayAwake, "Stay Awake"); } },
              { separator: true },
              { label: "Reset Toggles", onClick: resetAll }
            ]
          },
          panel: h("Column", {
            preferredWidth: 300,
            padding: [10, 16, 16, 16],
            gap: 12,
            safeArea: true,
            shortcuts: { "CmdOrCtrl+R": resetAll }
          },
            $ui.topBar({
              icon: "sliders",
              iconColor: "accent",
              title: "Quick Controls",
              subtitle: statusMsg,
              badge: activeCount + " ON",
              badgeTone: activeCount > 0 ? "accent" : "neutral"
            }),
            h("Segmented", {
              value: tab,
              options: [
                { value: "controls", label: "Toggles" },
                { value: "actions", label: "Quick Actions" }
              ],
              onChange: v => { tab = v; $metrix.invalidate(); }
            }),
            tab === "controls"
              ? h("Column", { fillWidth: true, gap: 10 },
                  h("Row", { fillWidth: true, alignItems: "center", gap: 8 },
                    h("Column", { grow: 1, minWidth: 0, gap: 1 },
                      h("Text", { fontSize: 13, fontWeight: 550 }, "Stay Awake"),
                      h("Text", { fontSize: 11, textColor: "textMuted" }, "Prevent display sleep during builds")
                    ),
                    h("Switch", { checked: stayAwake, color: "warning", onChange: v => { stayAwake = v; setFlag("stayAwake", v, "Stay Awake"); } })
                  ),
                  $ui.divider(),
                  h("Row", { fillWidth: true, alignItems: "center", gap: 8 },
                    h("Column", { grow: 1, minWidth: 0, gap: 1 },
                      h("Text", { fontSize: 13, fontWeight: 550 }, "Focus Mode"),
                      h("Text", { fontSize: 11, textColor: "textMuted" }, "Silence non-urgent notifications")
                    ),
                    h("Switch", { checked: focusDND, color: "purple", onChange: v => { focusDND = v; setFlag("focusDND", v, "Focus Mode"); } })
                  ),
                  $ui.divider(),
                  h("Row", { fillWidth: true, alignItems: "center", gap: 8 },
                    h("Column", { grow: 1, minWidth: 0, gap: 1 },
                      h("Text", { fontSize: 13, fontWeight: 550 }, "Smart Routing"),
                      h("Text", { fontSize: 11, textColor: "textMuted" }, "Auto-route split traffic rules")
                    ),
                    h("Switch", { checked: autoProxy, color: "success", onChange: v => { autoProxy = v; setFlag("autoProxy", v, "Smart Routing"); } })
                  )
                )
              : h("Column", { fillWidth: true, gap: 4 },
                  h("ListRow", {
                    icon: "terminal",
                    iconPlain: true,
                    iconColor: "accent",
                    title: "Copy Diagnostic Env",
                    subtitle: "Includes active profile flags",
                    shortcut: "Cmd+C",
                    chevron: true,
                    onClick: () => { $metrix.copy("stayAwake=" + stayAwake + " focus=" + focusDND + " proxy=" + autoProxy); statusMsg = "Copied env"; $metrix.invalidate(); }
                  }),
                  $ui.divider(),
                  h("ListRow", {
                    icon: "rotate-ccw",
                    iconPlain: true,
                    iconColor: "warning",
                    title: "Reset All Toggles",
                    subtitle: "Restore default switches",
                    shortcut: "Cmd+R",
                    badge: "Reset",
                    badgeTone: "warning",
                    onClick: resetAll
                  })
                ),
            $ui.divider(),
            $ui.footer("Cmd+R to reset toggles")
          )
        };
      }
    };
  ]]></script>
</bar>`,
	},
	{
		ID:               "ops-feed",
		Name:             "Ops Status Feed",
		Style:            "Calm Glass Incident List",
		Category:         "developer",
		Icon:             "server",
		Description:      "Compact glass endpoint feed with 26px top bar, Segmented severity filter, pulsing StatusDot rows, and tabular latencies.",
		RecommendedWidth: 300,
		Tags:             []string{"ops", "status", "listrow", "scroll", "dot"},
		Features:         []string{"$ui.topBar", "ListRow", "StatusDot(pulse)", "Segmented", "Scroll"},
		DesignNotes:      "Streamlines status scanning with clean single-line endpoints, right-aligned tnum latencies, pulsing StatusDot indicators, and 1px hairline separation.",
		Source: `<bar protocol-version="0"
     version="1.0.0"
     id="ops-feed"
     title="Service Health"
     category="developer"
     icon="server"
     description="Service uptime and latency monitor with Segmented severity filter and pulsing StatusDot rows on glass."
     refresh="1m"
     material="glass">
  <script><![CDATA[
    let filter = "all";
    let copied = "";
    const SERVICES = [
      { id: "edge-us", name: "US-West Ingress", host: "usw.api.internal", ms: "18ms", status: "up", tone: "success" },
      { id: "auth", name: "OAuth Gateway", host: "auth.internal", ms: "142ms", status: "warn", tone: "warning", badge: "Slow" },
      { id: "billing", name: "Billing Hooks", host: "hooks.internal", ms: "timeout", status: "warn", tone: "danger", badge: "Alert" },
      { id: "cache", name: "Redis Cluster", host: "kv-01.internal", ms: "3ms", status: "up", tone: "success" },
      { id: "cdn", name: "Static Assets Edge", host: "cdn.internal", ms: "11ms", status: "up", tone: "success" }
    ];

    export default {
      render() {
        const alertCount = SERVICES.filter(s => s.status !== "up").length;
        const rows = SERVICES.filter(s => filter === "all" || (filter === "warn" ? s.status !== "up" : s.status === "up"));

        return {
          tray: {
            title: alertCount > 0 ? alertCount + " WARN" : "ALL OK",
            icon: "lucide:server",
            tooltip: SERVICES.length + " services · " + alertCount + " warnings",
            menu: [{ label: "Show Warnings", onClick: () => { filter = "warn"; $metrix.invalidate(); } }]
          },
          panel: h("Column", {
            preferredWidth: 300,
            preferredHeight: 372,
            padding: [10, 16, 16, 16],
            gap: 12,
            safeArea: true
          },
            $ui.topBar({
              icon: "server",
              iconColor: "accent",
              title: "Service Health",
              subtitle: copied,
              badge: alertCount > 0 ? (alertCount + " Alerts") : "OK",
              badgeTone: alertCount > 0 ? "danger" : "success",
              onRefresh: () => { copied = "Refreshed"; $metrix.invalidate(); }
            }),
            h("Segmented", {
              value: filter,
              options: [
                { value: "all", label: "All (" + SERVICES.length + ")" },
                { value: "warn", label: "Alerts (" + alertCount + ")" },
                { value: "up", label: "Healthy" }
              ],
              onChange: v => { filter = v; $metrix.invalidate(); }
            }),
            $ui.divider(),
            h("Scroll", { grow: 1, minHeight: 0, fillWidth: true },
              h("Column", { fillWidth: true, gap: 2 },
                rows.map(s => h("ListRow", {
                  dot: s.tone,
                  pulse: s.status !== "up",
                  title: s.name,
                  subtitle: s.host,
                  value: s.ms,
                  valueColor: s.status === "up" ? "textMuted" : s.tone,
                  badge: s.badge || "",
                  badgeTone: s.tone,
                  chevron: true,
                  active: copied === ("Copied " + s.name),
                  onClick: () => {
                    $metrix.copy(s.name + " (" + s.host + "): " + s.ms);
                    copied = "Copied " + s.name;
                    $metrix.invalidate();
                  }
                }))
              )
            ),
            $ui.footer(SERVICES.length + " endpoints · Tap row to copy",
              filter !== "all" ? h("Button", { variant: "ghost", onClick: () => { filter = "all"; $metrix.invalidate(); } }, "Show All") : null
            )
          )
        };
      }
    };
  ]]></script>
</bar>`,
	},
	{
		ID:               "matrix-minimal",
		Name:             "Matrix LED Minimal",
		Style:            "Obsidian HUD & LED Matrix",
		Category:         "system",
		Icon:             "cpu",
		Description:      "Glowing LED dot-matrix Ring and Progress bars floating directly on obsidian HUD translucency with monospace telemetry and sparkline.",
		RecommendedWidth: 300,
		Tags:             []string{"matrix", "led", "minimal", "hud", "sparkline"},
		Features:         []string{"material=\"hud\"", "Ring(matrix)", "Progress(matrix)", "Chart(sparkline)", "font=\"mono\""},
		DesignNotes:      "Removes nested boxes so the glowing obsidian LED matrix Ring, matrix Progress bar, and cyan sparkline pop cleanly on HUD vibrancy.",
		Source: `<bar protocol-version="0"
     version="1.0.0"
     id="matrix-minimal"
     title="Compute Matrix"
     category="system"
     icon="cpu"
     description="Frameless HUD obsidian LED dot-matrix Ring and Progress meters with monospace telemetry and sparkline."
     refresh="30s"
     material="hud">
  <script><![CDATA[
    let gpu = 72;
    let mem = 64;
    let history = [58, 62, 69, 71, 66, 74, 72];

    const stepSample = () => {
      gpu = 45 + ((gpu * 7 + 19) % 48);
      mem = 50 + ((mem * 5 + 11) % 40);
      history = history.slice(1).concat([gpu]);
      $metrix.invalidate();
    };

    export default {
      render() {
        return {
          tray: {
            title: "GPU " + gpu + "%",
            icon: "lucide:cpu",
            tooltip: "Compute Load " + gpu + "% · VRAM " + mem + "%",
            menu: [{ label: "Sample Now", onClick: stepSample }]
          },
          panel: h("Column", {
            preferredWidth: 300,
            padding: [10, 16, 16, 16],
            gap: 12,
            safeArea: true,
            shortcuts: { "CmdOrCtrl+R": stepSample }
          },
            $ui.topBar({
              icon: "cpu",
              iconColor: "cyan",
              title: "Compute Matrix",
              subtitle: "MPS",
              badge: gpu > 80 ? "High" : "Nominal",
              badgeTone: gpu > 80 ? "warning" : "cyan",
              onRefresh: stepSample
            }),
            h("Row", { fillWidth: true, alignItems: "center", gap: 14 },
              h("Ring", {
                value: gpu,
                max: 100,
                size: 76,
                style: "matrix",
                color: "cyan",
                label: gpu + "%",
                sublabel: "CORE"
              }),
              h("Column", { grow: 1, minWidth: 0, gap: 6 },
                h("Text", { fontSize: 10.5, font: "mono", textColor: "textMuted" }, "TELEMETRY"),
                $ui.kv("VRAM", mem + " / 100 GB", { mono: true, valueColor: "accent" }),
                $ui.kv("Thermal", "54 °C", { mono: true, valueColor: "success" }),
                $ui.kv("Power", "38.4 W", { mono: true })
              )
            ),
            h("Progress", {
              value: mem,
              max: 100,
              height: 14,
              style: "matrix",
              color: "accent"
            }),
            $ui.divider(),
            h("Column", { fillWidth: true, gap: 4 },
              h("Row", { fillWidth: true, alignItems: "center" },
                h("Text", { fontSize: 10, font: "mono", textColor: "textMuted", grow: 1 }, "LOAD SPARKLINE"),
                h("Text", { fontSize: 10, font: "mono", fontFeatures: "tnum", textColor: "cyan" }, gpu + "%")
              ),
              h("Chart", {
                type: "sparkline",
                height: 32,
                curve: "monotone",
                strokeWidth: 2,
                series: [{
                  name: "GPU",
                  color: "cyan",
                  fill: { from: 0.24, to: 0 },
                  points: history.map((y, x) => ({ x, y }))
                }]
              })
            ),
            $ui.footer("Obsidian LED HUD",
              h("Button", { variant: "soft", onClick: stepSample }, "Sample")
            )
          )
        };
      }
    };
  ]]></script>
</bar>`,
	},
}
