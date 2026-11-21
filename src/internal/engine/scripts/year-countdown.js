// 年度与节日倒计时：纯离线自动按当前日期计算下一年、月底、季末与下个周末倒计时，搭配霓虹时光进度条。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("年度与节日倒计时", "Year & milestones"),
    description: t("年度倒计时霓虹进度条与近期时间节点列表，离线可用", "Year progress neon countdown and upcoming milestones, offline"),
    version: "1.0.0",
    platforms: ["desktop", "mobile"],
    needs: [],
    icon: "bell",
    dims: [
      {
        key: "view",
        label: t("视图", "View"),
        options: [
          { value: 0, label: t("年度进度", "Year") },
          { value: 1, label: t("近期节点", "Milestones") },
        ],
      },
    ],
    sources: {},
  };
}

function pad(n) {
  return (n < 10 ? "0" : "") + n;
}

function civilFromDays(z) {
  z += 719468;
  const era = Math.floor((z >= 0 ? z : z - 146096) / 146097);
  const doe = z - era * 146097;
  const yoe = Math.floor((doe - Math.floor(doe / 1460) + Math.floor(doe / 36524) - Math.floor(doe / 146096)) / 365);
  const y = yoe + era * 400;
  const doy = doe - (365 * yoe + Math.floor(yoe / 4) - Math.floor(yoe / 100));
  const mp = Math.floor((5 * doy + 2) / 153);
  const d = doy - Math.floor((153 * mp + 2) / 5) + 1;
  const m = mp < 10 ? mp + 3 : mp - 9;
  return { y: y + (m <= 2 ? 1 : 0), m: m, d: d };
}

function daysFromCivil(y, m, d) {
  const yr = m <= 2 ? y - 1 : y;
  const era = Math.floor((yr >= 0 ? yr : yr - 399) / 400);
  const yoe = yr - era * 400;
  const doy = Math.floor((153 * (m + (m > 2 ? -3 : 9)) + 2) / 5) + d - 1;
  const doe = yoe * 365 + Math.floor(yoe / 4) - Math.floor(yoe / 100) + doy;
  return era * 146097 + doe - 719468;
}

function fmtDate(y, m, d) {
  return y + "-" + pad(m) + "-" + pad(d);
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const off = ctx.tzOffset || 0;
  const dayNum = Math.floor((ctx.now + off * 1000) / 86400000);
  const cur = civilFromDays(dayNum);

  const yearStart = fmtDate(cur.y, 1, 1);
  const nextYear = fmtDate(cur.y + 1, 1, 1);
  const daysToNextYear = Math.max(0, daysFromCivil(cur.y + 1, 1, 1) - dayNum);

  // Next month start & quarter end.
  const nmY = cur.m === 12 ? cur.y + 1 : cur.y;
  const nmM = cur.m === 12 ? 1 : cur.m + 1;
  const monthStart = fmtDate(cur.y, cur.m, 1);
  const nextMonth = fmtDate(nmY, nmM, 1);

  const qStartM = Math.floor((cur.m - 1) / 3) * 3 + 1;
  const qEndY = qStartM + 3 > 12 ? cur.y + 1 : cur.y;
  const qEndM = qStartM + 3 > 12 ? 1 : qStartM + 3;
  const qStart = fmtDate(cur.y, qStartM, 1);
  const nextQuarter = fmtDate(qEndY, qEndM, 1);

  // Next Saturday (1970-01-01 was Thursday = 4).
  const dow = (((dayNum + 4) % 7) + 7) % 7; // 0=Sun..6=Sat
  const toSat = dow === 6 ? 7 : 6 - dow;
  const sat = civilFromDays(dayNum + toSat);
  const nextSat = fmtDate(sat.y, sat.m, sat.d);

  const title = t("年度与节日倒计时", "Year & milestones");
  const view = (ctx.view && ctx.view.view) || 0;

  if (view === 1) {
    return {
      title: title,
      icon: "bell",
      big: EN ? daysToNextYear + "d" : daysToNextYear + " 天",
      sub: EN ? `Until ${cur.y + 1}` : `距离 ${cur.y + 1} 年`,
      lines: [],
      chart: {
        type: "countdown",
        countdown: [
          { label: t("下个周末", "Next weekend"), target: nextSat, note: t("周六", "Saturday"), color: "#22C55E", to: "#00F5D4", style: "neon" },
          { label: t("本月末", "End of month"), target: nextMonth, start: monthStart, note: fmtDate(cur.y, cur.m, 1).slice(0, 7), color: "#38BDF8", to: "#A855F7", style: "gradient" },
          { label: t("本季末", "End of quarter"), target: nextQuarter, start: qStart, note: "Q" + (Math.floor((cur.m - 1) / 3) + 1), color: "#A855F7", to: "#FF2A85", style: "gradient" },
          { label: EN ? `New Year ${cur.y + 1}` : `${cur.y + 1} 新年`, target: nextYear, start: yearStart, color: "#00F5D4", to: "#38BDF8", style: "neon" },
        ],
      },
      layout: { legend: "hide", summary: "hide" },
    };
  }

  return {
    title: title,
    icon: "bell",
    big: EN ? daysToNextYear + "d" : daysToNextYear + " 天",
    sub: "",
    lines: [],
    chart: {
      type: "countdown",
      countdown: [
        {
          label: EN ? `Countdown to ${cur.y + 1}` : `距离 ${cur.y + 1} 年`,
          target: nextYear,
          start: yearStart,
          note: t("年度时光进度", "Year progress"),
          color: "#00F5D4",
          to: "#38BDF8",
          style: "neon",
        },
      ],
    },
    layout: { legend: "hide", summary: "hide" },
  };
}
