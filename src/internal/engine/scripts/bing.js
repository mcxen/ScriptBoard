// 必应壁纸：必应每日图片，最近 8 张，来自 bing.com（无需 key）。图片在卡片里轮播，历史会保存在本机。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("必应壁纸", "Bing wallpapers"),
    description: t("必应每日图片，可以翻看和收集", "Bing's daily pictures, to browse and keep"),
    version: "1.0.0",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    icon: "sparkles",
    sources: {
      bing: "https://www.bing.com/HPImageArchive.aspx?format=js&idx=0&n=8&mkt=zh-CN",
    },
  };
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const title = t("必应壁纸", "Bing wallpapers");
  const src = ctx.sources.bing;
  if (!src || src.error || !Array.isArray(src.images)) {
    const why = src && src.error ? String(src.error) : t("无数据", "no data");
    return {
      title: title,
      icon: "sparkles",
      big: "--",
      sub: t("未能取到图片：", "Could not fetch pictures: ") + why,
      lines: [],
      chart: null,
      images: [],
    };
  }
  const images = src.images.map((i) => ({
    url: "https://www.bing.com" + i.url,
    caption: [i.title, i.copyright].filter((x) => x).join(" · "),
  }));
  return {
    title: title,
    icon: "sparkles",
    big: String(images.length),
    sub: t("每日一图 · 可以翻看", "Daily pictures · browse them"),
    lines: [],
    chart: null,
    images: images,
  };
}
