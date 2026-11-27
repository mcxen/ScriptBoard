# ScriptBoard Design System — Paper & Ink

> Inspired by `Leonxlnx/taste-skill`, `pbakaus/impeccable` ("Operate" dashboard mode), Swiss typographic grids, and native macOS restraint.

ScriptBoard is a quiet, high-craft personal dashboard and menubar workspace. Data is the hero; interface chrome steps back.

---

## 1. Visual Philosophy

1. **Paper & Ink Surfaces (No Generic SaaS Slop)**
   - Daylight uses warm archival paper (`#F5F4F0`) paired with crisp white cards (`#FFFFFF`) and stone-warm hairlines rather than cold Tailwind slate (`#F5F6F9`) or clinical blue gradients.
   - Dark mode uses neutral charcoal obsidian (`#131314` / `#1C1C1F`) rather than blue-tinted slate blacks (`#111318`).
2. **Ink First, Single Quiet Accent**
   - Structural hierarchy comes from **ink contrast, weight, spacing, and 1px hairlines**—never from blue-filled buttons, rainbow pills, or glowing borders.
   - Primary UI ink (`#181715` light / `#F0EFEA` dark) carries navigation selection and primary hero values. Prussian ink accent (`#23537A` light / `#3F76AD` dark) is reserved cleanly for interactive focus, selected chart markers, and intentional highlights.
3. **Operate-Mode Clarity**
   - Avoid wrapping metrics inside nested grey boxes ("cards within cards"). Use whitespace, 1px hairline dividers, and baseline typographic alignment inside each widget card.
   - Default views remain crisp and flat; neon glows, LED dot matrices, and decorative gradients are **strictly opt-in** when explicitly configured (`style: "neon"` / `"matrix"`).

---

## 2. Canonical Color Tokens

### Light Theme — Warm Paper

| Token | Value | Role |
| --- | --- | --- |
| Canvas (`BgLayout`, `canvas`) | `#F5F4F0` | Main workspace paper background |
| Sidebar (`sidebar`) | `#ECEAE4` | Left navigation rail & docked shelf |
| Card (`BgContainer`, `card`) | `#FFFFFF` | Card surface |
| Elevated (`BgElevated`) | `#FFFFFF` | Popovers, menus, modal sheets |
| Recessed / Chip (`FillTertiary`, `chip`) | `rgba(28, 25, 23, 0.055)` | Segmented tracks, subtle pills, progress tracks |
| Hover (`FillQuaternary`, `hover`) | `rgba(28, 25, 23, 0.042)` | Interactive row/button hover tint |
| Hairline (`BorderSecondary`, `hairline`) | `rgba(28, 25, 23, 0.085)` | Card edges, dividers, column rules |
| Control Border (`Border`, `border`) | `rgba(28, 25, 23, 0.14)` | Interactive outlines & focused hairlines |
| Ink Primary (`Text`, `text`) | `#181715` | Headings, hero numerals, active nav |
| Ink Muted (`TextSecondary`, `muted`) | `#625F59` | Kickers, labels, units, subtitles |
| Ink Faint (`TextTertiary`, `faint`) | `#8F8B82` | Metadata, empty `--` readouts, timestamps |
| Accent (`Primary`, `accent`) | `#23537A` | Prussian ink accent (hover `#1B4363`, active `#15354F`) |
| Semantic Success / Down | `#1F7A5A` | Online status, positive completion |
| Semantic Warning | `#B46914` | Offline/stale notices, warm thresholds |
| Semantic Error / Up | `#C53D3D` | Errors, critical thresholds, market up-tick |

### Dark Theme — Neutral Charcoal Ink

| Token | Value | Role |
| --- | --- | --- |
| Canvas (`BgLayout`, `canvas`) | `#131314` | Main workspace charcoal background |
| Sidebar (`sidebar`) | `#18181A` | Left navigation rail & docked shelf |
| Card (`BgContainer`, `card`) | `#1C1C1F` | Elevated charcoal card surface |
| Elevated (`BgElevated`) | `#242428` | Popovers, menus, modal sheets |
| Recessed / Chip (`FillTertiary`, `chip`) | `rgba(255, 255, 255, 0.075)` | Segmented tracks, subtle pills, tracks |
| Hover (`FillQuaternary`, `hover`) | `rgba(255, 255, 255, 0.048)` | Interactive row/button hover tint |
| Hairline (`BorderSecondary`, `hairline`) | `rgba(255, 255, 255, 0.075)` | Card edges, dividers, column rules |
| Control Border (`Border`, `border`) | `rgba(255, 255, 255, 0.13)` | Interactive outlines & focused hairlines |
| Ink Primary (`Text`, `text`) | `#F0EFEA` | Warm bone ink for headings, hero figures, active nav |
| Ink Muted (`TextSecondary`, `muted`) | `#9D9A93` | Kickers, labels, units, subtitles |
| Ink Faint (`TextTertiary`, `faint`) | `#6E6B65` | Metadata, empty `--` readouts, timestamps |
| Accent (`Primary`, `accent`) | `#3F76AD` | Muted Prussian blue accent (hover `#5187BE`, active `#326394`) |
| Semantic Success / Down | `#4AA884` | Online status, positive completion |
| Semantic Warning | `#D99B43` | Offline/stale notices, warm thresholds |
| Semantic Error / Up | `#DC6B6B` | Errors, critical thresholds, market up-tick |

