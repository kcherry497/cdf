package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// ─── tappable wrapper ───────────────────────────────────────────
// Wraps any CanvasObject so a click fires a callback. Adds a subtle
// hover background so cells feel clickable.

type tappable struct {
	widget.BaseWidget
	content   fyne.CanvasObject
	onTap     func()
	hoverBg   *canvas.Rectangle
	isHovered bool
}

func newTappable(content fyne.CanvasObject, onTap func()) *tappable {
	t := &tappable{content: content, onTap: onTap}
	t.hoverBg = canvas.NewRectangle(colBg)
	t.hoverBg.StrokeWidth = 0
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappable) CreateRenderer() fyne.WidgetRenderer {
	return &tappableRenderer{t: t, objs: []fyne.CanvasObject{t.hoverBg, t.content}}
}

func (t *tappable) Tapped(_ *fyne.PointEvent) { if t.onTap != nil { t.onTap() } }
func (t *tappable) MouseIn(_ *desktop.MouseEvent) {
	t.isHovered = true
	t.hoverBg.FillColor = colHover
	t.hoverBg.Refresh()
}
func (t *tappable) MouseOut() {
	t.isHovered = false
	t.hoverBg.FillColor = colBg
	t.hoverBg.Refresh()
}
func (t *tappable) MouseMoved(_ *desktop.MouseEvent) {}
func (t *tappable) Cursor() desktop.Cursor { return desktop.PointerCursor }

type tappableRenderer struct {
	t    *tappable
	objs []fyne.CanvasObject
}

func (r *tappableRenderer) Layout(size fyne.Size) {
	r.t.hoverBg.Resize(size)
	r.t.hoverBg.Move(fyne.NewPos(0, 0))
	r.t.content.Resize(size)
	r.t.content.Move(fyne.NewPos(0, 0))
}
func (r *tappableRenderer) MinSize() fyne.Size            { return r.t.content.MinSize() }
func (r *tappableRenderer) Refresh()                      { r.t.hoverBg.Refresh(); r.t.content.Refresh() }
func (r *tappableRenderer) Objects() []fyne.CanvasObject  { return r.objs }
func (r *tappableRenderer) Destroy()                      {}

// ─── row layout ─────────────────────────────────────────────────
// A 4-column layout: fixed-width date, percentage-width action,
// flex-fill detail, fixed-width delete button. Draws optional
// vertical + horizontal borders between cells and at the bottom.

type rowLayout struct {
	dateW    float32 // fixed date column width in px
	actionW  float32 // fraction of remaining width, e.g. 0.30
	minRowH  float32
	hasBorder bool
	isHeader  bool
}

func (l *rowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	h := l.minRowH
	for _, o := range objects {
		ms := o.MinSize()
		if ms.Height > h { h = ms.Height }
	}
	return fyne.NewSize(400, h + 8)
}

func (l *rowLayout) Layout(objects []fyne.CanvasObject, containerSize fyne.Size) {
	// Reserve fixed-width columns
	deleteW := float32(30.0)
	if l.isHeader { deleteW = 0 }
	remaining := containerSize.Width - l.dateW - deleteW - 24 // 24 = padding
	actionW := remaining * l.actionW
	detailW := remaining - actionW

	// Compute row height: take max of cell min-heights, respect minRowH
	rowH := l.minRowH
	for _, o := range objects {
		ms := o.MinSize()
		if ms.Height > rowH { rowH = ms.Height }
	}
	if len(objects) < 4 { return }

	x := float32(4.0)
	// date
	objects[0].Move(fyne.NewPos(x, 4))
	objects[0].Resize(fyne.NewSize(l.dateW, rowH))
	x += l.dateW + 4
	// action
	objects[1].Move(fyne.NewPos(x, 4))
	objects[1].Resize(fyne.NewSize(actionW, rowH))
	x += actionW + 4
	// detail
	objects[2].Move(fyne.NewPos(x, 4))
	objects[2].Resize(fyne.NewSize(detailW, rowH))
	x += detailW + 4
	// delete btn
	if l.isHeader {
		objects[3].Resize(fyne.NewSize(0, 0))
	} else {
		objects[3].Move(fyne.NewPos(x, 4))
		objects[3].Resize(fyne.NewSize(deleteW - 8, rowH))
	}
}

// helper for containers where the layout needs to see all children
var _ fyne.Layout = (*rowLayout)(nil)

// Prevent "declared and not used" warnings when we only conditionally
// import things elsewhere.
var _ = container.NewStack
