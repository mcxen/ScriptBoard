// 汇率曲线：近 30 天所选货币兑人民币的汇率，来自 frankfurter（无需 key）。
// 网络失败时：美元用示例数据，并在 lines 里标明；其它货币不造数。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
// 货币是 vars 里的一个变量：要加货币，在 BASES 里加一行即可。
let EN = false;
const t = (zh, en) => (EN ? en : zh);

const BASES = ["USD", "EUR", "JPY", "HKD", "GBP", "KRW", "AUD", "SGD"];

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("汇率", "Exchange rate"),
    description: t("外币兑人民币汇率走势", "A foreign currency to yuan exchange rate"),
    version: "1.1.0",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    icon: "dollar-sign",
    dims: [
      {
        key: "range",
        label: t("范围", "Range"),
        options: [
          { value: 7, label: t("7 天", "7 days") },
          { value: 30, label: t("30 天", "30 days") },
          { value: 0, label: t("全部", "All") },
        ],
      },
    ],
    vars: [
      {
        key: "base",
        label: t("货币", "Currency"),
        default: "USD",
        options: BASES.map((b) => ({ value: b, label: b + " / CNY" })),
      },
    ],
    sources: {
      fx: {
        url: "https://api.frankfurter.dev/v1/{start}..{end}?from={base}&to=CNY",
        fallbacks: ["https://api.frankfurter.app/{start}..{end}?from={base}&to=CNY"],
      },
    },
  };
}

function fromRates(rates) {
  const dates = Object.keys(rates).sort();
  const dd = [];
  const vv = [];
  for (let i = 0; i < dates.length; i++) {
    const r = rates[dates[i]];
    if (r && typeof r.CNY === "number") {
      dd.push(dates[i]);
      vv.push(r.CNY);
    }
  }
  return { dates: dd, values: vv };
}

// 示例数据只对美元有意义（约 7.1）；其它货币没有示例。
function sample() {
  const values = [];
  const dates = [];
  for (let i = 0; i < 30; i++) {
    values.push(7.12 + 0.06 * Math.sin(i / 4) - i * 0.003);
    dates.push(t("第" + (i + 1) + "天", "Day " + (i + 1)));
  }
  return { dates: dates, values: values };
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const base = BASES.includes(ctx.vars.base) ? ctx.vars.base : "USD";
  const pair = base + " / CNY";
  const title = pair + " · " + t("近 30 天", "last 30 days");
  const src = ctx.sources.fx;
  let data = { dates: [], values: [] };
  let note = "";
  if (src && !src.error && src.rates) {
    data = fromRates(src.rates);
  }
  if (data.values.length === 0) {
    const why = src && src.error ? src.error : t("无数据", "no data");
    if (base === "USD") {
      data = sample();
      note = t("示例数据：", "Sample data: ") + why;
    } else {
      note = t("未能取到汇率：", "Could not fetch the rate: ") + why;
      return { title: title, icon: "dollar-sign", big: "--", sub: note, lines: [note], chart: null };
    }
  }
  // 范围：只看最近 n 个交易日（0 为全部）。
  const n = ctx.view.range ?? 0;
  if (n > 0 && data.values.length > n) {
    data = { dates: data.dates.slice(-n), values: data.values.slice(-n) };
  }
  const v = data.values;
  const last = v[v.length - 1];
  const first = v[0];
  const d = last - first;
  return {
    title: title,
    icon: "dollar-sign",
    big: last.toFixed(4),
    sub: `${t("区间", "Range")} ${d >= 0 ? "+" : ""}${d.toFixed(4)}`,
    lines: note ? [note] : [],
    chart: {
      type: "line",
      series: [{ name: base + "/CNY", color: "#10B981", points: v }],
      labels: data.dates,
      xLabels: [String(data.dates[0]), String(data.dates[data.dates.length - 1])],
    },
  };
}
