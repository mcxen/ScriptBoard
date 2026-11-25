package main

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/egoist/mygo"

	"scriptboard/src/internal/caps"
	"scriptboard/src/internal/store"
)

// isMobile reports whether the app is running on a phone or tablet (iOS or Android).
// Keep isAndroid() solely for Android-specific APK self-updates.
func isMobile() bool { return caps.Platform() == caps.Mobile }

func (a *app) hapticSelection() {
	if a.win != nil && isMobile() {
		mygo.Haptics.Selection()
	}
}

func (a *app) hapticLight() {
	if a.win != nil && isMobile() {
		mygo.Haptics.Light()
	}
}

func (a *app) hapticMedium() {
	if a.win != nil && isMobile() {
		mygo.Haptics.Medium()
	}
}

func (a *app) hapticSuccess() {
	if a.win != nil && isMobile() {
		mygo.Haptics.Success()
	}
}

func (a *app) hapticError() {
	if a.win != nil && isMobile() {
		mygo.Haptics.Error()
	}
}

// systemBarStyle picks the status/navigation bar icon brightness for the active
// theme mode and skin so mobile system icons remain readable over the page.
func systemBarStyle(themeMode string, macSkin, dark bool) mygo.BarStyle {
	if themeMode == themeSystem && !macSkin {
		return mygo.BarStyleAuto
	}
	if dark {
		return mygo.BarStyleLight
	}
	return mygo.BarStyleDark
}

func (a *app) syncSystemBars(dark bool) {
	if a.win == nil || !isMobile() {
		return
	}
	want := systemBarStyle(curThemeMode(), isMac(), dark)
	if want != a.lastBarStyle {
		mygo.SystemBars.SetStyle(want)
		a.lastBarStyle = want
	}
}

// shareContent opens the OS native share sheet on mobile (or calls a.shareFn in tests).
func (a *app) shareContent(opts mygo.ShareOptions, statusOK string) {
	fn := a.shareFn
	if fn == nil {
		if a.win == nil {
			a.status = tr("分享失败：") + errors.ErrUnsupported.Error()
			return
		}
		fn = mygo.Share
	}
	a.background(func() (string, error) {
		return "", fn(opts)
	}, func(_ string, err error) {
		if err != nil {
			a.status = tr("分享失败：") + err.Error()
			a.hapticError()
			return
		}
		if statusOK != "" {
			a.status = statusOK
		}
		a.hapticSuccess()
	})
}

func (a *app) shareScript(sc store.Script) {
	a.shareContent(mygo.ShareOptions{Text: exportScript(sc)}, "")
}

type savedMobileState struct {
	Path        string `json:"path,omitempty"`
	SettingsTab string `json:"settingsTab,omitempty"`
	EditTab     int    `json:"editTab,omitempty"`
}

func (a *app) encodeAppState() []byte {
	data, err := json.Marshal(savedMobileState{
		Path:        a.router.Path(),
		SettingsTab: a.settingsTab,
		EditTab:     a.editTab,
	})
	if err != nil {
		return nil
	}
	return data
}

func isValidRestorableRoute(path string) bool {
	if path == "" || path == "/" {
		return false
	}
	switch {
	case path == "/edit",
		strings.HasPrefix(path, "/edit/"),
		path == "/bar",
		strings.HasPrefix(path, "/bar/"),
		strings.HasPrefix(path, "/full/"),
		path == "/feeds",
		path == "/community",
		path == "/settings":
		return true
	default:
		return false
	}
}

func (a *app) restoreAppState(data []byte) {
	var s savedMobileState
	if err := json.Unmarshal(data, &s); err != nil {
		return
	}
	if s.SettingsTab != "" {
		a.settingsTab = settingsTabByKey(s.SettingsTab).key
	}
	if s.EditTab >= 0 && s.EditTab <= 2 {
		a.editTab = s.EditTab
	}
	if isValidRestorableRoute(s.Path) {
		a.router.Replace(s.Path)
	}
}
