package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"scriptboard/internal/engine"
	"scriptboard/internal/store"
)

// shareFormat tags the JSON a script is exported as, so that it can be
// recognised when it is imported again.
const shareFormat = "scriptboard/1"

// shareDoc is a script as it is shared: as JSON, in a file, a message or on
// the community.
type shareDoc struct {
	Format string `json:"format"`
	Name   string `json:"name"`
	Icon   string `json:"icon,omitempty"`
	Source string `json:"source"`
}

// exportScript returns sc as shareable JSON.
func exportScript(sc store.Script) string {
	doc := shareDoc{Format: shareFormat, Name: sc.Name, Source: sc.Source}
	if s, err := engine.Compile(sc.Name, sc.Source); err == nil {
		doc.Icon = s.Meta.Icon
	}
	b, _ := json.MarshalIndent(doc, "", "  ")
	return string(b)
}

// parseScript reads a script from text: either the JSON of exportScript, or
// the bare source of an ES module. The source must compile and export meta().
func parseScript(text string) (store.Script, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return store.Script{}, errors.New(tr("没有内容"))
	}
	var src, name string
	if strings.HasPrefix(text, "{") {
		var doc shareDoc
		if err := json.Unmarshal([]byte(text), &doc); err != nil {
			return store.Script{}, fmt.Errorf(tr("JSON 格式不对：%w"), err)
		}
		src, name = doc.Source, doc.Name
	} else {
		src = text
	}
	s, err := engine.Compile(name, src)
	if err != nil {
		return store.Script{}, fmt.Errorf(tr("脚本无法运行：%w"), err)
	}
	if name == "" {
		name = s.Meta.Name
	}
	if name == "" {
		name = tr("导入的脚本")
	}
	return store.Script{Name: name, Source: src, Home: true}, nil
}

// importText saves the script in text as a new script and opens it in the
// editor. It reports the problem instead when the text is not a script.
func (a *app) importText(text string) error {
	sc, err := parseScript(text)
	if err != nil {
		return err
	}
	sc.ID = fmt.Sprintf("import-%d", a.now().UnixNano())
	if err := a.store.SaveScript(sc); err != nil {
		return err
	}
	a.editorID = ""
	a.openEditor(sc.ID)
	a.status = tr("已导入")
	return nil
}

// transferPanel shows the import or export form under the editor's header,
// when one is open.
func (a *app) transferPanel(c *ui.Context) {
	p := pal(c)
	if a.panel == "" {
		return
	}
	ui.Column(c).Padding(14).Gap(10).Radius(radControl + 2).Background(p.chip).Children(func() {
		if a.panel == "import" {
			ui.Text(c, tr("导入脚本")).Bold()
			hint(c, tr("粘贴脚本：JSON（导出的格式）或 ES 模块源码"))
			textArea(c, &a.transferText, "", 8).Font("monospace").FontSize(fsSmall)
			if a.transferErr != "" {
				ui.Text(c, a.transferErr).FontSize(fsSmall).TextColor(p.up)
			}
			ui.Row(c).Gap(8).Children(func() {
				if primaryBtn(c, tr("导入")).Clicked() {
					if err := a.importText(a.transferText); err != nil {
						a.transferErr = err.Error()
					} else {
						a.transferErr = ""
						a.transferText = ""
						a.panel = ""
					}
				}
				if btn(c, tr("关闭")).Clicked() {
					a.panel = ""
					a.transferErr = ""
				}
			})
			return
		}
		ui.Text(c, tr("导出脚本")).Bold()
		hint(c, tr("复制后发给别人，或粘贴到「导入」里"))
		textArea(c, &a.transferText, "", 8).Font("monospace").FontSize(fsSmall)
		if btn(c, tr("关闭")).Clicked() {
			a.panel = ""
		}
	})
}
