// Claude Code 看板：使用霓虹进度条 / 环形占比 / 打卡热力格三种新样式展示本机用量。
// 读取本机日志（ctx.usage["claude-code"]），不联网。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("Claude Code 看板", "Claude Code board"),
    description: t("用霓虹进度条、模型占比环图与热力格展示本机 Claude Code 用量", "Claude Code usage with neon progress bars, model donut and activity grid"),
    version: "1.0.0",
    platforms: ["desktop"],
    needs: ["files"],
    icon: "sparkles",
    usage: ["claude-code"],
    dims: [
      {
        key: "view",
        label: t("样式", "View"),
        options: [
          { value: 0, label: t("霓虹额度", "Neon bars") },
          { value: 1, label: t("模型占比", "Models") },
          { value: 2, label: t("活跃热力", "Activity") },
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

function pctOf(v, max) {
  if (max <= 0) return 0;
  return Math.max(0, Math.min(100, (v / max) * 100));
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const u = ctx.usage && ctx.usage["claude-code"];
  const title = t("Claude Code 看板", "Claude Code board");
  if (!u || u.error) {
    return {
      title: title,
      icon: "sparkles",
      big: "--",
      sub: (u && u.error) || t("没有本机用量数据", "No local usage data"),
      lines: [],
      chart: null,
    };
  }
  const view = (ctx.view && ctx.view.view) || 0;
  const w = u.windows || {};
  const today = w.today || 0;
  const week = w.week || 0;
  const month = w.month || 0;
  const block = w.block || 0;
  const models = u.models || [];
  const daily = u.daily || [];
  const sub = EN
    ? `Today ${fmt(today)} · Week ${fmt(week)} · Month ${fmt(month)}`
    : `今日 ${fmt(today)} · 本周 ${fmt(week)} · 本月 ${fmt(month)}`;
  const lines = u.note ? [u.note] : [];

  if (view === 1) {
    const palette = ["#00F5D4", "#38BDF8", "#A855F7", "#FF2A85", "#FB923C", "#22C55E"];
    const slices = models
      .filter((m) => m && m.value > 0)
      .slice(0, 6)
      .map((m, i) => ({
        name: String(m.name || "").replace("claude-", ""),
        value: Math.round(m.value),
        color: palette[i % palette.length],
        unit: "",
      }));
    return {
      title: title + " · " + t("模型占比", "Models"),
      icon: "sparkles",
      big: fmt(month),
      sub: sub,
      lines: lines,
      chart: {
        type: "donut",
        donut: { center: fmt(month), sub: t("本月总计", "Month"), thickness: 0.28, neon: true },
        slices: slices,
      },
      layout: { legend: "hide" },
    };
  }

  if (view === 2) {
    const cells = daily.map((d) => ({
      date: String(d.name || ""),
      value: Math.round((d.value || 0) / 1000),
      label: String(d.name || ""),
    }));
    return {
      title: title + " · " + t("活跃热力", "Activity"),
      icon: "sparkles",
      big: fmt(month),
      sub: sub,
      lines: lines,
      chart: {
        type: "contrib",
        contrib: { color: "#00F5D4", weeks: 12, unit: "K", neon: true, cells: cells },
      },
      layout: { legend: "hide" },
    };
  }

  // Default: standalone glowing neon progress bars (no chart wrapper).
  const items = [];
  if (u.blockLimit > 0) {
    items.push({
      name: t("5 小时窗口", "5-hour window") + " · " + fmt(block) + " / " + fmt(u.blockLimit),
      value: pctOf(block, u.blockLimit),
      max: 0,
      color: "#00F5D4",
      to: "#38BDF8",
    });
  }
  if (u.limit > 0) {
    items.push({
      name: t("本月额度", "Monthly budget") + " · " + fmt(month) + " / " + fmt(u.limit),
      value: pctOf(month, u.limit),
      max: 0,
      color: "#A855F7",
      to: "#FF2A85",
    });
  }
  if (items.length === 0) {
    const base = Math.max(month, week, today, 1);
    items.push(
      { name: t("今日用量", "Today") + " · " + fmt(today), value: pctOf(today, base), max: 0, color: "#00F5D4", to: "#38BDF8" },
      { name: t("本周用量", "Week") + " · " + fmt(week), value: pctOf(week, base), max: 0, color: "#A855F7", to: "#38BDF8" },
      { name: t("本月用量", "Month") + " · " + fmt(month), value: month > 0 ? 100 : 0, max: 0, color: "#FF2A85", to: "#FB923C" }
    );
    lines.push(t("在「设置 → AI 用量」填写额度即可按上限显示进度", "Set a budget in Settings → AI usage to track against a limit"));
  } else {
    items.push({
      name: t("今日占比", "Today vs week") + " · " + fmt(today),
      value: pctOf(today, Math.max(week, today, 1)),
      max: 0,
      color: "#22C55E",
      to: "#00F5D4",
    });
  }

  return {
    title: title,
    icon: "sparkles",
    big: fmt(month) + " tokens",
    sub: sub,
    lines: lines,
    progressBar: {
      style: "neon",
      height: 10,
      track: "dark",
      glow: 1.0,
      items: items,
    },
    layout: { legend: "hide" },
  };
}
