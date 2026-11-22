package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
)

// bentoViewState holds per-instance interactive UI state for a Bento dashboard.
type bentoViewState struct {
	masked        bool
	selectedPill  int // -1 none, -2 hero, >=0 pill index
	selectedProbe int // -1 none, >=0 probe index
	showAllProbes bool
	copiedNote    string
}

func (a *app) bentoViewStateOf(key string) *bentoViewState {
	if a == nil {
		return &bentoViewState{selectedPill: -1, selectedProbe: -1}
	}
	if a.bentoViews == nil {
		a.bentoViews = map[string]*bentoViewState{}
	}
	st, ok := a.bentoViews[key]
	if !ok {
		st = &bentoViewState{selectedPill: -1, selectedProbe: -1}
		a.bentoViews[key] = st
	}
	return st
}

func isBento(ch *engine.Chart) bool {
	return ch != nil && ch.Type == "bento" && ch.Bento != nil
}

func isBentoHero(ch *engine.Chart) bool {
	return isBento(ch) && ch.Bento.Hero != nil
}

type bentoColors struct {
	dark       bool
	subBg      ui.Color
	subHover   ui.Color
	subBorder  ui.Color
	text       ui.Color
	soft       ui.Color
	muted      ui.Color
	good       ui.Color
	warn       ui.Color
	slow       ui.Color
	failText   ui.Color
	failDot    ui.Color
	pendingDot ui.Color
}

func bentoPalette(c *ui.Context, themeOverride string) bentoColors {
	p := pal(c)
	dark := p.dark
	switch strings.ToLower(strings.TrimSpace(themeOverride)) {
	case "dark":
		dark = true
	case "light":
		dark = false
	}
	if isMac() && themeOverride == "" {
		return bentoColors{
			dark:       false,
			subBg:      ui.Hex("#FFFFFF"),
			subHover:   ui.Hex("#E6E6E6"),
			subBorder:  p.border,
			text:       p.text,
			soft:       p.muted,
			muted:      p.faint,
			good:       ui.Hex("#1B8A2D"),
			warn:       ui.Hex("#2EA043"),
			slow:       ui.Hex("#D97706"),
			failText:   ui.Hex("#CF222E"),
			failDot:    ui.Hex("#999999"),
			pendingDot: ui.Hex("#CCCCCC"),
		}
	}
	if dark {
		return bentoColors{
			dark:       true,
			subBg:      ui.Hex("#11151C"),
			subHover:   ui.Hex("#21262D"),
			subBorder:  ui.Hex("#2D333D"),
			text:       ui.Hex("#E6EDF3"),
			soft:       ui.Hex("#C9D1D9"),
			muted:      ui.Hex("#8B949E"),
			good:       ui.Hex("#22C55E"),
			warn:       ui.Hex("#6FCF7C"),
			slow:       ui.Hex("#F0C040"),
			failText:   ui.Hex("#E17055"),
			failDot:    ui.Hex("#6E7681"),
			pendingDot: ui.Hex("#30363D"),
		}
	}
	return bentoColors{
		dark:       false,
		subBg:      ui.Hex("#F8F9FB"),
		subHover:   ui.Hex("#EEF1F5"),
		subBorder:  ui.Hex("#E1E4E8"),
		text:       ui.Hex("#151922"),
		soft:       ui.Hex("#57606A"),
		muted:      ui.Hex("#8B949E"),
		good:       ui.Hex("#1B8A2D"),
		warn:       ui.Hex("#2EA043"),
		slow:       ui.Hex("#D97706"),
		failText:   ui.Hex("#E17055"),
		failDot:    ui.Hex("#B2BEC3"),
		pendingDot: ui.Hex("#DFE6E9"),
	}
}

// maskIP obfuscates the lower segments of an IPv4 or IPv6 address (keeping an
// optional country prefix such as "JP 54.65.230.180" -> "JP 54.65.*.*").
func maskIP(s string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return ""
	}
	if idx := strings.IndexByte(t, ' '); idx > 0 && idx <= 4 {
		prefix := t[:idx]
		rest := strings.TrimSpace(t[idx+1:])
		if masked := maskRawIP(rest); masked != rest {
			return prefix + " " + masked
		}
	}
	return maskRawIP(t)
}

func maskRawIP(t string) string {
	parts := strings.Split(t, ".")
	if len(parts) == 4 && isAllDigits(parts) {
		return parts[0] + "." + parts[1] + ".*.*"
	}
	if i := strings.LastIndex(t, "…"); i >= 0 {
		return t[:i+len("…")] + "*"
	}
	if strings.Contains(t, ":") {
		hextets := strings.Split(t, ":")
		if len(hextets) >= 2 {
			return hextets[0] + ":" + hextets[1] + ":*"
		}
	}
	return t
}

func isAllDigits(parts []string) bool {
	for _, p := range parts {
		if p == "" || len(p) > 3 {
			return false
		}
		for i := 0; i < len(p); i++ {
			if p[i] < '0' || p[i] > '9' {
				return false
			}
		}
	}
	return true
}

func bentoWideHeight(b *engine.BentoSpec, contentW, scale float32) float32 {
	if b == nil {
		return 208 * scale
	}
	if contentW <= 0 {
		contentW = 880
	}
	innerH := float32(0)
	if b.Probes != nil && len(b.Probes.Items) > 0 {
		probeW := contentW*0.68 - 24
		cols := 3
		if probeW < 420 {
			cols = 2
		}
		rows := (len(b.Probes.Items) + cols - 1) / cols
		probeH := float32(22) + float32(rows)*50 + float32(max(rows-1, 0))*8
		innerH = max(probeH, 128)
	} else if b.Hero != nil {
		innerH = 104
	}
	if b.Pills != nil && len(b.Pills.Items) > 0 {
		cols := 4
		if contentW < 760 {
			cols = 3
		}
		if contentW < 520 {
			cols = 2
		}
		rows := (len(b.Pills.Items) + cols - 1) / cols
		innerH += 14 + 24 + float32(rows)*38 + float32(max(rows-1, 0))*8
	}
	if len(b.Cells) > 0 {
		rows := (len(b.Cells) + 2) / 3
		innerH += 14 + float32(rows)*66
	}
	// Card padding (32) + top/bottom action buffer (16).
	return max(260*scale, (innerH+46)*scale)
}

