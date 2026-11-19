// 门市金价：Tmini 免费接口，品牌金店当日挂牌价（元/克），无需 key。
// 离线或请求失败时不造数，显示错误。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("门市金价", "Store gold"),
    description: t("各金店和银行金条的当日金价", "Today's gold price at stores and for bank bars"),
    version: "1.0.0",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    icon: "coins",
    dims: [
      {
        key: "kind",
        label: t("种类", "Kind"),
        options: [
          { value: 0, label: t("金店", "Stores") },
          { value: 1, label: t("银行金条", "Bank bars") },
          { value: 2, label: t("回收", "Recycle") },
        ],
      },
    ],
    sources: { gold: "https://tmini.net/api/gold-price?type=json" },
  };
}

function num(x) {
  const n = typeof x === "number" ? x : parseFloat(x);
  return n === n ? n : 0;
}

function rowsOf(list) {
  const out = [];
  if (!list || !list.length) return out;
  for (let i = 0; i < list.length; i++) {
    const it = list[i];
    if (!it) continue;
    const v = num(it.price);
    if (v <= 0) continue;
    const name = String(it.brand || it.bank || it.type || "");
    if (!name) continue;
    out.push({ name: name, v: v });
  }
  return out;
}

function headline(rows, prefer) {
  for (let i = 0; i < rows.length; i++) {
    if (rows[i].name.indexOf(prefer) >= 0) return rows[i];
  }
  return rows[0];
}

function spotCNY(metals) {
  if (!metals || !metals.length) return 0;
  for (let i = 0; i < metals.length; i++) {
    const m = metals[i];
    if (!m || !m.unit || String(m.unit).indexOf("元") < 0) continue;
    if (m.name === "今日金价" || m.name === "黄金价格") return num(m.sell_price || m.today_price);
  }
  return 0;
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const src = ctx.sources.gold;
  const title = t("金价 · 元/克", "Gold · CNY/g");
  if (!src || src.error) {
    const why = src && src.error ? String(src.error) : t("无数据", "no data");
    return {
      title: title,
      icon: "coins",
      big: "--",
      sub: t("未能取到金价：", "Could not fetch gold: ") + why,
      lines: [t("需要网络才能取今日金店挂牌价", "Needs the network for today's store prices")],
      chart: null,
    };
  }
  const kind = ctx.view.kind ?? 0;
  let rows = [];
  let prefer = "周大福";
  let kindName = t("金店", "Stores");
  if (kind === 1) {
    rows = rowsOf(src.banks);
    prefer = "工商银行";
    kindName = t("银行金条", "Bank bars");
  } else if (kind === 2) {
    rows = rowsOf(src.recycle);
    prefer = "黄金回收";
    kindName = t("回收", "Recycle");
  } else {
    rows = rowsOf(src.stores);
  }
  if (rows.length === 0) {
    return {
      title: title,
      icon: "coins",
      big: "--",
      sub: t("接口没有返回报价", "The feed returned no prices"),
      lines: [],
      chart: null,
    };
  }
  const head = headline(rows, prefer);
  const spot = spotCNY(src.metals);
  const date = src.date ? String(src.date) : "";
  const lines = [];
  if (date) lines.push(date + " · " + kindName);
  if (spot > 0) lines.push(t("基础金价 ", "Spot ") + spot.toFixed(2) + t(" 元/克", " CNY/g"));
  return {
    title: title,
    icon: "coins",
    big: head.v.toFixed(2),
    sub: head.name,
    lines: lines,
    chart: {
      type: "barh",
      series: [{ name: t("金价", "Gold"), color: "#EAB308", points: rows.map((r) => r.v) }],
      labels: rows.map((r) => r.name),
    },
  };
}
