package main

import (
	"fmt"
	"image/color"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// logPanic wraps a func so any panic prints a stack + a user-facing dialog
// instead of taking the whole app down silently.
func (s *State) logPanic(context string) {
	if r := recover(); r != nil {
		stack := string(debug.Stack())
		fmt.Fprintf(os.Stderr, "\n*** PANIC in %s: %v\n%s\n", context, r, stack)
		if s != nil && s.win != nil {
			dialog.ShowError(fmt.Errorf("%s crashed: %v\n\nRun cdf-gui from a terminal to see the details.", context, r), s.win)
		}
	}
}

// State ————————————————————————————————————————————————————————

type State struct {
	win       fyne.Window
	app       fyne.App
	cdf       *CDF
	path      string
	dirty     bool
	selection int // current section index

	// widgets we mutate
	titleLbl    *canvas.Text
	subtitleLbl *canvas.Text
	metaLbl     *widget.Label
	sectionList *widget.List
	sectionHdr  *canvas.Text
	sectionNote *widget.Label
	rowsBox     *fyne.Container
	statusLbl   *widget.Label
	dirtyDot    *canvas.Circle
}

// entry point ————————————————————————————————————————————————————

func main() {
	a := app.NewWithID("io.github.kcherry497.cdf")
	a.SetIcon(appIcon)
	a.Settings().SetTheme(cdfTheme{})
	w := a.NewWindow("CDF Viewer")
	w.Resize(fyne.NewSize(1200, 780))

	s := &State{app: a, win: w}
	s.buildLayout()
	s.buildMenu()

	// Argument: open a file if given
	if len(os.Args) > 1 {
		s.loadFile(os.Args[1])
	} else {
		s.showEmpty()
	}

	// Prompt on close if dirty
	w.SetCloseIntercept(func() {
		if !s.dirty {
			w.Close()
			return
		}
		dialog.NewCustomConfirm("Unsaved changes", "Discard", "Cancel",
			widget.NewLabel("You have unsaved changes. Discard them and quit?"),
			func(ok bool) { if ok { w.Close() } }, w).Show()
	})

	w.ShowAndRun()
}

// menu ————————————————————————————————————————————————————————

// shortcutItem binds a File-menu action and its canvas shortcut, blocking both during modals.
func (s *State) shortcutItem(label string, key fyne.KeyName, mod fyne.KeyModifier, fn func()) *fyne.MenuItem {
	sc := &desktop.CustomShortcut{KeyName: key, Modifier: mod}
	action := func() {
		if s.win.Canvas().Overlays().Top() != nil { return }
		fn()
	}
	s.win.Canvas().AddShortcut(sc, func(fyne.Shortcut) { action() })
	item := fyne.NewMenuItem(label, action)
	item.Shortcut = sc
	return item
}

func (s *State) buildMenu() {
	const ctrl = fyne.KeyModifierShortcutDefault // Ctrl on Linux/Windows, Cmd on macOS
	fileMenu := fyne.NewMenu("File",
		s.shortcutItem("New…",  fyne.KeyN, ctrl, s.newDocument),
		s.shortcutItem("Open…", fyne.KeyO, ctrl, s.openDialog),
		fyne.NewMenuItem("Open Path…", func() { s.openPathDialog() }),
		fyne.NewMenuItemSeparator(),
		s.shortcutItem("Save",     fyne.KeyS, ctrl, s.save),
		s.shortcutItem("Save As…", fyne.KeyS, ctrl|fyne.KeyModifierShift, s.saveAsDialog),
		fyne.NewMenuItemSeparator(),
		s.shortcutItem("Export HTML…", fyne.KeyE, ctrl, s.exportHTMLDialog),
	)
	sectionMenu := fyne.NewMenu("Section",
		fyne.NewMenuItem("Add Section…",    func() { s.addSection() }),
		fyne.NewMenuItem("Rename Section…", func() { s.renameSection() }),
		fyne.NewMenuItem("Edit Note…",      func() { s.editSectionNote() }),
		fyne.NewMenuItem("Delete Section…", func() { s.deleteSection() }),
	)
	rowMenu := fyne.NewMenu("Row",
		fyne.NewMenuItem("Add Row",     func() { s.addRow() }),
	)
	helpMenu := fyne.NewMenu("Help",
		fyne.NewMenuItem("About",       func() { s.showAbout() }),
	)
	s.win.SetMainMenu(fyne.NewMainMenu(fileMenu, sectionMenu, rowMenu, helpMenu))
}

// layout ———————————————————————————————————————————————————————

func (s *State) buildLayout() {
	// Header
	s.titleLbl = canvas.NewText("", colInk)
	s.titleLbl.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	s.titleLbl.TextSize = 22

	s.subtitleLbl = canvas.NewText("", colMuted)
	s.subtitleLbl.TextSize = 12

	s.metaLbl = widget.NewLabel("")
	s.metaLbl.TextStyle = fyne.TextStyle{Monospace: true}

	s.dirtyDot = canvas.NewCircle(color.Transparent)
	s.dirtyDot.Resize(fyne.NewSize(10, 10))

	dirtyBox := container.NewGridWrap(fyne.NewSize(12, 12), s.dirtyDot)
	headerRight := container.NewHBox(dirtyBox, s.metaLbl)
	headerTop := container.NewBorder(nil, nil, nil, headerRight, s.titleLbl)
	header := container.NewVBox(
		headerTop,
		s.subtitleLbl,
		canvas.NewLine(colBorder),
	)

	// Sidebar
	s.sectionList = widget.NewList(
		func() int {
			if s.cdf == nil { return 0 }
			return len(s.cdf.Sections)
		},
		func() fyne.CanvasObject {
			t := canvas.NewText("Section", colInk)
			t.TextStyle = fyne.TextStyle{Monospace: true}
			return container.NewPadded(t)
		},
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			t := obj.(*fyne.Container).Objects[0].(*canvas.Text)
			t.Text = fmt.Sprintf("%d. %s", i+1, s.cdf.Sections[i].Name)
			t.Refresh()
		},
	)
	s.sectionList.OnSelected = func(i widget.ListItemID) {
		s.selection = i
		s.renderSection()
	}
	sidebarHdr := canvas.NewText("SECTIONS", colMuted)
	sidebarHdr.TextStyle = fyne.TextStyle{Monospace: true}
	sidebarHdr.TextSize = 11
	sidebar := container.NewBorder(
		container.NewPadded(sidebarHdr),
		nil, nil, nil,
		s.sectionList,
	)

	// Main content
	s.sectionHdr = canvas.NewText("", colAccent)
	s.sectionHdr.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	s.sectionHdr.TextSize = 18

	s.sectionNote = widget.NewLabel("")
	s.sectionNote.Wrapping = fyne.TextWrapWord

	s.rowsBox = container.NewVBox()
	rowsScroll := container.NewVScroll(container.NewPadded(s.rowsBox))

	contentTop := container.NewVBox(
		s.sectionHdr,
		s.sectionNote,
		widget.NewSeparator(),
	)

	// Add-row button
	addRowBtn := widget.NewButton("+ Add row", func() { s.addRow() })
	addRowBtn.Importance = widget.LowImportance

	rightSide := container.NewBorder(contentTop, addRowBtn, nil, nil, rowsScroll)

	// Status bar
	s.statusLbl = widget.NewLabel("(no file)")
	s.statusLbl.TextStyle = fyne.TextStyle{Monospace: true}
	statusBar := container.NewBorder(widget.NewSeparator(), nil, nil, nil, container.NewHBox(s.statusLbl))

	// Split sidebar / main
	split := container.NewHSplit(sidebar, container.NewPadded(rightSide))
	split.Offset = 0.20

	root := container.NewBorder(container.NewPadded(header), statusBar, nil, nil, split)
	s.win.SetContent(root)
}