func (a *app) bentoWidgetView(c *ui.Context, key string, b *engine.BentoSpec, width float32, size int, full bool) ui.Element {
	p := pal(c)
	if b == nil {
		return ui.Text(c, tr("暂无看板数据")).FontSize(fsSmall).TextColor(p.muted)
	}
	bp := bentoPalette(c, b.Theme)
	st := a.bentoViewStateOf(key)

	if !full && size == 0 {
		return bentoSmallView(c, b, bp, st.masked)
	}
	if !full && (size == 1 || (width > 0 && width < 520)) {
		return bentoMediumView(c, b, bp, st.masked)
	}
	return a.bentoFullOrWideView(c, key, b, bp, st, width, full)
}

func bentoSmallView(c *ui.Context, b *engine.BentoSpec, bp bentoColors, masked bool) ui.Element {
	return ui.Column(c).Gap(6).Children(func() {
		if b.Hero != nil {
			ip := b.Hero.Value
			if masked {
				ip = firstNonEmpty(b.Hero.Masked, maskIP(ip))
			}
			ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
				if b.Hero.Flag != "" {
					flagIcon(c, b.Hero.Flag, 18, 12)
				}
				ui.Text(c, ip).FontSize(fsBody).Bold().TextColor(bp.text).NoWrap()
			})
			if b.Hero.Location != "" {
				ui.Text(c, b.Hero.Location).FontSize(fsTiny).TextColor(bp.muted).NoWrap()
			}
		}
		if b.Probes != nil && len(b.Probes.Items) > 0 {
			okN := 0
			for _, pr := range b.Probes.Items {
				if pr.Latency >= 0 {
					okN++
				}
			}
			ui.Text(c, fmt.Sprintf(tr("连通 %d/%d"), okN, len(b.Probes.Items))).FontSize(fsTiny).TextColor(bp.good)
		}
	})
}

func bentoMediumView(c *ui.Context, b *engine.BentoSpec, bp bentoColors, masked bool) ui.Element {
	return ui.Column(c).Gap(6).Children(func() {
		if b.Hero != nil {
			ip := b.Hero.Value
			if masked {
				ip = firstNonEmpty(b.Hero.Masked, maskIP(ip))
			}
			ui.Row(c).AlignItems(ui.Center).Gap(7).Children(func() {
				if b.Hero.Flag != "" {
					flagIcon(c, b.Hero.Flag, 18, 12)
				}
				ui.Row(c).Grow(1).MinWidth(0).ClipX().Children(func() {
					ui.Text(c, ip).FontSize(16).Bold().TextColor(bp.text).NoWrap()
				})
				if b.Hero.Meta != "" {
					chip(c, b.Hero.Meta, bp.muted, bp.subBorder.Alpha(0.5))
				}
			})
			if b.Hero.Location != "" {
				ui.Row(c).MinWidth(0).ClipX().Children(func() {
					ui.Text(c, b.Hero.Location).FontSize(fsTiny).TextColor(bp.soft).NoWrap()
				})
			}
		}
		if b.Probes != nil && len(b.Probes.Items) > 0 {
			items := b.Probes.Items
			if len(items) > 6 {
				items = items[:6]
			}
			for r := 0; r < len(items); r += 2 {
				ui.Row(c).Gap(6).Children(func() {
					for col := 0; col < 2; col++ {
						idx := r + col
						if idx >= len(items) {
							ui.Box(c).Grow(1).MinWidth(0)
							continue
						}
						it := items[idx]
						latText, latCol := probeLatencyDisplay(it, bp)
						ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(5).
							Padding(4, 6).Radius(6).Background(bp.subBg).BorderWidth(1).BorderColor(bp.subBorder).
							Children(func() {
								probeBrandIcon(c, it.Icon, it.Name, pal(c).accent, bp.dark, 13)
								ui.Row(c).Grow(1).MinWidth(0).ClipX().Children(func() {
									ui.Text(c, it.Name).FontSize(11).TextColor(bp.text).NoWrap()
								})
								ui.Text(c, latText).FontSize(11).FontWeight(650).TextColor(latCol).NoWrap()
							})
					}
				})
			}
		}
		if b.Pills != nil && len(b.Pills.Items) > 0 {
			items := b.Pills.Items
			maxPills := min(len(items), 2)
			ui.Row(c).Gap(6).Children(func() {
				for i := 0; i < maxPills; i++ {
					pl := items[i]
					val := pl.Value
					if masked {
						val = firstNonEmpty(pl.Masked, maskIP(val))
					}
					ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(5).
						Padding(4, 7).Radius(6).Background(bp.subBg).BorderWidth(1).BorderColor(bp.subBorder).
						Children(func() {
							if pl.Flag != "" {
								flagIcon(c, pl.Flag, 14, 10)
							}
							ui.Row(c).Grow(1).MinWidth(0).ClipX().Children(func() {
								ui.Text(c, val).FontSize(11).TextColor(bp.soft).NoWrap()
							})
						})
				}
				if len(items) > maxPills {
					ui.Row(c).AlignItems(ui.Center).Padding(4, 6).Radius(6).
						Background(bp.subBg).BorderWidth(1).BorderColor(bp.subBorder).Children(func() {
						ui.Text(c, fmt.Sprintf("+%d", len(items)-maxPills)).FontSize(11).TextColor(bp.muted).NoWrap()
					})
				}
			})
		}
	})
}

