// IP 与网络看板：聚合出口 IPv4、地理位置与 IP 属性、6 站点 12 轮点阵连通性及网站分流出口汇总。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

function siteLabel(name) {
  const map = {
    "网易": t("网易", "NetEase"),
    "字节跳动": t("字节跳动", "ByteDance"),
    "淘宝": t("淘宝", "Taobao"),
    "微信": t("微信", "WeChat"),
    "Cloudflare中国": t("Cloudflare中国", "Cloudflare CN"),
    "高通中国": t("高通中国", "Qualcomm CN"),
  };
  return map[name] || name;
}

function maskIP(ip) {
  const s = String(ip || "").trim();
  if (!s) return "";
  if (/^\d{1,3}(\.\d{1,3}){3}$/.test(s)) {
    const p = s.split(".");
    return p[0] + "." + p[1] + ".*.*";
  }
  if (s.indexOf(":") >= 0) {
    const p = s.split(":");
    if (p.length >= 2) return p[0] + ":" + p[1] + ":*";
  }
  return s;
}

function catLabel(cat) {
  const m = {
    social: "Social",
    ai: "AI",
    crypto: "Crypto",
    tools: "Tools",
    speed: "Speed",
    dev: "Dev",
    static: "Static",
  };
  return m[cat] || "";
}

function riskStatusText(risk) {
  if (!risk) return { text: t("未获取风控信息", "Unavailable"), status: "info" };
  const flags = [];
  if (risk.is_tor) flags.push("Tor");
  if (risk.is_vpn) flags.push("VPN");
  if (risk.is_proxy) flags.push("Proxy");
  if (risk.is_abuser) flags.push("Abuser");
  if (risk.is_datacenter) flags.push(t("机房", "DC"));
  if (risk.isResidential) flags.push(t("住宅", "Residential"));
  if (flags.length === 0) return { text: t("正常 · 无异常标记", "Clean · No proxy flags"), status: "ok" };
  const warn = !!(risk.is_tor || risk.is_vpn || risk.is_proxy || risk.is_abuser);
  return { text: flags.join(" · "), status: warn ? "warn" : "ok" };
}

