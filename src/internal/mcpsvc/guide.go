package mcpsvc

import (
	"fmt"
	"strings"

	"scriptboard/src/internal/caps"
)

// Guide is the reference an assistant reads before it writes a plugin. It is
// built from the lists in package caps, so it says what the host really
// accepts, whatever the contract grows to.
func Guide() string {
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }

	w("# ScriptBoard plugin guide")
	w("")
	w("A plugin is one ES module that exports `meta()` and `render(ctx)`. Save it with `plugin_save`; check it first with `script_check`, which reports the contract below and warns when what the script declares differs from what it uses.")
	w("")
	w("## 1. meta()")
	w("")
	w("```js")
	w("export function meta(a) {          // a.lang is \"zh\" or \"en\"")
	w("  return {")
	w("    name: \"Bitcoin\",               // required, at most 60 characters")
	w("    description: \"Spot price and daily candles\",  // one line")
	w("    version: \"1.0.0\",              // x.y.z")
	w("    icon: \"circle-dollar-sign\",    // a Lucide icon name, see icon_list")
	w("    platforms: [%s],", quoted([]string{caps.Desktop, caps.Mobile}))
	w("    needs: [%s],             // [] when it needs nothing", quoted([]string{caps.Network}))
	w("    sources: { ticker: \"https://api.example.com/ticker\" },  // fetched by the host; an object gives method, headers, query, body and paging")
	w("    // dims: [...]  adjustable options;  inputs: [...]  values the user types in")
	w("    // feeds: [...]  data sources the user set up;  usage: [...]  AI usage providers")
	w("    // vars: [{ key: \"city\", label: \"城市\", default: \"shanghai\", options: [{ value: \"shanghai\", label: \"上海\" }] }]  values the user switches; a source writes {city}")
	w("  };")
	w("}")
	w("```")
	w("")
	w("## 2. platforms: where it runs")
	w("")
	w("Words: %s. `desktop` is %s; `mobile` is %s; the others are one system.", quoted(caps.Platforms),
		quoted(caps.Expand(caps.Desktop)), quoted(caps.Expand(caps.Mobile)))
	w("")
	w("- Leave `platforms` out and the script runs wherever its needs allow.")
	w("- A script for one system lists only that system: `platforms: [\"windows\"]` is not offered, and does not run, on macOS, Linux or a phone. It has no card on the home page there, and the editor marks it \"Windows only\".")
	w("- What the script uses can narrow it further: a script that reads the computer's files cannot run on a phone, whatever it declares. `script_check` warns when you claim a system it cannot run on.")
	w("")
	w("## 3. needs and permissions")
	w("")
	w("| need | meaning | phone | allowed by |")
	w("| --- | --- | --- | --- |")
	w("| `%s` | makes HTTP requests (`sources`, feeds) | yes | nobody: always |", caps.Network)
	w("| `%s` | reads files that only a computer has, such as another tool's logs | no | the user, per script |", caps.Files)
	w("| `%s` | runs a program on the computer | no | the user, per script |", caps.Exec)
	w("| `%s` | needs a service key the user entered | yes | the user, per script |", caps.Key)
	w("")
	w("Declare every need. The host adds what it sees you use (a `sources` entry is `network`; the `claude-code` usage provider is `files`), and `script_check` warns about one you used but did not declare.")
	w("")
	w("A script that is not built in uses `files`, `exec` or `key` only after the user allows it in the editor, one script and one feature at a time. Without it the script does not run: the card says which permission is missing. Do not try to work around this.")
	w("")
	w("## 4. What the host gives the script")
	w("")
	w("The script never opens a file, starts a program or makes a request itself. It names what it wants in `meta()`, and the host does the work, on any system, and hands the result to `render(ctx)`:")
	w("")
	w("| `ctx` | what it is |")
	w("| --- | --- |")
	w("| `ctx.now`, `ctx.tzOffset`, `ctx.lang` | time in milliseconds, local offset in seconds, `\"zh\"` or `\"en\"` |")
	w("| `ctx.view` | the value of each adjusted `dims` entry, by key |")
	w("| `ctx.metrics` | the values the user recorded, by `inputs` metric: `[{t, v}]` |")
	w("| `ctx.sources` | each `sources` URL, fetched and parsed as JSON; `{error}` if it failed. A source that needs the user's login takes its headers from `sourceHeaders: { name: { Cookie: \"session=…\" } }` in `meta()` |")
	w("| `ctx.feeds` | each `feeds` source (curl, tcp, file, exec) with its `hour` and `day` aggregates, `latest`, `stale`, `error` |")
	w("| `ctx.usage` | each `usage` provider's report: tokens or a balance, windows, models, daily |")
	w("| `ctx.sourceInfo` | for each source `{t, cached, stale, error}`: when it was fetched (ms), whether it came from the cache, and whether the cache stood in for a failed or offline request |")
	w("**Sources in full.** A source is its address as a string, or `{url, method, headers, query, body, paging}`: `method` (GET by default, or POST with a `body`), `headers` (a `Cookie` header is how a login is sent), `query` (added to the address), and `paging: {param, start, step, pages, items}` to read a list over several pages: the page number goes in `param`, and the lists are joined; `items` is the path of the list in each response (`data.list`), or empty when the response is the list. Reading stops at a page with no items. Variables and dates fill every part.")
	w("**Home position and size.** `plugin_save` takes `position` (1 is the first card on the home page; leave it out to put it last) and `size` (mini, small, medium, large or wide; the default is medium). The user can change either later, and the size they choose wins.")
	w("**Menu bar: `menubar`.** A script that can show in the menu bar (macOS) or the notification area (Windows, Linux) says so in `meta()`: `menubar: true`. The user chooses which cards show there, from the card's right-click menu; such a card has an icon of its own. A plugin saved with `plugin_save` `menubar: true` is put there when it first appears (its script must say `menubar: true` too). Its result may give `menubar: {title, tooltip, mono, click}`: the title shows beside the icon on macOS only, and without it the title is the headline (cut to 12 characters) and the tooltip the headline with the summary; `mono: true` draws the title in a monospaced font, so that a changing number keeps its width; `click: \"open\"` makes a click open the card in the window, with its menu on a right click. It may give `menu: [...]`: each entry is `{separator: true}` or `{label, ...}` with one of `items: [...]` (a submenu, one level), `action: \"refresh\"|\"open\"|\"window\"`, `url` (web addresses only) or `copy` (text for the clipboard); with none of them it is a line of text. An entry may also have `sub` (a second line), `badge` (short text at its end), `disabled` and `checked`. The menu ends with refresh, the window and quit. Ask for the menu bar only when a card is worth an icon: it runs every minute while it is there. `script_check` warns about what the app cannot use.")
	w("**Live: `live`.** A source whose value moves quickly (a price) says so in `meta()`: `live: {sources: [\"ticker\"], every: 5}`. While the card is on screen, in the menu bar or in its full view, the named sources are asked for again every `every` seconds (at least 5); the other sources keep the minute's cache. Their history is kept for the script: the live sources are in `history` already, and `ctx.history.ticker` is `[{t, v}]`, oldest first, so a line is drawn from those points. `historyLimit` is how far back the line goes: 180 answers is 15 minutes at 5 seconds. Nothing is asked for while the card is off screen or the app is offline.")
	w("**Icons.** `icon` is one name from `icon_catalog`, which lists the shipped icons by category (weather, finance, charts, time, body, ai, data, interface). A name that is not there shows the default picture; `script_check` warns about it.")
	w("| `ctx.vars` | the value the user chose for each of `meta().vars`, as `{key: value}`; the same value fills `{key}` in the sources, so each choice has its own data and history |")
	w("| `ctx.history` | with `history: [\"name\"]` (and optional `historyLimit`, default 120) in `meta()`: the past responses of those sources, oldest first, as `[{t, v}]`. Use it to chart something an API only gives the latest value of |")
	w("| `ctx.platform` | `{os, kind}`: the system (%s) and its group (`desktop` or `mobile`) |", quoted(caps.Systems))
	w("| `ctx.granted` | the sensitive features this script may use, as `{files: true}` |")
	w("| `ctx.settings` | `weightUnit`, `offline` |")
	w("")
	w("**Caching and offline belong to the host.** Every `sources` response is kept on the device: a response under a minute old is reused, a failed request falls back to the last one (`sourceInfo.stale`), and offline mode makes no request and serves the cache. Do not cache or retry in a script; read `ctx.sourceInfo` if you want to say that data is old. Only JSON responses are cached.")
	w("")
	w("**System differences belong to the host.** Where Claude Code keeps its logs on Windows, macOS or Linux, how a file path is written, which protocols a data source can use: ask for `ctx.usage[\"claude-code\"]` or a feed and the host knows. Do not branch on `ctx.platform.os` to find files; use `ctx.platform` only to word a message, and degrade gracefully (show `--` and say why) when something is not there. If a script needs a feature the host does not provide on its systems, declare `platforms` honestly rather than emulating it.")
	w("")
	w("## 5. render(ctx)")
	w("")
	w("Return `{title, icon, big, sub, lines, chart, stats, images, layout}`. `layout` is optional: `{legend: \"show\"|\"hide\", summary: \"top\"|\"hide\", stats: \"bottom\"|\"hide\", actions: \"hover\"|\"always\", image: \"cover\"|\"inline\", title: \"hide\"|\"show\"}`; `title` says whether a card has a row of its own for its name and icon at the top (`show`), or is only its content (`hide`, the default): its name and buttons then show over it while the pointer is on the card; `actions` says whether a card's buttons (refresh, size) show only while the pointer is on the card (the default; a touch screen always shows them) or all the time, and `image` whether a card with pictures is the picture, edge to edge, with its text and buttons over it on hover (the default), or has the picture as a block among its contents; the legend names a multi-series chart's lines, the summary is the line under the headline, and the stats are their own block under the chart. `stats` is an optional list of `{label, value, sub}` figures: a card shows them in their own block at its foot (a mini card shows none), and the full view as tiles (a live weather uses it for feels-like, humidity and wind). `big` is the headline figure (a number in it moves when it changes); `chart` is `null` or `{type, ...}` with `type`: `line`, `dot`, `candle`, `gauge`, `barh`, `combo`, `heat`, `panels`, `progress`, `progressRing`, `calendar`. A `calendar` chart is a month of days: the host fills the lunar date and the holidays, and a click on a day shows them; it needs no series. A `progressRing` chart takes the same `progress` list and draws a ring for each item instead of a bar (a percentage in the ring, the name and the count under it). It is highly adjustable: `ring: {layout: \"row\"|\"concentric\", size, thickness (0.04 to 0.3 of the size), cap: \"round\"|\"butt\", track: \"tint\"|\"neutral\"|\"none\", fill: \"gradient\"|\"solid\", center: \"percent\"|\"value\"|\"none\", align: \"left\"|\"center\"|\"right\"|\"between\"|\"around\"|\"evenly\", valign: \"top\"|\"middle\"|\"bottom\"}`, and an item may have `color` (\"#RRGGBB\") and `to` (the color its arc deepens to). `concentric` nests the rings in one, as Apple's activity rings are, with a legend; a value over its max goes round a second time. A `progress` chart is `{type: \"progress\", progress: [{name, value, max, unit}]}`: one bar per item, the value out of its max (or a percentage when max is 0), with its label and percentage; use it for completion (2 of 5 done). Any other type shows nothing.")
	w("")
	w("- `candle`: give `candles: [{o,h,l,c}]` and `labels`, or `ticks: [{t,v}]` with `period` in seconds (`-1` is a calendar month); the host groups them.")
	w("- Series are `{name, color, points}`; a dashed `series` has `dashed: true`.")
	w("- Names the user sees should come in both languages: `const t = (zh, en) => lang === \"en\" ? en : zh`.")
	w("- Offline or failing sources: show `--` and the reason. Never invent data.")
	w("")
	w("## 6. Before you save")
	w("")
	w("1. `script_check` with the source; fix every warning.")
	w("2. `plugin_save`; the app shows it within a second. Local plugins need no login and no review.")
	w("3. A plugin is marked \"Local plugin\" and is read-only in the app; change it here.")
	return b.String()
}

func quoted(words []string) string {
	q := make([]string, len(words))
	for i, w := range words {
		q[i] = `"` + w + `"`
	}
	return strings.Join(q, ", ")
}
