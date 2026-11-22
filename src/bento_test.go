package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
)

func mockIPNetCoffeeReport() map[string]any {
	return map[string]any{
		"fetchedAt": time.Date(2026, 11, 22, 21, 15, 0, 0, time.Local).UnixMilli(),
		"hero": map[string]any{
			"ip":          "39.180.138.19",
			"countryCode": "cn",
			"country":     "China",
			"region":      "Zhejiang",
			"city":        "Hangzhou",
			"isp":         "China Mobile",
			"geoZh":       "中国 浙江 杭州 余杭 移动",
			"geoEn":       "China Zhejiang Hangzhou China Mobile",
			"source":      "iP138.com / IP.cn",
			"propertyZh":  "家庭宽带 (ISP)",
			"propertyEn":  "Residential (ISP)",
			"risk": map[string]any{
				"ip":             "39.180.138.19",
				"cidr":           "39.180.138.0/24",
				"asn":            56041,
				"asOrganization": "China Mobile Communications Corporation",
				"company_type":   "isp",
				"isResidential":  true,
				"is_datacenter":  false,
				"abuser_score":   "0.0008 (Low)",
				"timezone":       "Asia/Shanghai",
			},
		},
		"probes": []any{
			map[string]any{
				"name": "字节跳动", "enName": "ByteDance", "icon": "bytedance", "flag": "cn", "tag": "domestic",
				"medianMs": 19, "minMs": 17, "maxMs": 22, "success": 12, "total": 12,
				"dots": []int{19, 18, 20, 19, 21, 18, 19, 17, 20, 19, 18, 22},
			},
			map[string]any{
				"name": "淘宝", "enName": "Taobao", "icon": "taobao", "flag": "cn", "tag": "domestic",
				"medianMs": 34, "minMs": 31, "maxMs": 39, "success": 12, "total": 12,
				"dots": []int{34, 33, 36, 35, 32, 34, 38, 33, 35, 31, 34, 39},
			},
			map[string]any{
				"name": "微信", "enName": "WeChat", "icon": "weixin", "flag": "cn", "tag": "domestic",
				"medianMs": 30, "minMs": 25, "maxMs": 36, "success": 12, "total": 12,
				"dots": []int{30, 28, 31, 29, 32, 27, 30, 34, 25, 31, 30, 36},
			},
			map[string]any{
				"name": "GitHub", "enName": "GitHub", "icon": "github", "tag": "international",
				"medianMs": 53, "minMs": 49, "maxMs": 62, "success": 12, "total": 12,
				"dots": []int{53, 55, 51, 54, 58, 52, 49, 53, 56, 51, 54, 62},
			},
			map[string]any{
				"name": "Cloudflare", "enName": "Cloudflare", "icon": "cloudflare", "tag": "international",
				"medianMs": -1, "minMs": -1, "maxMs": -1, "success": 0, "total": 12,
				"dots": []int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
			},
			map[string]any{
				"name": "YouTube", "enName": "YouTube", "icon": "youtube", "tag": "international",
				"medianMs": 189, "minMs": 181, "maxMs": 212, "success": 12, "total": 12,
				"dots": []int{189, 192, 185, 196, 184, 188, 207, 181, 190, 187, 194, 212},
			},
		},
		"exits": []any{
			map[string]any{
				"ip": "39.180.138.19", "countryCode": "cn", "isp": "China Mobile",
				"geoZh": "中国 浙江 杭州 余杭 移动", "geoEn": "China Zhejiang Hangzhou China Mobile",
				"propertyZh": "家庭宽带 (ISP)", "propertyEn": "Residential (ISP)",
				"count": 4, "sites": []string{"网易", "字节跳动", "Cloudflare中国", "高通中国"},
				"risk": map[string]any{
					"ip": "39.180.138.19", "cidr": "39.180.138.0/24", "asn": 56041,
					"asOrganization": "China Mobile Communications Corporation",
					"isResidential":  true, "abuser_score": "0.0008 (Low)", "timezone": "Asia/Shanghai",
				},
			},
			map[string]any{
				"ip": "54.65.230.180", "countryCode": "jp", "isp": "Amazon.com",
				"geoZh": "日本 Tokyo Amazon.com", "geoEn": "Japan Tokyo Amazon.com",
				"propertyZh": "机房 IP (Hosting)", "propertyEn": "Datacenter (Hosting)",
				"count": 3, "sites": []string{"discord.com", "anthropic.com", "grok.com"},
				"risk": map[string]any{
					"ip": "54.65.230.180", "cidr": "54.65.0.0/16", "asn": 16509,
					"asOrganization": "Amazon.com, Inc.", "is_datacenter": true,
					"abuser_score": "0.0012 (Low)", "timezone": "Asia/Tokyo",
				},
			},
			map[string]any{
				"ip": "13.158.209.152", "countryCode": "jp", "isp": "Amazon.com",
				"geoZh": "日本 Tokyo Amazon.com", "geoEn": "Japan Tokyo Amazon.com",
				"propertyZh": "机房 IP (Hosting)", "propertyEn": "Datacenter (Hosting)",
				"count": 11, "sites": []string{"x.com", "claude.ai", "chatgpt.com", "openai.com"},
				"risk": map[string]any{
					"ip": "13.158.209.152", "cidr": "13.158.0.0/16", "asn": 16509,
					"asOrganization": "Amazon.com, Inc.", "is_datacenter": true,
					"abuser_score": "0.0010 (Low)", "timezone": "Asia/Tokyo",
				},
			},
		},
		"splits": []any{
			map[string]any{"name": "网易", "type": "domestic", "ip": "39.180.138.19", "countryCode": "cn", "geoZh": "中国 浙江 杭州 移动", "geoEn": "China Zhejiang Hangzhou China Mobile", "latencyMs": 18, "status": "ok"},
			map[string]any{"name": "字节跳动", "type": "domestic", "ip": "39.180.138.19", "countryCode": "cn", "geoZh": "中国 浙江 杭州 移动", "geoEn": "China Zhejiang Hangzhou China Mobile", "latencyMs": 19, "status": "ok"},
			map[string]any{"name": "Cloudflare中国", "type": "domestic", "ip": "39.180.138.19", "countryCode": "cn", "geoZh": "中国 浙江 杭州 移动", "geoEn": "China Zhejiang Hangzhou China Mobile", "latencyMs": 42, "status": "ok"},
			map[string]any{"name": "高通中国", "type": "domestic", "ip": "39.180.138.19", "countryCode": "cn", "geoZh": "中国 浙江 杭州 移动", "geoEn": "China Zhejiang Hangzhou China Mobile", "latencyMs": 44, "status": "ok"},
			map[string]any{"name": "discord.com", "type": "international", "category": "social", "ip": "54.65.230.180", "countryCode": "jp", "geoZh": "日本 Tokyo Amazon.com", "geoEn": "Japan Tokyo Amazon.com", "latencyMs": 112, "status": "ok"},
			map[string]any{"name": "x.com", "type": "international", "category": "social", "ip": "13.158.209.152", "countryCode": "jp", "geoZh": "日本 Tokyo Amazon.com", "geoEn": "Japan Tokyo Amazon.com", "latencyMs": 92, "status": "ok"},
			map[string]any{"name": "anthropic.com", "type": "international", "category": "ai", "ip": "54.65.230.180", "countryCode": "jp", "geoZh": "日本 Tokyo Amazon.com", "geoEn": "Japan Tokyo Amazon.com", "latencyMs": 108, "status": "ok"},
			map[string]any{"name": "claude.ai", "type": "international", "category": "ai", "ip": "13.158.209.152", "countryCode": "jp", "geoZh": "日本 Tokyo Amazon.com", "geoEn": "Japan Tokyo Amazon.com", "latencyMs": 87, "status": "ok"},
			map[string]any{"name": "chatgpt.com", "type": "international", "category": "ai", "ip": "13.158.209.152", "countryCode": "jp", "geoZh": "日本 Tokyo Amazon.com", "geoEn": "Japan Tokyo Amazon.com", "latencyMs": 91, "status": "ok"},
			map[string]any{"name": "openai.com", "type": "international", "category": "ai", "ip": "13.158.209.152", "countryCode": "jp", "geoZh": "日本 Tokyo Amazon.com", "geoEn": "Japan Tokyo Amazon.com", "latencyMs": 94, "status": "ok"},
		},
	}
}