function buildIPDetails(ip, geo, isp, prop, risk, sites) {
  const kv = [];
  if (ip) kv.push({ label: t("IP 地址", "IP Address"), value: ip, status: "info" });
  if (geo) kv.push({ label: t("地理位置", "Location"), value: geo });
  if (prop) {
    const isRes = risk && risk.isResidential;
    kv.push({ label: t("IP 属性", "IP Type"), value: prop, status: isRes ? "ok" : "info" });
  }
  if (isp) kv.push({ label: t("运营商 / ISP", "ISP / Org"), value: isp });
  if (risk) {
    if (risk.asn) {
      const org = risk.asOrganization ? " · " + risk.asOrganization : "";
      kv.push({ label: "ASN", value: "AS" + risk.asn + org });
    }
    if (risk.cidr) kv.push({ label: t("网段 CIDR", "CIDR"), value: risk.cidr });
    if (risk.abuser_score) {
      const low = String(risk.abuser_score).toLowerCase().indexOf("low") >= 0;
      kv.push({ label: t("滥用评分", "Abuse Score"), value: risk.abuser_score, status: low ? "ok" : "warn" });
    }
    const rs = riskStatusText(risk);
    kv.push({ label: t("风险标记", "Risk Flags"), value: rs.text, status: rs.status });
    if (risk.timezone) kv.push({ label: t("时区", "Timezone"), value: risk.timezone });
  }
  if (sites && sites.length) {
    kv.push({
      label: t("分流站点", "Matched Sites"),
      value: sites.map(siteLabel).join(", "),
      badge: String(sites.length) + t(" 站", " sites"),
    });
  }
  return kv;
}

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("IP 与网络看板", "IP & Network"),
    description: t("当前 IP、地理位置、6 站点 12 轮网络连通性与网站分流出口汇总", "Local IP, geolocation, 12-round connectivity matrix, and split-routing exit summary"),
    version: "1.0.0",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    icon: "globe",
    menubar: true,
    dims: [
      {
        key: "group",
        label: t("分流分组", "Routing"),
        options: [
          { value: 0, label: t("全部", "All") },
          { value: 1, label: t("国内", "Domestic") },
          { value: 2, label: t("国际", "Global") },
          { value: 3, label: "AI" },
        ],
      },
    ],
    netprobe: { rounds: 12, includeRisk: true, includeSplit: true },
    sources: {},
  };
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const np = ctx.netprobe;
  const title = t("IP 与网络看板", "IP & Network");

  if (!np || np.error) {
    const why = (np && np.error) ? String(np.error) : t("等待网络探测完成", "Waiting for network probe");
    return {
      title: title,
      icon: "globe",
      big: "--",
      sub: why,
      lines: [],
      chart: null,
    };
  }

  const hero = np.hero || {};
  const probes = Array.isArray(np.probes) ? np.probes : [];
  const exits = Array.isArray(np.exits) ? np.exits : [];
  const splits = Array.isArray(np.splits) ? np.splits : [];

  if (!hero.ip && probes.length === 0 && exits.length === 0) {
    return {
      title: title,
      icon: "globe",
      big: "--",
      sub: t("未能获取网络探测结果", "No network probe data"),
      lines: [],
      chart: null,
    };
  }

  const heroIP = hero.ip || (exits[0] && exits[0].ip) || "--";
  const heroGeo = EN ? (hero.geoEn || hero.geoZh || "") : (hero.geoZh || hero.geoEn || "");
  const heroProp = EN ? (hero.propertyEn || hero.propertyZh || "") : (hero.propertyZh || hero.propertyEn || "");
  const heroFlag = hero.countryCode || (exits[0] && exits[0].countryCode) || "";

  let onlineCount = 0;
  let sumMs = 0;
  const probeItems = probes.map((p) => {
    const ms = typeof p.medianMs === "number" ? p.medianMs : -1;
    let st = "err";
    let txt = t("超时", "Timeout");
    if (ms >= 0) {
      onlineCount++;
      sumMs += ms;
      txt = Math.min(ms, 999) + "ms";
      st = ms < 100 ? "ok" : ms < 400 ? "warn" : "slow";
    }
    return {
      name: EN ? (p.enName || siteLabel(p.name)) : p.name,
      icon: p.icon || "",
      flag: p.flag || "",
      tag: p.tag || "",
      latency: ms,
      minMs: typeof p.minMs === "number" ? p.minMs : -1,
      maxMs: typeof p.maxMs === "number" ? p.maxMs : -1,
      success: p.success || 0,
      total: p.total || 0,
      status: st,
      text: txt,
      dots: Array.isArray(p.dots) ? p.dots : [],
      url: p.url || "",
    };
  });

  const pillSource = exits.length > 0 ? exits : (hero.ip ? [{
    ip: hero.ip,
    countryCode: heroFlag,
    geoZh: hero.geoZh,
    geoEn: hero.geoEn,
    isp: hero.isp,
    propertyZh: hero.propertyZh,
    propertyEn: hero.propertyEn,
    count: 0,
    sites: [],
    risk: hero.risk,
  }] : []);

  const pillItems = pillSource.map((ex) => {
    const g = EN ? (ex.geoEn || ex.geoZh || "") : (ex.geoZh || ex.geoEn || "");
    const pr = EN ? (ex.propertyEn || ex.propertyZh || "") : (ex.propertyZh || ex.propertyEn || "");
    const cnt = ex.count || 0;
    const badge = cnt > 0 ? String(cnt) + t(" 站", " sites") : "";
    return {
      flag: ex.countryCode || "",
      value: ex.ip || "",
      masked: maskIP(ex.ip),
      sub: g,
      badge: badge,
      mono: true,
      copy: ex.ip || "",
      url: ex.ip ? ("https://ip.net.coffee/ip/" + ex.ip) : "",
      details: buildIPDetails(ex.ip, g, ex.isp || "", pr, ex.risk, ex.sites || []),
    };
  });

  const groupVal = (ctx.view && typeof ctx.view.group === "number") ? ctx.view.group : 0;
  const rows = [];
  for (let i = 0; i < splits.length; i++) {
    const sp = splits[i];
    if (groupVal === 1 && sp.type !== "domestic") continue;
    if (groupVal === 2 && sp.type !== "international") continue;
    if (groupVal === 3 && sp.category !== "ai") continue;

    const grpText = sp.type === "domestic"
      ? t("国内", "Domestic")
      : (catLabel(sp.category) ? t("国际 · ", "Global · ") + catLabel(sp.category) : t("国际", "Global"));
    const geoText = EN ? (sp.geoEn || sp.geoZh || "") : (sp.geoZh || sp.geoEn || "");
    const latText = sp.status === "ok" && sp.latencyMs >= 0 ? (sp.latencyMs + "ms") : "--";
    const stVal = sp.status === "ok" ? "ok" : "err";
    const ipStr = sp.status === "ok" ? (sp.ip || "--") : (sp.error || t("获取失败", "Failed"));
    const ccPrefix = sp.countryCode ? (String(sp.countryCode).toUpperCase() + " ") : "";

    rows.push([
      siteLabel(sp.name),
      grpText,
      ccPrefix + ipStr,
      geoText,
      latText,
      stVal,
    ]);
  }

  const avgMs = onlineCount > 0 ? Math.round(sumMs / onlineCount) : 0;
  const menubarTitle = onlineCount > 0
    ? (avgMs + "ms " + onlineCount + "/" + probes.length)
    : (heroIP !== "--" ? heroIP : "--");
  const menubarColor = onlineCount === 0 ? "#EF4444" : (onlineCount === probes.length ? "#22C55E" : "#F59E0B");

  const menu = [];
  if (hero.ip) {
    menu.push({ label: t("复制出口 IP: ", "Copy IP: ") + hero.ip, copy: hero.ip });
    menu.push({ label: t("在 ip.net.coffee 查询质量", "Open on ip.net.coffee"), url: "https://ip.net.coffee/ip/" + hero.ip });
    menu.push({ separator: true });
  }
  menu.push({ label: t("立即重新探测", "Probe Now"), action: "refresh" });

  const subSummary = heroGeo
    ? (heroGeo + (probes.length ? (" · " + onlineCount + "/" + probes.length + t(" 在线", " online")) : ""))
    : (onlineCount + "/" + probes.length + t(" 在线", " online"));

  return {
    title: title,
    icon: "globe",
    big: heroIP,
    sub: subSummary,
    lines: [],
    menubar: {
      title: menubarTitle,
      tooltip: heroIP + (heroGeo ? (" · " + heroGeo) : "") + " (" + onlineCount + "/" + probes.length + ")",
      mono: true,
      icon: "dot",
      color: menubarColor,
    },
    menu: menu,
    chart: {
      type: "bento",
      bento: {
        maskable: true,
        hero: {
          label: "IPV4",
          flag: heroFlag,
          value: heroIP,
          masked: maskIP(heroIP),
          location: heroGeo || t("未知位置", "Unknown location"),
          meta: heroProp || hero.source || "",
          copy: hero.ip || "",
          details: buildIPDetails(heroIP, heroGeo, hero.isp || "", heroProp, hero.risk, null),
        },
        probes: {
          title: t("网络连通性", "Network Connectivity"),
          actionText: t("查看更多 ›", "More ›"),
          columns: 3,
          items: probeItems,
        },
        pills: {
          title: t("分流出口 IP 汇总（点击查询 IP 质量）", "Split-Route Exit IPs (click for IP quality)"),
          columns: 4,
          items: pillItems,
        },
        detailTitle: t("网站分流测试", "Website Split-Routing Test"),
        detailDesc: t(
          "检测访问不同网站时实际使用的出口 IP 及地理位置规则",
          "Observed exit IP and geolocation across domestic and global websites"
        ),
        detailTable: {
          searchable: true,
          striped: true,
          compact: 6,
          columns: [
            { title: t("网站", "Website"), width: 3 },
            { title: t("分组", "Group"), format: "badge", align: "center", width: 2 },
            { title: "IP", width: 2 },
            { title: t("地理位置", "Geolocation"), width: 4 },
            { title: t("延迟", "Latency"), align: "right", width: 1 },
            { title: t("状态", "Status"), format: "status", align: "center", width: 1 },
          ],
          rows: rows,
        },
      },
    },
    layout: {
      title: "hide",
      summary: "hide",
      stats: "hide",
      legend: "hide",
      actions: "hover",
    },
  };
}
