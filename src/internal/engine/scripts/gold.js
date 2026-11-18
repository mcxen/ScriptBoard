// 金价走势：读取 ctx.metrics.gold，按录入时间画折线（元/克）。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("金价走势", "Gold price"),
    icon: "coins",
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
function hhmm(t, off) {
  const s = (((t + off * 1000) % 86400000) + 86400000) % 86400000;
  return pad(Math.floor(s / 3600000)) + ":" + pad(Math.floor((s % 3600000) / 60000));
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const days = ctx.view.range ?? 0;
  const since = days > 0 ? ctx.now - days * 86400000 : 0;
  const pts = (ctx.metrics.gold ?? [])
    .slice()
    .sort((a, b) => a.t - b.t)
    .filter((p) => p.t >= since);
  if (pts.length === 0) {
    return {
      title: t("金价 · 元/克", "Gold · CNY/g"),
      icon: "coins",
      big: "--",
      sub: t("还没有录入", "Nothing recorded yet"),
      lines: [t("在「编辑」页录入金价", "Record gold prices in Edit")],
      chart: null,
    };
  }
  const off = ctx.tzOffset;
  const recent = pts.slice(-60);
  const v = recent.map((p) => p.v);
  const labels = recent.map((p) => mmdd(dayIndex(p.t, off)) + " " + hhmm(p.t, off));
  const last = v[v.length - 1];
  const first = v[0];
  const pct = first === 0 ? 0 : ((last - first) / first) * 100;
  return {
    title: t("金价 · 元/克", "Gold · CNY/g"),
    icon: "coins",
    big: last.toFixed(2),
    sub: `${t("区间", "Range")} ${pct >= 0 ? "+" : ""}${pct.toFixed(2)}%`,
    lines: [EN ? `${pts.length} records` : `共 ${pts.length} 条记录`],
    chart: {
      type: "line",
      series: [{ name: t("金价", "Gold"), color: "#FFD54F", points: v }],
      labels: labels,
      xLabels: [mmdd(dayIndex(recent[0].t, off)), mmdd(dayIndex(recent[recent.length - 1].t, off))],
    },
  };
}
