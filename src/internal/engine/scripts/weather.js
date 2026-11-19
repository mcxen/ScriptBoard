// 七日天气：上海最高、最低气温，来自 open-meteo（无需 key）。
// 网络失败时用示例数据，并在 lines 里标明。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("七日天气 · 上海", "7-day weather · Shanghai"),
    description: t("上海未来七日气温", "Shanghai's temperatures for the next seven days"),
    version: "1.0.0",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    icon: "cloud-sun",
    sources: {
      wx: "https://api.open-meteo.com/v1/forecast?latitude=31.23&longitude=121.47&daily=temperature_2m_max,temperature_2m_min&timezone=auto&forecast_days=7",
    },
  };
}

// 缺失值（null）沿用前一天，保证两条线长度一致。
function filled(arr) {
  const out = [];
  let prev = 0;
  for (let i = 0; i < arr.length; i++) {
    const x = arr[i];
    if (typeof x === "number") prev = x;
    out.push(prev);
  }
  return out;
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const src = ctx.sources.wx;
  let days = null;
  let note = "";
  if (src && !src.error && src.daily && src.daily.time) {
    days = {
      time: src.daily.time,
      max: filled(src.daily.temperature_2m_max),
      min: filled(src.daily.temperature_2m_min),
    };
  } else {
    note = t("示例数据：", "Sample data: ") + (src && src.error ? src.error : t("无数据", "no data"));
    days = {
      time: ["11-18", "11-19", "11-20", "11-21", "11-22", "11-23", "11-24"],
      max: [18, 19, 21, 22, 20, 17, 16],
      min: [11, 12, 13, 14, 12, 10, 9],
    };
  }
  const hi = days.max[0];
  const lo = days.min[0];
  return {
    title: t("七日天气 · 上海", "7-day weather · Shanghai"),
    icon: "cloud-sun",
    big: `${Math.round(hi)}°`,
    sub: `${t("今天", "Today")} ${Math.round(lo)}° ~ ${Math.round(hi)}°`,
    lines: note ? [note] : [],
    chart: {
      type: "line",
      series: [
        { name: t("最高", "High"), color: "#FF8A65", points: days.max },
        { name: t("最低", "Low"), color: "#64B5F6", points: days.min },
      ],
      labels: days.time,
      xLabels: [String(days.time[0]), String(days.time[days.time.length - 1])],
    },
  };
}
