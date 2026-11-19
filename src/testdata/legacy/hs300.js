// 沪深300 日 K：TickFlow 免费日线（无需 key）。离线或失败时不造数。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

// A-share daily bars are China-session dates (UTC+8).
const CN_OFFSET = 8 * 3600;

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("沪深300", "CSI 300"),
    description: t("沪深300 指数日 K 线", "CSI 300 daily candles"),
    version: "1.0.0",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    icon: "candlestick-chart",
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
      k: "https://free-api.tickflow.org/v1/klines?symbol=000300.SH&interval=1d&limit=60",
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

function md(ms, off) {
  const d = new Date(ms + (off || 0) * 1000);
  return pad(d.getUTCMonth() + 1) + "-" + pad(d.getUTCDate());
}

function signed(n, digits) {
  const s = n.toFixed(digits);
  return n > 0 ? "+" + s : s;
}

function barsOf(src) {
  const d = src && src.data ? src.data : src;
  if (!d) return [];
  const ts = d.timestamp;
  const o = d.open;
  const h = d.high;
  const l = d.low;
  const c = d.close;
  if (!ts || !o || !h || !l || !c) return [];
  const n = Math.min(ts.length, o.length, h.length, l.length, c.length);
  const out = [];
  for (let i = 0; i < n; i++) {
    const tms = num(ts[i]);
    const oo = num(o[i]);
    const hh = num(h[i]);
    const ll = num(l[i]);
    const cc = num(c[i]);
    if (tms <= 0 || oo <= 0 || hh <= 0 || ll <= 0 || cc <= 0) continue;
    if (hh < oo && hh < cc) continue;
    if (ll > oo && ll > cc) continue;
    out.push({ t: tms, o: oo, h: hh, l: ll, c: cc });
  }
  return out;
}

function empty(title, why, lines) {
  return {
    title: title,
    icon: "candlestick-chart",
    big: "--",
    sub: why,
    lines: lines || [],
    chart: null,
  };
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const title = t("沪深300 · 日K", "CSI 300 · daily");
  const src = ctx.sources && ctx.sources.k;
  if (!src || src.error) {
    const why = src && src.error ? String(src.error) : t("无数据", "no data");
    return empty(title, t("未能取到行情：", "Could not fetch quotes: ") + why, [
      t("需要网络才能取沪深300日线", "Needs the network for CSI 300 daily bars"),
    ]);
  }
  let bars = barsOf(src);
  const n = ctx.view.range ?? 30;
  if (n > 0 && bars.length > n) bars = bars.slice(-n);
  if (bars.length === 0) {
    return empty(title, t("接口没有返回K线", "The feed returned no candles"), []);
  }
  const last = bars[bars.length - 1];
  const prev = bars.length > 1 ? bars[bars.length - 2].c : last.o;
  const pct = prev === 0 ? 0 : ((last.c - prev) / prev) * 100;
  const labels = bars.map((b) => md(b.t, CN_OFFSET));
  return {
    title: title,
    icon: "candlestick-chart",
    big: last.c.toFixed(2),
    sub: t("较前一根 ", "vs previous ") + signed(pct, 2) + "%",
    lines: [
      labels[labels.length - 1] +
        t(" 开", " O ") +
        last.o.toFixed(2) +
        t(" 高", " H ") +
        last.h.toFixed(2) +
        t(" 低", " L ") +
        last.l.toFixed(2),
    ],
    chart: {
      type: "candle",
      candles: bars.map((b) => ({ o: b.o, h: b.h, l: b.l, c: b.c })),
      labels: labels,
      xLabels: [labels[0], labels[labels.length - 1]],
    },
  };
}
