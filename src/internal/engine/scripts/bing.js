// 必应壁纸：必应每日图片，两页共 16 张（去重），来自 bing.com（无需 key）。
// 图片在卡片里轮播，每张都会缓存到本机并保留历史；每张可以打开来源页或复制链接。
// 语言：meta({lang}) 和 render(ctx) 都带 lang（"zh" 或 "en"）。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("必应壁纸", "Bing wallpapers"),
    description: t("必应每日图片，可以翻看、收集和打开来源", "Bing's daily pictures: browse, keep, open the source"),
    version: "1.1.0",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    icon: "sparkles",
    sources: {
      // Two pages of eight: the archive's idx is the first picture of each page.
      bing: {
        url: "https://www.bing.com/HPImageArchive.aspx?format=js&n=8&pid=hp&uhd=1&uhdwidth=3840&uhdheight=2160&mkt=zh-CN",
        paging: { param: "idx", start: 0, step: 8, pages: 2, items: "images" },
      },
    },
  };
}

// The picture's id, from its address: the OHR name, or its date.
function idOf(i) {
  const m = /OHR\.([^_&./]+)/i.exec(String(i.urlbase || i.url || ""));
  return m ? m[1] : String(i.startdate || i.url || "");
}

// The copyright text without its credit in brackets: "Hill (© Someone)" is "Hill".
function titleOf(i) {
  const c = String(i.copyright || "").trim();
  return c.replace(/\s*\([^()]*(?:©|\bc\b)[^()]*\)\s*$/i, "").trim() || idOf(i);
}

// A date in the archive's form, 20261119, as 2026-11-19.
function dateOf(i) {
  const d = String(i.startdate || "");
  return d.length === 8 ? d.slice(0, 4) + "-" + d.slice(4, 6) + "-" + d.slice(6, 8) : d;
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const title = t("必应壁纸", "Bing wallpapers");
  const src = ctx.sources.bing;
  if (!src || src.error || !Array.isArray(src)) {
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
  const seen = {};
  const images = [];
  for (const i of src) {
    const id = idOf(i);
    if (!id || seen[id] || !i.urlbase) continue;
    seen[id] = true;
    const page = i.copyrightlink ? "https://www.bing.com" + i.copyrightlink : "";
    images.push({
      url: "https://www.bing.com" + i.urlbase + "_1920x1080.jpg",
      caption: [dateOf(i), titleOf(i)].filter((x) => x).join(" · "),
      source: page,
    });
  }
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
