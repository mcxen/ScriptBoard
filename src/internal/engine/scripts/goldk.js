// 金价 K 线：把金价录入交给宿主，按周期聚合成 K 线（秒、分、时、日、周、月）。
// 维度：范围（天）与周期。没有录入时不造数。
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
    description: t("把你录入的金价按周期聚合成 K 线", "Candles of the gold prices you record, by period"),
    version: "1.1.0",
    platforms: ["desktop", "mobile"],
    needs: [],
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

export function render(ctx) {
  EN = ctx.lang === "en";
  const period = ctx.view.period ?? 86400;
  const range = ctx.view.range ?? 0;
  const since = range > 0 ? ctx.now - range * 86400000 : 0;
  const ticks = (ctx.metrics.gold ?? [])
    .slice()
    .sort((a, b) => a.t - b.t)
    .filter((p) => p.t >= since);
  const unit = periodName(period);

  if (ticks.length === 0) {
    return {
      title: `${t("金价", "Gold")} · ${unit}K（${t("元/克", "CNY/g")}）`,
      icon: "candlestick-chart",
      big: "--",
      sub: t("还没有录入", "Nothing recorded yet"),
      lines: [t("在「编辑」页录入金价", "Record gold prices in Edit")],
      chart: null,
    };
  }
  const last = ticks[ticks.length - 1].v;
  const prev = ticks.length > 1 ? ticks[ticks.length - 2].v : last;
  const pct = prev === 0 ? 0 : ((last - prev) / prev) * 100;
  return {
    title: `${t("金价", "Gold")} · ${unit}K（${t("元/克", "CNY/g")}）`,
    icon: "candlestick-chart",
    big: last.toFixed(2),
    sub: `${t("较前一根", "vs previous")} ${pct >= 0 ? "+" : ""}${pct.toFixed(2)}%`,
    lines: [t("悬浮查看每根的开高低收", "Hover a candle for its open, high, low and close")],
    chart: { type: "candle", ticks: ticks, period: period, limit: 60 },
  };
}
