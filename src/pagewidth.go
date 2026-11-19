package main

import (
	"slices"
	"sync/atomic"

	"github.com/egoist/mygo/ui"
)

// How wide a page's content may be is the user's choice (settings, "内容宽度"):
//
//	auto         pages of data (the home cards, the detail page) fill the window;
//	             pages of text and forms keep a width that reads well
//	comfortable  every page keeps that width
//	wide         every page may be wider
//	full         every page fills the window
//
// A page that is capped sits in the middle of the window, not at its left with
// blank space on its right. What sizes itself from the page's width (the cards'
// grid, the statistics tiles, a picture) reads the same cap through contentW.

// The width choices, in the order of the settings row.
const (
	widthAuto        = ""
	widthComfortable = "comfortable"
	widthWide        = "wide"
	widthFull        = "full"
)

var widthModes = []string{widthAuto, widthComfortable, widthWide, widthFull}

// widthMode is the chosen mode; capNow is the cap of the page being drawn. Both
// are written and read on the UI thread, as the page is built.
var (
	widthMode atomic.Value
	capNow    float32
)

func init() { widthMode.Store(widthAuto) }

// setPageWidth picks a width mode. An unknown name is auto.
func setPageWidth(name string) {
	if !slices.Contains(widthModes, name) {
		name = widthAuto
	}
	widthMode.Store(name)
}

// widthIndex is the place of a mode in the settings row.
func widthIndex(name string) int { return max(slices.Index(widthModes, name), 0) }

// pageCap is the widest a page's content may be, 0 for the whole window. data
// says the page is data (cards, charts) rather than text.
func pageCap(mode string, data bool) float32 {
	switch mode {
	case widthComfortable:
		return 1120
	case widthWide:
		return 1600
	case widthFull:
		return 0
	}
	if data {
		return 0
	}
	return 1120
}

// pageIn is the scrolling body of a page, padded, with its blocks a section
// apart, no wider than the chosen width allows and in the middle of the window
// when it is narrower. data says the page is data rather than text.
func pageIn(c *ui.Context, compact, data bool, body func()) ui.Element {
	limit := pageCap(widthMode.Load().(string), data)
	return ui.Scroll(c).Grow(1).Padding(pad(compact)).Children(func() {
		capNow = limit
		if limit == 0 {
			ui.Column(c).Gap(gapSection).Children(body)
			return
		}
		ui.Row(c).Justify(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MaxWidth(limit).Gap(gapSection).Children(body)
		})
	})
}
