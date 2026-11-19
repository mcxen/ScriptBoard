// 加密货币：币安现货 24h、日 K、U 本位资金费与实时价格线，无需 key。离线或失败时不造数。
// 币种是 vars 里的一个变量：要加币种，在 COINS 里加一行即可（需要币安有该 USDT 合约）。
// 实时：视图选「实时」时，ticker 每 5 秒取一次，历史画成一条会动的线（最近约 15 分钟）。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

const COINS = ["BTC", "ETH", "SOL", "BNB", "XRP", "DOGE"];

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("加密货币", "Crypto"),
    description: t(
      "所选币种的实时价格线、现货、日 K 线和资金费率",
      "Spot price, a live price line, daily candles and funding rate for a coin"
    ),
    version: "1.2.0",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    icon: "circle-dollar-sign",
    dims: [
      {
        key: "range",
        label: t("视图", "View"),
        options: [
          { value: 0, label: t("实时", "Live") },
          { value: 7, label: t("7 天", "7 days") },
          { value: 30, label: t("30 天", "30 days") },
          { value: 60, label: t("60 天", "60 days") },
        ],
      },
    ],
    vars: [
      {
        key: "coin",
        label: t("币种", "Coin"),
        default: "BTC",
        options: COINS.map((c) => ({ value: c, label: c + " / USDT" })),
      },
    ],
    sources: {
      ticker: "https://api.binance.com/api/v3/ticker/24hr?symbol={coin}USDT",
      klines: "https://api.binance.com/api/v3/klines?symbol={coin}USDT&interval=1d&limit=60",
      funding: "https://fapi.binance.com/fapi/v1/premiumIndex?symbol={coin}USDT",
    },
    // The ticker is asked for every 5 seconds while the card is on screen; its
    // past answers, 180 of them (15 minutes), are the live line.
    live: { sources: ["ticker"], every: 5 },
    history: ["ticker"],
    historyLimit: 180,
  };
}

function num(x) {
  const n = typeof x === "number" ? x : parseFloat(x);
  return n === n ? n : 0;
}

function pad(n) {
  return (n < 10 ? "0" : "") + n;
}

function mdUTC(ms) {
  const d = new Date(ms);
  return pad(d.getUTCMonth() + 1) + "-" + pad(d.getUTCDate());
}

// hmsOf is the clock time of ms in the user's zone (tz is its offset in seconds).
function hmsOf(ms, tz) {
  const d = new Date(ms + (tz || 0) * 1000);
  return pad(d.getUTCHours()) + ":" + pad(d.getUTCMinutes()) + ":" + pad(d.getUTCSeconds());
}

function signed(n, digits) {
  const s = n.toFixed(digits);
  return n > 0 ? "+" + s : s;
}

function priceOf(src) {
  if (!src || src.error) return 0;
  const p = num(src.lastPrice || src.price);
  return p > 0 ? p : 0;
}

function barsOf(src) {
  if (!src || src.error || src.length == null) return [];
  const out = [];
  for (let i = 0; i < src.length; i++) {
    const row = src[i];
    if (!row || row.length < 5) continue;
    const tms = num(row[0]);
    const o = num(row[1]);
    const h = num(row[2]);
    const l = num(row[3]);
    const c = num(row[4]);
    if (tms <= 0 || o <= 0 || h <= 0 || l <= 0 || c <= 0) continue;
    out.push({ t: tms, o: o, h: h, l: l, c: c });
  }
  return out;
}

// liveChart draws the prices the ticker gave, oldest first: each answer of the
// live source is a point (ctx.history.ticker). Two points make a line.
function liveChart(ctx, coin) {
  const hist = (ctx.history && ctx.history.ticker) || [];
  const pts = [];
  for (let i = 0; i < hist.length; i++) {
    const p = hist[i];
    const px = priceOf(p && p.v);
    if (px > 0 && p.t > 0) pts.push({ t: p.t, px: px });
  }
  if (pts.length < 2) return null;
  return {
    type: "line",
    series: [{ name: coin + " / USDT", points: pts.map((p) => p.px) }],
    labels: pts.map((p) => hmsOf(p.t, ctx.tzOffset)),
    xLabels: [hmsOf(pts[0].t, ctx.tzOffset), hmsOf(pts[pts.length - 1].t, ctx.tzOffset)],
  };
}

function empty(title, why, lines) {
  return {
    title: title,
    icon: "circle-dollar-sign",
    big: "--",
    sub: why,
    lines: lines || [],
    chart: null,
  };
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const coin = COINS.includes(ctx.vars.coin) ? ctx.vars.coin : "BTC";
  const title = coin + " · USDT";
  const sources = ctx.sources || {};
  const n = ctx.view.range ?? 30;
  const live = n === 0;
  let bars = barsOf(sources.klines);
  if (!live && n > 0 && bars.length > n) bars = bars.slice(-n);
  const lastBar = bars.length ? bars[bars.length - 1] : null;
  const px = priceOf(sources.ticker) || (lastBar ? lastBar.c : 0);
  if (px <= 0) {
    const why =
      (sources.ticker && sources.ticker.error) ||
      (sources.klines && sources.klines.error) ||
      t("无数据", "no data");
    return empty(title, t("未能取到行情：", "Could not fetch quotes: ") + String(why), [
      t("需要网络才能取币安行情", "Needs the network for Binance quotes"),
    ]);
  }
  const ticker = sources.ticker && !sources.ticker.error ? sources.ticker : null;
  const fromTicker = ticker && ticker.priceChangePercent != null && ticker.priceChangePercent !== "";
  let pct = 0;
  if (fromTicker) {
    pct = num(ticker.priceChangePercent);
  } else if (lastBar && bars.length > 1) {
    const prev = bars[bars.length - 2].c;
    if (prev !== 0) pct = ((lastBar.c - prev) / prev) * 100;
  }
  const lines = [];
  if (ticker) {
    const hi = num(ticker.highPrice);
    const lo = num(ticker.lowPrice);
    if (hi > 0 && lo > 0) {
      lines.push(t("24h ", "24h ") + lo.toFixed(0) + " – " + hi.toFixed(0));
    }
  }
  const fund = sources.funding && !sources.funding.error ? sources.funding : null;
  if (fund && fund.lastFundingRate != null) {
    lines.push(t("资金费 ", "Funding ") + signed(num(fund.lastFundingRate) * 100, 4) + "%");
  }
  let chart = null;
  if (live) {
    chart = liveChart(ctx, coin);
    if (!chart) lines.push(t("实时线：正在记录，几秒后出现", "Live line: recording, it appears in a few seconds"));
  } else if (lastBar) {
    chart = {
      type: "candle",
      candles: bars.map((b) => ({ o: b.o, h: b.h, l: b.l, c: b.c })),
      labels: bars.map((b) => mdUTC(b.t)),
      xLabels: [mdUTC(bars[0].t), mdUTC(lastBar.t)],
    };
  }
  return {
    title: title,
    icon: "circle-dollar-sign",
    big: px.toFixed(2),
    sub: (fromTicker ? t("24h ", "24h ") : t("较前一根 ", "vs previous ")) + signed(pct, 2) + "%",
    lines: lines,
    chart: chart,
  };
}
