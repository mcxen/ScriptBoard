// 国内实况：所选城市此刻的气温、风力与空气质量，来自 60s（腾讯天气包装，无需 key）。
// 城市名要做百分号编码：60s 对未编码的中文 query 返回 HTTP 400。
// 离线或失败时不造数。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

// value 是 URL 编码后的城市名，填进地址的 {city}。
const CITIES = [
  { value: "%E4%B8%8A%E6%B5%B7", zh: "上海", en: "Shanghai" },
  { value: "%E5%8C%97%E4%BA%AC", zh: "北京", en: "Beijing" },
  { value: "%E5%B9%BF%E5%B7%9E", zh: "广州", en: "Guangzhou" },
  { value: "%E6%B7%B1%E5%9C%B3", zh: "深圳", en: "Shenzhen" },
  { value: "%E6%9D%AD%E5%B7%9E", zh: "杭州", en: "Hangzhou" },
  { value: "%E6%88%90%E9%83%BD", zh: "成都", en: "Chengdu" },
];

const SKY = {
  "00": ["晴", "Clear", "sun"],
  "01": ["多云", "Cloudy", "cloud-sun"],
  "02": ["阴", "Overcast", "cloud-sun"],
  "03": ["阵雨", "Showers", "cloud-rain"],
  "04": ["雷阵雨", "Thunderstorms", "zap"],
  "05": ["冰雹", "Hail", "zap"],
  "06": ["雨夹雪", "Sleet", "cloud-rain"],
  "07": ["小雨", "Light rain", "cloud-rain"],
  "08": ["中雨", "Rain", "cloud-rain"],
  "09": ["大雨", "Heavy rain", "cloud-rain"],
  "10": ["暴雨", "Downpour", "cloud-rain"],
  "13": ["阵雪", "Snow showers", "cloud-rain"],
  "14": ["小雪", "Light snow", "cloud-rain"],
  "15": ["中雪", "Snow", "cloud-rain"],
  "16": ["大雪", "Heavy snow", "cloud-rain"],
  "18": ["雾", "Fog", "cloud-sun"],
  "19": ["冻雨", "Freezing rain", "cloud-rain"],
  "20": ["沙尘暴", "Sandstorm", "wind"],
  "29": ["浮尘", "Dust", "wind"],
  "30": ["扬沙", "Blowing sand", "wind"],
  "53": ["霾", "Haze", "wind"],
  "301": ["雨", "Rain", "cloud-rain"],
  "302": ["雪", "Snow", "cloud-rain"],
};

const WIND = {
  "北": "N",
  "东北": "NE",
  "东": "E",
  "东南": "SE",
  "南": "S",
  "西南": "SW",
  "西": "W",
  "西北": "NW",
};

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("国内实况", "China now"),
    description: t("所选城市的气温、风力与空气质量", "Temperature, wind and air quality for a Chinese city"),
    version: "1.0.0",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    icon: "sun",
    vars: [
      {
        key: "city",
        label: t("城市", "City"),
        default: CITIES[0].value,
        options: CITIES.map((c) => ({ value: c.value, label: t(c.zh, c.en) })),
      },
    ],
    sources: {
      now: "https://60s.viki.moe/v2/weather?query={city}",
      fc: "https://60s.viki.moe/v2/weather/forecast?query={city}&days=7",
    },
  };
}

function num(x) {
  if (typeof x === "number" && x === x) return x;
  const n = parseFloat(x);
  return n === n ? n : null;
}

function packOf(src) {
  if (!src || src.error) return null;
  if (typeof src.code === "number" && src.code !== 200) return null;
  return src.data || src;
}

function skyOf(code, raw) {
  const key = String(code == null ? "" : code);
  const row = SKY[key] || SKY[key.length === 1 ? "0" + key : key];
  if (row) return { text: t(row[0], row[1]), icon: row[2] };
  if (!EN && raw) return { text: String(raw), icon: "sun" };
  return { text: t("未知", "Unknown"), icon: "sun" };
}

function dirText(s) {
  const raw = String(s || "");
  if (!raw) return "";
  if (!EN) return raw;
  const v = raw.replace("风", "");
  return WIND[v] || "wind";
}

