package cmdrun

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	BuiltinPingID      = "net-ping"
	BuiltinPingShell   = "sh"
	BuiltinPingCommand = `HOST="${SB_PARAM_HOST:-223.5.5.5}"
COUNT="${SB_PARAM_COUNT:-3}"
case "$HOST" in -*|*[!a-zA-Z0-9._:-]*) echo "invalid host" >&2; exit 2;; esac
case "$COUNT" in ''|*[!0-9]*) COUNT=3;; esac
[ "$COUNT" -lt 1 ] && COUNT=1
[ "$COUNT" -gt 10 ] && COUNT=10
if [ "$(uname -s)" = "Darwin" ]; then
  ping -n -c "$COUNT" -W 2000 "$HOST"
else
  ping -n -c "$COUNT" -W 2 "$HOST"
fi`
)

var safeHostPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,252}$`)

// BuiltinPingSpec returns the built-in network ping command specification.
func BuiltinPingSpec() Spec {
	return Spec{
		ID:          BuiltinPingID,
		Name:        "网络 Ping 检测",
		Description: "快速检测本机到目标主机的 ICMP 延迟、抖动与丢包率（支持自定义目标与历史趋势）",
		Shell:       BuiltinPingShell,
		Command:     BuiltinPingCommand,
		Every:       60,
		Timeout:     10,
		MaxHistory:  200,
		Builtin:     true,
		Params: []Param{
			{
				Key:         "host",
				Label:       "目标主机",
				Type:        "string",
				Default:     "223.5.5.5",
				Placeholder: "223.5.5.5 / 1.1.1.1 / github.com",
				Required:    true,
			},
			{
				Key:     "count",
				Label:   "发包数",
				Type:    "select",
				Default: "3",
				Options: []string{"1", "3", "5"},
			},
		},
		Extract: []ExtractRule{
			{Name: "lossPct", Kind: KindRegex, Pattern: `([0-9]+(?:\.[0-9]+)?)% packet loss`, Type: "number"},
			{Name: "minMs", Kind: KindRegex, Pattern: `(?:round-trip|rtt) min/avg/max/(?:stddev|mdev) = ([0-9.]+)/`, Type: "number"},
			{Name: "avgMs", Kind: KindRegex, Pattern: `(?:round-trip|rtt) min/avg/max/(?:stddev|mdev) = [0-9.]+/([0-9.]+)/`, Type: "number"},
			{Name: "maxMs", Kind: KindRegex, Pattern: `(?:round-trip|rtt) min/avg/max/(?:stddev|mdev) = [0-9.]+/[0-9.]+/([0-9.]+)/`, Type: "number"},
			{Name: "jitterMs", Kind: KindRegex, Pattern: `(?:round-trip|rtt) min/avg/max/(?:stddev|mdev) = [0-9.]+/[0-9.]+/[0-9.]+/([0-9.]+)`, Type: "number"},
			{Name: "resolvedIp", Kind: KindRegex, Pattern: `PING [^\s]+ \(([0-9a-fA-F.:]+)\)`, Type: "string"},
		},
		Display: builtinPingDisplayStyle(),
	}
}

func builtinPingDisplayStyle() *DisplayStyle {
	dec1 := 1
	dec0 := 0
	maxFast := 50.0
	maxNormal := 120.0
	maxZero := 0.0
	return &DisplayStyle{
		PrimaryField: "avgMs",
		Variant:      StyleVariantDot,
		Animation:    AnimRipple,
		DotSize:      8,
		DotCount:     5,
		Speed:        1.0,
		Fields: map[string]FieldStyle{
			"avgMs": {
				Label:    "平均延迟",
				Unit:     "ms",
				Decimals: &dec1,
				Thresholds: []StyleThreshold{
					{Max: &maxFast, Color: "success", Label: "极速"},
					{Max: &maxNormal, Color: "accent", Label: "畅通"},
					{Color: "warning", Label: "偏高"},
				},
			},
			"lossPct": {
				Label:    "丢包率",
				Unit:     "%",
				Decimals: &dec0,
				Variant:  StyleVariantBadge,
				Thresholds: []StyleThreshold{
					{Max: &maxZero, Color: "success"},
					{Color: "danger", Animation: AnimBlink},
				},
			},
			"jitterMs":   {Label: "抖动", Unit: "ms", Decimals: &dec1},
			"minMs":      {Label: "最小", Unit: "ms", Decimals: &dec1},
			"maxMs":      {Label: "最大", Unit: "ms", Decimals: &dec1},
			"resolvedIp": {Label: "解析 IP"},
		},
	}
}

// IsTrustedBuiltin reports whether spec matches an unmodified shipped built-in command
// that may run without enabling the global arbitrary-exec toggle.
func IsTrustedBuiltin(s Spec) bool {
	return s.Builtin &&
		s.ID == BuiltinPingID &&
		s.Shell == BuiltinPingShell &&
		s.Command == BuiltinPingCommand &&
		s.WorkDir == "" &&
		len(s.Env) == 0
}

// ValidateBuiltinPingParams enforces strict safety constraints on net-ping parameters.
func ValidateBuiltinPingParams(params map[string]string) error {
	host := strings.TrimSpace(params["host"])
	if !safeHostPattern.MatchString(host) || strings.HasPrefix(host, "-") {
		return fmt.Errorf("目标主机格式无效：%q", host)
	}
	cntRaw := strings.TrimSpace(params["count"])
	if cntRaw == "" {
		return nil
	}
	n, err := strconv.Atoi(cntRaw)
	if err != nil || n < 1 || n > 10 {
		return fmt.Errorf("发包数必须在 1 到 10 之间（当前 %q）", cntRaw)
	}
	return nil
}
