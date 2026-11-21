package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/feed"
)

// feedForm is the form for adding a data source from a curl command, or for
// changing one that exists.
type feedForm struct {
	name, curl, rule, names, id string
	fallbacks                   string // one request per paragraph
	kind                        int    // 0 JSON path, 1 regular expression
	every                       int    // index into feedEvery
	agg                         int    // index into feedAggs
	note                        string

	open    bool   // the form is shown
	editing string // id of the source being changed; "" when adding
	confirm string // id of the source whose delete waits for a second click
	testing bool   // a test request is out
}

var (
	feedEvery    = []int{60, 300, 3600}
	feedEveryTxt = []string{"1 分钟", "5 分钟", "1 小时"}
	feedAggs     = []string{"last", "avg", "max", "min"}
	feedAggTxt   = []string{"最新", "平均", "最大", "最小"}
)

// feedInput gathers, for each source a script names, its latest values and its
// samples grouped by hour (the last three days) and by day (the last month).
// It only reads the store, so it is safe to call from any goroutine.
func (a *app) feedInput(names []string, now time.Time) map[string]engine.FeedData {
	out := make(map[string]engine.FeedData, len(names))
	for _, id := range names {
		src, ok := a.store.Feed(id)
		if !ok {
			out[id] = engine.FeedData{Error: tr("没有数据源 ") + id + tr("，请到「设置」→「数据源」里添加")}
			continue
		}
		fd := engine.FeedData{Name: src.Name, Fails: a.runner.Health(id).Fails}
		samples, err := a.samples.Read(id, now.AddDate(0, 0, -30))
		if err != nil {
			fd.Error = err.Error()
		}
		numbers := 0
		if len(samples) > 0 {
			last := samples[len(samples)-1]
			fd.Latest = last.Values
			fd.LatestAt = last.T
			numbers = len(last.Values)
		}
		for i := 0; i < max(numbers, 1); i++ {
			fd.Names = append(fd.Names, src.NumberName(i))
		}
		hours := feed.Aggregate(samples, feed.Hour, src.Agg, now.Location(), now.Add(-72*time.Hour), now)
		days := feed.Aggregate(samples, feed.Day, src.Agg, now.Location(), now.AddDate(0, 0, -30), now)
		fd.Hour = bucketsFor(hours, fd.Names)
		fd.Day = bucketsFor(days, fd.Names)
		if e := a.runner.LastError(id); e != "" && fd.Error == "" {
			fd.Error = e
		}
		fd.Stale = a.runner.Stale(src, fd.LatestAt)
		out[id] = fd
	}
	return out
}

// bucketsFor converts aggregated buckets for a script. A bucket where a number
// was never seen is carried forward from the bucket before, since JSON has no
// NaN.
func bucketsFor(b feed.Buckets, names []string) engine.FeedBuckets {
	out := engine.FeedBuckets{Labels: b.Labels, Counts: b.Counts}
	for k, series := range b.Series {
		name := fmt.Sprintf(tr("值%d"), k+1)
		if k < len(names) {
			name = names[k]
		}
		pts := make([]float64, len(series))
		prev := 0.0
		for i, v := range series {
			if math.IsNaN(v) {
				v = prev
			}
			pts[i] = v
			prev = v
		}
		out.Series = append(out.Series, engine.Series{Name: name, Points: pts})
	}
	return out
}

// pollInBackground takes one reading of a source now. The result is shown on
// the page when it comes back.
func (a *app) pollInBackground(src feed.Source) {
	if a.set.Offline {
		a.feedForm.note = tr("离线模式下不请求网络，关闭离线模式后再获取")
		return
	}
	run := func() {
		smp, err := a.runner.Poll(context.Background(), src)
		a.post(func() {
			if err != nil {
				a.feedForm.note = src.Name + "：" + trErr(err)
			} else {
				a.feedForm.note = fmt.Sprintf(tr("%s：已获取 %v"), src.Name, smp.Values)
			}
			a.refreshFeed(src.ID)
		})
	}
	if a.win == nil {
		run()
		return
	}
	go run()
}