func (a *app) bentoFullOrWideView(c *ui.Context, key string, b *engine.BentoSpec, bp bentoColors, st *bentoViewState, width float32, full bool) ui.Element {
	p := pal(c)
	if width <= 0 {
		width = 900
	}
	return ui.Column(c).Gap(14).Children(func() {
		// Full view top toolbar: copy feedback + Hide IP toggle.
		if full && (b.Maskable || st.copiedNote != "") {
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				if st.copiedNote != "" {
					chip(c, st.copiedNote, p.accent, p.accentSoft)
				}
				ui.Spacer(c)
				if b.Maskable {
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						lbl := ui.Text(c, tr("隐藏 IP")).FontSize(fsSmall).TextColor(bp.muted).Cursor(ui.CursorPointer)
						if lbl.Clicked() {
							st.masked = !st.masked
						}
						sw(c.Key("bento-mask:"+key), &st.masked)
					})
				}
			})
		}

		// Top Bento row (Hero on left + Probes grid on right), stacking below 640 DIPs.
		hasHero := b.Hero != nil
		hasProbes := b.Probes != nil && len(b.Probes.Items) > 0
		if hasHero || hasProbes {
			stackTop := width < 640
			heroBlock := func(heroW float32) {
				if !hasHero {
					return
				}
				h := b.Hero
				ipVal := h.Value
				if st.masked {
					ipVal = firstNonEmpty(h.Masked, maskIP(ipVal))
				}
				colEl := ui.Column(c).Gap(6)
				if heroW > 0 {
					colEl = colEl.Width(heroW).Shrink(0)
				}
				colEl.Children(func() {
					if h.Label != "" {
						ui.Text(c, strings.ToUpper(h.Label)).FontSize(11.5).FontWeight(550).TextColor(bp.muted).NoWrap()
					}
					ipRow := ui.Row(c).AlignItems(ui.Center).Gap(10).Padding(2, 4).Radius(6)
					if full && len(h.Details) > 0 {
						if st.selectedPill == -2 {
							ipRow = ipRow.Background(bp.subHover)
						} else if ipRow.Hovered() {
							ipRow = ipRow.Background(bp.subHover.Alpha(0.65))
						}
						ipRow = ipRow.Cursor(ui.CursorPointer)
					}
					builtIP := ipRow.Children(func() {
						if h.Flag != "" {
							flagIcon(c, h.Flag, 22, 15)
						}
						ui.Row(c).Grow(1).MinWidth(0).ClipX().Children(func() {
							ui.Text(c, ipVal).FontSize(22).Bold().TextColor(bp.text).NoWrap()
						})
					})
					if full && len(h.Details) > 0 && builtIP.Clicked() {
						if st.selectedPill == -2 {
							st.selectedPill = -1
						} else {
							st.selectedPill = -2
						}
					}
					if h.Location != "" {
						ui.Row(c).MinWidth(0).ClipX().Children(func() {
							ui.Text(c, h.Location).FontSize(14.5).FontWeight(550).TextColor(bp.text).NoWrap()
						})
					}
					if h.Meta != "" || h.Badge != "" {
						ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(func() {
							if h.Meta != "" {
								ui.Text(c, h.Meta).FontSize(12).TextColor(bp.muted).NoWrap()
							}
							if h.Badge != "" {
								bc := parseColor(h.BadgeColor, bp.good)
								chip(c, h.Badge, bc, bc.Alpha(0.14))
							}
						})
					}
				})
			}

			probesBlock := func(probeAreaW float32) {
				if !hasProbes {
					return
				}
				pg := b.Probes
				cols := pg.Columns
				if cols <= 0 {
					cols = 3
				}
				if probeAreaW < 420 && cols > 2 {
					cols = 2
				}
				if probeAreaW < 250 {
					cols = 1
				}
				ui.Column(c).Gap(8).Grow(1).MinWidth(0).Children(func() {
					if pg.Title != "" || pg.ActionText != "" {
						ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
							if pg.Title != "" {
								ui.Text(c, pg.Title).FontSize(12.5).TextColor(bp.muted).NoWrap()
							}
							ui.Spacer(c)
							if pg.ActionText != "" {
								act := ui.Text(c, pg.ActionText).FontSize(12).TextColor(bp.soft).NoWrap()
								if full {
									act = act.Cursor(ui.CursorPointer)
								}
								if full && act.Clicked() {
									st.showAllProbes = !st.showAllProbes
								}
							}
						})
					}
					items := pg.Items
					for r := 0; r < len(items); r += cols {
						ui.Row(c).Gap(8).Children(func() {
							for col := 0; col < cols; col++ {
								idx := r + col
								if idx >= len(items) {
									ui.Box(c).Grow(1).MinWidth(0)
									continue
								}
								it := items[idx]
								latText, latCol := probeLatencyDisplay(it, bp)
								selected := full && st.selectedProbe == idx
								borderCol := bp.subBorder
								bgCol := bp.subBg
								if selected {
									borderCol = p.accent
									bgCol = bp.subHover
								}
								tile := ui.Row(c).Key(fmt.Sprintf("bento-probe:%s:%d", key, idx)).
									Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(7).
									Padding(7, 10).Radius(8).
									Background(bgCol).BorderWidth(1).BorderColor(borderCol)
								if full {
									if tile.Hovered() && !selected {
										tile = tile.Background(bp.subHover)
									}
									tile = tile.Cursor(ui.CursorPointer)
								}
								built := tile.Children(func() {
									probeBrandIcon(c, it.Icon, it.Name, p.accent, bp.dark, 18)
									ui.Column(c).Gap(3).Grow(1).MinWidth(0).Children(func() {
										ui.Row(c).AlignItems(ui.Center).Gap(5).MinWidth(0).ClipX().Children(func() {
											ui.Text(c, it.Name).FontSize(12.5).FontWeight(550).TextColor(bp.text).NoWrap()
											if it.Flag != "" {
												flagIcon(c, it.Flag, 14, 10)
											}
										})
										latencyDotsView(c, it.Dots, bp, -1)
									})
									ui.Text(c, latText).FontSize(12.5).FontWeight(650).TextColor(latCol).NoWrap().Shrink(0)
								})
								if full && built.Clicked() {
									if st.selectedProbe == idx {
										st.selectedProbe = -1
									} else {
										st.selectedProbe = idx
									}
								}
							}
						})
					}
				})
			}

			if stackTop {
				ui.Column(c).Gap(14).Children(func() {
					heroBlock(0)
					probesBlock(width)
				})
			} else {
				heroW := clampF(width*0.28, 210, 268)
				ui.Row(c).AlignItems(ui.Start).Gap(22).Children(func() {
					heroBlock(heroW)
					probesBlock(width - heroW - 22)
				})
			}
		}

		// Expanded Probe Latency Breakdown (in Full View when clicking "More ›" or a probe sub-card).
		if full && hasProbes && (st.showAllProbes || (st.selectedProbe >= 0 && st.selectedProbe < len(b.Probes.Items))) {
			bentoProbeBreakdownPanel(c, b.Probes.Items, st, bp, p)
		}

		// Bottom Exit IP Pills section.
		if b.Pills != nil && len(b.Pills.Items) > 0 {
			pg := b.Pills
			cols := pg.Columns
			if cols <= 0 {
				cols = 4
			}
			if width < 760 && cols > 3 {
				cols = 3
			}
			if width < 520 && cols > 2 {
				cols = 2
			}
			ui.Column(c).Gap(8).Children(func() {
				if pg.Title != "" {
					ui.Text(c, pg.Title).FontSize(13).TextColor(bp.soft)
				}
				items := pg.Items
				for r := 0; r < len(items); r += cols {
					ui.Row(c).Gap(8).Children(func() {
						for col := 0; col < cols; col++ {
							idx := r + col
							if idx >= len(items) {
								ui.Box(c).Grow(1).MinWidth(0)
								continue
							}
							pl := items[idx]
							val := pl.Value
							if st.masked {
								val = firstNonEmpty(pl.Masked, maskIP(val))
							}
							selected := full && st.selectedPill == idx
							borderCol := bp.subBorder
							bgCol := bp.subBg
							if selected {
								borderCol = p.accent
								bgCol = bp.subHover
							}
							pillEl := ui.Row(c).Key(fmt.Sprintf("bento-pill:%s:%d", key, idx)).
								Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(7).
								Padding(8, 10).Radius(7).
								Background(bgCol).BorderWidth(1).BorderColor(borderCol)
							if full {
								if pillEl.Hovered() && !selected {
									pillEl = pillEl.Background(bp.subHover)
								}
								pillEl = pillEl.Cursor(ui.CursorPointer)
							}
							built := pillEl.Children(func() {
								if pl.Flag != "" {
									flagIcon(c, pl.Flag, 20, 14)
								}
								ui.Row(c).Grow(1).MinWidth(0).ClipX().Children(func() {
									ipTxt := ui.Text(c, val).FontSize(13.5).FontWeight(550).TextColor(bp.text).NoWrap()
									if pl.Mono && !isMac() {
										ipTxt.Font("monospace")
									}
								})
								if pl.Badge != "" && width >= 660 {
									ui.Text(c, pl.Badge).FontSize(11).TextColor(bp.muted).NoWrap().Shrink(0)
								}
							})
							if full && built.Clicked() {
								if st.selectedPill == idx {
									st.selectedPill = -1
								} else {
									st.selectedPill = idx
								}
							}
						}
					})
				}
			})
		}

		// Selected IP Quality Inspector (when Hero or an Exit Pill is clicked in Full View).
		if full && (st.selectedPill == -2 || (b.Pills != nil && st.selectedPill >= 0 && st.selectedPill < len(b.Pills.Items))) {
			a.bentoIPInspector(c, key, b, st, bp, p)
		}

		// Optional generic Bento sub-cells (for user/visual custom Bento boards).
		if len(b.Cells) > 0 {
			bentoCellsGrid(c, b.Cells, width, bp, p)
		}

		// Full View Split-Routing Detail Table.
		if full && b.DetailTable != nil && len(b.DetailTable.Columns) > 0 {
			ui.Box(c).Height(1).Shrink(0).Background(p.hairline)
			ui.Column(c).Gap(6).Children(func() {
				if b.DetailTitle != "" {
					ui.Text(c, b.DetailTitle).FontSize(fsHeading).FontWeight(650).TextColor(bp.text)
				}
				if b.DetailDesc != "" {
					ui.Text(c, b.DetailDesc).FontSize(fsSmall).TextColor(bp.muted)
				}
				tb := b.DetailTable
				if st.masked {
					tb = maskDetailTableIPs(b.DetailTable)
				}
				a.tableWidgetView(c, "bento-detail:"+key, tb, true, 0)
			})
		}
	})
}

