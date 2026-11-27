// 体重打卡与目标：复用本机体重记录（ctx.metrics.weight），支持 84 天打卡热力格与霓虹目标进度条。
// Language: meta({lang}) and render(ctx) both get lang ("zh" or "en").
let EN = false;
const t = (zh, en) => (EN ? en : zh);

export function meta(a) {
  EN = !!a && a.lang === "en";
  return {
    name: t("体重打卡与目标", "Weight habit & goal"),
    description: t("用打卡热力格与霓虹进度条追踪称重坚持度与阶段目标", "Track weigh-in consistency with an activity grid and neon goal bars"),
    version: "1.0.0",
    platforms: ["desktop", "mobile"],
    needs: [],
    icon: "scale",
    inputs: [{ metric: "weight", label: t("体重", "Weight"), unit: "kg", step: 0.1 }],
    dims: [
      {
        key: "view",
        label: t("样式", "View"),
        options: [
          { value: 0, label: t("打卡热力格", "Habit grid") },
          { value: 1, label: t("霓虹目标", "Neon goals") },
        ],
      },
    ],
    sources: {},
  };
}

function dayIndex(ms, off) {
  return Math.floor((ms + (off || 0) * 1000) / 86400000);
}

function pad(n) {
  return (n < 10 ? "0" : "") + n;
}

function ymdFromDay(z) {
  z += 719468;
  const era = Math.floor((z >= 0 ? z : z - 146096) / 146097);
  const doe = z - era * 146097;
  const yoe = Math.floor((doe - Math.floor(doe / 1460) + Math.floor(doe / 36524) - Math.floor(doe / 146096)) / 365);
  const y = yoe + era * 400;
  const doy = doe - (365 * yoe + Math.floor(yoe / 4) - Math.floor(yoe / 100));
  const mp = Math.floor((5 * doy + 2) / 153);
  const d = doy - Math.floor((153 * mp + 2) / 5) + 1;
  const m = mp < 10 ? mp + 3 : mp - 9;
  const yr = y + (m <= 2 ? 1 : 0);
  return yr + "-" + pad(m) + "-" + pad(d);
}

export function render(ctx) {
  EN = ctx.lang === "en";
  const unit = ctx.settings && ctx.settings.weightUnit === "斤" ? "斤" : "kg";
  const k = unit === "斤" ? 2 : 1;
  const all = ((ctx.metrics && ctx.metrics.weight) || []).slice().sort((a, b) => a.t - b.t);
  const title = t("体重打卡与目标", "Weight habit & goal");

  if (all.length === 0) {
    return {
      title: title,
      icon: "scale",
      big: "--",
      sub: t("还没有体重记录", "No weight records yet"),
      lines: [t("在首页快速录入第一条体重即可生成打卡图", "Log your first weight to start tracking")],
      chart: null,
    };
  }

  const off = ctx.tzOffset || 0;
  const todayIdx = dayIndex(ctx.now, off);
  const counts = {};
  for (let i = 0; i < all.length; i++) {
    const d = dayIndex(all[i].t, off);
    counts[d] = (counts[d] || 0) + 1;
  }

  let weekDays = 0;
  for (let d = todayIdx - 6; d <= todayIdx; d++) {
    if (counts[d]) weekDays++;
  }
  let monthDays = 0;
  for (let d = todayIdx - 29; d <= todayIdx; d++) {
    if (counts[d]) monthDays++;
  }
  let streak = 0;
  for (let d = todayIdx; d >= todayIdx - 365; d--) {
    if (counts[d]) streak++;
    else if (d < todayIdx) break;
  }

  const first = all[0].v * k;
  const last = all[all.length - 1].v * k;
  const view = (ctx.view && ctx.view.view) || 0;

  if (view === 1) {
    // Stage target: 3 kg (or 6 斤) reduction from starting weight.
    const stepGoal = 3 * k;
    const lost = Math.max(0, first - last);
    const items = [
      {
        name: t("阶段减重目标", "Stage loss goal") + " (" + first.toFixed(1) + " → " + (first - stepGoal).toFixed(1) + " " + unit + ")",
        value: Math.round(lost * 10) / 10,
        max: stepGoal,
        unit: " " + unit,
        color: "#23537A",
      },
      {
        name: t("近 7 天打卡", "7-day check-ins"),
        value: weekDays,
        max: 7,
        unit: t(" 天", " d"),
        color: "#2A7B62",
      },
      {
        name: t("近 30 天打卡", "30-day check-ins"),
        value: monthDays,
        max: 30,
        unit: t(" 天", " d"),
        color: "#C26D2D",
      },
    ];
    return {
      title: title,
      icon: "scale",
      big: last.toFixed(1) + " " + unit,
      sub: EN ? `${monthDays}/30 days logged · streak ${streak}d` : `近 30 天打卡 ${monthDays} 天 · 连续 ${streak} 天`,
      lines: [],
      progressBar: {
        style: "neon",
        height: 8,
        track: "dark",
        glow: 0.9,
        items: items,
      },
      layout: { legend: "hide" },
    };
  }

  // Default: 84-day GitHub-style activity grid.
  const cells = [];
  for (let i = 83; i >= 0; i--) {
    const d = todayIdx - i;
    const n = counts[d] || 0;
    cells.push({ date: ymdFromDay(d), value: n });
  }
  return {
    title: title,
    icon: "scale",
    big: last.toFixed(1) + " " + unit,
    sub: EN ? `Streak ${streak}d · ${all.length} total logs` : `连续 ${streak} 天 · 累计 ${all.length} 次记录`,
    lines: [],
    chart: {
      type: "contrib",
      contrib: { color: "#2A7B62", max: 2, weeks: 12, unit: t(" 次", "x"), neon: false, cells: cells },
    },
    layout: { legend: "hide" },
  };
}