// refreshFeed drops the runs of every script that reads source id, so that
// their cards and pages show the new samples.
func (a *app) refreshFeed(id string) {
	for _, sc := range a.store.Scripts() {
		meta, _ := a.metaOf(sc)
		for _, f := range meta.Feeds {
			if f == id {
				a.invalidate(sc.ID)
			}
		}
	}
}

// feedLoop polls the due sources every few seconds while the window is open.
// Polling runs only while the app runs; the platform decides about background
// work.
func (a *app) feedLoop() {
	for range time.Tick(5 * time.Second) {
		if a.set.Offline {
			continue
		}
		// Several at a time: a slow endpoint must not hold up the rest.
		a.runner.PollDue(context.Background(), a.store.Feeds(), 4, func(src feed.Source, _ feed.Sample, _ error) {
			id := src.ID
			a.post(func() { a.refreshFeed(id) })
		})
	}
}

// formSource builds the source the form describes. When a source is being
// changed it starts from the stored one, so that what the form does not show
// (timeout, retries) is kept.
func (a *app) formSource(test bool) (feed.Source, error) {
	f := &a.feedForm
	req, err := feed.ParseRequest(f.curl)
	if err != nil {
		return feed.Source{}, errors.New(trErr(err))
	}
	var backups []feed.Request
	for _, para := range strings.Split(strings.ReplaceAll(f.fallbacks, "\r\n", "\n"), "\n\n") {
		if strings.TrimSpace(para) == "" {
			continue
		}
		b, err := feed.ParseRequest(para)
		if err != nil {
			return feed.Source{}, errors.New(tr("备用请求：") + trErr(err))
		}
		backups = append(backups, b)
	}
	name := strings.TrimSpace(f.name)
	if name == "" {
		name = tr("数据源")
	}
	id := strings.TrimSpace(f.id)
	if f.editing != "" {
		id = f.editing
	} else if id == "" {
		id = slugOf(name)
	}
	if !feed.ValidID(id) {
		return feed.Source{}, errors.New(tr("id 只能用小写字母、数字和连字符，2 到 40 个字符"))
	}
	if f.editing == "" && !test {
		if _, exists := a.store.Feed(id); exists {
			return feed.Source{}, errors.New(tr("已经有 id 为 ") + id + tr(" 的数据源了"))
		}
	}
	ex := feed.Extract{Kind: feed.KindJSON, Path: strings.TrimSpace(f.rule)}
	if f.kind == 1 {
		ex = feed.Extract{Kind: feed.KindRegex, Pattern: strings.TrimSpace(f.rule)}
	}
	var names []string
	for _, n := range strings.Split(f.names, ",") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	src := feed.Source{}
	if old, ok := a.store.Feed(f.editing); ok && f.editing != "" {
		src = old
	}
	src.ID, src.Name, src.Request, src.Extract = id, name, req, ex
	src.Names, src.Every, src.Agg, src.Fallbacks = names, feedEvery[f.every], feedAggs[f.agg], backups
	if err := src.Validate(); err != nil {
		return feed.Source{}, errors.New(trErr(err))
	}
	return src, nil
}

// addFeed stores the data source the form describes (a new one, or the changes
// to the one being edited), closes the form and takes a first reading.
func (a *app) addFeed() {
	f := &a.feedForm
	src, err := a.formSource(false)
	if err != nil {
		f.note = err.Error()
		return
	}
	if err := a.store.SaveFeed(src); err != nil {
		f.note = err.Error()
		return
	}
	if f.editing != "" {
		f.note = tr("已保存 ") + src.Name
		a.runner.Forget(src.ID)
	} else {
		f.note = tr("已添加 ") + src.Name + "（id：" + src.ID + tr("），正在获取第一次数据…")
	}
	a.closeFeedForm()
	a.refreshFeed(src.ID)
	a.pollInBackground(src)
}

// closeFeedForm hides the form and clears it; the note stays.
func (a *app) closeFeedForm() {
	f := &a.feedForm
	note := f.note
	*f = feedForm{note: note}
}

