package cmdrun

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Return-value display variants.
const (
	StyleVariantDefault = "default"
	StyleVariantDot     = "dot"
	StyleVariantDots    = "dots"
	StyleVariantBadge   = "badge"
	StyleVariantMatrix  = "matrix"
	StyleVariantBar     = "bar"
)

// Animated indicator modes supported on command return values and StatusDot UI.
const (
	AnimNone   = "none"
	AnimPulse  = "pulse"
	AnimRipple = "ripple"
	AnimDots   = "dots"
	AnimWave   = "wave"
	AnimOrbit  = "orbit"
	AnimBlink  = "blink"
	AnimMatrix = "matrix"
)

var templateTokenRe = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

// ValidDisplayVariants returns supported display variant names.
func ValidDisplayVariants() []string {
	return []string{
		StyleVariantDefault,
		StyleVariantDot,
		StyleVariantDots,
		StyleVariantBadge,
		StyleVariantMatrix,
		StyleVariantBar,
	}
}

// ValidDisplayAnimations returns supported dot animation mode names.
func ValidDisplayAnimations() []string {
	return []string{
		AnimNone,
		AnimPulse,
		AnimRipple,
		AnimDots,
		AnimWave,
		AnimOrbit,
		AnimBlink,
		AnimMatrix,
	}
}

// StyleThreshold overrides color, status label, or animation when an extracted
// numeric or string value matches.
type StyleThreshold struct {
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
	Equals    string   `json:"equals,omitempty"`
	Color     string   `json:"color,omitempty"`
	Label     string   `json:"label,omitempty"`
	Animation string   `json:"animation,omitempty"`
}

// Clone returns a deep copy of StyleThreshold.
func (t StyleThreshold) Clone() StyleThreshold {
	out := t
	out.Min = cloneFloatPtr(t.Min)
	out.Max = cloneFloatPtr(t.Max)
	return out
}

// FieldStyle configures presentation of one extracted command field.
type FieldStyle struct {
	Label      string            `json:"label,omitempty"`
	Variant    string            `json:"variant,omitempty"`
	Animation  string            `json:"animation,omitempty"`
	Color      string            `json:"color,omitempty"`
	Unit       string            `json:"unit,omitempty"`
	Prefix     string            `json:"prefix,omitempty"`
	Suffix     string            `json:"suffix,omitempty"`
	Format     string            `json:"format,omitempty"`
	ValueMap   map[string]string `json:"valueMap,omitempty"`
	Fallback   string            `json:"fallback,omitempty"`
	Decimals   *int              `json:"decimals,omitempty"`
	DotCount   int               `json:"dotCount,omitempty"`
	DotSize    float64           `json:"dotSize,omitempty"`
	Speed      float64           `json:"speed,omitempty"`
	Min        *float64          `json:"min,omitempty"`
	Max        *float64          `json:"max,omitempty"`
	Hidden     bool              `json:"hidden,omitempty"`
	Thresholds []StyleThreshold  `json:"thresholds,omitempty"`
}

// Clone returns a deep copy of FieldStyle.
func (f FieldStyle) Clone() FieldStyle {
	out := f
	out.Decimals = cloneIntPtr(f.Decimals)
	out.Min = cloneFloatPtr(f.Min)
	out.Max = cloneFloatPtr(f.Max)
	if f.ValueMap != nil {
		out.ValueMap = cloneStringMap(f.ValueMap)
	}
	if f.Thresholds != nil {
		out.Thresholds = make([]StyleThreshold, len(f.Thresholds))
		for i, th := range f.Thresholds {
			out.Thresholds[i] = th.Clone()
		}
	}
	return out
}

