// 金价 K 线：把金价录入交给宿主，按周期聚合成 K 线（秒、分、时、日、周、月）。
// 维度：范围（天）与周期。没有数据时用示例行情。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

// The periods, in seconds; -1 is a calendar month, grouped by the host.
const PERIODS = [
  { value: 86400, zh: "日", en: "Day" },
  { value: 604800, zh: "周", en: "Week" },
  { value: -1, zh: "月", en: "Month" },
  { value: 3600, zh: "时", en: "Hour" },
  { value: 60, zh: "分", en: "Minute" },
  { value: 1, zh: "秒", en: "Second" },
];

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
        options: PERIODS.map((p) => ({ value: p.value, label: t(p.zh, p.en) })),
      },
    ],
    sources: {},
  };
}

// The name of the period in the language of the user.
function periodName(value) {
  const p = PERIODS.find((x) => x.value === value) ?? PERIODS[0];
  return t(p.zh, p.en);
}

// 示例行情：40 根 K 线，每根 4 个报价，只在没有录入时使用。
function sample(period, now) {
  const step = period === -1 ? 2592000 : period; // a month, for the sample
  const start = Math.floor(now / 1000 / step) * step - 39 * step;
  const ticks = [];
  let base = 620;
  for (let i = 0; i < 40; i++) {
    const open = base;
    const close = base + 4 * Math.sin(i / 3) + (i % 4) - 1.5;
    const high = Math.max(open, close) + 2 + (i % 3);
    const low = Math.min(open, close) - 2 - (i % 2);
    const t0 = (start + i * step) * 1000;
    const span = step * 1000;
    ticks.push(
      { t: t0, v: open },
      { t: t0 + Math.floor(span / 4), v: high },
      { t: t0 + Math.floor(span / 2), v: low },
      { t: t0 + span - 1, v: close },
    );
    base = close;
  }
  return ticks;
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const period = ctx.view.period ?? 86400;
  const range = ctx.view.range ?? 0;
  const since = range > 0 ? ctx.now - range * 86400000 : 0;
  let ticks = (ctx.metrics.gold ?? [])
    .slice()
    .sort((a, b) => a.t - b.t)
    .filter((p) => p.t >= since);

  let note = "";
  if (ticks.length === 0) {
    ticks = sample(period, ctx.now);
    note = t("示例数据：还没有金价录入", "Sample data: no gold prices recorded yet");
  }
  const last = ticks[ticks.length - 1].v;
  const prev = ticks.length > 1 ? ticks[ticks.length - 2].v : last;
  const pct = prev === 0 ? 0 : ((last - prev) / prev) * 100;
  const unit = periodName(period);
  return {
    title: `${t("金价", "Gold")} · ${unit}K（${t("元/克", "CNY/g")}）`,
    icon: "candlestick-chart",
    big: last.toFixed(2),
    sub: `${t("较前一根", "vs previous")} ${pct >= 0 ? "+" : ""}${pct.toFixed(2)}%`,
    lines: note ? [note] : [t("悬浮查看每根的开高低收", "Hover a candle for its open, high, low and close")],
    chart: { type: "candle", ticks: ticks, period: period, limit: 60 },
  };
}
