// 价格面板：一张价格的面积图，下面是 RSI（带 70 / 30 的参考线）、MACD 线和柱状图。
// 所有面板共用一根时间轴，悬浮时十字线会同时出现在每个面板上。
// 参考线是虚线的常数序列，所以不需要特别的样式。
export function meta() {
  return { name: "价格与指标（示例）", icon: "chart-line", sources: {} };
}

const N = 90;

function series(f) {
  return Array.from({ length: N }, (_, i) => f(i));
}

function ema(values, k) {
  const out = [];
  let prev = values[0];
  for (const v of values) {
    prev = v * k + prev * (1 - k);
    out.push(prev);
  }
  return out;
}

export function render(ctx) {
  const price = series((i) => 230 + 18 * Math.sin(i / 9) + 6 * Math.sin(i / 3.1) - i * 0.12);
  const rsi = series((i) => 50 + 16 * Math.sin(i / 7) + 4 * Math.sin(i / 2.3));
  const fast = ema(price, 2 / 13);
  const slow = ema(price, 2 / 27);
  const macd = fast.map((v, i) => v - slow[i]);
  const signal = ema(macd, 2 / 10);
  const hist = macd.map((v, i) => v - signal[i]);
  const flat = (v) => series(() => v);
  const labels = series((i) => `D${i + 1}`);

  return {
    title: "SOL · CLOSE",
    big: price[N - 1].toFixed(2),
    sub: "RSI " + rsi[N - 1].toFixed(1),
    lines: [],
    chart: {
      type: "panels",
      panels: [
        {
          title: "SOL · close",
          style: "area",
          weight: 3,
          series: [{ name: "close", color: "#E11D48", points: price }],
        },
        {
          title: "RSI 14",
          style: "line",
          weight: 1.2,
          series: [
            { name: "rsi", color: "#7C3AED", points: rsi },
            { name: "70", color: "#E11D48", points: flat(70), dashed: true },
            { name: "30", color: "#16A34A", points: flat(30), dashed: true },
          ],
        },
        {
          title: "MACD 12 · 26 · 9",
          style: "line",
          weight: 1.4,
          series: [
            { name: "macd", color: "#2563EB", points: macd },
            { name: "signal", color: "#F59E0B", points: signal, dashed: true },
          ],
        },
        {
          title: "histogram",
          style: "bar",
          weight: 0.8,
          series: [{ name: "hist", color: "#16A34A", points: hist }],
        },
      ],
    },
  };
}