// DisplayStyle configures custom visual presentation and dot animations for
// command return values across /commands, .bar widgets, and script cards.
type DisplayStyle struct {
	PrimaryField string                `json:"primaryField,omitempty"`
	Variant      string                `json:"variant,omitempty"`
	Animation    string                `json:"animation,omitempty"`
	Color        string                `json:"color,omitempty"`
	Unit         string                `json:"unit,omitempty"`
	Prefix       string                `json:"prefix,omitempty"`
	Suffix       string                `json:"suffix,omitempty"`
	Format       string                `json:"format,omitempty"`
	ValueMap     map[string]string     `json:"valueMap,omitempty"`
	Fallback     string                `json:"fallback,omitempty"`
	Decimals     *int                  `json:"decimals,omitempty"`
	DotCount     int                   `json:"dotCount,omitempty"`
	DotSize      float64               `json:"dotSize,omitempty"`
	Speed        float64               `json:"speed,omitempty"`
	Min          *float64              `json:"min,omitempty"`
	Max          *float64              `json:"max,omitempty"`
	Thresholds   []StyleThreshold      `json:"thresholds,omitempty"`
	Fields       map[string]FieldStyle `json:"fields,omitempty"`
}

// ResolvedFieldStyle holds merged styling and formatted display text for one field value.
type ResolvedFieldStyle struct {
	Field          string  `json:"field"`
	Label          string  `json:"label"`
	Raw            any     `json:"raw,omitempty"`
	Value          string  `json:"value"`
	FormattedValue string  `json:"formattedValue"`
	Variant        string  `json:"variant"`
	Animation      string  `json:"animation"`
	Color          string  `json:"color,omitempty"`
	Unit           string  `json:"unit,omitempty"`
	Prefix         string  `json:"prefix,omitempty"`
	Suffix         string  `json:"suffix,omitempty"`
	StatusLabel    string  `json:"statusLabel,omitempty"`
	DotCount       int     `json:"dotCount"`
	DotSize        float64 `json:"dotSize"`
	Speed          float64 `json:"speed"`
	FillFrac       float64 `json:"fillFrac"`
	HasFill        bool    `json:"hasFill,omitempty"`
	Hidden         bool    `json:"hidden,omitempty"`
}

// ResolvedCommand holds every resolved return field and primary metric for a Record.
type ResolvedCommand struct {
	CommandID    string                        `json:"commandId"`
	OK           bool                          `json:"ok"`
	PrimaryField string                        `json:"primaryField,omitempty"`
	Primary      *ResolvedFieldStyle           `json:"primary,omitempty"`
	Fields       map[string]ResolvedFieldStyle `json:"fields,omitempty"`
	Order        []string                      `json:"order,omitempty"`
}

// Clone returns a deep copy of DisplayStyle (nil-safe).
func (d *DisplayStyle) Clone() *DisplayStyle {
	if d == nil {
		return nil
	}
	out := *d
	out.Decimals = cloneIntPtr(d.Decimals)
	out.Min = cloneFloatPtr(d.Min)
	out.Max = cloneFloatPtr(d.Max)
	if d.ValueMap != nil {
		out.ValueMap = cloneStringMap(d.ValueMap)
	}
	if d.Thresholds != nil {
		out.Thresholds = make([]StyleThreshold, len(d.Thresholds))
		for i, th := range d.Thresholds {
			out.Thresholds[i] = th.Clone()
		}
	}
	if d.Fields != nil {
		out.Fields = make(map[string]FieldStyle, len(d.Fields))
		for k, fs := range d.Fields {
			out.Fields[k] = fs.Clone()
		}
	}
	return &out
}

// NormalizeAndValidate canonicalizes Variant/Animation strings and validates style constraints.
func (d *DisplayStyle) NormalizeAndValidate() error {
	if d == nil {
		return nil
	}
	d.PrimaryField = strings.TrimSpace(d.PrimaryField)
	v, a, err := normalizeAndValidateStyleCommon(
		d.Variant, d.Animation, d.DotCount, d.DotSize, d.Speed, d.Decimals, d.Min, d.Max, d.Thresholds, "display",
	)
	if err != nil {
		return err
	}
	d.Variant = v
	d.Animation = a
	d.Color = strings.TrimSpace(d.Color)
	if d.Fields != nil {
		normFields := make(map[string]FieldStyle, len(d.Fields))
		for k, fs := range d.Fields {
			cleanKey := strings.TrimSpace(k)
			if cleanKey == "" {
				return fmt.Errorf("display.fields 中的字段名不能为空")
			}
			fv, fa, err := normalizeAndValidateStyleCommon(
				fs.Variant, fs.Animation, fs.DotCount, fs.DotSize, fs.Speed, fs.Decimals, fs.Min, fs.Max, fs.Thresholds, "display.fields."+cleanKey,
			)
			if err != nil {
				return err
			}
			fs.Variant = fv
			fs.Animation = fa
			fs.Color = strings.TrimSpace(fs.Color)
			normFields[cleanKey] = fs
		}
		d.Fields = normFields
	}
	return nil
}

