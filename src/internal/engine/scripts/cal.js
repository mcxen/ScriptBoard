// 日历：月视图由宿主画出；农历和国际节日在点击某一天后展示。
// 不请求网络。语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("日历", "Calendar"),
    description: t("农历与国际节日", "Lunar dates and holidays"),
    version: "1.0.0",
    platforms: ["desktop", "mobile"],
    needs: [],
    icon: "calendar-days",
    sources: {},
  };
}

export function render(ctx) {
  EN = ctx.lang === "en";
  return {
    title: t("日历", "Calendar"),
    icon: "calendar-days",
    big: "",
    sub: "",
    lines: [],
    chart: { type: "calendar" },
    layout: { legend: "hide" },
  };
}