// render helpers ————————————————————————————————————————————————

func (s *State) showEmpty() {
	s.titleLbl.Text = "CDF Viewer"
	s.titleLbl.Refresh()
	s.subtitleLbl.Text = "Open a .cdf file (File → Open) or create a new one (File → New)."
	s.subtitleLbl.Refresh()
	s.metaLbl.SetText("")
	s.sectionHdr.Text = ""
	s.sectionHdr.Refresh()
	s.sectionNote.SetText("")
	s.rowsBox.Objects = nil
	s.rowsBox.Refresh()
	s.statusLbl.SetText("(no file)")
	s.sectionList.Refresh()
}

func (s *State) renderAll() {
	if s.cdf == nil { s.showEmpty(); return }
	s.titleLbl.Text = s.cdf.Meta.Title
	s.titleLbl.Refresh()
	if s.cdf.Meta.Subtitle != "" {
		s.subtitleLbl.Text = s.cdf.Meta.Subtitle
	} else {
		s.subtitleLbl.Text = " "
	}
	s.subtitleLbl.Refresh()
	s.metaLbl.SetText("updated " + s.cdf.Meta.Updated)
	s.sectionList.Refresh()
	if s.selection >= len(s.cdf.Sections) { s.selection = 0 }
	if len(s.cdf.Sections) > 0 {
		s.sectionList.Select(s.selection)
	}
	s.renderSection()
	s.updateStatus()
	s.updateDirty()
}

