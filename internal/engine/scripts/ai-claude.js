// Claude Code 用量：读取本机日志（ctx.usage["claude-code"]）。
// 在「设置 → AI 用量」填月额度或 5 小时额度，就能看到剩余量；
// 没填额度时按模型显示横置条形图。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("Claude Code 用量", "Claude Code usage"),
    icon: "bot",
    usage: ["claude-code"],
    dims: [
      {
        key: "view",
        label: t("视图", "View"),
        options: [
          { value: 0, label: t("剩余", "Left") },
          { value: 1, label: t("按模型", "By model") },
          { value: 2, label: t("每日", "Daily") },
        ],
      },
      {
        key: "window",
        label: t("周期", "Period"),
        options: [
          { value: 0, label: t("本月", "Month") },
          { value: 1, label: t("5 小时", "5 hours") },
          { value: 2, label: t("今日", "Today") },
        ],
      },
    ],
    sources: {},
  };
}

function fmt(n) {
  if (n >= 1e9) return (n / 1e9).toFixed(1) + "B";
  if (n >= 1e6) return (n / 1e6).toFixed(1) + "M";
  if (n >= 1e4) return (n / 1e3).toFixed(1) + "K";
  return String(Math.round(n));
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const u = ctx.usage["claude-code"];
  if (!u || u.error) {
    return { title: t("Claude Code 用量", "Claude Code usage"), icon: "bot", big: "--", sub: (u && u.error) || t("没有数据", "No data"), lines: [], chart: null };
  }
  const view = ctx.view.view ?? 0;
  const win = ctx.view.window ?? 0;
  const names = [t("本月", "Month"), t("5 小时", "5 hours"), t("今日", "Today")];
  const used = [u.windows.month, u.windows.block, u.windows.today][win] || 0;
  const limit = [u.limit, u.blockLimit, 0][win] || 0;
  const w = u.windows;
  const head = {
    title: `Claude Code · ${names[win]}`,
    icon: "bot",
    big: `${fmt(used)} tokens`,
    sub: EN ? `Today ${fmt(w.today)} · Week ${fmt(w.week)} · Month ${fmt(w.month)}` : `今日 ${fmt(w.today)} · 本周 ${fmt(w.week)} · 本月 ${fmt(w.month)}`,
    lines: u.note ? [u.note] : [],
  };

  if (view === 2) {
    head.chart = {
      type: "line",
      series: [{ name: "tokens", color: "#4FC3F7", points: u.daily.map((d) => d.value) }],
      labels: u.daily.map((d) => d.name),
      xLabels: [u.daily[0].name, u.daily[u.daily.length - 1].name],
    };
    return head;
  }
  if (view === 0 && limit > 0) {
    head.chart = {
      type: "gauge",
      gauge: { used: used, limit: limit, unit: "tokens", label: names[win] + t("额度", " budget") },
    };
    return head;
  }
  if (view === 0 && limit === 0) head.lines.push(t("在「设置 → AI 用量」填写额度即可看到剩余量", "Set a budget in Settings → AI usage to see what is left"));
  head.chart = {
    type: "barh",
    series: [{ name: "tokens", color: "#4FC3F7", points: u.models.map((m) => m.value) }],
    labels: u.models.map((m) => m.name.replace("claude-", "")),
  };
  return head;
}
