// 金价 K 线：把 ctx.metrics.gold 聚合成 K 线（开、高、低、收）。
// 维度：范围（天）与周期（1 = 日K，7 = 周K）。没有数据时用示例数据。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("金价 K 线", "Gold candles"),
    icon: "candlestick-chart",
    inputs: [{ metric: "gold", label: t("金价", "Gold"), unit: t("元/克", "CNY/g"), step: 0.1 }],
    dims: [
      {
        key: "range",
        label: t("范围", "Range"),
        options: [
          { value: 7, label: t("7 天", "7 days") },
          { value: 30, label: t("30 天", "30 days") },
          { value: 0, label: t("全部", "All") },
        ],
      },
      {
        key: "period",
        label: t("周期", "Period"),
        options: [
          { value: 1, label: t("日K", "Daily") },
          { value: 7, label: t("周K", "Weekly") },
        ],
      },
    ],
    sources: {},
  };
}

function dayIndex(t, off) {
  return Math.floor((t + off * 1000) / 86400000);
}
function pad(n) {
  return (n < 10 ? "0" : "") + n;
}
function monthDay(z) {
  z += 719468;
  const era = Math.floor(z / 146097);
  const doe = z - era * 146097;
  const yoe = Math.floor((doe - Math.floor(doe / 1460) + Math.floor(doe / 36524) - Math.floor(doe / 146096)) / 365);
  const doy = doe - (365 * yoe + Math.floor(yoe / 4) - Math.floor(yoe / 100));
  const mp = Math.floor((5 * doy + 2) / 153);
  const d = doy - Math.floor((153 * mp + 2) / 5) + 1;
  const m = mp < 10 ? mp + 3 : mp - 9;
  return [m, d];
}
function mmdd(day) {
  const md = monthDay(day);
  return pad(md[0]) + "-" + pad(md[1]);
}

// 按周期聚合：period 天为一根 K 线。开 = 第一条，收 = 最后一条，
// 高 / 低 = 区间内最高 / 最低。
function aggregate(pts, off, period) {
  const buckets = {};
  for (let i = 0; i < pts.length; i++) {
    const k = Math.floor(dayIndex(pts[i].t, off) / period);
    const v = pts[i].v;
    const b = buckets[k];
    if (!b) {
      buckets[k] = { o: v, h: v, l: v, c: v };
    } else {
      if (v > b.h) b.h = v;
      if (v < b.l) b.l = v;
      b.c = v;
    }
  }
  const keys = Object.keys(buckets).map(Number).sort((a, b) => a - b);
  return {
    days: keys.map((k) => k * period),
    candles: keys.map((k) => buckets[k]),
  };
}

// 示例：30 个交易日的走势，只在没有录入时使用。
function sample() {
  const candles = [];
  const days = [];
  let base = 620;
  for (let i = 0; i < 30; i++) {
    const o = base;
    const c = base + 4 * Math.sin(i / 3) + (i % 4) - 1.5;
    const h = Math.max(o, c) + 2 + (i % 3);
    const l = Math.min(o, c) - 2 - (i % 2);
    candles.push({ o: o, h: h, l: l, c: c });
    days.push(i);
    base = c;
  }
  return { days: days, candles: candles };
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const off = ctx.tzOffset;
  const range = ctx.view.range ?? 0;
  const period = ctx.view.period ?? 1;
  const since = range > 0 ? ctx.now - range * 86400000 : 0;
  const pts = (ctx.metrics.gold ?? [])
    .slice()
    .sort((a, b) => a.t - b.t)
    .filter((p) => p.t >= since);

  let data = pts.length > 0 ? aggregate(pts, off, period) : null;
  let note = "";
  if (!data) {
    data = sample();
    note = t("示例数据：还没有金价录入", "Sample data: no gold prices recorded yet");
  }
  // 最多显示最近 40 根，避免挤在一起。
  const from = Math.max(0, data.candles.length - 40);
  const cs = data.candles.slice(from);
  const days = data.days.slice(from);
  const last = cs[cs.length - 1].c;
  const prev = cs.length > 1 ? cs[cs.length - 2].c : cs[0].o;
  const pct = prev === 0 ? 0 : ((last - prev) / prev) * 100;
  const labels = days.map((d) => mmdd(d));
  const unit = period === 7 ? t("周", "week") : t("日", "day");
  return {
    title: EN ? `Gold · ${unit} candles (CNY/g)` : `金价 · ${unit}K（元/克）`,
    icon: "candlestick-chart",
    big: last.toFixed(2),
    sub: `${t("较前", "vs prev ")}${unit} ${pct >= 0 ? "+" : ""}${pct.toFixed(2)}%`,
    lines: note ? [note] : [EN ? `${cs.length} candles · hover for open, high, low, close` : `${cs.length} 根K线 · 悬浮查看每根的开高低收`],
    chart: {
      type: "candle",
      candles: cs,
      labels: labels,
      xLabels: [labels[0], labels[labels.length - 1]],
    },
  };
}