func normalizeAndValidateStyleCommon(
	variant, anim string,
	dotCount int,
	dotSize, speed float64,
	decimals *int,
	minVal, maxVal *float64,
	thresholds []StyleThreshold,
	scope string,
) (string, string, error) {
	v := strings.ToLower(strings.TrimSpace(variant))
	if v != "" && !isValidVariant(v) {
		return "", "", fmt.Errorf("%s.variant 无效：%q（可选 %s）", scope, variant, strings.Join(ValidDisplayVariants(), "/"))
	}
	a := strings.ToLower(strings.TrimSpace(anim))
	if a != "" && !isValidAnimation(a) {
		return "", "", fmt.Errorf("%s.animation 无效：%q（可选 %s）", scope, anim, strings.Join(ValidDisplayAnimations(), "/"))
	}
	if dotCount != 0 && (dotCount < 1 || dotCount > 12) {
		return "", "", fmt.Errorf("%s.dotCount 需在 1 到 12 之间（当前 %d）", scope, dotCount)
	}
	if dotSize != 0 && (dotSize < 4 || dotSize > 16) {
		return "", "", fmt.Errorf("%s.dotSize 需在 4 到 16 之间（当前 %g）", scope, dotSize)
	}
	if speed != 0 && (speed < 0.25 || speed > 4.0) {
		return "", "", fmt.Errorf("%s.speed 需在 0.25 到 4.0 之间（当前 %g）", scope, speed)
	}
	if decimals != nil && (*decimals < 0 || *decimals > 6) {
		return "", "", fmt.Errorf("%s.decimals 需在 0 到 6 之间（当前 %d）", scope, *decimals)
	}
	if minVal != nil && maxVal != nil && *minVal >= *maxVal {
		return "", "", fmt.Errorf("%s 的 min(%g) 必须小于 max(%g)", scope, *minVal, *maxVal)
	}
	for i := range thresholds {
		ta := strings.ToLower(strings.TrimSpace(thresholds[i].Animation))
		if ta != "" && !isValidAnimation(ta) {
			return "", "", fmt.Errorf("%s.thresholds[%d].animation 无效：%q", scope, i, thresholds[i].Animation)
		}
		thresholds[i].Animation = ta
		thresholds[i].Color = strings.TrimSpace(thresholds[i].Color)
		if thresholds[i].Min != nil && thresholds[i].Max != nil && *thresholds[i].Min > *thresholds[i].Max {
			return "", "", fmt.Errorf("%s.thresholds[%d] 的 min 大于 max", scope, i)
		}
	}
	return v, a, nil
}

func isValidVariant(v string) bool {
	switch v {
	case StyleVariantDefault, StyleVariantDot, StyleVariantDots, StyleVariantBadge, StyleVariantMatrix, StyleVariantBar:
		return true
	}
	return false
}

func isValidAnimation(a string) bool {
	switch a {
	case AnimNone, AnimPulse, AnimRipple, AnimDots, AnimWave, AnimOrbit, AnimBlink, AnimMatrix:
		return true
	}
	return false
}

