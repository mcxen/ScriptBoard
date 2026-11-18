// DeepSeek 余额：在「设置 → AI 用量」填写密钥后读取（ctx.usage["deepseek-balance"]）。
// 填写「充值总额」后显示剩余比例的环形图。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

export function meta(a) {
  EN = !!a && a.lang === "en";
  return { name: t("DeepSeek 余额", "DeepSeek balance"), icon: "wallet", usage: ["deepseek-balance"], sources: {} };
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const u = ctx.usage["deepseek-balance"];
  if (!u || u.error) {
    return { title: t("DeepSeek 余额", "DeepSeek balance"), icon: "wallet", big: "--", sub: (u && u.error) || t("没有数据", "No data"), lines: [], chart: null };
  }
  const out = {
    title: t("DeepSeek 余额", "DeepSeek balance"),
    icon: "wallet",
    big: u.balance.toFixed(2) + " " + u.unit,
    sub: u.limit > 0 ? t("已用 ", "Used ") + u.used.toFixed(2) + " / " + u.limit.toFixed(2) : t("在设置里填写充值总额可看剩余比例", "Enter the total you topped up in Settings to see the share left"),
    lines: [],
    chart: null,
  };
  if (u.limit > 0) {
    out.chart = { type: "gauge", gauge: { used: u.used, limit: u.limit, unit: u.unit, label: t("余额", "Balance") } };
  }
  return out;
}
