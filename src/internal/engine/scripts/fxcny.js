// 兑人民币：当日美元、欧元、日元、英镑、港币兑人民币，来自 frankfurter（无需 key）。
// 日元按 100 日元报价，和银行牌价一样。离线或失败时不造数。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

const PAIRS = [
  { code: "USD", unit: 1, zh: "美元", en: "USD" },
  { code: "EUR", unit: 1, zh: "欧元", en: "EUR" },
  { code: "JPY", unit: 100, zh: "日元×100", en: "JPY×100" },
  { code: "GBP", unit: 1, zh: "英镑", en: "GBP" },
  { code: "HKD", unit: 1, zh: "港币", en: "HKD" },
];

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("兑人民币", "vs CNY"),
    description: t("美元、欧元、日元、英镑、港币兑人民币", "Dollar, euro, yen, pound and Hong Kong dollar in yuan"),
    version: "1.0.0",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    icon: "globe",
    sources: {
      fx: "https://api.frankfurter.dev/v1/latest?from=CNY&to=USD,EUR,JPY,GBP,HKD",
    },
  };
}

function num(x) {
  const n = typeof x === "number" ? x : parseFloat(x);
  return n === n ? n : 0;
}

function rowsOf(rates) {
  const out = [];
  if (!rates) return out;
  for (let i = 0; i < PAIRS.length; i++) {
    const p = PAIRS[i];
    const r = num(rates[p.code]);
    if (r <= 0) continue;
    out.push({
      code: p.code,
      name: t(p.zh, p.en),
      v: p.unit / r,
    });
  }
  return out;
}

function empty(title, why, lines) {
  return {
    title: title,
    icon: "globe",
    big: "--",
    sub: why,
    lines: lines || [],
    chart: null,
  };
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const title = t("兑人民币 · 今日", "vs CNY · today");
  const src = ctx.sources && ctx.sources.fx;
  if (!src || src.error) {
    const why = src && src.error ? String(src.error) : t("无数据", "no data");
    return empty(title, t("未能取到汇率：", "Could not fetch the rate: ") + why, [
      t("需要网络才能取今日参考汇率", "Needs the network for today's reference rates"),
    ]);
  }
  const rows = rowsOf(src.rates);
  if (rows.length === 0) {
    return empty(title, t("接口没有返回汇率", "The feed returned no rates"), []);
  }
  const head = rows[0];
  const lines = [];
  if (src.date) lines.push(String(src.date));
  lines.push(t("欧洲央行参考汇率", "ECB reference rates"));
  return {
    title: title,
    icon: "globe",
    big: head.v.toFixed(4),
    sub: head.name,
    lines: lines,
    chart: {
      type: "barh",
      series: [{ name: t("兑人民币", "vs CNY"), color: "#10B981", points: rows.map((r) => r.v) }],
      labels: rows.map((r) => r.name),
    },
  };
}
