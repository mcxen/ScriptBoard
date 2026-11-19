// 积存金：国内银行积存金实时价（元/克），无需 key。
// 接口：京东金融官方 latestPrice；GoldMonitor 聚合 type=icbc/ms/zs/cib/cgb；
// 工行当日走势 action=chart&type=icbc。离线或失败时不造数。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

const BANKS = [
  { key: "icbc", zh: "工行", en: "ICBC", fullZh: "工商银行", fullEn: "ICBC" },
  { key: "ms", zh: "民生", en: "CMBC", fullZh: "民生银行", fullEn: "CMBC" },
  { key: "zs", zh: "浙商", en: "CZB", fullZh: "浙商银行", fullEn: "CZB" },
  { key: "cib", zh: "兴业", en: "CIB", fullZh: "兴业银行", fullEn: "CIB" },
  { key: "cgb", zh: "广发", en: "CGB", fullZh: "广发银行", fullEn: "CGB" },
  { key: "jd", zh: "京东", en: "JD", fullZh: "京东黄金", fullEn: "JD gold" },
];

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("积存金", "Bank gold"),
    description: t("银行积存金的实时价", "Live prices of banks' gold accumulation plans"),
    version: "1.1.0",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    icon: "hand-coins",
    dims: [
      {
        key: "kind",
        label: t("种类", "Kind"),
        options: [
          { value: 0, label: t("各行对比", "Banks") },
          { value: 1, label: t("工行当日", "ICBC today") },
        ],
      },
    ],
    sources: {
      icbc: "https://jin.20021002.xyz/api.php?type=icbc",
      ms: "https://jin.20021002.xyz/api.php?type=ms",
      zs: "https://jin.20021002.xyz/api.php?type=zs",
      cib: "https://jin.20021002.xyz/api.php?type=cib",
      cgb: "https://jin.20021002.xyz/api.php?type=cgb",
      jd: "https://ms.jr.jd.com/gw2/generic/hj/h5/m/latestPrice",
      ticks: "https://jin.20021002.xyz/api.php?action=chart&type=icbc",
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
      name: String(src.data.name || ""),
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
    name: t("京东黄金", "JD gold"),
    price: p,
    change: num(d.upAndDownAmt),
    pct: num(d.upAndDownRate),
    time: "",
  };
}

function firstError(sources) {
  const keys = ["icbc", "ms", "zs", "cib", "cgb", "jd", "ticks"];
  for (let i = 0; i < keys.length; i++) {
    const s = sources[keys[i]];
    if (s && s.error) return String(s.error);
  }
  return t("无数据", "no data");
}

function rowsOf(sources) {
  const out = [];
  for (let i = 0; i < BANKS.length; i++) {
    const b = BANKS[i];
    const q = quote(sources[b.key]);
    if (!q) continue;
    out.push({
      key: b.key,
      label: t(b.zh, b.en),
      full: q.name || t(b.fullZh, b.fullEn),
      price: q.price,
      change: q.change,
      pct: q.pct,
      time: q.time,
    });
  }
  return out;
}

function headline(rows) {
  for (let i = 0; i < rows.length; i++) {
    if (rows[i].key === "icbc") return rows[i];
  }
  return rows[0];
}

function ticksOf(src) {
  const list = src && src.data;
  if (!list || !list.length) return [];
  const out = [];
  for (let i = 0; i < list.length; i++) {
    const it = list[i];
    if (!it) continue;
    const p = num(it.p);
    const sec = num(it.t);
    if (p <= 0 || sec <= 0) continue;
    out.push({ sec: sec, p: p });
  }
  return out;
}

function downsample(pts, n) {
  if (pts.length <= n) return pts;
  const out = [];
  const last = pts.length - 1;
  let prev = -1;
  for (let i = 0; i < n; i++) {
    const idx = Math.round((i * last) / (n - 1));
    if (idx === prev) continue;
    prev = idx;
    out.push(pts[idx]);
  }
  return out;
}

function hhmm(sec, tz) {
  let x = Math.floor(sec + (tz || 0));
  x = ((x % 86400) + 86400) % 86400;
  const h = Math.floor(x / 3600);
  const m = Math.floor((x % 3600) / 60);
  const p = (n) => (n < 10 ? "0" : "") + n;
  return p(h) + ":" + p(m);
}

function signed(n, digits) {
  const s = n.toFixed(digits);
  return n > 0 ? "+" + s : s;
}

function empty(title, why, lines) {
  return {
    title: title,
    icon: "hand-coins",
    big: "--",
    sub: why,
    lines: lines || [],
    chart: null,
  };
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const title = t("积存金 · 元/克", "Bank gold · CNY/g");
  const sources = ctx.sources || {};
  const kind = ctx.view.kind ?? 0;

  if (kind === 1) {
    const pts = downsample(ticksOf(sources.ticks), 80);
    if (pts.length === 0) {
      const why = sources.ticks && sources.ticks.error
        ? t("未能取到工行走势：", "Could not fetch ICBC chart: ") + String(sources.ticks.error)
        : t("接口没有返回走势", "The feed returned no chart");
      return empty(title, why, [t("需要网络才能取工行当日积存金", "Needs the network for today's ICBC prices")]);
    }
    const last = pts[pts.length - 1];
    const first = pts[0];
    const ch = last.p - first.p;
    const labels = pts.map((p) => hhmm(p.sec, ctx.tzOffset));
    const icbc = quote(sources.icbc);
    const sub = icbc && icbc.name ? icbc.name : t("工商银行", "ICBC");
    const lines = [
      t("当日 ", "Today ") + signed(ch, 2) + t(" 元/克", " CNY/g"),
    ];
    if (icbc && icbc.time) lines.push(icbc.time);
    return {
      title: title,
      icon: "hand-coins",
      big: last.p.toFixed(2),
      sub: sub,
      lines: lines,
      chart: {
        type: "line",
        series: [{ name: t("工行", "ICBC"), color: "#EAB308", points: pts.map((p) => p.p) }],
        labels: labels,
        xLabels: [labels[0], labels[labels.length - 1]],
      },
    };
  }

  const rows = rowsOf(sources);
  if (rows.length === 0) {
    const why = t("未能取到积存金：", "Could not fetch bank gold: ") + firstError(sources);
    return empty(title, why, [t("需要网络才能取各行积存金报价", "Needs the network for bank gold prices")]);
  }
  const head = headline(rows);
  const lines = [t("共 ", "") + rows.length + t(" 家", " banks")];
  if (head.time) lines[0] = head.time + " · " + lines[0];
  if (head.change || head.pct) {
    lines.push(signed(head.change, 2) + " (" + signed(head.pct, 2) + "%)");
  }
  return {
    title: title,
    icon: "hand-coins",
    big: head.price.toFixed(2),
    sub: head.full,
    lines: lines,
    chart: {
      type: "barh",
      series: [{ name: t("积存金", "Bank gold"), color: "#EAB308", points: rows.map((r) => r.price) }],
      labels: rows.map((r) => r.label),
    },
  };
}