func bentoProbeBreakdownPanel(c *ui.Context, items []engine.BentoProbe, st *bentoViewState, bp bentoColors, p palette) {
	showList := items
	if !st.showAllProbes && st.selectedProbe >= 0 && st.selectedProbe < len(items) {
		showList = []engine.BentoProbe{items[st.selectedProbe]}
	}
	ui.Column(c).Gap(8).Padding(12, 14).Radius(9).Background(bp.subBg).BorderWidth(1).BorderColor(bp.subBorder).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, tr("连通性探针明细")).FontSize(fsSmall).FontWeight(650).TextColor(bp.text)
			ui.Spacer(c)
			if btn(c, tr("收起")).Clicked() {
				st.showAllProbes = false
				st.selectedProbe = -1
			}
		})
		for i, it := range showList {
			if i > 0 {
				ui.Box(c).Height(1).Shrink(0).Background(bp.subBorder.Alpha(0.6))
			}
			latText, latCol := probeLatencyDisplay(it, bp)
			ui.Column(c).Gap(4).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(func() {
					probeBrandIcon(c, it.Icon, it.Name, p.accent, bp.dark, 15)
					ui.Text(c, it.Name).FontSize(fsSmall).FontWeight(600).TextColor(bp.text)
					if it.Flag != "" {
						flagIcon(c, it.Flag, 16, 11)
					}
					ui.Text(c, fmt.Sprintf(tr("中位 %s"), latText)).FontSize(fsTiny).FontWeight(600).TextColor(latCol)
					if it.MinMs >= 0 && it.MaxMs >= 0 {
						ui.Text(c, fmt.Sprintf("Min %dms · Max %dms", it.MinMs, it.MaxMs)).FontSize(fsTiny).TextColor(bp.muted)
					}
					if it.Total > 0 {
						ui.Text(c, fmt.Sprintf(tr("成功 %d/%d"), it.Success, it.Total)).FontSize(fsTiny).TextColor(bp.muted)
					}
				})
				if len(it.Dots) > 0 {
					var parts []string
					for rIdx, ms := range it.Dots {
						if ms < 0 {
							parts = append(parts, fmt.Sprintf("#%d:%s", rIdx+1, tr("超时")))
						} else {
							parts = append(parts, fmt.Sprintf("#%d:%dms", rIdx+1, ms))
						}
					}
					ui.Text(c, strings.Join(parts, "  ")).FontSize(fsTiny).TextColor(bp.muted)
				}
			})
		}
	})
}

