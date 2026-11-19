# Changelog

## Unreleased

- Menu bar, more to build with: a menu entry can be a submenu, run an action (`refresh`, `open` the card, `window`), open a web link or copy text, and can have a subtitle, a badge, a check mark, or be disabled. An item can draw its title in a monospaced font (`menubar.mono`), and open the card on a click with the menu on a right click (`menubar.click: "open"`).
- MCP knows the menu bar: `plugin_save` takes `menubar` (the card is put in the bar when the plugin first appears; the user decides after that), `plugin_list` shows it, and `script_check` warns about a menu on a card that did not ask for the bar, a link that is not http, an unknown action and a title that will be cut.
- Closing the main window minimizes it instead of quitting: the app keeps running, with its menu bar items and the runs of its cards, and the Dock or taskbar icon brings the window back. It quits with Cmd-Q or the menu's Quit on a Mac, Ctrl+Q elsewhere, the Quit of a menu bar item, or the new Quit in Settings → About. The settings window still closes as a window does.
- Settings open in a window of their own on a computer, at a width that suits them; the main window stays where it was, and asking again brings the window forward. A phone keeps the page. The window has its title once, in its title row.
- Settings, tidied: the daily reminder (it was only saved, never used), the tech stack row and the plugin group's two lines of explanation are gone; the language moved under appearance; the data file and the plugin folder moved into About, which now shows the real version (it said 0.1.0); the MCP server shows its port, keys and the buttons that copy its configuration only once it is on, and the four copy buttons are one row.

## 0.1.8

- Content width is a setting (设置 → 外观 → 内容宽度): auto (pages of data, the home cards and the detail page, fill the window; text pages such as settings keep a reading width), comfortable, wide, or full. A page that is capped sits in the middle of the window, and what is sized from the page (the card grid, the statistics tiles, a picture) follows the same cap. Before, the detail page stopped at a fixed width with blank space on its right, and its tiles wrapped.
- A wide card with selectors and a value to type in no longer spills over the next card: its chart takes what the rest of the content leaves, and the card grows when that is too little.
- Cards are their content: no title row at rest. The name shows with the buttons in a small panel at the top right while the pointer is on the card, and the change chip sits on the summary line. A script can ask for the row back: `layout: {title: "show"}`.
- Cards show their buttons (refresh, size) only while the pointer is on them; a touch screen has no pointer and shows them all the time. A script can ask for them to stay: `layout: {actions: "always"}`.
- A card with pictures is the picture, edge to edge. Its title, caption, arrows, source and copy-link actions, and buttons show over the picture while the pointer is on the card. A script that wants the picture among its other contents says `layout: {image: "inline"}`. The edit layout, the mini size and the full view keep their layouts.
- Progress rings: a `progressRing` chart takes the same `progress` list as `progress` and draws a thin ring for each item, with round ends, a faint track in the ring's own color and an arc that deepens along its length (a medium card has room for three). Everything is adjustable with `ring: {layout, size, thickness, cap, track, fill, center, align, valign}`, and each item can have a `color` and a `to`. `layout: "concentric"` nests the rings in one, as Apple's activity rings are, with a legend, and a value over its maximum goes round a second time.
- A card with no headline or summary no longer keeps an empty line for them, and a progress chart sits right under the summary instead of at the bottom of the card.

## 0.1.7

- Refresh keeps what a card showed: while a card's run is asked again (refresh, a feed's poll, a metric recorded, a table edit, an AI usage update) the card keeps its last result, pictures and figures, marked as updating, instead of going blank. The editor's previews are not kept.
- Live lines: a script can ask for a source every few seconds while its card is on screen (`meta()` `live: {sources, every}`, 5 seconds at least) and draw the source's history as a line that moves. The built-in crypto card's view is live by default (the ticker every 5 seconds, the last 15 minutes); the daily candles are one choice away. Built-in crypto is 1.2.0.
- Menu bar: a script that asks for it (`meta()` `menubar: true`) can show a card in the menu bar (macOS) or notification area, as an icon of its own. The user picks the cards from the card's right-click menu. The icon shows the headline beside it on macOS (the script can give its own title and tooltip), and the card's menu has the script's items (web links only), refresh, the window and quit. With a card in the bar, closing the window hides it and the app keeps running.
- Charts: a `progress` chart shows completion as bars (`progress: [{name, value, max, unit}]`): each bar says "3 / 10 · 30%", or its percentage when there is no maximum. It draws on every card size with a chart; a medium card has room for two bars and says how many more it leaves out, while large cards and the full view show them all.
- Layout: a result can set `layout: {legend, summary, stats}`. The legend (a multi-series chart's names) and the summary line can be hidden; the stats are their own block under the chart. The right-click menu of a card hides or shows its legend, and the choice is kept. A medium card with progress bars shows no stats block; its full view does.
- Plugins: `plugin_save` takes `position` (where the card sits on the home page; leave it out to put it last) and `size` (mini, small, medium, large or wide). A card the user has moved or resized keeps its own place and size.
- MCP: `script_check` warns about a chart type the host does not draw; `plugin_list` shows each plugin's position and size.

## 0.1.6

- Built-in scripts are 1.1.0: unedited installed copies of the previous versions update on start; edited copies are kept.
- Android: the app looks for a newer release when it starts, and offers its APK in the update settings. mygo cannot install updates on a phone, so the APK is installed by hand.
- Phone settings: the AI usage rows stack their inputs under the title instead of squeezing it.
- Sources in full: a source is an address, or `{url, method, headers, query, body, paging}`, so a POST with a body, a login cookie, or a query works. Paging: `paging: {param, start, step, pages, items}` reads a list over several pages and joins them.
- Icons: a catalog of the shipped icons by category (`icon_catalog` over MCP); `script_check` warns about an icon that does not exist.
- Results can carry `stats` (label, value, sub): a card shows them as a row under the headline, the full view as tiles. A live result's chart shows no percentage change.

## 0.1.5

- New built-in: 汇率 · CNY（fxcny），外币兑人民币的汇率走势。
- Built-in scripts that switch: weather and air quality (city), the exchange rate (currency), the crypto price (coin), and the index candles (CSI 300, CSI 500, STAR 50, SSE, SZSE, ChiNext). Each keeps its old default. Unedited copies of the previous versions update on start; edited copies are kept.
- Switchable variables: a script declares `vars` (a city, a currency pair) in `meta()`, and each `{key}` in a source is filled with the value the user picked on the card or the full view. Each value has its own cache and history; `ctx.vars` gives the value to `render`. No code is needed beyond the config.
- Reset data: a script's full view and editor have 「重置数据」 (two clicks), which clears its cached responses and history for every value of its variables, its saved results, and its data source samples.
- The full view's details are folded into one line (「关于 · 自建 · 电脑与手机 · 展开」); the reset is among them.
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
