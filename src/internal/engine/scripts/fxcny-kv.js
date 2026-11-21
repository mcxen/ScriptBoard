// 兑人民币牌价表：复制自兑人民币，改用状态键值面板（kv）清晰展示各币种精确 4 位汇率。
// 离线或失败时不造数。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

const PAIRS = [
  { code: "USD", unit: 1, zh: "美元", en: "US Dollar", color: "#3B82F6" },
  { code: "EUR", unit: 1, zh: "欧元", en: "Euro", color: "#8B5CF6" },
  { code: "JPY", unit: 100, zh: "日元 ×100", en: "Yen ×100", color: "#F59E0B" },
  { code: "GBP", unit: 1, zh: "英镑", en: "British Pound", color: "#EC4899" },
  { code: "HKD", unit: 1, zh: "港币", en: "HK Dollar", color: "#10B981" },
];

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("兑人民币牌价表", "FX vs CNY board"),
    description: t("用键值牌价面板展示美元、欧元、日元、英镑、港币兑人民币参考汇率", "Key-value reference rate board for major currencies vs CNY"),
    version: "1.0.0",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    icon: "globe",
    sources: {
      fx: {
        url: "https://api.frankfurter.dev/v1/latest?from=CNY&to=USD,EUR,JPY,GBP,HKD",
        fallbacks: ["https://api.frankfurter.app/latest?from=CNY&to=USD,EUR,JPY,GBP,HKD"],
      },
    },
  };
}

function num(x) {
  const n = typeof x === "number" ? x : parseFloat(x);
  return n === n ? n : 0;
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const title = t("兑人民币牌价表", "FX vs CNY board");
  const src = ctx.sources && ctx.sources.fx;
  if (!src || src.error) {
    const why = src && src.error ? String(src.error) : t("无数据", "no data");
    return {
      title: title,
      icon: "globe",
      big: "--",
      sub: t("未能取到汇率：", "Could not fetch rates: ") + why,
      lines: [t("需要网络才能取今日参考汇率", "Needs the network for reference rates")],
      chart: null,
    };
  }
  const rates = src.rates || {};
  const kv = [];
  let headVal = "";
  let headSub = "";
  for (let i = 0; i < PAIRS.length; i++) {
    const p = PAIRS[i];
    const r = num(rates[p.code]);
    if (r <= 0) continue;
    const v = p.unit / r;
    const valStr = v.toFixed(4);
    if (!headVal) {
      headVal = valStr;
      headSub = t(p.zh, p.en) + (src.date ? " · " + String(src.date) : "");
    }
    kv.push({
      label: t(p.zh, p.en),
      value: valStr,
      sub: p.unit === 1 ? `1 ${p.code} → CNY` : `${p.unit} ${p.code} → CNY`,
      badge: p.code,
      color: p.color,
    });
  }
  if (kv.length === 0) {
    return {
      title: title,
      icon: "globe",
      big: "--",
      sub: t("接口没有返回汇率", "The feed returned no rates"),
      lines: [],
      chart: null,
    };
  }
  return {
    title: title,
    icon: "globe",
    big: headVal,
    sub: headSub,
    lines: [t("欧洲央行参考汇率", "ECB reference rates")],
    chart: { type: "kv", kv: kv },
    layout: { legend: "hide" },
  };
}
