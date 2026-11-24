package bar

import (
	"sync"

	"github.com/MoeForge/moejs"
)

var (
	barUIKitOnce   sync.Once
	barUIKitScript *moejs.Script
	barUIKitErr    error
)

func installBarUIKit(rt *moejs.Runtime) error {
	barUIKitOnce.Do(func() {
		barUIKitScript, barUIKitErr = moejs.CompileScript("bar-ui-kit.js", barUIKitJS)
	})
	if barUIKitErr != nil {
		return barUIKitErr
	}
	_, err := rt.RunScript(barUIKitScript)
	return err
}

const barUIKitJS = `(function () {
  function flatten(arr, out) {
    for (var i = 0; i < arr.length; i++) {
      var item = arr[i];
      if (Array.isArray(item)) {
        flatten(item, out);
      } else if (item !== null && item !== undefined && item !== false) {
        out.push(item);
      }
    }
    return out;
  }

  function h(type, props) {
    var children = [];
    for (var i = 2; i < arguments.length; i++) {
      var arg = arguments[i];
      if (Array.isArray(arg)) {
        flatten(arg, children);
      } else if (arg !== null && arg !== undefined && arg !== false) {
        children.push(arg);
      }
    }
    return {
      type: type,
      props: props || {},
      children: children
    };
  }

  function card(props) {
    var base = {
      background: "surfaceCard",
      borderWidth: 1,
      borderColor: "border",
      radius: 12,
      padding: 12,
      gap: 8,
      fillWidth: true
    };
    var merged = Object.assign(base, props || {});
    var args = ["Column", merged];
    for (var i = 1; i < arguments.length; i++) {
      args.push(arguments[i]);
    }
    return h.apply(null, args);
  }

  function section(title) {
    var header = h("Text", {
      fontSize: 11,
      fontWeight: 600,
      letterSpacing: 0.4,
      textColor: "textMuted"
    }, String(title || ""));
    var args = ["Column", { fillWidth: true, gap: 6 }, header];
    for (var i = 1; i < arguments.length; i++) {
      args.push(arguments[i]);
    }
    return h.apply(null, args);
  }

  function badge(text, tone, props) {
    return h("Badge", Object.assign({ tone: tone || "accent" }, props || {}), String(text == null ? "" : text));
  }

  function dot(color, pulse, size) {
    return h("StatusDot", {
      color: color || "success",
      pulse: !!pulse,
      size: size || 7
    });
  }

  function iconButton(opts) {
    opts = opts || {};
    var boxSize = opts.boxSize || 26;
    var icSize = opts.size || 14;
    var iconProps = {
      fontSize: icSize,
      width: icSize,
      height: icSize,
      shrink: 0,
      textColor: opts.color || "textMuted"
    };
    if (opts.svg) {
      iconProps.svg = String(opts.svg);
    } else if (opts.icon) {
      iconProps.name = String(opts.icon);
    }
    if (opts.spin) {
      iconProps.keyframes = {
        duration: 900,
        repeat: -1,
        ease: "linear",
        frames: [{ at: 0, rotate: 0 }, { at: 1, rotate: 360 }]
      };
    }
    var boxProps = {
      width: boxSize,
      height: boxSize,
      radius: 7,
      alignItems: "center",
      justify: "center",
      hover: { background: "surfaceHover" },
      pressed: { background: "surfacePressed" }
    };
    if (opts.tooltip) {
      boxProps.tooltip = String(opts.tooltip);
    }
    if (opts.onClick) {
      boxProps.onClick = opts.onClick;
    }
    return h("Column", boxProps, h("Icon", iconProps));
  }

  function topBar(opts) {
    opts = opts || {};
    var leftIcon = null;
    if (opts.svg || opts.icon) {
      var icProps = {
        fontSize: 13,
        width: 13,
        height: 13,
        shrink: 0,
        textColor: opts.iconColor || "textMuted"
      };
      if (opts.svg) {
        icProps.svg = String(opts.svg);
      } else {
        icProps.name = String(opts.icon);
      }
      leftIcon = h("Icon", icProps);
    }
    var rightItems = [];
    if (opts.badge) {
      rightItems.push(badge(opts.badge, opts.badgeTone || "accent"));
    }
    if (Array.isArray(opts.actions)) {
      for (var i = 0; i < opts.actions.length; i++) {
        if (opts.actions[i]) {
          rightItems.push(opts.actions[i]);
        }
      }
    }
    if (opts.externalURL) {
      var extUrl = String(opts.externalURL);
      rightItems.push(iconButton({
        icon: "external-link",
        tooltip: opts.externalTip || extUrl,
        onClick: function () { $metrix.openURL(extUrl); }
      }));
    }
    if (opts.onRefresh) {
      rightItems.push(iconButton({
        icon: "refresh-cw",
        tooltip: opts.refreshTip || "Refresh",
        onClick: opts.onRefresh,
        spin: !!opts.loading
      }));
    }
    return h("Row", { fillWidth: true, height: 26, alignItems: "center", gap: 6 },
      leftIcon,
      h("Text", { fontSize: 13, fontWeight: 600, singleLine: true }, String(opts.title || "")),
      opts.subtitle ? h("Text", { fontSize: 12, textColor: "textMuted", singleLine: true, shrink: 1 }, String(opts.subtitle)) : null,
      h("Spacer"),
      rightItems.length ? h("Row", { alignItems: "center", gap: 4, shrink: 0 }, rightItems) : null
    );
  }

  function hero(opts) {
    opts = opts || {};
    var fs = opts.fontSize || 48;
    var fw = opts.fontWeight || 300;
    var ls = opts.letterSpacing != null ? opts.letterSpacing : -2;
    var valueRow = h("Row", { alignItems: "end", gap: 4 },
      h("Text", {
        fontSize: fs,
        fontWeight: fw,
        letterSpacing: ls,
        fontFeatures: "tnum",
        contentTransition: { numeric: true, down: !!opts.down },
        textColor: opts.tone || "text",
        singleLine: true
      }, String(opts.value == null ? "--" : opts.value)),
      opts.unit ? h("Text", {
        fontSize: opts.unitSize || 16,
        fontWeight: 400,
        textColor: "textMuted",
        padding: [0, 0, 7, 0],
        singleLine: true
      }, String(opts.unit)) : null
    );
    var subRow = (opts.label || opts.secondary) ? h("Row", { gap: 8, alignItems: "center" },
      opts.label ? h("Text", { fontSize: 13, fontWeight: 500, singleLine: true }, String(opts.label)) : null,
      opts.secondary ? h("Text", { fontSize: 12, textColor: "textMuted", fontFeatures: "tnum", singleLine: true }, String(opts.secondary)) : null
    ) : null;

    var rightNode = opts.right || null;
    if (!rightNode && (opts.svg || opts.icon)) {
      var icSize = opts.iconSize || 48;
      var icProps = {
        fontSize: icSize,
        width: icSize,
        height: icSize,
        shrink: 0,
        textColor: opts.iconColor || "text"
      };
      if (opts.svg) {
        icProps.svg = String(opts.svg);
      } else {
        icProps.name = String(opts.icon);
      }
      rightNode = h("Icon", icProps);
    }

    var metricStrip = null;
    if (Array.isArray(opts.metrics) && opts.metrics.length > 0) {
      var cells = [];
      for (var i = 0; i < opts.metrics.length; i++) {
        var m = opts.metrics[i] || {};
        var mIc = null;
        if (m.svg || m.icon) {
          var micProps = {
            fontSize: 12,
            width: 12,
            height: 12,
            shrink: 0,
            textColor: m.iconColor || "textMuted"
          };
          if (m.svg) {
            micProps.svg = String(m.svg);
          } else {
            micProps.name = String(m.icon);
          }
          mIc = h("Icon", micProps);
        }
        var itemProps = { gap: 4, alignItems: "center" };
        if (m.tooltip) {
          itemProps.tooltip = String(m.tooltip);
        }
        cells.push(h("Row", itemProps,
          mIc,
          h("Text", {
            fontSize: 12,
            fontWeight: 500,
            fontFeatures: "tnum",
            textColor: m.tone || "text",
            singleLine: true
          }, String(m.value == null ? "--" : m.value))
        ));
      }
      metricStrip = h("Row", { fillWidth: true, gap: 14, wrap: true }, cells);
    }

    return h("Column", { fillWidth: true, gap: 10 },
      h("Row", { fillWidth: true, alignItems: "center", gap: 12 },
        h("Column", { grow: 1, shrink: 1, minWidth: 0, gap: 2 },
          valueRow,
          subRow
        ),
        rightNode
      ),
      metricStrip
    );
  }

  function rangeBar(opts) {
    opts = opts || {};
    var minV = Number(opts.min != null ? opts.min : 0);
    var maxV = Number(opts.max != null ? opts.max : 100);
    var lo = Number(opts.low != null ? opts.low : minV);
    var hi = Number(opts.high != null ? opts.high : lo);
    var span = maxV - minV;
    if (!(span > 0)) {
      span = 1;
    }
    var leftPct = ((lo - minV) / span) * 100;
    if (leftPct < 0) leftPct = 0;
    if (leftPct > 96) leftPct = 96;
    var widthPct = ((hi - lo) / span) * 100;
    if (widthPct < 4) widthPct = 4;
    if (widthPct > 100 - leftPct) widthPct = 100 - leftPct;
    var hgt = opts.height || 4;
    var rad = hgt / 2;
    var innerProps = {
      absolute: true,
      top: 0,
      bottom: 0,
      radius: rad,
      leftPercent: leftPct,
      widthPercent: widthPct
    };
    if (opts.toColor) {
      innerProps.gradient = { from: opts.color || "accent", to: opts.toColor, angle: 90 };
    } else {
      innerProps.background = opts.color || "accent";
    }
    return h("Box", {
      grow: 1,
      height: hgt,
      radius: rad,
      clip: true,
      background: opts.trackColor || "surfaceHover"
    }, h("Box", innerProps));
  }

  function rangeRow(opts) {
    opts = opts || {};
    var iconCell = null;
    if (opts.svg || opts.icon) {
      var icProps = {
        fontSize: 15,
        width: 15,
        height: 15,
        shrink: 0,
        textColor: opts.iconColor || "textMuted"
      };
      if (opts.svg) {
        icProps.svg = String(opts.svg);
      } else {
        icProps.name = String(opts.icon);
      }
      iconCell = h("Row", { width: 20, justify: "center", shrink: 0 }, h("Icon", icProps));
    }
    var rowProps = { fillWidth: true, height: 26, alignItems: "center", gap: 8 };
    if (opts.key != null) {
      rowProps.key = opts.key;
    }
    var valW = opts.valWidth != null ? opts.valWidth : 34;
    return h("Row", rowProps,
      h("Text", {
        width: opts.labelWidth || 52,
        fontSize: 13,
        fontWeight: opts.active ? 600 : 500,
        singleLine: true
      }, String(opts.label || "")),
      iconCell,
      opts.tag != null && opts.tag !== ""
        ? h("Text", {
            width: opts.tagWidth || 32,
            fontSize: 11,
            fontWeight: 500,
            textColor: opts.tagColor || "accent",
            fontFeatures: "tnum",
            singleLine: true
          }, String(opts.tag))
        : null,
      h("Text", {
        width: valW,
        fontSize: 12.5,
        textColor: "textMuted",
        textAlign: "end",
        fontFeatures: "tnum",
        singleLine: true
      }, String(opts.lowLabel != null ? opts.lowLabel : (opts.low != null ? opts.low : ""))),
      rangeBar({
        min: opts.min,
        max: opts.max,
        low: opts.low,
        high: opts.high,
        color: opts.color || "accent",
        toColor: opts.toColor,
        height: opts.barHeight || 4,
        trackColor: opts.trackColor
      }),
      h("Text", {
        width: valW,
        fontSize: 12.5,
        fontWeight: 600,
        textAlign: "end",
        fontFeatures: "tnum",
        singleLine: true
      }, String(opts.highLabel != null ? opts.highLabel : (opts.high != null ? opts.high : "")))
    );
  }

  function staleError(message) {
    if (!message) {
      return null;
    }
    return h("Row", {
      fillWidth: true,
      gap: 6,
      alignItems: "center",
      padding: [6, 8],
      radius: 8,
      background: "dangerSurface"
    },
      h("Icon", { name: "alert-circle", fontSize: 12, width: 12, height: 12, shrink: 0, textColor: "danger" }),
      h("Text", { fontSize: 12, textColor: "danger", wrap: true, shrink: 1 }, String(message))
    );
  }

  function header(opts) {
    opts = opts || {};
    var leftIcon = null;
    if (opts.icon) {
      leftIcon = h("Box", {
        width: 30,
        height: 30,
        radius: 9,
        background: opts.iconBg || "accentSurface",
        alignItems: "center",
        justify: "center",
        shrink: 0
      }, h("Icon", {
        name: opts.icon,
        fontSize: 15,
        textColor: opts.iconColor || "accent"
      }));
    }
    var titleCol = h("Column", { grow: 1, shrink: 1, minWidth: 0, gap: 1 },
      h("Text", { fontSize: 14, fontWeight: 650, singleLine: true }, String(opts.title || "")),
      opts.subtitle ? h("Text", { fontSize: 11, textColor: "textMuted", singleLine: true }, String(opts.subtitle)) : null
    );
    var rightItems = [];
    if (opts.badge) {
      rightItems.push(badge(opts.badge, opts.badgeTone || "accent"));
    }
    if (Array.isArray(opts.actions)) {
      for (var i = 0; i < opts.actions.length; i++) {
        rightItems.push(opts.actions[i]);
      }
    }
    if (opts.onRefresh) {
      rightItems.push(iconButton({
        icon: "refresh-cw",
        tooltip: "Refresh",
        onClick: opts.onRefresh,
        spin: !!opts.loading
      }));
    }
    return h("Row", { fillWidth: true, alignItems: "center", gap: 10 },
      leftIcon,
      titleCol,
      rightItems.length ? h("Row", { alignItems: "center", gap: 6, shrink: 0 }, rightItems) : null
    );
  }

  function metric(opts) {
    opts = opts || {};
    var fs = opts.fontSize || 26;
    var fw = opts.fontWeight || (fs >= 36 ? 300 : 650);
    var txtProps = {
      fontSize: fs,
      fontWeight: fw,
      fontFeatures: "tnum",
      font: opts.mono ? "mono" : undefined,
      textColor: opts.tone || "text",
      contentTransition: { numeric: true, down: !!opts.down },
      singleLine: true
    };
    if (opts.letterSpacing != null) {
      txtProps.letterSpacing = opts.letterSpacing;
    } else if (fs >= 36) {
      txtProps.letterSpacing = -1.5;
    }
    return h("Column", { gap: 2 },
      opts.label ? h("Text", { fontSize: 11, fontWeight: 500, textColor: "textMuted", singleLine: true }, String(opts.label)) : null,
      h("Row", { alignItems: "center", gap: 6 },
        h("Text", txtProps, String(opts.value == null ? "--" : opts.value)),
        opts.unit ? h("Text", { fontSize: 12, fontWeight: 500, textColor: "textMuted" }, String(opts.unit)) : null,
        opts.delta ? badge(opts.delta, opts.deltaTone || "accent") : null
      ),
      opts.sub ? h("Text", { fontSize: 11, textColor: "textMuted", singleLine: true }, String(opts.sub)) : null
    );
  }

  function statGrid(items, cols, opts) {
    items = Array.isArray(items) ? items : [];
    cols = cols && cols > 0 ? cols : 2;
    opts = opts || {};
    var bordered = !!opts.bordered;
    var rows = [];
    for (var i = 0; i < items.length; i += cols) {
      var cells = [];
      for (var c = 0; c < cols; c++) {
        var it = items[i + c];
        if (!it) {
          cells.push(h("Box", { grow: 1, shrink: 1, basis: 0, minWidth: 0 }));
          continue;
        }
        var head = h("Row", { fillWidth: true, alignItems: "center", gap: 5 },
          it.dot ? dot(it.dot, !!it.pulse, 6) : null,
          it.icon ? h("Icon", { name: it.icon, fontSize: 12, textColor: it.tone || "textMuted" }) : null,
          h("Text", { fontSize: 11, fontWeight: 500, textColor: "textMuted", singleLine: true, grow: 1, shrink: 1 }, String(it.label || ""))
        );
        var valRow = h("Row", { alignItems: "center", gap: 4 },
          h("Text", {
            fontSize: 16,
            fontWeight: 650,
            fontFeatures: "tnum",
            font: it.mono ? "mono" : undefined,
            textColor: it.tone || "text",
            singleLine: true
          }, String(it.value == null ? "--" : it.value)),
          it.unit ? h("Text", { fontSize: 11, textColor: "textMuted" }, String(it.unit)) : null
        );
        var cellProps = {
          grow: 1,
          shrink: 1,
          basis: 0,
          minWidth: 0,
          background: bordered ? "surfaceCard" : "surfaceSubtle",
          radius: bordered ? 11 : 10,
          padding: [9, 10],
          gap: 3
        };
        if (bordered) {
          cellProps.borderWidth = 1;
          cellProps.borderColor = "border";
        }
        cells.push(h("Column", cellProps,
          head,
          valRow,
          it.sub ? h("Text", { fontSize: 10, textColor: "textMuted", singleLine: true }, String(it.sub)) : null
        ));
      }
      rows.push(h("Row", { fillWidth: true, gap: 8, alignItems: "stretch" }, cells));
    }
    return h("Column", { fillWidth: true, gap: 8 }, rows);
  }

  function kv(label, value, opts) {
    opts = opts || {};
    return h("Row", { fillWidth: true, alignItems: "center", gap: 8 },
      h("Text", { fontSize: 12, textColor: "textMuted", grow: 1, shrink: 1, singleLine: true }, String(label || "")),
      opts.badge ? badge(opts.badge, opts.badgeTone || "accent") : null,
      h("Text", {
        fontSize: 12,
        fontWeight: 600,
        fontFeatures: "tnum",
        font: opts.mono ? "mono" : undefined,
        textColor: opts.valueColor || "text",
        singleLine: true
      }, String(value == null ? "--" : value))
    );
  }

  function empty(opts) {
    opts = opts || {};
    return h("Column", { grow: 1, fillWidth: true, alignItems: "center", justify: "center", gap: 8, padding: [20, 12] },
      opts.icon ? h("Box", {
        width: 36,
        height: 36,
        radius: 10,
        background: "surfaceSubtle",
        alignItems: "center",
        justify: "center"
      }, h("Icon", { name: opts.icon, fontSize: 18, textColor: "textMuted" })) : null,
      h("Text", { fontSize: 13, fontWeight: 600, textAlign: "center" }, String(opts.title || "Nothing yet")),
      opts.subtitle ? h("Text", { fontSize: 11, textColor: "textMuted", textAlign: "center", wrap: true }, String(opts.subtitle)) : null,
      (opts.actionLabel && opts.onAction) ? h("Button", { variant: "primary", onClick: opts.onAction }, String(opts.actionLabel)) : null
    );
  }

  function skeleton(spec) {
    if (spec === "hero" || spec === "weather") {
      var skel = function (w, hgt, extra) {
        return h("Box", Object.assign({ width: w, height: hgt, radius: 4, background: "surfaceHover" }, extra || {}));
      };
      var pills = [];
      for (var p = 0; p < 4; p++) {
        pills.push(skel(44, 14));
      }
      var cols = [];
      for (var cIdx = 0; cIdx < 7; cIdx++) {
        cols.push(skel(24, 50, { radius: 6 }));
      }
      var bars = [];
      for (var r = 0; r < 5; r++) {
        bars.push(skel(268, 16));
      }
      return h("Column", { fillWidth: true, gap: 14 },
        h("Row", { fillWidth: true, alignItems: "center", gap: 12 },
          h("Column", { grow: 1, gap: 10 }, skel(96, 50, { radius: 6 }), skel(140, 14)),
          skel(50, 50, { radius: 25 })
        ),
        h("Row", { gap: 14 }, pills),
        h("Row", { fillWidth: true, justify: "spaceBetween", padding: [0, 6] }, cols),
        h("Column", { fillWidth: true, gap: 10 }, bars)
      );
    }
    var n = spec && spec > 0 ? spec : 3;
    var items = [];
    for (var i = 0; i < n; i++) {
      items.push(h("Box", {
        height: i === 0 ? 18 : 12,
        widthPercent: i === 0 ? 58 : (100 - i * 14),
        radius: 6,
        background: "surfaceHover"
      }));
    }
    return h("Column", { fillWidth: true, gap: 8 }, items);
  }

  function footer(leftText) {
    var actions = [];
    for (var i = 1; i < arguments.length; i++) {
      if (arguments[i]) {
        actions.push(arguments[i]);
      }
    }
    return h("Row", { fillWidth: true, alignItems: "center", gap: 8 },
      h("Text", { fontSize: 11, fontFeatures: "tnum", textColor: "textMuted", grow: 1, shrink: 1, singleLine: true }, String(leftText || "")),
      actions.length ? h("Row", { alignItems: "center", gap: 6, shrink: 0 }, actions) : null
    );
  }

  globalThis.h = h;
  globalThis.$ui = {
    h: h,
    card: card,
    section: section,
    header: header,
    topBar: topBar,
    hero: hero,
    rangeBar: rangeBar,
    rangeRow: rangeRow,
    staleError: staleError,
    iconButton: iconButton,
    metric: metric,
    statGrid: statGrid,
    kv: kv,
    badge: badge,
    dot: dot,
    empty: empty,
    skeleton: skeleton,
    footer: footer,
    divider: function (props) { return h("Divider", props || {}); },
    spacer: function (props) { return h("Spacer", props || {}); }
  };
})();
`