### Archival Data Palette

Use this restrained archival sequence for multi-series charts, donuts, tags, and visual templates instead of saturated neons (`#00F5D4`, `#FF2A85`, `#A855F7`):

1. **Prussian Blue**: `#23537A`
2. **Pine Sage**: `#2A7B62`
3. **Burnt Ochre**: `#C26D2D`
4. **Terracotta**: `#B84A4A`
5. **Muted Indigo**: `#5B5F97`
6. **Mineral Teal**: `#3D7A80`
7. **Warm Taupe**: `#8C6A5D`

---

## 3. Typography & Rhythm

- **Editorial Masthead (Home)**:
  - Eyebrow line first (`11.5px`, weight `500`, `tnum`, `muted`) carrying date & widget count.
  - Display title underneath (`26px` desktop / `24px` compact, weight `600`, tight tracking `-0.65`, primary ink).
- **Tabular Numerals Everywhere (`FontFeatures("tnum")`)**:
  - Mandatory on every changing number, time readout, percentage delta, table numeric column, countdown, and scale badge.
- **Hero Number + Unit Pairing**:
  - Display hero metrics (`big`) with tight tracking (`-1.3`) and light weight (`350`) for effortless scannability.
  - When formatted as `<number> <unit>` (e.g., `65.3 kg`, `35 天`), separate the unit onto the baseline at `0.44×` scale (`12–16px`), weight `450`, in `muted` ink while keeping full accessibility label fidelity.
- **Quiet Card Kickers**:
  - Always give cards calm context via an understated `12px` `muted` kicker row (or explicit `layout: { title: "show" }`), avoiding naked floating numerals (`7.0824`) whose subject is invisible until hover.

---

## 4. Radii, Elevation & Density

- **Radii**: `radCard = 14`, `radControl = 7` (compact segmented pills/menus `8–12`).
- **Elevation**: Prefer crisp `1px` `hairline`/`border` boundaries plus whisper-light paper shadow (`0 1px 2px rgba(28,25,23,0.035)` light; `0 1px 2px rgba(0,0,0,0.18)` dark). Never use heavy drop shadows or outer glow halos on standard UI cards.
- **Card Padding**: Generous `18 * scale` (`13 * scale` narrow) so content breathes; keep inter-card grid spacing clean (`16px` desktop, `12px` mobile).
- **Header Toolbar Budget**: Home masthead right side carries **at most 3 quiet controls**:
  1. Recessed segmented default size switch (`小 | 中 | 大`)
  2. Compact scale readout pill (`100%`) opening an on-demand popover slider
  3. Clean outline `编辑布局` / solid-ink `完成` toggle

---

## 5. Component Discipline

- **Sidebar**: Minimal typographic wordmark (8px geometric ink square + product title + faint subtitle) without colored icon-in-rounded-square badges (`iconBadge`). Active row uses high-contrast ink (`p.text`) on warm elevated selection tile (`rowSel`), never bright accent text or neon indicators.
- **Quick Entry inside Cards**: Use secondary outline controls (`btn`, never `primaryBtn`) inside cards so form buttons never overpower the card's primary telemetry figure.
- **Countdowns**: Align numbers cleanly in baseline/column groups divided by `1px` vertical hairlines—no nested grey rounded boxes around day/hour/minute units and no neon glow bars by default (`style: "solid"` `5px` bar).
- **Offline / Empty (`--`)**: Render `--` in quiet `faint` ink (`26px`, weight `350`) with calm `muted`/`faint` explanatory caption—never shouty alert styling.

---

## 6. Anti-AI-Slop Checklist (Pre-Merge Gate)

Before finalizing any UI view, widget, built-in script, or `.bar` component:

- [ ] **No `✨ sparkles` in chrome or navigation**: never use `sparkles` for sliders, menubar navigation, or generic empty states.
- [ ] **No `iconBadge` colored-square clichés in headers**: use clean typography + hairline structure.
- [ ] **No unrequested neon glow or candy rainbow**: default presets and built-in scripts use Paper & Ink / Archival Data Palette (`neon` / `matrix` are opt-in styles only).
- [ ] **No cards-within-cards**: use hairlines and spacing, not grey rounded boxes nested inside white rounded cards.
- [ ] **No loud primary buttons inside data cards**: secondary in-card inputs use outline/ghost controls.
- [ ] **No naked ambiguous figures**: scripts displaying raw domain values (`USD / CNY`, `体重`, `天气`, `金价`) keep `layout: { title: "show" }` or clear internal labels.
- [ ] **Visual QA passed**: inspect rendered PNG screenshots across Light desktop, Dark desktop, Rich widgets, and Compact phone widths (`390×844`) for alignment, clipping, and restraint.
