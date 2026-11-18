// ScriptBoard is a small desktop app with three pages: home cards that draw
// curves from scripts, an editor for those scripts and their data, and
// settings. The scripts run in moejs; the interface is drawn by MyGo.
package main

import (
	"log"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/mcpsvc"
	"scriptboard/src/internal/plugins"
	"scriptboard/src/internal/store"
)

func main() {
	dir, err := plugins.DefaultDataDir()
	if err != nil {
		log.Fatal(err)
	}
	// --mcp-stdio: serve MCP over stdio for a client that starts a program.
	// No window opens; the data folder is read and written directly.
	if len(os.Args) > 1 && os.Args[1] == "--mcp-stdio" {
		srv := mcpsvc.Server{DataDir: dir, ReadOnly: func() bool {
			st, err := store.Open(dir)
			return err == nil && st.Settings().MCP.ReadOnly
		}}
		if err := srv.Serve(os.Stdin, os.Stdout); err != nil {
			log.Fatal(err)
		}
		return
	}
	a, err := newApp(dir, false)
	if err != nil {
		log.Fatal(err)
	}
	// Looks for a signed release in the background shortly after launch, then
	// daily, and shows the release notes with Install / Later / Skip. Builds
	// that cannot update themselves (development builds, a package manager's
	// install) never check.
	mygo.Use(native.New(updater.Options{Language: curLang()}))
	mygo.App.WhenReady(func() {
		applyUpdateSource(a.set.UpdateSource)
		w := mygo.NewWindow(mygo.WindowOptions{
			Title:     appName(),
			Width:     1180,
			Height:    760,
			MinWidth:  900,
			MinHeight: 480,
			// Opens where the user left it last time.
			StateKey: "main",
			// The window shows the interface MyGo draws, not a web page.
			Content: ui.View(a.view),
		})
		a.attach(w)
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