func (s *State) renderSection() {
	if s.cdf == nil || len(s.cdf.Sections) == 0 {
		s.sectionHdr.Text = "(no sections)"
		s.sectionHdr.Refresh()
		s.sectionNote.SetText("")
		s.rowsBox.Objects = nil
		s.rowsBox.Refresh()
		return
	}
	if s.selection < 0 { s.selection = 0 }
	if s.selection >= len(s.cdf.Sections) { s.selection = len(s.cdf.Sections) - 1 }
	sec := s.cdf.Sections[s.selection]
	s.sectionHdr.Text = sec.Name
	s.sectionHdr.Refresh()
	s.sectionNote.SetText(sec.Note)

	// Header row
	headerRow := s.buildHeaderRow()

	rows := []fyne.CanvasObject{headerRow}
	for i := range sec.Rows {
		i := i
		rows = append(rows, s.buildRow(i, &sec.Rows[i]))
	}
	s.rowsBox.Objects = rows
	s.rowsBox.Refresh()
}

func (s *State) buildHeaderRow() fyne.CanvasObject {
	mk := func(text string) fyne.CanvasObject {
		t := canvas.NewText(text, colMuted)
		t.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
		t.TextSize = 10
		return container.NewPadded(t)
	}
	// Grid layout with fixed proportions via container.NewGridWithColumns won't give us weights;
	// we use a manual GridLayout via container.New with our custom weighted layout.
	return &fyne.Container{
		Layout: &rowLayout{dateW: 110, actionW: 0.30, minRowH: 26, isHeader: true},
		Objects: []fyne.CanvasObject{
			mk("DATE"),
			mk("ACTION"),
			mk("DETAIL"),
			container.NewWithoutLayout(), // spacer for delete button column
		},
	}
}

func (s *State) buildRow(idx int, r *Row) fyne.CanvasObject {
	// A row = 4 cells (date, action, detail, delete-btn), each independently editable.
	// Cells are borderless clickable widgets; a click opens an editor dialog.
	dateCell := s.newCell(r.Date, "date", idx, r)
	actionCell := s.newActionCell(r, idx)
	detailCell := s.newCell(r.Detail, "detail", idx, r)
	delBtn := widget.NewButton("×", func() { s.deleteRow(idx) })
	delBtn.Importance = widget.LowImportance

	return &fyne.Container{
		Layout: &rowLayout{dateW: 110, actionW: 0.30, minRowH: 44, hasBorder: true},
		Objects: []fyne.CanvasObject{dateCell, actionCell, detailCell, delBtn},
	}
}

// newCell returns a clickable label whose click opens an editor.
func (s *State) newCell(text, field string, idx int, r *Row) fyne.CanvasObject {
	// Show text with word wrap; wrap in a tappable overlay so click opens the editor.
	lbl := widget.NewLabel(text)
	lbl.Wrapping = fyne.TextWrapWord
	if field == "date" { lbl.TextStyle = fyne.TextStyle{Monospace: true} }
	if text == "" { lbl.SetText("—") }

	return newTappable(lbl, func() { s.editCell(field, idx, r) })
}

