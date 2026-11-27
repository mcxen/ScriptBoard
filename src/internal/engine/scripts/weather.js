// 七日天气：所选城市的最高、最低气温，来自 open-meteo（无需 key）。
// 网络失败时用示例数据，并在 lines 里标明。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
// 城市是 vars 里的一个变量：要加城市，在 CITIES 里加一行即可。
let EN = false;
const t = (zh, en) => (EN ? en : zh);

// value 是接口需要的坐标，直接填进地址的 {city} 位置。
const CITIES = [
  { value: "latitude=31.23&longitude=121.47", zh: "上海", en: "Shanghai" },
  { value: "latitude=39.90&longitude=116.41", zh: "北京", en: "Beijing" },
  { value: "latitude=23.13&longitude=113.26", zh: "广州", en: "Guangzhou" },
  { value: "latitude=22.54&longitude=114.06", zh: "深圳", en: "Shenzhen" },
  { value: "latitude=30.27&longitude=120.15", zh: "杭州", en: "Hangzhou" },
  { value: "latitude=30.57&longitude=104.07", zh: "成都", en: "Chengdu" },
];

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("七日天气", "7-day weather"),
    description: t("所选城市未来七日气温", "A city's temperatures for the next seven days"),
    version: "1.1.0",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    icon: "cloud-sun",
    vars: [
      {
        key: "city",
        label: t("城市", "City"),
        default: CITIES[0].value,
        options: CITIES.map((c) => ({ value: c.value, label: t(c.zh, c.en) })),
      },
    ],
    sources: {
      wx: "https://api.open-meteo.com/v1/forecast?{city}&daily=temperature_2m_max,temperature_2m_min&timezone=auto&forecast_days=7",
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
  const city = CITIES.find((c) => c.value === ctx.vars.city) || CITIES[0];
  const place = t(city.zh, city.en);
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
    title: t("七日天气 · ", "7-day weather · ") + place,
    icon: "cloud-sun",
    big: `${Math.round(hi)}°`,
    sub: `${t("今天", "Today")} ${Math.round(lo)}° ~ ${Math.round(hi)}°`,
    lines: note ? [note] : [],
    chart: {
      type: "line",
      series: [
        { name: t("最高", "High"), color: "#C26D2D", points: days.max },
        { name: t("最低", "Low"), color: "#4D7EA8", points: days.min },
      ],
      labels: days.time,
      xLabels: [String(days.time[0]), String(days.time[days.time.length - 1])],
    },
    layout: { title: "show" },
  };
}
