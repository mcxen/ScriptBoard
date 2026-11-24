package main

import (
	"context"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/community"
	"scriptboard/src/internal/store"
)

// commState is what the community page shows. Requests run off the main
// thread; their results come back through post.
type commState struct {
	loaded  bool // the list has been requested for the current address
	loading bool
	items   []community.Script
	err     string
	busy    bool   // a login, upload or download is running
	note    string // one line of feedback
}

// communityClient returns the community client for the address in the settings.
func (a *app) communityClient() *community.Client {
	return community.New(a.set.CommunityURL)
}

// loadCommunity requests the list of approved scripts once per address.
func (a *app) loadCommunity() {
	cl := a.communityClient()
	if !cl.Configured() || a.comm.loading || a.comm.loaded {
		return
	}
	a.comm.loading = true
	a.comm.loaded = true
	run := func() {
		items, err := cl.List(context.Background(), 50)
		a.post(func() {
			a.comm.loading = false
			a.comm.items = items
			a.comm.err = ""
			if err != nil {
				a.comm.err = err.Error()
			}
		})
	}
	if a.win == nil {
		run()
		return
	}
	go run()
}

// background runs fn off the main thread when there is a window, and inline
// in tests, then posts its result to the main thread with done.
func (a *app) background(fn func() (string, error), done func(string, error)) {
	run := func() {
		msg, err := fn()
		a.post(func() { done(msg, err) })
	}
	if a.win == nil {
		run()
		return
	}
	go run()
}

// authenticate logs in or registers with the form values. The token is kept
// in the settings on this computer.
func (a *app) authenticate(register bool) {
	cl := a.communityClient()
	if !cl.Configured() {
		a.comm.note = tr("先在「设置」里填写社区地址")
		return
	}
	user, pass := a.authUser, a.authPass
	a.comm.busy = true
	a.background(func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		var token string
		var err error
		if register {
			token, err = cl.Register(ctx, user, pass)
		} else {
			token, err = cl.Login(ctx, user, pass)
		}
		if err != nil {
			return "", err
		}
		return token, nil
	}, func(token string, err error) {
		a.comm.busy = false
		if err != nil {
			a.comm.note = err.Error()
			return
		}
		a.set.Username = user
		a.set.Token = token
		a.authPass = ""
		if err := a.store.SetSettings(a.set); err != nil {
			a.comm.note = tr("登录成功，但没能保存：") + err.Error()
			return
		}
		a.comm.note = tr("已登录：") + user
	})
}

// logout forgets the token on this computer and ends it on the community.
func (a *app) logout() {
	cl := a.communityClient()
	token := a.set.Token
	a.set.Username, a.set.Token = "", ""
	if err := a.store.SetSettings(a.set); err != nil {
		a.comm.note = err.Error()
		return
	}
	a.comm.note = tr("已退出")
	if cl.Configured() && token != "" {
		go func() { _ = cl.Logout(context.Background(), token) }()
	}
}

// uploadDraft sends the draft in the editor for review. It needs a login.
func (a *app) uploadDraft() {
	if a.draft.IsTable() {
		a.comm.note = tr("数据表不能上传到社区，只有脚本可以")
		return
	}
	cl := a.communityClient()
	if a.set.Token == "" {
		a.comm.note = tr("上传需要先登录")
		return
	}
	meta, _ := a.metaOf(a.draft)
	name, icon, desc, src, token := a.draft.Name, meta.Icon, a.uploadDesc, a.draft.Source, a.set.Token
	a.comm.busy = true
	a.background(func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_, msg, err := cl.Upload(ctx, token, name, icon, desc, src)
		return msg, err
	}, func(msg string, err error) {
		a.comm.busy = false
		if err != nil {
			a.comm.note = err.Error()
			return
		}
		a.comm.note = msg
	})
}

// importFromCommunity downloads an approved script and imports it into the
// local scripts.
func (a *app) importFromCommunity(id string) {
	cl := a.communityClient()
	a.comm.busy = true
	a.background(func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return cl.Download(ctx, id)
	}, func(raw string, err error) {
		a.comm.busy = false
		if err != nil {
			a.comm.note = err.Error()
			return
		}
		if err := a.importScript(raw, store.OriginCommunity, id); err != nil {
			a.comm.note = tr("下载的内容不能用：") + err.Error()
			return
		}
		a.comm.note = tr("已导入，可以在「编辑」里查看")
	})
}