// editFeed fills the form with a stored source.
func (a *app) editFeed(src feed.Source) {
	f := &a.feedForm
	*f = feedForm{open: true, editing: src.ID, id: src.ID, name: src.Name}
	f.curl = requestText(src.Request)
	var backups []string
	for _, r := range src.Fallbacks {
		backups = append(backups, requestText(r))
	}
	f.fallbacks = strings.Join(backups, "\n\n")
	if src.Extract.Kind == feed.KindRegex {
		f.kind, f.rule = 1, src.Extract.Pattern
	} else {
		f.rule = src.Extract.Path
	}
	f.names = strings.Join(src.Names, ",")
	for i, e := range feedEvery {
		if e == src.Every {
			f.every = i
		}
	}
	for i, g := range feedAggs {
		if g == src.Agg {
			f.agg = i
		}
	}
}

// testFeed makes the requests of the form once, without storing anything, and
// says which numbers the rule reads from the first one that answers.
func (a *app) testFeed() {
	f := &a.feedForm
	if a.set.Offline {
		f.note = tr("离线模式下不请求网络，关闭离线模式后再获取")
		return
	}
	src, err := a.formSource(true)
	if err != nil {
		f.note = err.Error()
		return
	}
	if f.testing {
		return
	}
	f.testing = true
	run := func() {
		note := a.tryFeed(src)
		a.post(func() {
			a.feedForm.testing = false
			a.feedForm.note = note
		})
	}
	if a.win == nil {
		run()
		return
	}
	go run()
}

// tryFeed fetches each request of src in turn, and describes what the first
// that gives numbers reads.
func (a *app) tryFeed(src feed.Source) string {
	var last error
	for i, req := range src.Requests() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		body, err := a.runner.Fetcher.Fetch(ctx, req)
		cancel()
		if err == nil {
			var nums []float64
			if nums, err = src.Extract.Numbers(body); err == nil {
				var parts []string
				for k, v := range nums {
					parts = append(parts, fmt.Sprintf("%s = %g", src.NumberName(k), v))
				}
				from := tr("主请求")
				if i > 0 {
					from = fmt.Sprintf(tr("备用请求 %d"), i)
				}
				return tr("测试通过（") + from + tr("）：") + strings.Join(parts, tr("，"))
			}
		}
		last = err
	}
	return tr("测试失败：") + trErr(last)
}

// requestText writes a request as the text the form takes: its address when
// that says it all, else a curl command.
func requestText(r feed.Request) string {
	scheme := strings.ToLower(strings.SplitN(r.URL, ":", 2)[0])
	method := strings.ToUpper(r.Method)
	plain := (method == "" || method == "GET") && len(r.Headers) == 0 && len(r.Cookies) == 0 && r.Body == ""
	if (scheme != "http" && scheme != "https") || plain {
		return r.URL
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
	parts := []string{"curl", quote(r.URL)}
	if method != "" && method != "GET" {
		parts = append(parts, "-X", method)
	}
	keys := make([]string, 0, len(r.Headers))
	for k := range r.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, "-H", quote(k+": "+r.Headers[k]))
	}
	if len(r.Cookies) > 0 {
		keys = keys[:0]
		for k := range r.Cookies {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var cs []string
		for _, k := range keys {
			cs = append(cs, k+"="+r.Cookies[k])
		}
		parts = append(parts, "-b", quote(strings.Join(cs, "; ")))
	}
	if r.Body != "" {
		parts = append(parts, "-d", quote(r.Body))
	}
	return strings.Join(parts, " ")
}

// deleteFeed removes a source. Scripts that read it show the missing source.
func (a *app) deleteFeed(id string) {
	if err := a.store.DeleteFeed(id); err != nil {
		a.feedForm.note = err.Error()
		return
	}
	a.runner.Forget(id)
	a.refreshFeed(id)
	a.feedForm.note = tr("已删除 ") + id
	a.feedForm.confirm = ""
	if a.feedForm.editing == id {
		a.closeFeedForm()
	}
}

// slugOf turns a name into an ID: lowercase letters, digits and hyphens.
func slugOf(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteRune('-')
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) < 2 {
		return fmt.Sprintf("feed-%d", time.Now().Unix()%100000)
	}
	if len(s) > 40 {
		s = strings.Trim(s[:40], "-")
	}
	return s
}