// PresetDisplayStyle returns a preconfigured DisplayStyle for common dot and badge styles.
func PresetDisplayStyle(preset string) (*DisplayStyle, bool) {
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "default":
		return &DisplayStyle{Variant: StyleVariantDefault, Animation: AnimNone}, true
	case "dot-pulse", "pulse":
		return &DisplayStyle{Variant: StyleVariantDot, Animation: AnimPulse, DotSize: 8, Speed: 1}, true
	case "dot-ripple", "ripple":
		return &DisplayStyle{Variant: StyleVariantDot, Animation: AnimRipple, DotSize: 8, Speed: 1}, true
	case "dot-dots", "dots":
		return &DisplayStyle{Variant: StyleVariantDots, Animation: AnimDots, DotCount: 4, DotSize: 6, Speed: 1}, true
	case "dot-wave", "wave":
		return &DisplayStyle{Variant: StyleVariantDots, Animation: AnimWave, DotCount: 5, DotSize: 6, Speed: 1}, true
	case "dot-orbit", "orbit":
		return &DisplayStyle{Variant: StyleVariantDot, Animation: AnimOrbit, DotCount: 3, DotSize: 6, Speed: 1}, true
	case "dot-blink", "blink":
		return &DisplayStyle{Variant: StyleVariantDot, Animation: AnimBlink, DotSize: 8, Speed: 1}, true
	case "dot-matrix", "matrix":
		return &DisplayStyle{Variant: StyleVariantMatrix, Animation: AnimMatrix, DotCount: 6, DotSize: 5, Speed: 1}, true
	case "badge":
		return &DisplayStyle{Variant: StyleVariantBadge, Animation: AnimPulse, DotSize: 6, Speed: 1}, true
	case "bar":
		return &DisplayStyle{Variant: StyleVariantBar, Animation: AnimNone}, true
	default:
		return nil, false
	}
}

// MergeDisplayStyle overlays non-zero properties from patch onto a clone of base.
func MergeDisplayStyle(base, patch *DisplayStyle) *DisplayStyle {
	if base == nil && patch == nil {
		return nil
	}
	if base == nil {
		return patch.Clone()
	}
	out := base.Clone()
	if patch == nil {
		return out
	}
	if patch.PrimaryField != "" {
		out.PrimaryField = patch.PrimaryField
	}
	if patch.Variant != "" {
		out.Variant = patch.Variant
	}
	if patch.Animation != "" {
		out.Animation = patch.Animation
	}
	if patch.Color != "" {
		out.Color = patch.Color
	}
	if patch.Unit != "" {
		out.Unit = patch.Unit
	}
	if patch.Prefix != "" {
		out.Prefix = patch.Prefix
	}
	if patch.Suffix != "" {
		out.Suffix = patch.Suffix
	}
	if patch.Format != "" {
		out.Format = patch.Format
	}
	if patch.ValueMap != nil {
		out.ValueMap = cloneStringMap(patch.ValueMap)
	}
	if patch.Fallback != "" {
		out.Fallback = patch.Fallback
	}
	if patch.Decimals != nil {
		out.Decimals = cloneIntPtr(patch.Decimals)
	}
	if patch.DotCount > 0 {
		out.DotCount = patch.DotCount
	}
	if patch.DotSize > 0 {
		out.DotSize = patch.DotSize
	}
	if patch.Speed > 0 {
		out.Speed = patch.Speed
	}
	if patch.Min != nil {
		out.Min = cloneFloatPtr(patch.Min)
	}
	if patch.Max != nil {
		out.Max = cloneFloatPtr(patch.Max)
	}
	if patch.Thresholds != nil {
		out.Thresholds = make([]StyleThreshold, len(patch.Thresholds))
		for i, th := range patch.Thresholds {
			out.Thresholds[i] = th.Clone()
		}
	}
	if patch.Fields != nil {
		if out.Fields == nil {
			out.Fields = make(map[string]FieldStyle, len(patch.Fields))
		}
		for k, pfs := range patch.Fields {
			if existing, ok := out.Fields[k]; ok {
				out.Fields[k] = mergeFieldStyle(existing, pfs)
			} else {
				out.Fields[k] = pfs.Clone()
			}
		}
	}
	return out
}

