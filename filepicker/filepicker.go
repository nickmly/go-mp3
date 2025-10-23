package filepicker

import (
	"log"
	"os"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type FilePicker struct {
	cursor     int
	paths      []string
	currentDir string
}

func (fp *FilePicker) moveCursorUp() {
	if fp.cursor > 0 {
		fp.cursor--
	} else {
		fp.cursor = len(fp.paths) - 1
	}
}

func (fp *FilePicker) moveCursorDown() {
	if fp.cursor < len(fp.paths)-1 {
		fp.cursor++
	} else {
		fp.cursor = 0
	}
}

func (fp *FilePicker) populatePaths(dir string) {
	fp.cursor = 0
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	fp.paths = []string{".."}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fp.paths = append(fp.paths, entry.Name())
	}
}

func (fp *FilePicker) SelectPath() {
	selected := fp.paths[fp.cursor]
	if selected == ".." {
		parentDir := filepath.Dir(fp.currentDir)
		if parentDir == fp.currentDir {
			return
		}
		fp.currentDir = parentDir
		fp.populatePaths(fp.currentDir)
	} else {
		nextDir := filepath.Join(fp.currentDir, selected)
		info, err := os.Stat(nextDir)
		if err != nil {
			log.Fatal(err)
		}
		if !info.IsDir() {
			return
		}
		fp.currentDir = nextDir
		fp.populatePaths(fp.currentDir)
	}
}

func (fp *FilePicker) PrintPaths() string {
	text := ""
	for i, path := range fp.paths {
		if i == fp.cursor {
			text += "[\"active\"][yellow]" + path + "[\"\"]\n"
		} else {
			text += "[white]" + path + "\n"
		}
	}
	return text
}

func NewFilePicker(dir string) *FilePicker {
	fp := &FilePicker{cursor: 0, currentDir: dir}
	fp.populatePaths(dir)
	return fp
}

func Open(app *tview.Application, root tview.Primitive, dir string, onSelect func(path string)) {
	fp := NewFilePicker(dir)
	base := tview.NewFlex().SetDirection(tview.FlexColumnCSS)
	base.SetBorder(true)
	base.SetTitle("Select Directory (Esc to go back)")
	textView := tview.NewTextView()
	refresh := func() {
		textView.SetText(fp.PrintPaths())
		textView.Highlight("active")
		textView.ScrollToHighlight()
	}
	textView.SetDynamicColors(true)
	textView.SetScrollable(true)
	textView.SetRegions(true)
	textView.SetChangedFunc(func() {
		app.Draw()
	})
	refresh()
	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyUp {
			fp.moveCursorUp()
			refresh()
			return nil
		} else if event.Key() == tcell.KeyDown {
			fp.moveCursorDown()
			refresh()
			return nil
		} else if event.Key() == tcell.KeyEnter {
			fp.SelectPath()
			refresh()
			return nil
		}
		return event
	})
	base.AddItem(textView, 0, 1, true)
	base.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			app.SetRoot(root, true)
			onSelect(fp.currentDir)
			return nil
		}
		return event
	})
	app.SetRoot(base, true)
}
