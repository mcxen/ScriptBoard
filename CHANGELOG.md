# Changelog

## Unreleased

- Home layout: an edit mode (「编辑布局」). Drag a card to move it, drag its bottom-right corner to resize it (width snaps to columns, height in steps of 16). The layout is saved. The size guide strip above the cards is gone; the right-click menu stays.
- The home grid uses the whole window: a wide window no longer leaves a blank strip on the right, and a row's last card fills what the row has left.

## 0.1.4

- Dark mode: series colors are lifted on dark cards, and the connecting line of dot charts is clearer.
- Offline messages follow the app language.
- Mac OS 9 on a phone: the desktop menu bar and window buttons are no longer shown.
- Sources can send headers, so a login-protected source gets its cookie: `sourceHeaders: { name: { Cookie: "…" } }` in `meta()`.

## 0.1.3

- Look: a calmer palette (cool neutrals, a steadier blue), one type scale (11, 12, 14, 16, 20, 24), settings on the same white surface as the cards, and chart colors that are the same everywhere. Built-in scripts that were never edited take the new colors on start.
- Phone: the first frame is laid out for a phone, not the desktop; the card-size control sits under the title; wide settings controls stack under their label; a long chart-style list is a dropdown; charts say "tap" rather than "hover".
- Code: one stats tile row for charts and gauges, one page header, one section-title and home-switch helper in place of the copies.
- Phone editor, redone: the editor is a list of scripts and data tables, and the thing you pick opens on its own screen with Save at the top and a switch between the editor (source, or the rows) and the preview. A table's rows are cards of labelled fields, eight to a page; the columns are edited on request.
- Data-table plugins: a plugin can be a table instead of a script (Edit → + Data table). It is edited as rows (cells, add and delete rows and columns, sort by time, pages of 40), imported from Excel (.xlsx), CSV or TSV (GBK too) or pasted, joined or replacing, and exported as CSV; its numbers become a chart and a home card. The editor shows a script as source and a table as rows, never both; the "record data" panel of the script editor is gone. The built-in Weight data and Gold data tables edit the recorded values the cards read.
- Local cache and offline: every source response is kept on the device (fresh for a minute; a failed request falls back to the last one), offline mode serves it, and a card opens at once from its last result and updates behind it, saying how old it is. Scripts can ask for the history of a source with `meta.history` (`ctx.history`) and see `ctx.sourceInfo`. Settings show the cache size and clear it.
- Platforms are isolated by system: a script can be for `macos`, `windows`, `linux`, `android` or `ios` (or `desktop`, `mobile`); one for Windows has no card, and does not run, anywhere else.
- Permissions: a script that is not built in uses the computer's files, programs or a service key only after the user allows it, per script and per feature; a built-in script is trusted while unchanged. Data sources that read a file or run a program count too.
- The host provides the system-specific parts: scripts get `ctx.platform` and `ctx.granted`, and Claude Code's log folders are found per system (including `CLAUDE_CONFIG_DIR`).
- MCP: a `plugin_guide` tool with the full plugin reference, generated from the contract.

## 0.1.2

- Plugin contract: every script declares its version, the platforms it runs on (computer, phone) and the features it needs (network, local files, programs, a key); the host adds what it sees the script use, and warns when they disagree. A script this device cannot run has no home card and says why. Claude Code usage is computer only.
- Origins: built-in, community, local plugin and own scripts are marked in the editor and the full view; built-in scripts can be restored, own and community scripts deleted.
- MCP: `script_check`, `plugin_list` and `plugin_save` report the contract and its warnings.
- New built-in widgets: Shanghai air, Bitcoin, bank gold (accumulation plans) and CSI 300; sources are fetched in parallel.

## 0.1.1

- Mac OS 9 skin, after AvaMedia's Platinum: pixel font (Fusion Pixel 12px, OFL), Platinum colors, bevelled controls, category strips, lavender selection.
- Appearance: style (default or Mac OS 9) and light, dark or system; settings are kept.
- Charts: combo (curve with rings), heat table (click a cell for its detail), stacked panels with a shared crosshair; K-line periods by second, minute, hour, day, week and month; hollow candles and OHLC bars.
- Numbers ease to their new value; the full view follows the point under the pointer, with its date and O/H/L/C.
- Window: macOS traffic lights in a title row, and Windows/Linux minimize, maximize and close (as notes does).
- The sidebar folds into an icon rail; Escape goes back a page.
- Editor: copy a script as a new home card; home-card switches are in the editor.
- Fixes: home cards keep the chosen chart style; the quick-entry button stays on medium cards.

## 0.1.0

First release.

- Home cards drawn from scripts: line, area, dot, bar, step, candle, remaining-amount ring and horizontal bar charts; cards of different sizes; click to refresh; full-screen view with crosshair and statistics.
- Scripts are ES modules run in moejs (`meta()` and `render(ctx)`), with an editor, import and export, and local plugins that need no login.
- Data sources: curl, http(s), tcp, file and exec protocols, hour and day aggregation, retries, fallback requests, circuit breaker and health status.
- AI usage: Claude Code local logs and the balances of DeepSeek, Moonshot, SiliconFlow and OpenRouter.
- Chinese (盯盯板) and English (ScriptBoard) interface.
- MCP server (HTTP with a key, or stdio) for building plugins and icons with an assistant.
- Community on Cloudflare Workers: download without login, upload with login and review.
- Signed updates from GitHub releases, with a tvv.tw mirror option.
