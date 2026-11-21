// 外汇牌价数据表：展示主流外币兑人民币的交互式数据表格，支持列排序、关键词筛选、单击复制与备选源自动回退。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("外汇牌价数据表", "FX rates table"),
    description: t("主流外币兑人民币交互式表格（支持排序、筛选、CSV 导出与备选源回退）", "Interactive FX vs CNY table with sorting, filtering, CSV export and fallback"),
    version: "1.0.0",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    icon: "file-spreadsheet",
    menubar: true,
    sources: {
      rates: {
        url: "https://api.frankfurter.dev/v1/latest?base=CNY&symbols=USD,EUR,GBP,JPY,HKD,SGD,AUD,CAD,CHF",
        fallbacks: [
          "https://open.er-api.com/v6/latest/CNY",
        ],
      },
    },
  };
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const raw = ctx.sources.rates;
  const info = (ctx.sourceInfo && ctx.sourceInfo.rates) || {};

  if (!raw || raw.error || !raw.rates) {
    return {
      title: t("外汇牌价数据表", "FX rates table"),
      icon: "file-spreadsheet",
      big: "--",
      sub: (raw && raw.error) ? String(raw.error) : t("未能取到汇率数据", "Could not fetch FX rates"),
      lines: [],
      chart: null,
    };
  }

  const pairs = [
    { code: "USD", name: t("美元", "US Dollar"), badge: "USD" },
    { code: "EUR", name: t("欧元", "Euro"), badge: "EUR" },
    { code: "GBP", name: t("英镑", "British Pound"), badge: "GBP" },
    { code: "HKD", name: t("港币", "HK Dollar"), badge: "HKD" },
    { code: "JPY", name: t("日元 (100)", "100 JPY"), badge: "JPY", scale: 100 },
    { code: "SGD", name: t("新加坡元", "Singapore Dollar"), badge: "SGD" },
    { code: "AUD", name: t("澳元", "Australian Dollar"), badge: "AUD" },
    { code: "CAD", name: t("加元", "Canadian Dollar"), badge: "CAD" },
    { code: "CHF", name: t("瑞士法郎", "Swiss Franc"), badge: "CHF" },
  ];

  const rows = [];
  let usdCny = "";
  for (const p of pairs) {
    const r = raw.rates[p.code];
    if (typeof r !== "number" || r <= 0) continue;
    const scale = p.scale || 1;
    const perUnit = scale / r;
    const for100 = 100 * r;
    if (p.code === "USD") usdCny = perUnit.toFixed(4);
    rows.push([
      p.name,
      p.badge,
      perUnit.toFixed(4),
      for100.toFixed(2),
      info.stale ? "warn" : "ok",
    ]);
  }

  const dateText = raw.date || t("实时", "Live");
  const usedNote = info.used > 0 ? t(" · 备选源", " · fallback") : "";

  return {
    title: t("外汇牌价数据表", "FX rates table"),
    icon: "file-spreadsheet",
    big: usdCny ? "¥" + usdCny : String(rows.length),
    sub: t("美元兑人民币 · ", "USD/CNY · ") + dateText + usedNote,
    lines: [],
    menubar: {
      title: usdCny ? "¥" + usdCny : "--",
      tooltip: t("美元兑人民币 ", "USD/CNY ") + (usdCny || "--"),
      mono: true,
      icon: "bars",
    },
    menu: [
      { label: t("复制美元汇率", "Copy USD/CNY"), action: "copy_headline" },
      { label: t("打开设置", "Open Settings"), action: "settings" },
    ],
    chart: {
      type: "table",
      table: {
        searchable: true,
        striped: true,
        compact: 5,
        columns: [
          { title: t("币种", "Currency"), width: 2 },
          { title: t("代码", "Code"), format: "badge", align: "center" },
          { title: t("折合人民币", "In CNY"), align: "right", format: "number" },
          { title: t("100 CNY 换", "Per 100 CNY"), align: "right", format: "number" },
          { title: t("状态", "Status"), format: "status" },
        ],
        rows: rows,
      },
    },
    layout: { legend: "hide", summary: "top", stats: "hide" },
  };
}
