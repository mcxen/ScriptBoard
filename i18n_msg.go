package main

import (
	"regexp"
	"strings"
)

// Error messages come from the packages under internal/, which write them in
// Chinese. trMsg gives their English: an exact message from msgEN, or one of
// the patterns in msgRules (for messages with a value in them), or, for a
// message that wraps another ("请求失败：…"), each part on its own. A message
// that matches nothing is shown as it is.

var msgEN = map[string]string{
	"地址必须是完整的 http 或 https 地址":         "The address must be a full http or https address",
	"数据源 id 只能用小写字母、数字和连字符，2 到 40 个字符": "A source id is 2–40 lowercase letters, digits or hyphens",
	"JSON 路径不能为空，例如 data.price":        "The JSON path cannot be empty, e.g. data.price",
	"取数方式只能是 json 或 regex":             "The extract kind is json or regex",
	"数据源需要一个名称":                        "A data source needs a name",
	"超时只能在 1 到 120 秒之间":                "The timeout is 1 to 120 seconds",
	"轮询间隔至少 1 分钟":                      "The interval is at least 1 minute",
	"汇总方式只能是 last、avg、max 或 min":       "The summary is last, avg, max or min",
	"没有 curl 命令":                       "No curl command",
	"没有请求":                             "No request",
	"命令里没有地址":                          "No address in the command",
	"单引号没有配对":                          "Unmatched single quote",
	"双引号没有配对":                          "Unmatched double quote",
	"不支持从文件读取 cookie，请把 cookie 直接写在命令里，例如 -b 'name=value'": "Reading cookies from a file is not supported; write them in the command, e.g. -b 'name=value'",
	"不支持从文件读取请求体，请把内容直接写在命令里":                              "Reading the body from a file is not supported; write it in the command",
	"正则没有匹配到数字":               "The regex matched no number",
	"未知的取数方式":                 "Unknown extract kind",
	"这个协议没有开启":                "This protocol is not turned on",
	"文件地址应写成 file:///绝对路径":    "A file address looks like file:///absolute/path",
	"TCP 地址应写成 tcp://主机:端口":   "A TCP address looks like tcp://host:port",
	"exec 地址应写成 exec:程序 参数…":  "An exec address looks like exec:program args…",
	"这个数据源正在获取":               "This source is being fetched already",
	"exec 会运行本机程序，请先在「设置」里允许": "exec runs a program on this computer; allow it in Settings first",
}

var msgRules = []struct {
	re *regexp.Regexp
	en string
}{
	{regexp.MustCompile(`^不支持的协议 "(.*)"，可用：(.*)$`), `Unsupported protocol "$1"; available: $2`},
	{regexp.MustCompile(`^不支持的协议 "(.*)"$`), `Unsupported protocol "$1"`},
	{regexp.MustCompile(`^不支持的请求方法 (.*)$`), `Unsupported request method $1`},
	{regexp.MustCompile(`^参数 (.*) 后面缺少内容$`), `Nothing follows the option $1`},
	{regexp.MustCompile(`^请求头格式不对：(.*)$`), `Bad header: $1`},
	{regexp.MustCompile(`^不认识的参数 (.*)，这个 curl 命令暂时不能用$`), `Unknown option $1; this curl command cannot be used yet`},
	{regexp.MustCompile(`^多余的内容：(.*)$`), `Unexpected: $1`},
	{regexp.MustCompile(`^路径 (.*) 里没有数字$`), `No number at the path $1`},
	{regexp.MustCompile(`^需要 curl 命令，或以 (.*) 开头的地址$`), `Needs a curl command, or an address starting with $1`},
	{regexp.MustCompile(`^数据源 id 不对：(.*)$`), `Bad source id: $1`},
}

var fallbackRule = regexp.MustCompile(`^备用请求 (\d+)：(.*)$`)

// trMsg translates an error message from an internal package.
func trMsg(s string) string {
	if curLang() != langEN {
		return s
	}
	if m := fallbackRule.FindStringSubmatch(s); m != nil {
		return "Fallback " + m[1] + ": " + trMsg(m[2])
	}
	if e, ok := msgEN[s]; ok {
		return e
	}
	if e, ok := enText[s]; ok {
		return e
	}
	for _, r := range msgRules {
		if r.re.MatchString(s) {
			return r.re.ReplaceAllString(s, r.en)
		}
	}
	// A message that wraps another: "head：rest" or "head：rest".
	if i := strings.Index(s, "："); i > 0 {
		head, rest := s[:i+len("：")], s[i+len("："):]
		if h, ok := wrapEN[head]; ok {
			return h + trMsg(rest)
		}
	}
	return s
}

var wrapEN = map[string]string{
	"请求失败：":      "Request failed: ",
	"响应不是 JSON：": "The response is not JSON: ",
	"正则表达式不对：":   "Bad regular expression: ",
	"命令失败：":      "Command failed: ",
}

// trErr is trMsg of an error's message.
func trErr(err error) string {
	if err == nil {
		return ""
	}
	return trMsg(err.Error())
}
