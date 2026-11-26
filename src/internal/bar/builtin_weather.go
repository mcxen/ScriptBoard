package bar

// BuiltinWeatherBar is ScriptBoard's MIT-licensed built-in Weather .bar
// component backed by Open-Meteo geocoding and forecast APIs.
const BuiltinWeatherBar = `<bar protocol-version="0" version="1.0.0" id="sb-weather" title="Weather"
     category="weather" icon="cloud.sun" description="Current conditions, 12-hour and 6-day forecasts for any city, via Open-Meteo."
     license="MIT" homepage="https://open-meteo.com/" refresh="15m" material="glass">
  <permissions>
    <network origin="https://api.open-meteo.com" />
    <network origin="https://geocoding-api.open-meteo.com" />
  </permissions>
  <inputs>
    <input name="location" type="string" label="Location" default="Hangzhou" help="City name, or coordinates such as 52.52,13.41." required="true" />
    <input name="units" type="select" label="Units" default="metric">
      <option value="metric" label="Metric (°C, km/h)" />
      <option value="imperial" label="Imperial (°F, mph)" />
    </input>
  </inputs>
  <script apis="shell"><![CDATA[
    const h = (type, props, ...children) => ({
      type,
      props: props || {},
      children: children.flat().filter(c => c !== null && c !== undefined && c !== false),
    });

    const mkSvg = inner =>
      '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' + inner + '</svg>';
    const p = d => '<path d="' + d + '"/>';
    const CLOUD_ARC = "M4 14.899A7 7 0 1 1 15.71 8h1.79a4.5 4.5 0 0 1 2.5 8.242";

    const GLYPHS = {
      sun: mkSvg('<circle cx="12" cy="12" r="4"/>' + p("M12 2v2m0 16v2M4.93 4.93l1.41 1.41m11.32 11.32l1.41 1.41M2 12h2m16 0h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41")),
      moon: mkSvg(p("M20.985 12.486a9 9 0 1 1-9.473-9.472c.405-.022.617.46.402.803a6 6 0 0 0 8.268 8.268c.344-.215.825-.004.803.401")),
      cloudSun: mkSvg(p("M12 2v2m-7.07.93l1.41 1.41M20 12h2m-2.93-7.07l-1.41 1.41m-1.713 6.31a4 4 0 0 0-5.925-4.128M13 22H7a5 5 0 1 1 4.9-6H13a3 3 0 0 1 0 6")),
      cloudMoon: mkSvg(p("M13 16a3 3 0 0 1 0 6H7a5 5 0 1 1 4.9-6zm5.376-1.488a6 6 0 0 0 3.461-4.127c.148-.625-.659-.97-1.248-.714a4 4 0 0 1-5.259-5.26c.255-.589-.09-1.395-.716-1.248a6 6 0 0 0-4.594 5.36")),
      cloud: mkSvg(p("M17.5 19H9a7 7 0 1 1 6.71-9h1.79a4.5 4.5 0 1 1 0 9")),
      fog: mkSvg(p(CLOUD_ARC + "M16 17H7m10 4H9")),
      drizzle: mkSvg(p(CLOUD_ARC + "M8 19v1m0-6v1m8 4v1m0-6v1m-4 6v1m0-6v1")),
      rain: mkSvg(p(CLOUD_ARC + "M16 14v6m-8-6v6m4-4v6")),
      snow: mkSvg(p(CLOUD_ARC + "M8 15h.01M8 19h.01M12 17h.01M12 21h.01M16 15h.01M16 19h.01")),
      storm: mkSvg(p("M6 16.326A7 7 0 1 1 15.71 8h1.79a4.5 4.5 0 0 1 .5 8.973") + p("m13 12l-3 5h4l-3 5")),
      feels: mkSvg(p("M14 4v10.54a4 4 0 1 1-4 0V4a2 2 0 0 1 4 0")),
      wind: mkSvg(p("M12.8 19.6A2 2 0 1 0 14 16H2m15.5-8a2.5 2.5 0 1 1 2 4H2m7.8-7.6A2 2 0 1 1 11 8H2")),
      humidity: mkSvg(p("M7 16.3c2.2 0 4-1.83 4-4.05c0-1.16-.57-2.26-1.71-3.19S7.29 6.75 7 5.3c-.29 1.45-1.14 2.84-2.29 3.76S3 11.1 3 12.25c0 2.22 1.8 4.05 4 4.05") + p("M12.56 6.6A11 11 0 0 0 14 3.02c.5 2.5 2 4.9 4 6.5s3 3.5 3 5.5a6.98 6.98 0 0 1-11.91 4.97")),
      rainChance: mkSvg(p("M12 13v7a2 2 0 0 0 4 0M12 2v2") + p("M20.992 13a1 1 0 0 0 .97-1.274a10.284 10.284 0 0 0-19.923 0A1 1 0 0 0 3 13z")),
      refresh: mkSvg(p("M3 12a9 9 0 0 1 9-9a9.75 9.75 0 0 1 6.74 2.74L21 8") + p("M21 3v5h-5m5 4a9 9 0 0 1-9 9a9.75 9.75 0 0 1-6.74-2.74L3 16") + p("M8 16H3v5")),
      check: mkSvg(p("M20 6 9 17l-5-5")),
      pin: mkSvg(p("M20 10c0 4.993-5.539 10.193-7.399 11.799a1 1 0 0 1-1.202 0C9.539 20.193 4 14.993 4 10a8 8 0 0 1 16 0") + '<circle cx="12" cy="10" r="3"/>'),
      alert: mkSvg('<circle cx="12" cy="12" r="10"/>' + p("M12 8v4m0 4h.01")),
      external: mkSvg(p("M7 7h10v10M7 17L17 7")),
    };

    function conditionFor(code, isDay) {
      if (code === 0) {
        return isDay
          ? { label: "Clear", icon: GLYPHS.sun, tint: "warning", tray: "lucide:sun" }
          : { label: "Clear", icon: GLYPHS.moon, tint: null, tray: "lucide:moon" };
      }
      if (code <= 2) {
        return {
          label: code === 1 ? "Mostly clear" : "Partly cloudy",
          icon: isDay ? GLYPHS.cloudSun : GLYPHS.cloudMoon,
          tint: isDay ? "warning" : null,
          tray: isDay ? "lucide:cloud-sun" : "lucide:cloud-moon",
        };
      }
      if (code === 3) return { label: "Overcast", icon: GLYPHS.cloud, tint: null, tray: "lucide:cloud" };
      if (code <= 48) return { label: "Fog", icon: GLYPHS.fog, tint: null, tray: "lucide:cloud-fog" };
      if (code <= 57) return { label: "Drizzle", icon: GLYPHS.drizzle, tint: "accent", tray: "lucide:cloud-drizzle" };
      if (code <= 67 || (code >= 80 && code <= 82)) {
        return { label: code >= 80 ? "Showers" : "Rain", icon: GLYPHS.rain, tint: "accent", tray: "lucide:cloud-rain" };
      }
      if (code <= 77 || code === 85 || code === 86) {
        return { label: "Snow", icon: GLYPHS.snow, tint: null, tray: "lucide:cloud-snow" };
      }
      return { label: "Thunderstorm", icon: GLYPHS.storm, tint: "warning", tray: "lucide:cloud-lightning" };
    }

    const PANEL_W = 300;
    const PAD = 16;
    const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
    const fmtDeg = v => Math.round(v) + "°";
    const hourText = iso => String(iso || "").slice(11, 13);
    const dayText = (iso, idx) => idx === 0 ? "Today" : WEEKDAYS[new Date(iso + "T12:00:00Z").getUTCDay()];

    let state = null;
    let lastError = null;
    let pending = null;
    let refreshStatus = "";
    let refreshTimer = null;
    const geoCache = new Map();

    function mustNum(v) {
      if (typeof v !== "number" || !Number.isFinite(v)) {
        throw new Error("Invalid weather payload.");
      }
      return v;
    }

    function maybeProb(v) {
      return typeof v === "number" && Number.isFinite(v) ? v : NaN;
    }

    async function fetchJSON(url) {
      const resp = await fetch(url);
      const bodyText = await resp.text();
      if (!resp.ok) {
        throw new Error("Open-Meteo HTTP " + resp.status);
      }
      try {
        return JSON.parse(bodyText);
      } catch (_) {
        throw new Error("Malformed JSON from weather service.");
      }
    }

    const COORD_RE = /^\s*(-?\d+(?:\.\d+)?)\s*,\s*(-?\d+(?:\.\d+)?)\s*$/;
    async function resolvePlace(query) {
      const m = COORD_RE.exec(query);
      if (m) {
        const lat = Number(m[1]);
        const lon = Number(m[2]);
        if (Math.abs(lat) > 90 || Math.abs(lon) > 180) {
          throw new Error("Coordinates out of range.");
        }
        return { name: lat.toFixed(2) + ", " + lon.toFixed(2), region: "", lat, lon };
      }
      if (geoCache.has(query)) {
        return geoCache.get(query);
      }
      const data = await fetchJSON(
        "https://geocoding-api.open-meteo.com/v1/search?name=" + encodeURIComponent(query) + "&count=1&language=en&format=json"
      );
      const first = data && Array.isArray(data.results) ? data.results[0] : null;
      if (!first || !Number.isFinite(first.latitude) || !Number.isFinite(first.longitude)) {
        throw new Error("Location not found: " + query);
      }
      const place = {
        name: first.name || query,
        region: [first.admin1, first.country_code].filter(Boolean).join(", "),
        lat: first.latitude,
        lon: first.longitude,
      };
      geoCache.set(query, place);
      return place;
    }

    async function fetchWeather(env) {
      const inputs = (env && env.inputs) || {};
      const query = String(inputs.location ?? "").trim();
      if (!query) {
        throw new Error("Please configure a location.");
      }
      const imperial = inputs.units === "imperial";
      const place = await resolvePlace(query);
      const unitParam = imperial ? "&temperature_unit=fahrenheit&wind_speed_unit=mph" : "";
      const payload = await fetchJSON(
        "https://api.open-meteo.com/v1/forecast?latitude=" + place.lat + "&longitude=" + place.lon +
        "&current=temperature_2m,apparent_temperature,relative_humidity_2m,weather_code,is_day,wind_speed_10m" +
        "&hourly=temperature_2m,weather_code,is_day,precipitation_probability" +
        "&daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max" +
        "&timezone=auto&forecast_days=6&forecast_hours=13" +
        unitParam
      );
      const cur = payload && payload.current;
      const hr = payload && payload.hourly;
      const dl = payload && payload.daily;
      if (!cur || !hr || !dl || !Array.isArray(hr.time) || !Array.isArray(dl.time)) {
        throw new Error("Incomplete weather payload.");
      }
      const hours = hr.time.map((t, i) => ({
        t,
        temp: mustNum(hr.temperature_2m?.[i]),
        code: mustNum(hr.weather_code?.[i]),
        day: hr.is_day?.[i] !== 0,
        rain: maybeProb(hr.precipitation_probability?.[i]),
      }));
      const days = dl.time.map((t, i) => ({
        t,
        code: mustNum(dl.weather_code?.[i]),
        hi: mustNum(dl.temperature_2m_max?.[i]),
        lo: mustNum(dl.temperature_2m_min?.[i]),
        rain: maybeProb(dl.precipitation_probability_max?.[i]),
      }));
      if (hours.length < 2 || days.length === 0) {
        throw new Error("Empty forecast range.");
      }
      return {
        place,
        imperial,
        hours,
        days,
        temp: mustNum(cur.temperature_2m),
        feels: mustNum(cur.apparent_temperature),
        humidity: mustNum(cur.relative_humidity_2m),
        wind: mustNum(cur.wind_speed_10m),
        code: mustNum(cur.weather_code),
        day: cur.is_day !== 0,
      };
    }

    function triggerLoad(env, manual) {
      if (pending) return pending;
      if (refreshTimer) {
        clearTimeout(refreshTimer);
        refreshTimer = null;
      }
      refreshStatus = "";
      const trackOutcome = !!manual;
      pending = fetchWeather(env)
        .then(next => {
          state = next;
          lastError = null;
          if (trackOutcome) refreshStatus = "ok";
        })
        .catch(err => {
          lastError = String((err && err.message) || err);
          if (trackOutcome) refreshStatus = "error";
        })
        .finally(() => {
          pending = null;
          $metrix.invalidate();
          if (refreshStatus) {
            refreshTimer = setTimeout(() => {
              refreshTimer = null;
              refreshStatus = "";
              $metrix.invalidate();
            }, 2500);
            if (refreshTimer && refreshTimer.unref) refreshTimer.unref();
          }
        });
      $metrix.invalidate();
      return pending;
    }

    const iconNode = (svgStr, size, tint) =>
      h("Icon", { svg: svgStr, fontSize: size, width: size, height: size, shrink: 0, ...(tint ? { textColor: tint } : {}) });
    const mutedText = (txt, props) =>
      h("Text", { fontSize: 12, textColor: "textMuted", ...props }, txt);
    const skelBox = (w, hgt, extra) =>
      h("Box", { width: w, height: hgt, radius: 4, background: "surfaceHover", ...extra });

    function actionBtn(svgStr, tip, onClick, spin, tint, bg) {
      return h(
        "Column",
        {
          width: 26,
          height: 26,
          shrink: 0,
          radius: 7,
          alignItems: "center",
          justify: "center",
          ...(bg ? { background: bg } : {}),
          hover: { background: "surfaceHover" },
          pressed: { background: "surfacePressed" },
          tooltip: tip,
          ...(onClick ? { onClick } : {}),
        },
        h("Icon", {
          svg: svgStr,
          fontSize: 14,
          width: 14,
          height: 14,
          shrink: 0,
          textColor: tint || "textMuted",
          ...(spin ? { keyframes: { duration: 850, repeat: -1, ease: "linear", frames: [{ at: 0, rotate: 0 }, { at: 1, rotate: 360 }] } } : {}),
        })
      );
    }

    function refreshActionBtn(onRefresh) {
      if (pending) {
        return actionBtn(GLYPHS.refresh, "Refreshing...", null, true, "accent", "accentSurface");
      }
      if (refreshStatus === "ok") {
        return actionBtn(GLYPHS.check, "Updated", onRefresh, false, "success", "successSurface");
      }
      if (refreshStatus === "error") {
        return actionBtn(GLYPHS.alert, lastError ? "Refresh failed: " + lastError : "Refresh failed", onRefresh, false, "danger", "dangerSurface");
      }
      return actionBtn(GLYPHS.refresh, "Refresh", onRefresh, false);
    }

    function headerRow(place, fallbackName, onRefresh) {
      return h(
        "Row",
        { alignItems: "center", gap: 6, height: 26 },
        iconNode(GLYPHS.pin, 13, "textMuted"),
        h("Text", { fontSize: 13, fontWeight: 600, singleLine: true }, place ? place.name : fallbackName || "Weather"),
        place && place.region ? mutedText(place.region, { singleLine: true, shrink: 1 }) : null,
        h("Spacer"),
        actionBtn(GLYPHS.external, "Weather data by Open-Meteo.com", () => $metrix.openURL("https://open-meteo.com/")),
        refreshActionBtn(onRefresh)
      );
    }

    function heroBlock(d) {
      const cond = conditionFor(d.code, d.day);
      const today = d.days[0];
      const metric = (svgStr, val, tip) =>
        h("Row", { gap: 4, alignItems: "center", tooltip: tip },
          iconNode(svgStr, 12, "textMuted"),
          h("Text", { fontSize: 12, fontWeight: 500, fontFeatures: "tnum" }, val)
        );
      return h(
        "Column",
        { gap: 10 },
        h(
          "Row",
          { alignItems: "center", gap: 12 },
          h(
            "Column",
            { grow: 1, gap: 2 },
            h("Text", { fontSize: 52, fontWeight: 300, letterSpacing: -2, fontFeatures: "tnum", contentTransition: { numeric: true } }, fmtDeg(d.temp)),
            h(
              "Row",
              { gap: 8 },
              h("Text", { fontSize: 13, fontWeight: 500 }, cond.label),
              mutedText(fmtDeg(today.hi) + " / " + fmtDeg(today.lo), { fontFeatures: "tnum" })
            )
          ),
          iconNode(cond.icon, 52, cond.tint)
        ),
        h(
          "Row",
          { gap: 14 },
          metric(GLYPHS.feels, fmtDeg(d.feels), "Feels like"),
          metric(GLYPHS.wind, Math.round(d.wind) + " " + (d.imperial ? "mph" : "km/h"), "Wind"),
          metric(GLYPHS.humidity, Math.round(d.humidity) + "%", "Humidity"),
          metric(GLYPHS.rainChance, Number.isFinite(today.rain) ? Math.round(today.rain) + "%" : "—", "Chance of rain today")
        )
      );
    }

    function hourlyBlock(d) {
      const cols = d.hours.filter((_, i) => i % 2 === 0).slice(0, 7);
      const showWet = cols.some(x => x.rain >= 30);
      const colNode = (x, i) => {
        const cond = conditionFor(x.code, x.day);
        return h(
          "Column",
          { grow: 1, basis: 0, alignItems: "center", gap: 5 },
          mutedText(i === 0 ? "Now" : hourText(x.t), { fontSize: 11, fontFeatures: "tnum" }),
          iconNode(cond.icon, 18, cond.tint),
          h("Text", { fontSize: 13, fontWeight: 600, fontFeatures: "tnum" }, fmtDeg(x.temp)),
          showWet ? h("Text", { fontSize: 10, fontWeight: 500, textColor: "accent", fontFeatures: "tnum" }, x.rain >= 30 ? Math.round(x.rain) + "%" : " ") : null
        );
      };
      return h(
        "Column",
        { gap: 2 },
        h("Row", {}, cols.map(colNode)),
        h("Chart", {
          type: "area",
          height: 30,
          fillWidth: true,
          axes: false,
          grid: false,
          hover: false,
          curve: "monotone",
          strokeWidth: 2,
          xDomain: [-0.5, cols.length - 0.5],
          series: [{
            name: "Temperature",
            color: "accent",
            fill: { from: 0.16, to: 0 },
            points: d.hours.slice(0, 13).map((x, i) => ({ x: i / 2, y: x.temp })),
          }],
        })
      );
    }

    function dailyBlock(d) {
      const minT = Math.min(...d.days.map(x => x.lo));
      const maxT = Math.max(...d.days.map(x => x.hi));
      const span = Math.max(maxT - minT, 1);
      return h(
        "Column",
        { gap: 2 },
        d.days.map((x, i) => {
          const cond = conditionFor(x.code, true);
          return h(
            "Row",
            { key: x.t, height: 26, alignItems: "center", gap: 8 },
            h("Text", { width: 44, fontSize: 13, fontWeight: i === 0 ? 600 : 500 }, dayText(x.t, i)),
            h("Row", { width: 22, justify: "center" }, iconNode(cond.icon, 16, cond.tint)),
            h("Text", { width: 30, fontSize: 11, fontWeight: 500, textColor: "accent", fontFeatures: "tnum" }, x.rain >= 30 ? Math.round(x.rain) + "%" : ""),
            h("Text", { width: 28, fontSize: 13, textColor: "textMuted", textAlign: "end", fontFeatures: "tnum" }, fmtDeg(x.lo)),
            h(
              "Box",
              { grow: 1, height: 4, radius: 2, background: "surfaceHover" },
              h("Box", {
                absolute: true,
                top: 0,
                bottom: 0,
                radius: 2,
                background: "accent",
                leftPercent: ((x.lo - minT) / span) * 100,
                widthPercent: Math.max(((x.hi - x.lo) / span) * 100, 4),
              })
            ),
            h("Text", { width: 28, fontSize: 13, fontWeight: 600, textAlign: "end", fontFeatures: "tnum" }, fmtDeg(x.hi))
          );
        })
      );
    }

    function skeletonView() {
      return [
        h("Row", { alignItems: "center", gap: 12 },
          h("Column", { grow: 1, gap: 10 }, skelBox(96, 52), skelBox(140, 14)),
          skelBox(52, 52, { radius: 26 })
        ),
        h("Row", { gap: 14 }, [0, 1, 2, 3].map(() => skelBox(44, 14))),
        h("Row", { justify: "spaceBetween", padding: [0, 6] }, [0, 1, 2, 3, 4, 5, 6].map(() => skelBox(24, 52, { radius: 6 }))),
        h("Column", { gap: 12 }, [0, 1, 2, 3, 4, 5].map(() => skelBox(268, 16))),
      ];
    }

    function errorView(message, retry) {
      return h(
        "Column",
        { grow: 1, alignItems: "center", justify: "center", gap: 8, padding: [0, 12] },
        iconNode(GLYPHS.alert, 22, "danger"),
        h("Text", { fontSize: 13, fontWeight: 600, textAlign: "center", wrap: true }, message),
        h("Button", { onClick: retry }, "Try again")
      );
    }

    const divider = () => h("Divider", {});

    export default {
      async scheduled(env) {
        await triggerLoad(env);
      },
      render(env) {
        const refresh = () => { void triggerLoad(env, true); };
        const cond = state ? conditionFor(state.code, state.day) : null;
        const tray = {
          title: state ? fmtDeg(state.temp) : "",
          ...(cond ? { icon: cond.tray, iconTemplate: true } : {}),
          tooltip: state ? state.place.name + " · " + cond.label : "Weather",
          error: lastError || "",
          menu: [{ label: "Refresh", onClick: refresh }],
        };
        let sections;
        if (state) {
          sections = [
            lastError
              ? h("Row", { gap: 6, alignItems: "center", padding: [6, 8], radius: 8, background: "surfaceHover" },
                  iconNode(GLYPHS.alert, 12, "danger"),
                  h("Text", { fontSize: 12, textColor: "danger", wrap: true, shrink: 1 }, lastError))
              : null,
            heroBlock(state),
            divider(),
            hourlyBlock(state),
            divider(),
            dailyBlock(state),
          ];
        } else if (lastError) {
          sections = [errorView(lastError, refresh)];
        } else {
          sections = skeletonView();
        }
        return {
          tray,
          panel: h(
            "Column",
            {
              preferredWidth: PANEL_W,
              preferredHeight: 512,
              fill: true,
              minHeight: 0,
              safeArea: true,
              padding: [10, PAD, PAD, PAD],
              gap: 14,
              shortcuts: { "CmdOrCtrl+R": refresh },
            },
            headerRow(state && state.place, String(((env && env.inputs) || {}).location ?? "").trim(), refresh),
            h("Scroll", { grow: 1, minHeight: 0, fillWidth: true }, h("Column", { fillWidth: true, gap: 14 }, ...sections))
          ),
        };
      },
    };
  ]]></script>
</bar>
`
