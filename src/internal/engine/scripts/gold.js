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
    version: "1.1.0",
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
    sources: {
      gold: {
        url: "https://tmini.net/api/gold-price?type=json",
        fallbacks: [
          "https://push2.eastmoney.com/api/qt/clist/get?pn=1&pz=30&po=1&np=1&fltt=2&invt=2&fid=f3&fs=m:118&fields=f2,f3,f4,f12,f14,f15,f16,f17,f18",
        ],
      },
    },
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

const SGE_NAMES = {
  AU9999: ["黄金9999", "Au99.99"],
  AUTD: ["黄金T+D", "Au(T+D)"],
  mAUTD: ["Mini黄金T+D", "mAu(T+D)"],
  AU100: ["黄金100g", "Au100g"],
  iAu999: ["国际版9999", "iAu99.99"],
  SHAU: ["上海金", "SHAU"],
  AU9995: ["黄金9995", "Au99.95"],
  PGC30: ["熊猫金币30g", "Panda 30g"],
};

function sgeRowsOf(src) {
  const list = src && src.data && src.data.diff;
  const out = [];
  if (!list || !list.length) return out;
  for (let i = 0; i < list.length; i++) {
    const it = list[i];
    if (!it) continue;
    const code = String(it.f12 || "");
    const names = SGE_NAMES[code];
    if (!names) continue;
    const v = num(it.f2) || num(it.f18);
    if (v <= 100 || v >= 5000) continue;
    out.push({ code: code, name: t(names[0], names[1]), v: v });
  }
  out.sort((a, b) => (a.code === "AU9999" ? -1 : b.code === "AU9999" ? 1 : b.v - a.v));
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
    const sge = sgeRowsOf(src);
    if (sge.length > 0) {
      rows = sge;
      prefer = t("黄金9999", "Au99.99");
      kindName = t("上海黄金交易所", "Shanghai Gold Exchange");
    }
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