func (a *app) bentoIPInspector(c *ui.Context, key string, b *engine.BentoSpec, st *bentoViewState, bp bentoColors, p palette) {
	var flag, realVal, maskedVal, copyVal, urlStr, sub string
	var details []engine.KVItem
	if st.selectedPill == -2 && b.Hero != nil {
		flag = b.Hero.Flag
		realVal = b.Hero.Value
		maskedVal = firstNonEmpty(b.Hero.Masked, maskIP(realVal))
		copyVal = firstNonEmpty(b.Hero.Copy, realVal)
		sub = b.Hero.Location
		details = b.Hero.Details
	} else if b.Pills != nil && st.selectedPill >= 0 && st.selectedPill < len(b.Pills.Items) {
		pl := b.Pills.Items[st.selectedPill]
		flag = pl.Flag
		realVal = pl.Value
		maskedVal = firstNonEmpty(pl.Masked, maskIP(realVal))
		copyVal = firstNonEmpty(pl.Copy, realVal)
		urlStr = pl.URL
		sub = pl.Sub
		details = pl.Details
	}
	if realVal == "" {
		return
	}
	shownIP := realVal
	if st.masked {
		shownIP = maskedVal
	}

	ui.Column(c).Gap(10).Padding(12, 14).Radius(9).Background(bp.subBg).BorderWidth(1).BorderColor(p.accent.Alpha(0.55)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(func() {
			if flag != "" {
				flagIcon(c, flag, 22, 15)
			}
			ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, shownIP).FontSize(fsHeading).Bold().TextColor(bp.text).NoWrap()
				if sub != "" {
					ui.Text(c, sub).FontSize(fsTiny).TextColor(bp.muted)
				}
			})
			if btn(c.Key("bento-copy-ip:"+key), tr("复制 IP")).Clicked() {
				copyText(copyVal)
				st.copiedNote = tr("已复制：") + copyVal
			}
			if strings.HasPrefix(urlStr, "https://") || strings.HasPrefix(urlStr, "http://") {
				if btn(c.Key("bento-open-ip:"+key), tr("打开详情页")).Clicked() {
					_ = mygo.Shell.OpenExternal(urlStr)
				}
			}
			if btn(c.Key("bento-close-ip:"+key), tr("关闭")).Clicked() {
				st.selectedPill = -1
			}
		})

		if len(details) > 0 {
			for r := 0; r < len(details); r += 2 {
				ui.Row(c).Gap(10).Children(func() {
					for col := 0; col < 2; col++ {
						idx := r + col
						if idx >= len(details) {
							ui.Box(c).Grow(1).MinWidth(0)
							continue
						}
						kv := details[idx]
						val := kv.Value
						if st.masked && (kv.Label == "IP 地址" || kv.Label == "IP Address") {
							val = maskIP(val)
						}
						dotCol, hasDot := kvStatusColor(p, kv.Status, kv.Color)
						ui.Column(c).Grow(1).MinWidth(0).Gap(2).Padding(6, 9).Radius(6).
							Background(p.card.Alpha(0.5)).BorderWidth(1).BorderColor(bp.subBorder.Alpha(0.7)).
							Children(func() {
								ui.Row(c).AlignItems(ui.Center).Gap(5).Children(func() {
									if hasDot {
										ui.Box(c).Size(6, 6).Radius(3).Background(dotCol).Shrink(0)
									}
									ui.Text(c, kv.Label).FontSize(fsTiny).TextColor(bp.muted).NoWrap()
									if kv.Badge != "" {
										chip(c, kv.Badge, p.accent, p.accentSoft)
									}
								})
								ui.Text(c, val).FontSize(fsSmall).FontWeight(550).TextColor(bp.text)
							})
					}
				})
			}
		}
	})
}

func bentoCellsGrid(c *ui.Context, cells []engine.BentoCell, width float32, bp bentoColors, p palette) {
	cols := 3
	if width < 640 {
		cols = 2
	}
	for r := 0; r < len(cells); r += cols {
		ui.Row(c).Gap(8).Children(func() {
			for col := 0; col < cols; col++ {
				idx := r + col
				if idx >= len(cells) {
					ui.Box(c).Grow(1).MinWidth(0)
					continue
				}
				cl := cells[idx]
				accentCol := parseColor(cl.Color, p.accent)
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Padding(10, 12).Radius(8).
					Background(bp.subBg).BorderWidth(1).BorderColor(bp.subBorder).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
						if cl.Flag != "" {
							flagIcon(c, cl.Flag, 16, 11)
						}
						if cl.Title != "" {
							ui.Text(c, cl.Title).FontSize(fsTiny).TextColor(bp.muted).Grow(1).MinWidth(0).NoWrap()
						}
						if cl.Badge != "" {
							chip(c, cl.Badge, accentCol, accentCol.Alpha(0.14))
						}
					})
					if cl.Value != "" {
						ui.Text(c, cl.Value).FontSize(fsHeading).Bold().TextColor(bp.text).NoWrap()
					}
					if cl.Sub != "" {
						ui.Text(c, cl.Sub).FontSize(fsTiny).TextColor(bp.soft).NoWrap()
					}
					if len(cl.Dots) > 0 {
						latencyDotsView(c, cl.Dots, bp, -1)
					}
				})
			}
		})
	}
}

func maskDetailTableIPs(tb *engine.TableSpec) *engine.TableSpec {
	if tb == nil {
		return nil
	}
	ipCol := -1
	for i, col := range tb.Columns {
		if strings.EqualFold(strings.TrimSpace(col.Title), "IP") {
			ipCol = i
			break
		}
	}
	if ipCol < 0 {
		return tb
	}
	cp := *tb
	cp.Rows = make([][]string, len(tb.Rows))
	for i, row := range tb.Rows {
		nr := append([]string(nil), row...)
		if ipCol < len(nr) {
			nr[ipCol] = maskIP(nr[ipCol])
		}
		cp.Rows[i] = nr
	}
	return &cp
}

func probeLatencyDisplay(it engine.BentoProbe, bp bentoColors) (string, ui.Color) {
	if it.Latency < 0 {
		txt := it.Text
		if txt == "" {
			txt = tr("超时")
		}
		return txt, bp.failText
	}
	txt := it.Text
	if txt == "" {
		txt = fmt.Sprintf("%dms", min(it.Latency, 999))
	}
	switch it.Status {
	case "err":
		return txt, bp.failText
	case "slow":
		return txt, bp.slow
	case "warn":
		return txt, bp.warn
	case "ok":
		return txt, bp.good
	default:
		if it.Latency < 100 {
			return txt, bp.good
		}
		if it.Latency < 400 {
			return txt, bp.warn
		}
		return txt, bp.slow
	}
}

// ---------------------------------------------------------------------------
// Crisp vector painters: Country Flags, Brand Badges, 12-Dot Latency Strip
// ---------------------------------------------------------------------------

