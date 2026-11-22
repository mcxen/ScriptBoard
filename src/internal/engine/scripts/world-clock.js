// 霓虹世界时钟：纯离线，支持一键切换多时区世界时钟、霓虹模拟指针表盘与数字大表，也支持放进菜单栏。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("世界时钟", "World clock"),
    description: t("支持世界多时区、指针表盘与数字大表，离线可用", "World time zones, analog dial and digital clock, offline"),
    version: "1.1.0",
    platforms: ["desktop", "mobile"],
    needs: [],
    icon: "calendar-days",
    menubar: true,
    sources: {},
  };
}

function pad(n) {
  return (n < 10 ? "0" : "") + n;
}

function localHHMM(nowMs, offSec) {
  let s = Math.floor(nowMs / 1000) + (offSec || 0);
  s = ((s % 86400) + 86400) % 86400;
  return pad(Math.floor(s / 3600)) + ":" + pad(Math.floor((s % 3600) / 60));
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const modes = ["world", "analog", "digital"];
  const idx = (ctx.view && ctx.view.style) || 0;
  const style = modes[idx] || "world";
  const hhmm = localHHMM(ctx.now, ctx.tzOffset);

  const zones = [
    { label: t("北京 / 上海", "Shanghai"), offset: 8 },
    { label: t("东京", "Tokyo"), offset: 9 },
    { label: t("伦敦", "London"), offset: 0 },
    { label: t("纽约", "New York"), offset: -5 },
    { label: t("旧金山", "San Francisco"), offset: -8 },
  ];

  return {
    title: t("世界时钟", "World clock"),
    icon: "calendar-days",
    big: hhmm,
    sub: "",
    lines: [],
    menubar: {
      title: hhmm,
      tooltip: t("本地时间 ", "Local time ") + hhmm,
      mono: true,
      click: "open",
      icon: "ring",
    },
    menu: [
      { label: t("复制当前时间", "Copy time"), action: "copy_headline" },
      { separator: true },
      { label: t("打开设置", "Open Settings"), action: "settings" },
    ],
    chart: {
      type: "clock",
      clock: {
        style: style,
        seconds: true,
        color: "#00F5D4",
        neon: true,
        zones: zones,
      },
    },
    layout: { legend: "hide", summary: "hide" },
  };
}
