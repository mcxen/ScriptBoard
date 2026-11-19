// Package iconcat is the catalog of the icons that ship with the app, by
// category, so that a script's icon, the editor and an assistant (over MCP)
// pick from one list. The SVG of each name is in the app's icons folder; a
// test there checks that every name in the catalog has one.
package iconcat

// Item is one icon: its name, as a script writes it, and its labels.
type Item struct {
	Name string
	Zh   string
	En   string
}

// Group is a category of icons.
type Group struct {
	Key   string
	Zh    string
	En    string
	Items []Item
}

// Categories are the shipped icons, grouped; a script uses one name from here
// for meta().icon.
var Categories = []Group{
	{Key: "weather", Zh: "天气", En: "Weather", Items: []Item{
		{"sun", "晴", "Sun"},
		{"cloud-sun", "多云", "Cloudy"},
		{"cloud-rain", "雨", "Rain"},
		{"wind", "风", "Wind"},
		{"thermometer", "气温", "Temperature"},
		{"zap", "雷雨", "Storm"},
	}},
	{Key: "finance", Zh: "财经", En: "Finance", Items: []Item{
		{"dollar-sign", "汇率", "Exchange rate"},
		{"circle-dollar-sign", "币价", "Crypto"},
		{"coins", "金币", "Coins"},
		{"wallet", "钱包", "Wallet"},
		{"hand-coins", "积存", "Savings"},
		{"gem", "金价", "Gold"},
		{"trending-up", "上涨", "Up"},
		{"trending-down", "下跌", "Down"},
		{"candlestick-chart", "K 线", "Candles"},
		{"gauge", "额度", "Quota"},
	}},
	{Key: "charts", Zh: "图表", En: "Charts", Items: []Item{
		{"chart-line", "曲线", "Line"},
		{"chart-column", "柱状", "Columns"},
		{"chart-bar-big", "大柱状", "Big bars"},
		{"chart-scatter", "散点", "Scatter"},
		{"activity", "活动", "Activity"},
		{"file-spreadsheet", "表格", "Spreadsheet"},
	}},
	{Key: "time", Zh: "时间", En: "Time", Items: []Item{
		{"calendar-days", "日历", "Calendar"},
		{"bell", "提醒", "Reminder"},
	}},
	{Key: "body", Zh: "身体与生活", En: "Body and life", Items: []Item{
		{"scale", "体重", "Weight"},
		{"thermometer", "体温", "Temperature"},
		{"activity", "心率", "Heart"},
	}},
	{Key: "ai", Zh: "智能", En: "AI", Items: []Item{
		{"bot", "助手", "Assistant"},
		{"sparkles", "用量", "Usage"},
		{"zap", "速度", "Speed"},
	}},
	{Key: "data", Zh: "数据与网络", En: "Data and network", Items: []Item{
		{"globe", "网络", "Web"},
		{"download", "下载", "Download"},
		{"upload", "上传", "Upload"},
		{"refresh-cw", "刷新", "Refresh"},
		{"rotate-ccw", "重置", "Reset"},
		{"users", "社区", "Community"},
		{"user-round", "账号", "Account"},
		{"log-in", "登录", "Sign in"},
		{"log-out", "退出", "Sign out"},
	}},
	{Key: "interface", Zh: "界面", En: "Interface", Items: []Item{
		{"house", "首页", "Home"},
		{"layout-grid", "总览", "Overview"},
		{"panel-left", "侧栏", "Sidebar"},
		{"settings", "设置", "Settings"},
		{"pencil-line", "编辑", "Edit"},
		{"copy", "复制", "Copy"},
		{"plus", "添加", "Add"},
		{"minus", "减", "Minus"},
		{"x", "关闭", "Close"},
		{"chevron-right", "展开", "Expand"},
		{"info", "说明", "Info"},
		{"square", "方块", "Square"},
		{"maximize-2", "大小", "Size"},
	}},
}

// Has reports whether name is in the catalog.
func Has(name string) bool {
	for _, g := range Categories {
		for _, it := range g.Items {
			if it.Name == name {
				return true
			}
		}
	}
	return false
}

// Names is every icon name in the catalog, in order, each once.
func Names() []string {
	var out []string
	seen := map[string]bool{}
	for _, g := range Categories {
		for _, it := range g.Items {
			if !seen[it.Name] {
				seen[it.Name] = true
				out = append(out, it.Name)
			}
		}
	}
	return out
}