func flagIcon(c *ui.Context, cc string, w, h float32) {
	code := strings.ToLower(strings.TrimSpace(cc))
	ui.Box(c).Size(w, h).Shrink(0).Draw(func(pt *ui.Painter, r ui.Rect) {
		paintCountryFlag(pt, r, code)
	})
}

func paintCountryFlag(pt *ui.Painter, r ui.Rect, cc string) {
	if r.W <= 2 || r.H <= 2 {
		return
	}
	pt.Clip(r, 2, func() {
		switch cc {
		case "cn":
			pt.Fill(r, ui.Hex("#DE2910"), 0)
			// Big star
			paintStar(pt, r.X+r.W*0.21, r.Y+r.H*0.28, r.H*0.16, ui.Hex("#FFDE00"))
			// 4 small stars
			for _, off := range [][2]float32{{0.38, 0.14}, {0.44, 0.25}, {0.44, 0.39}, {0.38, 0.50}} {
				paintStar(pt, r.X+r.W*off[0], r.Y+r.H*off[1], r.H*0.065, ui.Hex("#FFDE00"))
			}
		case "jp":
			pt.Fill(r, ui.Hex("#FFFFFF"), 0)
			rad := min(r.W, r.H) * 0.29
			cx, cy := r.X+r.W/2, r.Y+r.H/2
			pt.Fill(ui.Rect{X: cx - rad, Y: cy - rad, W: rad * 2, H: rad * 2}, ui.Hex("#BC002D"), rad)
		case "us":
			pt.Fill(r, ui.Hex("#FFFFFF"), 0)
			stripeH := r.H / 13
			for i := 0; i < 13; i += 2 {
				pt.Fill(ui.Rect{X: r.X, Y: r.Y + float32(i)*stripeH, W: r.W, H: stripeH + 0.5}, ui.Hex("#B22234"), 0)
			}
			canton := ui.Rect{X: r.X, Y: r.Y, W: r.W * 0.44, H: stripeH * 7}
			pt.Fill(canton, ui.Hex("#3C3B6E"), 0)
			for row := 0; row < 3; row++ {
				for col := 0; col < 4; col++ {
					sx := canton.X + canton.W*(float32(col)+0.5)/4
					sy := canton.Y + canton.H*(float32(row)+0.5)/3
					pt.Fill(ui.Rect{X: sx - 0.8, Y: sy - 0.8, W: 1.6, H: 1.6}, ui.Hex("#FFFFFF"), 0.8)
				}
			}
		case "hk":
			pt.Fill(r, ui.Hex("#DE2910"), 0)
			cx, cy := r.X+r.W/2, r.Y+r.H/2
			pr := min(r.W, r.H) * 0.13
			for i := 0; i < 5; i++ {
				ang := float64(i)*2*math.Pi/5 - math.Pi/2
				px := cx + float32(math.Cos(ang))*pr*1.5
				py := cy + float32(math.Sin(ang))*pr*1.5
				pt.Fill(ui.Rect{X: px - pr, Y: py - pr, W: pr * 2, H: pr * 2}, ui.Hex("#FFFFFF"), pr)
			}
		case "tw":
			pt.Fill(r, ui.Hex("#FE0000"), 0)
			canton := ui.Rect{X: r.X, Y: r.Y, W: r.W * 0.5, H: r.H * 0.52}
			pt.Fill(canton, ui.Hex("#000095"), 0)
			rad := min(canton.W, canton.H) * 0.28
			pt.Fill(ui.Rect{X: canton.X + canton.W/2 - rad, Y: canton.Y + canton.H/2 - rad, W: rad * 2, H: rad * 2}, ui.Hex("#FFFFFF"), rad)
		case "sg":
			pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H * 0.5}, ui.Hex("#EF3340"), 0)
			pt.Fill(ui.Rect{X: r.X, Y: r.Y + r.H*0.5, W: r.W, H: r.H * 0.5}, ui.Hex("#FFFFFF"), 0)
			cr := r.H * 0.16
			pt.Fill(ui.Rect{X: r.X + r.W*0.18 - cr, Y: r.Y + r.H*0.25 - cr, W: cr * 2, H: cr * 2}, ui.Hex("#FFFFFF"), cr)
			pt.Fill(ui.Rect{X: r.X + r.W*0.22 - cr, Y: r.Y + r.H*0.25 - cr, W: cr * 2, H: cr * 2}, ui.Hex("#EF3340"), cr)
		case "kr":
			pt.Fill(r, ui.Hex("#FFFFFF"), 0)
			rad := min(r.W, r.H) * 0.25
			cx, cy := r.X+r.W/2, r.Y+r.H/2
			pt.Fill(ui.Rect{X: cx - rad, Y: cy - rad, W: rad * 2, H: rad}, ui.Hex("#CD2E3A"), rad)
			pt.Fill(ui.Rect{X: cx - rad, Y: cy, W: rad * 2, H: rad}, ui.Hex("#0047A0"), rad)
			bar := ui.Hex("#111827")
			pt.Line(r.X+r.W*0.16, r.Y+r.H*0.26, r.X+r.W*0.26, r.Y+r.H*0.16, 1.4, bar)
			pt.Line(r.X+r.W*0.74, r.Y+r.H*0.16, r.X+r.W*0.84, r.Y+r.H*0.26, 1.4, bar)
			pt.Line(r.X+r.W*0.16, r.Y+r.H*0.74, r.X+r.W*0.26, r.Y+r.H*0.84, 1.4, bar)
			pt.Line(r.X+r.W*0.74, r.Y+r.H*0.84, r.X+r.W*0.84, r.Y+r.H*0.74, 1.4, bar)
		case "gb":
			pt.Fill(r, ui.Hex("#012169"), 0)
			pt.Line(r.X, r.Y, r.X+r.W, r.Y+r.H, r.H*0.18, ui.Hex("#FFFFFF"))
			pt.Line(r.X+r.W, r.Y, r.X, r.Y+r.H, r.H*0.18, ui.Hex("#FFFFFF"))
			pt.Fill(ui.Rect{X: r.X + r.W*0.41, Y: r.Y, W: r.W * 0.18, H: r.H}, ui.Hex("#FFFFFF"), 0)
			pt.Fill(ui.Rect{X: r.X, Y: r.Y + r.H*0.38, W: r.W, H: r.H * 0.24}, ui.Hex("#FFFFFF"), 0)
			pt.Fill(ui.Rect{X: r.X + r.W*0.44, Y: r.Y, W: r.W * 0.12, H: r.H}, ui.Hex("#C8102E"), 0)
			pt.Fill(ui.Rect{X: r.X, Y: r.Y + r.H*0.42, W: r.W, H: r.H * 0.16}, ui.Hex("#C8102E"), 0)
		case "de":
			paintHBands(pt, r, "#000000", "#DD0000", "#FFCE00")
		case "nl":
			paintHBands(pt, r, "#AE1C28", "#FFFFFF", "#21468B")
		case "ru":
			paintHBands(pt, r, "#FFFFFF", "#0039A6", "#D52B1E")
		case "fr":
			paintVBands(pt, r, "#0055A4", "#FFFFFF", "#EF4135")
		case "it":
			paintVBands(pt, r, "#009246", "#FFFFFF", "#CE2B37")
		case "ca":
			pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * 0.25, H: r.H}, ui.Hex("#FF0000"), 0)
			pt.Fill(ui.Rect{X: r.X + r.W*0.25, Y: r.Y, W: r.W * 0.5, H: r.H}, ui.Hex("#FFFFFF"), 0)
			pt.Fill(ui.Rect{X: r.X + r.W*0.75, Y: r.Y, W: r.W * 0.25, H: r.H}, ui.Hex("#FF0000"), 0)
			cx, cy := r.X+r.W/2, r.Y+r.H/2
			leaf := min(r.W, r.H) * 0.22
			pt.FillPath((&ui.Path{}).MoveTo(cx, cy-leaf).LineTo(cx+leaf, cy).LineTo(cx, cy+leaf).LineTo(cx-leaf, cy).Close(), ui.Hex("#FF0000"))
		case "au":
			pt.Fill(r, ui.Hex("#012169"), 0)
			pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * 0.45, H: r.H * 0.5}, ui.Hex("#C8102E"), 0)
			pt.Fill(ui.Rect{X: r.X + r.W*0.72, Y: r.Y + r.H*0.28, W: 2.2, H: 2.2}, ui.Hex("#FFFFFF"), 1.1)
			pt.Fill(ui.Rect{X: r.X + r.W*0.80, Y: r.Y + r.H*0.58, W: 2.2, H: 2.2}, ui.Hex("#FFFFFF"), 1.1)
		default:
			pt.Fill(r, ui.Hex("#334155"), 0)
			lbl := strings.ToUpper(cc)
			if lbl == "" {
				lbl = "IP"
			}
			if len(lbl) > 2 {
				lbl = lbl[:2]
			}
			pt.Text(r.X+r.W*0.22, r.Y+r.H*0.18, lbl, clampF(r.H*0.55, 7, 10), ui.Hex("#F8FAFC"))
		}
	})
	pt.Stroke(r, ui.Hex("#94A3B8").Alpha(0.32), 2, 0.5)
}