func newIPBentoTestApp(t *testing.T) *app {
	t.Helper()
	a := newTestApp(t)
	t.Cleanup(a.closeBarInstances)
	a.netProbeFn = func(_ context.Context, _ engine.NetProbeConfig, _ bool) map[string]any {
		return mockIPNetCoffeeReport()
	}
	return a
}

func TestIPDashboardBentoHomeDarkLightAndMedium(t *testing.T) {
	t.Cleanup(func() { setThemeMode(themeSystem); setSkin(skinDefault); setLang(langZH) })
	a := newIPBentoTestApp(t)

	// Put ip-dashboard first on Home so the wide Bento card is front and center.
	for _, sc := range a.store.Scripts() {
		sc.Home = sc.ID == "ip-dashboard" || sc.ID == "weight" || sc.ID == "fx" || sc.ID == "weather"
		_ = a.store.SaveScript(sc)
	}
	a.set.CardOrder = []string{"ip-dashboard", "weight", "fx", "weather"}
	_ = a.store.SetSettings(a.set)

	setThemeMode(themeDark)
	tt := ui.NewTester(a.view, 1280, 760)
	settle(tt)
	settle(tt)
	requireText(t, tt, "39.180.138.19")
	requireText(t, tt, "中国 浙江 杭州 余杭 移动")
	requireText(t, tt, "家庭宽带 (ISP)")
	requireText(t, tt, "网络连通性")
	requireText(t, tt, "54.65.230.180")
	requireText(t, tt, "13.158.209.152")
	requireText(t, tt, "19ms")
	requireText(t, tt, "超时")
	savePNG(t, tt, "33-ip-bento-home-dark")

	setThemeMode(themeLight)
	settle(tt)
	settle(tt)
	savePNG(t, tt, "33-ip-bento-home-light")

	// Medium card compact Bento layout.
	setThemeMode(themeDark)
	a.setSize("ip-dashboard", 1)
	a.invalidate("ip-dashboard")
	settle(tt)
	settle(tt)
	requireText(t, tt, "39.180.138.19")
	requireText(t, tt, "19ms")
	savePNG(t, tt, "33-ip-bento-medium")
}

