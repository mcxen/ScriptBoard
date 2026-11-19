// 指数日 K：TickFlow 免费日线（无需 key），默认沪深300。离线或失败时不造数。
// 指数是 vars 里的一个变量：要加指数，在 INDEXES 里加一行即可。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

// A-share daily bars are China-session dates (UTC+8).
const CN_OFFSET = 8 * 3600;

// value 是 TickFlow 的代码（市场后缀 .SH 上交所、.SZ 深交所）。
const INDEXES = [
  { value: "000300.SH", zh: "沪深300", en: "CSI 300" },
  { value: "000905.SH", zh: "中证500", en: "CSI 500" },
  { value: "000688.SH", zh: "科创50", en: "STAR 50" },
  { value: "000001.SH", zh: "上证指数", en: "SSE Composite" },
  { value: "399001.SZ", zh: "深证成指", en: "SZSE Component" },
  { value: "399006.SZ", zh: "创业板指", en: "ChiNext" },
];

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("指数日 K", "Index daily candles"),
    description: t("所选指数的日 K 线", "Daily candles of an index"),
    version: "1.1.0",
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
    vars: [
      {
        key: "index",
        label: t("指数", "Index"),
        default: INDEXES[0].value,
        options: INDEXES.map((x) => ({ value: x.value, label: t(x.zh, x.en) })),
      },
    ],
    sources: {
      k: "https://free-api.tickflow.org/v1/klines?symbol={index}&interval=1d&limit=60",
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
  const ix = INDEXES.find((x) => x.value === ctx.vars.index) || INDEXES[0];
  const title = t(ix.zh, ix.en) + " · " + t("日K", "daily");
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