// newActionCell renders a colored marker + text.
func (s *State) newActionCell(r *Row, idx int) fyne.CanvasObject {
	dot := canvas.NewCircle(markerColor(r.Marker()))
	dot.Resize(fyne.NewSize(8, 8))
	dotBox := container.NewGridWrap(fyne.NewSize(10, 10), dot)

	lbl := widget.NewLabel(r.Action)
	lbl.Wrapping = fyne.TextWrapWord
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	if r.Action == "" { lbl.SetText("(click to edit)") }

	content := container.NewBorder(nil, nil, container.NewPadded(dotBox), nil, lbl)
	return newTappable(content, func() { s.editCell("action", idx, r) })
}

func markerColor(m Marker) color.Color {
	switch m {
	case MarkerMilestone: return colAccent
	case MarkerExposure:  return colDanger
	case MarkerDeferred:  return colWarn
	}
	return colMuted
}

// updateStatus refreshes the bottom status bar.
func (s *State) updateStatus() {
	if s.path == "" {
		s.statusLbl.SetText("(unsaved)")
	} else {
		s.statusLbl.SetText(s.path)
	}
}

// updateDirty toggles the top-right dirty dot.
func (s *State) updateDirty() {
	if s.dirty {
		s.dirtyDot.FillColor = colWarn
	} else {
		s.dirtyDot.FillColor = color.Transparent
	}
	s.dirtyDot.Refresh()
}

func (s *State) markDirty() {
	s.dirty = true
	s.updateDirty()
}

// mutations ————————————————————————————————————————————————————

func (s *State) editCell(field string, idx int, r *Row) {
	var cur string
	switch field {
	case "date":   cur = r.Date
	case "action": cur = r.Action
	case "detail": cur = r.Detail
	}
	entry := widget.NewMultiLineEntry()
	entry.SetText(cur)
	entry.Wrapping = fyne.TextWrapWord
	entry.SetMinRowsVisible(6)

	label := strings.ToUpper(field[:1]) + field[1:]
	form := dialog.NewCustomConfirm("Edit " + label, "Save", "Cancel",
		container.NewPadded(entry),
		func(ok bool) {
			if !ok { return }
			nv := entry.Text
			switch field {
			case "date":   if nv != r.Date   { r.Date = nv;   s.markDirty(); s.renderSection() }
			case "action": if nv != r.Action { r.Action = nv; s.markDirty(); s.renderSection() }
			case "detail": if nv != r.Detail { r.Detail = nv; s.markDirty(); s.renderSection() }
			}
		}, s.win)
	form.Resize(fyne.NewSize(700, 400))
	form.Show()
}

func (s *State) addRow() {
	if s.cdf == nil { return }
	if len(s.cdf.Sections) == 0 {
		s.addSection()
		return
	}
	sec := &s.cdf.Sections[s.selection]
	sec.Rows = append(sec.Rows, Row{Date: time.Now().Format("2006-01-02"), Action: ""})
	s.markDirty()
	s.renderSection()
}

func (s *State) deleteRow(idx int) {
	sec := &s.cdf.Sections[s.selection]
	dialog.NewConfirm("Delete row?", fmt.Sprintf("Delete row %d?", idx+1), func(ok bool) {
		if !ok { return }
		sec.Rows = append(sec.Rows[:idx], sec.Rows[idx+1:]...)
		s.markDirty()
		s.renderSection()
	}, s.win).Show()
}

func (s *State) addSection() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("Section name")
	dialog.NewCustomConfirm("Add section", "Add", "Cancel", container.NewPadded(entry),
		func(ok bool) {
			if !ok || entry.Text == "" { return }
			id := SectionIDFromName(entry.Text)
			if s.cdf == nil { s.newDocumentFromEmpty(entry.Text) }
			s.cdf.Sections = append(s.cdf.Sections, Section{ID: id, Name: entry.Text, Rows: []Row{}})
			s.selection = len(s.cdf.Sections) - 1
			s.markDirty()
			s.renderAll()
		}, s.win).Show()
}