func paintHBands(pt *ui.Painter, r ui.Rect, c1, c2, c3 string) {
	bh := r.H / 3
	pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: bh + 0.5}, ui.Hex(c1), 0)
	pt.Fill(ui.Rect{X: r.X, Y: r.Y + bh, W: r.W, H: bh + 0.5}, ui.Hex(c2), 0)
	pt.Fill(ui.Rect{X: r.X, Y: r.Y + bh*2, W: r.W, H: bh + 0.5}, ui.Hex(c3), 0)
}

func paintVBands(pt *ui.Painter, r ui.Rect, c1, c2, c3 string) {
	bw := r.W / 3
	pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: bw + 0.5, H: r.H}, ui.Hex(c1), 0)
	pt.Fill(ui.Rect{X: r.X + bw, Y: r.Y, W: bw + 0.5, H: r.H}, ui.Hex(c2), 0)
	pt.Fill(ui.Rect{X: r.X + bw*2, Y: r.Y, W: bw + 0.5, H: r.H}, ui.Hex(c3), 0)
}

func paintStar(pt *ui.Painter, cx, cy, outerR float32, col ui.Color) {
	innerR := outerR * 0.42
	p := &ui.Path{}
	for i := 0; i < 10; i++ {
		ang := float64(i)*math.Pi/5 - math.Pi/2
		rad := outerR
		if i%2 == 1 {
			rad = innerR
		}
		x := cx + rad*float32(math.Cos(ang))
		y := cy + rad*float32(math.Sin(ang))
		if i == 0 {
			p.MoveTo(x, y)
		} else {
			p.LineTo(x, y)
		}
	}
	p.Close()
	pt.FillPath(p, col)
}

func probeBrandIcon(c *ui.Context, iconKey, name string, accent ui.Color, dark bool, size float32) {
	key := strings.ToLower(strings.TrimSpace(iconKey))
	if key == "" {
		nl := strings.ToLower(name)
		switch {
		case strings.Contains(nl, "字节") || strings.Contains(nl, "byte"):
			key = "bytedance"
		case strings.Contains(nl, "淘宝") || strings.Contains(nl, "taobao"):
			key = "taobao"
		case strings.Contains(nl, "微信") || strings.Contains(nl, "wechat"):
			key = "weixin"
		case strings.Contains(nl, "github"):
			key = "github"
		case strings.Contains(nl, "cloudflare"):
			key = "cloudflare"
		case strings.Contains(nl, "youtube"):
			key = "youtube"
		}
	}
	ui.Box(c).Size(size, size).Shrink(0).Draw(func(pt *ui.Painter, r ui.Rect) {
		paintProbeBrandIcon(pt, r, key, name, accent, dark)
	})
}

