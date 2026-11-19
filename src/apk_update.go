package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// On Android mygo's updater does nothing: a phone installs an update only when
// the user installs the APK by hand. So the app looks for the newest release
// itself, when it starts and when the user asks, and offers its APK; the user
// taps it, and installs.

// androidLatestURL is GitHub's address of the newest release. It redirects to
// the release's tag, which names the version.
const androidLatestURL = "https://github.com/" + updateRepo + "/releases/latest"

// apkRelease is the newest release: its version and the address of its APK.
type apkRelease struct {
	Version string
	URL     string
}

func isAndroid() bool { return runtime.GOOS == "android" }

// newerVersion reports whether version a is later than b: dotted numbers,
// compared part by part, so that 0.1.10 is later than 0.1.9.
func newerVersion(a, b string) bool {
	pa, pb := strings.Split(strings.TrimPrefix(a, "v"), "."), strings.Split(strings.TrimPrefix(b, "v"), ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x > y
		}
	}
	return false
}

// latestAPK reads the newest release from the address base (GitHub's, or a
// test's): the redirect's Location names the tag, and the APK is named after
// the version, as the release workflow names it.
func latestAPK(ctx context.Context, client *http.Client, base string) (apkRelease, error) {
	if client == nil {
		client = http.DefaultClient
	}
	noFollow := *client
	noFollow.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base, nil)
	if err != nil {
		return apkRelease{}, err
	}
	resp, err := noFollow.Do(req)
	if err != nil {
		return apkRelease{}, err
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	i := strings.LastIndex(loc, "/tag/")
	if i < 0 {
		return apkRelease{}, errors.New(fmt.Sprintf("no release found (HTTP %d)", resp.StatusCode))
	}
	tag := loc[i+len("/tag/"):]
	version := strings.TrimPrefix(tag, "v")
	return apkRelease{
		Version: version,
		URL:     updateDownload + tag + "/scriptboard-" + version + ".apk",
	}, nil
}

// checkAPK asks for the newest release on a background goroutine: through the
// mirror for a Chinese user, GitHub otherwise, and GitHub when the mirror is
// down. The result is kept until the next check.
func (a *app) checkAPK() {
	if a.apkBusy {
		return
	}
	a.apkBusy = true
	var rel apkRelease
	bases := []string{androidLatestURL}
	if curLang() == langZH {
		bases = []string{updateMirror + androidLatestURL, androidLatestURL}
	}
	a.background(func() (string, error) {
		var err error
		for _, base := range bases {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			rel, err = latestAPK(ctx, a.client, base)
			cancel()
			if err == nil {
				return rel.Version, nil
			}
		}
		return "", err
	}, func(v string, err error) {
		a.apkBusy = false
		a.apkChecked = true
		a.apkErr = ""
		if err != nil {
			a.apkErr = err.Error()
			a.apkRel = nil
			return
		}
		a.apkRel = &rel
	})
}

// apkNewer is the newest release, when it is later than the running version.
func (a *app) apkNewer() *apkRelease {
	if a.apkRel == nil || !newerVersion(a.apkRel.Version, versionString()) {
		return nil
	}
	return a.apkRel
}

// androidUpdateLine says what the last check found.
func (a *app) androidUpdateLine() string {
	switch {
	case a.apkBusy:
		return tr("正在检查…")
	case a.apkErr != "":
		return tr("检查失败：") + a.apkErr
	case a.apkNewer() != nil:
		return tr("有新版本 ") + a.apkNewer().Version + tr("：下载 APK 后安装，安装会覆盖旧版本，数据保留。")
	case a.apkChecked:
		return tr("已是最新版本。")
	}
	return tr("还没有检查过。")
}

// androidRelease opens the APK of the newer release for the user to install.
func (a *app) androidRelease() {
	if r := a.apkNewer(); r != nil {
		if err := mygo.Shell.OpenExternal(r.URL); err != nil {
			a.apkErr = err.Error()
		}
	}
}
