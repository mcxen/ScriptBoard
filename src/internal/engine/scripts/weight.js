// 体重记录：每次录入是一个点，用点状图展示；标签是录入的日期和时间。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("体重记录", "Weight log"),
    description: t("记录体重并画出变化", "Log your weight and chart it"),
    version: "1.1.0",
    platforms: ["desktop", "mobile"],
    needs: [],
    icon: "scale",
    inputs: [{ metric: "weight", label: t("体重", "Weight"), unit: "kg", step: 0.1 }],
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

// 本地日期与时间（由宿主给出时区偏移，不依赖 Date 的时区实现）。
function dayIndex(t, off) {
  return Math.floor((t + off * 1000) / 86400000);
}
function pad(n) {
  return (n < 10 ? "0" : "") + n;
}
// 从 1970-01-01 起的天数换算成 [月, 日]。
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
  const unit = ctx.settings.weightUnit === "斤" ? "斤" : "kg";
  const k = unit === "斤" ? 2 : 1;
  const days = ctx.view.range ?? 0;
  const since = days > 0 ? ctx.now - days * 86400000 : 0;
  const all = (ctx.metrics.weight ?? []).slice().sort((a, b) => a.t - b.t);
  const pts = all.filter((p) => p.t >= since);
  if (pts.length === 0) {
    return {
      title: t("体重", "Weight"),
      icon: "scale",
      big: "--",
      sub: t("还没有记录", "No records yet"),
      lines: [t("在「编辑」页录入第一条体重", "Record your first weight in Edit")],
      chart: null,
    };
  }
  const off = ctx.tzOffset;
  const recent = pts.slice(-40);
  const v = recent.map((p) => p.v * k);
  const labels = recent.map((p) => mmdd(dayIndex(p.t, off)) + " " + hhmm(p.t, off));
  const last = v[v.length - 1];
  const week = v[Math.max(0, v.length - 8)];
  const d = last - week;
  return {
    title: `${t("体重", "Weight")} · ${unit}`,
    icon: "scale",
    big: `${last.toFixed(1)} ${unit}`,
    sub: `${t("近一周", "Last week")} ${d >= 0 ? "+" : ""}${d.toFixed(1)} ${unit}`,
    lines: [EN ? `${pts.length} in range · ${all.length} records in all` : `范围内 ${pts.length} 次 · 共 ${all.length} 次记录`],
    chart: {
      type: "dot",
      series: [{ name: t("体重", "Weight"), color: "#3B82F6", points: v }],
      labels: labels,
      xLabels: [mmdd(dayIndex(recent[0].t, off)), mmdd(dayIndex(recent[recent.length - 1].t, off))],
    },
  };
}
