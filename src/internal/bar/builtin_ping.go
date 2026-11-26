package bar

// BuiltinPingBar is ScriptBoard's built-in desktop network ping monitor backed
// by the host-managed "net-ping" command, its persistent execution history, and
// customizable DisplayStyle rendering via $metrix.command.resolve().
const BuiltinPingBar = `<bar protocol-version="0" version="1.2.0" id="sb-ping" title="Ping Monitor"
     category="system" icon="activity"
     description="Quickly ping hosts from this machine to inspect RTT latency, jitter, packet loss, and history."
     refresh="60s" material="glass">
  <permissions>
    <command id="net-ping" />
  </permissions>
  <inputs>
    <input name="host" type="string" label="Target Host" default="223.5.5.5" help="Hostname or IP address (for example 223.5.5.5, 1.1.1.1, github.com)." required="true" />
    <input name="count" type="select" label="Ping Count" default="3">
      <option value="1" label="1 packet" />
      <option value="3" label="3 packets" />
      <option value="5" label="5 packets" />
    </input>
  </inputs>
  <script><![CDATA[
    const CMD_ID = "net-ping";
    const PRESETS = [
      { value: "223.5.5.5", label: "AliDNS" },
      { value: "1.1.1.1", label: "1.1.1.1" },
      { value: "8.8.8.8", label: "8.8.8.8" },
      { value: "github.com", label: "GitHub" },
    ];

    let activeHost = "";
    let draftHost = "";
    let customHost = false;
    let lastSeededInput = "";
    let pingCount = "3";
    let pending = false;
    let refreshStatus = "";
    let refreshTimer = null;
    let latestRec = null;
    let histRecs = [];
    let lastError = "";

    function clearTimer() {
      if (refreshTimer) {
        clearTimeout(refreshTimer);
        refreshTimer = null;
      }
    }

    function syncFromHostHistory() {
      try {
        const recs = $metrix.command.history(CMD_ID, 24);
        if (Array.isArray(recs)) {
          histRecs = recs;
        }
        const cur = $metrix.command.latest(CMD_ID);
        if (cur) {
          latestRec = cur;
        } else if (histRecs.length > 0) {
          latestRec = histRecs[0];
        } else {
          latestRec = null;
        }
      } catch (_) {}
    }

    function fmtMs(v, decimals) {
      if (typeof v !== "number" || !isFinite(v)) return "--";
      if (typeof decimals === "number" && decimals >= 0 && decimals <= 6) {
        return v.toFixed(decimals);
      }
      return v >= 100 ? String(Math.round(v)) : v.toFixed(1);
    }

    function trBuiltin(txt, zh) {
      if (zh || !txt) return txt;
      switch (txt) {
        case "极速": return "Fast";
        case "畅通": return "Healthy";
        case "偏高": return "Elevated";
        case "平均延迟": return "Avg RTT";
        case "丢包率": return "Loss";
        case "抖动": return "Jitter";
        case "最小": return "Min";
        case "最大": return "Max";
        case "解析 IP": return "Resolved IP";
        default: return txt;
      }
    }

    function fmtTime(ts) {
      if (!ts) return "";
      const d = new Date(ts);
      const p = n => (n < 10 ? "0" + n : String(n));
      return p(d.getHours()) + ":" + p(d.getMinutes()) + ":" + p(d.getSeconds());
    }

    async function runPing(targetHost, manual) {
      if (pending) return latestRec;
      const host = String(targetHost || activeHost || "223.5.5.5").trim() || "223.5.5.5";
      activeHost = host;
      draftHost = host;
      clearTimer();
      if (manual) refreshStatus = "";
      pending = true;
      $metrix.invalidate();

      try {
        const rec = await $metrix.command.run(CMD_ID, {
          host,
          count: String(pingCount || "3"),
        });
        latestRec = rec || null;
        syncFromHostHistory();
        if (rec && rec.ok) {
          lastError = "";
          if (manual) refreshStatus = "ok";
        } else {
          lastError = (rec && rec.error) ? rec.error : "Ping failed";
          if (manual) refreshStatus = "error";
        }
      } catch (err) {
        lastError = err && err.message ? err.message : String(err);
        syncFromHostHistory();
        if (manual) refreshStatus = "error";
      } finally {
        pending = false;
        $metrix.invalidate();
        if (refreshStatus) {
          refreshTimer = setTimeout(() => {
            refreshTimer = null;
            refreshStatus = "";
            $metrix.invalidate();
          }, 2500);
          if (refreshTimer && refreshTimer.unref) refreshTimer.unref();
        }
      }
      return latestRec;
    }

    function statusInfo(rec, zh) {
      if (!rec) {
        return { label: zh ? "待检测" : "Ready", tone: "textMuted", icon: "activity", tray: "--" };
      }
      const ext = rec.extracted || {};
      const avg = typeof ext.avgMs === "number" ? ext.avgMs : null;
      const loss = typeof ext.lossPct === "number" ? ext.lossPct : (rec.ok ? 0 : 100);
      if (!rec.ok || avg === null) {
        return {
          label: zh ? "不可达" : "Unreachable",
          tone: "danger",
          icon: "wifi-off",
          tray: "ERR",
        };
      }
      const trayStr = Math.round(avg) + "ms";
      if (loss > 0) {
        return {
          label: zh ? "丢包波动" : "Packet loss",
          tone: "warning",
          icon: "alert-triangle",
          tray: trayStr,
        };
      }
      if (avg >= 120) {
        return {
          label: zh ? "延迟偏高" : "Elevated",
          tone: "warning",
          icon: "activity",
          tray: trayStr,
        };
      }
      return {
        label: zh ? "网络畅通" : "Healthy",
        tone: "success",
        icon: "wifi",
        tray: trayStr,
      };
    }

    function triggerBadgeLabel(trig, zh) {
      switch (trig) {
        case "schedule": return zh ? "定时" : "Auto";
        case "mcp": return "MCP";
        case "script": return zh ? "组件" : "Card";
        case "manual":
        case "bar":
        default: return zh ? "手动" : "Manual";
      }
    }

    function bareUnitText(f) {
      if (!f) return "";
      if (f.value == null || f.value === "--") return String(f.value || "--");
      let out = (f.prefix || "") + String(f.value);
      if (f.unit) out += (f.unit === "%" ? "%" : " " + f.unit);
      return out + (f.suffix || "");
    }

    export default {
      async scheduled(env) {
        const inputs = (env && env.inputs) || {};
        const inHost = String(inputs.host || "223.5.5.5").trim() || "223.5.5.5";
        if (!customHost) {
          activeHost = inHost;
          draftHost = inHost;
        }
        lastSeededInput = inHost;
        pingCount = String(inputs.count || "3");
        syncFromHostHistory();
        if (env && env.offline) {
          $metrix.invalidate();
          return;
        }
        await runPing(activeHost, false);
      },

      render(env) {
        const zh = !!(env && env.lang === "zh");
        const inputs = (env && env.inputs) || {};
        const inHost = String(inputs.host || "223.5.5.5").trim() || "223.5.5.5";
        if (!activeHost || (!customHost && inHost !== lastSeededInput)) {
          activeHost = inHost;
          draftHost = inHost;
          lastSeededInput = inHost;
        }
        pingCount = String(inputs.count || "3");
        if (!pending) {
          syncFromHostHistory();
        }

        const st = statusInfo(latestRec, zh);
        const ext = (latestRec && latestRec.extracted) || {};
        const hasAvg = latestRec && latestRec.ok && typeof ext.avgMs === "number";

        const resolvedCmd = ($metrix.command && typeof $metrix.command.resolve === "function")
          ? ($metrix.command.resolve(CMD_ID, latestRec) || null)
          : null;
        const fieldsMap = (resolvedCmd && resolvedCmd.fields) || {};
        const primaryKey = (resolvedCmd && resolvedCmd.primaryField) || "avgMs";
        const heroStyle = (resolvedCmd && resolvedCmd.primary) || fieldsMap[primaryKey] || null;

        const minStyle = fieldsMap.minMs || null;
        const maxStyle = fieldsMap.maxMs || null;
        const jitStyle = fieldsMap.jitterMs || null;
        const lossStyle = fieldsMap.lossPct || null;

        const minStr = minStyle ? (minStyle.formattedValue || "--") : fmtMs(ext.minMs);
        const maxStr = maxStyle ? (maxStyle.formattedValue || "--") : fmtMs(ext.maxMs);
        const jitStr = jitStyle ? (jitStyle.formattedValue || "--") : fmtMs(ext.jitterMs);
        const lossStr = lossStyle && lossStyle.formattedValue && lossStyle.formattedValue !== "--"
          ? lossStyle.formattedValue
          : (typeof ext.lossPct === "number" ? ext.lossPct.toFixed(0) + "%" : (latestRec && !latestRec.ok ? "100%" : "--"));

        const hasPrimary = latestRec && heroStyle && (heroStyle.raw != null || (ext[primaryKey] != null));
        let heroValueStr = heroStyle && heroStyle.value ? String(heroStyle.value) : "--";
        let heroUnitStr = "";
        if (hasPrimary && heroStyle) {
          const fmtVal = String(heroStyle.formattedValue || "");
          if (fmtVal && fmtVal !== bareUnitText(heroStyle)) {
            heroValueStr = fmtVal;
            heroUnitStr = "";
          } else {
            heroValueStr = (heroStyle.prefix || "") + String(heroStyle.value || "--") + (heroStyle.suffix || "");
            heroUnitStr = heroStyle.unit || (primaryKey === "avgMs" ? "ms" : "");
          }
        }
        const heroTone = (heroStyle && heroStyle.color) || st.tone;
        const statusText = trBuiltin((heroStyle && heroStyle.statusLabel) || "", zh);
        const heroLabel = (hasPrimary && statusText) ? statusText : st.label;

        let effAnim = "none";
        if (pending) {
          effAnim = "dots";
        } else if (latestRec) {
          effAnim = (heroStyle && heroStyle.animation && heroStyle.animation !== "none") ? heroStyle.animation : "ripple";
        }

        const heroDotNode = (pending || (latestRec && effAnim !== "none"))
          ? $ui.commandDot(heroStyle, {
              pending,
              animation: effAnim,
              fallbackColor: heroTone,
              label: heroLabel,
            })
          : null;

        const onRefresh = () => { void runPing(activeHost, true); };
        const onPickPreset = val => {
          customHost = true;
          activeHost = val;
          draftHost = val;
          void runPing(val, true);
        };
        const onSubmitHost = val => {
          const next = String(val != null ? val : draftHost).trim();
          if (!next) return;
          customHost = true;
          activeHost = next;
          draftHost = next;
          void runPing(next, true);
        };

        const pts = [];
        const rev = histRecs.slice(0, 20).reverse();
        for (let i = 0; i < rev.length; i++) {
          const r = rev[i];
          const re = (r && r.extracted) || {};
          if (typeof re.avgMs === "number") {
            pts.push({ x: pts.length, y: re.avgMs });
          }
        }

        const recentRows = histRecs.slice(0, 4).map((r, idx) => {
          const re = (r && r.extracted) || {};
          const rHost = (r && r.params && r.params.host) || activeHost;
          const rRes = ($metrix.command && typeof $metrix.command.resolve === "function")
            ? $metrix.command.resolve(CMD_ID, r)
            : null;
          const rp = (rRes && rRes.primary) || null;
          const rVal = (r && r.ok)
            ? (rp && rp.formattedValue && rp.formattedValue !== "--" ? rp.formattedValue : (typeof re.avgMs === "number" ? fmtMs(re.avgMs) + " ms" : "OK"))
            : (zh ? "失败" : "Err");
          const rTone = (r && r.ok) ? ((rp && rp.color) || "success") : "danger";
          const rowAnim = idx === 0 && effAnim !== "none" ? (effAnim === "matrix" || effAnim === "orbit" ? "pulse" : effAnim) : "none";
          return h("Row", { key: r.id || String(idx), fillWidth: true, height: 22, alignItems: "center", gap: 6 },
            rowAnim !== "none"
              ? $ui.dot(rTone, { animation: rowAnim, size: 6, count: 3, speed: (heroStyle && heroStyle.speed) || 1 })
              : $ui.dot(rTone, false, 6),
            h("Text", { fontSize: 11, fontWeight: 500, singleLine: true, grow: 1, shrink: 1, minWidth: 0 }, rHost),
            h("Text", { fontSize: 10, textColor: "textMuted", singleLine: true }, triggerBadgeLabel(r.trigger, zh)),
            h("Text", { fontSize: 10, textColor: "textMuted", fontFeatures: "tnum", singleLine: true }, fmtTime(r.t)),
            h("Text", { fontSize: 11, fontWeight: 600, fontFeatures: "tnum", textColor: rTone, singleLine: true }, rVal)
          );
        });

        const segValue = PRESETS.some(x => x.value === activeHost) ? activeHost : "";
        const errBanner = lastError ? $ui.staleError(lastError) : (latestRec && !latestRec.ok && latestRec.error ? $ui.staleError(latestRec.error) : null);
        const resolvedSub = ext.resolvedIp && String(ext.resolvedIp) !== activeHost
          ? activeHost + " (" + ext.resolvedIp + ")"
          : activeHost;

        const trayMenu = PRESETS.map(pr => ({
          label: "Ping " + pr.value,
          onClick: () => { onPickPreset(pr.value); },
        }));
        trayMenu.unshift({ label: zh ? "立即 Ping" : "Ping Now", onClick: onRefresh });

        const tip = latestRec
          ? activeHost + " · " + heroLabel + (hasAvg ? " (" + fmtMs(ext.avgMs) + " ms)" : "")
          : (zh ? "网络 Ping · " : "Ping Monitor · ") + activeHost;

        const variant = (heroStyle && heroStyle.variant) || "default";
        const extraVariantBar = (hasPrimary && (variant === "matrix" || variant === "bar"))
          ? h("Progress", {
              value: (heroStyle && heroStyle.fillFrac > 0) ? heroStyle.fillFrac * 100 : (latestRec && latestRec.ok ? 65 : 15),
              max: 100,
              style: variant === "matrix" ? "matrix" : "gradient",
              color: heroTone || "accent",
              animate: effAnim !== "none",
              height: variant === "matrix" ? 12 : 6,
            })
          : null;

        return {
          tray: {
            title: st.tray,
            icon: "lucide:activity",
            iconTemplate: true,
            tooltip: tip,
            error: latestRec && !latestRec.ok ? (latestRec.error || "Ping failed") : "",
            menu: trayMenu,
          },
          panel: h(
            "Column",
            {
              preferredWidth: 300,
              preferredHeight: 440,
              fill: true,
              minHeight: 0,
              safeArea: true,
              padding: [10, 16, 16, 16],
              gap: 10,
              shortcuts: { "CmdOrCtrl+R": onRefresh },
            },
            $ui.topBar({
              icon: "activity",
              iconColor: "accent",
              title: zh ? "网络 Ping" : "Ping Monitor",
              subtitle: resolvedSub,
              refreshStatus: pending ? "loading" : refreshStatus,
              refreshTip: pending ? (zh ? "正在检测…" : "Pinging...") : (refreshStatus === "ok" ? (zh ? "检测完成" : "Updated") : (refreshStatus === "error" ? (zh ? "检测失败" : "Ping failed") : (zh ? "立即检测" : "Ping now"))),
              onRefresh,
            }),
            h("Segmented", {
              value: segValue,
              options: PRESETS,
              onChange: onPickPreset,
            }),
            h("Row", { fillWidth: true, gap: 6, alignItems: "center" },
              h("Input", {
                grow: 1,
                shrink: 1,
                minWidth: 0,
                fillWidth: true,
                value: draftHost,
                placeholder: zh ? "输入主机或 IP…" : "Host or IP…",
                onChange: v => { draftHost = String(v == null ? "" : v); },
                onSubmit: onSubmitHost,
              }),
              h("Button", {
                variant: "primary",
                shrink: 0,
                width: 62,
                disabled: !!pending,
                onClick: () => onSubmitHost(draftHost),
              }, "Ping")
            ),
            errBanner,
            $ui.hero({
              value: heroValueStr,
              unit: heroUnitStr,
              fontSize: heroValueStr.length > 8 ? 28 : 42,
              tone: (heroStyle && heroStyle.color) || undefined,
              dot: heroDotNode,
              label: (variant === "badge" && heroDotNode) ? "" : heroLabel,
              secondary: latestRec ? fmtTime(latestRec.t) : (zh ? "尚未运行" : "No runs yet"),
              icon: st.icon,
              iconColor: heroTone,
              iconSize: 38,
              metrics: [
                { icon: "arrow-down", value: (trBuiltin(minStyle && minStyle.label, zh) || (zh ? "最小" : "Min")) + " " + minStr, tone: (minStyle && minStyle.color) || undefined, tooltip: "Min RTT (ms)" },
                { icon: "arrow-up", value: (trBuiltin(maxStyle && maxStyle.label, zh) || (zh ? "最大" : "Max")) + " " + maxStr, tone: (maxStyle && maxStyle.color) || undefined, tooltip: "Max RTT (ms)" },
                { icon: "activity", value: (trBuiltin(jitStyle && jitStyle.label, zh) || (zh ? "抖动" : "Jitter")) + " " + jitStr, tone: (jitStyle && jitStyle.color) || undefined, tooltip: "Jitter (ms)" },
                { icon: "shield-alert", value: (trBuiltin(lossStyle && lossStyle.label, zh) || (zh ? "丢包" : "Loss")) + " " + lossStr, tone: (lossStyle && lossStyle.color) || (ext.lossPct > 0 ? "danger" : "text"), tooltip: "Packet loss" },
              ],
            }),
            extraVariantBar,
            pts.length >= 2 ? h("Column", { fillWidth: true, gap: 4 },
              $ui.divider(),
              h("Chart", {
                type: "area",
                height: 44,
                fillWidth: true,
                axes: false,
                grid: false,
                hover: false,
                curve: "monotone",
                strokeWidth: 2,
                series: [{
                  name: "RTT",
                  color: "accent",
                  fill: { from: 0.22, to: 0 },
                  points: pts,
                }],
              })
            ) : null,
            $ui.divider(),
            $ui.section(zh ? "最近记录" : "Recent runs",
              recentRows.length > 0
                ? recentRows
                : h("Text", { fontSize: 12, textColor: "textMuted" }, zh ? "点击 Ping 或刷新按钮开始检测本机网络" : "Click Ping or refresh to test connectivity")
            )
          ),
        };
      },
    };
  ]]></script>
</bar>
`