func (s *State) renameSection() {
	if s.cdf == nil || len(s.cdf.Sections) == 0 { return }
	cur := s.cdf.Sections[s.selection].Name
	entry := widget.NewEntry()
	entry.SetText(cur)
	dialog.NewCustomConfirm("Rename section", "Save", "Cancel", container.NewPadded(entry),
		func(ok bool) {
			if !ok || entry.Text == "" || entry.Text == cur { return }
			s.cdf.Sections[s.selection].Name = entry.Text
			s.markDirty()
			s.renderAll()
		}, s.win).Show()
}

func (s *State) editSectionNote() {
	if s.cdf == nil || len(s.cdf.Sections) == 0 { return }
	sec := &s.cdf.Sections[s.selection]
	entry := widget.NewMultiLineEntry()
	entry.SetText(sec.Note)
	entry.Wrapping = fyne.TextWrapWord
	entry.SetMinRowsVisible(6)
	form := dialog.NewCustomConfirm("Edit note", "Save", "Cancel",
		container.NewPadded(entry),
		func(ok bool) {
			if !ok || entry.Text == sec.Note { return }
			sec.Note = entry.Text
			s.markDirty()
			s.renderSection()
		}, s.win)
	form.Resize(fyne.NewSize(700, 300))
	form.Show()
}

func (s *State) deleteSection() {
	if s.cdf == nil || len(s.cdf.Sections) == 0 { return }
	name := s.cdf.Sections[s.selection].Name
	dialog.NewConfirm("Delete section?", fmt.Sprintf("Delete section \"%s\" and all its rows?", name),
		func(ok bool) {
			if !ok { return }
			s.cdf.Sections = append(s.cdf.Sections[:s.selection], s.cdf.Sections[s.selection+1:]...)
			if s.selection > 0 { s.selection-- }
			s.markDirty()
			s.renderAll()
		}, s.win).Show()
}

// file ops ——————————————————————————————————————————————————————

func (s *State) newDocument() {
	if s.dirty {
		dialog.NewConfirm("Discard unsaved changes?", "Continue anyway?", func(ok bool) {
			if !ok { return }
			s.newDocumentFromEmpty("Untitled")
		}, s.win).Show()
		return
	}
	s.newDocumentFromEmpty("Untitled")
}

func (s *State) newDocumentFromEmpty(title string) {
	s.cdf = NewSkeleton(title)
	s.path = ""
	s.selection = 0
	s.dirty = true
	s.renderAll()
	s.win.SetTitle("CDF — " + title)
}

// openPathDialog shows a small paste-a-path prompt so the user can bypass the
// file picker entirely. A file path loads directly; a directory path becomes
// the picker's starting location and reopens the standard file dialog.
// Supports ~ prefix and trims surrounding whitespace/quotes (the shape a path
// arrives in when copied from a terminal or file manager).
func (s *State) openPathDialog() {
	defer s.logPanic("openPathDialog")
	entry := widget.NewEntry()
	entry.SetPlaceHolder("/path/to/file.cdf  or  /path/to/dir")

	hint := widget.NewLabel("Paste a full path to a .cdf/.json file to open it directly, or a directory to browse from there.")
	hint.Wrapping = fyne.TextWrapWord

	body := container.NewVBox(hint, entry)

	dlg := dialog.NewCustomConfirm("Open Path", "Open", "Cancel",
		container.NewPadded(body),
		func(ok bool) {
			defer s.logPanic("openPathDialog.callback")
			if !ok {
				return
			}
			path := strings.TrimSpace(entry.Text)
			// Strip a pair of matching surrounding quotes (common when
			// paths get copied from terminal output).
			if len(path) >= 2 {
				a, b := path[0], path[len(path)-1]
				if (a == '"' && b == '"') || (a == '\'' && b == '\'') {
					path = path[1 : len(path)-1]
				}
			}
			// Expand ~ / ~user is out of scope; ~ alone is fine too.
			if path == "~" {
				if home, err := os.UserHomeDir(); err == nil {
					path = home
				}
			} else if strings.HasPrefix(path, "~/") {
				if home, err := os.UserHomeDir(); err == nil {
					path = filepath.Join(home, path[2:])
				}
			}
			// file:// URIs (Nemo/other file managers sometimes copy these).
			if strings.HasPrefix(path, "file://") {
				if u, err := url.Parse(path); err == nil && u.Path != "" {
					path = u.Path
				} else {
					path = path[7:]
				}
			}
			if path == "" {
				return
			}
			fi, err := os.Stat(path)
			if err != nil {
				dialog.ShowError(fmt.Errorf("path not found: %w", err), s.win)
				return
			}
			if fi.IsDir() {
				// Remember it and reopen the standard picker starting there.
				_ = os.WriteFile(s.lastDirFile(), []byte(path), 0644)
				s.openDialog()
				return
			}
			s.loadFile(path)
			s.rememberDir(path)
		}, s.win)
	dlg.Resize(fyne.NewSize(560, 180))
	dlg.Show()
	s.win.Canvas().Focus(entry)
}

