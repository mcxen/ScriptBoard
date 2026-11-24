package mcpsvc

import "strings"

// BarGuide returns the complete reference for developing .bar XML menubar
// components (floating panels, tray menus, persistent CookieJar, KV storage,
// inputs, permissions, VDOM components, and the MCP test/debug workflow).
func BarGuide() string {
	return strings.ReplaceAll(barGuideTemplate, "§", "`")
}

const barGuideTemplate = `# ScriptBoard .bar Menubar Component Guide

A §.bar§ component is an XML bundle containing permissions, user-configurable inputs, and an ES module that runs in an isolated §moejs§ event loop with §fetch§, timers, a per-component persistent §CookieJar§, and persistent key-value storage. It can be pinned to the macOS/desktop menu bar (floating panel or dropdown menu), docked in the left/right/bottom sidebars, triggered by shortcut (§Alt+W§), or shown on the home page.

---

## 1. XML Structure

§§§xml
<bar protocol-version="0"
     version="1.0.0"
     id="my-service-bar"
     title="My Service"
     category="developer"
     icon="activity"
     description="Monitors account status using persistent session cookies."
     homepage="https://example.com"
     refresh="5m"
     material="glass">
  <permissions>
    <network origin="https://api.example.com" />
    <network origin="https://*.example.com" />
  </permissions>
  <inputs>
    <input name="endpoint" type="string" label="Endpoint" default="https://api.example.com/me" required="true" />
    <input name="mode" type="select" label="Mode" default="compact">
      <option value="compact" label="Compact" />
      <option value="full" label="Full" />
    </input>
    <input name="limit" type="number" label="Limit" default="10" />
    <input name="autoRefresh" type="boolean" label="Auto refresh" default="true" />
    <input name="apiToken" type="password" label="API Token" help="Optional bearer token" />
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
- §<permissions>§:
  - §<network origin="https://api.example.com" />§ — exact scheme + host + effective port.
  - Wildcard subdomains are supported: §<network origin="https://*.example.com" />§ matches §app.example.com§ and §v2.api.example.com§ (not the bare apex §example.com§).
  - Unpermitted URLs are blocked in §fetch()§ and §$metrix.cookies.*§.
- §<inputs>§: §type§ can be §string§, §select§ (with §<option value="..." label="..."/>§), §number§, §boolean§, or §password§ (masked in UI and redacted in MCP read outputs by default).

---

## 2. Script Lifecycle & §env§

The script exports a default object with:
- §async scheduled(env)§ (optional): called on startup, on §refresh§ interval, when user clicks Refresh, or when inputs/cookies update. Fetch data here, then call §$metrix.invalidate()§ to trigger a UI re-render.
- §render(env)§ (required): synchronous function returning §{ tray, panel }§.
- §env§ contains:
  - §env.inputs§: typed map of resolved §<inputs>§ (§number§ parsed to number, §boolean§ parsed to bool, others strings).
  - §env.lang§: §"zh"§ or §"en"§.
  - §env.platform§: §"darwin"§, §"windows"§, or §"linux"§.

---

## 3. Host Runtime APIs (Cookies, Storage, Web APIs, §$metrix§)

### Web APIs
- §fetch(url, init)§, §Headers§ (including §resp.headers.getSetCookie()§), §Request§, §Response§, §URL§, §URLSearchParams§, §ReadableStream§, §AbortController§
- Timers: §setTimeout§, §clearTimeout§, §setInterval§, §clearInterval§
- Console: §console.log/info/warn/error§ and global §log(...)§ (captured in §logs§)
- §localStorage§ (persisted per component): §getItem(k)§, §setItem(k, v)§, §removeItem(k)§, §clear()§, §key(i)§, §length§

### Automatic Persistent §CookieJar§
Every §bar.Instance§ has a dedicated §http.CookieJar§ persisted to §<dataDir>/bars/<id>/cookies.json§ (file mode §0600§):
- Any §Set-Cookie§ response header from §fetch()§ is automatically captured, persisted across restarts, and automatically sent on matching subsequent requests (including redirects).
- Users can also import browser cookies (§curl§ or §Cookie:§ string) via the Desktop UI ("Manage cookies") or via MCP (§bar_cookies_set§).
- Note: standard §fetch§ redirect logic drops §Authorization§ / §Cookie§ headers on cross-origin redirects; §CookieJar§ supplies cookies belonging to the redirect target origin if permitted.

### §$metrix§ Global Object
- §$metrix.invalidate()§: re-runs §render(env)§ and updates the tray & open floating panel.
- §$metrix.openURL(url)§: opens an §http://§ or §https://§ URL in the system browser.
- §$metrix.copy(text)§: copies text to the system clipboard.
- §$metrix.log(...args)§: appends to component logs.
- §$metrix.storage§ (persisted to §<dataDir>/bars/<id>/storage.json§, limit 256 keys / 1 MiB):
  - §get(key)§ -> string or §null§
  - §set(key, val)§
  - §delete(key)§
  - §clear()§
  - §keys()§ -> §string[]§
- §$metrix.cookies§ (enforces §<permissions><network .../></permissions>§):
  - §get(url)§ -> §{ [name]: value }§ matching §url§
  - §header(url)§ -> §"name=value; ..."§
  - §set(url, name, value, opts?)§ — §opts§ optional: §{ path, domain, secure, httpOnly, maxAge }§
  - §delete(url, name)§
  - §clear(url?)§ — omits §url§ to clear all cookies for this component, or passes §url§ to clear cookies matching that host

---

## 4. Return Shape: §{ tray, panel }§

### §tray§
§§§js
tray: {
  title: "98%",                 // macOS menubar text (<= 12 chars)
  icon: "lucide:activity",      // optional icon name
  iconTemplate: true,           // monochrome template mode (default true)
  tooltip: "Service Status",
  error: lastError || "",
  menu: [
    { label: "Refresh", onClick: () => { void load(); } },
    { separator: true },
    { label: "Copy Token", onClick: () => $metrix.copy(token), disabled: !token }
  ]
}
§§§

### §panel§ VDOM Nodes (§{ type, props, children }§)
Helper: §const h = (type, props, ...children) => ({ type, props: props || {}, children: children.flat().filter(c => c !== null && c !== undefined && c !== false) });§

Supported §type§ values:
- Layout containers: §Column§, §Row§, §Box§, §Scroll§
  - Common props: §preferredWidth§ (root, e.g. 300), §preferredHeight§ (root), §width§, §height§, §minWidth§, §minHeight§, §maxWidth§, §maxHeight§, §fill§, §fillWidth§, §fillHeight§, §grow§, §shrink§, §basis§, §gap§, §padding§ (§n§ or §[v,h]§ or §[t,r,b,l]§), §radius§, §alignItems§ (§start§|§center§|§end§|§stretch§|§spaceBetween§), §justify§, §background§, §borderWidth§, §borderColor§, §absolute§, §top§, §right§, §bottom§, §left§, §leftPercent§, §widthPercent§, §topPercent§, §heightPercent§, §tooltip§, §onClick§, §hover: { background }§, §pressed: { background }§, §shortcuts: { "CmdOrCtrl+R": fn }§.
- §Text§: §fontSize§, §fontWeight§, §letterSpacing§, §textColor§, §textAlign§ (§start§|§center§|§end§), §singleLine§, §wrap§.
- §Icon§: §name§ (Lucide icon name) or §svg§ (inline SVG), §fontSize§ / §width§, §textColor§, §keyframes§.
- §Button§: §onClick§, §variant§ (§"primary"§, §"danger"§, §"ghost"§), §background§, §textColor§, plus size props.
- §Badge§: pill badge (§background§, §textColor§, §fontSize§).
- §Progress§: progress bar (§value§, §max§ default 100, §height§, §radius§, §color§, §trackColor§, §fillWidth§).
- §Chart§: §type§ (§"line"§ or §"area"§), §height§, §curve§ (§"monotone"§), §strokeWidth§, §xDomain§, §series: [{ name, color, fill: { from, to }, points: [{ x, y }] }]§.
- §Spacer§, §Divider§.

Theme color tokens: §surface§, §surfaceHover§, §surfacePressed§, §text§, §textMuted§, §accent§, §warning§, §danger§, §success§, §border§, §transparent§, or §"#RRGGBB"§.

---

## 5. MCP Development & Debugging Workflow

1. **Inspect existing components**: §bar_list§, §bar_get§, §icon_catalog§.
2. **Validate before saving**: §bar_check§ validates XML, permissions, inputs, compiles ES module, and tests initial §render(env)§.
3. **Sandbox test & simulate Cookies / clicks**: §bar_run§ executes §scheduled(env)§ + §render(env)§ in an isolated in-memory sandbox:
   - Pass §mockFetch§ (including §setCookie§ headers) to test login/session/Set-Cookie flows without hitting real servers.
   - Or pass §curl§ / §cookieHeader§ + §cookieURL§ / §cookies§ / §useSavedState: true§ and §allowNetwork: true§ to test real authenticated endpoints safely.
   - Pass §actionID§ (discovered from the §actions§ output) to test clicking buttons or tray menu actions.
4. **Save & hot-reload**: §bar_save§ saves the XML and configures §mode§ (§panel§|§menu§), §display§ (§cursor§|§internal§|§external§), docks, and §inputs§. The running app automatically restarts the component instance when its source changes.
5. **Manage persisted cookies & storage**: §bar_cookies_set§ (accepts browser §curl§ command, §cookieHeader§ + §url§, or structured cookies), §bar_cookies_get§, §bar_cookies_clear§, §bar_storage_get§, §bar_storage_set§, §bar_storage_clear§.
6. **Verify live output**: §tray_list§, §tray_set§, §menubar_show§.

---

## 6. Complete Cookie-Authenticated Example

§§§xml
<bar id="cookie-session-demo" title="Account Quota" version="1.0.0" category="developer" icon="key" refresh="5m">
  <permissions>
    <network origin="https://api.example.com" />
  </permissions>
  <inputs>
    <input name="baseUrl" type="string" label="Base URL" default="https://api.example.com" />
  </inputs>
  <script><![CDATA[
    const h = (type, props, ...children) => ({
      type, props: props || {}, children: children.flat().filter(c => c !== null && c !== undefined && c !== false)
    });
    let data = null;
    let errMsg = "";

    async function load(env) {
      const base = (env && env.inputs && env.inputs.baseUrl) || "https://api.example.com";
      const cookies = $metrix.cookies.get(base);
      if (Object.keys(cookies).length === 0) {
        errMsg = "No session cookie. Import via Manage Cookies or bar_cookies_set.";
        $metrix.invalidate();
        return;
      }
      try {
        // CookieJar automatically sends persisted cookies and stores any Set-Cookie updates.
        const resp = await fetch(base + "/v1/quota");
        if (!resp.ok) throw new Error("HTTP " + resp.status);
        data = await resp.json();
        errMsg = "";
        $metrix.storage.set("lastQuota", JSON.stringify(data));
      } catch (e) {
        errMsg = String(e && e.message || e);
        const cached = $metrix.storage.get("lastQuota");
        if (cached && !data) {
          try { data = JSON.parse(cached); } catch (_) {}
        }
      }
      $metrix.invalidate();
    }

    export default {
      async scheduled(env) { await load(env); },
      render(env) {
        const pct = data ? Math.round((data.used / Math.max(data.limit, 1)) * 100) : 0;
        return {
          tray: {
            title: data ? pct + "%" : "--",
            tooltip: data ? "Quota " + pct + "%" : (errMsg || "Loading"),
            menu: [{ label: "Refresh", onClick: () => { void load(env); } }]
          },
          panel: h("Column", { preferredWidth: 300, padding: 14, gap: 10 },
            h("Row", { alignItems: "center", gap: 8 },
              h("Text", { fontSize: 14, fontWeight: 600 }, "Account Quota"),
              h("Spacer"),
              h("Badge", { background: errMsg ? "danger" : "accent" }, errMsg ? "Check Cookie" : "Live")
            ),
            errMsg ? h("Text", { fontSize: 12, textColor: "danger", wrap: true }, errMsg) : null,
            h("Progress", { value: pct, max: 100, height: 8 }),
            h("Row", { gap: 8 },
              h("Button", { variant: "primary", onClick: () => { void load(env); } }, "Refresh"),
              data ? h("Button", { onClick: () => $metrix.copy(String(pct) + "%") }, "Copy") : null
            )
          )
        };
      }
    };
  ]]></script>
</bar>
§§§
`
