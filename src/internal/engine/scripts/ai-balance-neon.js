// AI 余额总览：用霓虹进度条或分段进度条在一张卡片里汇总多家 AI 余额。
// 在「设置 → AI 用量」填写各家 API 密钥与充值总额即可显示剩余比例。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

const PROVIDERS = [
  { id: "deepseek-balance", name: "DeepSeek", color: "#00F5D4", to: "#38BDF8" },
  { id: "openrouter-credits", name: "OpenRouter", color: "#A855F7", to: "#38BDF8" },
  { id: "moonshot-balance", name: "Moonshot", color: "#FF2A85", to: "#FB923C" },
  { id: "siliconflow-balance", name: "SiliconFlow", color: "#22C55E", to: "#00F5D4" },
];

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("AI 余额总览", "AI balances"),
    description: t("用霓虹或分段进度条汇总 DeepSeek、OpenRouter、Moonshot 与 SiliconFlow 余额", "Neon or segmented progress bars for DeepSeek, OpenRouter, Moonshot and SiliconFlow balances"),
    version: "1.0.0",
    platforms: ["desktop", "mobile"],
    needs: ["network", "key"],
    icon: "zap",
    usage: PROVIDERS.map((p) => p.id),
    dims: [
      {
        key: "style",
        label: t("风格", "Style"),
        options: [
          { value: 0, label: t("霓虹灯", "Neon") },
          { value: 1, label: t("分段", "Segmented") },
        ],
      },
    ],
    sources: {},
  };
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const title = t("AI 余额总览", "AI balances");
  const usage = ctx.usage || {};
  const style = (ctx.view && ctx.view.style) === 1 ? "segmented" : "neon";

  const items = [];
  const lines = [];
  let okCount = 0;
  let firstBig = "";

  for (let i = 0; i < PROVIDERS.length; i++) {
    const p = PROVIDERS[i];
    const u = usage[p.id];
    if (!u || u.error) continue;
    okCount++;
    const bal = typeof u.balance === "number" ? u.balance : 0;
    const lim = typeof u.limit === "number" ? u.limit : 0;
    const unit = u.unit || "";
    const balText = bal.toFixed(2) + (unit ? " " + unit : "");
    if (!firstBig) firstBig = balText;
    if (lim > 0) {
      const leftPct = Math.max(0, Math.min(100, (bal / lim) * 100));
      let col = p.color;
      let to = p.to;
      if (leftPct <= 10) {
        col = "#FF2A85";
        to = "#FB7185";
      } else if (leftPct <= 30) {
        col = "#F59E0B";
        to = "#FB923C";
      }
      items.push({
        name: p.name + " · " + balText,
        value: leftPct,
        max: 0,
        color: col,
        to: to,
      });
    } else {
      items.push({
        name: p.name + " · " + balText,
        value: bal > 0 ? 100 : 0,
        max: 0,
        color: p.color,
        to: p.to,
      });
      lines.push(p.name + ": " + t("未填充值总额，按满格显示", "set top-up total in Settings for %"));
    }
  }

  if (okCount === 0) {
    return {
      title: title,
      icon: "zap",
      big: "--",
      sub: t("在「设置 → AI 用量」填写任意平台密钥即可显示", "Enter any provider API key in Settings → AI usage"),
      lines: [],
      chart: null,
    };
  }

  return {
    title: title,
    icon: "zap",
    big: okCount === 1 ? firstBig : EN ? okCount + " providers" : okCount + " 家在线",
    sub: EN ? okCount + " active AI accounts" : "已接入 " + okCount + " 个 AI 余额账户",
    lines: lines,
    progressBar: {
      style: style,
      height: 10,
      track: "dark",
      glow: 1.0,
      segments: 12,
      items: items,
    },
    layout: { legend: "hide" },
  };
}