func mergeFieldStyle(base, patch FieldStyle) FieldStyle {
	out := base.Clone()
	if patch.Label != "" {
		out.Label = patch.Label
	}
	if patch.Variant != "" {
		out.Variant = patch.Variant
	}
	if patch.Animation != "" {
		out.Animation = patch.Animation
	}
	if patch.Color != "" {
		out.Color = patch.Color
	}
	if patch.Unit != "" {
		out.Unit = patch.Unit
	}
	if patch.Prefix != "" {
		out.Prefix = patch.Prefix
	}
	if patch.Suffix != "" {
		out.Suffix = patch.Suffix
	}
	if patch.Format != "" {
		out.Format = patch.Format
	}
	if patch.ValueMap != nil {
		out.ValueMap = cloneStringMap(patch.ValueMap)
	}
	if patch.Fallback != "" {
		out.Fallback = patch.Fallback
	}
	if patch.Decimals != nil {
		out.Decimals = cloneIntPtr(patch.Decimals)
	}
	if patch.DotCount > 0 {
		out.DotCount = patch.DotCount
	}
	if patch.DotSize > 0 {
		out.DotSize = patch.DotSize
	}
	if patch.Speed > 0 {
		out.Speed = patch.Speed
	}
	if patch.Min != nil {
		out.Min = cloneFloatPtr(patch.Min)
	}
	if patch.Max != nil {
		out.Max = cloneFloatPtr(patch.Max)
	}
	if patch.Hidden {
		out.Hidden = true
	}
	if patch.Thresholds != nil {
		out.Thresholds = make([]StyleThreshold, len(patch.Thresholds))
		for i, th := range patch.Thresholds {
			out.Thresholds[i] = th.Clone()
		}
	}
	return out
}

// ResolveRecord resolves all extracted fields of rec using DisplayStyle rules,
// producing ordered fields and a resolved Primary field.
func (d *DisplayStyle) ResolveRecord(rec Record) ResolvedCommand {
	primaryKey := ""
	if d != nil {
		primaryKey = d.PrimaryField
	}
	out := ResolvedCommand{
		CommandID:    rec.CommandID,
		OK:           rec.OK,
		PrimaryField: primaryKey,
		Fields:       map[string]ResolvedFieldStyle{},
	}

	seen := map[string]bool{}
	var keys []string
	addKey := func(k string) {
		if k == "" || seen[k] {
			return
		}
		seen[k] = true
		keys = append(keys, k)
	}
	if primaryKey != "" {
		addKey(primaryKey)
	}
	for k := range rec.Extracted {
		addKey(k)
	}
	if d != nil {
		for k, fs := range d.Fields {
			if fs.Fallback != "" {
				addKey(k)
			}
		}
	}
	sort.SliceStable(keys, func(i, j int) bool {
		if primaryKey != "" {
			if keys[i] == primaryKey {
				return true
			}
			if keys[j] == primaryKey {
				return false
			}
		}
		return keys[i] < keys[j]
	})

	for _, k := range keys {
		val := rec.Extracted[k]
		res := d.resolveFieldWithContext(k, val, rec.OK, rec.Extracted, rec.Params)
		out.Fields[k] = res
		if !res.Hidden {
			out.Order = append(out.Order, k)
		}
	}

	if primaryKey != "" {
		if pf, ok := out.Fields[primaryKey]; ok && !pf.Hidden {
			cp := pf
			out.Primary = &cp
		}
	}
	if out.Primary == nil && len(out.Order) > 0 {
		first := out.Order[0]
		out.PrimaryField = first
		cp := out.Fields[first]
		out.Primary = &cp
	}
	return out
}

// ResolveField merges DisplayStyle defaults, per-field overrides, and matching
// thresholds to resolve final formatting, variant, color, and dot animation props.
func (d *DisplayStyle) ResolveField(field string, val any, ok bool) ResolvedFieldStyle {
	return d.resolveFieldWithContext(field, val, ok, nil, nil)
}

