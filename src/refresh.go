package main

import (
	"context"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"scriptboard/src/internal/store"
)

// refreshScript brings a card's data up to date now, instead of at the next
// interval: it polls the data sources the script reads (curl, tcp, file…),
// forgets the AI usage it has cached, and runs the script again, which asks
// its own sources (meta.sources, such as a custom endpoint) afresh. Offline,
// nothing is requested and the script runs again on what is stored.
func (a *app) refreshScript(sc store.Script) {
	if a.refreshing[sc.ID] {
		return
	}
	meta, _ := a.metaOf(sc)
	feeds := a.store.Feeds()
	offline := a.store.Settings().Offline
	a.refreshing[sc.ID] = true

	run := func() {
		if !offline {
			for _, id := range meta.Feeds {
				for _, src := range feeds {
					if src.ID == id {
						_, _ = a.runner.Poll(context.Background(), src)
					}
				}
			}
		}
		a.post(func() {
			a.forced[sc.ID] = true
			delete(a.refreshing, sc.ID)
			if len(meta.Usage) > 0 {
				a.usage.mu.Lock()
				a.usage.entries = nil
				a.usage.mu.Unlock()
			}
			if meta.NetProbe != nil {
				a.forgetNetProbe()
			}
			a.invalidate(sc.ID)
		})
	}
	if a.win == nil {
		run()
		return
	}
	go run()
}

// refreshButton is the button that refreshes a script: it shows as pressed
// while the refresh runs.
// show false hides it until the card is hovered: it stays built, transparent and
// out of the pointer's way, so that assistive technology still finds it.
func (a *app) refreshButton(c *ui.Context, sc store.Script, show bool) {
	el := action.ActionIcon(c, icon("refresh-cw"), action.ActionIconProps{
		Size: theme.Small, Title: tr("刷新数据"), Active: a.refreshing[sc.ID],
	})
	if !show {
		el = el.Opacity(0).IgnoreInput(true)
	}
	if el.Clicked() {
		a.refreshScript(sc)
	}
}
