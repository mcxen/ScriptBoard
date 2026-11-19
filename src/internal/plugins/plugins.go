// Package plugins keeps local plugins and icons in folders of the data
// directory. A plugin is a folder with plugin.json, script.js and optional
// icons/*.svg. Nothing here needs a login or a review: a plugin is code on
// this computer, loaded as it is. The app and the MCP server both use this
// package, so a plugin written by one is seen by the other.
//
//	<data>/plugins/<id>/plugin.json
//	<data>/plugins/<id>/script.js
//	<data>/plugins/<id>/icons/<name>.svg
//	<data>/icons/<name>.svg            icons not tied to a plugin
package plugins

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"scriptboard/src/internal/engine"
)

// MaxIconBytes limits the size of one SVG icon.
const MaxIconBytes = 32 * 1024

var (
	idPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,39}$`)
	iconPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
)

// Manifest is plugin.json.
type Manifest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Icon        string `json:"icon,omitempty"`
	Description string `json:"description,omitempty"`
	Home        bool   `json:"home"`
	Version     string `json:"version,omitempty"`
	// Position is where the card sits on the home page, from 1 (first); 0
	// leaves it after the cards the user arranged.
	Position int `json:"position,omitempty"`
	// Size is the card's default size: mini, small, medium, large or wide. The
	// user's own size, once chosen, wins.
	Size string `json:"size,omitempty"`
	// Menubar puts the card in the menu bar (or notification area) when the
	// plugin first appears in the app. The script must ask for the bar with
	// menubar: true in meta(); the user can take the card out, and put it back.
	Menubar bool `json:"menubar,omitempty"`
}

// CardSizes are the sizes a plugin may name for its card.
var CardSizes = []string{"mini", "small", "medium", "large", "wide"}

// Plugin is a plugin read from its folder.
type Plugin struct {
	Manifest Manifest
	Source   string
	Dir      string
}

// PluginDir is the folder that holds the plugins.
func PluginDir(dataDir string) string { return filepath.Join(dataDir, "plugins") }

// IconDir is the folder of icons that belong to no plugin.
func IconDir(dataDir string) string { return filepath.Join(dataDir, "icons") }

// CheckSource compiles a script and reads its meta. A script that does not
// compile, or does not export meta() and render(), is refused.
func CheckSource(name, source string) (engine.Meta, error) {
	s, err := engine.Compile(name, source)
	if err != nil {
		return engine.Meta{}, err
	}
	if !s.HasRender() {
		return engine.Meta{}, errors.New("脚本需要导出 render(ctx)")
	}
	return s.Meta, nil
}

// Save writes a plugin, creating or replacing it. The script must pass
// CheckSource. The write goes through a temporary file, so a crash never
// leaves a half-written plugin.json.
func Save(dataDir string, m Manifest, source string) error {
	if !idPattern.MatchString(m.ID) {
		return errors.New("插件 id 只能用小写字母、数字和连字符，2 到 40 个字符")
	}
	if m.Position < 0 || m.Position > 200 {
		return errors.New("首页位置是 1 到 200 的数字，1 是第一张；不写就排在最后")
	}
	if m.Size != "" && !containsWord(CardSizes, m.Size) {
		return fmt.Errorf("默认尺寸要是 %s 之一", strings.Join(CardSizes, "、"))
	}
	if strings.TrimSpace(m.Name) == "" || len([]rune(m.Name)) > 60 {
		return errors.New("插件名称为 1 到 60 个字")
	}
	if m.Icon != "" && !iconPattern.MatchString(m.Icon) {
		return errors.New("图标名只能用小写字母、数字和连字符")
	}
	meta, err := CheckSource(m.ID+".js", source)
	if err != nil {
		return fmt.Errorf("脚本无法运行：%w", err)
	}
	if m.Menubar && !meta.Menubar {
		return errors.New("要放进菜单栏，脚本的 meta() 里要写 menubar: true")
	}
	dir := filepath.Join(PluginDir(dataDir), m.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	manifest, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(dir, "plugin.json"), append(manifest, '\n')); err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, "script.js"), []byte(source))
}

// Delete removes a plugin folder.
func Delete(dataDir, id string) error {
	if !idPattern.MatchString(id) {
		return errors.New("插件 id 不对")
	}
	return os.RemoveAll(filepath.Join(PluginDir(dataDir), id))
}

// List reads every plugin. A folder that is not a valid plugin is reported in
// the error and skipped; the valid ones are still returned.
func List(dataDir string) ([]Plugin, error) {
	entries, err := os.ReadDir(PluginDir(dataDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Plugin
	var problems []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(PluginDir(dataDir), e.Name())
		p, err := readPlugin(dir)
		if err != nil {
			problems = append(problems, e.Name()+"："+err.Error())
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Manifest.ID < out[j].Manifest.ID })
	if len(problems) > 0 {
		return out, errors.New(strings.Join(problems, "；"))
	}
	return out, nil
}

func readPlugin(dir string) (Plugin, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "plugin.json"))
	if err != nil {
		return Plugin{}, errors.New("没有 plugin.json")
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Plugin{}, fmt.Errorf("plugin.json 格式不对：%w", err)
	}
	if !idPattern.MatchString(m.ID) || filepath.Base(dir) != m.ID {
		return Plugin{}, errors.New("plugin.json 的 id 要和文件夹名一样")
	}
	src, err := os.ReadFile(filepath.Join(dir, "script.js"))
	if err != nil {
		return Plugin{}, errors.New("没有 script.js")
	}
	if _, err := CheckSource(m.ID+".js", string(src)); err != nil {
		return Plugin{}, err
	}
	if m.Name == "" {
		meta, _ := CheckSource(m.ID+".js", string(src))
		m.Name = meta.Name
	}
	return Plugin{Manifest: m, Source: string(src), Dir: dir}, nil
}

// IconDirs lists the folders icons are looked up in, most specific first:
// each plugin's icons, then the shared folder.
func IconDirs(dataDir string) []string {
	var dirs []string
	entries, _ := os.ReadDir(PluginDir(dataDir))
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(PluginDir(dataDir), e.Name(), "icons"))
		}
	}
	return append(dirs, IconDir(dataDir))
}

// AddIcon saves an SVG icon as name, in the icons of plugin, or in the shared
// icons when plugin is empty. It returns the path written.
func AddIcon(dataDir, plugin, name string, svg []byte) (string, error) {
	if !iconPattern.MatchString(name) {
		return "", errors.New("图标名只能用小写字母、数字和连字符")
	}
	if err := ValidateSVG(svg); err != nil {
		return "", err
	}
	dir := IconDir(dataDir)
	if plugin != "" {
		if !idPattern.MatchString(plugin) {
			return "", errors.New("插件 id 不对")
		}
		dir = filepath.Join(PluginDir(dataDir), plugin, "icons")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name+".svg")
	return path, writeAtomic(path, svg)
}

// ListIcons returns the names of the icons available to scripts, with the
// plugin they belong to in brackets when there is one.
func ListIcons(dataDir string) []string {
	var out []string
	for _, dir := range IconDirs(dataDir) {
		entries, _ := os.ReadDir(dir)
		owner := ""
		if dir != IconDir(dataDir) {
			owner = filepath.Base(filepath.Dir(dir))
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".svg") {
				name := strings.TrimSuffix(e.Name(), ".svg")
				if owner != "" {
					name += " (" + owner + ")"
				}
				out = append(out, name)
			}
		}
	}
	sort.Strings(out)
	return out
}

// ValidateSVG accepts an SVG that is small and can do nothing but draw: no
// scripts, no foreign content, no event handlers and no links out.
func ValidateSVG(svg []byte) error {
	if len(svg) == 0 {
		return errors.New("SVG 是空的")
	}
	if len(svg) > MaxIconBytes {
		return errors.New("SVG 太大，最多 32 KB")
	}
	lower := strings.ToLower(string(svg))
	for _, bad := range []string{"<script", "<foreignobject", "<iframe", "<object", "<embed", "javascript:"} {
		if strings.Contains(lower, bad) {
			return fmt.Errorf("SVG 里不能有 %s", bad)
		}
	}
	dec := xml.NewDecoder(bytes.NewReader(svg))
	root := true
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("SVG 格式不对：%w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if root {
			root = false
			if se.Name.Local != "svg" {
				return errors.New("根元素必须是 svg")
			}
		}
		for _, a := range se.Attr {
			if strings.HasPrefix(strings.ToLower(a.Name.Local), "on") {
				return errors.New("SVG 里不能有事件属性")
			}
		}
	}
	if root {
		return errors.New("SVG 格式不对")
	}
	return nil
}

// Fingerprint summarises the plugin and icon folders, so that a change in any
// file shows as a different string.
func Fingerprint(dataDir string) string {
	var parts []string
	walk := func(root string) {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			parts = append(parts, fmt.Sprintf("%s:%d:%d", path, info.Size(), info.ModTime().UnixNano()))
			return nil
		})
	}
	walk(PluginDir(dataDir))
	walk(IconDir(dataDir))
	return strings.Join(parts, "|")
}

func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// DefaultDataDir is where ScriptBoard keeps its data unless SCRIPTBOARD_DATA
// says otherwise: the ScriptBoard folder in the user's configuration folder.
func DefaultDataDir() (string, error) {
	if d := os.Getenv("SCRIPTBOARD_DATA"); d != "" {
		return d, nil
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "ScriptBoard"), nil
}

// containsWord reports whether word is one of list.
func containsWord(list []string, word string) bool {
	for _, w := range list {
		if w == word {
			return true
		}
	}
	return false
}
