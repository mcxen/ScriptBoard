package mcpsvc

import "strings"

// BarGuide returns the complete reference for developing visually polished .bar
// XML menubar components (design tokens, typography, vibrancy materials, VDOM
// widgets, $ui layout helpers, persistent CookieJar/KV storage, and the MCP
// template + preview workflow).
func BarGuide() string {
	return strings.ReplaceAll(barGuideTemplate, "§", "`")
}

const barGuideTemplate = `# ScriptBoard .bar Menubar Component Guide

A §.bar§ component is an XML bundle containing permissions, user-configurable inputs, and an ES module that runs in an isolated §moejs§ event loop with §fetch§, timers, a per-component persistent §CookieJar§, persistent KV storage, built-in §h§/§$ui§ design helpers, and native Light/Dark/Glass rendering.

---

## 1. Quick-Start Design Workflow (MCP Visual Loop)

1. **Pick a reference style first**: call §bar_templates§ and §bar_template_get§ to inspect the 6 built-in benchmark styles (§native-glass§, §bento-glass§, §quota-ring§, §utility-switchboard§, §ops-feed§, §matrix-minimal§).
2. **Validate structure & compilation**: call §bar_check§ before saving.
3. **Test data, Cookies & interactions in sandbox**: call §bar_run§ (with §mockFetch§, §curl§/§cookieHeader§, §useSavedState§, §actionID§ + §actionArg§ for switches/tabs).
4. **Render Light & Dark PNG previews + Visual Audit**: call §bar_preview§ to render real headless screenshots (§lightPath§ / §darkPath§) and receive automatic contrast/overflow/layout audit warnings.
5. **Save & hot-reload live**: call §bar_save§; the running app automatically restarts updated components and reflects §material§ and placements immediately.

---

## 1.5. Design Standard (Apple Native / Weather Bar Style)

ScriptBoard's flagship visual aesthetic follows the built-in Weather Bar (§BuiltinWeatherBar§): **frameless glass layering**, **light-weight oversized numerals**, and **restraint with borders**.
- **Frameless Glass Layering**: Set §material="glass"§ and let the OS translucency do the work. Prefer root §Column({ preferredWidth: 300, padding: [10, 16, 16, 16], gap: 14, safeArea: true })§ separated into 3–4 calm bands by 1px §$ui.divider()§ lines instead of nesting multiple bordered §$ui.card§ containers.
- **26px Minimal Top Bar**: Use §$ui.topBar({ icon, title, subtitle, loading, externalURL, onRefresh })§ for a whisper-quiet 26px header with hoverable 26×26 icon buttons and a spinning refresh icon while §loading: true§.
- **Oversized Light Hero Typography**: Use §$ui.hero({ value, unit, label, secondary, icon, metrics })§ (§fontSize: 44–52§, §fontWeight: 300§, §letterSpacing: -1.5..-2§, §fontFeatures: "tnum"§, §contentTransition: { numeric: true }§) paired with a crisp right-side wireframe icon or gradient Ring and one horizontal line of 12px §tnum§ micro-metrics.
- **Range Bars & Stale-While-Error**: Use §$ui.rangeRow(...)§ for low/high distributions (latency bounds, temperatures, P50–P99) and §$ui.staleError(lastError)§ to surface refresh failures without wiping cached data.

---

## 2. XML Structure & §material§

§§§xml
<bar protocol-version="0"
     version="1.0.0"
     id="my-service-bar"
     title="My Service"
     category="developer"
     icon="activity"
     description="Monitors account status with session cookies and glass Bento cards."
     homepage="https://example.com"
     refresh="5m"
     material="glass">
  <permissions>
    <network origin="https://api.example.com" />
    <network origin="https://*.example.com" />
    <command id="net-ping" />
  </permissions>
  <inputs>
    <input name="endpoint" type="string" label="Endpoint" default="https://api.example.com/me" required="true" />
    <input name="mode" type="select" label="Mode" default="compact">
      <option value="compact" label="Compact" />
      <option value="full" label="Full" />
    </input>
    <input name="limit" type="number" label="Limit" default="10" />
    <input name="autoRefresh" type="boolean" label="Auto refresh" default="true" />
    <input name="apiToken" type="password" label="API Token" help="Optional bearer token (redacted by MCP)" />
  </inputs>
  <script><![CDATA[
    export default {
      async scheduled(env) { ... },
      render(env) { return { tray: {...}, panel: {...} }; }
    };
  ]]></script>
</bar>
§§§

### Manifest Attributes & Tags
- §id§ (required): unique lowercase identifier (no §/§, §\§, or §..§).
- §title§, §version§, §category§ (§weather§, §finance§, §system§, §developer§, §productivity§), §icon§ (Lucide name from §icon_catalog§), §description§, §homepage§.
- §refresh§: Go duration (§30s§, §5m§, §1h§) or integer seconds; minimum §10s§, default §15m§.
- §material§: floating window vibrancy backdrop — §"glass"§ (macOS Liquid Glass / translucent), §"popover"§ (default macOS popover translucency), §"hud"§ (dark HUD obsidian material), or §"solid"§ (opaque panel surface).
- §<permissions>§:
  - §<network origin="https://api.example.com" />§ — exact scheme + host + effective port.
  - Wildcard subdomains are supported: §<network origin="https://*.example.com" />§ matches §app.example.com§ and §v2.api.example.com§ (not the bare apex §example.com§).
- §<inputs>§: §type§ can be §string§, §select§ (with §<option value="..." label="..."/>§), §number§, §boolean§, or §password§ (masked in UI and redacted in MCP read outputs by default).

---

## 3. Script Lifecycle & §env§

Export default object:
- §async scheduled(env)§ (optional): runs on startup, §refresh§ timer, user refresh, and input/cookie updates. Call §$metrix.invalidate()§ when state updates.
- §render(env)§ (required): synchronous function returning §{ tray, panel }§.
- §env§ fields:
  - §env.inputs§: typed map of resolved §<inputs>§ (§number§ parsed to number, §boolean§ to bool, others string).
  - §env.lang§: §"zh"§ or §"en"§.
  - §env.theme§: §"dark"§ or §"light"§.
  - §env.vibrancy§: boolean indicating whether desktop translucent material is active.
  - §env.platform§: §"darwin"§, §"windows"§, or §"linux"§.

---

## 4. Built-in UI Helpers: §h§ & §$ui§

§globalThis.h§ and §globalThis.$ui§ are pre-installed (you no longer need to define §h§ boilerplate; local §const h = ...§ still works cleanly if defined):
- §h(type, props, ...children)§ — automatically flattens nested arrays and skips §null§/§undefined§/§false§.
- §$ui.header({ icon, iconColor, iconBg, title, subtitle, badge, badgeTone, loading, onRefresh, actions })§ — standard header with tinted icon pill, status badge, and hoverable (optionally spinning) refresh button.
- §$ui.topBar({ icon, iconColor, title, subtitle, badge, badgeTone, externalURL, loading, onRefresh, actions })§ — Weather-bar 26px borderless top bar with muted icon, title/subtitle, optional external link, and spinning refresh button.
- §$ui.hero({ value, unit, label, secondary, icon, iconColor, iconSize, right, metrics, fontSize, fontWeight, letterSpacing, down })§ — Weather-bar light-weight 48px §tnum§ Hero with §contentTransition: { numeric: true }§, right icon/widget slot, and compact 12px §metrics§ strip.
- §$ui.rangeBar({ min, max, low, high, color, toColor, height, trackColor })§ & §$ui.rangeRow({ label, icon, tag, low, high, lowLabel, highLabel, min, max, color, toColor, active })§ — 4px min–max distribution bars.
- §$ui.staleError(message)§ — compact non-destructive top error bar preserving cached content.
- §$ui.iconButton({ icon, svg, tooltip, onClick, spin, color })§ — 26×26 hover/pressed icon button with optional spin animation.
- §$ui.card(props, ...children)§ — container defaulting to §background: "surfaceCard"§, 1px hairline §"border"§, §radius: 12§, §padding: 12§, §gap: 8§ (use sparingly on glass).
- §$ui.section(title, ...children)§ — muted 11px tracked section header + content column.
- §$ui.metric({ label, value, unit, delta, deltaTone, tone, mono, fontSize, fontWeight, down, sub })§ — tabular-numerals (§tnum§) KPI display with §contentTransition: { numeric: true }§ (auto-light 300 weight at §fontSize >= 36§).
- §$ui.statGrid(items, cols = 2, { bordered })§ — frameless §surfaceSubtle§ multi-column Bento grid (pass §{ bordered: true }§ to restore card borders); each item supports §{ label, value, unit, sub, tone, dot, pulse, icon, mono }§.
- §$ui.kv(label, value, { valueColor, badge, badgeTone, mono })§ — aligned key-value row with §tnum§ right value.
- §$ui.badge(text, tone, props)§ (supports §dot: true, dotAnimation: "pulse"|"ripple"|"dots"|"wave"|"orbit"|"blink"|"matrix"§) & §$ui.dot(color, pulseOrAnimOrOpts, size)§ & §$ui.dotAnim({ color, animation, count, size, speed })§
- §$ui.commandDot(resolvedField, opts?)§, §$ui.commandValue(resolvedField, opts?)§ & §$ui.commandFieldStrip(resolvedCmd, opts?)§ — generic host-styled return value & dot animation renderers driven by §$metrix.command.resolve(id, record?)§
- §$ui.empty({ icon, title, subtitle, actionLabel, onAction })§ & §$ui.skeleton(linesOrPreset)§ (supports §"hero"§ silhouette or numeric line count)
- §$ui.footer(leftText, ...actions)§, §$ui.divider()§, §$ui.spacer()§

---

## 5. Styling Tokens & Aesthetic Conventions

### Semantic Theme Tokens (Adapt Automatically to Light / Dark / Glass)
Always prefer semantic tokens over hardcoded §#000000§ / §#FFFFFF§:
- **Surfaces**: §surface§, §surfaceCard§ (soft card on top of surface/glass), §surfaceElevated§ (raised active pill/card), §surfaceSubtle§, §surfaceHover§, §surfacePressed§
- **Text**: §text§, §textMuted§, §textFaint§
- **Semantic Accents & Soft Surfaces**: §accent§ / §accentSurface§, §success§ / §successSurface§, §warning§ / §warningSurface§, §danger§ / §dangerSurface§, §purple§ / §purpleSurface§, §cyan§ / §cyanSurface§, §neutral§ / §neutralSurface§
- **Borders**: §border§, §borderStrong§, §transparent§
- Custom CSS colors also supported: §#RGB§, §#RGBA§, §#RRGGBB§, §#RRGGBBAA§, §rgb(r,g,b)§, §rgba(r,g,b,a)§.

### Rich Styling Props
- **Typography (§Text§)**: §fontSize§, §fontWeight§, §letterSpacing§, §font: "mono" | "sans"§, §fontFeatures: "tnum"§ (use §tnum§ for all numbers/timers/percentages!), §contentTransition: { numeric: true, down?: bool, morph?: bool }§, §textColor§, §textAlign§, §textShadow: { x, y, blur, color }§, §padding§, §singleLine§, §wrap§, §opacity§.
- **Containers (§Column§, §Row§, §Box§, §Scroll§)**: §preferredWidth§ (typically 300), §preferredHeight§ (set if using §Scroll§ with §grow: 1, minHeight: 0§), §padding§ (§n§, §[v,h]§, §[t,r,b,l]§), §safeArea: true§ (adds OS notch/bar insets), §gap§, §radius§, §wrap§, §clip§, §opacity§, §borderWidth§ (scalar or §[t,r,b,l]§), §borderColor§, §borderStyle: "solid" | "dashed"§, §shadow: "sm" | "md" | { x, y, blur, spread, color }§, §gradient: { from, to, angle, stops, oklab }§, §hover§, §pressed§, §shortcuts: { "CmdOrCtrl+R": fn }§.

---

## 6. Supported VDOM Components

- **Layout**: §Column§, §Row§, §Box§, §Scroll§, §Spacer§, §Divider§.
- **Text & Media**:
  - §Text§, §Icon§ (§name§ from §icon_catalog§ or inline §svg§, optional spin §keyframes§), §Image§ (§src: "data:image/png;base64,..."§, §width§, §height§, §radius§, §fit: "contain" | "cover"§).
- **Status & Visualization**:
  - §Badge§: §tone§ (§"accent"§|§"success"§|§"warning"§|§"danger"§|§"purple"§|§"cyan"§|§"neutral"§) auto-selects matched soft background + high-contrast text, §dot: true§, §dotAnimation§, §pulse§, optional §background§/§textColor§.
  - §StatusDot§: §color§, §size§ (4..16), §pulse: true§, §animation§ (§"none"§ | §"pulse"§ | §"ripple"§ | §"dots"§ | §"wave"§ | §"orbit"§ | §"blink"§ | §"matrix"§), §count§ (dots/wave/orbit/matrix count), §speed§ (0.25..4.0), §animate§ (default true).
  - §Progress§: §value§, §max§ (100), §height§, §style§ (§"solid"§ | §"gradient"§ | §"segmented"§ | §"matrix"§), §color§, §toColor§, §segments§, §animate§ (matrix shimmer).
  - §Ring§: circular progress ring — §value§, §max§ (100), §size§ (default 74), §thickness§, §style§ (§"solid"§ | §"gradient"§ | §"matrix"§), §color§, §toColor§, §label§, §sublabel§.
  - §Chart§: §type§ (§"line"§ | §"area"§ | §"bar"§ | §"sparkline"§), §height§, §curve: "monotone"§, §strokeWidth§, §xDomain§, §referenceLine§ (§number§ or §{ value, color }§), §series: [{ name, color, fill: { from, to }, points: [{ x, y }] }]§.
- **Interactive Controls** (testable via MCP §bar_run§ / §bar_preview§ with §actionID§ + §actionArg§):
  - §Button§: §variant§ (§"primary"§ | §"danger"§ | §"soft"§ | §"ghost"§), §icon§, §disabled§, §onClick: () => ...§.
  - §Segmented§: §value§, §options: [{ value, label }]§, §onChange: val => ...§.
  - §Switch§: §checked§, §color§, §disabled§, §onChange: nextBool => ...§.
  - §ListRow§: native clickable menu row — §icon§, §iconPlain: true§ (omit tinted box), §iconColor§, §iconBg§, §dot§, §dotAnimation§, §pulse§, §title§, §subtitle§, §value§, §valueColor§, §badge§, §badgeTone§, §shortcut§, §chevron§, §active§, §disabled§, §onClick: () => ...§.
  - §Input§: inline single-line filter/input — §value§, §placeholder§, §onChange: txt => ...§, §onSubmit: txt => ...§.

---

## 7. Host APIs: Cookies, Storage & §$metrix§

- **Web APIs**: §fetch§, §Headers§ (§getSetCookie()§), §Request§, §Response§, §URL§, §URLSearchParams§, §ReadableStream§, §AbortController§, timers, §console§, persisted §localStorage§.
- **Automatic Persistent §CookieJar§** (§<dataDir>/bars/<id>/cookies.json§, §0600§): captures §Set-Cookie§ headers automatically and attaches cookies on subsequent requests; manageable via UI ("Manage cookies") or MCP (§bar_cookies_set/get/clear§).
- **§$metrix§**:
  - §$metrix.invalidate()§, §$metrix.openURL(url)§, §$metrix.copy(text)§, §$metrix.log(...args)§
  - §$metrix.storage§: §get(k)§, §set(k, v)§, §delete(k)§, §clear()§, §keys()§
  - §$metrix.cookies§ (permission-checked): §get(url)§, §header(url)§, §set(url, name, value, opts?)§, §delete(url, name)§, §clear(url?)§
  - §$metrix.command§ (requires §<command id="..."/>§ permission): §async run(id, params?)§ -> §Record§ (§{id, commandId, t, trigger, params, ok, exitCode, durationMs, stdout, stderr, json, extracted, error}§), §latest(id)§, §history(id, limit?)§, §style(id)§ -> custom §DisplayStyle§, §resolve(id, record?)§ -> §ResolvedCommand§ (resolves §format§ templates, §valueMap§, §fallback§, §thresholds§ & dot animations configurable via §cmd_style_set§ / §cmd_save§)
`