func (s *State) openDialog() {
	defer s.logPanic("openDialog")
	// Prefer the native GTK dialog on Linux (zenity) — it shows mounted disks,
	// bookmarks, etc., the way every other GTK app does.
	if hasZenity() {
		go func() {
			defer s.logPanic("openDialog.zenity-go")
			startDir := s.lastDir()
			if p, ok := nativeOpen(startDir); ok && p != "" {
				fyne.Do(func() {
					s.loadFile(p)
					s.rememberDir(p)
				})
			}
		}()
		return
	}
	fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		defer s.logPanic("openDialog.callback")
		if err != nil {
			dialog.ShowError(err, s.win)
			return
		}
		if r == nil { return }
		u := r.URI()
		r.Close()   // close before load; we re-open by path
		if u == nil {
			dialog.ShowError(fmt.Errorf("no URI returned"), s.win)
			return
		}
		// Prefer Path() when available; some backends return "" — fall back to parsing String().
		path := u.Path()
		if path == "" {
			str := u.String()
			if strings.HasPrefix(str, "file://") { path = str[7:] } else { path = str }
		}
		if path == "" {
			dialog.ShowError(fmt.Errorf("could not extract path from URI: %s", u.String()), s.win)
			return
		}
		s.loadFile(path)
		s.rememberDir(path)
	}, s.win)
	// SetFilter is best-effort — some backends ignore it. Wrap in recovery just in case.
	func() {
		defer s.logPanic("openDialog.SetFilter")
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".cdf", ".json"}))
	}()
	// Try to start the Fyne dialog at the last-used directory
	if d := s.lastDir(); d != "" {
		if uri, err := storage.ListerForURI(storage.NewFileURI(d)); err == nil {
			func() { defer s.logPanic("openDialog.SetLocation"); fd.SetLocation(uri) }()
		}
	}
	fd.Show()
}

// ─── remember last-used directory ───────────────────────────────

func (s *State) lastDirFile() string {
	dir, err := os.UserConfigDir()
	if err != nil { return "" }
	d := filepath.Join(dir, "cdf-gui")
	os.MkdirAll(d, 0755)
	return filepath.Join(d, "last-dir")
}

func (s *State) lastDir() string {
	b, err := os.ReadFile(s.lastDirFile())
	if err != nil { return "" }
	return strings.TrimSpace(string(b))
}

func (s *State) rememberDir(pathOrFile string) {
	d := pathOrFile
	if fi, err := os.Stat(d); err == nil && !fi.IsDir() {
		d = filepath.Dir(d)
	}
	if d == "" { return }
	_ = os.WriteFile(s.lastDirFile(), []byte(d), 0644)
}

func (s *State) loadFile(path string) {
	c, err := LoadCDF(path)
	if err != nil {
		dialog.ShowError(fmt.Errorf("open %s: %w", path, err), s.win)
		return
	}
	s.cdf = c
	s.path = path
	s.selection = 0
	s.dirty = false
	s.renderAll()
	s.win.SetTitle("CDF — " + filepath.Base(path))
}

func (s *State) save() {
	if s.cdf == nil { return }
	if s.path == "" { s.saveAsDialog(); return }
	if err := SaveCDF(s.path, s.cdf); err != nil {
		dialog.ShowError(err, s.win); return
	}
	s.dirty = false
	s.updateDirty()
	s.updateStatus()
	s.metaLbl.SetText("updated " + s.cdf.Meta.Updated)
}