// feedsPage lists the data sources, with their latest values and errors, and
// the form that adds one.
func (a *app) feedsPage(c *ui.Context, compact bool) {
	p := pal(c)
	page(c, compact, func() {
		subheader(c, a, tr("数据源"), tr("从 curl、tcp、文件等取数，按小时或按天画曲线"), nil)
		hint(c, tr("每隔一段时间请求一次，把响应里的数字存下来。请求在本机发出，脚本拿不到 cookie 和网络。"))
		if a.set.Offline {
			chip(c, tr("现在是离线模式，不会自动获取"), p.warn, p.warn.Alpha(0.14))
		}

		srcs := a.store.Feeds()
		if len(srcs) == 0 {
			card(c).Children(func() {
				empty(c, "activity", tr("还没有数据源"), tr("点下面的「添加数据源」，粘贴一条 curl 命令。"))
			})
		}
		for _, src := range srcs {
			a.feedCard(c, src)
		}

		if a.feedForm.note != "" {
			chip(c, a.feedForm.note, p.accent, p.accentSoft)
		}
		if a.feedForm.open {
			a.feedAddForm(c, compact)
		} else {
			ui.Row(c).Children(func() {
				if primaryBtn(c, tr("+ 添加数据源")).Clicked() {
					a.feedForm = feedForm{open: true, note: a.feedForm.note}
				}
			})
		}
	})
}

func (a *app) feedCard(c *ui.Context, src feed.Source) {
	p := pal(c)
	card(c).Padding(16).Gap(8).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(10).Wrap().Children(func() {
			iconBadge(c, "activity", p.accent, 34)
			ui.Column(c).Children(func() {
				ui.Text(c, src.Name).Bold()
				ui.Text(c, "id "+src.ID+tr(" · 每 ")+everyText(src.Every)+" · "+aggText(src.Agg)).FontSize(fsTiny).TextColor(p.muted)
			})
			ui.Spacer(c)
			if btn(c, tr("立即获取")).Clicked() {
				a.pollInBackground(src)
			}
			if btn(c, tr("编辑")).Clicked() {
				a.editFeed(src)
			}
			label := tr("删除")
			if a.feedForm.confirm == src.ID {
				label = tr("确认删除")
			}
			if btn(c, label).Clicked() {
				if a.feedForm.confirm == src.ID {
					a.deleteFeed(src.ID)
				} else {
					a.feedForm.confirm = src.ID
				}
			}
		})
		if latest, ok, _ := a.samples.Latest(src.ID); ok {
			ui.Row(c).Gap(18).Wrap().Children(func() {
				for i, v := range latest.Values {
					ui.Column(c).Children(func() {
						ui.Text(c, src.NumberName(i)).FontSize(fsTiny).TextColor(p.muted)
						ui.Text(c, fmt.Sprintf("%g", v)).FontSize(22).Bold()
					})
				}
			})
			hint(c, tr("更新于 ")+fmtTime(latest.T))
		} else {
			hint(c, tr("还没有数据"))
		}
		a.feedHealth(c, src)
	})
}

