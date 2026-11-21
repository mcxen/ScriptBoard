// 积存金行情板：复制自积存金，改用状态键值面板（kv）直观呈现各银行实时报价与涨跌幅。
// 离线或失败时不造数。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

const BANKS = [
  { key: "icbc", zh: "工商银行", en: "ICBC" },
  { key: "ms", zh: "民生银行", en: "CMBC" },
  { key: "zs", zh: "浙商银行", en: "CZB" },
  { key: "cib", zh: "兴业银行", en: "CIB" },
  { key: "cgb", zh: "广发银行", en: "CGB" },
  { key: "jd", zh: "京东黄金", en: "JD Gold" },
];

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("积存金行情板", "Bank gold board"),
    description: t("用状态面板展示各银行积存金实时价与涨跌幅", "Status panel of live bank gold prices and daily changes"),
    version: "1.0.0",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    icon: "hand-coins",
    sources: {
      icbc: "https://jin.20021002.xyz/api.php?type=icbc",
      ms: "https://jin.20021002.xyz/api.php?type=ms",
      zs: "https://jin.20021002.xyz/api.php?type=zs",
      cib: "https://jin.20021002.xyz/api.php?type=cib",
      cgb: "https://jin.20021002.xyz/api.php?type=cgb",
      jd: "https://ms.jr.jd.com/gw2/generic/hj/h5/m/latestPrice",
    },
  };
}

function num(x) {
  const n = typeof x === "number" ? x : parseFloat(x);
  return n === n ? n : 0;
}

function quote(src) {
  if (!src || src.error) return null;
  if (src.data && src.data.price != null) {
    const p = num(src.data.price);
    if (p <= 0) return null;
    return {
      price: p,
      change: num(src.data.change),
      pct: num(src.data.change_pct),
      time: String(src.data.update_time || ""),
    };
  }
  const d = src.resultData && src.resultData.datas;
  if (!d || d.price == null) return null;
  const p = num(d.price);
  if (p <= 0) return null;
  return {
    price: p,
    change: num(d.upAndDownAmt),
    pct: num(d.upAndDownRate),
    time: "",
  };
}

function signed(n, digits) {
  const s = n.toFixed(digits);
  return n > 0 ? "+" + s : s;
}

function firstError(sources) {
  for (let i = 0; i < BANKS.length; i++) {
    const s = sources[BANKS[i].key];
    if (s && s.error) return String(s.error);
  }
  return t("无数据", "no data");
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const title = t("积存金行情板 · 元/克", "Bank gold board · CNY/g");
  const sources = ctx.sources || {};
  const rows = [];

  for (let i = 0; i < BANKS.length; i++) {
    const b = BANKS[i];
    const q = quote(sources[b.key]);
    if (!q) continue;
    let status = "info";
    let badge = "";
    if (q.pct > 0.005 || q.change > 0) {
      status = "err"; // red for rising in CN market
      badge = signed(q.pct, 2) + "%";
    } else if (q.pct < -0.005 || q.change < 0) {
      status = "ok"; // green for falling in CN market
      badge = signed(q.pct, 2) + "%";
    }
    rows.push({
      label: t(b.zh, b.en),
      value: q.price.toFixed(2),
      sub: q.time || (q.change ? signed(q.change, 2) + t(" 元/克", " CNY/g") : ""),
      badge: badge,
      status: status,
      price: q.price,
    });
  }

  if (rows.length === 0) {
    return {
      title: title,
      icon: "hand-coins",
      big: "--",
      sub: t("未能取到积存金：", "Could not fetch bank gold: ") + firstError(sources),
      lines: [t("需要网络才能取各行积存金报价", "Needs the network for bank gold prices")],
      chart: null,
    };
  }

  const head = rows[0];
  return {
    title: title,
    icon: "hand-coins",
    big: head.price.toFixed(2),
    sub: head.label + " · " + (EN ? rows.length + " banks" : "共 " + rows.length + " 家"),
    lines: [],
    chart: {
      type: "kv",
      kv: rows.map((r) => ({
        label: r.label,
        value: r.value,
        sub: r.sub,
        badge: r.badge,
        status: r.status,
      })),
    },
    layout: { legend: "hide" },
  };
}