func paintProbeBrandIcon(pt *ui.Painter, r ui.Rect, key, name string, accent ui.Color, dark bool) {
	switch key {
	case "bytedance":
		cols := []string{"#325BFF", "#57C7FF", "#78E6D0", "#3C8CFF"}
		fracs := []float32{0.82, 0.54, 0.72, 0.90}
		bw := r.W * 0.17
		gap := (r.W - bw*4) / 3
		for i := 0; i < 4; i++ {
			bh := r.H * fracs[i]
			x := r.X + float32(i)*(bw+gap)
			y := r.Y + (r.H-bh)*0.55
			pt.Fill(ui.Rect{X: x, Y: y, W: bw, H: bh}, ui.Hex(cols[i]), bw*0.4)
		}
	case "taobao":
		pt.Fill(r, ui.Hex("#FF5000"), r.W*0.24)
		pt.Stroke(ui.Rect{X: r.X + r.W*0.24, Y: r.Y + r.H*0.24, W: r.W * 0.52, H: r.H * 0.52}, ui.Hex("#FFFFFF"), r.W*0.2, 1.5)
		pt.Fill(ui.Rect{X: r.X + r.W*0.42, Y: r.Y + r.H*0.42, W: r.W * 0.16, H: r.H * 0.16}, ui.Hex("#FFFFFF"), r.W*0.08)
	case "weixin":
		pt.Fill(ui.Rect{X: r.X + r.W*0.04, Y: r.Y + r.H*0.08, W: r.W * 0.92, H: r.H * 0.80}, ui.Hex("#07C160"), r.W*0.36)
		pt.FillPath((&ui.Path{}).
			MoveTo(r.X+r.W*0.26, r.Y+r.H*0.80).
			LineTo(r.X+r.W*0.18, r.Y+r.H*0.95).
			LineTo(r.X+r.W*0.44, r.Y+r.H*0.82).Close(), ui.Hex("#07C160"))
		dotR := max(r.W*0.08, 1.2)
		pt.Fill(ui.Rect{X: r.X + r.W*0.34 - dotR, Y: r.Y + r.H*0.44 - dotR, W: dotR * 2, H: dotR * 2}, ui.Hex("#FFFFFF"), dotR)
		pt.Fill(ui.Rect{X: r.X + r.W*0.64 - dotR, Y: r.Y + r.H*0.44 - dotR, W: dotR * 2, H: dotR * 2}, ui.Hex("#FFFFFF"), dotR)
	case "github":
		bg := ui.Hex("#24292F")
		fg := ui.Hex("#FFFFFF")
		if dark {
			bg = ui.Hex("#E6EDF3")
			fg = ui.Hex("#0D1117")
		}
		pt.Fill(r, bg, r.W/2)
		pt.Fill(ui.Rect{X: r.X + r.W*0.24, Y: r.Y + r.H*0.30, W: r.W * 0.52, H: r.H * 0.44}, fg, r.W*0.24)
		pt.FillPath((&ui.Path{}).
			MoveTo(r.X+r.W*0.26, r.Y+r.H*0.40).
			LineTo(r.X+r.W*0.28, r.Y+r.H*0.20).
			LineTo(r.X+r.W*0.42, r.Y+r.H*0.32).Close(), fg)
		pt.FillPath((&ui.Path{}).
			MoveTo(r.X+r.W*0.74, r.Y+r.H*0.40).
			LineTo(r.X+r.W*0.72, r.Y+r.H*0.20).
			LineTo(r.X+r.W*0.58, r.Y+r.H*0.32).Close(), fg)
		pt.Fill(ui.Rect{X: r.X + r.W*0.36, Y: r.Y + r.H*0.68, W: r.W * 0.28, H: r.H * 0.28}, fg, r.W*0.08)
	case "cloudflare":
		orange := ui.Hex("#F38020")
		pt.Fill(ui.Rect{X: r.X + r.W*0.06, Y: r.Y + r.H*0.46, W: r.W * 0.88, H: r.H * 0.34}, orange, r.H*0.17)
		pt.Fill(ui.Rect{X: r.X + r.W*0.22, Y: r.Y + r.H*0.26, W: r.W * 0.44, H: r.W * 0.44}, orange, r.W*0.22)
		pt.Fill(ui.Rect{X: r.X + r.W*0.50, Y: r.Y + r.H*0.36, W: r.W * 0.34, H: r.W * 0.34}, orange, r.W*0.17)
	case "youtube":
		pt.Fill(ui.Rect{X: r.X, Y: r.Y + r.H*0.14, W: r.W, H: r.H * 0.72}, ui.Hex("#FF0000"), r.W*0.22)
		pt.FillPath((&ui.Path{}).
			MoveTo(r.X+r.W*0.40, r.Y+r.H*0.34).
			LineTo(r.X+r.W*0.68, r.Y+r.H*0.50).
			LineTo(r.X+r.W*0.40, r.Y+r.H*0.66).Close(), ui.Hex("#FFFFFF"))
	default:
		pt.Fill(r, accent.Alpha(0.18), r.W*0.24)
		lbl := "?"
		if runes := []rune(strings.TrimSpace(name)); len(runes) > 0 {
			lbl = strings.ToUpper(string(runes[0]))
		}
		pt.Text(r.X+r.W*0.26, r.Y+r.H*0.14, lbl, r.H*0.58, accent)
	}
}

func latencyDotsView(c *ui.Context, dots []int, bp bentoColors, activeIdx int) {
	ui.Box(c).Height(9).FillWidth().Draw(func(pt *ui.Painter, r ui.Rect) {
		paintLatencyDots(pt, r, dots, bp, activeIdx)
	})
}

func paintLatencyDots(pt *ui.Painter, r ui.Rect, dots []int, bp bentoColors, activeIdx int) {
	n := len(dots)
	if n == 0 {
		n = 12
	}
	gap := float32(2.0)
	dotD := clampF((r.W-float32(n-1)*gap)/float32(n), 3.4, 5.8)
	if float32(n)*dotD+float32(n-1)*gap > r.W {
		gap = 1.2
		dotD = max((r.W-float32(n-1)*gap)/float32(n), 2.5)
	}
	y := r.Y + (r.H-dotD)/2
	for i := 0; i < n; i++ {
		ms := -2
		if i < len(dots) {
			ms = dots[i]
		}
		var col ui.Color
		switch {
		case ms == -2:
			col = bp.pendingDot
		case ms < 0:
			col = bp.failDot
		case ms < 100:
			col = bp.good
		case ms < 400:
			col = bp.warn
		default:
			col = bp.slow
		}
		x := r.X + float32(i)*(dotD+gap)
		if x+dotD > r.X+r.W+0.5 {
			break
		}
		dr := ui.Rect{X: x, Y: y, W: dotD, H: dotD}
		if i == activeIdx {
			pt.Fill(ui.Rect{X: x - 1.5, Y: y - 1.5, W: dotD + 3, H: dotD + 3}, col.Alpha(0.32), (dotD+3)/2)
		}
		pt.Fill(dr, col, dotD/2)
	}
}
