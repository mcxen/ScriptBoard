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
	w("    sources: { ticker: \"https://api.example.com/ticker\" },  // fetched by the host")
	w("    // dims: [...]  adjustable options;  inputs: [...]  values the user types in")
	w("    // feeds: [...]  data sources the user set up;  usage: [...]  AI usage providers")
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
	w("Return `{title, icon, big, sub, lines, chart}`. `big` is the headline figure (a number in it moves when it changes); `chart` is `null` or `{type, ...}` with `type`: `line`, `dot`, `candle`, `gauge`, `barh`, `combo`, `heat`, `panels`.")
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