function windText(dir, power) {
  const d = dirText(dir);
  const p = power ? String(power) : "";
  const right = p ? p + t("级", "") : "";
  if (d && right) return d + " " + right;
  return d || right;
}

function aqiName(v) {
  if (v === null) return "";
  if (v <= 50) return t("优", "Good");
  if (v <= 100) return t("良", "Moderate");
  if (v <= 150) return t("轻度污染", "Lightly polluted");
  if (v <= 200) return t("中度污染", "Moderately polluted");
  if (v <= 300) return t("重度污染", "Heavily polluted");
  return t("严重污染", "Severely polluted");
}

function hourLabel(s) {
  const v = String(s || "");
  if (v.length >= 16) return v.slice(5, 16);
  return v;
}

function hoursOf(src, keep) {
  const pack = packOf(src);
  const list = pack && pack.hourly_forecast;
  if (!list || !list.length) return { times: [], vals: [] };
  const n = keep > 0 && list.length > keep ? keep : list.length;
  const times = [];
  const vals = [];
  for (let i = 0; i < n; i++) {
    const row = list[i];
    if (!row) continue;
    const v = num(row.temperature);
    if (v === null) continue;
    times.push(row.datetime);
    vals.push(v);
  }
  return { times: times, vals: vals };
}

function empty(title, why, lines) {
  return {
    title: title,
    icon: "sun",
    big: "--",
    sub: why,
    lines: lines || [],
    chart: null,
    stats: [],
  };
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const city = CITIES.find((c) => c.value === ctx.vars.city) || CITIES[0];
  const place = t(city.zh, city.en);
  const title = t("国内实况 · ", "China now · ") + place;
  const src = ctx.sources && ctx.sources.now;
  if (!src || src.error) {
    const why = src && src.error ? String(src.error) : t("无数据", "no data");
    return empty(title, t("未能取到实况：", "Could not fetch the conditions: ") + why, [
      t("需要网络才能取国内天气实况", "Needs the network for live weather in China"),
    ]);
  }
  const pack = packOf(src);
  const wx = pack && pack.weather;
  const temp = wx ? num(wx.temperature) : null;
  if (!wx || temp === null) {
    return empty(title, t("接口没有返回实况", "The feed returned no conditions"), []);
  }
  const sky = skyOf(wx.condition_code, wx.condition);
  const wind = windText(wx.wind_direction, wx.wind_power);
  const air = (pack && pack.air_quality) || {};
  const aqi = num(air.aqi);
  const humidity = num(wx.humidity);
  const precip = num(wx.precipitation);
  const pressure = num(wx.pressure);
  const stats = [];
  if (humidity !== null) stats.push({ label: t("湿度", "Humidity"), value: Math.round(humidity) + "%", sub: "" });
  if (precip !== null) stats.push({ label: t("降水", "Precip"), value: String(precip), sub: "mm" });
  if (aqi !== null) stats.push({ label: "AQI", value: String(Math.round(aqi)), sub: aqiName(aqi) });
  if (pressure !== null) stats.push({ label: t("气压", "Pressure"), value: String(Math.round(pressure)), sub: "hPa" });
  const lines = [];
  const sun = pack && pack.sunrise;
  if (sun && (sun.sunrise_desc || sun.sunset_desc)) {
    lines.push(t("日出 ", "Sunrise ") + (sun.sunrise_desc || "--") + t(" · 日落 ", " · sunset ") + (sun.sunset_desc || "--"));
  }
  if (wx.updated) lines.push(String(wx.updated));
  const fc = ctx.sources && ctx.sources.fc;
  const hrs = fc && !fc.error ? hoursOf(fc, 24) : { times: [], vals: [] };
  const chart = hrs.vals.length
    ? {
        type: "line",
        series: [{ name: t("气温", "Temperature"), color: "#F97316", points: hrs.vals }],
        labels: hrs.times.map(hourLabel),
        xLabels: [hourLabel(hrs.times[0]), hourLabel(hrs.times[hrs.times.length - 1])],
      }
    : null;
  const bits = [sky.text];
  if (wind) bits.push(wind);
  return {
    title: title,
    icon: sky.icon,
    big: Math.round(temp) + "°",
    sub: bits.join(" · "),
    lines: lines,
    chart: chart,
    stats: stats,
  };
}
