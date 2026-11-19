// 空气质量：所选城市的 PM2.5 与美标 AQI，来自 Open-Meteo（无需 key）。离线或失败时不造数。
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
    name: t("空气质量", "Air quality"),
    description: t("所选城市的 PM2.5 与美标 AQI", "PM2.5 and US AQI for a city"),
    version: "1.1.0",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    icon: "wind",
    vars: [
      {
        key: "city",
        label: t("城市", "City"),
        default: CITIES[0].value,
        options: CITIES.map((c) => ({ value: c.value, label: t(c.zh, c.en) })),
      },
    ],
    sources: {
      air: "https://air-quality-api.open-meteo.com/v1/air-quality?{city}&current=pm2_5,pm10,us_aqi&hourly=pm2_5&timezone=auto&past_days=2&forecast_days=1",
    },
  };
}

function num(x) {
  const n = typeof x === "number" ? x : parseFloat(x);
  return n === n ? n : 0;
}

function hourLabel(s) {
  const v = String(s || "");
  if (v.length >= 16) return v.slice(5, 16).replace("T", " ");
  return v;
}

function aqiName(v) {
  if (v <= 50) return t("优", "Good");
  if (v <= 100) return t("良", "Moderate");
  if (v <= 150) return t("轻度污染", "Unhealthy for sensitive");
  if (v <= 200) return t("中度污染", "Unhealthy");
  if (v <= 300) return t("重度污染", "Very unhealthy");
  return t("严重污染", "Hazardous");
}

function hoursOf(src, endTime, keep) {
  const h = src && src.hourly;
  if (!h || !h.time || !h.pm2_5) return { times: [], vals: [] };
  let end = h.time.length;
  if (endTime) {
    for (let i = 0; i < h.time.length; i++) {
      if (h.time[i] === endTime) {
        end = i + 1;
        break;
      }
    }
  }
  const start = keep > 0 && end > keep ? end - keep : 0;
  const times = [];
  const vals = [];
  for (let i = start; i < end; i++) {
    const v = h.pm2_5[i];
    if (typeof v !== "number") continue;
    times.push(h.time[i]);
    vals.push(v);
  }
  return { times: times, vals: vals };
}

function empty(title, why, lines) {
  return {
    title: title,
    icon: "wind",
    big: "--",
    sub: why,
    lines: lines || [],
    chart: null,
  };
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const city = CITIES.find((c) => c.value === ctx.vars.city) || CITIES[0];
  const place = t(city.zh, city.en);
  const title = t(place + "空气 · PM2.5", place + " air · PM2.5");
  const src = ctx.sources && ctx.sources.air;
  if (!src || src.error) {
    const why = src && src.error ? String(src.error) : t("无数据", "no data");
    return empty(title, t("未能取到空气质量：", "Could not fetch air quality: ") + why, [
      t("需要网络才能取上海空气质量", "Needs the network for Shanghai air quality"),
    ]);
  }
  const cur = src.current || {};
  const endTime = cur.time ? String(cur.time) : "";
  const hrs = hoursOf(src, endTime, 48);
  let pm = num(cur.pm2_5);
  if (pm <= 0 && hrs.vals.length) pm = hrs.vals[hrs.vals.length - 1];
  const aqi = num(cur.us_aqi);
  if (pm <= 0 && aqi <= 0) {
    return empty(title, t("接口没有返回空气质量", "The feed returned no air quality"), []);
  }
  const lines = [];
  if (aqi > 0) lines.push("AQI " + Math.round(aqi) + " · " + aqiName(aqi));
  const pm10 = num(cur.pm10);
  if (pm10 > 0) lines.push("PM10 " + pm10.toFixed(1));
  if (endTime) lines.push(hourLabel(endTime));
  const chart = hrs.vals.length
    ? {
        type: "line",
        series: [{ name: "PM2.5", color: "#A1887F", points: hrs.vals }],
        labels: hrs.times.map(hourLabel),
        xLabels: [hourLabel(hrs.times[0]), hourLabel(hrs.times[hrs.times.length - 1])],
      }
    : null;
  return {
    title: title,
    icon: "wind",
    big: pm.toFixed(1),
    sub: t("微克/立方米", "µg/m³"),
    lines: lines,
    chart: chart,
  };
}
