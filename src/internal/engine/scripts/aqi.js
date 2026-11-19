// 上海空气：Open-Meteo 空气质量（PM2.5 与美标 AQI），无需 key。离线或失败时不造数。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("上海空气", "Shanghai air"),
    description: t("上海的 PM2.5 与美标 AQI", "Shanghai PM2.5 and US AQI"),
    version: "1.0.0",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    icon: "wind",
    sources: {
      air: "https://air-quality-api.open-meteo.com/v1/air-quality?latitude=31.23&longitude=121.47&current=pm2_5,pm10,us_aqi&hourly=pm2_5&timezone=Asia%2FShanghai&past_days=2&forecast_days=1",
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
  const title = t("上海空气 · PM2.5", "Shanghai air · PM2.5");
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
