# Changelog

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
