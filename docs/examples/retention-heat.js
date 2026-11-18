// 留存热力表：每个注册月份一行，列是注册后第几个月的留存百分比。
// 一个格子是一个数字 0–100，颜色越深越高；没有数据的格子写 -1。
// 点格子可以选中它，底部显示它的详情。
export function meta() {
  return { name: "注册留存（示例）", icon: "users", sources: {} };
}

const cohorts = [
  ["Sep 2025", "674", [100, 61, 59, 54, 51, 47, 43, 39, 38, 36, 35, 33]],
  ["Oct 2025", "1.2K", [100, 63, 61, 57, 54, 51, 48, 45, 43, 42, 39]],
  ["Nov 2025", "764", [100, 56, 54, 53, 51, 48, 47, 46, 44, 43]],
  ["Dec 2025", "855", [100, 64, 63, 63, 59, 58, 54, 54, 52]],
  ["Jan 2026", "1.2K", [100, 66, 66, 63, 62, 59, 56, 56]],
  ["Feb 2026", "1.6K", [100, 65, 63, 63, 62, 60, 58]],
  ["Mar 2026", "1.5K", [100, 65, 65, 65, 64, 62]],
  ["Apr 2026", "1K", [100, 64, 63, 63, 62]],
  ["May 2026", "1.5K", [100, 59, 59, 59]],
  ["Jun 2026", "1.2K", [100, 58, 58]],
  ["Jul 2026", "1.8K", [100, 59]],
  ["Aug 2026", "1.3K", [100]],
];

export function render(ctx) {
  const columns = Array.from({ length: 12 }, (_, i) => String(i));
  const rows = cohorts.map(([label, users, cells]) => ({
    label,
    note: users,
    cells: columns.map((_, i) => (i < cells.length ? cells[i] : -1)),
  }));
  const average = columns.map((_, i) => {
    const vals = cohorts.filter((c) => c[2].length > i).map((c) => c[2][i]);
    return vals.length ? Math.round(vals.reduce((a, b) => a + b, 0) / vals.length) : -1;
  });
  return {
    title: "注册留存",
    big: "61.7%",
    sub: "第 1 个月平均留存",
    lines: [],
    chart: {
      type: "heat",
      grid: { columns: columns, rows: rows, average: average, footer: "12 个注册月份 · 14.5K 用户" },
    },
  };
}