func (a *app) feedAddForm(c *ui.Context, compact bool) {
	f := &a.feedForm
	w := float32(380)
	if compact {
		w = 300
	}
	title := tr("添加数据源")
	if f.editing != "" {
		title = tr("编辑数据源")
	}
	sectionTitle(c, title)
	card(c).Padding(18).Gap(14).Children(func() {
		field(c, tr("名称"), "", func() {
			textInput(c, &f.name, tr("例如 金价实时")).Width(w)
		})
		field(c, tr("请求"), tr("curl 命令（可带 -b 'cookie' 和 -H，直接从浏览器开发者工具复制），或 https://、tcp://、file://、exec: 地址"), func() {
			textArea(c, &f.curl, "curl 'https://…' -b 'session=…'", 5).Font("monospace").FontSize(fsSmall)
		})
		field(c, tr("备用请求（可选）"), tr("主请求失败，或取不到数字时依次尝试；多个之间空一行"), func() {
			textArea(c, &f.fallbacks, "curl 'https://backup…'", 3).Font("monospace").FontSize(fsSmall)
		})
		field(c, tr("取数方式"), "", func() {
			segmented(c, &f.kind, tr("JSON 路径"), tr("正则"))
		})
		if f.kind == 0 {
			field(c, tr("JSON 路径"), tr("用 . 连接字段；# 表示数组的每一项，如 data.items.#.v"), func() {
				textInput(c, &f.rule, "data.price").Width(w)
			})
		} else {
			field(c, tr("正则"), tr("第一个分组就是数字，如 价格 ([0-9.]+)"), func() {
				textInput(c, &f.rule, tr("价格 ([0-9.]+)")).Width(w)
			})
		}
		ui.Row(c).Gap(gapSection).Wrap().Children(func() {
			field(c, tr("间隔"), "", func() { segmented(c, &f.every, trAll(feedEveryTxt)...) })
			field(c, tr("汇总"), "", func() { segmented(c, &f.agg, trAll(feedAggTxt)...) })
		})
		field(c, tr("数字的名字"), tr("可选，逗号分隔，如 金价,成交量"), func() {
			textInput(c, &f.names, tr("金价,成交量")).Width(w)
		})
		if f.editing != "" {
			field(c, "id", tr("id 创建后不能改，脚本用它在 meta.feeds 里引用"), func() {
				ui.Text(c, f.editing).Font("monospace").FontSize(fsSmall)
			})
		} else {
			field(c, "id", tr("可选，留空自动生成；脚本用它在 meta.feeds 里引用"), func() {
				textInput(c, &f.id, "gold-live").Width(w)
			})
		}
		ui.Row(c).Gap(8).Wrap().Children(func() {
			okLabel := tr("添加并获取一次")
			if f.editing != "" {
				okLabel = tr("保存")
			}
			if primaryBtn(c, okLabel).Clicked() {
				a.addFeed()
			}
			testLabel := tr("测试请求")
			if f.testing {
				testLabel = tr("测试中…")
			}
			if btn(c, testLabel).Clicked() {
				a.testFeed()
			}
			if btn(c, tr("取消")).Clicked() {
				note := f.note
				a.closeFeedForm()
				a.feedForm.note = note
			}
		})
		hint(c, tr("「测试请求」只发一次请求并显示取到的数字，不保存。"))
	})
}

func everyText(sec int) string {
	switch {
	case sec >= 3600 && sec%3600 == 0:
		return fmt.Sprintf(tr("%d 小时"), sec/3600)
	case sec >= 60 && sec%60 == 0:
		return fmt.Sprintf(tr("%d 分钟"), sec/60)
	}
	return fmt.Sprintf(tr("%d 秒"), sec)
}

func aggText(agg string) string {
	for i, a := range feedAggs {
		if a == agg {
			return tr("汇总：") + tr(feedAggTxt[i])
		}
	}
	return tr("汇总：最新")
}

// feedHealth shows how a source is doing: its state, the number of failures in
// a row, when it is asked next, and the last error.
func (a *app) feedHealth(c *ui.Context, src feed.Source) {
	p := pal(c)
	h := a.runner.Health(src.ID)
	var text string
	fg := p.muted
	switch h.State {
	case feed.OK:
		text, fg = tr("正常"), p.down
		if h.Used > 0 {
			text = fmt.Sprintf(tr("正常 · 由备用请求 %d 提供"), h.Used)
			fg = p.warn
		}
	case feed.Retrying:
		text, fg = fmt.Sprintf(tr("重试中 · 连续失败 %d 次"), h.Fails), p.warn
	case feed.Open:
		text, fg = fmt.Sprintf(tr("已暂停 · 连续失败 %d 次，%s 再试"), h.Fails, fmtTime(h.Next.UnixMilli())), p.up
	default:
		return
	}
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		chip(c, text, fg, fg.Alpha(0.13))
		chip(c, protocolLabel(src), p.muted, p.chip)
		if n := len(src.Fallbacks); n > 0 {
			chip(c, fmt.Sprintf(tr("%d 个备用"), n), p.muted, p.chip)
		}
	})
	if h.Err != "" {
		ui.Text(c, tr("上一次失败：")+trMsg(h.Err)).FontSize(fsSmall).TextColor(p.up)
	}
}

// protocolLabel names the protocols a source uses: "https", or "https + tcp"
// when a fallback uses another.
func protocolLabel(src feed.Source) string {
	seen := []string{}
	for _, r := range src.Requests() {
		s := strings.SplitN(r.URL, ":", 2)[0]
		dup := false
		for _, x := range seen {
			dup = dup || x == s
		}
		if !dup {
			seen = append(seen, s)
		}
	}
	return strings.Join(seen, " + ")
}