// community is the community page: the account, an upload button for the
// draft, and the list of approved scripts to download.
func (a *app) community(c *ui.Context, compact bool) {
	p := pal(c)
	cl := a.communityClient()
	a.loadCommunity()

	page(c, compact, func() {
		header(c, tr("社区"), tr("下载无需登录，上传需要登录，并等管理员审核"), func() {
			if btn(c, tr("刷新")).Clicked() {
				a.comm.loaded = false
			}
		})

		// The account.
		card(c).Padding(18).Gap(gapItem).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				iconBadge(c, "user-round", p.accent, 28)
				ui.Column(c).Gap(1).Children(func() {
					if a.set.Token == "" {
						ui.Text(c, tr("未登录")).FontWeight(600)
						hint(c, tr("登录后可以上传脚本（下载不需要登录）"))
					} else {
						ui.Text(c, tr("已登录：")+a.set.Username).FontWeight(600)
						hint(c, tr("上传后需要管理员审核，通过后才会出现在列表里"))
					}
				})
			})
			ui.Box(c).FillWidth().Height(1).Background(p.hairline)
			if a.set.Token == "" {
				// Fields of a form are one width, as wide as the form allows.
				ui.Column(c).Gap(gapItem).MaxWidth(formMaxW).Children(func() {
					field(c, tr("用户名"), "", func() {
						textInput(c, &a.authUser, tr("3 到 20 位字母、数字或下划线"))
					})
					field(c, tr("密码"), "", func() {
						passwordInput(c, &a.authPass, tr("至少 8 位"))
					})
				})
				ui.Row(c).Gap(8).Children(func() {
					if primaryBtn(c, tr("登录")).Clicked() {
						a.authenticate(false)
					}
					if btn(c, tr("注册")).Clicked() {
						a.authenticate(true)
					}
				})
			} else {
				ui.Column(c).MaxWidth(formMaxW).Children(func() {
					field(c, tr("简介"), "", func() {
						textInput(c, &a.uploadDesc, tr("一句话说明这个脚本做什么（可选）"))
					})
				})
				ui.Row(c).Gap(8).Children(func() {
					if primaryBtn(c, tr("上传编辑中的脚本")).Clicked() {
						a.uploadDraft()
					}
					if btn(c, tr("退出登录")).Clicked() {
						a.logout()
					}
				})
			}
			if a.comm.note != "" {
				chip(c, a.comm.note, p.accent, p.accentSoft)
			}
		})

		// The scripts.
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			sectionTitle(c, tr("已通过审核的脚本"))
			if a.comm.loading {
				ui.Text(c, tr("加载中…")).FontSize(fsSmall).TextColor(p.muted)
			}
		})
		if a.comm.err != "" {
			ui.Text(c, a.comm.err).FontSize(fsSmall).TextColor(p.up)
		}
		switch {
		case !cl.Configured():
			card(c).Children(func() {
				empty(c, "globe", tr("还没有连接社区"), tr("还没有设置社区地址：到「设置」里填写 Cloudflare Worker 的地址。"))
			})
		case len(a.comm.items) == 0 && !a.comm.loading && a.comm.err == "":
			card(c).Children(func() {
				empty(c, "users", tr("社区里还没有脚本"), tr("第一个上传并通过审核的脚本会出现在这里。"))
			})
		default:
			cols := 2
			if compact {
				cols = 1
			} else if a.contentW() > 940 {
				cols = 3
			}
			w := (a.contentW() - cardGap*float32(cols-1)) / float32(cols)
			ui.Row(c).Wrap().Gap(cardGap).Children(func() {
				for _, s := range a.comm.items {
					a.communityCard(c, s, w)
				}
			})
		}
	})
}

// communityCard is one approved script, with the button that imports it.
func (a *app) communityCard(c *ui.Context, s community.Script, width float32) {
	p := pal(c)
	card(c).Width(width).Padding(16).Gap(8).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			iconBadge(c, s.Icon, p.accent, 34)
			ui.Column(c).Children(func() {
				ui.Text(c, s.Name).Bold().NoWrap()
				ui.Text(c, s.Author).FontSize(fsTiny).TextColor(p.muted)
			})
			ui.Spacer(c)
			chip(c, tr("下载 ")+strconv.Itoa(s.Downloads), p.muted, p.chip)
		})
		if s.Description != "" {
			ui.Text(c, s.Description).FontSize(fsCaption).TextColor(p.muted)
		}
		if btn(c, tr("下载并导入")).Clicked() && !a.comm.busy {
			a.importFromCommunity(s.ID)
		}
	})
}
