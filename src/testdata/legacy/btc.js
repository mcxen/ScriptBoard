// 比特币：币安现货 24h、日 K 与 U 本位资金费，无需 key。离线或失败时不造数。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("比特币", "Bitcoin"),
    description: t("比特币现货、日 K 线和资金费率", "Bitcoin spot, daily candles and funding rate"),
    version: "1.0.0",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    icon: "circle-dollar-sign",
    dims: [
      {
        key: "range",
        label: t("范围", "Range"),
        options: [
          { value: 7, label: t("7 天", "7 days") },
          { value: 30, label: t("30 天", "30 days") },
          { value: 60, label: t("60 天", "60 days") },
        ],
      },
    ],
    sources: {
      ticker: "https://api.binance.com/api/v3/ticker/24hr?symbol=BTCUSDT",
      klines: "https://api.binance.com/api/v3/klines?symbol=BTCUSDT&interval=1d&limit=60",
      funding: "https://fapi.binance.com/fapi/v1/premiumIndex?symbol=BTCUSDT",
    },
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
  const title = t("比特币 · USDT", "Bitcoin · USDT");
  const sources = ctx.sources || {};
  let bars = barsOf(sources.klines);
  const n = ctx.view.range ?? 30;
  if (n > 0 && bars.length > n) bars = bars.slice(-n);
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
  const chart = lastBar
    ? {
        type: "candle",
        candles: bars.map((b) => ({ o: b.o, h: b.h, l: b.l, c: b.c })),
        labels: bars.map((b) => mdUTC(b.t)),
        xLabels: [mdUTC(bars[0].t), mdUTC(lastBar.t)],
      }
    : null;
  return {
    title: title,
    icon: "circle-dollar-sign",
    big: px.toFixed(2),
    sub: (fromTicker ? t("24h ", "24h ") : t("较前一根 ", "vs previous ")) + signed(pct, 2) + "%",
    lines: lines,
    chart: chart,
  };
}
