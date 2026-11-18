package main

import (
	"strings"
	"time"

	"scriptboard/internal/plugins"
	"scriptboard/internal/store"
)

// pluginPrefix marks the IDs of scripts that come from the plugin folder.
const pluginPrefix = "plugin-"

func isPluginScript(id string) bool { return strings.HasPrefix(id, pluginPrefix) }

// syncPlugins brings the plugin folder into the scripts and points the icon
// lookup at its icon folders. The folder is the source of truth for plugins:
// when a plugin's files change, its stored copy is replaced; a plugin that is
// gone is removed. Local plugins need no login and no review.
func (a *app) syncPlugins() {
	list, err := plugins.List(a.dataDir)
	seen := map[string]bool{}
	for _, p := range list {
		id := pluginPrefix + p.Manifest.ID
		seen[id] = true
		sc := store.Script{ID: id, Name: p.Manifest.Name, Source: p.Source, Home: p.Manifest.Home}
		if cur, ok := a.store.Script(id); !ok || cur != sc {
			if err := a.store.SaveScript(sc); err == nil {
				a.invalidate(id)
			}
		}
	}
	for _, sc := range a.store.Scripts() {
		if isPluginScript(sc.ID) && !seen[sc.ID] {
			if err := a.store.DeleteScript(sc.ID); err == nil {
				a.invalidate(sc.ID)
				if a.editorID == sc.ID {
					a.editorID = ""
				}
			}
		}
	}
	setIconDirs(plugins.IconDirs(a.dataDir))
	a.pluginNote = ""
	if err != nil {
		a.pluginNote = tr("有插件没能加载：") + err.Error()
	}
}

// watchPlugins checks the plugin and icon folders once a second, and syncs on
// the main thread when something changed.
func (a *app) watchPlugins() {
	last := plugins.Fingerprint(a.dataDir)
	for range time.Tick(time.Second) {
		fp := plugins.Fingerprint(a.dataDir)
		if fp == last {
			continue
		}
		last = fp
		a.post(a.syncPlugins)
	}
}
