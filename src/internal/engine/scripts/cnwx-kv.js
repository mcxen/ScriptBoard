// 国内实况面板：复制自国内实况，改用状态键值面板（kv）一屏直读天气、风力、湿度、AQI、气压与日出日落。
// 离线或失败时不造数。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

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
  "07": ["小雨", "Light rain", "cloud-rain"],
  "08": ["中雨", "Rain", "cloud-rain"],
  "09": ["大雨", "Heavy rain", "cloud-rain"],
  "10": ["暴雨", "Downpour", "cloud-rain"],
  "18": ["雾", "Fog", "cloud-sun"],
  "53": ["霾", "Haze", "wind"],
};

const WIND = { "北": "N", "东北": "NE", "东": "E", "东南": "SE", "南": "S", "西南": "SW", "西": "W", "西北": "NW" };

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("国内实况", "China weather"),
    description: t("用状态面板一屏展示城市实况天气、湿度、AQI、气压与日出日落", "Status panel of live weather, humidity, AQI, pressure and sun times"),
    version: "1.1.0",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    icon: "sun",
    menubar: true,
    vars: [
      {
        key: "city",
        label: t("城市", "City"),
        default: CITIES[0].value,
        options: CITIES.map((c) => ({ value: c.value, label: t(c.zh, c.en) })),
      },
    ],
    sources: {
      now: {
        url: "https://60s.viki.moe/v2/weather?query={city}",
        fallbacks: ["https://60s.crystelf.top/v2/weather?query={city}"],
      },
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

function windText(dir, power) {
  const raw = String(dir || "");
  const d = EN ? WIND[raw.replace("风", "")] || "wind" : raw;
  const p = power ? String(power) + t("级", "") : "";
  return d && p ? d + " " + p : d || p;
}

function aqiInfo(v) {
  if (v <= 50) return { label: t("优", "Good"), status: "ok" };
  if (v <= 100) return { label: t("良", "Moderate"), status: "info" };
  if (v <= 150) return { label: t("轻度污染", "Sensitive"), status: "warn" };
  return { label: t("污染", "Unhealthy"), status: "err" };
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const city = CITIES.find((c) => c.value === (ctx.vars && ctx.vars.city)) || CITIES[0];
  const place = t(city.zh, city.en);
  const title = t("国内实况面板 · ", "Weather panel · ") + place;
  const src = ctx.sources && ctx.sources.now;
  if (!src || src.error) {
    const why = src && src.error ? String(src.error) : t("无数据", "no data");
    return {
      title: title,
      icon: "sun",
      big: "--",
      sub: t("未能取到实况：", "Could not fetch conditions: ") + why,
      lines: [t("需要网络才能取国内天气实况", "Needs the network for live weather")],
      chart: null,
    };
  }
  const pack = packOf(src);
  const wx = pack && pack.weather;
  const temp = wx ? num(wx.temperature) : null;
  if (!wx || temp === null) {
    return {
      title: title,
      icon: "sun",
      big: "--",
      sub: t("接口没有返回实况", "The feed returned no conditions"),
      lines: [],
      chart: null,
    };
  }

  const sky = skyOf(wx.condition_code, wx.condition);
  const wind = windText(wx.wind_direction, wx.wind_power);
  const air = (pack && pack.air_quality) || {};
  const aqi = num(air.aqi);
  const humidity = num(wx.humidity);
  const precip = num(wx.precipitation);
  const pressure = num(wx.pressure);
  const sun = pack && pack.sunrise;

  const kv = [];
  if (aqi !== null) {
    const ai = aqiInfo(aqi);
    kv.push({ label: t("空气质量", "Air quality"), value: String(Math.round(aqi)), badge: ai.label, status: ai.status });
  }
  if (humidity !== null) {
    kv.push({ label: t("相对湿度", "Humidity"), value: Math.round(humidity) + "%", status: "info" });
  }
  if (wind) {
    kv.push({ label: t("风向风力", "Wind"), value: wind, status: "info" });
  }
  if (precip !== null) {
    kv.push({ label: t("降水量", "Precipitation"), value: precip + " mm", status: precip > 0 ? "warn" : "ok" });
  }
  if (pressure !== null) {
    kv.push({ label: t("大气压", "Pressure"), value: Math.round(pressure) + " hPa", status: "info" });
  }
  if (sun && (sun.sunrise_desc || sun.sunset_desc)) {
    kv.push({ label: t("日出 / 日落", "Sun"), value: (sun.sunrise_desc || "--") + " / " + (sun.sunset_desc || "--"), color: "#F59E0B" });
  }

  return {
    title: title,
    icon: sky.icon,
    big: Math.round(temp) + "°",
    sub: place + " · " + sky.text + (wind ? " · " + wind : ""),
    lines: wx.updated ? [String(wx.updated)] : [],
    menubar: {
      title: Math.round(temp) + "°",
      tooltip: place + " · " + sky.text + " · " + Math.round(temp) + "°",
    },
    chart: { type: "kv", kv: kv },
    layout: { legend: "hide" },
  };
}