func (d *DisplayStyle) resolveFieldWithContext(
	field string,
	val any,
	ok bool,
	extracted map[string]any,
	params map[string]string,
) ResolvedFieldStyle {
	out := ResolvedFieldStyle{
		Field:     field,
		Label:     field,
		Raw:       val,
		Variant:   StyleVariantDefault,
		Animation: AnimNone,
		DotCount:  4,
		DotSize:   7,
		Speed:     1.0,
	}

	var thresholds []StyleThreshold
	var decimals *int
	var minV, maxV *float64
	var formatTmpl string
	var valueMap map[string]string
	fallbackText := "--"

	if d != nil {
		isPrimary := d.PrimaryField == "" || d.PrimaryField == field
		if isPrimary {
			if d.Variant != "" {
				out.Variant = d.Variant
			}
			if d.Animation != "" {
				out.Animation = d.Animation
			}
			if d.Color != "" {
				out.Color = d.Color
			}
			if d.Unit != "" {
				out.Unit = d.Unit
			}
			if d.Prefix != "" {
				out.Prefix = d.Prefix
			}
			if d.Suffix != "" {
				out.Suffix = d.Suffix
			}
			if d.Format != "" {
				formatTmpl = d.Format
			}
			if d.ValueMap != nil {
				valueMap = d.ValueMap
			}
			if d.Fallback != "" {
				fallbackText = d.Fallback
			}
			if d.Decimals != nil {
				decimals = d.Decimals
			}
			if d.Min != nil {
				minV = d.Min
			}
			if d.Max != nil {
				maxV = d.Max
			}
			thresholds = d.Thresholds
		}
		if d.DotCount > 0 {
			out.DotCount = d.DotCount
		}
		if d.DotSize > 0 {
			out.DotSize = d.DotSize
		}
		if d.Speed > 0 {
			out.Speed = d.Speed
		}

		if fs, exists := d.Fields[field]; exists {
			if fs.Label != "" {
				out.Label = fs.Label
			}
			if fs.Variant != "" {
				out.Variant = fs.Variant
			}
			if fs.Animation != "" {
				out.Animation = fs.Animation
			}
			if fs.Color != "" {
				out.Color = fs.Color
			}
			if fs.Unit != "" {
				out.Unit = fs.Unit
			}
			if fs.Prefix != "" {
				out.Prefix = fs.Prefix
			}
			if fs.Suffix != "" {
				out.Suffix = fs.Suffix
			}
			if fs.Format != "" {
				formatTmpl = fs.Format
			}
			if fs.ValueMap != nil {
				valueMap = fs.ValueMap
			}
			if fs.Fallback != "" {
				fallbackText = fs.Fallback
			}
			if fs.Decimals != nil {
				decimals = fs.Decimals
			}
			if fs.DotCount > 0 {
				out.DotCount = fs.DotCount
			}
			if fs.DotSize > 0 {
				out.DotSize = fs.DotSize
			}
			if fs.Speed > 0 {
				out.Speed = fs.Speed
			}
			if fs.Min != nil {
				minV = fs.Min
			}
			if fs.Max != nil {
				maxV = fs.Max
			}
			if fs.Hidden {
				out.Hidden = true
			}
			if len(fs.Thresholds) > 0 {
				thresholds = fs.Thresholds
			}
		}
	}

	numVal, isNum := asFloat64(val)
	rawStr := ""
	if val != nil {
		rawStr = fmt.Sprint(val)
	}

	for _, th := range thresholds {
		if matchThreshold(th, numVal, isNum, rawStr) {
			if th.Color != "" {
				out.Color = th.Color
			}
			if th.Label != "" {
				out.StatusLabel = th.Label
			}
			if th.Animation != "" {
				out.Animation = th.Animation
			}
			break
		}
	}

	if !ok && out.Color == "" {
		out.Color = "danger"
	}

	// Harmonize default variant/animation combinations cleanly.
	if out.Variant == StyleVariantDefault && out.Animation != "" && out.Animation != AnimNone {
		switch out.Animation {
		case AnimMatrix:
			out.Variant = StyleVariantMatrix
		case AnimDots, AnimWave:
			out.Variant = StyleVariantDots
		default:
			out.Variant = StyleVariantDot
		}
	} else if out.Animation == "" || out.Animation == AnimNone {
		switch out.Variant {
		case StyleVariantMatrix:
			out.Animation = AnimMatrix
		case StyleVariantDots:
			out.Animation = AnimDots
		case StyleVariantDot:
			out.Animation = AnimPulse
		}
	}

	coreVal := formatCoreValue(val, numVal, isNum, decimals, fallbackText)
	mapped := false
	if val != nil && len(valueMap) > 0 {
		if mv, found := lookupValueMap(valueMap, rawStr, coreVal); found {
			coreVal = mv
			mapped = true
		}
	}
	out.Value = coreVal

	if formatTmpl != "" && val != nil {
		out.FormattedValue = expandFormatTemplate(formatTmpl, out, rawStr, extracted, params)
	} else if val == nil {
		out.FormattedValue = fallbackText
	} else {
		unitStr := out.Unit
		if mapped {
			unitStr = ""
		}
		out.FormattedValue = wrapPrefixUnitSuffix(coreVal, out.Prefix, unitStr, out.Suffix)
	}

	if isNum {
		if minV != nil && maxV != nil && *maxV > *minV {
			frac := (numVal - *minV) / (*maxV - *minV)
			out.FillFrac = math.Max(0, math.Min(1, frac))
			out.HasFill = true
		} else if maxV != nil && *maxV > 0 {
			frac := numVal / *maxV
			out.FillFrac = math.Max(0, math.Min(1, frac))
			out.HasFill = true
		} else if out.Variant == StyleVariantBar || out.Variant == StyleVariantMatrix {
			out.FillFrac = math.Max(0, math.Min(1, numVal/100.0))
			out.HasFill = true
		}
	}

	return out
}