func TestIPDashboardFullViewInteractionsAndModes(t *testing.T) {
	t.Cleanup(func() { setThemeMode(themeSystem); setSkin(skinDefault); setLang(langZH) })
	a := newIPBentoTestApp(t)

	var copied string
	oldCopy := copyText
	copyText = func(s string) { copied = s }
	defer func() { copyText = oldCopy }()

	setThemeMode(themeDark)
	tt := ui.NewTester(a.view, 1280, 920)
	a.router.Push("/full/ip-dashboard")
	settle(tt)
	settle(tt)

	requireText(t, tt, "隐藏 IP")
	requireText(t, tt, "网站分流测试")
	requireText(t, tt, "claude.ai")
	absent(t, tt, "大图", "悬浮图表查看详情")

	// Click an exit IP pill -> opens inline IP Quality Inspector.
	click(t, tt, "54.65.230.180")
	requireText(t, tt, "AS16509 · Amazon.com, Inc.")
	requireText(t, tt, "54.65.0.0/16")
	savePNG(t, tt, "33-ip-bento-full-dark")

	// Toggle Hide IP -> masks all displayed IPs while keeping Copy IP copying real IP.
	click(t, tt, "隐藏 IP")
	requireText(t, tt, "39.180.*.*")
	requireText(t, tt, "54.65.*.*")
	requireText(t, tt, "13.158.*.*")

	click(t, tt, "复制 IP")
	if copied != "54.65.230.180" {
		t.Fatalf("copied IP = %q, want real unmasked 54.65.230.180", copied)
	}
	requireText(t, tt, "已复制：54.65.230.180")

	// Expand probe latency breakdown via "查看更多 ›".
	click(t, tt, "查看更多 ›")
	requireText(t, tt, "连通性探针明细")
	requireText(t, tt, "成功 12/12")

	// Switch to Data Table and Raw Data modes.
	click(t, tt, "数据表")
	requireText(t, tt, "claude.ai")
	click(t, tt, "原始数据")
	requireText(t, tt, "Bento 看板 (bento)")
	click(t, tt, "展开")
	requireText(t, tt, "网络探测")
}

func TestIPDashboardEnglishZeroLeakedChinese(t *testing.T) {
	t.Cleanup(func() { setThemeMode(themeSystem); setSkin(skinDefault); setLang(langZH) })
	a := newIPBentoTestApp(t)
	a.set.Lang = langEN
	setLang(langEN)

	for _, sc := range a.store.Scripts() {
		sc.Home = sc.ID == "ip-dashboard"
		_ = a.store.SaveScript(sc)
	}

	tt := ui.NewTester(a.view, 1280, 860)
	settle(tt)
	settle(tt)
	requireText(t, tt, "39.180.138.19")
	requireText(t, tt, "Network Connectivity")
	requireText(t, tt, "Timeout")
	onlyEnglish(t, tt, "ip-dashboard-home-en")

	a.router.Push("/full/ip-dashboard")
	settle(tt)
	settle(tt)
	click(t, tt, "54.65.230.180")
	requireText(t, tt, "Website Split-Routing Test")
	requireText(t, tt, "Copy IP")
	onlyEnglish(t, tt, "ip-dashboard-full-en")
}

func TestVisualEditorBentoTemplate(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 1240, 1080)
	click(t, tt, "组件管理")
	click(t, tt, "+ 脚本")
	requireText(t, tt, "Bento 看板")
	click(t, tt, "Bento 看板")
	if !strings.Contains(a.draft.Source, `"type": "bento"`) {
		t.Fatalf("bento template missing type=bento:\n%s", a.draft.Source)
	}
	requireText(t, tt, "Bento 看板配置")
	requireText(t, tt, "203.0.113.10")
	absent(t, tt, "syntax error", "unexpected")
}