func (s *State) saveAsDialog() {
	defer s.logPanic("saveAsDialog")
	if s.cdf == nil { return }
	if hasZenity() {
		defName := SectionIDFromName(s.cdf.Meta.Title) + ".cdf"
		go func() {
			defer s.logPanic("saveAsDialog.zenity-go")
			if p, ok := nativeSave("Save CDF As", defName, s.lastDir()); ok && p != "" {
				if !strings.HasSuffix(strings.ToLower(p), ".cdf") { p = p + ".cdf" }
				fyne.Do(func() {
					if err := SaveCDF(p, s.cdf); err != nil { dialog.ShowError(err, s.win); return }
					s.path = p
					s.dirty = false
					s.updateDirty()
					s.updateStatus()
					s.metaLbl.SetText("updated " + s.cdf.Meta.Updated)
					s.win.SetTitle("CDF — " + filepath.Base(p))
					s.rememberDir(p)
				})
			}
		}()
		return
	}
	fd := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		defer s.logPanic("saveAsDialog.callback")
		if err != nil { dialog.ShowError(err, s.win); return }
		if w == nil { return }
		u := w.URI()
		w.Close()
		if u == nil { dialog.ShowError(fmt.Errorf("no URI returned"), s.win); return }
		p := u.Path()
		if p == "" {
			str := u.String()
			if strings.HasPrefix(str, "file://") { p = str[7:] } else { p = str }
		}
		if !strings.HasSuffix(strings.ToLower(p), ".cdf") { p = p + ".cdf" }
		if err := SaveCDF(p, s.cdf); err != nil { dialog.ShowError(err, s.win); return }
		s.path = p
		s.dirty = false
		s.updateDirty()
		s.updateStatus()
		s.metaLbl.SetText("updated " + s.cdf.Meta.Updated)
		s.win.SetTitle("CDF — " + filepath.Base(p))
	}, s.win)
	func() { defer s.logPanic("saveAsDialog.SetFileName"); fd.SetFileName(SectionIDFromName(s.cdf.Meta.Title) + ".cdf") }()
	fd.Show()
}

func (s *State) exportHTMLDialog() {
	defer s.logPanic("exportHTMLDialog")
	if s.cdf == nil { return }
	if hasZenity() {
		defName := SectionIDFromName(s.cdf.Meta.Title) + ".html"
		go func() {
			defer s.logPanic("exportHTMLDialog.zenity-go")
			if p, ok := nativeSave("Export HTML", defName, s.lastDir()); ok && p != "" {
				if !strings.HasSuffix(strings.ToLower(p), ".html") { p = p + ".html" }
				fyne.Do(func() {
					if err := os.WriteFile(p, []byte(ToHTML(s.cdf)), 0644); err != nil { dialog.ShowError(err, s.win); return }
					s.rememberDir(p)
				})
			}
		}()
		return
	}
	fd := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		defer s.logPanic("exportHTMLDialog.callback")
		if err != nil { dialog.ShowError(err, s.win); return }
		if w == nil { return }
		defer w.Close()
		if _, err := w.Write([]byte(ToHTML(s.cdf))); err != nil { dialog.ShowError(err, s.win); return }
	}, s.win)
	func() { defer s.logPanic("exportHTMLDialog.SetFileName"); fd.SetFileName(SectionIDFromName(s.cdf.Meta.Title) + ".html") }()
	fd.Show()
}

func (s *State) showAbout() {
	link, _ := url.Parse("https://claude.com/claude-code")
	dialog.ShowCustom("About CDF Viewer", "OK",
		container.NewVBox(
			widget.NewLabelWithStyle("CDF Viewer", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewLabel("ClaudeDocFormat reader/editor"),
			widget.NewLabel(""),
			widget.NewLabel("Format: pure JSON, cdf_version 1"),
			widget.NewLabel("GUI: Fyne (Go)"),
			widget.NewHyperlink("Claude Code", link),
		), s.win)
}
