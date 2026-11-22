// 空气质量霓虹条：复制自空气质量，改用独立霓虹/分段进度条直出 US AQI、PM2.5、PM10，
// 并按空气质量等级自动切换绿 / 黄 / 橙 / 红霓虹色，无多余图表与详情控件。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

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
    description: t("用进度条展示所选城市的 US AQI、PM2.5 与 PM10 等级", "Progress bars for US AQI, PM2.5 and PM10"),
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
      air: "https://air-quality-api.open-meteo.com/v1/air-quality?{city}&current=pm2_5,pm10,us_aqi&timezone=auto",
    },
  };
}

function num(x) {
  const n = typeof x === "number" ? x : parseFloat(x);
  return n === n ? n : 0;
}

function levelInfo(aqi) {
  if (aqi <= 50) return { label: t("优", "Good"), color: "#00F5D4", to: "#22C55E" };
  if (aqi <= 100) return { label: t("良", "Moderate"), color: "#38BDF8", to: "#00F5D4" };
  if (aqi <= 150) return { label: t("轻度污染", "Sensitive"), color: "#F59E0B", to: "#FB923C" };
  if (aqi <= 200) return { label: t("中度污染", "Unhealthy"), color: "#FB923C", to: "#FF2A85" };
  return { label: t("重度污染", "Hazardous"), color: "#FF2A85", to: "#A855F7" };
}

function pmColor(v, good, warn) {
  if (v <= good) return ["#00F5D4", "#22C55E"];
  if (v <= warn) return ["#F59E0B", "#FB923C"];
  return ["#FF2A85", "#A855F7"];
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const city = CITIES.find((c) => c.value === (ctx.vars && ctx.vars.city)) || CITIES[0];
  const place = t(city.zh, city.en);
  const title = t(place + "空气 · 霓虹指标", place + " air · neon");
  const src = ctx.sources && ctx.sources.air;
  if (!src || src.error) {
    const why = src && src.error ? String(src.error) : t("无数据", "no data");
    return {
      title: title,
      icon: "wind",
      big: "--",
      sub: t("未能取到空气质量：", "Could not fetch air quality: ") + why,
      lines: [t("需要网络才能取实时空气质量", "Needs the network for live air quality")],
      chart: null,
    };
  }
  const cur = src.current || {};
  const aqi = num(cur.us_aqi);
  const pm25 = num(cur.pm2_5);
  const pm10 = num(cur.pm10);
  if (aqi <= 0 && pm25 <= 0 && pm10 <= 0) {
    return {
      title: title,
      icon: "wind",
      big: "--",
      sub: t("接口没有返回空气质量", "The feed returned no air quality"),
      lines: [],
      chart: null,
    };
  }

  const lv = levelInfo(aqi);
  const c25 = pmColor(pm25, 35, 75);
  const c10 = pmColor(pm10, 50, 150);
  const style = (ctx.view && ctx.view.style) === 1 ? "segmented" : "neon";
  const items = [];
  if (aqi > 0) {
    items.push({ name: "US AQI", value: Math.round(aqi), max: 300, unit: "", color: lv.color, to: lv.to });
  }
  if (pm25 > 0) {
    items.push({ name: "PM2.5", value: Math.round(pm25 * 10) / 10, max: 150, unit: " µg/m³", color: c25[0], to: c25[1] });
  }
  if (pm10 > 0) {
    items.push({ name: "PM10", value: Math.round(pm10 * 10) / 10, max: 250, unit: " µg/m³", color: c10[0], to: c10[1] });
  }

  return {
    title: title,
    icon: "wind",
    big: aqi > 0 ? "AQI " + Math.round(aqi) : pm25.toFixed(1),
    sub: place + " · " + lv.label,
    lines: cur.time ? [String(cur.time).replace("T", " ")] : [],
    progressBar: {
      style: style,
      height: 10,
      track: "dark",
      glow: 1.0,
      segments: 12,
      items: items,
    },
    layout: { legend: "hide" },
  };
}