func expandFormatTemplate(
	tmpl string,
	cur ResolvedFieldStyle,
	rawStr string,
	extracted map[string]any,
	params map[string]string,
) string {
	return templateTokenRe.ReplaceAllStringFunc(tmpl, func(tok string) string {
		m := templateTokenRe.FindStringSubmatch(tok)
		if len(m) < 2 {
			return tok
		}
		key := m[1]
		switch key {
		case "value":
			return cur.Value
		case "raw":
			return rawStr
		case "unit":
			return cur.Unit
		case "label":
			return cur.Label
		case "status":
			return cur.StatusLabel
		case "field":
			return cur.Field
		}
		if strings.HasPrefix(key, "param.") {
			pkey := strings.TrimPrefix(key, "param.")
			if params != nil {
				return params[pkey]
			}
			return ""
		}
		if extracted != nil {
			if sibVal, ok := extracted[key]; ok && sibVal != nil {
				num, isN := asFloat64(sibVal)
				return formatCoreValue(sibVal, num, isN, nil, "--")
			}
		}
		return ""
	})
}

func lookupValueMap(m map[string]string, rawStr, formattedCore string) (string, bool) {
	if v, ok := m[rawStr]; ok {
		return v, true
	}
	if v, ok := m[formattedCore]; ok {
		return v, true
	}
	trimmedRaw := strings.TrimSpace(rawStr)
	trimmedCore := strings.TrimSpace(formattedCore)
	for k, v := range m {
		tk := strings.TrimSpace(k)
		if strings.EqualFold(tk, trimmedRaw) || strings.EqualFold(tk, trimmedCore) {
			return v, true
		}
	}
	return "", false
}

func matchThreshold(th StyleThreshold, num float64, isNum bool, rawStr string) bool {
	if th.Equals != "" {
		return strings.EqualFold(strings.TrimSpace(rawStr), strings.TrimSpace(th.Equals))
	}
	if th.Min == nil && th.Max == nil {
		return true
	}
	if !isNum {
		return false
	}
	if th.Min != nil && num < *th.Min {
		return false
	}
	if th.Max != nil && num > *th.Max {
		return false
	}
	return true
}

func formatCoreValue(val any, num float64, isNum bool, decimals *int, fallback string) string {
	switch {
	case val == nil:
		if fallback != "" {
			return fallback
		}
		return "--"
	case isNum && decimals != nil:
		return strconv.FormatFloat(num, 'f', *decimals, 64)
	case isNum:
		if num == math.Trunc(num) {
			return strconv.FormatFloat(num, 'f', 0, 64)
		}
		return strconv.FormatFloat(num, 'f', 1, 64)
	default:
		return fmt.Sprint(val)
	}
}

func wrapPrefixUnitSuffix(body, prefix, unit, suffix string) string {
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString(body)
	if unit != "" {
		if unit != "%" {
			b.WriteByte(' ')
		}
		b.WriteString(unit)
	}
	b.WriteString(suffix)
	return b.String()
}

func asFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, !math.IsNaN(x) && !math.IsInf(x, 0)
	case float32:
		f := float64(x)
		return f, !math.IsNaN(f) && !math.IsInf(f, 0)
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case int32:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint64:
		return float64(x), true
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		return f, !math.IsNaN(f) && !math.IsInf(f, 0)
	default:
		return 0, false
	}
}

func cloneFloatPtr(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func cloneIntPtr(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func cloneStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
